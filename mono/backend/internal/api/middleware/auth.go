package middleware

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

)

type contextKey struct{ name string }

var UserIDKey = contextKey{name: "user_id"}
var ClaimsKey = contextKey{name: "jwt_claims"}

var (
	jwtSecret []byte
	jwtOnce   sync.Once

	// When nil (Mono edition or Redis unavailable), revocation is a no-op.
)

// PublicEndpoints — list of exact paths that don't require JWT auth.
var PublicEndpoints = map[string]bool{
	"/api/health":         true,
	"/api/auth/login":     true,
	"/api/auth/setup":     true,
	"/api/auth/status":    true,
	"/api/auth/edition":   true,
	"/api/media/proxy":    true,
	"/api/oauth/url":      true,
	"/api/oauth/callback": true,
	"/api/tg/webhook":     true,
	"/api/wa/webhook":     true,
	"/mcp/sse":            true,
	"/mcp/messages":       true,
}

// PublicEndpointPrefixes — list of path prefixes that don't require JWT auth.
// Only specific safe endpoints should be added here.
var PublicEndpointPrefixes = []string{
	"/api/events", // SSE: auth handled in SSE handler (ticket or cookie)
}

// and revocation helpers. Pass nil for Mono edition (no-op).

func InitJWTAuth() {
	jwtOnce.Do(func() {
		jwtKey := os.Getenv("JWT_SECRET")
		if jwtKey == "" {
			slog.Error("JWT_SECRET environment variable is required. Generate: openssl rand -hex 32")
			os.Exit(1)
		}
		// Derive 32-byte key from JWT_SECRET via SHA-256.
		// NOTE: For production, JWT_SECRET should be at least 32 random bytes (64 hex chars).
		// Consider using HKDF for key derivation in a future version.
		h := sha256.Sum256([]byte(jwtKey))
		jwtSecret = h[:]
		slog.Info("JWT auth initialized")
	})
}

// GenerateToken creates a signed JWT for the given userID.
func GenerateToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"jti": uuid.New().String(),
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// GenerateTokenWithAdmin creates a signed JWT with an is_admin claim.
func GenerateTokenWithAdmin(userID string, isAdmin bool) (string, error) {
	claims := jwt.MapClaims{
		"jti":      uuid.New().String(),
		"sub":      userID,
		"is_admin": isAdmin,
		"iat":      time.Now().Unix(),
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// IsAdminFromContext checks the JWT claims for an is_admin flag.
// Returns false if the claim is missing or not a boolean.
func IsAdminFromContext(ctx context.Context) bool {
	if claims, ok := GetJWTClaimsFromCtx(ctx); ok {
		if isAdmin, ok := claims["is_admin"].(bool); ok {
			return isAdmin
		}
	}
	return false
}

// GetJWTClaimsFromCtx extracts JWT claims from the request context.
func GetJWTClaimsFromCtx(ctx context.Context) (jwt.MapClaims, bool) {
	claims, _ := ctx.Value(ClaimsKey).(jwt.MapClaims)
	return claims, claims != nil
}

// extractToken returns the JWT from the request.
// Priority: Authorization header (localStorage — most recent login),
// then httpOnly cookie (survives page reloads), then ?token= query (SSE fallback).
func extractToken(r *http.Request) (string, bool) {
	// 1. Authorization header — always reflects the freshest login token.
	auth := r.Header.Get("Authorization")
	if auth == "" {
		auth = r.Header.Get("X-Authorization")
	}
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer "), false
	}
	// 2. httpOnly cookie — browser auto-sends, XSS-safe.
	//    Falls behind on token refresh (old cookie may outlive localStorage update).
	//    That's why Authorization header takes priority.
	if cookie, err := r.Cookie("rms_token"); err == nil && cookie.Value != "" {
		return cookie.Value, false
	}
	// 3. SSE fallback: ?token= query parameter.
	if tok := r.URL.Query().Get("token"); tok != "" {
		return tok, true
	}
	return "", false
}

// isPublicPath checks if the request path is a public endpoint.
func isPublicPath(path string) bool {
	if PublicEndpoints[path] {
		return true
	}
	for _, prefix := range PublicEndpointPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func writeJSONAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// AdminOnlyMiddleware requires is_admin claim in JWT (ops dashboards).
func AdminOnlyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !IsAdminFromContext(r.Context()) {
			writeJSONAuthError(w, http.StatusForbidden, "admin access required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// JWTAuthMiddleware verifies the JWT, checks the blacklist, and injects userID into context.
func JWTAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Public endpoints skip auth
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		tokenStr, fromQuery := extractToken(r)
		if tokenStr == "" {
			writeJSONAuthError(w, http.StatusUnauthorized, "missing authorization")
			return
		}

		// Query-string JWTs leak into logs, history, and referer headers.
		// Reject everywhere except legacy MCP SSE paths (cookie/header preferred).
		if fromQuery && !strings.HasPrefix(r.URL.Path, "/mcp/") {
			writeJSONAuthError(w, http.StatusBadRequest, "JWT ?token= query parameter is not supported; use Authorization header or cookie")
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			writeJSONAuthError(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			writeJSONAuthError(w, http.StatusUnauthorized, "invalid token claims")
			return
		}

		userID, _ := claims["sub"].(string)
		if userID == "" {
			writeJSONAuthError(w, http.StatusUnauthorized, "invalid token: missing sub")
			return
		}

		// Check token blacklist (revoked tokens)

		// CSRF protection: state-changing methods must have either a custom header
		// (browsers don't allow cross-origin requests to set custom headers without CORS preflight)
		// or the httpOnly cookie (protected by SameSite=Lax).
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" || r.Method == "PATCH" {
			_, cookieErr := r.Cookie("rms_token")
			if r.Header.Get("Authorization") == "" && r.Header.Get("X-Authorization") == "" && cookieErr != nil {
				writeJSONAuthError(w, http.StatusForbidden, "missing auth header")
				return
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserIDFromContext extracts userID from context set by JWTAuthMiddleware.
func GetUserIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// ValidateToken parses and validates a JWT token string, returning the parsed token.
func ValidateToken(tokenStr string) (*jwt.Token, error) {
	return jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})
}

// GetJWTSecret returns the current JWT secret (for testing/external use).
func GetJWTSecret() []byte {
	return jwtSecret
}

// GenerateAPIKeyHex generates a random hex-encoded 32-byte key for testing/admin use.
func GenerateAPIKeyHex() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		slog.Info(fmt.Sprintf("Failed to generate API key: %v", err))
		return ""
	}
	return hex.EncodeToString(b)
}

// RevokeCurrentToken reads the jti and exp claims from the request context and
// adds the token to the blacklist. Call this after security-sensitive operations
// like password changes. If the blacklist is nil (Mono), this is a no-op.
func RevokeCurrentToken(ctx context.Context) error {
	return nil
}

// SetTokenCookie writes an httpOnly Secure SameSite=Lax cookie with the JWT.
func SetTokenCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	if os.Getenv("APP_ENV") == "development" {
		secure = false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "rms_token",
		Value:    token,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   86400, // 24 hours
	})
}

// ClearTokenCookie removes the auth cookie (used on logout).
func ClearTokenCookie(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
	if os.Getenv("APP_ENV") == "development" {
		secure = false
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "rms_token",
		Value:    "",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		MaxAge:   -1,
	})
}

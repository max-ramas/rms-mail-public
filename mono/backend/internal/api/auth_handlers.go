package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"rmsmail/internal/api/middleware"
	"rmsmail/internal/edition"
	"rmsmail/internal/mail"

	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
)

// LoginRequest represents a login form submission.
type LoginRequest struct {
	Email          string `json:"email"`
	Password       string `json:"password"`
	IMAPHost       string `json:"imap_host,omitempty"`
	IMAPPort       int    `json:"imap_port,omitempty"`
	IMAPEncryption string `json:"imap_encryption,omitempty"`
	SMTPHost       string `json:"smtp_host,omitempty"`
	SMTPPort       int    `json:"smtp_port,omitempty"`
	SMTPEncryption string `json:"smtp_encryption,omitempty"`
}

// LoginResponse is returned on successful authentication.
type LoginResponse struct {
	Token   string `json:"token"`
	Edition string `json:"edition"`
	User    struct {
		Email string `json:"email"`
	} `json:"user"`
}

// HandleAuthStatus returns whether an admin user exists in the database.
// This is used by the frontend to decide whether to show the setup or login page.
func (h *Handler) HandleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if edition.IsMono() || edition.IsMonoPro() {
		// M version: no setup needed, always ready
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"setup_needed": false,
			"edition":      edition.Current.String(),
		})
		return
	}

	exists, err := h.Store.AdminExists(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	resp := map[string]interface{}{
		"setup_needed": !exists,
		"edition":      edition.Current.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// EditionInfo returns the current edition of the application.
// Public endpoint (no JWT required).
func (h *Handler) EditionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"edition":     edition.Current.String(),
		"llm_envonly": os.Getenv("LLM_ENVONLY") == "true",
		"tg_envonly":  os.Getenv("TG_ENVONLY") == "true",
		"ai_disabled": h.AIDisabled,
	})
}

// HandleVerifyToken returns 200 if the JWT token is valid.
// Used by the frontend auth-guard to detect stale tokens after JWT_SECRET rotation.
func (h *Handler) HandleVerifyToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID := middleware.GetUserIDFromContext(r.Context())
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid":   true,
		"user_id": userID,
	})
}

// HandleTicket generates a short-lived one-time ticket for SSE connections.
// Tickets are 30s TTL, single-use ("burn after reading"), and generated
// with crypto/rand for cryptographic strength.
func (h *Handler) HandleTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	userID := middleware.GetUserIDFromContext(r.Context())
	if userID == "" {
		WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	accountID := r.URL.Query().Get("account_id")
	ticket, err := h.ticketStore.GenerateTicket(userID, accountID)
	if err != nil {
		WriteJSONError(w, http.StatusInternalServerError, "failed to generate ticket")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"ticket": ticket})
}

// ValidateTicket checks a one-time ticket and returns the user ID if valid.
func (h *Handler) ValidateTicket(ticket string) string {
	data, ok := h.ticketStore.ValidateTicket(ticket)
	if ok {
		return data.UserID
	}
	return ""
}

// HandleSetup creates the first admin user (only if no admin exists yet).
func (h *Handler) HandleSetup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Check if admin already exists
	exists, err := h.Store.AdminExists(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	if edition.IsMono() || edition.IsMonoPro() {
		WriteJSONError(w, http.StatusForbidden, "Setup is not available in Mono edition")
		return
	}
	if exists {
		WriteJSONError(w, http.StatusConflict, "admin already exists")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteJSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	if len(req.Password) < 8 {
		WriteJSONError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	// Create admin in database
	_, err = h.Store.CreateAdmin(r.Context(), req.Email, string(hash))
	if err != nil {
		slog.Error(fmt.Sprintf(" creating admin: %v", err))
		WriteJSONError(w, http.StatusInternalServerError, "failed to create admin")
		return
	}

	// Generate JWT token (first admin always receives is_admin)
	token, err := h.issueAuthToken(r.Context(), req.Email, true)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(24 * time.Hour / time.Second), // 24 hours
	})

	resp := LoginResponse{
		Token:   token,
		Edition: edition.Current.String(),
	}
	resp.User.Email = req.Email

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) HandleGetMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	claims, ok := middleware.GetJWTClaimsFromCtx(r.Context())
	if !ok {
		WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	email, _ := claims["sub"].(string)
	isAdmin, _ := claims["is_admin"].(bool)
	role := "user"
	if isAdmin {
		role = "admin"
	}
	// Fetch actual role from users table; create record if missing (MP/U/T only)
	if email != "" && !edition.IsMono() {
		if user, err := h.Store.GetUserByEmail(r.Context(), email); err == nil && user != nil && user.Role != "" {
			role = user.Role
		} else {
			// Auto-create user record on first /me call (covers pre-existing logins)
			h.Store.UpsertUser(r.Context(), email, email, role)
		}
		// Update last_seen_at on the account (Mono Pro / Teams only)
		if edition.IsMonoPro() || edition.IsTeams() {
			if acct, err := h.Store.GetAccountCredentialsByEmail(r.Context(), email); err == nil && acct != nil {
				h.Store.UpdateAccountTimestamp(r.Context(), acct.ID, "last_seen")
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"email":    email,
		"is_admin": isAdmin,
		"role":     role,
	})
}

func (h *Handler) resolveAdminClaim(ctx context.Context, email string) bool {
	if edition.IsMono() {
		return true
	}

	if edition.IsMonoPro() {
		// 1. Check ADMIN_EMAIL env var
		adminEmail := os.Getenv("ADMIN_EMAIL")
		if adminEmail != "" && email == adminEmail {
			return true
		}

		// 2. Check users table role
		if user, err := h.Store.GetUserByEmail(ctx, email); err == nil && user != nil && user.Role == "admin" {
			return true
		}

		// 3. Fallback: Check if this user is the first registered account
		firstUserEmail, err := h.Store.GetFirstRegisteredAccountEmail(ctx)
		if err == nil && firstUserEmail != "" && email == firstUserEmail {
			return true
		}
		return false // Mono Pro users who are not the admin do not get is_admin
	}

	_, hash, err := h.Store.GetAdminByEmail(ctx, email)
	return err == nil && hash != ""
}

// issueAuthToken mints a JWT and optionally sets forceAdmin for first-time setup.
func (h *Handler) issueAuthToken(ctx context.Context, email string, forceAdmin bool) (string, error) {
	isAdmin := forceAdmin || h.resolveAdminClaim(ctx, email)
	return middleware.GenerateTokenWithAdmin(email, isAdmin)
}

// HandleChangePassword updates the authenticated admin password.
func (h *Handler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get authenticated user email from JWT context
	email := middleware.GetUserIDFromContext(r.Context())
	if email == "" {
		WriteJSONError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		WriteJSONError(w, http.StatusBadRequest, "current and new password are required")
		return
	}

	if len(req.NewPassword) < 8 {
		WriteJSONError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	// Verify current password
	_, passwordHash, err := h.Store.GetAdminByEmail(r.Context(), email)
	if err != nil || passwordHash == "" {
		WriteJSONError(w, http.StatusNotFound, "admin not found")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.CurrentPassword)); err != nil {
		WriteJSONError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	// Update password in database
	if err := h.Store.UpdateAdminPassword(r.Context(), email, string(newHash)); err != nil {
		slog.Error(fmt.Sprintf(" updating password: %v", err))
		WriteJSONError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	// Revoke the current JWT so old tokens can't be reused after password change
	if err := middleware.RevokeCurrentToken(r.Context()); err != nil {
		slog.Warn(fmt.Sprintf(" failed to revoke token after password change: %v", err))
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleLogin authenticates a user with email + password against IMAP (Roundcube-style).
// Falls back to bcrypt if IMAP is unreachable.
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteJSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	if !edition.IsMono() && !edition.IsMonoPro() {
		// Unified / Teams — authenticate strictly against the database
		_, dbHash, err := h.Store.GetAdminByEmail(r.Context(), req.Email)
		if err != nil {
			// DB error (pool exhausted, timeout) — not the user's fault
			slog.Error("login: failed to query admin", "error", err, "email", req.Email)
			WriteJSONError(w, http.StatusServiceUnavailable, "temporarily unavailable, try again")
			return
		}
		if dbHash == "" {
			WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(dbHash), []byte(req.Password)) != nil {
			WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		token, err := h.issueAuthToken(r.Context(), req.Email, false)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
		middleware.SetTokenCookie(w, r, token)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token:   token,
			Edition: edition.Current.String(),
		})
		return
	}

	// Mono edition — Try IMAP LOGIN first (Roundcube-style)
	var imapHost, imapEncryption, smtpHost, smtpEncryption string
	var imapPort, smtpPort int
	var useSSL bool

	if req.IMAPHost != "" && req.SMTPHost != "" {
		imapHost = req.IMAPHost
		imapPort = req.IMAPPort
		imapEncryption = req.IMAPEncryption
		smtpHost = req.SMTPHost
		smtpPort = req.SMTPPort
		smtpEncryption = req.SMTPEncryption
		useSSL = (req.SMTPEncryption == "ssl")
	} else {
		resolved, err := mail.Resolve(r.Context(), req.Email)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "resolution_failed",
				"code":  "ERROR_RESOLUTION_FAILED",
			})
			return
		}
		imapHost = resolved.IMAPHost
		imapPort = resolved.IMAPPort
		imapEncryption = resolved.IMAPEncryption
		smtpHost = resolved.SMTPHost
		smtpPort = resolved.SMTPPort
		useSSL = resolved.UseSSL
		if useSSL {
			smtpEncryption = "ssl"
		} else {
			smtpEncryption = "starttls"
		}
	}

	var token string

	if err := mail.ValidateManualConfig(imapHost, smtpHost); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": "host_not_allowed",
			"msg":   err.Error(),
		})
		return
	}

	if false {

	}

	loginErr := tryIMAPLogin(req.Email, req.Password, imapHost, imapPort, imapEncryption)
	if loginErr == nil {
		// IMAP login successful — create account if needed, issue JWT
		acct, err := h.Store.GetAccountCredentialsByEmail(r.Context(), req.Email)
		if err != nil || acct == nil {
			// Auto-create account
			newAcc, err := h.Store.CreateAccount(r.Context(),
				req.Email, "", "custom", imapHost, imapPort, useSSL, imapEncryption,
				smtpHost, smtpPort, useSSL, smtpEncryption,
				req.Email, req.Password, "{}", "")
			if err != nil {
				WriteInternalError(w, r, err)
				return
			}
			acct = newAcc

			if h.SyncManager != nil {
				h.SyncManager.TriggerRefresh()
			}
		}

		// Cache bcrypt hash for offline fallback
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		h.Store.UpdateAdminPassword(r.Context(), req.Email, string(hash))

		token, err = h.issueAuthToken(r.Context(), req.Email, false)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
	} else {
		// IMAP failed — try bcrypt (offline fallback)
		_, dbHash, err := h.Store.GetAdminByEmail(r.Context(), req.Email)
		if err != nil || dbHash == "" {
			WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(dbHash), []byte(req.Password)) != nil {
			WriteJSONError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		// Generate token for cached credentials
		token, err = h.issueAuthToken(r.Context(), req.Email, false)
		if err != nil {
			WriteInternalError(w, r, err)
			return
		}
	}

	// Ensure user record in users table with proper role
	{
		isAdmin := h.resolveAdminClaim(r.Context(), req.Email)
		role := "user"
		if isAdmin {
			role = "admin"
		}
		h.Store.UpsertUser(r.Context(), req.Email, req.Email, role)
	}

	middleware.SetTokenCookie(w, r, token)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(LoginResponse{
		Token:   token,
		Edition: edition.Current.String(),
	})
}

// HandleLogout revokes the current JWT (adds JTI to blacklist) and clears the httpOnly cookie.
func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	middleware.RevokeCurrentToken(r.Context())
	middleware.ClearTokenCookie(w, r)
	w.WriteHeader(http.StatusOK)
}

// tryIMAPLogin attempts IMAP authentication, trying TLS, STARTTLS, and plain
// connections in sequence until one succeeds or all fail.
// Uses IMAP LOGIN command (most compatible) with SASL PLAIN as fallback.
func tryIMAPLogin(email, password, host string, port int, encryption string) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	type attempt struct {
		dial func() (*imapclient.Client, error)
		name string
	}

	var attempts []attempt
	switch encryption {
	case "starttls":
		attempts = []attempt{
			{func() (*imapclient.Client, error) {
				return imapclient.DialStartTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "STARTTLS"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialInsecure(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "plain"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "TLS"},
		}
	case "none":
		attempts = []attempt{
			{func() (*imapclient.Client, error) {
				return imapclient.DialInsecure(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "plain"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialStartTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "STARTTLS"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "TLS"},
		}
	default: // "ssl" or unknown — try TLS first
		attempts = []attempt{
			{func() (*imapclient.Client, error) {
				return imapclient.DialTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "TLS"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialStartTLS(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "STARTTLS"},
			{func() (*imapclient.Client, error) {
				return imapclient.DialInsecure(addr, &imapclient.Options{
					Dialer:    &net.Dialer{Timeout: 5 * time.Second},
					TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
				})
			}, "plain"},
		}
	}

	var lastErr error
	for _, a := range attempts {
		client, err := a.dial()
		if err != nil {
			lastErr = err
			slog.Info(fmt.Sprintf("tryIMAPLogin: dial %s %s failed: %v", a.name, addr, err))
			continue
		}
		// Use IMAP LOGIN command (compatible with Dovecot, Courier, etc.)
		// Fall back to SASL PLAIN if server requires it.
		loginErr := client.Login(email, password).Wait()
		if loginErr != nil {
			saslClient := sasl.NewPlainClient("", email, password)
			if authErr := client.Authenticate(saslClient); authErr != nil {
				client.Close()
				slog.Info(fmt.Sprintf("tryIMAPLogin: auth failed via %s for %s: %v", a.name, email, authErr))
				// Auth error = wrong credentials, no point trying other dial methods
				return authErr
			}
		}
		client.Close()
		slog.Info(fmt.Sprintf("tryIMAPLogin: success via %s for %s", a.name, email))
		return nil
	}
	return lastErr
}

// ScanLocalResponse is returned by POST /api/auth/scan-local
type ScanLocalResponse struct {
	Found    bool               `json:"found"`
	Accounts []ScanLocalAccount `json:"accounts"`
}

// ScanLocalAccount represents a discovered local mailbox candidate.
type ScanLocalAccount struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	IMAPHost string `json:"imap_host"`
	IMAPPort int    `json:"imap_port"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	UseSSL   bool   `json:"use_ssl"`
}

// HandleScanLocal scans localhost IMAP for common mailbox names using the user's password.
// Only works in Unified edition.
// POST /api/auth/scan-local
//
// Body: { "email": "user@domain.com", "password": "user-password" }
func (h *Handler) HandleScanLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if edition.IsMono() || edition.IsMonoPro() {
		writeJSON(w, http.StatusOK, ScanLocalResponse{Found: false})
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		WriteJSONError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	// Extract domain from email
	parts := strings.SplitN(req.Email, "@", 2)
	if len(parts) != 2 {
		writeJSON(w, http.StatusOK, ScanLocalResponse{Found: false})
		return
	}
	domain := parts[1]

	settings, err := mail.Resolve(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusOK, ScanLocalResponse{Found: false})
		return
	}

	// If the resolved host is not localhost, we don't scan
	if !isLocalAddr(settings.IMAPHost) {
		writeJSON(w, http.StatusOK, ScanLocalResponse{Found: false})
		return
	}

	// Common local mailbox names to probe
	candidates := []string{
		"admin@" + domain,
		"webmaster@" + domain,
		"postmaster@" + domain,
		"mailer-daemon@" + domain,
		"noreply@" + domain,
		"info@" + domain,
		"support@" + domain,
		"contact@" + domain,
		"hello@" + domain,
		"mail@" + domain,
		req.Email, // also try the user's own email
	}

	// Deduplicate
	seen := map[string]bool{}
	var unique []string
	for _, c := range candidates {
		if !seen[c] {
			seen[c] = true
			unique = append(unique, c)
		}
	}

	var discovered []ScanLocalAccount

	for _, candidate := range unique {
		if testIMAPLogin(settings.IMAPHost, settings.IMAPPort, candidate, req.Password, settings.UseSSL) {
			discovered = append(discovered, ScanLocalAccount{
				Email:    candidate,
				Username: candidate,
				IMAPHost: settings.IMAPHost,
				IMAPPort: settings.IMAPPort,
				SMTPHost: settings.SMTPHost,
				SMTPPort: settings.SMTPPort,
				UseSSL:   settings.UseSSL,
			})
		}
	}

	writeJSON(w, http.StatusOK, ScanLocalResponse{
		Found:    len(discovered) > 0,
		Accounts: discovered,
	})
}

// HandleImportLocal imports discovered local mailboxes into the database.
// POST /api/auth/import-local
//
// Body: { "accounts": [{ "email": "...", "username": "...", "imap_host": "...", ... }], "password": "..." }
func (h *Handler) HandleImportLocal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if edition.IsMono() || edition.IsMonoPro() {
		writeJSON(w, http.StatusOK, map[string]interface{}{"imported": 0})
		return
	}

	var req struct {
		Accounts []ScanLocalAccount `json:"accounts"`
		Password string             `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	imported := 0
	for _, acc := range req.Accounts {
		imapEncryption := "none"
		if acc.UseSSL {
			imapEncryption = "ssl"
		}
		smtpEncryption := "starttls"
		if acc.UseSSL {
			smtpEncryption = "ssl"
		}

		_, err := h.Store.CreateAccount(
			r.Context(),
			acc.Email,
			"",
			"manual",
			acc.IMAPHost,
			acc.IMAPPort,
			acc.UseSSL,
			imapEncryption,
			acc.SMTPHost,
			acc.SMTPPort,
			acc.UseSSL,
			smtpEncryption,
			acc.Username,
			req.Password,
			"",
			"",
		)
		if err != nil {
			slog.Info(fmt.Sprintf("import-local: failed to create account %s: %v", acc.Email, err))
			continue
		}
		imported++
	}

	slog.Info(fmt.Sprintf("import-local: imported %d accounts", imported))
	if imported > 0 && h.SyncManager != nil {
		h.SyncManager.TriggerRefresh()
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"imported": imported})
}

// testIMAPLogin attempts an IMAP login with the given credentials.
func testIMAPLogin(host string, port int, username, password string, useSSL bool) bool {
	addr := net.JoinHostPort(host, strconv.Itoa(port))

	var client *imapclient.Client
	var dialErr error

	opts := &imapclient.Options{
		Dialer:    &net.Dialer{Timeout: 5 * time.Second},
		TLSConfig: &tls.Config{InsecureSkipVerify: os.Getenv("ALLOW_INSECURE_TLS") == "true"},
	}
	if useSSL {
		client, dialErr = imapclient.DialTLS(addr, opts)
	} else {
		client, dialErr = imapclient.DialInsecure(addr, opts)
	}
	if dialErr != nil {
		// Try STARTTLS on insecure dial
		if !useSSL {
			client2, err2 := imapclient.DialStartTLS(addr, opts)
			if err2 != nil {
				return false
			}
			client = client2
		} else {
			return false
		}
	}
	defer client.Close()

	loginErr := client.Login(username, password).Wait()
	if loginErr != nil {
		return false
	}

	// Clean logout (fire-and-forget, connection will be closed)
	client.Logout()
	return true
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// isLocalAddr checks if the host is a local/loopback address.
func isLocalAddr(host string) bool {
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

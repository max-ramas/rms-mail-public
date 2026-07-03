package middleware

import (
	"net/http"
	"os"
	"strings"
)

// AllowedOrigin returns the request Origin if it is on the allowlist, otherwise "".
func AllowedOrigin(r *http.Request) string {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return ""
	}

	isDev := os.Getenv("APP_ENV") != "production"
	if isDev && (origin == "http://localhost:3000" || origin == "http://127.0.0.1:3000" ||
		origin == "http://localhost:3500" || origin == "http://127.0.0.1:3500" ||
		origin == "http://localhost:8087") {
		return origin
	}

	if appURL := os.Getenv("NEXT_PUBLIC_APP_URL"); appURL != "" && origin == appURL {
		return origin
	}

	if allowedOrigins := os.Getenv("ALLOWED_ORIGINS"); allowedOrigins != "" {
		for _, o := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(o) == origin {
				return origin
			}
		}
	}

	return ""
}

// SetCORSHeaders applies allowlisted CORS headers for MCP and other handlers outside the main wrapper.
func SetCORSHeaders(w http.ResponseWriter, r *http.Request, allowHeaders string) {
	if origin := AllowedOrigin(r); origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	if allowHeaders != "" {
		w.Header().Set("Access-Control-Allow-Headers", allowHeaders)
	}
}

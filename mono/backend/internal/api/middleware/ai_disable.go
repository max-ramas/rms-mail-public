package middleware

import (
	"net/http"
	"strings"
)

var aiRoutes = []string{
	"/api/ai/chat",
	"/api/ai/models",
	"/api/ai/categorize",
	"/api/ai/stats",
	"/api/ai/log",
	"/api/ai/settings",
}

// AIDisableMiddleware returns a middleware that blocks AI-related API routes
// when AI is disabled, returning HTTP 403 Forbidden.
func AIDisableMiddleware(aiDisabled bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !aiDisabled {
			next.ServeHTTP(w, r)
			return
		}

		path := r.URL.Path

		// Check exact AI routes
		for _, route := range aiRoutes {
			if path == route {
				WriteJSONError(w, http.StatusForbidden, "AI features are disabled")
				return
			}
		}

		// Check /api/emails/.../summarize and /api/emails/.../categorize
		if strings.HasPrefix(path, "/api/emails/") {
			for _, suffix := range []string{"/summarize", "/categorize"} {
				if strings.HasSuffix(path, suffix) {
					WriteJSONError(w, http.StatusForbidden, "AI features are disabled")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

package middleware

import (
	"net/http"

	"rmsmail/internal/sentry"

	sentrygo "github.com/getsentry/sentry-go"
)

// SentryMiddleware wraps an HTTP handler with Sentry error tracking.
// Panics are caught and reported. The request context is added to the Sentry scope.
// When SENTRY_DSN is unset, this is a pass-through no-op.
func SentryMiddleware(next http.Handler) http.Handler {
	if !sentry.IsEnabled() {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub := sentrygo.CurrentHub().Clone()
		hub.Scope().SetRequest(r)
		ctx := sentrygo.SetHubOnContext(r.Context(), hub)
		r = r.WithContext(ctx)

		defer func() {
			if rec := recover(); rec != nil {
				hub.Recover(rec)
				writeJSONAuthError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

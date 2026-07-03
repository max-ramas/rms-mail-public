// Package sentry provides optional GlitchTip/Sentry error monitoring.
// If SENTRY_DSN is not set, all functions are no-ops (zero overhead).
package sentry

import (
	"log/slog"
	"os"
	"strings"
	"time"

	sentrygo "github.com/getsentry/sentry-go"
)

// Init starts the Sentry SDK. If SENTRY_DSN is empty, this is a no-op.
func Init() {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return
	}
	env := os.Getenv("SENTRY_ENVIRONMENT")
	if env == "" {
		env = "production"
	}
	release := os.Getenv("APP_VERSION")

	err := sentrygo.Init(sentrygo.ClientOptions{
		Dsn:              dsn,
		Environment:      env,
		Release:          release,
		TracesSampleRate: 0.0, // no performance tracing yet
	})
	if err != nil {
		slog.Error("sentry init failed", "error", err)
		return
	}

	// Log masked DSN for debugging (hide secret key)
	masked := dsn
	if idx := strings.LastIndex(dsn, "@"); idx >= 0 {
		masked = "https://***" + dsn[idx:]
	}
	slog.Info("sentry initialized", "env", env, "release", release, "dsn", masked)
}

// IsEnabled returns true if SENTRY_DSN is configured.
func IsEnabled() bool {
	return os.Getenv("SENTRY_DSN") != ""
}

// Flush waits for buffered events to be sent (call before shutdown).
func Flush() {
	if IsEnabled() {
		sentrygo.Flush(2 * time.Second)
	}
}

// CaptureException sends an error to Sentry. No-op if disabled.
func CaptureException(err error) {
	if IsEnabled() && err != nil {
		sentrygo.CaptureException(err)
	}
}

// CaptureMessage sends a message to Sentry.
func CaptureMessage(msg string) {
	if IsEnabled() {
		sentrygo.CaptureMessage(msg)
	}
}

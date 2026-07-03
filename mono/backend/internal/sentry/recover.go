package sentry

import (
	"log/slog"

	sentrygo "github.com/getsentry/sentry-go"
)

// Recover catches panics in deferred goroutines and reports them to Sentry.
// Usage: defer sentry.Recover()
func Recover() {
	if r := recover(); r != nil {
		sentrygo.CurrentHub().Recover(r)
		slog.Error("panic recovered", "panic", r)
	}
}

// Go starts fn in a new goroutine with automatic panic recovery.
func Go(fn func()) {
	go func() {
		defer Recover()
		fn()
	}()
}

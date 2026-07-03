package sync

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"rmsmail/internal/sentry"
	"strconv"
	"strings"
	"time"
)

const (
	defaultPerHostCap       = 10
	maxPerHostCap           = 128
	imapSemAcquireTimeout   = 90 * time.Second
	tokenRefreshMaxAttempts = 3
)

func init() {
	perHostCap = defaultPerHostCap
	envVal := os.Getenv("IMAP_PER_HOST_CONN")
	if envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n >= 1 && n <= maxPerHostCap {
			perHostCap = n
		} else {
			slog.Warn("sync: invalid IMAP_PER_HOST_CONN, using default", "value", envVal, "default", defaultPerHostCap, "max", maxPerHostCap)
		}
	}
	slog.Info("sync: IMAP per-host dial concurrency cap", "cap", perHostCap, "IMAP_PER_HOST_CONN", envVal)
}

func isTokenRefreshReconnect(err error) bool {
	return err != nil && strings.Contains(err.Error(), "token refreshed, need reconnect")
}

func isFatalSyncAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no refresh token") ||
		strings.Contains(msg, "invalid_grant") ||
		strings.Contains(msg, "oauth provider not configured") ||
		strings.Contains(msg, "no password for account") ||
		strings.Contains(msg, "no authentication credentials") {
		return true
	}
	// Permanent Google/Microsoft refresh rejection — user must re-authorize.
	if strings.Contains(msg, "token refresh failed") &&
		(strings.Contains(msg, "invalid_grant") || strings.Contains(msg, "401")) {
		return true
	}
	return false
}

func (w *SyncWorker) reloadAccountCredentials(ctx context.Context, f *Fetcher) {
	if f == nil || f.Store == nil {
		return
	}
	fresh, err := f.Store.GetAccountCredentials(ctx, w.Account.ID)
	if err != nil || fresh == nil {
		return
	}
	w.Account = *fresh
}

func (w *SyncWorker) reportProvisionalSyncError(ctx context.Context, f *Fetcher, attempt, maxAttempts int, err error) {
	if f == nil || f.Store == nil || err == nil {
		return
	}
	msg := fmt.Sprintf("%s (attempt %d/%d)", err.Error(), attempt+1, maxAttempts+1)
	_ = f.Store.UpdateAccountSyncError(ctx, w.Account.ID, msg)
	sentry.CaptureException(err)
}

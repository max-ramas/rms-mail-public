package sync

import (
	"context"
	"fmt"
	"log/slog"

	"rmsmail/internal/auth"
	"rmsmail/internal/models"
)

// PriorityChecker performs a one-off, short-lived IMAP scan for an account.
// It is isolated from long-lived CheckWorker/SyncWorker goroutines and is used
// for on-demand sync triggers (e.g. user clicks an account in the UI).
type PriorityChecker struct {
	Store SyncStore
	OAuth *auth.OAuthManager
}

// NewPriorityChecker creates a new PriorityChecker.
func NewPriorityChecker(store SyncStore, oauth *auth.OAuthManager) *PriorityChecker {
	return &PriorityChecker{
		Store: store,
		OAuth: oauth,
	}
}

// CheckAccount opens a temporary IMAP connection, authenticates, and runs
// SyncAllFolders (INBOX + all other folders). It never blocks longer than
// the context deadline.
func (pc *PriorityChecker) CheckAccount(ctx context.Context, acc models.Account) error {
	// Create a temporary SyncWorker only to reuse dial/authenticate/SyncAllFolders.
	// mgrWakeUp and Manager are nil because we don't run the consumer loop.
	worker := NewSyncWorker(acc, DefaultTiming(), nil, nil)

	serverAddr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)

	c, releaseSem, err := worker.dialWithRateLimit(ctx, serverAddr, nil)
	if err != nil {
		return fmt.Errorf("priority check dial failed: %w", err)
	}
	defer func() {
		c.Logout()
		c.Close()
		if releaseSem != nil {
			releaseSem()
		}
	}()

	fetcher := &Fetcher{
		Store: pc.Store,
		OAuth: pc.OAuth,
	}

	if err := worker.authenticate(ctx, c, fetcher); err != nil {
		return fmt.Errorf("priority check auth failed: %w", err)
	}

	// Clear any stale sync error on successful connect
	if pc.Store != nil {
		_ = pc.Store.UpdateAccountSyncError(ctx, acc.ID, "")
	}

	slog.Info("PriorityCheck: starting folder scan", "account", acc.Email)
	if err := worker.SyncAllFolders(ctx, c, fetcher); err != nil {
		slog.Info("PriorityCheck: folder scan finished with errors", "account", acc.Email, "error", err)
		return fmt.Errorf("priority check sync failed: %w", err)
	}

	slog.Info("PriorityCheck: completed", "account", acc.Email)
	return nil
}

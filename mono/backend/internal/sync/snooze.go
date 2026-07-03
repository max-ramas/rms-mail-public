package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"rmsmail/internal/models"
)

// SnoozeInterval is how often the snooze worker checks for expired snoozes.
const SnoozeInterval = 1 * time.Minute

// SnoozeWorker handles unsnoozing emails by moving them back to INBOX.
type SnoozeWorker struct {
	Store SnoozeStore
	Event func(ctx context.Context, accountID, emailID string) // OnNewEmail callback
}

// SnoozeStore is the minimal interface needed by SnoozeWorker.
type SnoozeStore interface {
	GetSnoozedEmails(ctx context.Context) ([]models.Email, error)
	UnsnoozeEmail(ctx context.Context, emailID string) error
	MoveEmail(ctx context.Context, emailID string, accountID string, folderID string) error
}

// NewSnoozeWorker creates a new snooze worker.
func NewSnoozeWorker(store SnoozeStore, onNewEmail func(ctx context.Context, accountID, emailID string)) *SnoozeWorker {
	return &SnoozeWorker{
		Store: store,
		Event: onNewEmail,
	}
}

// Start begins the snooze checking loop.
func (w *SnoozeWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(SnoozeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("SnoozeWorker: stopping (context done)")
			return
		case <-ticker.C:
			w.check(ctx)
		}
	}
}

func (w *SnoozeWorker) check(ctx context.Context) {
	if w.Store == nil {
		return
	}

	emails, err := w.Store.GetSnoozedEmails(ctx)
	if err != nil {
		slog.Info(fmt.Sprintf("SnoozeWorker: failed to get snoozed emails: %v", err))
		return
	}

	for _, email := range emails {
		if email.SnoozeUntil != nil && email.SnoozeUntil.After(time.Now()) {
			continue // Not yet time to unsnooze
		}

		// Clear snooze_until in DB
		if err := w.Store.UnsnoozeEmail(ctx, email.ID); err != nil {
			slog.Info(fmt.Sprintf("SnoozeWorker: failed to clear snooze for email %s: %v", email.ID, err))
			continue
		}

		// Move back to INBOX
		if err := w.Store.MoveEmail(ctx, email.ID, email.AccountID, ""); err != nil {
			slog.Info(fmt.Sprintf("SnoozeWorker: failed to move email %s back to INBOX: %v", email.ID, err))
			continue
		}

		slog.Info(fmt.Sprintf("SnoozeWorker: unsnoozed email %s (account: %s)", email.ID[:12], email.AccountID))

		if w.Event != nil {
			w.Event(ctx, email.AccountID, email.ID)
		}
	}
}

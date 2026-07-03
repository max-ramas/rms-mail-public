package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"rmsmail/internal/mail"
	"rmsmail/internal/models"
)

// AccountSyncStore interface for the account sync worker.
type AccountSyncStore interface {
	GetAccounts(ctx context.Context) ([]models.Account, error)
	CreateAccount(ctx context.Context, email, name, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username, password, aiConfig, signature string) (*models.Account, error)
	DeleteAccount(ctx context.Context, id string) error
	UpdateAccountTimestamp(ctx context.Context, id string, field string) error
}

// AccountSync performs periodic discovery and cleanup of local mail accounts.
type AccountSync struct {
	store    AccountSyncStore
	interval time.Duration
}

// NewAccountSync creates a new account sync worker.
func NewAccountSync(store AccountSyncStore) *AccountSync {
	return &AccountSync{
		store:    store,
		interval: 1 * time.Hour,
	}
}

// Start begins the account sync loop.
func (s *AccountSync) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run once immediately
	s.sync(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sync(ctx)
		}
	}
}

func (s *AccountSync) sync(ctx context.Context) {
	discovered, err := mail.DiscoverLocalAccounts()
	if err != nil {
		slog.Info(fmt.Sprintf("[AccountSync] Discovery failed (fail-safe, skipping): %v", err))
		return
	}

	// If no discovery method configured, skip
	if discovered == nil {
		return
	}

	dbAccounts, err := s.store.GetAccounts(ctx)
	if err != nil {
		slog.Info(fmt.Sprintf("[AccountSync] Failed to get accounts: %v", err))
		return
	}

	dbMap := make(map[string]*models.Account)
	for i := range dbAccounts {
		dbMap[dbAccounts[i].Email] = &dbAccounts[i]
	}

	discoveredSet := make(map[string]bool)

	for _, d := range discovered {
		discoveredSet[d.Email] = true

		if existing, ok := dbMap[d.Email]; ok {
			// Account exists — if it was marked absent, restore it
			if existing.AbsentSince != nil {
				slog.Info(fmt.Sprintf("[AccountSync] Restoring account %s (reappeared)", d.Email))
				s.store.UpdateAccountTimestamp(ctx, existing.ID, "clear_absent")
			}
		} else {
			// New account — create from discovered info
			resolved, err := mail.Resolve(ctx, d.Email)
			if err != nil {
				slog.Info(fmt.Sprintf("[AccountSync] Failed to resolve settings for %s: %v", d.Email, err))
				continue
			}
			imapEncryption := resolved.IMAPEncryption
			if imapEncryption == "" {
				imapEncryption = "ssl"
			}
			smtpEncryption := "starttls"
			if resolved.UseSSL {
				smtpEncryption = "ssl"
			}
			_, err = s.store.CreateAccount(ctx,
				d.Email, "", "custom", resolved.IMAPHost, resolved.IMAPPort, resolved.UseSSL, imapEncryption,
				resolved.SMTPHost, resolved.SMTPPort, resolved.UseSSL, smtpEncryption,
				d.Email, d.Password, "{}", "")
			if err != nil {
				slog.Info(fmt.Sprintf("[AccountSync] Failed to create skeleton for %s: %v", d.Email, err))
			} else {
				slog.Info(fmt.Sprintf("[AccountSync] Created skeleton account for %s", d.Email))
			}
		}
	}

	// Mark accounts as absent if they no longer exist in discovery
	for _, dbAcc := range dbAccounts {
		if !discoveredSet[dbAcc.Email] {
			if dbAcc.AbsentSince == nil {
				slog.Info(fmt.Sprintf("[AccountSync] Marking %s as absent", dbAcc.Email))
				s.store.UpdateAccountTimestamp(ctx, dbAcc.ID, "set_absent")
			}
		}
	}
}

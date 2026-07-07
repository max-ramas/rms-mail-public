package sync

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"rmsmail/internal/models"
	"strings"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
)

type CheckWorker struct {
	Account  models.Account
	Manager  *Manager
	Store    SyncStore
	idleWake chan struct{} // signaled on IMAP unilateral EXISTS while in IDLE
}

func NewCheckWorker(acc models.Account, mgr *Manager) *CheckWorker {
	return &CheckWorker{
		Account:  acc,
		Manager:  mgr,
		Store:    mgr.Store,
		idleWake: make(chan struct{}, 1),
	}
}

func (w *CheckWorker) Start(ctx context.Context) {
	slog.Info("=== CheckWorker.Start() ENTERED ===", "accountID", w.Account.Email)

	consecutiveErrors := 0
	baseDelay := 15 * time.Second
	maxDelay := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := w.runSession(ctx)
		if err != nil {
			if err == context.Canceled || strings.Contains(err.Error(), "context canceled") {
				return
			}
			consecutiveErrors++
			// Exponential backoff with 30% jitter to prevent thundering herd
			backoff := baseDelay * time.Duration(1<<min(consecutiveErrors-1, 4))
			if backoff > maxDelay {
				backoff = maxDelay
			}
			jitter := time.Duration(rand.Int63n(int64(float64(backoff) * 0.3)))
			delay := backoff + jitter
			slog.Info("CheckWorker error, backing off", "accountID", w.Account.Email, "attempt", consecutiveErrors, "delay", delay.Round(time.Second), "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}
		consecutiveErrors = 0
	}
}

func (w *CheckWorker) runSession(ctx context.Context) error {
	return w.runSessionAuth(ctx, 0)
}

func (w *CheckWorker) runSessionAuth(ctx context.Context, tokenRefreshTries int) error {
	sw := NewSyncWorker(w.Account, w.Manager.Timing, nil, w.Manager)
	sw.refreshLocker = w.Manager.LockTokenRefresh
	fetcher := &Fetcher{Store: w.Store, OAuth: w.Manager.OAuth}

	serverAddr := fmt.Sprintf("%s:%d", w.Account.IMAPHost, w.Account.IMAPPort)
	idleHandler := &imapclient.UnilateralDataHandler{
		Mailbox: func(data *imapclient.UnilateralDataMailbox) {
			if data.NumMessages != nil {
				select {
				case w.idleWake <- struct{}{}:
				default:
				}
			}
		},
	}
	c, releaseSem, err := sw.dialWithRateLimit(ctx, serverAddr, idleHandler)
	if err != nil {
		return err
	}
	defer func() {
		c.Logout()
		c.Close()
		releaseSem()
	}()

	if err := sw.authenticate(ctx, c, fetcher); err != nil {
		if isFatalSyncAuthError(err) {
			if w.Store != nil {
				_ = w.Store.UpdateAccountSyncError(ctx, w.Account.ID, err.Error())
			}
		}
		return err
	}

	if w.Store != nil {
		_ = w.Store.UpdateAccountSyncError(ctx, w.Account.ID, "")
	}

	slog.Info("CheckWorker connected successfully", "accountID", w.Account.Email)

	mb, err := c.Select("INBOX", nil).Wait()
	if err != nil {
		return fmt.Errorf("SELECT INBOX failed: %w", err)
	}

	idleSupported := c.Caps().Has("IDLE")
	slog.Info("CheckWorker: INBOX selected", "accountID", w.Account.Email, "numMessages", mb.NumMessages, "idleSupported", idleSupported)

	if err := w.checkNewEmails(ctx, c); err != nil {
		return err
	}

	if idleSupported {
		return w.runIDLELoop(ctx, c)
	}
	return w.runTimerLoop(ctx, c)
}

func (w *CheckWorker) checkNewEmails(ctx context.Context, c *imapclient.Client) error {
	folders, err := w.Store.GetFolders(ctx, w.Account.ID)
	var lastUID uint32 = 0
	var inboxFolderID string
	if err == nil {
		for _, f := range folders {
			if strings.EqualFold(f.Name, "INBOX") || strings.EqualFold(f.Path, "INBOX") {
				lastUID = uint32(f.LastSyncUID)
				inboxFolderID = f.ID
				break
			}
		}
	}

	var uidSet imap.UIDSet
	if lastUID > 0 {
		uidSet.AddRange(imap.UID(lastUID+1), imap.UID((1<<32)-1))
	} else {
		uidSet.AddRange(1, imap.UID((1<<32)-1))
	}

	searchCriteria := &imap.SearchCriteria{
		UID: []imap.UIDSet{uidSet},
	}

	searchData, err := c.UIDSearch(searchCriteria, nil).Wait()
	if err != nil {
		return fmt.Errorf("UID SEARCH failed: %w", err)
	}

	var uids []uint32
	for _, uid := range searchData.AllUIDs() {
		uids = append(uids, uint32(uid))
	}

	if len(uids) > 0 {
		maxUID := uids[0]
		for _, u := range uids[1:] {
			if u > maxUID {
				maxUID = u
			}
		}

		slog.Info("CheckWorker found new UIDs in INBOX", "accountID", w.Account.Email, "count", len(uids), "maxUID", maxUID)
		if err := w.Store.EnqueueUIDs(ctx, w.Account.ID, "INBOX", uids, 10); err != nil {
			slog.Info("CheckWorker failed to enqueue UIDs", "accountID", w.Account.Email, "error", err)
		} else if inboxFolderID != "" && int(maxUID) > int(lastUID) {
			if err := w.Store.UpdateFolderLastUID(ctx, inboxFolderID, int(maxUID)); err != nil {
				slog.Info("CheckWorker failed to update last_sync_uid", "accountID", w.Account.Email, "error", err)
			}
		}

		w.Manager.WakeUpAccountNow(w.Account.ID)
	}
	return nil
}

func (w *CheckWorker) idleRefreshInterval() time.Duration {
	interval := w.Manager.Timing.IDLETimeout
	if interval <= 0 {
		interval = 2 * time.Minute
	}
	const maxRefresh = 14 * time.Minute
	if interval > maxRefresh {
		interval = maxRefresh
	}
	return interval
}

func (w *CheckWorker) idleWatchdogInterval() time.Duration {
	watchdog := w.Manager.Timing.IDLEWatchdog
	if watchdog <= 0 {
		watchdog = 3 * time.Minute
	}
	return watchdog
}

func (w *CheckWorker) runIDLELoop(ctx context.Context, c *imapclient.Client) error {
	idleCommand, err := c.Idle()
	if err != nil {
		return fmt.Errorf("IDLE command failed: %w", err)
	}

	refreshInterval := w.idleRefreshInterval()
	watchdogInterval := w.idleWatchdogInterval()

	refreshTimer := time.NewTimer(refreshInterval)
	watchdogTimer := time.NewTimer(watchdogInterval)
	defer refreshTimer.Stop()
	defer watchdogTimer.Stop()

	resetWatchdog := func() {
		if !watchdogTimer.Stop() {
			select {
			case <-watchdogTimer.C:
			default:
			}
		}
		watchdogTimer.Reset(watchdogInterval)
	}

	restartIdle := func() error {
		if err := idleCommand.Close(); err != nil {
			return err
		}
		if err := idleCommand.Wait(); err != nil {
			slog.Info("IDLE Wait returned error", "accountID", w.Account.Email, "error", err)
		}
		if err := w.checkNewEmails(ctx, c); err != nil {
			return err
		}
		idleCommand, err = c.Idle()
		if err != nil {
			return fmt.Errorf("IDLE restart failed: %w", err)
		}
		resetWatchdog()
		if !refreshTimer.Stop() {
			select {
			case <-refreshTimer.C:
			default:
			}
		}
		refreshTimer.Reset(refreshInterval)
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			idleCommand.Close()
			return ctx.Err()
		case <-refreshTimer.C:
			slog.Info("Refreshing IDLE session", "accountID", w.Account.Email, "interval", refreshInterval)
			if err := restartIdle(); err != nil {
				return err
			}
		case <-watchdogTimer.C:
			return fmt.Errorf("IDLE watchdog: no activity for %v", watchdogInterval)
		case <-w.idleWake:
			slog.Info("IDLE update received (unilateral EXISTS)", "accountID", w.Account.Email)
			if err := restartIdle(); err != nil {
				return err
			}
		}
	}
}

func (w *CheckWorker) runTimerLoop(ctx context.Context, c *imapclient.Client) error {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.Noop().Wait(); err != nil {
				return fmt.Errorf("NOOP failed: %w", err)
			}
			if err := w.checkNewEmails(ctx, c); err != nil {
				return err
			}
		}
	}
}

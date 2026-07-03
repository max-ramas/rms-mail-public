package sync

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net"
	"strings"
	stdsync "sync"
	"time"

	"rmsmail/internal/models"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"
	"github.com/emersion/go-sasl"
)

type SyncWorker struct {
	Account           models.Account
	Manager           *Manager
	consecutiveErrors int
	wakeUp            chan struct{}   // internal wake-up (syncFolderByUID)
	mgrWakeUp         <-chan struct{} // manager wake-up (WakeUpAccount)
	timing            Timing
	refreshLocker     func(accountID string) func() // per-account token refresh serialization
}

// perHostSem limits concurrent TCP dials to the same host (burst protection).
// Slots are released once the dial succeeds; open IDLE/sync sessions do not hold them.
// Cap defaults to 10 (override with IMAP_PER_HOST_CONN, max 128).
// Limits concurrent TCP dials only; IDLE/sync sessions do not hold slots.
var (
	perHostMu  stdsync.Mutex
	perHostSem = make(map[string]chan struct{})
	perHostCap int
)

func NewSyncWorker(acc models.Account, timing Timing, mgrWakeUp <-chan struct{}, mgr *Manager) *SyncWorker {
	return &SyncWorker{
		Account:   acc,
		Manager:   mgr,
		timing:    timing,
		wakeUp:    make(chan struct{}, 1),
		mgrWakeUp: mgrWakeUp, // nil is OK — consumer loop handles nil channel
	}
}

func (w *SyncWorker) deleteLocalEmailCopy(ctx context.Context, f *Fetcher, emailID string) {
	email, err := f.Store.GetEmail(ctx, emailID, w.Account.ID)
	if err == nil && email != nil {
		PurgeEmailLocalFiles(email)
	}
	if err := f.Store.DeleteEmail(ctx, emailID, w.Account.ID); err != nil {
		slog.Info("Failed to delete email for missing UID", "accountID", w.Account.Email, "emailID", emailID, "error", err)
	}
}

func (w *SyncWorker) Start(ctx context.Context, f *Fetcher) {
	slog.Info("=== SyncWorker.Start() ENTERED ===", "accountID", w.Account.Email)

	defer func() {
		if r := recover(); r != nil {
			slog.Error("Panic in SyncWorker", "email", w.Account.Email, "panic", r)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Worker stopping (context done)", "accountID", w.Account.Email)
			return
		default:
			slog.Info("Starting sync cycle...", "accountID", w.Account.Email)

			// Clear cache so we don't hold stale IDs across sync cycles
			f.ClearFolderCache(w.Account.ID)

			err := w.sync(ctx, f)

			if err != nil {
				if err == context.Canceled || strings.Contains(err.Error(), "context canceled") {
					slog.Info("Worker sync canceled gracefully.", "accountID", w.Account.Email)
					return // Just exit the worker
				}

				if isFatalSyncAuthError(err) || strings.Contains(err.Error(), "account not found in DB") {
					slog.Info("Worker: fatal sync error (will not retry)", "accountID", w.Account.Email, "error", err)
					if f.Store != nil {
						f.Store.UpdateAccountSyncError(ctx, w.Account.ID, err.Error())
					}
					return // No point retrying — re-auth or config fix required
				}

				w.consecutiveErrors++
				backoff := w.calculateBackoff()
				slog.Info("!!! SYNC ERROR !!!", "accountID", w.Account.Email, "attempt", w.consecutiveErrors, "error", err)
				slog.Info("Backing off", "accountID", w.Account.Email, "backoff", backoff)
				if f.Store != nil {
					f.Store.UpdateAccountSyncError(ctx, w.Account.ID, err.Error())
				}

				timer := time.NewTimer(backoff)
				defer timer.Stop()
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			} else {
				w.consecutiveErrors = 0
				if f.Store != nil {
					f.Store.UpdateAccountSyncError(ctx, w.Account.ID, "")
				}
			}
		}
	}
}

func (w *SyncWorker) calculateBackoff() time.Duration {
	base := w.timing.ErrorBackoffBase
	maxBackoff := w.timing.ErrorBackoffMax
	maxAttempts := 20

	if w.consecutiveErrors >= maxAttempts {
		return maxBackoff
	}

	backoff := base * time.Duration(w.consecutiveErrors)
	if backoff > maxBackoff {
		backoff = maxBackoff
	}
	if backoff <= 0 {
		backoff = time.Second
	}

	fullJitter := time.Duration(rand.Int63n(int64(backoff)))
	return backoff + fullJitter
}

func (w *SyncWorker) sync(ctx context.Context, f *Fetcher) error {
	slog.Info("=== SYNC FUNCTION CALLED ===", "accountID", w.Account.Email)

	serverAddr := fmt.Sprintf("%s:%d", w.Account.IMAPHost, w.Account.IMAPPort)

	isOAuth := w.Account.OAuthAccessToken != ""
	password := w.Account.PasswordEncrypted

	slog.Info("Account info", "accountID", w.Account.Email, "oauth", isOAuth, "hasPassword", password != "", "host", fmt.Sprintf("%s:%d", w.Account.IMAPHost, w.Account.IMAPPort))

	if !isOAuth && password == "" {
		slog.Info("No password configured for account", "accountID", w.Account.Email)
		return fmt.Errorf("no password for account %s", w.Account.Email)
	}

	slog.Info("Starting IMAP sync", "accountID", w.Account.Email, "host", serverAddr, "oauth", isOAuth)

	maxReconnects := w.timing.ReconnectRetries
	baseDelay := w.timing.ReconnectBase

	var lastErr error
	tokenRefreshTries := 0
	for attempt := 0; attempt <= maxReconnects; attempt++ {
		if attempt > 0 {
			delay := CalculateReconnectDelay(baseDelay, attempt)
			slog.Info("Reconnect attempt", "accountID", w.Account.Email, "attempt", attempt, "maxReconnects", maxReconnects, "delay", delay.Round(time.Millisecond))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			timer.Stop()
		}

		c, releaseSem, err := w.dialWithRateLimit(ctx, serverAddr, nil)
		if err != nil {
			lastErr = err
			w.reportProvisionalSyncError(ctx, f, attempt, maxReconnects, err)
			slog.Info("Dial error", "accountID", w.Account.Email, "attempt", attempt+1, "error", err)
			continue
		}
		slog.Info("IMAP dial successful", "accountID", w.Account.Email, "encryption", w.Account.IMAPEncryption)

		authErr := w.authenticate(ctx, c, f)
		if authErr != nil {
			c.Logout()
			c.Close()
			releaseSem()
			if isTokenRefreshReconnect(authErr) && tokenRefreshTries < tokenRefreshMaxAttempts {
				tokenRefreshTries++
				w.reloadAccountCredentials(ctx, f)
				slog.Info("Token refreshed, immediate reconnect", "accountID", w.Account.Email, "tokenRefreshTry", tokenRefreshTries)
				attempt--
				continue
			}
			if isFatalSyncAuthError(authErr) {
				return authErr
			}
			lastErr = authErr
			w.reportProvisionalSyncError(ctx, f, attempt, maxReconnects, authErr)
			slog.Info("Auth error", "accountID", w.Account.Email, "attempt", attempt+1, "error", authErr)
			continue
		}

		slog.Info("IMAP login successful", "accountID", w.Account.Email)
		tokenRefreshTries = 0
		// Clear error and update sync timestamp immediately on successful connect
		if f.Store != nil {
			f.Store.UpdateAccountSyncError(ctx, w.Account.ID, "")
		}
		slog.Info("Entering runSyncCycle...", "accountID", w.Account.Email)
		err = w.runSyncCycle(ctx, c, f)
		c.Logout()
		c.Close()
		releaseSem()

		if err == nil {
			return nil
		}

		lastErr = err
		w.reportProvisionalSyncError(ctx, f, attempt, maxReconnects, err)
		slog.Info("Sync cycle error", "accountID", w.Account.Email, "error", err)
		if attempt == maxReconnects {
			return fmt.Errorf("failed after %d reconnect attempts: %w", maxReconnects, err)
		}
	}

	if lastErr != nil {
		return fmt.Errorf("failed after %d reconnect attempts: %w", maxReconnects, lastErr)
	}
	return fmt.Errorf("failed after %d reconnect attempts", maxReconnects)
}

func (w *SyncWorker) authenticate(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	password := w.Account.PasswordEncrypted

	// Prefer XOAUTH2 if we have an access token
	if w.Account.OAuthAccessToken != "" {
		auth := NewXOAUTH2Client(w.Account.Username, w.Account.OAuthAccessToken)
		if err := c.Authenticate(auth); err != nil {
			slog.Info("XOAUTH2 error. Trying to refresh token...", "accountID", w.Account.Email, "error", err)

			// Пытаемся обновить токен через OAuth провайдера
			if refreshErr := w.refreshToken(ctx, f); refreshErr != nil {
				slog.Info("Failed to refresh token", "accountID", w.Account.Email, "error", refreshErr)
				return fmt.Errorf("XOAUTH2 auth failed and token refresh failed: %w", refreshErr)
			}

			slog.Info("Token refreshed successfully. Retrying connection in next attempt...", "accountID", w.Account.Email)
			return fmt.Errorf("token refreshed, need reconnect")
		}
	} else if password != "" {
		if err := c.Login(w.Account.Username, password).Wait(); err != nil {
			return fmt.Errorf("login failed: %w", err)
		}
	} else {
		return fmt.Errorf("no authentication credentials provided")
	}
	return nil
}

func (w *SyncWorker) runSyncCycle(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	slog.Info("runSyncCycle (consumerLoop) started", "accountID", w.Account.Email)

	// Discover and sync all folders on each connection cycle.
	if err := w.SyncAllFolders(ctx, c, f); err != nil {
		slog.Info("SyncAllFolders error", "accountID", w.Account.Email, "error", err)
	}

	// Update query planner statistics after a sync batch so Index Scans are
	// chosen instead of Seq Scans on recently-populated tables.
	if analyzer, ok := f.Store.(interface{ AnalyzeAfterBulk(context.Context) error }); ok {
		go func() {
			actx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := analyzer.AnalyzeAfterBulk(actx); err != nil {
				slog.Warn("ANALYZE after bulk sync failed", "account", w.Account.Email, "error", err)
			}
		}()
	}

	// Keep existing flag/move/draft syncs on startup for safety
	if err := w.syncDrafts(ctx, c, f); err != nil {
		slog.Info("Draft sync error", "accountID", w.Account.Email, "error", err)
		if f.Store != nil {
			f.Store.UpdateAccountSyncError(ctx, w.Account.ID, "draft sync: "+err.Error())
		}
	}

	if err := w.syncFlags(ctx, c, f); err != nil {
		slog.Info("Flag sync error", "accountID", w.Account.Email, "error", err)
	}

	if err := w.syncMoves(ctx, c, f); err != nil {
		slog.Info("IMAP move sync error", "accountID", w.Account.Email, "error", err)
		if f.Store != nil {
			f.Store.UpdateAccountSyncError(ctx, w.Account.ID, "imap move sync: "+err.Error())
		}
	}

	slog.Info("Entering consumer loop", "accountID", w.Account.Email)
	if f.OnActivity != nil {
		f.OnActivity(w.Account.ID)
	}
	return w.consumerLoop(ctx, c, f)
}

func (w *SyncWorker) consumerLoop(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	pollTicker := time.NewTicker(30 * time.Second)
	defer pollTicker.Stop()

	folderScanInterval := w.timing.FolderScanInterval
	if folderScanInterval <= 0 {
		folderScanInterval = 5 * time.Minute
	}
	folderScanTicker := time.NewTicker(folderScanInterval)
	defer folderScanTicker.Stop()

	for {
		// Record heartbeat activity every iteration so refreshWorkers
		// doesn't falsely flag this worker as inactive and kill it.
		if f.OnActivity != nil {
			f.OnActivity(w.Account.ID)
		}

		// Attempt to dequeue tasks across all folders
		tasks, err := f.Store.DequeueUIDs(ctx, w.Account.ID, 50) // process up to 50
		if err != nil {
			slog.Info("Dequeue error", "accountID", w.Account.Email, "error", err)
			// On SQLITE_BUSY, back off to give HTTP handlers a chance to acquire the lock.
			if strings.Contains(err.Error(), "database is locked") {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(200 * time.Millisecond):
				}
			}
		} else if len(tasks) > 0 {
			slog.Info("Dequeued tasks", "accountID", w.Account.Email, "count", len(tasks))
			// Group tasks by folder
			byFolder := make(map[string][]models.SyncTask)
			for _, t := range tasks {
				byFolder[t.FolderName] = append(byFolder[t.FolderName], t)
			}

			for folderName, folderTasks := range byFolder {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}
				w.processTasks(ctx, c, f, folderName, folderTasks)
			}
			// don't sleep if we had tasks
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pollTicker.C:
			// Ping connection to keep it alive
			if err := c.Noop().Wait(); err != nil {
				return err
			}
			w.syncInboundFlags(ctx, c, f)
			w.syncFlags(ctx, c, f)
			w.syncMoves(ctx, c, f)
			w.syncDrafts(ctx, c, f)
		case <-folderScanTicker.C:
			if err := w.syncNonInboxFolders(ctx, c, f); err != nil {
				slog.Info("Non-INBOX folder scan error", "accountID", w.Account.Email, "error", err)
			}
		case <-w.wakeUp:
			// Internal wake-up (syncFolderByUID enqueued new UIDs)
			w.syncInboundFlags(ctx, c, f)
			w.syncFlags(ctx, c, f)
			w.syncMoves(ctx, c, f)
			w.syncDrafts(ctx, c, f)
		case <-w.mgrWakeUp:
			// Manager wake-up (WakeUpAccount, e.g. from CheckWorker)
			w.syncInboundFlags(ctx, c, f)
			w.syncFlags(ctx, c, f)
			w.syncMoves(ctx, c, f)
			w.syncDrafts(ctx, c, f)
		}
	}
}

func (w *SyncWorker) processTasks(ctx context.Context, c *imapclient.Client, f *Fetcher, folder string, tasks []models.SyncTask) {
	// Select the folder
	if _, err := c.Select(folder, nil).Wait(); err != nil {
		slog.Info("Failed to select folder for consumer task", "accountID", w.Account.Email, "folder", folder, "error", err)
		for _, task := range tasks {
			f.Store.FailSyncTask(ctx, task.ID, "select failed: "+err.Error())
		}
		return
	}

	var uidSet imap.UIDSet
	var taskMap stdsync.Map
	for _, task := range tasks {
		uidSet.AddNum(imap.UID(task.UID))
		taskMap.Store(uint32(task.UID), task.ID)
	}

	fetchOptions := &imap.FetchOptions{
		Envelope: true,
		Flags:    true,
		BodySection: []*imap.FetchItemBodySection{
			{Peek: true},
		},
		UID: true,
	}

	cmd := c.Fetch(uidSet, fetchOptions)
	defer cmd.Close() // ensure cleanup even on panic

	for {
		msg := cmd.Next()
		if msg == nil {
			break
		}

		if f.OnActivity != nil {
			f.OnActivity(w.Account.ID)
		}

		// Process stream sequentially
		uid, err := f.ProcessMessageStreamToFolder(ctx, w.Account.ID, folder, msg)
		if uid > 0 {
			val, ok := taskMap.Load(uid)
			if ok {
				taskID := val.(int64)
				if err != nil {
					f.Store.FailSyncTask(ctx, taskID, err.Error())
				} else {
					f.Store.CompleteSyncTask(ctx, taskID)
				}
				taskMap.Delete(uid) // Mark as processed
			}
		}
	}

	if err := cmd.Close(); err != nil {
		slog.Info("UIDFetch close error", "accountID", w.Account.Email, "error", err)
	}

	// Batch-complete remaining tasks not returned by IMAP (deleted on server).
	// Single log line + batched DB updates instead of one-by-one.
	var missingUIDs []uint32
	var missingTaskIDs []int64
	taskMap.Range(func(key, value any) bool {
		missingUIDs = append(missingUIDs, key.(uint32))
		missingTaskIDs = append(missingTaskIDs, value.(int64))
		return true
	})
	if len(missingUIDs) > 0 {
		slog.Debug(fmt.Sprintf("[%s] %d UIDs not found on server, purging local copies", w.Account.Email, len(missingUIDs)))
		for _, uid := range missingUIDs {
			emailID, err := f.Store.GetEmailIDByFolderUID(ctx, w.Account.ID, folder, uid)
			if err != nil {
				slog.Info("Failed to resolve email for missing UID", "accountID", w.Account.Email, "folder", folder, "uid", uid, "error", err)
				continue
			}
			if emailID != "" {
				w.deleteLocalEmailCopy(ctx, f, emailID)
			}
		}
		f.Store.CompleteSyncTasks(ctx, missingTaskIDs)
	}
}

func (w *SyncWorker) syncNonInboxFolders(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	folders, err := w.ListFolders(ctx, c, f)
	if err != nil {
		return err
	}
	for _, folder := range folders {
		if strings.EqualFold(folder.Path, "INBOX") {
			continue
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if err := w.syncFolder(ctx, c, f, folder); err != nil {
			slog.Info("Incremental folder scan failed", "accountID", w.Account.Email, "folder", folder.Path, "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(syncBatchDelay):
		}
	}
	return nil
}

func (w *SyncWorker) dialWithRateLimit(ctx context.Context, addr string, unilateral *imapclient.UnilateralDataHandler) (*imapclient.Client, func(), error) {
	slog.Info("Trying to dial IMAP server", "accountID", w.Account.Email, "address", addr)
	w.applyJitter(100 * time.Millisecond)

	// Per-host dial limit: acquire a slot only while opening TCP/TLS to the host.
	host, _, _ := net.SplitHostPort(addr)
	if host == "" {
		host = addr
	}
	perHostMu.Lock()
	sem, ok := perHostSem[host]
	if !ok {
		sem = make(chan struct{}, perHostCap)
		perHostSem[host] = sem
	}
	perHostMu.Unlock()
	select {
	case sem <- struct{}{}:
	case <-ctx.Done():
		return nil, nil, ctx.Err()
	case <-time.After(imapSemAcquireTimeout):
		return nil, nil, fmt.Errorf("timed out waiting for IMAP dial slot to %s (cap %d, set IMAP_PER_HOST_CONN)", host, perHostCap)
	}
	releaseDialSlot := func() { <-sem }

	encryption := w.Account.IMAPEncryption
	if encryption == "" {
		if w.Account.IMAPSSL {
			encryption = "ssl"
		} else {
			encryption = "none"
		}
	}

	opts := &imapclient.Options{
		Dialer: &net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		},
		TLSConfig: &tls.Config{
			InsecureSkipVerify: false,
		},
	}
	if unilateral != nil {
		opts.UnilateralDataHandler = unilateral
	}

	var client *imapclient.Client
	var dialErr error
	switch strings.ToLower(encryption) {
	case "ssl", "tls", "ssl/tls":
		client, dialErr = imapclient.DialTLS(addr, opts)
	case "starttls":
		client, dialErr = imapclient.DialStartTLS(addr, opts)
	default:
		client, dialErr = imapclient.DialInsecure(addr, opts)
	}
	if dialErr != nil {
		releaseDialSlot()
		return nil, nil, dialErr
	}
	releaseDialSlot()
	return client, func() {}, nil
}

func (w *SyncWorker) applyJitter(maxDelay time.Duration) {
	delay := time.Duration(rand.Intn(int(maxDelay)))
	time.Sleep(delay)
}

func (w *SyncWorker) syncDrafts(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	drafts, err := f.Store.GetDirtyDrafts(ctx, w.Account.ID)
	if err != nil || len(drafts) == 0 {
		return err
	}

	draftsFolder := resolveDraftsFolder(c)

	// Rate limiter: max 1 APPEND per DraftThrottle per account
	if w.timing.DraftThrottle <= 0 {
		w.timing.DraftThrottle = 3 * time.Second
	}
	rateLimiter := time.NewTicker(w.timing.DraftThrottle)
	defer rateLimiter.Stop()

	for _, e := range drafts {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if e.DraftReply == "" || e.DraftReply == "[AI draft pending]" {
			slog.Info("Skipping draft: empty or placeholder", "accountID", w.Account.Email, "emailID", e.ID)
			continue
		}

		// Remove old draft version with retry
		if e.DraftRemoteUID > 0 {
			if err := retryWithBackoff(ctx, 3, 500*time.Millisecond, 4*time.Second, func() error {
				return w.deleteOldDraft(ctx, c, draftsFolder, int(e.DraftRemoteUID))
			}); err != nil {
				slog.Info("Failed to delete old draft after retries", "accountID", w.Account.Email, "remoteUID", e.DraftRemoteUID, "error", err)
			}
		}

		// Wait for rate limiter between APPENDs
		select {
		case <-rateLimiter.C:
		case <-ctx.Done():
			return ctx.Err()
		}

		// APPEND new draft version with retry (max 3 attempts)
		var appendUID uint32
		err := retryWithBackoff(ctx, 3, 2*time.Second, 10*time.Second, func() error {
			draftBody := fmt.Sprintf("Subject: %s (draft)\nMIME-Version: 1.0\nContent-Type: text/plain; charset=utf-8\n\n%s",
				e.Subject, e.DraftReply)

			appendCmd := c.Append(draftsFolder, int64(len(draftBody)), nil)
			if _, werr := appendCmd.Write([]byte(draftBody)); werr != nil {
				return fmt.Errorf("APPEND write: %w", werr)
			}
			if cerr := appendCmd.Close(); cerr != nil {
				return fmt.Errorf("APPEND close: %w", cerr)
			}
			appendData, werr := appendCmd.Wait()
			if werr != nil {
				return fmt.Errorf("APPEND wait: %w", werr)
			}
			if appendData != nil {
				appendUID = uint32(appendData.UID)
			}
			return nil
		})

		if err != nil {
			slog.Info("Draft APPEND failed after retries", "accountID", w.Account.Email, "emailID", e.ID, "error", err)
			continue
		}

		if appendUID > 0 {
			f.Store.SetDraftRemoteUID(ctx, e.ID, w.Account.ID, int(appendUID))
		} else {
			f.Store.SetDraftRemoteUID(ctx, e.ID, w.Account.ID, 0)
		}

		slog.Info("Draft synced", "accountID", w.Account.Email, "emailID", e.ID, "uid", appendUID)
	}
	return nil
}

// syncInboundFlags pulls IMAP \\Seen, \\Flagged, and \\Answered for recent non-dirty messages
// so flag changes from other clients propagate into RMS Mail (both directions).
func (w *SyncWorker) syncInboundFlags(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	candidates, err := f.Store.GetEmailsForInboundFlagSync(ctx, w.Account.ID, inboundFlagSyncLimit)
	if err != nil {
		return err
	}
	if w.Manager != nil {
		for _, id := range w.Manager.drainFlagRefresh(w.Account.ID) {
			email, getErr := f.Store.GetEmail(ctx, id, w.Account.ID)
			if getErr != nil || email == nil || email.UID == 0 || email.IsDirtyLocally {
				continue
			}
			candidates = mergeFlagSyncCandidates(candidates, []models.Email{*email})
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	folders, err := f.Store.GetFolders(ctx, w.Account.ID)
	if err != nil {
		return err
	}
	folderMap := make(map[string]string)
	for _, fld := range folders {
		folderMap[fld.ID] = fld.Path
	}

	type emailRef struct {
		id         string
		isRead     bool
		isFlagged  bool
		isAnswered bool
	}
	type folderBatch struct {
		uids    imap.UIDSet
		idByUID map[uint32]emailRef
	}
	byFolder := make(map[string]*folderBatch)

	for _, e := range candidates {
		path, ok := folderMap[e.FolderID]
		if !ok || path == "" || e.UID == 0 {
			continue
		}
		if byFolder[path] == nil {
			byFolder[path] = &folderBatch{idByUID: make(map[uint32]emailRef)}
		}
		batch := byFolder[path]
		batch.uids.AddNum(imap.UID(e.UID))
		batch.idByUID[uint32(e.UID)] = emailRef{
			id:         e.ID,
			isRead:     e.IsRead,
			isFlagged:  e.IsFlagged,
			isAnswered: e.IsAnswered,
		}
	}

	fetchOpts := &imap.FetchOptions{Flags: true, UID: true}

	for folderPath, batch := range byFolder {
		if len(batch.idByUID) == 0 {
			continue
		}
		if _, err := c.Select(folderPath, nil).Wait(); err != nil {
			slog.Info("inbound flag sync: select failed", "accountID", w.Account.Email, "folder", folderPath, "error", err)
			continue
		}
		cmd := c.Fetch(batch.uids, fetchOpts)
		for {
			msg := cmd.Next()
			if msg == nil {
				break
			}
			var uid uint32
			var flags []imap.Flag
			for {
				item := msg.Next()
				if item == nil {
					break
				}
				switch v := item.(type) {
				case imapclient.FetchItemDataUID:
					uid = uint32(v.UID)
				case imapclient.FetchItemDataFlags:
					flags = v.Flags
				}
			}
			ref, ok := batch.idByUID[uid]
			if !ok {
				continue
			}
			serverRead, serverFlagged, serverAnswered := imapFlagsState(flags)
			if ref.isRead == serverRead && ref.isFlagged == serverFlagged && ref.isAnswered == serverAnswered {
				continue
			}
			changed, err := f.Store.ApplyServerEmailFlags(ctx, ref.id, w.Account.ID, serverRead, serverFlagged, serverAnswered)
			if err != nil {
				slog.Info("inbound flag sync: apply failed", "emailID", ref.id, "error", err)
				continue
			}
			if changed {
				w.broadcastEmailFlagsUpdated(ctx, f, ref.id, serverRead, serverFlagged, serverAnswered)
			}
		}
		if err := cmd.Close(); err != nil {
			slog.Info("inbound flag sync: fetch close", "accountID", w.Account.Email, "error", err)
		}
	}
	return nil
}

func imapFlagsState(flags []imap.Flag) (isRead, isFlagged, isAnswered bool) {
	for _, flag := range flags {
		switch flag {
		case imap.FlagSeen:
			isRead = true
		case imap.FlagFlagged:
			isFlagged = true
		case imap.FlagAnswered:
			isAnswered = true
		}
	}
	return isRead, isFlagged, isAnswered
}

func (w *SyncWorker) broadcastEmailFlagsUpdated(ctx context.Context, f *Fetcher, emailID string, isRead, isFlagged, isAnswered bool) {
	if f.BroadcastEvent == nil {
		return
	}
	payload, err := json.Marshal(map[string]any{
		"email_id":    emailID,
		"account_id":  w.Account.ID,
		"is_read":     isRead,
		"is_flagged":  isFlagged,
		"is_answered": isAnswered,
	})
	if err != nil {
		return
	}
	f.BroadcastEvent(ctx, "email_updated", string(payload))
}

func (w *SyncWorker) syncFlags(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	dirty, err := f.Store.GetDirtyEmails(ctx, w.Account.ID)
	if err != nil || len(dirty) == 0 {
		return err
	}

	folders, err := f.Store.GetFolders(ctx, w.Account.ID)
	if err != nil {
		return err
	}
	folderMap := make(map[string]string)
	for _, fld := range folders {
		folderMap[fld.ID] = fld.Path
	}

	type folderFlags struct {
		toMarkRead       []uint32
		toMarkUnread     []uint32
		toMarkFlagged    []uint32
		toMarkUnflagged  []uint32
		toMarkAnswered   []uint32
		toMarkUnanswered []uint32
		emails           []models.Email
	}
	byFolder := make(map[string]*folderFlags)

	for _, e := range dirty {
		if e.FolderID == "" || e.UID == 0 {
			continue
		}
		if _, ok := byFolder[e.FolderID]; !ok {
			byFolder[e.FolderID] = &folderFlags{}
		}
		byFolder[e.FolderID].emails = append(byFolder[e.FolderID].emails, e)
		if e.IsRead {
			byFolder[e.FolderID].toMarkRead = append(byFolder[e.FolderID].toMarkRead, uint32(e.UID))
		} else {
			byFolder[e.FolderID].toMarkUnread = append(byFolder[e.FolderID].toMarkUnread, uint32(e.UID))
		}
		if e.IsFlagged {
			byFolder[e.FolderID].toMarkFlagged = append(byFolder[e.FolderID].toMarkFlagged, uint32(e.UID))
		} else {
			byFolder[e.FolderID].toMarkUnflagged = append(byFolder[e.FolderID].toMarkUnflagged, uint32(e.UID))
		}
		if e.IsAnswered {
			byFolder[e.FolderID].toMarkAnswered = append(byFolder[e.FolderID].toMarkAnswered, uint32(e.UID))
		} else {
			byFolder[e.FolderID].toMarkUnanswered = append(byFolder[e.FolderID].toMarkUnanswered, uint32(e.UID))
		}
	}

	const batchSize = 200

	storeFlagBatch := func(uids []uint32, flag imap.Flag, add bool) error {
		for i := 0; i < len(uids); i += batchSize {
			end := i + batchSize
			if end > len(uids) {
				end = len(uids)
			}
			chunk := uids[i:end]
			set := make(imap.UIDSet, 0, len(chunk))
			for _, u := range chunk {
				set.AddNum(imap.UID(u))
			}

			op := imap.StoreFlagsAdd
			if !add {
				op = imap.StoreFlagsDel
			}
			flags := &imap.StoreFlags{Op: op, Flags: []imap.Flag{flag}}

			if _, err := c.Store(set, flags, nil).Collect(); err != nil {
				return fmt.Errorf("STORE FLAGS %s at uid %d: %w", flag, chunk[0], err)
			}
		}
		return nil
	}

	for fID, fg := range byFolder {
		remoteName, ok := folderMap[fID]
		if !ok || remoteName == "" {
			continue
		}

		if _, err := c.Select(remoteName, nil).Wait(); err != nil {
			slog.Info("Failed to select folder for flag sync", "accountID", w.Account.Email, "folder", remoteName, "error", err)
			continue
		}

		folderOK := true

		if len(fg.toMarkRead) > 0 {
			slog.Info("Syncing read flags to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkRead), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkRead, imap.FlagSeen, true); err != nil {
				slog.Info("markReadBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}
		if len(fg.toMarkUnread) > 0 {
			slog.Info("Syncing unread flags to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkUnread), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkUnread, imap.FlagSeen, false); err != nil {
				slog.Info("markUnreadBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}
		if len(fg.toMarkFlagged) > 0 {
			slog.Info("Syncing flagged to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkFlagged), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkFlagged, imap.FlagFlagged, true); err != nil {
				slog.Info("markFlaggedBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}
		if len(fg.toMarkUnflagged) > 0 {
			slog.Info("Syncing unflagged to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkUnflagged), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkUnflagged, imap.FlagFlagged, false); err != nil {
				slog.Info("markUnflaggedBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}
		if len(fg.toMarkAnswered) > 0 {
			slog.Info("Syncing answered to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkAnswered), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkAnswered, imap.FlagAnswered, true); err != nil {
				slog.Info("markAnsweredBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}
		if len(fg.toMarkUnanswered) > 0 {
			slog.Info("Syncing unanswered to IMAP", "accountID", w.Account.Email, "count", len(fg.toMarkUnanswered), "folder", remoteName)
			if err := storeFlagBatch(fg.toMarkUnanswered, imap.FlagAnswered, false); err != nil {
				slog.Info("markUnansweredBatch error", "accountID", w.Account.Email, "folder", remoteName, "error", err)
				folderOK = false
			}
		}

		if !folderOK {
			slog.Info("Skipping dirty flag clear after STORE errors", "accountID", w.Account.Email, "folder", remoteName)
			continue
		}

		for _, e := range fg.emails {
			if err := f.Store.ClearDirtyFlag(ctx, e.ID); err != nil {
				slog.Info("Failed to clear dirty flag", "accountID", w.Account.Email, "emailID", e.ID, "error", err)
			}
		}
	}

	return nil
}

// deleteOldDraft selects the drafts folder and marks a remote draft as deleted and expunges it, with retry.
func (w *SyncWorker) deleteOldDraft(ctx context.Context, c *imapclient.Client, draftsFolder string, remoteUID int) error {
	if _, err := c.Select(draftsFolder, nil).Wait(); err != nil {
		return fmt.Errorf("SELECT %s: %w", draftsFolder, err)
	}
	uidSet := imap.UIDSetNum(imap.UID(remoteUID))
	flags := &imap.StoreFlags{Op: imap.StoreFlagsAdd, Flags: []imap.Flag{imap.FlagDeleted}}

	// Store +FLAGS (\Deleted) — this flags the old version for deletion
	if _, err := c.Store(uidSet, flags, nil).Collect(); err != nil {
		return fmt.Errorf("STORE +FLAGS: %w", err)
	}

	// UIDExpunge removes only the flagged UID (UIDPLUS capability)
	if _, err := c.UIDExpunge(uidSet).Collect(); err != nil {
		return fmt.Errorf("UIDExpunge: %w", err)
	}

	return nil
}

// retryWithBackoff retries a function up to maxAttempts with exponential backoff + jitter.
func retryWithBackoff(ctx context.Context, maxAttempts int, baseDelay, maxDelay time.Duration, fn func() error) error {
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := fn(); err != nil {
			lastErr = err
			if attempt == maxAttempts {
				break
			}
			// Exponential backoff with jitter
			backoff := baseDelay * time.Duration(1<<uint(attempt-1))
			if backoff > maxDelay {
				backoff = maxDelay
			}
			jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
			wait := backoff + jitter
			slog.Info("Retry", "wait", wait, "attempt", attempt, "maxAttempts", maxAttempts, "error", err)
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		} else {
			return nil
		}
	}
	return fmt.Errorf("all %d attempts failed: %w", maxAttempts, lastErr)
}

// syncMoves processes pending IMAP UID MOVE operations.
// It executes UID MOVE on the IMAP server and removes completed items from the queue.
func (w *SyncWorker) syncMoves(ctx context.Context, c *imapclient.Client, f *Fetcher) error {
	moves, err := f.Store.GetPendingIMAPMoves(ctx, w.Account.ID)
	if err != nil || len(moves) == 0 {
		return err
	}

	slog.Info("Processing pending IMAP moves", "accountID", w.Account.Email, "count", len(moves))

	// Filter valid moves and group by (source_folder, target_folder).
	// This way we do one SELECT + one batched MOVE per group, not N round-trips.
	type groupKey struct {
		source string
		target string
	}
	groups := make(map[groupKey][]models.IMAPMove)
	for _, move := range moves {
		if move.RemoteUID == 0 {
			slog.Info("Skipping move: no remote UID", "accountID", w.Account.Email, "moveID", move.ID)
			f.Store.FailIMAPMove(ctx, move.ID, "no remote UID")
			continue
		}
		if move.SourceFolderName == "" {
			move.SourceFolderName = "INBOX"
		}
		// Verify we can resolve the target folder name
		if move.TargetFolderID != "" {
			tf, fErr := f.Store.GetFolderByID(ctx, move.TargetFolderID)
			if fErr == nil && tf != nil && tf.Name != "" {
				move.TargetFolderName = tf.Name
			}
		}
		if move.TargetFolderName == "" {
			f.Store.FailIMAPMove(ctx, move.ID, "target folder not found")
			continue
		}
		key := groupKey{source: move.SourceFolderName, target: move.TargetFolderName}
		groups[key] = append(groups[key], move)
	}

	for key, group := range groups {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// SELECT source folder once per group
		if _, selErr := c.Select(key.source, nil).Wait(); selErr != nil {
			slog.Info("IMAP move: cannot SELECT source folder", "accountID", w.Account.Email, "source", key.source, "error", selErr)
			for _, move := range group {
				f.Store.FailIMAPMove(ctx, move.ID, "SELECT source failed: "+selErr.Error())
			}
			continue
		}

		// Batch MOVE with retry (individual UIDs, single SELECT per group)
		err := retryWithBackoff(ctx, 3, 2*time.Second, 10*time.Second, func() error {
			for _, move := range group {
				singleSet := imap.UIDSetNum(imap.UID(move.RemoteUID))
				if _, mErr := c.Move(singleSet, key.target).Wait(); mErr != nil {
					errStr := mErr.Error()
					if strings.Contains(errStr, "NONEXISTENT") || strings.Contains(errStr, "not found") {
						f.Store.CompleteIMAPMove(ctx, move.ID)
						slog.Info("IMAP move: already moved (NONEXISTENT)", "accountID", w.Account.Email, "moveID", move.ID)
						continue
					}
					return fmt.Errorf("UID MOVE %s→%s (uid %d): %w", key.source, key.target, move.RemoteUID, mErr)
				}
				f.Store.CompleteIMAPMove(ctx, move.ID)
				slog.Info("IMAP move complete", "accountID", w.Account.Email, "emailID", move.EmailID, "uid", move.RemoteUID, "from", key.source, "to", key.target)
			}
			return nil
		})

		if err != nil {
			slog.Info("IMAP batch move failed", "accountID", w.Account.Email, "source", key.source, "target", key.target, "error", err)
		}
	}

	return nil
}

func (w *SyncWorker) refreshToken(ctx context.Context, f *Fetcher) error {
	if f.OAuth == nil {
		return fmt.Errorf("OAuth providers not configured in Fetcher")
	}

	// Serialize token refresh per account: only one goroutine refreshes, others wait.
	if w.refreshLocker != nil {
		unlock := w.refreshLocker(w.Account.ID)
		defer unlock()
	}

	acc, err := f.Store.GetAccountCredentials(ctx, w.Account.ID)
	if err != nil {
		slog.Info("refreshToken: GetAccountCredentials error", "accountID", w.Account.Email, "error", err)
		return err
	}
	if acc == nil {
		slog.Info("refreshToken: account not found in DB, skipping reconnect", "accountID", w.Account.Email, "id", w.Account.ID)
		return fmt.Errorf("account not found in DB during token refresh")
	}

	if acc.OAuthRefreshToken == "" {
		slog.Info("refreshToken: no refresh token stored, skipping reconnect", "accountID", acc.Email)
		return fmt.Errorf("no refresh token for account %s", acc.Email)
	}

	slog.Info("Refreshing OAuth token", "accountID", acc.Email, "refreshTokenLen", len(acc.OAuthRefreshToken))

	var newAccessToken, newRefreshToken string
	switch acc.Provider {
	case "google":
		google, errProv := f.OAuth.GetGoogleProvider(ctx)
		if errProv != nil || google == nil {
			return fmt.Errorf("Google OAuth provider not configured")
		}
		tokens, err := google.RefreshTokens(ctx, acc.OAuthRefreshToken)
		if err != nil {
			return fmt.Errorf("Google token refresh failed: %w", err)
		}
		newAccessToken = tokens.AccessToken
		if tokens.RefreshToken != "" {
			newRefreshToken = tokens.RefreshToken
		} else {
			newRefreshToken = acc.OAuthRefreshToken
		}
	case "microsoft":
		microsoft, errProv := f.OAuth.GetMicrosoftProvider(ctx)
		if errProv != nil || microsoft == nil {
			return fmt.Errorf("Microsoft OAuth provider not configured")
		}
		tokens, err := microsoft.RefreshTokens(ctx, acc.OAuthRefreshToken)
		if err != nil {
			return fmt.Errorf("Microsoft token refresh failed: %w", err)
		}
		newAccessToken = tokens.AccessToken
		if tokens.RefreshToken != "" {
			newRefreshToken = tokens.RefreshToken
		} else {
			newRefreshToken = acc.OAuthRefreshToken
		}
	default:
		return fmt.Errorf("unsupported OAuth provider %s", acc.Provider)
	}

	err = f.Store.UpdateAccountTokens(ctx, acc.ID, newAccessToken, newRefreshToken)
	if err != nil {
		return fmt.Errorf("failed to save refreshed tokens to DB: %w", err)
	}

	// Обновляем локальное состояние аккаунта в воркере
	w.Account.OAuthAccessToken = newAccessToken
	w.Account.OAuthRefreshToken = newRefreshToken
	return nil
}

type xoauth2Client struct {
	Username string
	Token    string
}

func NewXOAUTH2Client(username, token string) sasl.Client {
	return &xoauth2Client{
		Username: username,
		Token:    token,
	}
}

func (c *xoauth2Client) Start() (string, []byte, error) {
	ir := []byte(fmt.Sprintf("user=%s\x01auth=Bearer %s\x01\x01", c.Username, c.Token))
	return "XOAUTH2", ir, nil
}

func (c *xoauth2Client) Next(challenge []byte) ([]byte, error) {
	return nil, nil
}

package sync

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	"rmsmail/internal/auth"
	"rmsmail/internal/models"
	"rmsmail/internal/notification"

)

// SyncStore is the store interface needed by sync Manager and workers.
type SyncStore interface {
	GetAccounts(ctx context.Context) ([]models.Account, error)
	GetAccount(ctx context.Context, id string) (*models.Account, error)
	GetAccountCredentials(ctx context.Context, id string) (*models.Account, error)
	GetAccountCredentialsByEmail(ctx context.Context, email string) (*models.Account, error)
	UpdateAccountTokens(ctx context.Context, id string, accessToken, refreshToken string) error
	UpdateAccountOAuth(ctx context.Context, id, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username string) error
	UpdateAccountSyncError(ctx context.Context, id string, errText string) error
	UpdateAccountIsGmail(ctx context.Context, id string, isGmail bool) error
	UpdateAccountLastUID(ctx context.Context, id string, lastUID uint32) error
	UpdateAccountUIDValidity(ctx context.Context, id string, uidValidity uint32) error
	ResetAccountSync(ctx context.Context, accountID string) error

	GetFolders(ctx context.Context, accountID string) ([]models.Folder, error)
	CreateFolder(ctx context.Context, accountID, name, path string, subscribed bool) (*models.Folder, error)
	GetFolderByID(ctx context.Context, id string) (*models.Folder, error)
	UpdateFolderLastUID(ctx context.Context, folderID string, lastUID int) error
	UpdateFolderUIDValidity(ctx context.Context, folderID string, uidValidity uint32) error

	GetEmail(ctx context.Context, id string, accountID string) (*models.Email, error)
	GetEmailByMsgIDAccount(ctx context.Context, msgID, accountID string) (*models.Email, error)
	EmailExistsByMsgID(ctx context.Context, accountID, msgID string) (bool, error)
	GetEmailIDByFolderUID(ctx context.Context, accountID, folderPath string, uid uint32) (string, error)
	SaveEmail(ctx context.Context, email models.Email) error
	SaveEmailToFolder(ctx context.Context, email models.Email, folderID string) (bool, error)
	UpdateEmailHasAttachments(ctx context.Context, emailID string, accountID string, has bool) error
	GetDirtyDrafts(ctx context.Context, accountID string) ([]models.Email, error)
	GetDirtyEmails(ctx context.Context, accountID string) ([]models.Email, error)
	GetEmailsForInboundFlagSync(ctx context.Context, accountID string, limit int) ([]models.Email, error)
	ApplyServerEmailFlags(ctx context.Context, emailID, accountID string, isRead, isFlagged, isAnswered bool) (bool, error)
	ClearDirtyFlag(ctx context.Context, emailID string) error
	SetDraftRemoteUID(ctx context.Context, emailID string, accountID string, uid int) error
	DeleteEmail(ctx context.Context, id string, accountID string) error
	DeleteEmailsInFolder(ctx context.Context, folderID string) error
	MoveEmail(ctx context.Context, emailID string, accountID string, folderID string) error
	SaveDraftReply(ctx context.Context, emailID string, accountID string, draftReply string) error
	ClearDraftReply(ctx context.Context, emailID string, accountID string) error
	GetEmailsByThreadID(ctx context.Context, threadID, accountID string, limit int) ([]models.Email, error)

	IndexEmailFTS(ctx context.Context, emailID, accountID, subject, senderName, senderAddress, recipientAddress, body string) error

	SaveAttachment(ctx context.Context, att *models.Attachment) error
	GetAttachmentByHash(ctx context.Context, hash string) (*models.Attachment, error)
	GetEmailAttachments(ctx context.Context, emailID string, accountID string) ([]models.Attachment, error)

	AddEmailTag(ctx context.Context, emailID string, accountID string, tag string) error
	GetEmailTags(ctx context.Context, emailID string, accountID string) ([]string, error)
	UpsertEmailLabels(ctx context.Context, emailID, accountID string, labels []string) error
	GetGmailLabels(ctx context.Context, emailID, accountID string) ([]string, error)
	CleanupGmailDuplicates(ctx context.Context, accountID string) (int, error)
	BackfillGmailLabels(ctx context.Context, accountID string) error
	GetSystemSetting(ctx context.Context, key string) (string, error)
	SetSystemSetting(ctx context.Context, key, value string) error
	GetActiveRules(ctx context.Context, accountID string) ([]models.FilterRule, error)

	SenderProfileValid(ctx context.Context, email string) bool
	UpsertSenderProfile(ctx context.Context, email, name, avatarURL string) error

	EnqueueIMAPMove(ctx context.Context, emailID, accountID, targetFolderID, targetFolderName, sourceFolderName string, remoteUID int32) error
	GetPendingIMAPMoves(ctx context.Context, accountID string) ([]models.IMAPMove, error)
	CompleteIMAPMove(ctx context.Context, moveID string) error
	FailIMAPMove(ctx context.Context, moveID string, errMsg string) error

	GetTelegramSettings(ctx context.Context, email string) (userID int64, enabled bool, aiNotifications bool, aiChat bool, botToken string, err error)
	GetAnyTelegramSettings(ctx context.Context) (userID int64, enabled bool, aiNotifications bool, aiChat bool, botToken string, err error)
	GetAISettings(ctx context.Context, accountID string) (*models.AISetting, error)

	EnqueueJob(ctx context.Context, jobType string, payload string, runAt time.Time) error

	GetWebhook(ctx context.Context, id string) (*models.Webhook, error)
	EnqueueWebhookRetry(ctx context.Context, id, url, secret string, payload []byte, nextRetryAtUnix int64) error
	GetDueWebhookRetries(ctx context.Context, nowUnix int64, limit int) ([]models.WebhookRetry, error)
	DeleteWebhookRetry(ctx context.Context, id string) error
	UpdateWebhookRetryAttempt(ctx context.Context, id string, attempt int, nextRetryAtUnix int64) error

	// Email Sync Queue Methods
	EnqueueUIDs(ctx context.Context, accountID, folderName string, uids []uint32, priority int) error
	DequeueUIDs(ctx context.Context, accountID string, limit int) ([]models.SyncTask, error)
	CompleteSyncTask(ctx context.Context, taskID int64) error
	CompleteSyncTasks(ctx context.Context, taskIDs []int64) error
	FailSyncTask(ctx context.Context, taskID int64, errReason string) error
	RemoveSyncTaskByUID(ctx context.Context, accountID, folderName string, uid uint32) error
	ClearFolderQueue(ctx context.Context, accountID, folderName string) error
	ProcessQueueRetries(ctx context.Context) (int64, error)
	CleanQueueGarbage(ctx context.Context, retentionCompleted time.Duration, retentionFailed time.Duration) (int64, error)

	// Folder unread counts (recalculated periodically, not per-request)
	RefreshUnreadCounts(ctx context.Context) error
}

type Manager struct {
	Store           SyncStore
	CAS             CASStore
	AI              AIProvider
	OAuth           *auth.OAuthManager
	Timing          Timing
	mu              sync.Mutex
	workers         map[string]context.CancelFunc
	checkWorkers    map[string]context.CancelFunc
	workerStartedAt map[string]time.Time     // accountID → when the worker goroutine was spawned
	workerWakeUp    map[string]chan struct{} // per-account wakeUp signal for consumer loop
	maxWorkers      int
	refreshCh       chan struct{}
	JobNotify       chan struct{} // Used to wake up the JobWorker instantly
	OnNewEmail      func(ctx context.Context, accountID, emailID, subject, senderName, senderAddr string)
	notifier        *notification.RateLimiter
	notifProvider   notification.Provider
	lastRefresh     time.Time
	BroadcastEvent  func(ctx context.Context, channel, message string) // set by server init to publish SSE/Redis events
	lastActivity    map[string]time.Time                               // accountID → last successful IDLE wakeup
	// Redis client for persistent webhook retry queue
	LockChecker        func(ctx context.Context, index int) bool // live lock computation (nil = no locking)
	IsPaused           func(accountID string) bool               // nil = no pause support
	tokenRefreshMu     sync.Mutex                                // guards tokenRefreshLocks map
	tokenRefreshLocks  map[string]*sync.Mutex                    // per-account mutex to serialize token refresh
	flagRefreshMu      sync.Mutex                                // guards flagRefreshPending
	flagRefreshPending map[string]map[string]struct{}            // accountID → emailIDs awaiting inbound flag sync
	wg                 sync.WaitGroup                            // tracks running worker goroutines for graceful shutdown
	// Asynq client for fire-and-forget tasks
}

const defaultMaxWorkers = 50

// ManagerOption is a functional option for configuring a Manager.
type ManagerOption func(*Manager)

// WithAIGateway sets the AI gateway for the manager.
func WithAIGateway(gw AIProvider) ManagerOption {
	return func(m *Manager) {
		m.AI = gw
	}
}

func NewManager(store SyncStore, cas CASStore, opts ...ManagerOption) *Manager {
	maxW := defaultMaxWorkers
	if envMax := os.Getenv("SYNC_MAX_WORKERS"); envMax != "" {
		if n, e := strconv.Atoi(envMax); e == nil && n > 0 {
			maxW = n
		}
	}
	m := &Manager{
		Store:              store,
		CAS:                cas,
		Timing:             DefaultTiming(),
		workers:            make(map[string]context.CancelFunc),
		checkWorkers:       make(map[string]context.CancelFunc),
		workerStartedAt:    make(map[string]time.Time),
		workerWakeUp:       make(map[string]chan struct{}),
		maxWorkers:         maxW,
		refreshCh:          make(chan struct{}, 1),
		notifier:           notification.NewRateLimiter(30),
		notifProvider:      notification.NewTelegramProvider(),
		lastActivity:       make(map[string]time.Time),
		JobNotify:          make(chan struct{}, 1),
		tokenRefreshLocks:  make(map[string]*sync.Mutex),
		flagRefreshPending: make(map[string]map[string]struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

func (m *Manager) signalWorker(accountID string, immediate bool) {
	if m.IsPaused != nil && m.IsPaused(accountID) {
		slog.Debug("Skip worker wake-up: account is paused", "account_id", accountID)
		return
	}
	send := func() {
		m.mu.Lock()
		ch, ok := m.workerWakeUp[accountID]
		m.mu.Unlock()
		if ok {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	}
	if immediate {
		send()
		return
	}
	// Jitter (0-30s) spreads load when many accounts restart at once.
	jitter := time.Duration(rand.Intn(30_000)) * time.Millisecond
	time.AfterFunc(jitter, send)
}

// WakeUpAccount signals the sync consumer with random jitter (bulk/restart paths).
func (m *Manager) WakeUpAccount(accountID string) {
	m.signalWorker(accountID, false)
}

// WakeUpAccountNow signals the sync consumer immediately (IDLE, user actions, opened email).
func (m *Manager) WakeUpAccountNow(accountID string) {
	m.signalWorker(accountID, true)
}

// RequestFlagRefresh queues a single email for the next inbound IMAP flag sync pass.
func (m *Manager) RequestFlagRefresh(accountID, emailID string) {
	if accountID == "" || emailID == "" {
		return
	}
	m.flagRefreshMu.Lock()
	defer m.flagRefreshMu.Unlock()
	if m.flagRefreshPending[accountID] == nil {
		m.flagRefreshPending[accountID] = make(map[string]struct{})
	}
	m.flagRefreshPending[accountID][emailID] = struct{}{}
}

func (m *Manager) drainFlagRefresh(accountID string) []string {
	m.flagRefreshMu.Lock()
	defer m.flagRefreshMu.Unlock()
	pending := m.flagRefreshPending[accountID]
	if len(pending) == 0 {
		return nil
	}
	ids := make([]string, 0, len(pending))
	for id := range pending {
		ids = append(ids, id)
	}
	delete(m.flagRefreshPending, accountID)
	return ids
}

// StopAccount cancels a specific account's sync and check workers.
// Used by resync handler to kill the old worker before resetting DB state.
func (m *Manager) StopAccount(accountID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cancel, ok := m.workers[accountID]; ok {
		cancel()
		delete(m.workers, accountID)
		delete(m.workerStartedAt, accountID)
		delete(m.workerWakeUp, accountID)
	}
	if cancel, ok := m.checkWorkers[accountID]; ok {
		cancel()
		delete(m.checkWorkers, accountID)
	}
}

// PauseAccount sets the pause flag and stops workers for the account.
// The flag lives in the cache layer (Redis or in-memory), surviving restarts.
func (m *Manager) PauseAccount(accountID string) {
	m.StopAccount(accountID)
}

// ResumeAccount clears the pause flag and triggers a worker refresh.
func (m *Manager) ResumeAccount(accountID string) {
	m.TriggerRefresh()
}

func (m *Manager) RecordActivity(accountID string) {
	m.mu.Lock()
	m.lastActivity[accountID] = time.Now()
	m.mu.Unlock()
}

// Notifier returns the notification rate limiter.
func (m *Manager) Notifier() *notification.RateLimiter {
	return m.notifier
}

// NotifProvider returns the notification provider.
func (m *Manager) NotifProvider() notification.Provider {
	return m.notifProvider
}

// LockTokenRefresh returns an unlock function that serializes OAuth token refreshes per account.
// Only one goroutine per account can refresh at a time; others wait and then proceed.
func (m *Manager) LockTokenRefresh(accountID string) func() {
	m.tokenRefreshMu.Lock()
	mu, ok := m.tokenRefreshLocks[accountID]
	if !ok {
		mu = &sync.Mutex{}
		m.tokenRefreshLocks[accountID] = mu
	}
	m.tokenRefreshMu.Unlock()

	mu.Lock()
	return func() { mu.Unlock() }
}

func (m *Manager) SetEventBroadcast(fn func(ctx context.Context, channel, message string)) {
	m.BroadcastEvent = fn
}

func (m *Manager) TriggerRefresh() {
	select {
	case m.refreshCh <- struct{}{}:
	default:
	}
}

// ResyncAccount kills the worker for accountID, resets its DB state,
// and immediately starts a fresh worker. Atomic — no race with TriggerRefresh.
func (m *Manager) ResyncAccount(ctx context.Context, accountID string) {
	m.mu.Lock()
	// Cancel old workers
	if cancel, ok := m.workers[accountID]; ok {
		cancel()
		delete(m.workers, accountID)
		delete(m.workerStartedAt, accountID)
		delete(m.workerWakeUp, accountID)
	}
	if cancel, ok := m.checkWorkers[accountID]; ok {
		cancel()
		delete(m.checkWorkers, accountID)
	}
	m.mu.Unlock()

	// Wait for goroutines to actually exit (they check ctx.Done() in their loops).
	// Poll every 100ms up to 3 seconds.
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		m.mu.Lock()
		_, syncAlive := m.workers[accountID]
		_, checkAlive := m.checkWorkers[accountID]
		m.mu.Unlock()
		if !syncAlive && !checkAlive {
			break
		}
	}

	// Reset DB state while worker is dead
	if err := m.Store.ResetAccountSync(ctx, accountID); err != nil {
		slog.Error("Manager: ResyncAccount ResetAccountSync failed", "accountID", accountID, "error", err)
		return
	}

	// Start fresh worker
	m.bootstrapMissingWorkers(ctx)
}

func (m *Manager) Start(ctx context.Context) {
	ticker := time.NewTicker(m.Timing.ManagerTicker)
	defer ticker.Stop()

	// Round-Robin rotation: every 5 minutes, evict the oldest worker so
	// no single account can hog a slot indefinitely. Only activates when
	// the number of active accounts exceeds maxWorkers.
	rotateTicker := time.NewTicker(5 * time.Minute)
	defer rotateTicker.Stop()

	// Start Background Email Sync Queue Maintenance (Retry & GC)
	StartQueueManager(ctx, m.Store)

	m.refreshWorkers(ctx)

	// One-time unread count recalculation at startup.
	if err := m.Store.RefreshUnreadCounts(ctx); err != nil {
		slog.Info("Manager: initial RefreshUnreadCounts failed", "error", err)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.refreshWorkers(ctx)
		case <-rotateTicker.C:
			m.maybeRotateWorkers(ctx)
		case <-m.refreshCh:
			m.lastRefresh = time.Now()
			m.forceRestartWorkers(ctx)
		}
	}
}

func (m *Manager) forceRestartWorkers(ctx context.Context) {
	accounts, err := m.Store.GetAccounts(ctx)
	if err != nil {
		slog.Info("Manager: failed to get accounts", "error", err)
		return
	}

	// Prioritize newest accounts (they need initial sync; old ones already have data)
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].CreatedAt.After(accounts[j].CreatedAt)
	})

	m.mu.Lock()
	// Stop all existing workers
	for id, cancel := range m.workers {
		slog.Info("Manager: force-stopping worker for restart", "accountID", id)
		cancel()
		delete(m.workers, id)
		delete(m.workerStartedAt, id)
		delete(m.workerWakeUp, id)
	}
	m.mu.Unlock()

	// Brief pause to let old goroutines exit
	time.Sleep(200 * time.Millisecond)

	m.mu.Lock()
	// Start fresh workers for all active accounts
	for i, acc := range accounts {
		// Re-check lock state right before starting (TOCTOU defense).
		// An account could have been locked between the GetAccounts call and now.
		locked := acc.IsLocked
		if m.LockChecker != nil {
			locked = m.LockChecker(ctx, i)
		}
		if locked {
			slog.Info("Manager: skipping locked account", "email", acc.Email)
			continue
		}
		if len(m.workers) >= m.maxWorkers {
			slog.Info("Manager: max workers limit, skipping", "maxWorkers", m.maxWorkers, "email", acc.Email)
			continue
		}
	}
	m.mu.Unlock()
}

// maybeRotateWorkers evicts the oldest active worker when all maxWorkers slots are
// occupied and there are accounts waiting. This prevents a single heavy account from
// starving others indefinitely (Round-Robin fairness).
func (m *Manager) maybeRotateWorkers(ctx context.Context) {
	m.mu.Lock()
	activeCount := len(m.workers)
	if activeCount < m.maxWorkers {
		m.mu.Unlock()
		m.bootstrapMissingWorkers(ctx)
		return
	}

	// Find the oldest-running worker
	var oldestID string
	var oldestStart time.Time
	for id, started := range m.workerStartedAt {
		if oldestStart.IsZero() || started.Before(oldestStart) {
			oldestStart = started
			oldestID = id
		}
	}

	if oldestID != "" {
		slog.Info("Manager: Round-Robin rotation, evicting oldest worker", "accountID", oldestID, "uptime", time.Since(oldestStart).Round(time.Second))
		if cancel, ok := m.workers[oldestID]; ok {
			cancel()
			delete(m.workers, oldestID)
			delete(m.workerStartedAt, oldestID)
			delete(m.workerWakeUp, oldestID)
		}
	}
	m.mu.Unlock()

	// Fill the freed slot from the waiting queue
	m.bootstrapMissingWorkers(ctx)
}

// bootstrapMissingWorkers starts workers for accounts that don't have one yet,
// up to maxWorkers. Called at startup and after eviction during rotation.
func (m *Manager) bootstrapMissingWorkers(ctx context.Context) {
	accounts, err := m.Store.GetAccounts(ctx)
	if err != nil {
		slog.Info("Manager: bootstrap failed to get accounts", "error", err)
		return
	}

	// Newest accounts first
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].CreatedAt.After(accounts[j].CreatedAt)
	})

	m.mu.Lock()
	started := 0
	for i, acc := range accounts {
		if _, ok := m.workers[acc.ID]; ok {
			continue // already has a worker
		}
		locked := acc.IsLocked
		if m.LockChecker != nil {
			locked = m.LockChecker(ctx, i)
		}
		if locked {
			continue
		}
		if m.IsPaused != nil && m.IsPaused(acc.ID) {
			continue
		}
		if len(m.workers) >= m.maxWorkers {
			break
		}
	}
	m.mu.Unlock()
	if started > 0 {
		slog.Info("Manager: bootstrap started new workers", "started", started, "active", len(m.workers), "maxWorkers", m.maxWorkers)
	}
}

func (m *Manager) refreshWorkers(ctx context.Context) {
	accounts, err := m.Store.GetAccounts(ctx)
	if err != nil {
		slog.Info("Manager: failed to get accounts", "error", err)
		return
	}

	// Prioritize newest accounts (they need initial sync; old ones already have data)
	sort.Slice(accounts, func(i, j int) bool {
		return accounts[i].CreatedAt.After(accounts[j].CreatedAt)
	})

	slog.Info("Manager: got active accounts", "count", len(accounts))

	activeIDs := make(map[string]bool)
	newWorkers := 0

	m.mu.Lock()
	for i, acc := range accounts {
		activeIDs[acc.ID] = true
		if _, ok := m.workers[acc.ID]; !ok {
			// Live lock check via callback (from LicenseMgr) or fallback to DB flag
			locked := acc.IsLocked
			if m.LockChecker != nil {
				locked = m.LockChecker(ctx, i)
			}
			if locked {
				slog.Info("Manager: skipping locked account", "email", acc.Email)
				continue
			}
			if m.IsPaused != nil && m.IsPaused(acc.ID) {
				continue
			}
			if newWorkers >= m.maxWorkers {
				slog.Warn("Manager: max workers limit reached, deferring", "maxWorkers", m.maxWorkers, "email", acc.Email)
				continue
			}
			newWorkers++
			// Start worker with jitter proportional to fleet size
			// Use GetAccountCredentials to get decrypted passwords for the sync worker
		}
	}

	for id, cancel := range m.workers {
		if !activeIDs[id] {
			slog.Info("Manager: stopping worker (account removed)", "accountID", id)
			cancel()
			delete(m.workers, id)
			delete(m.workerStartedAt, id)
			delete(m.workerWakeUp, id)
			// Clean up associated state
			delete(m.lastActivity, id)
			m.tokenRefreshMu.Lock()
			delete(m.tokenRefreshLocks, id)
			m.tokenRefreshMu.Unlock()
			continue
		}
		// Restart worker if no activity for >5 minutes (IDLE connection may have silently died).
		if last, ok := m.lastActivity[id]; ok && time.Since(last) > m.Timing.InactivityRestart {
			slog.Info("Manager: worker inactive, restarting", "accountID", id, "inactiveDuration", time.Since(last).Round(time.Second))
			cancel()
			delete(m.workers, id)
			delete(m.workerStartedAt, id)
			delete(m.workerWakeUp, id)
			delete(m.lastActivity, id)
		}
	}
	m.mu.Unlock()
}

func (m *Manager) startWorkerWithJitter(ctx context.Context, acc models.Account, fleetSize int) {
	baseJitter := m.Timing.WorkerStartJitter
	if fleetSize > 10 {
		baseJitter += time.Duration((fleetSize-10)/2) * time.Second // add 0.5s per extra account
	}
	jitter := time.Duration(rand.Intn(int(baseJitter.Milliseconds())+1)) * time.Millisecond
	slog.Info("Manager: starting worker with jitter", "email", acc.Email, "jitterMs", jitter.Milliseconds(), "fleetSize", fleetSize)

	wCtx, cancel := context.WithCancel(ctx)
	// m.mu is already held by refreshWorkers, don't lock again
	m.workers[acc.ID] = cancel
	m.workerStartedAt[acc.ID] = time.Now()

	// Per-account wakeUp channel so WakeUpAccount can directly signal the consumer loop.
	wakeUpCh := make(chan struct{}, 1)
	m.workerWakeUp[acc.ID] = wakeUpCh

	cCtx, cCancel := context.WithCancel(ctx)
	m.checkWorkers[acc.ID] = cCancel

	fetcher := &Fetcher{Store: m.Store, CAS: m.CAS, AI: m.AI, notifier: m.notifier, notifProvider: m.notifProvider, OAuth: m.OAuth, BroadcastEvent: m.BroadcastEvent, OnActivity: m.RecordActivity, OnNewEmail: m.OnNewEmail, JobNotify: m.JobNotify}
	worker := NewSyncWorker(acc, m.Timing, wakeUpCh, m)
	worker.refreshLocker = m.LockTokenRefresh // wire per-account token refresh serialization
	checkWorker := NewCheckWorker(acc, m)

	m.wg.Add(2)
	go func() {
		slog.Info("sync worker goroutine started", "email", acc.Email, "jitter_ms", jitter.Milliseconds())
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC in sync worker goroutine", "email", acc.Email, "panic", r)
			}
			m.mu.Lock()
			delete(m.workers, acc.ID)
			delete(m.workerStartedAt, acc.ID)
			delete(m.workerWakeUp, acc.ID)
			m.mu.Unlock()
			slog.Info("sync worker removed from manager", "email", acc.Email, "account_id", acc.ID)
			m.wg.Done()
		}()
		time.Sleep(jitter)
		slog.Info("sync worker starting sync", "email", acc.Email, "jitter_done", true)
		worker.Start(wCtx, fetcher)
	}()

	go func() {
		slog.Info("check worker goroutine started", "email", acc.Email, "jitter_ms", jitter.Milliseconds())
		defer func() {
			if r := recover(); r != nil {
				slog.Error("PANIC in check worker goroutine", "email", acc.Email, "panic", r)
			}
			m.mu.Lock()
			delete(m.checkWorkers, acc.ID)
			m.mu.Unlock()
			slog.Info("check worker removed from manager", "email", acc.Email, "account_id", acc.ID)
			m.wg.Done()
		}()
		time.Sleep(jitter)
		slog.Info("check worker starting polling", "email", acc.Email, "jitter_done", true)
		checkWorker.Start(cCtx)
	}()
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	slog.Info("Manager: stopping all workers", "syncWorkers", len(m.workers), "checkWorkers", len(m.checkWorkers))
	for id, cancel := range m.workers {
		slog.Info("Manager: stopping sync worker", "accountID", id)
		cancel()
	}
	for id, cancel := range m.checkWorkers {
		slog.Info("Manager: stopping check worker", "accountID", id)
		cancel()
	}
	m.workers = make(map[string]context.CancelFunc)
	m.checkWorkers = make(map[string]context.CancelFunc)
	m.workerWakeUp = make(map[string]chan struct{})
	m.mu.Unlock()

	// Wait for all worker goroutines to finish (with timeout)
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		slog.Info("Manager: all workers drained gracefully")
	case <-time.After(15 * time.Second):
		slog.Info("Manager: timeout waiting for workers to drain")
	}
}

func (m *Manager) AppendToSent(ctx context.Context, accountID string, emailBody string) error {
	acc, err := m.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		return err
	}

	serverAddr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	encryption := acc.IMAPEncryption
	if encryption == "" {
		if acc.IMAPSSL {
			encryption = "ssl"
		} else {
			encryption = "none"
		}
	}

	var c *imapclient.Client
	var dialErr error
	switch encryption {
	case "ssl":
		c, dialErr = imapclient.DialTLS(serverAddr, nil)
	case "starttls":
		c, dialErr = imapclient.DialStartTLS(serverAddr, nil)
	default:
		c, dialErr = imapclient.DialInsecure(serverAddr, nil)
	}

	if dialErr != nil {
		return fmt.Errorf("IMAP dial failed: %w", dialErr)
	}
	defer c.Close()

	defer c.Logout()

	var sentFolder string
	listCmd := c.List("", "*", &imap.ListOptions{ReturnSpecialUse: true})
	mailboxes, err := listCmd.Collect()
	if err == nil {
		for _, mbox := range mailboxes {
			for _, attr := range mbox.Attrs {
				if attr == imap.MailboxAttrSent {
					sentFolder = mbox.Mailbox
					break
				}
			}
			if sentFolder != "" {
				break
			}
		}
	}

	if sentFolder == "" {
		commonNames := []string{"Sent", "Sent Messages", "Sent Items", "Отправленные", "INBOX.Sent"}
		for _, name := range commonNames {
			cmd := c.List("", name, nil)
			if mbx, err := cmd.Collect(); err == nil && len(mbx) > 0 {
				sentFolder = name
				break
			}
		}
	}

	if sentFolder == "" {
		sentFolder = "Sent" // final fallback
	}

	appendCmd := c.Append(sentFolder, int64(len(emailBody)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
	})
	if _, err := appendCmd.Write([]byte(emailBody)); err != nil {
		return fmt.Errorf("Append write failed: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("Append close failed: %w", err)
	}
	if _, err := appendCmd.Wait(); err != nil {
		return fmt.Errorf("Append wait failed: %w", err)
	}

	slog.Info("Successfully appended sent email", "folder", sentFolder, "email", acc.Email)
	return nil
}

func (m *Manager) AppendToNotes(ctx context.Context, accountID string, noteBody string) error {
	acc, err := m.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		return err
	}

	serverAddr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	encryption := acc.IMAPEncryption
	if encryption == "" {
		if acc.IMAPSSL {
			encryption = "ssl"
		} else {
			encryption = "none"
		}
	}

	var c *imapclient.Client
	var dialErr error
	switch encryption {
	case "ssl":
		c, dialErr = imapclient.DialTLS(serverAddr, nil)
	case "starttls":
		c, dialErr = imapclient.DialStartTLS(serverAddr, nil)
	default:
		c, dialErr = imapclient.DialInsecure(serverAddr, nil)
	}

	if dialErr != nil {
		return fmt.Errorf("IMAP dial failed: %w", dialErr)
	}
	defer c.Close()

	defer c.Logout()

	var notesFolder string
	// Try standard list
	listCmd := c.List("", "*", nil)
	mailboxes, err := listCmd.Collect()
	if err == nil {
		for _, mbox := range mailboxes {
			if strings.EqualFold(mbox.Mailbox, "Notes") || strings.EqualFold(mbox.Mailbox, "RMS_Notes") {
				notesFolder = mbox.Mailbox
				break
			}
		}
	}

	if notesFolder == "" {
		// Attempt to create it if it doesn't exist
		notesFolder = "Notes"
		if err := c.Create(notesFolder, nil).Wait(); err != nil {
			slog.Info("Failed to create Notes folder, falling back to INBOX", "error", err)
			notesFolder = "INBOX" // Fallback to INBOX if creation fails and doesn't exist
		}
	}

	appendCmd := c.Append(notesFolder, int64(len(noteBody)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen},
	})
	if _, err := appendCmd.Write([]byte(noteBody)); err != nil {
		return fmt.Errorf("Append write failed: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("Append close failed: %w", err)
	}
	if _, err := appendCmd.Wait(); err != nil {
		return fmt.Errorf("Append wait failed: %w", err)
	}

	slog.Info("Successfully appended note", "folder", notesFolder, "email", acc.Email)
	return nil
}

func (m *Manager) AppendToDrafts(ctx context.Context, accountID string, draftBody string) error {
	acc, err := m.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		return err
	}

	serverAddr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	encryption := acc.IMAPEncryption
	if encryption == "" {
		if acc.IMAPSSL {
			encryption = "ssl"
		} else {
			encryption = "none"
		}
	}

	var c *imapclient.Client
	var dialErr error
	switch encryption {
	case "ssl":
		c, dialErr = imapclient.DialTLS(serverAddr, nil)
	case "starttls":
		c, dialErr = imapclient.DialStartTLS(serverAddr, nil)
	default:
		c, dialErr = imapclient.DialInsecure(serverAddr, nil)
	}

	if dialErr != nil {
		return fmt.Errorf("IMAP dial failed: %w", dialErr)
	}
	defer c.Close()

	defer c.Logout()

	var draftsFolder string
	// Try standard list
	listCmd := c.List("", "*", nil)
	mailboxes, err := listCmd.Collect()
	if err == nil {
		for _, mbox := range mailboxes {
			nameLower := strings.ToLower(mbox.Mailbox)
			if strings.Contains(nameLower, "drafts") || strings.Contains(nameLower, "черновики") {
				draftsFolder = mbox.Mailbox
				break
			}
		}
	}

	if draftsFolder == "" {
		// Attempt to create it if it doesn't exist
		draftsFolder = "Drafts"
		if err := c.Create(draftsFolder, nil).Wait(); err != nil {
			slog.Info("Failed to create Drafts folder, falling back to INBOX", "error", err)
			draftsFolder = "INBOX" // Fallback to INBOX if creation fails and doesn't exist
		}
	}

	appendCmd := c.Append(draftsFolder, int64(len(draftBody)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen, imap.FlagDraft},
	})
	if _, err := appendCmd.Write([]byte(draftBody)); err != nil {
		return fmt.Errorf("Append write failed: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("Append close failed: %w", err)
	}
	if _, err := appendCmd.Wait(); err != nil {
		return fmt.Errorf("Append wait failed: %w", err)
	}

	slog.Info("Successfully appended draft", "folder", draftsFolder, "email", acc.Email)
	return nil
}

func (m *Manager) AppendToDraftsDeduplicated(ctx context.Context, accountID string, emailID string, draftBody string) error {
	acc, err := m.Store.GetAccountCredentials(ctx, accountID)
	if err != nil {
		return err
	}

	email, err := m.Store.GetEmail(ctx, emailID, accountID)
	if err != nil {
		return fmt.Errorf("failed to get email %s: %w", emailID, err)
	}
	if email == nil {
		return fmt.Errorf("email not found: %s", emailID)
	}

	serverAddr := fmt.Sprintf("%s:%d", acc.IMAPHost, acc.IMAPPort)
	encryption := acc.IMAPEncryption
	if encryption == "" {
		if acc.IMAPSSL {
			encryption = "ssl"
		} else {
			encryption = "none"
		}
	}

	var c *imapclient.Client
	var dialErr error
	switch encryption {
	case "ssl":
		c, dialErr = imapclient.DialTLS(serverAddr, nil)
	case "starttls":
		c, dialErr = imapclient.DialStartTLS(serverAddr, nil)
	default:
		c, dialErr = imapclient.DialInsecure(serverAddr, nil)
	}

	if dialErr != nil {
		return fmt.Errorf("IMAP dial failed: %w", dialErr)
	}
	defer c.Close()

	defer c.Logout()

	var draftsFolder string
	listCmd := c.List("", "*", nil)
	mailboxes, err := listCmd.Collect()
	if err == nil {
		for _, mbox := range mailboxes {
			nameLower := strings.ToLower(mbox.Mailbox)
			if strings.Contains(nameLower, "drafts") || strings.Contains(nameLower, "черновики") {
				draftsFolder = mbox.Mailbox
				break
			}
		}
	}

	if draftsFolder == "" {
		draftsFolder = "Drafts"
		if err := c.Create(draftsFolder, nil).Wait(); err != nil {
			slog.Info("Failed to create Drafts folder, falling back to INBOX", "error", err)
			draftsFolder = "INBOX"
		}
	}

	// Select the drafts folder to allow STORE and EXPUNGE commands
	_, err = c.Select(draftsFolder, nil).Wait()
	if err != nil {
		return fmt.Errorf("failed to select Drafts folder: %w", err)
	}

	// Check if UIDPLUS is supported
	caps, err := c.Capability().Wait()
	hasUIDPlus := false
	if err == nil {
		_, hasUIDPlus = caps["UIDPLUS"]
	}

	if email.DraftRemoteUID > 0 {
		// Delete the old draft
		uidSet := imap.UIDSetNum(imap.UID(email.DraftRemoteUID))
		storeCmd := c.Store(uidSet, &imap.StoreFlags{
			Op:    imap.StoreFlagsAdd,
			Flags: []imap.Flag{imap.FlagDeleted},
		}, nil)
		if err := storeCmd.Close(); err == nil {
			if hasUIDPlus {
				c.UIDExpunge(uidSet).Close()
			} else {
				c.Expunge().Close()
			}
		} else {
			slog.Info("failed to mark old draft as deleted", "error", err)
		}
	}

	appendCmd := c.Append(draftsFolder, int64(len(draftBody)), &imap.AppendOptions{
		Flags: []imap.Flag{imap.FlagSeen, imap.FlagDraft},
	})
	if _, err := appendCmd.Write([]byte(draftBody)); err != nil {
		return fmt.Errorf("Append write failed: %w", err)
	}
	if err := appendCmd.Close(); err != nil {
		return fmt.Errorf("Append close failed: %w", err)
	}
	appendData, err := appendCmd.Wait()
	if err != nil {
		return fmt.Errorf("Append wait failed: %w", err)
	}

	if appendData != nil && appendData.UID > 0 {
		m.Store.SetDraftRemoteUID(ctx, emailID, acc.ID, int(appendData.UID))
		slog.Info("Successfully deduplicated draft, new UID saved", "uid", appendData.UID)
	}

	slog.Info("Successfully appended draft (deduplicated)", "folder", draftsFolder, "email", acc.Email)
	return nil
}

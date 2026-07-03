package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"rmsmail/internal/ai"
	"rmsmail/internal/api/middleware"
	"rmsmail/internal/attachment"
	"rmsmail/internal/auth"
	"rmsmail/internal/crypto"
	"rmsmail/internal/edition"
	"rmsmail/internal/models"
)

// Store is the composite data-access interface.
// Prefer the segregated interfaces (EmailReader, EmailWriter, etc.) in new code.
// Kept for backward compatibility — main.go passes a single *Storage to all fields.
type Store interface {
	EmailReader
	EmailWriter
	AccountStore
	FolderStore
	EntityStore
	AdminStore
	SystemStore
}

type Handler struct {
	// Store is the composite interface — kept for backward compatibility.
	// Prefer the segregated fields below for new code.
	Store Store

	// Segregated interfaces (ISP). Each handler method depends on 1–3 of these.
	Emails   EmailReader
	Writer   EmailWriter
	Accounts AccountStore
	Folders  FolderStore
	Entities EntityStore
	Admin    AdminStore
	System   SystemStore

	CAS        *attachment.CASStorage
	AI         *ai.Gateway
	AIDisabled bool
	OAuth      *auth.OAuthManager

	// Asynq task queue (nil = fallback to Scheduler)
	EventBus *EventBus

	MemCache    *MemoryCache // in-memory cache fallback for Mono edition
	SyncManager interface {
		TriggerRefresh()
		StopAccount(string)
		PauseAccount(string)
		ResumeAccount(string)
		ResyncAccount(ctx context.Context, accountID string)
		WakeUpAccount(accountID string)
		WakeUpAccountNow(accountID string)
		RequestFlagRefresh(accountID, emailID string)
	}
	PriorityChecker interface {
		CheckAccount(ctx context.Context, acc models.Account) error
	}
	TestConnLimiter *middleware.InMemoryRateLimiter

	TGBot interface {
		HandleWebhook(w http.ResponseWriter, r *http.Request)
		IsConfigured() bool
		GetUsername() string
		GetToken() string
		SetToken(token string)
		FetchUsername(ctx context.Context) (string, error)
		SetWebhook(ctx context.Context, publicURL string) error
	}
	ticketStore *TicketStore
	creationMu  sync.Mutex
}

func (h *Handler) cacheGet(ctx context.Context, key string) (string, bool) {
	if false {

	}
	if h.MemCache != nil {
		return h.MemCache.Get(key)
	}
	return "", false
}

func (h *Handler) cacheSet(ctx context.Context, key, value string, ttl time.Duration) {
	if false {

	}

}

// IsAccountSyncPaused returns whether sync is paused for the given account.
func (h *Handler) IsAccountSyncPaused(ctx context.Context, accountID string) bool {
	cacheKey := "account:sync_paused:" + accountID
	val, ok := h.cacheGet(ctx, cacheKey)
	return ok && val != ""
}

func (h *Handler) tryCache(w http.ResponseWriter, r *http.Request, key string) bool {
	if cached, ok := h.cacheGet(r.Context(), key); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(cached))
		return true
	}
	return false
}

func (h *Handler) InitTicketStore() {
	h.ticketStore = NewTicketStore()
}

// InvalidateEmailCache clears the email list cache for a specific account.
// It also clears the unified cache to ensure consistency.
func (h *Handler) InvalidateEmailCache(ctx context.Context, accountID string) {
	if false {
		// Use SCAN instead of KEYS to avoid blocking the Redis event loop.

	}

	// MemCache (Mono edition) — clear relevant caches
	if h.MemCache != nil {
		if accountID != "" {
			h.MemCache.Del("folders:" + accountID)
			for _, k := range h.MemCache.Keys() {
				if strings.HasPrefix(k, "email_list:acc:"+accountID+":") ||
					strings.HasPrefix(k, "email_list:acc:unified:") ||
					strings.HasPrefix(k, "email_list:acc:grp:") {
					h.MemCache.Del(k)
				}
			}
		} else {
			// Empty accountID = clear all caches (bulk operations)
			h.MemCache.Del("folders:unified", "accounts:list")
			for _, k := range h.MemCache.Keys() {
				if strings.HasPrefix(k, "email_list:acc:") || strings.HasPrefix(k, "folders:") {
					h.MemCache.Del(k)
				}
			}
		}
	}
}

// InvalidateMetaCache clears Redis caches for accounts, folders, and groups.
// Call after account/group mutations. Safe to call when h.Redis is nil.
func (h *Handler) InvalidateMetaCache(ctx context.Context, accountID string) {
	if false {

	}
	// MemCache fallback (Mono edition)
	if h.MemCache != nil {
		h.MemCache.Del("accounts:list", "groups:list")
		if accountID != "" {
			h.MemCache.Del(
				"folders:"+accountID,
				"labels:"+accountID,
				"rules:"+accountID,
				"templates:"+accountID,
				"contacts:"+accountID,
				"identities:"+accountID,
				"webhooks:"+accountID,
			)
		}
	}
}

// InvalidateEmailCacheByEmailID finds the account ID for a given email and invalidates its cache.
func (h *Handler) InvalidateEmailCacheByEmailID(ctx context.Context, emailID string) {
	email, err := h.Store.GetEmail(ctx, emailID, "")
	if err == nil && email != nil {
		h.InvalidateEmailCache(ctx, email.AccountID)
	}
}

func (h *Handler) HandleLicense(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status":"free"})
}

func (h *Handler) HandleCheckUpdates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status":"ok"})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.CAS.GetStats(r.Context())
	if err != nil {
		WriteInternalError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// Must be called after environment variables are parsed but before the server starts.
func InitializeSecurity() {
	middleware.InitJWTAuth()
	slog.Info("Security module initialized (JWT + encryption)")
}

func generateAPIKey() (raw string, hash string, prefix string) {
	b := make([]byte, 24)
	rand.Read(b)
	raw = "rmsm_" + hex.EncodeToString(b)
	h := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(h[:])
	prefix = raw[:8]
	return
}

func readEncryptedFile(path string) ([]byte, error) {
	return crypto.ReadEncryptedFile(path, crypto.GetPrimaryEncryptionKey())
}

// CheckAccountAccess enforces data isolation for Mono edition.
// It verifies that the requested accountID actually belongs to the authenticated user.
// In Unified edition, this check is a no-op (relies on its own RBAC).
func (h *Handler) CheckAccountAccess(ctx context.Context, accountID string) error {
	if accountID == "" {
		return errors.New("account_id is required")
	}
	if accountID == "unified" {
		return nil
	}
	if accountID == "00000000-0000-0000-0000-000000000000" {
		if edition.IsMono() || edition.IsMonoPro() {
			return errors.New("access to system account is forbidden in Mono edition")
		}
	}

	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		return errors.New("unauthorized")
	}

	// Try cache first — avoids N+1 DB queries under concurrent load (M edition).
	// cache:account:meta:{id} stores the account email, populated by OnSendTelegram callback.
	if cachedEmail, ok := h.cacheGet(ctx, "cache:account:meta:"+accountID); ok {
		if edition.IsMono() || edition.IsMonoPro() {
			if !strings.EqualFold(cachedEmail, userID) {
				return errors.New("access denied: account does not belong to the current user")
			}
			return nil
		}
		return h.requireAdminForNonMono(ctx)
	}

	account, err := h.Store.GetAccount(ctx, accountID)
	if err != nil {
		return errors.New("failed to verify account access")
	}
	if account == nil {
		return errors.New("account not found")
	}

	// Populate cache for next request
	h.cacheSet(ctx, "cache:account:meta:"+accountID, account.Email, 30*time.Second)

	if edition.IsMono() || edition.IsMonoPro() {
		if !strings.EqualFold(account.Email, userID) {
			return errors.New("access denied: account does not belong to the current user")
		}
		return nil
	}
	return h.requireAdminForNonMono(ctx)
}

// requireAdminForNonMono enforces admin access in Unified edition (not used in Mono/MonoPro).
func (h *Handler) requireAdminForNonMono(ctx context.Context) error {
	if edition.IsMono() || edition.IsMonoPro() {
		return nil
	}
	userID := middleware.GetUserIDFromContext(ctx)
	if userID == "" {
		return errors.New("unauthorized")
	}
	if middleware.IsAdminFromContext(ctx) {
		return nil
	}
	adminID, _, err := h.Store.GetAdminByEmail(ctx, userID)
	if err != nil || adminID == "" {
		return errors.New("access denied: admin required")
	}
	return nil
}

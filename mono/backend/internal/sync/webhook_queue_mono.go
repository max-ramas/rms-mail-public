package sync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"rmsmail/internal/models"
	"strings"
	"time"
)

// StartSQLiteWebhookPoller runs a background loop that dispatches webhooks from the SQLite embedded queue.
// It should be launched as a goroutine in the Mono version.
func StartSQLiteWebhookPoller(ctx context.Context, store SyncStore) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("PANIC in sqlite webhook poller", "panic", r)
		}
	}()

	slog.Info("webhook poller started (sqlite-backed)")
	consecutiveBusy := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		nowUnix := time.Now().Unix()
		// Claim due webhooks. SQLite has no atomic pop (ZClaim), so we just SELECT and hope we are the only poller.
		// Since it's Mono, there's only one instance running.
		retries, err := store.GetDueWebhookRetries(ctx, nowUnix, 100)
		if err != nil {
			errStr := err.Error()
			slog.Warn("webhook poller sqlite error", "error", err)
			// SQLITE_BUSY: back off exponentially to reduce contention
			if strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "SQLITE_BUSY") {
				consecutiveBusy++
				backoff := time.Duration(min(consecutiveBusy, 10)) * time.Second
				time.Sleep(backoff)
				continue
			}
			consecutiveBusy = 0
			time.Sleep(5 * time.Second)
			continue
		}
		consecutiveBusy = 0

		for _, retry := range retries {
			dispatchSQLiteWebhookJob(store, retry)
		}

		time.Sleep(1 * time.Second)
	}
}

func dispatchSQLiteWebhookJob(store SyncStore, retry models.WebhookRetry) {
	parsed, parseErr := url.Parse(retry.URL)
	if parseErr != nil {
		store.DeleteWebhookRetry(context.Background(), retry.ID)
		return
	}

	// SSRF rebinding guard: resolve + pin IP
	pinnedIP, err := ValidateWebhookURL(retry.URL)
	if err != nil {
		slog.Error("sqlite webhook poller: SSRF blocked", "url", retry.URL, "error", err)
		store.DeleteWebhookRetry(context.Background(), retry.ID)
		return
	}

	httpClient := &http.Client{
		Transport: createPinnedTransport(pinnedIP, parsed.Hostname()),
		Timeout:   10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", retry.URL, bytes.NewReader(retry.Payload))
	if err != nil {
		requeueSQLiteWebhook(store, retry)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "RMS-Mail-Webhook/1.0")

	// HMAC signature
	if retry.Secret != "" {
		mac := hmac.New(sha256.New, []byte(retry.Secret))
		mac.Write(retry.Payload)
		signature := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-RMS-Signature", signature)
		// For backward compatibility with some legacy receivers
		req.Header.Set("X-Signature-256", signature)
	}

	resp, err := httpClient.Do(req)
	if err != nil || resp == nil {
		requeueSQLiteWebhook(store, retry)
		return
	}
	resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		requeueSQLiteWebhook(store, retry)
		return
	}

	// Success, delete from DB
	if err := store.DeleteWebhookRetry(context.Background(), retry.ID); err != nil {
		slog.Warn("failed to remove webhook from sqlite after success", "url", retry.URL, "error", err)
	}
}

func requeueSQLiteWebhook(store SyncStore, retry models.WebhookRetry) {
	maxRetry := 3
	if retry.Attempt >= maxRetry {
		slog.Error("webhook permanently failed after retries", "url", retry.URL, "attempts", retry.Attempt)
		store.DeleteWebhookRetry(context.Background(), retry.ID)
		return
	}

	retry.Attempt++
	backoff := time.Duration(1<<uint(retry.Attempt-1)) * time.Second
	if backoff > 10*time.Second {
		backoff = 10 * time.Second
	}

	nextRetryAtUnix := time.Now().Add(backoff).Unix()

	if err := store.UpdateWebhookRetryAttempt(context.Background(), retry.ID, retry.Attempt, nextRetryAtUnix); err != nil {
		slog.Error("failed to update webhook retry attempt", "url", retry.URL, "error", err)
	}
}

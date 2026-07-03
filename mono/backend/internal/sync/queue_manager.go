package sync

import (
	"context"
	"log/slog"
	"strings"
	"time"
)

// StartQueueManager starts the background workers for Email Sync Queue (Retry & GC).
func StartQueueManager(ctx context.Context, store SyncStore) {
	if store == nil {
		return
	}

	slog.Info("Starting Email Sync Queue Manager (Retry, GC)")

	retryTicker := time.NewTicker(30 * time.Second)
	gcTicker := time.NewTicker(1 * time.Hour)

	go func() {
		defer retryTicker.Stop()
		defer gcTicker.Stop()

		consecutiveBusy := 0

		for {
			select {
			case <-retryTicker.C:
				count, err := store.ProcessQueueRetries(ctx)
				if err != nil {
					errStr := err.Error()
					slog.Error("QueueManager: Failed to process retries", "error", err)
					if strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "SQLITE_BUSY") {
						consecutiveBusy++
						backoff := time.Duration(min(consecutiveBusy, 10)) * time.Second
						time.Sleep(backoff)
						continue
					}
					consecutiveBusy = 0
				} else {
					consecutiveBusy = 0
					if count > 0 {
						slog.Info("QueueManager: Processed retries", "count", count)
					}
				}

			case <-gcTicker.C:
				count, err := store.CleanQueueGarbage(ctx, 24*time.Hour, 7*24*time.Hour)
				if err != nil {
					slog.Error("QueueManager: Failed to clean garbage", "error", err)
				} else if count > 0 {
					slog.Info("QueueManager: Cleaned garbage", "count", count)
				}

			case <-ctx.Done():
				slog.Info("QueueManager: Shutting down gracefully")
				return
			}
		}
	}()
}

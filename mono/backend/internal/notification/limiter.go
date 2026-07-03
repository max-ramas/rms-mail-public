package notification

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type job struct {
	ctx      context.Context
	provider Provider
	targetID string
	text     string
}

// RateLimiter limits the rate of outgoing notifications using a token-bucket
// approach and processes them via a fixed-size worker pool to avoid goroutine leaks.
type RateLimiter struct {
	ch     chan struct{}
	jobs   chan job
	stopCh chan struct{}
}

func NewRateLimiter(maxPerSec int) *RateLimiter {
	if maxPerSec <= 0 {
		maxPerSec = 1
	}
	l := &RateLimiter{
		ch:     make(chan struct{}, maxPerSec),
		jobs:   make(chan job, 100),
		stopCh: make(chan struct{}),
	}
	for i := 0; i < maxPerSec; i++ {
		l.ch <- struct{}{}
	}
	go func() {
		ticker := time.NewTicker(time.Second / time.Duration(maxPerSec))
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				select {
				case l.ch <- struct{}{}:
				default:
				}
			case <-l.stopCh:
				return
			}
		}
	}()
	// Start a fixed worker pool (20 workers) to prevent goroutine explosion.
	for i := 0; i < 20; i++ {
		go l.worker()
	}
	return l
}

func (l *RateLimiter) worker() {
	for j := range l.jobs {
		l.Wait()
		if err := j.provider.Send(j.ctx, j.targetID, j.text); err != nil {
			slog.Info(fmt.Sprintf("Notification send failed: %v", err))
		}
	}
}

func (l *RateLimiter) Wait() {
	<-l.ch
}

// SendAsync enqueues a notification for asynchronous delivery.
// If the internal queue is full the message is dropped and logged.
func (l *RateLimiter) SendAsync(ctx context.Context, provider Provider, targetID, text string) {
	select {
	case l.jobs <- job{ctx: ctx, provider: provider, targetID: targetID, text: text}:
	default:
		slog.Info(fmt.Sprintf("Notification queue full (%d pending), dropping message to %s", len(l.jobs), targetID))
	}
}

// Stop signals the rate limiter to stop refilling tokens and shut down workers.
func (l *RateLimiter) Stop() {
	close(l.stopCh)
	close(l.jobs)
}

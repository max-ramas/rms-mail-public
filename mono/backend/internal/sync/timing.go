package sync

import (
	"math"
	"math/rand"
	"time"
)

// Timing holds sync timing parameters that vary by edition.
type Timing struct {
	IDLETimeout        time.Duration // Periodic full-sync while in IDLE
	IDLEWatchdog       time.Duration // Detect hung IDLE connections
	ManagerTicker      time.Duration // Account refresh interval
	InactivityRestart  time.Duration // Restart worker after silent inactivity
	WorkerStartJitter  time.Duration // Max random jitter before starting sync
	ReconnectBase      time.Duration // Base delay for reconnect backoff
	ReconnectMaxWait   time.Duration // Max delay for reconnect backoff
	ReconnectRetries   int           // Max reconnect attempts
	DraftThrottle      time.Duration // Per-draft APPEND rate limit
	FolderScanInterval time.Duration // Periodic non-INBOX folder UID scan
	ErrorBackoffBase   time.Duration // Base backoff for error recovery
	ErrorBackoffMax    time.Duration // Max backoff for error recovery
}

// DefaultTiming returns timing tuned for multi-tenant Unified/Teams editions.
func DefaultTiming() Timing {
	return Timing{
		IDLETimeout:        2 * time.Minute,
		IDLEWatchdog:       3 * time.Minute,
		ManagerTicker:      1 * time.Minute,
		InactivityRestart:  5 * time.Minute,
		WorkerStartJitter:  5 * time.Second,
		ReconnectBase:      15 * time.Second,
		ReconnectMaxWait:   300 * time.Second,
		ReconnectRetries:   10,
		DraftThrottle:      3 * time.Second,
		FolderScanInterval: 5 * time.Minute,
		ErrorBackoffBase:   60 * time.Second,
		ErrorBackoffMax:    10 * time.Minute,
	}
}

// MonoTiming returns timing for single-user Mono edition.
func MonoTiming() Timing {
	return Timing{
		IDLETimeout:        30 * time.Second,
		IDLEWatchdog:       60 * time.Second,
		ManagerTicker:      15 * time.Second,
		InactivityRestart:  2 * time.Minute,
		WorkerStartJitter:  500 * time.Millisecond,
		ReconnectBase:      10 * time.Second,
		ReconnectMaxWait:   60 * time.Second,
		ReconnectRetries:   6,
		DraftThrottle:      500 * time.Millisecond,
		FolderScanInterval: 2 * time.Minute,
		ErrorBackoffBase:   30 * time.Second,
		ErrorBackoffMax:    5 * time.Minute,
	}
}

// maxBackoff caps reconnect delays to prevent unbounded waits.
const maxBackoff = 5 * time.Minute

// CalculateReconnectDelay computes exponential backoff with ±15% jitter
// to prevent thundering herd when multiple accounts reconnect simultaneously.
//
// Formula: base * 2^attempt + random(-15%, +15%) of that value.
// Capped at maxBackoff (5 minutes). Minimum is ReconnectBase.
func CalculateReconnectDelay(base time.Duration, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}

	backoff := float64(base) * math.Pow(2, float64(attempt))
	if backoff > float64(maxBackoff) {
		backoff = float64(maxBackoff)
	}

	// ±15% jitter: spreads reconnect attempts across time
	jitter := (rand.Float64()*0.3 - 0.15) * backoff
	final := time.Duration(backoff + jitter)

	if final < base {
		return base
	}
	return final
}

package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// extractClientIP returns the real client IP, respecting reverse proxy headers.
// Priority: X-Forwarded-For (leftmost IP) → X-Real-IP → RemoteAddr.
func extractClientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		if idx := strings.IndexByte(fwd, ','); idx > 0 {
			return strings.TrimSpace(fwd[:idx])
		}
		return strings.TrimSpace(fwd)
	}
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

var rateLimitScript = redis.NewScript(`
	local current = redis.call('incr', KEYS[1])
	if current == 1 then
		redis.call('expire', KEYS[1], ARGV[1])
	end
	return current
`)

func NewRedisRateLimiter(rdb *redis.Client, prefix string, limit, windowSec int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractClientIP(r)

			key := prefix + ip
			res, err := rateLimitScript.Run(r.Context(), rdb, []string{key}, windowSec).Int64()
			if err != nil {
				WriteInternalError(w, r, err)
				return
			}

			if res > int64(limit) {
				w.Header().Set("Retry-After", strconv.Itoa(windowSec))
				WriteJSONError(w, http.StatusTooManyRequests, "too many requests")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type visitorEntry struct {
	count     int
	expiresAt time.Time
}

// InMemoryRateLimiter implements a per-IP rate limiter with automatic TTL-based cleanup.
// Stale entries are removed periodically to prevent memory leaks.
type InMemoryRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitorEntry
	limit    int
	window   time.Duration
	stopCh   chan struct{}
}

// NewInMemoryRateLimiter creates a rate limiter allowing `limit` requests per 60-second window.
// A background goroutine cleans up expired entries every 5 minutes.
func NewInMemoryRateLimiter(limit int) *InMemoryRateLimiter {
	rl := &InMemoryRateLimiter{
		visitors: make(map[string]*visitorEntry),
		limit:    limit,
		window:   60 * time.Second,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

// Stop terminates the background cleanup goroutine.
func (rl *InMemoryRateLimiter) Stop() {
	close(rl.stopCh)
}

func (rl *InMemoryRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for ip, entry := range rl.visitors {
				if now.After(entry.expiresAt) {
					delete(rl.visitors, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *InMemoryRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractClientIP(r)

		rl.mu.Lock()
		entry, ok := rl.visitors[ip]
		now := time.Now()

		if !ok || now.After(entry.expiresAt) {
			entry = &visitorEntry{
				count:     1,
				expiresAt: now.Add(rl.window),
			}
			rl.visitors[ip] = entry
			rl.mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		entry.count++
		count := entry.count
		rl.mu.Unlock()

		if count > rl.limit {
			slog.Info(fmt.Sprintf("rate limit exceeded for IP %s: %d > %d", ip, count, rl.limit))
			WriteJSONError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		next.ServeHTTP(w, r)
	})
}

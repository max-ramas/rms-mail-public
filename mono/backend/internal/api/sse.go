package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"rmsmail/internal/api/middleware"

	"github.com/golang-jwt/jwt/v5"
)

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan string]struct{}
	closeOnce   map[chan string]*sync.Once
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string]map[chan string]struct{}),
		closeOnce:   make(map[chan string]*sync.Once),
	}
}

func (b *EventBus) Subscribe(channel string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribers[channel] == nil {
		b.subscribers[channel] = make(map[chan string]struct{})
	}
	ch := make(chan string, 10)
	b.subscribers[channel][ch] = struct{}{}
	b.closeOnce[ch] = &sync.Once{}
	return ch
}

func (b *EventBus) Unsubscribe(channel string, ch chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if subs, ok := b.subscribers[channel]; ok {
		delete(subs, ch)
		if once, ok := b.closeOnce[ch]; ok {
			once.Do(func() { close(ch) })
			delete(b.closeOnce, ch)
		}
	}
}

func (b *EventBus) Publish(channel, message string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.subscribers[channel]; ok {
		for ch := range subs {
			select {
			case ch <- message:
			default:
			}
		}
	}
}

func (h *Handler) SSE(w http.ResponseWriter, r *http.Request) {
	AppMetrics.ActiveSSEConns.Add(1)
	defer AppMetrics.ActiveSSEConns.Add(-1)

	// Auth via httpOnly cookie (primary) or ?ticket= (short-lived one-time ticket).
	if r.URL.Query().Get("token") != "" {
		WriteJSONError(w, http.StatusBadRequest, "SSE ?token= is no longer supported; use POST /api/auth/ticket or cookie auth")
		return
	}

	ticketStr := r.URL.Query().Get("ticket")
	var userID string
	var jwtClaims jwt.MapClaims

	if ticketStr != "" {
		userID = h.ValidateTicket(ticketStr)
	} else if cookie, err := r.Cookie("rms_token"); err == nil && cookie.Value != "" {
		jwtToken, err := middleware.ValidateToken(cookie.Value)
		if err == nil && jwtToken.Valid {
			if claims, ok := jwtToken.Claims.(jwt.MapClaims); ok {
				userID, _ = claims["sub"].(string)
				jwtClaims = claims
			}
		}
	}

	if userID == "" {
		WriteJSONError(w, http.StatusUnauthorized, "invalid or expired token")
		return
	}

	ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
	ctx = context.WithValue(ctx, middleware.ClaimsKey, jwtClaims)
	r = r.WithContext(ctx)

	flusher, ok := w.(http.Flusher)
	if !ok {
		WriteJSONError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Send retry field with 5-10s jitter so browsers spread reconnects
	// rather than stampeding the server simultaneously after a restart.
	fmt.Fprintf(w, "retry: %d\n", 5000+rand.Intn(5000))
	// Send immediate heartbeat so the browser knows the connection is alive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	msgCh := make(chan struct {
		Event string
		Data  string
	}, 10)

	if false {

	}

	for {
		select {
		case <-r.Context().Done():
			return
		case msg := <-msgCh:
			// Security filtering (C2): skip events for accounts the user cannot access
			if len(msg.Data) > 0 && msg.Data[0] == '{' {
				var payload struct {
					AccountID string `json:"account_id"`
				}
				if err := json.Unmarshal([]byte(msg.Data), &payload); err == nil && payload.AccountID != "" {
					if chkErr := h.CheckAccountAccess(r.Context(), payload.AccountID); chkErr != nil {
						continue // skip
					}
				}
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", msg.Event, msg.Data)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: heartbeat\ndata: ping\n\n")
			flusher.Flush()
		}
	}
}

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

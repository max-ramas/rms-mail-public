package api

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

// TicketData holds the payload associated with a one-time ticket.
type TicketData struct {
	UserID    string
	AccountID string
	ExpiresAt time.Time
}

// TicketStore holds short-lived one-time tickets for SSE/attachment URL auth.
// Tickets are generated with crypto/rand, have a 30s TTL, and are single-use
// ("burn after reading").
type TicketStore struct {
	mu      sync.RWMutex
	tickets map[string]TicketData
	ttl     time.Duration
}

func NewTicketStore() *TicketStore {
	ts := &TicketStore{
		tickets: make(map[string]TicketData),
		ttl:     30 * time.Second,
	}
	go ts.cleanupLoop()
	return ts
}

// GenerateTicket creates a cryptographically random 32-byte hex ticket,
// stores it with the given user/account, and returns the ticket string.
func (ts *TicketStore) GenerateTicket(userID, accountID string) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	ticket := hex.EncodeToString(b)

	ts.mu.Lock()
	ts.tickets[ticket] = TicketData{
		UserID:    userID,
		AccountID: accountID,
		ExpiresAt: time.Now().Add(ts.ttl),
	}
	ts.mu.Unlock()

	return ticket, nil
}

// ValidateTicket checks a one-time ticket. If valid, it returns the data
// and deletes the ticket ("burn after reading"). Returns false if the
// ticket does not exist or has expired.
func (ts *TicketStore) ValidateTicket(ticket string) (TicketData, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	data, exists := ts.tickets[ticket]
	if !exists {
		return TicketData{}, false
	}

	// Burn after reading — ticket is single-use
	delete(ts.tickets, ticket)

	if time.Now().After(data.ExpiresAt) {
		return TicketData{}, false
	}

	return data, true
}

// cleanupLoop periodically removes expired tickets to prevent unbounded
// memory growth in case of leaked/abandoned tickets.
func (ts *TicketStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		ts.mu.Lock()
		for k, v := range ts.tickets {
			if now.After(v.ExpiresAt) {
				delete(ts.tickets, k)
			}
		}
		ts.mu.Unlock()
	}
}

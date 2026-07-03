
package store

import (
	"rmsmail/internal/store/sqlite"
)

// NewStorage creates a local LibSQL-backed storage (Mono edition).
// Uses embedded libsql driver — pure Go, CGO_ENABLED=0, no external services.
func NewStorage(dbURL string, encKeys [][]byte) (*sqlite.Storage, error) {
	return sqlite.NewStorage(dbURL, encKeys)
}

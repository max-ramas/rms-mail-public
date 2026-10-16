package main

import (
	"rmsmail/internal/store/sqlite"
)

// In Mono edition, backfillAvatars is a no-op
// since it depends on PostgreSQL-specific features.
func backfillAvatars(store *sqlite.Storage) {
	// No-op in Mono edition
}

func buildFTSPartitions(store *sqlite.Storage) {
	// No-op in Mono edition (uses SQLite FTS5)
}

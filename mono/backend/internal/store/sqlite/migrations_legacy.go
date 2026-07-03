package sqlite

import (
	"context"
	"log/slog"
	"strings"
)

// firstNewTrackedMigration is the first migration that must actually run on legacy Mono DBs
// (schema predates schema_migrations table). Earlier files are backfilled when detected.
const firstNewTrackedMigration = "022_folders_uid_validity_mono.sql"

func isBenignSQLiteMigrationError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column") ||
		strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "near \"exists\"") // ADD COLUMN IF NOT EXISTS unsupported on LibSQL
}

// backfillLegacySQLiteMigrations marks pre-022 migrations as applied on DBs provisioned
// from schema_mono.sql before schema_migrations tracking existed.
func (s *Storage) backfillLegacySQLiteMigrations(ctx context.Context, files []string) (int, error) {
	var tableName string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='emails'`,
	).Scan(&tableName)
	if err != nil {
		return 0, nil
	}

	backfilled := 0
	for _, f := range files {
		if f >= firstNewTrackedMigration {
			break
		}
		var exists int
		if err := s.db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", f,
		).Scan(&exists); err != nil {
			return backfilled, err
		}
		if exists > 0 {
			continue
		}
		if _, err := s.db.ExecContext(ctx,
			"INSERT INTO schema_migrations (filename) VALUES (?)", f,
		); err != nil {
			return backfilled, err
		}
		slog.Info("migration backfilled (legacy mono schema)", "file", f)
		backfilled++
	}
	return backfilled, nil
}

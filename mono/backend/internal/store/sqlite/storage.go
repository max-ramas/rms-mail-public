package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"rmsmail/internal/migrations"
	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// Storage is the SQLite/LibSQL implementation of the api.Store interface for Mono edition.
type Storage struct {
	db         *sql.DB
	highWrites chan writeJob
	lowWrites  chan writeJob
	encKey     []byte   // primary (first) key for encryption
	encKeys    [][]byte // all keys for decryption (primary + fallbacks)
	dbPath     string
}

// NewStorage creates a new local LibSQL Storage for Mono edition.
// Uses embedded libsql driver (pure Go, CGO_ENABLED=0, no external services).
func NewStorage(dbPath string, encKeys [][]byte) (*Storage, error) {
	if dbPath == "" {
		dbPath = "./rms-mail.db"
	}

	// DSN with all critical PRAGMAs so every connection from the pool gets them,
	// not just the first one. _busy_timeout=30000 gives writers 30s before SQLITE_BUSY.
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=30000&_synchronous=NORMAL&_cache_size=-64000&_foreign_keys=ON", dbPath)

	slog.Info(fmt.Sprintf("LibSQL: opening local DSN=%s", dsn))

	db, err := sql.Open("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("libsql open: %w", err)
	}

	// Connection pool tuned for multi-user concurrency under WAL mode:
	// - Up to 25 concurrent connections: readers never block writers, writers never starve.
	// - 5 idle connections kept warm: avoids open/close churn on every request.
	// - SQLite WAL handles the single-writer constraint internally; busy_timeout=30000
	//   gives colliding writers up to 30s to acquire the lock before SQLITE_BUSY.
	// Shared SQLite for all Mono users (WAL); keep pool small to reduce lock churn.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(0)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA temp_store=MEMORY;",
		"PRAGMA mmap_size=268435456;", // 256MB memory-mapped I/O
		"PRAGMA cache_size=-64000;",
		"PRAGMA foreign_keys=ON;",
		"PRAGMA busy_timeout=30000;",
		// WAL autocheckpoint: trigger checkpoint every 1000 pages (~4MB).
		// Prevents WAL file ballooning during large sync batches.
		"PRAGMA wal_autocheckpoint=1000;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("libsql pragma %s failed: %w", pragma, err)
		}
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("libsql ping: %w", err)
	}

	if len(encKeys) == 0 {
		slog.Error("ENCRYPTION_KEYS (or ENCRYPTION_KEY) must not be empty — set it to a random string of any length (will be SHA-256 derived to 32 bytes)")
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("LibSQL: connected to %s (WAL, CGO_ENABLED=0)", dbPath))
	s := &Storage{
		db:         db,
		highWrites: make(chan writeJob, 64),
		lowWrites:  make(chan writeJob, 512),
		encKey:     encKeys[0],
		encKeys:    encKeys,
		dbPath:     dbPath,
	}
	s.startWriteWorker()
	return s, nil
}

// WithTx executes fn within a SQLite transaction, committing on success
// and rolling back on error. Uses the write worker for serialization.
func (s *Storage) WithTx(ctx context.Context, fn func(ctx context.Context, tx *sql.Tx) error) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin tx: %w", err)
		}
		defer tx.Rollback()

		if err := fn(ctx, tx); err != nil {
			return err
		}
		return tx.Commit()
	})
}

// Ping checks database connectivity.
func (s *Storage) InitSchema(ctx context.Context, schemaSQL string) error {
	// SQLite doesn't support multiple statements in one Exec; split by semicolons
	for _, stmt := range strings.Split(schemaSQL, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "duplicate column name") || strings.Contains(errStr, "already exists") {
				continue // Ignore expected migration errors
			}
			slog.Info(fmt.Sprintf("⚠️ SQLite schema statement warning: %v (SQL: %.80s...)", err, stmt))
		}
	}

	// FTS5 virtual table — SQLite only
	if _, err := s.db.ExecContext(ctx, fts5Schema); err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "already exists") {
			slog.Info(fmt.Sprintf("⚠️ FTS5 init warning: %v", err))
		}
	}

	// Unified cursor index for keyset pagination (P3)
	if _, err := s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_emails_cursor ON emails (is_pinned DESC, date_sent DESC, id DESC);`); err != nil {
		errStr := err.Error()
		if !strings.Contains(errStr, "already exists") {
			slog.Info(fmt.Sprintf("⚠️ cursor index warning: %v", err))
		}
	}

	// SQLite doesn't support ALTER TABLE ADD COLUMN IF NOT EXISTS.
	// Manually add missing columns that the schema SQL may have skipped.
	s.addColumnIfMissing(ctx, "folders", "is_inbox", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "folders", "name_lower", "TEXT")
	s.addColumnIfMissing(ctx, "folders", "uid_validity", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "accounts", "is_manual", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "emails", "cc_address", "TEXT DEFAULT ''")
	s.addColumnIfMissing(ctx, "emails", "status", "TEXT DEFAULT 'new'")
	s.addColumnIfMissing(ctx, "emails", "first_response_at", "TEXT")
	s.addColumnIfMissing(ctx, "emails", "resolved_at", "TEXT")
	s.addColumnIfMissing(ctx, "emails", "smart_category", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "emails", "is_answered", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "accounts", "is_gmail", "INTEGER DEFAULT 0")
	s.addColumnIfMissing(ctx, "imap_move_queue", "source_folder_name", "TEXT DEFAULT ''")
	// Ensure name_lower index and is_inbox index exist
	s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_folders_name_lower ON folders (account_id, name_lower)`)
	s.db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_folders_is_inbox ON folders (is_inbox)`)
	// Populate name_lower and is_inbox for existing rows
	s.db.ExecContext(ctx, `UPDATE folders SET name_lower = LOWER(name) WHERE name_lower IS NULL`)
	s.db.ExecContext(ctx, `UPDATE folders SET is_inbox = 1 WHERE LOWER(name) = 'inbox' AND is_inbox = 0`)
	s.db.ExecContext(ctx, `UPDATE emails SET smart_category = 0 WHERE smart_category IS NULL`)

	return nil
}

// RunMigrations executes all embedded SQL migration files in sorted order.
// Uses a tracking table for idempotency. Returns the number of migrations applied.
func (s *Storage) RunMigrations(ctx context.Context) (int, error) {
	files, err := migrations.ListSQLFiles()
	if err != nil {
		return 0, nil
	}
	files = migrations.FilesForSQLite(files)

	// Create migrations tracking table
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		filename TEXT PRIMARY KEY,
		applied_at TEXT DEFAULT (datetime('now'))
	)`); err != nil {
		return 0, err
	}

	if backfilled, err := s.backfillLegacySQLiteMigrations(ctx, files); err != nil {
		return 0, fmt.Errorf("legacy migration backfill: %w", err)
	} else if backfilled > 0 {
		slog.Info("legacy migrations backfilled", "count", backfilled)
	}

	applied := 0
	for _, f := range files {
		var exists int
		if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE filename = ?", f).Scan(&exists); err != nil {
			return applied, err
		}
		if exists > 0 {
			continue
		}

		sqlBytes, err := migrations.FS.ReadFile("migrations/" + f)
		if err != nil {
			return applied, fmt.Errorf("read migration %s: %w", f, err)
		}

		// SQLite: split by ; (no dollar-quoting to worry about in migrations)
		for _, stmt := range strings.Split(string(sqlBytes), ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := s.db.ExecContext(ctx, stmt); err != nil {
				if isBenignSQLiteMigrationError(err) {
					continue
				}
				return applied, fmt.Errorf("migration %s: %w (SQL: %.100s)", f, err, stmt)
			}
		}

		if _, err := s.db.ExecContext(ctx, "INSERT INTO schema_migrations (filename) VALUES (?)", f); err != nil {
			return applied, fmt.Errorf("record migration %s: %w", f, err)
		}
		slog.Info("migration applied", "file", f)
		applied++
	}
	s.ensureMonoEmailColumns(ctx)
	return applied, nil
}

// ensureMonoEmailColumns repairs columns that embedded migrations may have skipped
// (e.g. ADD COLUMN IF NOT EXISTS is unsupported on some SQLite builds and is treated as benign).
func (s *Storage) addColumnIfMissing(ctx context.Context, table, column, colType string) {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, column).Scan(&count)
	if err != nil || count > 0 {
		return
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, colType))
	if err != nil {
		slog.Info(fmt.Sprintf("⚠️ addColumnIfMissing %s.%s: %v", table, column, err))
	}
}

// ReindexFTS rebuilds the FTS5 index from the emails table.
// Safe to call on startup — only rebuilds if FTS is empty and emails exist.
func (s *Storage) Close() error {
	return s.db.Close()
}

// ClearSyncErrors clears all stale sync errors from accounts.
func (s *Storage) ClearSyncErrors(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET last_sync_error = '' WHERE last_sync_error != ''`)
	return err
}

// ============================================================================
// Encryption helpers
// ============================================================================

func (s *Storage) UpdateSmartCategories(ctx context.Context, accountID string, enabled bool) error {
	val := 0
	if enabled {
		val = 1
	}
	query := "UPDATE accounts SET smart_categories = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?"
	_, err := s.db.ExecContext(ctx, query, val, accountID)
	return err
}

func (s *Storage) GetIdentities(ctx context.Context, accountID string) ([]models.Identity, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, email, COALESCE(name, '') FROM identities WHERE account_id = ? ORDER BY email`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []models.Identity
	for rows.Next() {
		var i models.Identity
		if err := rows.Scan(&i.ID, &i.AccountID, &i.Email, &i.Name); err != nil {
			return nil, err
		}
		ids = append(ids, i)
	}
	return ids, rows.Err()
}

func (s *Storage) SenderProfileValid(ctx context.Context, email string) bool {
	var resolvedAt sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT resolved_at FROM sender_profiles WHERE email = ?", email).Scan(&resolvedAt)
	if err != nil {
		return false
	}
	if !resolvedAt.Valid {
		return false
	}
	t := parseTime(resolvedAt)
	return time.Since(t) < 30*24*time.Hour
}

func (s *Storage) UpsertSenderProfile(ctx context.Context, email, name, avatarURL string) error {
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sender_profiles (email, name, avatar_url, resolved_at, updated_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (email) DO UPDATE SET name = excluded.name, avatar_url = excluded.avatar_url, resolved_at = excluded.resolved_at, updated_at = excluded.updated_at`,
		email, name, avatarURL, now, now)
	return err
}

func (s *Storage) AdminExists(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM admins").Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Storage) CreateAdmin(ctx context.Context, email, passwordHash string) (string, error) {
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO admins (id, email, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		id, email, passwordHash, now)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Storage) UpdateAdminPassword(ctx context.Context, email, passwordHash string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE admins SET password_hash = ? WHERE email = ?", passwordHash, email)
	return err
}

func (s *Storage) EnqueueJob(ctx context.Context, jobType string, payload string, runAt time.Time) error {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, payload, next_run_at)
		VALUES (?, ?, ?, ?)
	`, id, jobType, payload, runAt.Format(time.RFC3339))
	return err
}

func (s *Storage) DequeueJobs(ctx context.Context, limit int) ([]models.Job, error) {
	// Simple polling
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, payload, status, attempt, next_run_at, created_at, updated_at
		FROM jobs
		WHERE status = 'pending' AND next_run_at <= datetime('now')
		ORDER BY next_run_at ASC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var j models.Job
		var nextRun, created, updated string
		if err := rows.Scan(&j.ID, &j.Type, &j.Payload, &j.Status, &j.Attempt, &nextRun, &created, &updated); err != nil {
			continue
		}
		j.NextRunAt, _ = time.Parse(time.RFC3339, nextRun)
		j.CreatedAt, _ = time.Parse(time.RFC3339, created)
		j.UpdatedAt, _ = time.Parse(time.RFC3339, updated)
		jobs = append(jobs, j)
	}

	// Mark as processing
	for _, j := range jobs {
		s.db.ExecContext(ctx, "UPDATE jobs SET status = 'processing', updated_at = datetime('now') WHERE id = ?", j.ID)
	}

	return jobs, nil
}

func (s *Storage) CompleteJob(ctx context.Context, jobID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM jobs WHERE id = ?", jobID)
	return err
}

func (s *Storage) FailJob(ctx context.Context, jobID string, errMessage string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE jobs SET status = 'pending', attempt = attempt + 1,
		next_run_at = datetime('now', '+' || (1 << attempt) || ' seconds'),
		updated_at = datetime('now')
		WHERE id = ?
	`, jobID)
	return err
}

// ============================================================================
// Webhooks
// ============================================================================

func (s *Storage) RekeyAll(ctx context.Context) (int, error) {
	count := 0

	// 1. Accounts
	rows, err := s.db.QueryContext(ctx, "SELECT id, password_encrypted FROM accounts WHERE password_encrypted != ''")
	if err != nil {
		return count, fmt.Errorf("query accounts: %w", err)
	}
	defer rows.Close()

	type row struct {
		id string
		pw string
	}
	var toUpdate []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.pw); err != nil {
			continue
		}
		plain, err := s.decryptPassword(r.pw, "imap_password")
		if err != nil || plain == "" || plain == r.pw {
			continue
		}
		reEnc, err := s.encryptPassword(plain, "imap_password")
		if err != nil || reEnc == r.pw {
			continue
		}
		toUpdate = append(toUpdate, row{r.id, reEnc})
	}
	for _, r := range toUpdate {
		if _, err := s.db.ExecContext(ctx, "UPDATE accounts SET password_encrypted = ? WHERE id = ?", r.pw, r.id); err != nil {
			slog.Warn("rekey: failed to update account", "id", r.id, "error", err)
			continue
		}
		count++
	}

	// 2. MCP keys
	rows2, err := s.db.QueryContext(ctx, "SELECT id, key_encrypted FROM mcp_keys WHERE key_encrypted != ''")
	if err != nil {
		return count, fmt.Errorf("query mcp_keys: %w", err)
	}
	defer rows2.Close()

	type mcpRow struct {
		id  string
		key string
	}
	var mcpToUpdate []mcpRow
	for rows2.Next() {
		var r mcpRow
		if err := rows2.Scan(&r.id, &r.key); err != nil {
			continue
		}
		plain, err := s.decryptPassword(r.key, "mcp_key")
		if err != nil || plain == "" || plain == r.key {
			continue
		}
		reEnc, err := s.encryptPassword(plain, "mcp_key")
		if err != nil || reEnc == r.key {
			continue
		}
		mcpToUpdate = append(mcpToUpdate, mcpRow{r.id, reEnc})
	}
	for _, r := range mcpToUpdate {
		if _, err := s.db.ExecContext(ctx, "UPDATE mcp_keys SET key_encrypted = ? WHERE id = ?", r.key, r.id); err != nil {
			slog.Warn("rekey: failed to update mcp_key", "id", r.id, "error", err)
			continue
		}
		count++
	}

	return count, nil
}

func (s *Storage) GetGmailLabels(ctx context.Context, emailID, accountID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT label FROM email_labels_junction WHERE email_id = ? AND account_id = ?`, emailID, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var labels []string
	for rows.Next() {
		var label string
		if err := rows.Scan(&label); err != nil {
			return nil, err
		}
		labels = append(labels, label)
	}
	return labels, rows.Err()
}

// BackfillGmailLabels creates email_labels_junction entries for all existing Gmail
// emails based on their current folder_id. One-time operation after is_gmail is set.
func (s *Storage) BackfillGmailLabels(ctx context.Context, accountID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO email_labels_junction (email_id, account_id, label, system)
		SELECT e.id, e.account_id, COALESCE(f.path, ''), 0
		FROM emails e
		JOIN folders f ON e.folder_id = f.id
		WHERE e.account_id = ? AND e.msg_id != '' AND COALESCE(f.path, '') != ''
		AND e.id NOT IN (SELECT email_id FROM email_labels_junction WHERE account_id = ?)
	`, accountID, accountID)
	if err != nil {
		return err
	}
	n, _ := s.db.ExecContext(ctx, `SELECT COUNT(*) FROM email_labels_junction WHERE account_id = ?`, accountID)
	slog.Info("Gmail: backfilled labels from folder_id", "accountID", accountID)
	_ = n
	return nil
}

// CleanupGmailDuplicates merges duplicate email rows for Gmail accounts.
// Emails with the same (msg_id, account_id) but different folder_ids are merged:
// the row with the most fields kept, folder_ids collected as labels in the junction.
func (s *Storage) CleanupGmailDuplicates(ctx context.Context, accountID string) (int, error) {
	var removed int
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil { return err }
		defer tx.Rollback()

		// Find duplicate groups: msg_ids that appear more than once for this account
		rows, err := tx.QueryContext(ctx,
			`SELECT msg_id FROM emails WHERE account_id = ? AND msg_id != '' GROUP BY msg_id HAVING COUNT(*) > 1`, accountID)
		if err != nil { return err }
		defer rows.Close()

		var dupMsgIDs []string
		for rows.Next() {
			var msgID string
			if err := rows.Scan(&msgID); err != nil { return err }
			dupMsgIDs = append(dupMsgIDs, msgID)
		}
		rows.Close()

		for _, msgID := range dupMsgIDs {
			// Find the best row to keep (one with most non-empty fields)
			dupRows, err := tx.QueryContext(ctx,
				`SELECT id, folder_id, uid, subject, snippet, body_path FROM emails WHERE msg_id = ? AND account_id = ?`, msgID, accountID)
			if err != nil { return err }

			var (
				bestID      string
				bestScore   int
				labels      []string
				deleteIDs   []string
			)
			for dupRows.Next() {
				var id, folderID, subject, snippet, bodyPath string
				var uid int32
				if err := dupRows.Scan(&id, &folderID, &uid, &subject, &snippet, &bodyPath); err != nil { return err }
				score := 0
				if bodyPath != "" { score += 3 }
				if subject != "" { score += 2 }
				if uid > 0 { score++ }
				if score > bestScore || bestID == "" {
					if bestID != "" { deleteIDs = append(deleteIDs, bestID) }
					bestID = id
					bestScore = score
				} else {
					deleteIDs = append(deleteIDs, id)
				}
			}
			dupRows.Close()

			if bestID == "" { continue }

			// Collect all folder_ids as labels from ALL rows
			folderRows, _ := tx.QueryContext(ctx,
				`SELECT DISTINCT f.path FROM emails e JOIN folders f ON e.folder_id = f.id WHERE e.msg_id = ? AND e.account_id = ? AND f.path != ''`, msgID, accountID)
			if folderRows != nil {
				for folderRows.Next() {
					var path string
					if folderRows.Scan(&path) == nil { labels = append(labels, path) }
				}
				folderRows.Close()
			}

			// Delete duplicate rows
			for _, did := range deleteIDs {
				tx.ExecContext(ctx, "DELETE FROM emails_fts WHERE email_id = ?", did)
				tx.ExecContext(ctx, "DELETE FROM emails WHERE id = ? AND account_id = ?", did, accountID)
				removed++
			}

			// Insert labels for survivor
			if len(labels) > 0 {
				tx.ExecContext(ctx, "DELETE FROM email_labels_junction WHERE email_id = ? AND account_id = ?", bestID, accountID)
				for _, label := range labels {
					tx.ExecContext(ctx,
						`INSERT OR IGNORE INTO email_labels_junction (email_id, account_id, label, system) VALUES (?, ?, ?, 0)`,
						bestID, accountID, label)
				}
			}
		}
		return tx.Commit()
	})
	return removed, err
}

func (s *Storage) UpsertEmailLabels(ctx context.Context, emailID, accountID string, labels []string) error {
	return s.withWriteRetryLow(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, `DELETE FROM email_labels_junction WHERE email_id = ? AND account_id = ?`, emailID, accountID); err != nil {
			return err
		}
		for _, label := range labels {
			sys := 0
			if isGmailSystemLabel(label) {
				sys = 1
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO email_labels_junction (email_id, account_id, label, system) VALUES (?, ?, ?, ?)`,
				emailID, accountID, label, sys); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

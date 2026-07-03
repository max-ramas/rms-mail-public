package sqlite

import (
	"context"
	"database/sql"
	"log/slog"
	"rmsmail/internal/models"
	"strings"
	"time"
)

func (s *Storage) EnqueueUIDs(ctx context.Context, accountID, folderName string, uids []uint32, priority int) error {
	if len(uids) == 0 {
		return nil
	}

	return s.withWriteRetryLow(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO email_sync_queue (account_id, folder_name, uid, priority, status)
		VALUES (?, ?, ?, ?, 'pending')
		ON CONFLICT (account_id, folder_name, uid)
		DO UPDATE SET priority = MAX(email_sync_queue.priority, excluded.priority), status = 'pending', updated_at = CURRENT_TIMESTAMP
		WHERE email_sync_queue.status != 'completed'
	`)
		if err != nil {
			return err
		}
		defer stmt.Close()

		for _, uid := range uids {
			_, err = stmt.ExecContext(ctx, accountID, folderName, uid, priority)
			if err != nil {
				return err
			}
		}

		return tx.Commit()
	})
}

func (s *Storage) DequeueUIDs(ctx context.Context, accountID string, limit int) ([]models.SyncTask, error) {
	// SQLite supports UPDATE ... RETURNING since 3.35.0.
	// As there is no concurrent contention per account, a standard subquery is safe.
	query := `
		UPDATE email_sync_queue
		SET status = 'processing', updated_at = CURRENT_TIMESTAMP, attempts = attempts + 1
		WHERE id IN (
			SELECT id FROM email_sync_queue
			WHERE account_id = ? AND (status = 'pending' OR (status = 'processing' AND updated_at < datetime('now', '-5 minutes')))
			ORDER BY priority DESC, created_at ASC
			LIMIT ?
		)
		RETURNING id, account_id, folder_name, uid, priority, status, attempts, created_at, updated_at
	`
	rows, err := s.db.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.SyncTask
	for rows.Next() {
		var t models.SyncTask
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&t.ID, &t.AccountID, &t.FolderName, &t.UID, &t.Priority, &t.Status, &t.Attempts, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt.String)
		}
		if updatedAt.Valid {
			t.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt.String)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Storage) CompleteSyncTask(ctx context.Context, taskID int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE email_sync_queue SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id = ?", taskID)
	return err
}

func (s *Storage) CompleteSyncTasks(ctx context.Context, taskIDs []int64) error {
	if len(taskIDs) == 0 {
		return nil
	}
	placeholders := make([]string, len(taskIDs))
	args := make([]interface{}, len(taskIDs))
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "UPDATE email_sync_queue SET status = 'completed', updated_at = CURRENT_TIMESTAMP WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *Storage) FailSyncTask(ctx context.Context, taskID int64, errReason string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE email_sync_queue
		SET status = 'failed', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, taskID)
	if err != nil {
		return err
	}

	var accountID, folderName string
	var uid uint32
	var attempts int
	err = s.db.QueryRowContext(ctx,
		"SELECT account_id, folder_name, uid, attempts FROM email_sync_queue WHERE id = ?", taskID,
	).Scan(&accountID, &folderName, &uid, &attempts)
	if err != nil {
		return err
	}

	if attempts >= 5 {
		slog.Error("[QUEUE CRITICAL] Task permanently failed after 5 attempts",
			"task_id", taskID,
			"account_id", accountID,
			"folder", folderName,
			"uid", uid,
			"error", errReason)
	}
	return nil
}

func (s *Storage) RemoveSyncTaskByUID(ctx context.Context, accountID, folderName string, uid uint32) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM email_sync_queue WHERE account_id = ? AND folder_name = ? AND uid = ?", accountID, folderName, uid)
	return err
}

func (s *Storage) ClearFolderQueue(ctx context.Context, accountID, folderName string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM email_sync_queue WHERE account_id = ? AND folder_name = ?", accountID, folderName)
	return err
}

func (s *Storage) ProcessQueueRetries(ctx context.Context) (int64, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, attempts, updated_at FROM email_sync_queue WHERE status = 'failed' AND attempts < 5")
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	var idsToUpdate []interface{}
	var placeholders []string

	for rows.Next() {
		var id int64
		var attempts int
		var updatedAt sql.NullString
		if err := rows.Scan(&id, &attempts, &updatedAt); err != nil {
			return 0, err
		}

		if updatedAt.Valid {
			// Try parsing as ISO8601/RFC3339 first (standard in Go/SQLite usage sometimes), then fallback to standard SQLite format
			t, err := time.Parse(time.RFC3339, updatedAt.String)
			if err != nil {
				t, err = time.Parse("2006-01-02 15:04:05", updatedAt.String)
			}
			if err == nil {
				// Base backoff is 30 seconds
				backoff := time.Duration(30*(1<<(attempts-1))) * time.Second
				if time.Since(t) >= backoff {
					idsToUpdate = append(idsToUpdate, id)
					placeholders = append(placeholders, "?")
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	if len(idsToUpdate) == 0 {
		return 0, nil
	}

	query := "UPDATE email_sync_queue SET status = 'pending', updated_at = CURRENT_TIMESTAMP WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	res, err := s.db.ExecContext(ctx, query, idsToUpdate...)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (s *Storage) CleanQueueGarbage(ctx context.Context, retentionCompleted time.Duration, retentionFailed time.Duration) (int64, error) {
	query := `
		DELETE FROM email_sync_queue
		WHERE (status = 'completed' AND updated_at < datetime('now', '-' || ? || ' seconds'))
		   OR (status = 'failed' AND attempts >= 5 AND updated_at < datetime('now', '-' || ? || ' seconds'))
	`
	res, err := s.db.ExecContext(ctx, query, int(retentionCompleted.Seconds()), int(retentionFailed.Seconds()))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

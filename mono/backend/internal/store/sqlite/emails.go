package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

// InitSchema executes the SQLite schema DDL.

func (s *Storage) ensureMonoEmailColumns(ctx context.Context) {
	s.addColumnIfMissing(ctx, "emails", "is_answered", "INTEGER DEFAULT 0")
}

const fts5Schema = `CREATE VIRTUAL TABLE IF NOT EXISTS emails_fts USING fts5(
    email_id,
    account_id,
    subject,
    sender_name,
    sender_address,
    recipient_address,
    body,
    content=''
);`

// addColumnIfMissing adds a column to a table if it doesn't already exist.
// Works around SQLite's lack of ALTER TABLE ADD COLUMN IF NOT EXISTS.

func (s *Storage) ReindexFTS(ctx context.Context) error {
	var ftsCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emails_fts").Scan(&ftsCount); err != nil {
		return nil // FTS table may not exist yet on fresh install
	}
	if ftsCount > 0 {
		return nil // Already indexed
	}

	var emailCount int
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emails").Scan(&emailCount); err != nil || emailCount == 0 {
		return nil
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO emails_fts (email_id, account_id, subject, sender_name, sender_address, recipient_address, body)
		 SELECT id, COALESCE(account_id, ''), subject, sender_name, sender_address, recipient_address, snippet FROM emails`)
	return err
}

// Close closes the database connection.

func (s *Storage) SaveEmail(ctx context.Context, email models.Email) error {
	return s.withWriteRetryLow(ctx, func(ctx context.Context) error {
		if email.ThreadID == "" {
			email.ThreadID = email.ID
		}

		query := `INSERT INTO emails (id, account_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, body_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (msg_id, account_id) DO UPDATE SET is_read = CASE WHEN emails.is_dirty_locally THEN emails.is_read ELSE excluded.is_read END, is_dirty_locally = excluded.is_dirty_locally, has_attachments = excluded.has_attachments, in_reply_to = excluded.in_reply_to, thread_id = excluded.thread_id, subject = excluded.subject, sender_name = excluded.sender_name, sender_address = excluded.sender_address, recipient_address = excluded.recipient_address, cc_address = excluded.cc_address, date_sent = excluded.date_sent`

		_, err := s.db.ExecContext(ctx, query,
			email.ID, email.AccountID, email.MsgID, email.UID, email.Subject,
			email.SenderName, email.SenderAddress, email.RecipientAddress, email.CcAddress, formatTime(email.DateSent),
			boolToInt(email.IsRead), boolToInt(email.HasAttachments), boolToInt(email.IsDirtyLocally),
			email.InReplyTo, email.ThreadID, email.DraftReply, email.DraftRemoteUID,
			email.Snippet, email.BodyPath,
		)
		return err
	})
}

func (s *Storage) SaveEmailToFolder(ctx context.Context, email models.Email, folderID string) (bool, error) {
	var isNew bool
	err := s.withWriteRetryLow(ctx, func(ctx context.Context) error {
		if email.ThreadID == "" {
			email.ThreadID = email.ID
		}

		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		var existingID string
		row := tx.QueryRowContext(ctx,
			`SELECT id FROM emails WHERE msg_id = ? AND account_id = ? AND folder_id = ?`,
			email.MsgID, email.AccountID, folderID)
		isNew = row.Scan(&existingID) == sql.ErrNoRows

		_, err = tx.ExecContext(ctx,
			`INSERT INTO emails (id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, body_path)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (msg_id, account_id, folder_id) DO UPDATE SET is_read = CASE WHEN emails.is_dirty_locally THEN emails.is_read ELSE excluded.is_read END, is_dirty_locally = excluded.is_dirty_locally, has_attachments = excluded.has_attachments, in_reply_to = excluded.in_reply_to, thread_id = excluded.thread_id, subject = excluded.subject, sender_name = excluded.sender_name, sender_address = excluded.sender_address, recipient_address = excluded.recipient_address, cc_address = excluded.cc_address, date_sent = excluded.date_sent, uid = excluded.uid, folder_id = excluded.folder_id, snippet = excluded.snippet, body_path = excluded.body_path`,
			email.ID, email.AccountID, folderID, email.MsgID, email.UID, email.Subject,
			email.SenderName, email.SenderAddress, email.RecipientAddress, email.CcAddress, formatTime(email.DateSent),
			boolToInt(email.IsRead), boolToInt(email.HasAttachments), boolToInt(email.IsDirtyLocally),
			email.InReplyTo, email.ThreadID, email.DraftReply, email.DraftRemoteUID,
			email.Snippet, email.BodyPath,
		)
		if err != nil {
			return err
		}

		if isNew && !email.IsRead {
			_, err = tx.ExecContext(ctx,
				`UPDATE folders SET unread_count = MAX(unread_count + 1, 0) WHERE id = ?`,
				folderID)
			if err != nil {
				return err
			}
		}

		return tx.Commit()
	})
	return isNew, err
}

func (s *Storage) GetEmails(ctx context.Context, unified bool, accountID string, folderID string, folderName string, offset int, limit int, filter models.EmailFilterOpts) ([]models.Email, error) {
	cols := "id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at"

	var query string
	var args []interface{}

	if unified {
		if folderName == "" {
			folderName = "INBOX"
		}
		unifiedCols := "e.id, e.account_id, e.folder_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address, e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally, e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, COALESCE(e.spf_pass,0), COALESCE(e.dkim_pass,0), e.is_pinned, e.snooze_until, e.is_muted, e.status, e.first_response_at, e.resolved_at, e.created_at"
		var folderFilter string
		if strings.EqualFold(folderName, "INBOX") {
			folderFilter = "e.folder_id IN (SELECT id FROM folders WHERE is_inbox = 1)"
		} else {
			folderFilter = "e.folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?))"
			args = append(args, folderName)
		}
		query = `SELECT ` + unifiedCols + `
			         FROM emails e
			         WHERE ` + folderFilter + ` AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now'))`

		if strings.EqualFold(folderName, "INBOX") {
			query += ` AND (e.smart_category = 0)`
		}
		if filter.Unread {
			query += " AND e.is_read = 0 AND e.is_muted = 0"
		}
		if filter.Flagged {
			query += " AND e.is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND e.has_attachments = 1"
		}
		if filter.Search != "" {
			query += " AND (e.subject LIKE ? OR e.sender_name LIKE ? OR e.sender_address LIKE ? OR e.recipient_address LIKE ? OR e.snippet LIKE ?)"
			args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
		}
		if filter.LabelID != "" {
			query += " AND e.id IN (SELECT email_id FROM email_labels WHERE label_id = ?)"
			args = append(args, filter.LabelID)
		}
		if filter.Tag != "" {
			query += " AND e.id IN (SELECT email_id FROM email_tags WHERE tag = ?)"
			args = append(args, filter.Tag)
		}
		query += " GROUP BY e.id ORDER BY e.is_pinned DESC, e.date_sent DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	} else if folderID != "" {
		unifiedCols := "e.id, e.account_id, e.folder_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address, e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally, e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, COALESCE(e.spf_pass,0), COALESCE(e.dkim_pass,0), e.is_pinned, e.snooze_until, e.is_muted, e.status, e.first_response_at, e.resolved_at, e.created_at"
		query = "SELECT " + unifiedCols + " FROM emails e JOIN folders f ON e.folder_id = f.id WHERE e.account_id = ? AND e.folder_id = ? AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now'))"

		query += ` AND (f.name_lower != 'inbox' OR e.smart_category = 0)`

		args = append(args, accountID, folderID)
		if filter.Unread {
			query += " AND e.is_read = 0 AND e.is_muted = 0"
		}
		if filter.Flagged {
			query += " AND e.is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND e.has_attachments = 1"
		}
		if filter.Search != "" {
			query += " AND (e.subject LIKE ? OR e.sender_name LIKE ? OR e.sender_address LIKE ? OR e.recipient_address LIKE ? OR e.snippet LIKE ?)"
			args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
		}
		if filter.LabelID != "" {
			query += " AND e.id IN (SELECT email_id FROM email_labels WHERE label_id = ?)"
			args = append(args, filter.LabelID)
		}
		if filter.Tag != "" {
			query += " AND e.id IN (SELECT email_id FROM email_tags WHERE tag = ?)"
			args = append(args, filter.Tag)
		}
		query += " ORDER BY e.date_sent DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	} else if accountID != "" {
		query = "SELECT " + cols + " FROM emails WHERE account_id = ? AND (snooze_until IS NULL OR snooze_until <= datetime('now'))"
		args = append(args, accountID)
		if filter.Unread {
			query += " AND is_read = 0 AND is_muted = 0"
		}
		if filter.Flagged {
			query += " AND is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND has_attachments = 1"
		}
		if filter.Search != "" {
			query += " AND (subject LIKE ? OR sender_name LIKE ? OR sender_address LIKE ? OR recipient_address LIKE ? OR snippet LIKE ?)"
			args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
		}
		if filter.LabelID != "" {
			query += " AND id IN (SELECT email_id FROM email_labels WHERE label_id = ?)"
			args = append(args, filter.LabelID)
		}
		if filter.Tag != "" {
			query += " AND id IN (SELECT email_id FROM email_tags WHERE tag = ?)"
			args = append(args, filter.Tag)
		}
		query += " ORDER BY date_sent DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	} else {
		query = "SELECT " + cols + " FROM emails WHERE (snooze_until IS NULL OR snooze_until <= datetime('now'))"
		if filter.Unread {
			query += " AND is_read = 0 AND is_muted = 0"
		}
		if filter.Flagged {
			query += " AND is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND has_attachments = 1"
		}
		if filter.Search != "" {
			query += " AND (subject LIKE ? OR sender_name LIKE ? OR sender_address LIKE ? OR recipient_address LIKE ? OR snippet LIKE ?)"
			args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
		}
		if filter.LabelID != "" {
			query += " AND id IN (SELECT email_id FROM email_labels WHERE label_id = ?)"
			args = append(args, filter.LabelID)
		}
		if filter.Tag != "" {
			query += " AND id IN (SELECT email_id FROM email_tags WHERE tag = ?)"
			args = append(args, filter.Tag)
		}
		query += " ORDER BY date_sent DESC LIMIT ? OFFSET ?"
		args = append(args, limit, offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
		err := rows.Scan(
			&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		if firstRespAt.Valid {
			t := parseTime(firstRespAt)
			e.FirstResponseAt = &t
		}
		if resolvedAt.Valid {
			t := parseTime(resolvedAt)
			e.ResolvedAt = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) GetEmailsByAccounts(ctx context.Context, accountIDs []string, folderName string, offset, limit int, filter models.EmailFilterOpts) ([]models.Email, error) {
	if len(accountIDs) == 0 {
		return []models.Email{}, nil
	}
	if folderName == "" {
		folderName = "INBOX"
	}
	cols := "e.id, e.account_id, e.folder_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address, e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally, e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, COALESCE(e.spf_pass,0), COALESCE(e.dkim_pass,0), e.is_pinned, e.snooze_until, e.is_muted, e.status, e.first_response_at, e.resolved_at, e.created_at"
	// Build IN (?, ?, ...) for SQLite
	placeholders := make([]string, len(accountIDs))
	args := make([]interface{}, 0, len(accountIDs)+6)
	for i, id := range accountIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, folderName)
	folderFilter := "e.folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?))"
	if strings.EqualFold(folderName, "INBOX") {
		folderFilter = "e.folder_id IN (SELECT id FROM folders WHERE is_inbox = 1)"
	}
	query := `SELECT ` + cols + `
	         FROM emails e
	         WHERE e.account_id IN (` + strings.Join(placeholders, ",") + `) AND ` + folderFilter + ` AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now'))`
	if filter.Unread {
		query += " AND e.is_read = 0 AND e.is_muted = 0"
	}
	if filter.Flagged {
		query += " AND e.is_flagged = 1"
	}
	if filter.Attachments {
		query += " AND e.has_attachments = 1"
	}
	if filter.Search != "" {
		query += " AND (e.subject LIKE ? OR e.sender_name LIKE ? OR e.sender_address LIKE ? OR e.recipient_address LIKE ? OR e.snippet LIKE ?)"
		args = append(args, "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%", "%"+filter.Search+"%")
	}
	query += " ORDER BY e.is_pinned DESC, e.date_sent DESC"
	query += " LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
		err := rows.Scan(
			&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		if firstRespAt.Valid {
			t := parseTime(firstRespAt)
			e.FirstResponseAt = &t
		}
		if resolvedAt.Valid {
			t := parseTime(resolvedAt)
			e.ResolvedAt = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) GetEmail(ctx context.Context, emailID string, accountID string) (*models.Email, error) {
	query := "SELECT id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, body_path, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at FROM emails WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, emailID)
	var e models.Email
	var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
	err := row.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
		&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
		&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
		&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.BodyPath, &e.SpfPass, &e.DkimPass,
		&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	e.DateSent = parseTime(dateSent)
	if snoozeUntil.Valid {
		t := parseTime(snoozeUntil)
		e.SnoozeUntil = &t
	}
	if firstRespAt.Valid {
		t := parseTime(firstRespAt)
		e.FirstResponseAt = &t
	}
	if resolvedAt.Valid {
		t := parseTime(resolvedAt)
		e.ResolvedAt = &t
	}
	e.CreatedAt = parseTime(createdAt)
	return &e, nil
}

func (s *Storage) GetEmailsByIDs(ctx context.Context, ids []string) ([]models.Email, error) {
	if len(ids) == 0 {
		return []models.Email{}, nil
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	query := "SELECT id, account_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at FROM emails WHERE id IN (" + strings.Join(placeholders, ",") + ")"

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
		err := rows.Scan(&e.ID, &e.AccountID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		if firstRespAt.Valid {
			t := parseTime(firstRespAt)
			e.FirstResponseAt = &t
		}
		if resolvedAt.Valid {
			t := parseTime(resolvedAt)
			e.ResolvedAt = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) SearchEmails(ctx context.Context, q string, accountID string, limit int) ([]models.Email, error) {
	pattern := "%" + q + "%"
	cols := "id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at"
	query := "SELECT " + cols + " FROM emails WHERE (subject LIKE ? OR sender_name LIKE ? OR sender_address LIKE ? OR recipient_address LIKE ? OR snippet LIKE ?)"
	args := []interface{}{pattern, pattern, pattern, pattern, pattern}

	if accountID != "" {
		query += " AND account_id = ?"
		args = append(args, accountID)
	}
	query += " ORDER BY date_sent DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
		err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		if firstRespAt.Valid {
			t := parseTime(firstRespAt)
			e.FirstResponseAt = &t
		}
		if resolvedAt.Valid {
			t := parseTime(resolvedAt)
			e.ResolvedAt = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func sanitizeFTSQuery(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return ""
	}
	replacer := strings.NewReplacer(
		"*", " ",
		"\"", " ",
		"'", " ",
		"-", " ",
		"(", " ",
		")", " ",
		":", " ",
		"NEAR/", " ",
		"@", " ",
		".", " ",
		"<", " ",
		">", " ",
		"_", " ",
	)
	clean := replacer.Replace(q)
	words := strings.Fields(clean)
	if len(words) == 0 {
		return ""
	}
	return strings.Join(words, " ")
}

func (s *Storage) IndexEmailFTS(ctx context.Context, emailID, accountID, subject, senderName, senderAddress, recipientAddress, body string) error {
	// FTS5 virtual table: no UNIQUE/PRIMARY KEY, so DELETE+INSERT.
	// Each statement auto-commits — no explicit transaction needed.
	_, err := s.db.ExecContext(ctx, "DELETE FROM emails_fts WHERE email_id = ?", emailID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO emails_fts (email_id, account_id, subject, sender_name, sender_address, recipient_address, body) VALUES (?, ?, ?, ?, ?, ?, ?)",
		emailID, accountID, subject, senderName, senderAddress, recipientAddress, body,
	)
	return err
}

func (s *Storage) SearchFTS(ctx context.Context, q, accountID string, limit int) ([]string, error) {
	clean := sanitizeFTSQuery(q)
	if clean == "" {
		return nil, nil
	}
	var query string
	var args []interface{}
	if accountID != "" {
		query = "SELECT email_id FROM emails_fts WHERE account_id = ? AND emails_fts MATCH ? ORDER BY rank LIMIT ?"
		args = []interface{}{accountID, clean, limit}
	} else {
		query = "SELECT email_id FROM emails_fts WHERE emails_fts MATCH ? ORDER BY rank LIMIT ?"
		args = []interface{}{clean, limit}
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Storage) DeleteEmail(ctx context.Context, id string, accountID string) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		_, err = tx.ExecContext(ctx, "DELETE FROM emails_fts WHERE email_id = ?", id)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, "DELETE FROM email_labels_junction WHERE email_id = ? AND account_id = ?", id, accountID)
		if err != nil {
			return err
		}
		if accountID != "" {
			_, err = tx.ExecContext(ctx, "DELETE FROM emails WHERE id = ? AND account_id = ?", id, accountID)
		} else {
			_, err = tx.ExecContext(ctx, "DELETE FROM emails WHERE id = ?", id)
		}
		if err != nil {
			return err
		}
		return tx.Commit()
	})
}

func (s *Storage) GetEmailIDByFolderUID(ctx context.Context, accountID, folderPath string, uid uint32) (string, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id FROM emails e
		INNER JOIN folders f ON e.folder_id = f.id
		WHERE e.account_id = ? AND f.path = ? AND e.uid = ?
		LIMIT 1`, accountID, folderPath, uid).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *Storage) DeleteEmailsInFolder(ctx context.Context, folderID string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM emails_fts WHERE email_id IN (SELECT id FROM emails WHERE folder_id = ?)", folderID)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, "DELETE FROM emails WHERE folder_id = ?", folderID)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Storage) UpdateEmailHasAttachments(ctx context.Context, emailID string, accountID string, has bool) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET has_attachments = ? WHERE id = ? AND account_id = ?", boolToInt(has), emailID, accountID)
	return err
}

func (s *Storage) MarkEmailRead(ctx context.Context, emailID string, accountID string) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			slog.Error("MarkEmailRead: begin tx failed", "error", err, "emailID", emailID)
			return err
		}
		defer tx.Rollback()

		var folderID string
		err = tx.QueryRowContext(ctx,
			"SELECT folder_id FROM emails WHERE id = ? AND account_id = ? AND is_read = 0",
			emailID, accountID).Scan(&folderID)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			slog.Error("MarkEmailRead: query failed", "error", err, "emailID", emailID, "accountID", accountID)
			return err
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE emails SET is_read = 1, is_dirty_locally = 1 WHERE id = ?", emailID)
		if err != nil {
			slog.Error("MarkEmailRead: update failed", "error", err, "emailID", emailID)
			return err
		}

		_, err = tx.ExecContext(ctx,
			"UPDATE folders SET unread_count = MAX(unread_count - 1, 0) WHERE id = ?", folderID)
		if err != nil {
			slog.Error("MarkEmailRead: folder update failed", "error", err, "folderID", folderID)
			return err
		}

		return tx.Commit()
	})
}

func (s *Storage) ToggleFlagEmail(ctx context.Context, emailID string, accountID string) (bool, error) {
	var isFlagged bool
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		if accountID != "" {
			return s.db.QueryRowContext(ctx, "UPDATE emails SET is_flagged = NOT is_flagged, is_dirty_locally = 1 WHERE id = ? AND account_id = ? RETURNING is_flagged", emailID, accountID).Scan(&isFlagged)
		}
		return s.db.QueryRowContext(ctx, "UPDATE emails SET is_flagged = NOT is_flagged, is_dirty_locally = 1 WHERE id = ? RETURNING is_flagged", emailID).Scan(&isFlagged)
	})
	return isFlagged, err
}

func (s *Storage) MarkEmailAnsweredByMsgID(ctx context.Context, accountID, msgID string) error {
	msgID = strings.TrimSpace(msgID)
	if msgID == "" {
		return nil
	}
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		_, err := s.db.ExecContext(ctx,
			`UPDATE emails SET is_answered = 1, is_dirty_locally = 1
			 WHERE account_id = ? AND msg_id = ? AND is_answered = 0`,
			accountID, msgID)
		return err
	})
}

func (s *Storage) EmailExistsByMsgID(ctx context.Context, accountID, msgID string) (bool, error) {
	msgID = strings.TrimSpace(msgID)
	if msgID == "" {
		return false, nil
	}
	var id string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM emails WHERE account_id = ? AND msg_id = ? LIMIT 1`,
		accountID, msgID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Storage) TogglePinEmail(ctx context.Context, emailID string, accountID string) (bool, error) {
	var pinned bool
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		if accountID != "" {
			return s.db.QueryRowContext(ctx, "UPDATE emails SET is_pinned = NOT is_pinned, is_dirty_locally = 1 WHERE id = ? AND account_id = ? RETURNING is_pinned", emailID, accountID).Scan(&pinned)
		}
		return s.db.QueryRowContext(ctx, "UPDATE emails SET is_pinned = NOT is_pinned, is_dirty_locally = 1 WHERE id = ? RETURNING is_pinned", emailID).Scan(&pinned)
	})
	return pinned, err
}

func (s *Storage) ToggleMuteEmail(ctx context.Context, emailID string, accountID string) (bool, error) {
	var muted bool
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		if accountID != "" {
			return s.db.QueryRowContext(ctx, "UPDATE emails SET is_muted = NOT is_muted, is_dirty_locally = 1 WHERE id = ? AND account_id = ? RETURNING is_muted", emailID, accountID).Scan(&muted)
		}
		return s.db.QueryRowContext(ctx, "UPDATE emails SET is_muted = NOT is_muted, is_dirty_locally = 1 WHERE id = ? RETURNING is_muted", emailID).Scan(&muted)
	})
	return muted, err
}

func (s *Storage) GetSnoozedEmails(ctx context.Context) ([]models.Email, error) {
	query := "SELECT id, account_id, subject, sender_name, sender_address, snippet, snooze_until FROM emails WHERE snooze_until IS NOT NULL AND snooze_until <= datetime('now')"
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var snoozeUntil sql.NullString
		if err := rows.Scan(&e.ID, &e.AccountID, &e.Subject, &e.SenderName, &e.SenderAddress, &e.Snippet, &snoozeUntil); err != nil {
			return nil, err
		}
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) UnsnoozeEmail(ctx context.Context, emailID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE emails SET snooze_until = NULL WHERE id = ?`, emailID)
	return err
}

func (s *Storage) SnoozeEmail(ctx context.Context, emailID string, accountID string, minutes int) error {
	if accountID != "" {
		_, err := s.db.ExecContext(ctx,
			`UPDATE emails SET snooze_until = datetime('now', '+' || ? || ' minutes') WHERE id = ? AND account_id = ?`,
			fmt.Sprintf("%d", minutes), emailID, accountID)
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE emails SET snooze_until = datetime('now', '+' || ? || ' minutes') WHERE id = ?`,
		fmt.Sprintf("%d", minutes), emailID)
	return err
}

func (s *Storage) MoveEmail(ctx context.Context, emailID, accountID, folderID string) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := s.moveEmailFolderInTx(ctx, tx, emailID, accountID, folderID); err != nil {
			return err
		}
		return tx.Commit()
	})
}

// moveEmailFolderInTx moves an email to targetFolderID. If the same msg_id already exists
// in the target folder (common when IMAP synced Trash separately), the source row is removed
// instead of updating folder_id to avoid UNIQUE(msg_id, account_id, folder_id) violations.
// Returns the email_id to use for imap_move_queue (may be the surviving duplicate row).

func (s *Storage) moveEmailFolderInTx(ctx context.Context, tx *sql.Tx, emailID, accountID, targetFolderID string) (queueEmailID string, err error) {
	queueEmailID = emailID
	var msgID string
	if err := tx.QueryRowContext(ctx,
		`SELECT COALESCE(msg_id, '') FROM emails WHERE id = ? AND account_id = ?`,
		emailID, accountID).Scan(&msgID); err != nil {
		return "", err
	}
	if msgID != "" {
		var dupID string
		dupErr := tx.QueryRowContext(ctx,
			`SELECT id FROM emails WHERE msg_id = ? AND account_id = ? AND folder_id = ? AND id != ? LIMIT 1`,
			msgID, accountID, targetFolderID, emailID).Scan(&dupID)
		if dupErr == nil {
			if _, err := tx.ExecContext(ctx, `DELETE FROM emails_fts WHERE email_id = ?`, emailID); err != nil {
				return "", err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM emails WHERE id = ? AND account_id = ?`, emailID, accountID); err != nil {
				return "", err
			}
			return dupID, nil
		}
		if dupErr != sql.ErrNoRows {
			return "", dupErr
		}
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE emails SET folder_id = ? WHERE id = ? AND account_id = ?`,
		targetFolderID, emailID, accountID)
	return queueEmailID, err
}

func (s *Storage) GetEmailBodyPath(ctx context.Context, emailID string, accountID string) (string, error) {
	var path string
	var err error
	if accountID != "" {
		err = s.db.QueryRowContext(ctx, "SELECT COALESCE(body_path, '') FROM emails WHERE id = ? AND account_id = ?", emailID, accountID).Scan(&path)
	} else {
		err = s.db.QueryRowContext(ctx, "SELECT COALESCE(body_path, '') FROM emails WHERE id = ?", emailID).Scan(&path)
	}
	return path, err
}

func (s *Storage) SaveDraftReply(ctx context.Context, emailID, accountID, draftReply string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET draft_reply = ?, is_dirty_locally = 1 WHERE id = ? AND account_id = ?", draftReply, emailID, accountID)
	return err
}

func (s *Storage) ClearDraftReply(ctx context.Context, emailID string, accountID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET draft_reply = '', draft_remote_uid = 0, is_dirty_locally = 1 WHERE id = ? AND account_id = ?", emailID, accountID)
	return err
}

func (s *Storage) DeleteDraft(ctx context.Context, emailID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM emails WHERE id = ? AND status = 'draft'", emailID)
	return err
}

func (s *Storage) GetDraftsFolder(ctx context.Context, accountID string) (*models.Folder, error) {
	query := `
		SELECT id, account_id, name, path, is_subscribed, COALESCE(last_sync_uid,0), created_at
		FROM folders
		WHERE account_id = ? AND name_lower IN ('drafts', 'черновики', 'draft', '[gmail]/drafts')
		LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query, accountID)
	var f models.Folder
	var createdAt sql.NullString
	err := row.Scan(&f.ID, &f.AccountID, &f.Name, &f.Path, &f.IsSubscribed, &f.LastSyncUID, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	f.CreatedAt = parseTime(createdAt)
	return &f, nil
}

func (s *Storage) SaveStandaloneDraft(ctx context.Context, accountID, emailID, folderID, to, cc, subject, draftPayload string, isDirty bool) error {
	snippet := subject
	if len(snippet) > 100 {
		snippet = snippet[:100]
	}
	msgID := fmt.Sprintf("<draft-%s@rmsmail>", emailID)
	now := formatTime(time.Now())
	query := `
		INSERT INTO emails (id, account_id, folder_id, msg_id, uid, subject, recipient_address, cc_address, date_sent, is_read, is_dirty_locally, status, draft_reply, snippet)
		VALUES (?, ?, ?, ?, 0, ?, ?, ?, ?, 1, ?, 'draft', ?, ?)
		ON CONFLICT (id, account_id) DO UPDATE SET
			subject = excluded.subject,
			recipient_address = excluded.recipient_address,
			cc_address = excluded.cc_address,
			date_sent = excluded.date_sent,
			is_dirty_locally = excluded.is_dirty_locally,
			draft_reply = excluded.draft_reply,
			snippet = excluded.snippet
	`
	_, err := s.db.ExecContext(ctx, query, emailID, accountID, folderID, msgID, subject, to, cc, now, boolToInt(isDirty), draftPayload, snippet)
	return err
}

func (s *Storage) SetDraftRemoteUID(ctx context.Context, emailID string, accountID string, uid int) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET draft_remote_uid = ?, is_dirty_locally = 0 WHERE id = ? AND account_id = ?", uid, emailID, accountID)
	return err
}

func (s *Storage) GetDirtyDrafts(ctx context.Context, accountID string) ([]models.Email, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id,
		 draft_reply, draft_remote_uid, snippet, body_path, created_at
		 FROM emails WHERE account_id = ? AND is_dirty_locally = 1 AND draft_reply != ''`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, createdAt sql.NullString
		err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.BodyPath, &createdAt)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) GetDirtyEmails(ctx context.Context, accountID string) ([]models.Email, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, account_id, folder_id, uid, is_read, is_flagged, is_answered, is_dirty_locally
		 FROM emails WHERE account_id = ? AND is_dirty_locally = 1 LIMIT 500`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var isRead, isFlagged, isAnswered int
		if err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.UID, &isRead, &isFlagged, &isAnswered, &e.IsDirtyLocally); err != nil {
			return nil, err
		}
		e.IsRead = isRead != 0
		e.IsFlagged = isFlagged != 0
		e.IsAnswered = isAnswered != 0
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) GetEmailsForInboundFlagSync(ctx context.Context, accountID string, limit int) ([]models.Email, error) {
	if limit <= 0 {
		limit = 2000
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, account_id, folder_id, uid, is_read, is_flagged, is_answered
		FROM emails
		WHERE account_id = ? AND is_dirty_locally = 0 AND uid > 0
		ORDER BY date_sent DESC
		LIMIT ?`, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var isRead, isFlagged, isAnswered int
		if err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.UID, &isRead, &isFlagged, &isAnswered); err != nil {
			return nil, err
		}
		e.IsRead = isRead != 0
		e.IsFlagged = isFlagged != 0
		e.IsAnswered = isAnswered != 0
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) ApplyServerEmailFlags(ctx context.Context, emailID, accountID string, isRead, isFlagged, isAnswered bool) (bool, error) {
	var changed bool
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		var folderID string
		var currentRead, currentFlagged, currentAnswered int
		err = tx.QueryRowContext(ctx,
			`SELECT folder_id, is_read, is_flagged, is_answered FROM emails WHERE id = ? AND account_id = ? AND is_dirty_locally = 0`,
			emailID, accountID).Scan(&folderID, &currentRead, &currentFlagged, &currentAnswered)
		if err != nil {
			if err == sql.ErrNoRows {
				return nil
			}
			return err
		}
		wantRead := boolToInt(isRead)
		wantFlagged := boolToInt(isFlagged)
		wantAnswered := boolToInt(isAnswered)
		if currentRead == wantRead && currentFlagged == wantFlagged && currentAnswered == wantAnswered {
			return nil
		}

		res, err := tx.ExecContext(ctx,
			`UPDATE emails SET is_read = ?, is_flagged = ?, is_answered = ? WHERE id = ? AND account_id = ? AND is_dirty_locally = 0`,
			wantRead, wantFlagged, wantAnswered, emailID, accountID)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return nil
		}
		changed = true

		if folderID != "" {
			if currentRead == 0 && wantRead == 1 {
				_, err = tx.ExecContext(ctx,
					`UPDATE folders SET unread_count = MAX(unread_count - 1, 0) WHERE id = ?`, folderID)
			} else if currentRead == 1 && wantRead == 0 {
				_, err = tx.ExecContext(ctx,
					`UPDATE folders SET unread_count = unread_count + 1 WHERE id = ?`, folderID)
			}
			if err != nil {
				return err
			}
		}
		return tx.Commit()
	})
	return changed, err
}

func (s *Storage) ClearDirtyFlag(ctx context.Context, emailID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE emails SET is_dirty_locally = 0 WHERE id = ?`, emailID)
	return err
}

func (s *Storage) GetEmailsByThreadID(ctx context.Context, threadID, accountID string, limit int) ([]models.Email, error) {
	cols := "id, account_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, created_at"
	query := "SELECT " + cols + " FROM emails WHERE thread_id = ? AND account_id = ? ORDER BY date_sent DESC LIMIT ?"
	rows, err := s.db.QueryContext(ctx, query, threadID, accountID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, createdAt sql.NullString
		err := rows.Scan(&e.ID, &e.AccountID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &createdAt)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// ============================================================================
// Accounts
// ============================================================================

func (s *Storage) GetAccountCredentialsByEmail(ctx context.Context, email string) (*models.Account, error) {
	query := `SELECT COALESCE(id,''), COALESCE(email,''), COALESCE(name,''), COALESCE(provider,''), COALESCE(imap_host,''), COALESCE(imap_port,0), COALESCE(imap_ssl,0), COALESCE(imap_encryption,''), COALESCE(smtp_host,''), COALESCE(smtp_port,0), COALESCE(smtp_ssl,0), COALESCE(smtp_encryption,''), COALESCE(username,''), COALESCE(password_encrypted,''), COALESCE(oauth_access_token,''), COALESCE(oauth_refresh_token,''), COALESCE(last_uid,0), COALESCE(uid_validity,0), COALESCE(ai_provider_config,'{}'), COALESCE(signature,''), COALESCE(is_active,1), COALESCE(last_sync_error,''), COALESCE(last_sync_at, '0001-01-01T00:00:00Z'), COALESCE(is_locked, 0), COALESCE(avatar_url, ''), COALESCE(color, ''), COALESCE(sort_order, 0), COALESCE(smart_categories, 1) FROM accounts WHERE email = ?`
	row := s.db.QueryRowContext(ctx, query, email)
	var a models.Account
	var imapSSL, smtpSSL, isActive, isLocked, smartCategories int
	var lastSyncAt sql.NullString
	err := row.Scan(&a.ID, &a.Email, &a.Name, &a.Provider, &a.IMAPHost, &a.IMAPPort, &imapSSL, &a.IMAPEncryption, &a.SMTPHost, &a.SMTPPort, &smtpSSL, &a.SMTPEncryption, &a.Username, &a.PasswordEncrypted, &a.OAuthAccessToken, &a.OAuthRefreshToken, &a.LastUID, &a.UIDValidity, &a.AIProviderConfig, &a.Signature, &isActive, &a.LastSyncError, &lastSyncAt, &isLocked, &a.AvatarURL, &a.Color, &a.SortOrder, &smartCategories)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	a.IMAPSSL = imapSSL == 1
	a.SMTPSSL = smtpSSL == 1
	a.IsActive = isActive == 1
	a.IsLocked = isLocked == 1
	a.SmartCategories = smartCategories == 1
	dec, err := s.decryptPassword(a.PasswordEncrypted, "imap_password")
	if err != nil {
		return nil, err
	}
	a.PasswordEncrypted = dec
	if decAccess, err := s.decryptPassword(a.OAuthAccessToken, "oauth_token"); err == nil && decAccess != "" {
		a.OAuthAccessToken = decAccess
	}
	if decRefresh, err := s.decryptPassword(a.OAuthRefreshToken, "oauth_token"); err == nil && decRefresh != "" {
		a.OAuthRefreshToken = decRefresh
	}
	if lastSyncAt.Valid {
		a.LastSyncAt = parseTime(lastSyncAt)
	}
	return &a, nil
}

func (s *Storage) RefreshUnreadCounts(ctx context.Context) error {
	// Tag Gmail smart-category emails first so inbox counts stay aligned with the list view.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE emails SET smart_category = 1
		WHERE folder_id IN (
			SELECT id FROM folders
			WHERE name LIKE '%Promotions%' OR name LIKE '%Social%' OR name LIKE '%Updates%'
		)
		AND smart_category = 0
	`); err != nil {
		return err
	}

	// Inbox folders: exclude smart_category (same rule as GetEmailsCursor).
	if _, err := s.db.ExecContext(ctx, `
		WITH uc AS (
			SELECT e.folder_id, COUNT(*) as cnt
			FROM emails e
			INNER JOIN folders f ON f.id = e.folder_id
			WHERE e.is_read = 0 AND e.is_muted = 0
				AND NOT (f.is_inbox = 1 AND COALESCE(e.smart_category, 0) != 0)
			GROUP BY e.folder_id
		)
		UPDATE folders SET unread_count = uc.cnt
		FROM uc
		WHERE folders.id = uc.folder_id AND folders.unread_count != uc.cnt
	`); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE folders SET unread_count = 0
		WHERE unread_count != 0
		AND id NOT IN (
			SELECT e.folder_id FROM emails e
			INNER JOIN folders f ON f.id = e.folder_id
			WHERE e.is_read = 0 AND e.is_muted = 0
				AND NOT (f.is_inbox = 1 AND COALESCE(e.smart_category, 0) != 0)
		)
	`); err != nil {
		return err
	}
	return nil
}

func (s *Storage) GetEmailAttachments(ctx context.Context, emailID string, accountID string) ([]models.Attachment, error) {
	query := "SELECT id, email_id, filename, size, hash, content_id, path, created_at FROM attachments WHERE email_id = ? ORDER BY created_at ASC"
	rows, err := s.db.QueryContext(ctx, query, emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attachments []models.Attachment
	for rows.Next() {
		var att models.Attachment
		var createdAt sql.NullString
		if err := rows.Scan(&att.ID, &att.EmailID, &att.Filename, &att.Size, &att.Hash, &att.ContentID, &att.Path, &createdAt); err != nil {
			return nil, err
		}
		att.CreatedAt = parseTime(createdAt)
		attachments = append(attachments, att)
	}
	return attachments, rows.Err()
}

func (s *Storage) AddEmailTag(ctx context.Context, emailID string, accountID string, tag string) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO email_tags (id, email_id, account_id, tag) VALUES (?, ?, ?, ?)", uuid.New().String(), emailID, accountID, tag)
	return err
}

func (s *Storage) AddEmailTags(ctx context.Context, emailID string, accountID string, tags []string) error {
	if len(tags) == 0 {
		return nil
	}
	placeholders := make([]string, len(tags))
	args := make([]interface{}, 0, len(tags)*4)
	for i, tag := range tags {
		placeholders[i] = "(?, ?, ?, ?)"
		args = append(args, uuid.New().String(), emailID, accountID, tag)
	}
	query := "INSERT OR IGNORE INTO email_tags (id, email_id, account_id, tag) VALUES " + strings.Join(placeholders, ", ")
	_, err := s.db.ExecContext(ctx, query, args...)
	return err
}

func (s *Storage) GetEmailTags(ctx context.Context, emailID string, accountID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT tag FROM email_tags WHERE email_id = ?", emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func (s *Storage) GetEmailsByTag(ctx context.Context, tag string) ([]models.Email, error) {
	query := `SELECT e.id, e.account_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address,
		e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally,
		e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, e.created_at
		FROM emails e JOIN email_tags t ON e.id = t.email_id
		WHERE t.tag = ? ORDER BY e.date_sent DESC LIMIT 100`

	rows, err := s.db.QueryContext(ctx, query, tag)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, createdAt sql.NullString
		err := rows.Scan(&e.ID, &e.AccountID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &createdAt)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

// ============================================================================
// Labels
// ============================================================================

func (s *Storage) SetEmailLabels(ctx context.Context, emailID, accountID string, labelIDs []string) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(ctx, "DELETE FROM email_labels WHERE email_id = ?", emailID); err != nil {
			return err
		}
		if len(labelIDs) > 0 {
			var placeholders []string
			var args []interface{}
			for _, lid := range labelIDs {
				placeholders = append(placeholders, "(?, ?, ?)")
				args = append(args, emailID, lid, accountID)
			}
			query := fmt.Sprintf("INSERT OR IGNORE INTO email_labels (email_id, label_id, account_id) VALUES %s", strings.Join(placeholders, ","))
			if _, err := tx.ExecContext(ctx, query, args...); err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

func (s *Storage) GetEmailLabels(ctx context.Context, emailID string) ([]models.Label, error) {
	query := `SELECT l.id, COALESCE(l.account_id, ''), l.name, l.color, l.created_at
		FROM labels l JOIN email_labels el ON l.id = el.label_id WHERE el.email_id = ?`
	rows, err := s.db.QueryContext(ctx, query, emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var labels []models.Label
	for rows.Next() {
		var l models.Label
		var createdAt sql.NullString
		if err := rows.Scan(&l.ID, &l.AccountID, &l.Name, &l.Color, &createdAt); err != nil {
			return nil, err
		}
		l.CreatedAt = parseTime(createdAt)
		labels = append(labels, l)
	}
	return labels, rows.Err()
}

func (s *Storage) GetBatchEmailLabels(ctx context.Context, emailIDs []string) (map[string][]models.Label, error) {
	result := make(map[string][]models.Label)
	if len(emailIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(emailIDs))
	args := make([]interface{}, len(emailIDs))
	for i, id := range emailIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT el.email_id, l.id, COALESCE(l.account_id, ''), l.name, l.color, l.created_at
		FROM email_labels el JOIN labels l ON el.label_id = l.id
		WHERE el.email_id IN (` + strings.Join(placeholders, ",") + `)`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var emailID string
		var l models.Label
		var createdAt sql.NullString
		if err := rows.Scan(&emailID, &l.ID, &l.AccountID, &l.Name, &l.Color, &createdAt); err != nil {
			return nil, err
		}
		l.CreatedAt = parseTime(createdAt)
		result[emailID] = append(result[emailID], l)
	}
	return result, rows.Err()
}

func (s *Storage) GetBatchEmailTags(ctx context.Context, emailIDs []string) (map[string][]string, error) {
	result := make(map[string][]string)
	if len(emailIDs) == 0 {
		return result, nil
	}

	placeholders := make([]string, len(emailIDs))
	args := make([]interface{}, len(emailIDs))
	for i, id := range emailIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := `SELECT email_id, tag FROM email_tags WHERE email_id IN (` + strings.Join(placeholders, ",") + `) ORDER BY tag`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var emailID, tag string
		if err := rows.Scan(&emailID, &tag); err != nil {
			return nil, err
		}
		result[emailID] = append(result[emailID], tag)
	}
	return result, rows.Err()
}

// ============================================================================
// Rules
// ============================================================================

func (s *Storage) GetActiveRules(ctx context.Context, accountID string) ([]models.FilterRule, error) {
	query := `SELECT id, account_id, name, enabled, condition_field, condition_operator, condition_value, action_type, action_value, priority, ai_provider, ai_model, webhook_secret
		FROM filter_rules WHERE account_id = ? AND enabled = 1 ORDER BY priority`
	rows, err := s.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.FilterRule
	for rows.Next() {
		var r models.FilterRule
		if err := rows.Scan(&r.ID, &r.AccountID, &r.Name, &r.Enabled, &r.ConditionField, &r.ConditionOperator, &r.ConditionValue, &r.ActionType, &r.ActionValue, &r.Priority, &r.AIProvider, &r.AIModel, &r.WebhookSecret); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Storage) GetEmailsCursor(ctx context.Context, unified bool, accountID string, folderID string, folderName string, limit int, filter models.EmailFilterOpts, cursor *models.Cursor, scopedAccountIDs []string) ([]models.Email, *models.Cursor, error) {
	cols := "id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at"
	unifiedCols := "e.id, e.account_id, e.folder_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address, e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally, e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, COALESCE(e.spf_pass,0), COALESCE(e.dkim_pass,0), e.is_pinned, e.snooze_until, e.is_muted, e.status, e.first_response_at, e.resolved_at, e.created_at"

	var query string
	var args []interface{}

	if unified {
		if folderName == "" {
			folderName = "INBOX"
		}
		var folderFilter string
		if strings.EqualFold(folderName, "INBOX") {
			folderFilter = "e.folder_id IN (SELECT id FROM folders WHERE is_inbox = 1)"
		} else {
			folderFilter = "e.folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?))"
			args = append(args, folderName)
		}
		query = "SELECT " + unifiedCols + " FROM emails e WHERE " + folderFilter + " AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now'))"
		if strings.EqualFold(folderName, "INBOX") {
			query += " AND (e.smart_category = 0 OR e.smart_category IS NULL)"
		}
		if filter.Unread {
			query += " AND e.is_read = 0 AND e.is_muted = 0"
		}
		if filter.Flagged {
			query += " AND e.is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND e.has_attachments = 1"
		}
		if len(scopedAccountIDs) > 0 {
			ph := make([]string, len(scopedAccountIDs))
			for i, id := range scopedAccountIDs {
				ph[i] = "?"
				args = append(args, id)
			}
			query += " AND e.account_id IN (" + strings.Join(ph, ",") + ")"
		}
		if cursor != nil {
			query += " AND (e.is_pinned < ? OR (e.is_pinned = ? AND e.date_sent < ?) OR (e.is_pinned = ? AND e.date_sent = ? AND e.id < ?))"
			args = append(args, cursor.IsPinned, cursor.IsPinned, cursor.DateSent, cursor.IsPinned, cursor.DateSent, cursor.ID)
		}
		query += " ORDER BY e.is_pinned DESC, e.date_sent DESC, e.id DESC"
		query += " LIMIT ?"
		args = append(args, limit)
	} else if folderID != "" {
		query = "SELECT " + unifiedCols + " FROM emails e JOIN folders f ON e.folder_id = f.id WHERE e.account_id = ? AND e.folder_id = ? AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now'))"
		query += " AND (f.name_lower != 'inbox' OR e.smart_category = 0 OR e.smart_category IS NULL)"
		args = append(args, accountID, folderID)
		if filter.Unread {
			query += " AND e.is_read = 0 AND e.is_muted = 0"
		}
		if filter.Flagged {
			query += " AND e.is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND e.has_attachments = 1"
		}
		if cursor != nil {
			query += " AND (e.is_pinned < ? OR (e.is_pinned = ? AND e.date_sent < ?) OR (e.is_pinned = ? AND e.date_sent = ? AND e.id < ?))"
			args = append(args, cursor.IsPinned, cursor.IsPinned, cursor.DateSent, cursor.IsPinned, cursor.DateSent, cursor.ID)
		}
		query += " ORDER BY e.is_pinned DESC, e.date_sent DESC, e.id DESC"
		query += " LIMIT ?"
		args = append(args, limit)
	} else if accountID != "" {
		query = "SELECT " + cols + " FROM emails WHERE account_id = ? AND (snooze_until IS NULL OR snooze_until <= datetime('now'))"
		args = append(args, accountID)
		if filter.Unread {
			query += " AND is_read = 0 AND is_muted = 0"
		}
		if filter.Flagged {
			query += " AND is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND has_attachments = 1"
		}
		if cursor != nil {
			query += " AND (is_pinned < ? OR (is_pinned = ? AND date_sent < ?) OR (is_pinned = ? AND date_sent = ? AND id < ?))"
			args = append(args, cursor.IsPinned, cursor.IsPinned, cursor.DateSent, cursor.IsPinned, cursor.DateSent, cursor.ID)
		}
		query += " ORDER BY is_pinned DESC, date_sent DESC, id DESC"
		query += " LIMIT ?"
		args = append(args, limit)
	} else {
		query = "SELECT " + cols + " FROM emails WHERE (snooze_until IS NULL OR snooze_until <= datetime('now'))"
		if filter.Unread {
			query += " AND is_read = 0 AND is_muted = 0"
		}
		if filter.Flagged {
			query += " AND is_flagged = 1"
		}
		if filter.Attachments {
			query += " AND has_attachments = 1"
		}
		if cursor != nil {
			query += " AND (is_pinned < ? OR (is_pinned = ? AND date_sent < ?) OR (is_pinned = ? AND date_sent = ? AND id < ?))"
			args = append(args, cursor.IsPinned, cursor.IsPinned, cursor.DateSent, cursor.IsPinned, cursor.DateSent, cursor.ID)
		}
		query += " ORDER BY is_pinned DESC, date_sent DESC, id DESC"
		query += " LIMIT ?"
		args = append(args, limit)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		slog.Error("GetEmailsCursor query failed", "error", err)
		return nil, nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSentStr sql.NullString
		var createdAt sql.NullString
		var snoozeUntil sql.NullTime
		var firstRespAt sql.NullTime
		var resolvedAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject, &e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSentStr, &e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID, &e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass, &e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt); err != nil {
			slog.Error("GetEmailsCursor row scan failed", "error", err)
			return nil, nil, err
		}
		e.DateSent = parseTime(dateSentStr)
		e.CreatedAt = parseTime(createdAt)
		if snoozeUntil.Valid {
			e.SnoozeUntil = &snoozeUntil.Time
		}
		if firstRespAt.Valid {
			e.FirstResponseAt = &firstRespAt.Time
		}
		if resolvedAt.Valid {
			e.ResolvedAt = &resolvedAt.Time
		}
		emails = append(emails, e)
	}

	var nextCursor *models.Cursor
	if len(emails) == limit {
		last := emails[len(emails)-1]
		nextCursor = &models.Cursor{IsPinned: last.IsPinned, DateSent: last.DateSent, ID: last.ID}
	}
	return emails, nextCursor, rows.Err()
}

func (s *Storage) GetUnreadCountByAccount(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, COALESCE(SUM(unread_count), 0)
		FROM folders
		GROUP BY account_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var accountID string
		var count int
		if err := rows.Scan(&accountID, &count); err != nil {
			return nil, err
		}
		result[accountID] = count
	}
	return result, rows.Err()
}

func (s *Storage) GetUnreadInboxCountByAccount(ctx context.Context) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT account_id, COALESCE(SUM(unread_count), 0)
		FROM folders
		WHERE is_inbox = 1
		GROUP BY account_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var accountID string
		var count int
		if err := rows.Scan(&accountID, &count); err != nil {
			return nil, err
		}
		result[accountID] = count
	}
	return result, rows.Err()
}

func (s *Storage) GetUnreadCountByFolder(ctx context.Context, accountID string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, COALESCE(unread_count, 0) FROM folders WHERE account_id = ?
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int)
	for rows.Next() {
		var folderID string
		var count int
		if err := rows.Scan(&folderID, &count); err != nil {
			return nil, err
		}
		result[folderID] = count
	}
	return result, rows.Err()
}

// ============================================================================
// Templates
// ============================================================================

func (s *Storage) AttachAvatars(ctx context.Context, emails []models.Email) error {
	if len(emails) == 0 {
		return nil
	}
	addrs := make([]string, 0, len(emails))
	for _, e := range emails {
		if e.SenderAddress != "" {
			addrs = append(addrs, e.SenderAddress)
		}
	}
	if len(addrs) == 0 {
		return nil
	}

	placeholders := make([]string, len(addrs))
	args := make([]interface{}, len(addrs))
	for i, a := range addrs {
		placeholders[i] = "?"
		args[i] = a
	}

	rows, err := s.db.QueryContext(ctx,
		"SELECT email, avatar_url FROM sender_profiles WHERE email IN ("+strings.Join(placeholders, ",")+")",
		args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	profiles := make(map[string]string)
	for rows.Next() {
		var email, url string
		if err := rows.Scan(&email, &url); err != nil {
			continue
		}
		profiles[email] = url
	}

	for i := range emails {
		if url, ok := profiles[emails[i].SenderAddress]; ok {
			emails[i].AvatarURL = url
		}
	}
	return nil
}

// ============================================================================
// Groups (stubs for Mono)
// ============================================================================

func (s *Storage) GetGroupEmailAccountIDs(ctx context.Context, groupID string) ([]string, error) {
	return s.GetGroupAccounts(ctx, groupID)
}

// ============================================================================
// Users (stubs for Mono)
// ============================================================================

func (s *Storage) AssignEmail(ctx context.Context, emailID, userID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET assigned_to = ? WHERE id = ?", userID, emailID)
	return err
}

func (s *Storage) UnassignEmail(ctx context.Context, emailID string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET assigned_to = NULL WHERE id = ?", emailID)
	return err
}

func (s *Storage) GetUnassignedCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM emails WHERE (assigned_to IS NULL OR assigned_to = '') AND is_read = 0").Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Storage) GetStatsByAgent(ctx context.Context) ([]models.AgentStats, error) {
	query := `
		SELECT e.assigned_to, COALESCE(u.name, 'Unknown') AS name,
		       COUNT(*) AS assigned,
		       SUM(CASE WHEN DATE(e.resolved_at) = DATE('now') THEN 1 ELSE 0 END) AS resolved,
		       SUM(CASE WHEN e.is_read = 0 THEN 1 ELSE 0 END) AS unread_by_user
		FROM emails e
		LEFT JOIN users u ON u.id = e.assigned_to
		WHERE e.assigned_to IS NOT NULL AND e.assigned_to != ''
		GROUP BY e.assigned_to, u.name
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.AgentStats
	for rows.Next() {
		var st models.AgentStats
		if err := rows.Scan(&st.UserID, &st.Name, &st.Assigned, &st.Resolved, &st.UnreadByUser); err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}
	return stats, rows.Err()
}

func (s *Storage) GetSLABreaches(ctx context.Context, slaHours int) (int, error) {
	var count int
	cutoff := time.Now().Add(-time.Duration(slaHours) * time.Hour)
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM emails WHERE first_response_at IS NULL AND date_sent < ?",
		formatTime(cutoff)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *Storage) UpdateEmailStatus(ctx context.Context, emailID, status string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET status = ? WHERE id = ?", status, emailID)
	return err
}

func (s *Storage) UpdateEmailFirstResponseAt(ctx context.Context, emailID string, t time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET first_response_at = ? WHERE id = ?", formatTime(t), emailID)
	return err
}

func (s *Storage) UpdateEmailResolvedAt(ctx context.Context, emailID string, t time.Time) error {
	_, err := s.db.ExecContext(ctx, "UPDATE emails SET resolved_at = ? WHERE id = ?", formatTime(t), emailID)
	return err
}

// ============================================================================
// Comments (stubs for Mono)
// ============================================================================

func (s *Storage) EnqueueIMAPMove(ctx context.Context, emailID, accountID, targetFolderID, targetFolderName, sourceFolderName string, remoteUID int32) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO imap_move_queue (id, email_id, account_id, target_folder_id, target_folder_name, source_folder_name, remote_uid, retry_count, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
		uuid.New().String(), emailID, accountID, targetFolderID, targetFolderName, sourceFolderName, remoteUID, formatTime(time.Now()))
	return err
}

// MoveEmailAndEnqueueIMAP updates the email's folder_id and enqueues an IMAP move atomically.
// Uses a transaction wrapped in withWriteRetry so if the DB is locked the entire operation retries together.

func (s *Storage) MoveEmailAndEnqueueIMAP(ctx context.Context, emailID, accountID, targetFolderID, targetFolderName, sourceFolderName string, remoteUID int32) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()

		queueEmailID, err := s.moveEmailFolderInTx(ctx, tx, emailID, accountID, targetFolderID)
		if err != nil {
			return err
		}

		if remoteUID > 0 {
			_, err = tx.ExecContext(ctx,
				`INSERT OR REPLACE INTO imap_move_queue (id, email_id, account_id, target_folder_id, target_folder_name, source_folder_name, remote_uid, retry_count, created_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)`,
				uuid.New().String(), queueEmailID, accountID, targetFolderID, targetFolderName, sourceFolderName, remoteUID, formatTime(time.Now()))
			if err != nil {
				return err
			}
		}

		return tx.Commit()
	})
}

func (s *Storage) GetPendingIMAPMoves(ctx context.Context, accountID string) ([]models.IMAPMove, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, email_id, account_id, target_folder_id, target_folder_name, COALESCE(source_folder_name,''), remote_uid, retry_count, COALESCE(last_error,''), created_at
		 FROM imap_move_queue WHERE account_id = ? AND retry_count < 5 ORDER BY created_at ASC LIMIT 20`,
		accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var moves []models.IMAPMove
	for rows.Next() {
		var m models.IMAPMove
		var createdAt sql.NullString
		if err := rows.Scan(&m.ID, &m.EmailID, &m.AccountID, &m.TargetFolderID, &m.TargetFolderName, &m.SourceFolderName, &m.RemoteUID, &m.RetryCount, &m.LastError, &createdAt); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(createdAt)
		moves = append(moves, m)
	}
	return moves, rows.Err()
}

func (s *Storage) CompleteIMAPMove(ctx context.Context, moveID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM imap_move_queue WHERE id = ?", moveID)
	return err
}

func (s *Storage) FailIMAPMove(ctx context.Context, moveID string, errMsg string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE imap_move_queue SET retry_count = retry_count + 1, last_error = ? WHERE id = ?`,
		errMsg, moveID)
	return err
}

// ============================================================================
// AI Log (stubs for Mono)
// ============================================================================

func (s *Storage) ToggleMCPKey(ctx context.Context, id string) (*models.MCPKey, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE mcp_keys SET is_active = NOT is_active WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	var k models.MCPKey
	var lastUsedAt, createdAt sql.NullString
	err = s.db.QueryRowContext(ctx,
		"SELECT id, name, COALESCE(account_id, ''), key_prefix, is_active, COALESCE(last_used_at, ''), created_at FROM mcp_keys WHERE id = ?", id).
		Scan(&k.ID, &k.Name, &k.AccountID, &k.KeyPrefix, &k.IsActive, &lastUsedAt, &createdAt)
	if err != nil {
		return nil, err
	}
	if lastUsedAt.Valid && lastUsedAt.String != "" {
		t := parseTime(lastUsedAt)
		k.LastUsedAt = &t
	}
	k.CreatedAt = parseTime(createdAt)
	return &k, nil
}

func (s *Storage) GetAdminByEmail(ctx context.Context, email string) (string, string, error) {
	var id, passwordHash string
	err := s.db.QueryRowContext(ctx, "SELECT id, password_hash FROM admins WHERE email = ?", email).Scan(&id, &passwordHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", err
	}
	return id, passwordHash, nil
}

func (s *Storage) GetEmailByTelegramUserID(ctx context.Context, userID int64) (string, error) {
	if userID == 0 {
		return "", nil
	}
	var email string
	err := s.db.QueryRowContext(ctx, "SELECT email FROM admins WHERE telegram_user_id = ?", userID).Scan(&email)
	if err != nil {
		if err == sql.ErrNoRows {
			// Also check in users table (for agents)
			err = s.db.QueryRowContext(ctx, "SELECT email FROM users WHERE telegram_user_id = ?", userID).Scan(&email)
			if err != nil {
				if err == sql.ErrNoRows {
					return "", nil
				}
				return "", err
			}
			return email, nil
		}
		return "", err
	}
	return email, nil
}

func (s *Storage) executeInChunks(ctx context.Context, queryPrefix string, querySuffix string, ids []string) error {
	const chunkSize = 500
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]

		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			placeholders[j] = "?"
			args[j] = id
		}

		query := queryPrefix + strings.Join(placeholders, ",") + querySuffix
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return nil
}

func (s *Storage) BulkDeleteEmails(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		if err := s.executeInChunks(ctx, "DELETE FROM emails_fts WHERE email_id IN (", ")", ids); err != nil {
			return err
		}
		return s.executeInChunks(ctx, "DELETE FROM emails WHERE id IN (", ")", ids)
	})
}

func (s *Storage) BulkMarkEmailsRead(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		return s.bulkMarkEmailsRead(ctx, ids)
	})
}

func (s *Storage) bulkMarkEmailsRead(ctx context.Context, ids []string) error {
	// Two-phase: count affected unread per folder, update emails, then adjust folder counters.
	// Avoids CTE UPDATE...RETURNING (not supported in all SQLite builds) and avoids
	// full-table-scan RefreshUnreadCounts which kills perf on 200K+ inboxes.
	const chunkSize = 500
	folderDeltas := make(map[string]int) // folderID -> negative delta
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			placeholders[j] = "?"
			args[j] = id
		}
		inClause := strings.Join(placeholders, ",")

		// Phase 1: count how many are currently unread per folder
		rows, err := s.db.QueryContext(ctx,
			`SELECT folder_id, COUNT(*) FROM emails WHERE id IN (`+inClause+`) AND is_read = 0 GROUP BY folder_id`,
			args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var fid string
			var cnt int
			if err := rows.Scan(&fid, &cnt); err != nil {
				rows.Close()
				return err
			}
			folderDeltas[fid] -= cnt
		}
		rows.Close()

		// Phase 2: mark emails as read
		if _, err := s.db.ExecContext(ctx,
			`UPDATE emails SET is_read = 1, is_dirty_locally = 1 WHERE id IN (`+inClause+`)`,
			args...); err != nil {
			return err
		}
	}
	// Phase 3: apply folder counter deltas
	for fid, delta := range folderDeltas {
		s.db.ExecContext(ctx,
			`UPDATE folders SET unread_count = MAX(unread_count + ?, 0) WHERE id = ?`,
			delta, fid)
	}
	return nil
}

func (s *Storage) BulkMarkEmailsUnread(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		return s.bulkMarkEmailsUnread(ctx, ids)
	})
}

func (s *Storage) bulkMarkEmailsUnread(ctx context.Context, ids []string) error {
	// Two-phase: count affected read per folder, update emails, then adjust folder counters.
	// Avoids CTE UPDATE...RETURNING (not supported in all SQLite builds) and avoids
	// full-table-scan RefreshUnreadCounts which kills perf on 200K+ inboxes.
	const chunkSize = 500
	folderDeltas := make(map[string]int) // folderID -> positive delta
	for i := 0; i < len(ids); i += chunkSize {
		end := i + chunkSize
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for j, id := range chunk {
			placeholders[j] = "?"
			args[j] = id
		}
		inClause := strings.Join(placeholders, ",")

		// Phase 1: count how many are currently read per folder
		rows, err := s.db.QueryContext(ctx,
			`SELECT folder_id, COUNT(*) FROM emails WHERE id IN (`+inClause+`) AND is_read = 1 GROUP BY folder_id`,
			args...)
		if err != nil {
			return err
		}
		for rows.Next() {
			var fid string
			var cnt int
			if err := rows.Scan(&fid, &cnt); err != nil {
				rows.Close()
				return err
			}
			folderDeltas[fid] += cnt
		}
		rows.Close()

		// Phase 2: mark emails as unread
		if _, err := s.db.ExecContext(ctx,
			`UPDATE emails SET is_read = 0, is_dirty_locally = 1 WHERE id IN (`+inClause+`)`,
			args...); err != nil {
			return err
		}
	}
	// Phase 3: apply folder counter deltas
	for fid, delta := range folderDeltas {
		s.db.ExecContext(ctx,
			`UPDATE folders SET unread_count = unread_count + ? WHERE id = ?`,
			delta, fid)
	}
	return nil
}

func (s *Storage) BulkSetFlagEmails(ctx context.Context, ids []string, flagged bool) error {
	if len(ids) == 0 {
		return nil
	}
	val := 0
	if flagged {
		val = 1
	}
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		return s.executeInChunks(ctx, fmt.Sprintf("UPDATE emails SET is_flagged = %d, is_dirty_locally = 1 WHERE id IN (", val), ")", ids)
	})
}

func (s *Storage) GetEmailIDs(ctx context.Context, accountID, folderID string) ([]string, error) {
	var query string
	var args []interface{}

	if accountID == "unified" || accountID == "all" {
		if folderID == "" {
			folderID = "INBOX"
		}
		if strings.EqualFold(folderID, "INBOX") {
			query = `SELECT id FROM emails WHERE folder_id IN (SELECT id FROM folders WHERE is_inbox = 1) LIMIT 10000`
		} else {
			query = `SELECT id FROM emails WHERE folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?)) LIMIT 10000`
			args = append(args, folderID)
		}
	} else {
		query = "SELECT id FROM emails WHERE account_id = ?"
		args = append(args, accountID)
		if folderID != "" {
			query += " AND folder_id = ?"
			args = append(args, folderID)
		}
		query += " LIMIT 10000"
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Storage) GetActiveFilePaths(ctx context.Context) ([]string, error) {
	query := `
		SELECT path FROM attachments WHERE path IS NOT NULL AND path != ''
		UNION
		SELECT body_path FROM emails WHERE body_path IS NOT NULL AND body_path != ''
	`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (s *Storage) StoreScheduledEmail(ctx context.Context, id, accountID, payload string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scheduled_emails (id, account_id, email_data)
		VALUES (?, ?, ?)
	`, id, accountID, payload)
	return err
}

func (s *Storage) GetScheduledEmail(ctx context.Context, id string) (string, string, error) {
	var accountID, payload string
	err := s.db.QueryRowContext(ctx, `SELECT account_id, email_data FROM scheduled_emails WHERE id = ?`, id).Scan(&accountID, &payload)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", err
	}
	return accountID, payload, nil
}

func (s *Storage) DeleteScheduledEmail(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scheduled_emails WHERE id = ?`, id)
	return err
}

// ============================================================================
// Filter-based bulk operations (alternative to ID-list based, no LIMIT)
// ============================================================================

// buildFilterWhere builds a WHERE clause and args for filtering emails.
// When accountID is "unified", it joins with folders table for folder name matching.
// inboxListUnreadExcludeSQL excludes smart-category emails in inbox folders (list view parity).
const inboxListUnreadExcludeSQL = ` AND NOT (folder_id IN (SELECT id FROM folders WHERE is_inbox = 1) AND COALESCE(smart_category, 0) != 0)`

func buildFilterWhere(accountID, folderID string) (clause string, args []interface{}) {
	if accountID == "unified" || accountID == "all" {
		if folderID == "" {
			folderID = "INBOX"
		}
		if strings.EqualFold(folderID, "INBOX") {
			clause = `folder_id IN (SELECT id FROM folders WHERE is_inbox = 1) AND (COALESCE(smart_category, 0) = 0) AND (snooze_until IS NULL OR snooze_until <= datetime('now'))`
		} else {
			clause = `folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?)) AND (snooze_until IS NULL OR snooze_until <= datetime('now'))`
			args = append(args, folderID)
		}
		return
	}
	if folderID != "" {
		if strings.EqualFold(folderID, "INBOX") {
			clause = `account_id = ? AND folder_id IN (SELECT id FROM folders WHERE account_id = ? AND is_inbox = 1) AND (COALESCE(smart_category, 0) = 0) AND (snooze_until IS NULL OR snooze_until <= datetime('now'))`
			args = append(args, accountID, accountID)
			return
		}
		clause = `account_id = ? AND folder_id = ?`
		args = append(args, accountID, folderID)
		return
	}
	clause = `account_id = ?`
	args = append(args, accountID)
	return
}

func (s *Storage) GetEmailCount(ctx context.Context, accountID, folderID string, opts models.EmailCountOpts) (int, error) {
	where, args := buildFilterWhere(accountID, folderID)
	query := `SELECT COUNT(*) FROM emails WHERE ` + where
	// UUID folder_id path: still exclude smart-category rows in inbox folders.
	if folderID != "" && !strings.EqualFold(folderID, "INBOX") {
		query += inboxListUnreadExcludeSQL
	}
	if opts.Unread {
		query += ` AND is_read = 0 AND is_muted = 0`
	}
	if opts.Flagged {
		query += ` AND is_flagged = 1`
	}
	if opts.HasAttachments {
		query += ` AND has_attachments = 1`
	}
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *Storage) GetAccountIDsByFilter(ctx context.Context, accountID, folderID string) ([]string, error) {
	if accountID == "unified" || accountID == "all" {
		if folderID == "" {
			folderID = "INBOX"
		}
		var rows *sql.Rows
		var err error
		if strings.EqualFold(folderID, "INBOX") {
			rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT account_id FROM emails WHERE folder_id IN (SELECT id FROM folders WHERE is_inbox = 1)`)
		} else {
			rows, err = s.db.QueryContext(ctx, `SELECT DISTINCT account_id FROM emails WHERE folder_id IN (SELECT id FROM folders WHERE name_lower = LOWER(?))`, folderID)
		}
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			ids = append(ids, id)
		}
		return ids, rows.Err()
	}
	rows, err := s.db.QueryContext(ctx, `VALUES (?)`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Storage) GetEmailIDsByFilter(ctx context.Context, accountID, folderID string) ([]string, error) {
	where, args := buildFilterWhere(accountID, folderID)
	rows, err := s.db.QueryContext(ctx, `SELECT e.id FROM emails e WHERE `+where+` LIMIT 10000`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Storage) BulkReadByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	var affected int64
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		var err error
		affected, err = s.bulkReadByFilter(ctx, accountID, folderID)
		return err
	})
	return affected, err
}

func (s *Storage) bulkReadByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	where, args := buildFilterWhere(accountID, folderID)

	// Phase 1: count affected unread emails per folder
	countQuery := `SELECT folder_id, COUNT(*) FROM emails WHERE ` + where + ` AND is_read = 0 GROUP BY folder_id`
	rows, err := s.db.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return 0, err
	}
	type folderDelta struct {
		id    string
		count int
	}
	var deltas []folderDelta
	var totalAffected int64
	for rows.Next() {
		var fd folderDelta
		if err := rows.Scan(&fd.id, &fd.count); err != nil {
			rows.Close()
			return 0, err
		}
		deltas = append(deltas, fd)
		totalAffected += int64(fd.count)
	}
	rows.Close()

	if totalAffected == 0 {
		return 0, nil
	}

	// Phase 2: mark emails as read
	updateQuery := `UPDATE emails SET is_read = 1, is_dirty_locally = 1 WHERE ` + where + ` AND is_read = 0`
	if _, err := s.db.ExecContext(ctx, updateQuery, args...); err != nil {
		return 0, err
	}

	// Phase 3: update folder counters
	for _, d := range deltas {
		s.db.ExecContext(ctx, `UPDATE folders SET unread_count = MAX(unread_count - ?, 0) WHERE id = ?`, d.count, d.id)
	}

	return totalAffected, nil
}

func (s *Storage) BulkUnreadByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	var affected int64
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		var err error
		affected, err = s.bulkUnreadByFilter(ctx, accountID, folderID)
		return err
	})
	return affected, err
}

func (s *Storage) bulkUnreadByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	where, args := buildFilterWhere(accountID, folderID)

	// Phase 1: count affected read emails per folder
	countQuery := `SELECT folder_id, COUNT(*) FROM emails WHERE ` + where + ` AND is_read = 1 GROUP BY folder_id`
	rows, err := s.db.QueryContext(ctx, countQuery, args...)
	if err != nil {
		return 0, err
	}
	type folderDelta struct {
		id    string
		count int
	}
	var deltas []folderDelta
	var totalAffected int64
	for rows.Next() {
		var fd folderDelta
		if err := rows.Scan(&fd.id, &fd.count); err != nil {
			rows.Close()
			return 0, err
		}
		deltas = append(deltas, fd)
		totalAffected += int64(fd.count)
	}
	rows.Close()

	if totalAffected == 0 {
		return 0, nil
	}

	// Phase 2: mark emails as unread
	updateQuery := `UPDATE emails SET is_read = 0, is_dirty_locally = 1 WHERE ` + where + ` AND is_read = 1`
	if _, err := s.db.ExecContext(ctx, updateQuery, args...); err != nil {
		return 0, err
	}

	// Phase 3: update folder counters
	for _, d := range deltas {
		s.db.ExecContext(ctx, `UPDATE folders SET unread_count = unread_count + ? WHERE id = ?`, d.count, d.id)
	}

	return totalAffected, nil
}

func (s *Storage) BulkFlagByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	var affected int64
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		var err error
		affected, err = s.bulkFlagByFilter(ctx, accountID, folderID)
		return err
	})
	return affected, err
}

func (s *Storage) bulkFlagByFilter(ctx context.Context, accountID, folderID string) (int64, error) {
	where, args := buildFilterWhere(accountID, folderID)
	result, err := s.db.ExecContext(ctx, `UPDATE emails SET is_flagged = CASE WHEN is_flagged = 1 THEN 0 ELSE 1 END, is_dirty_locally = 1 WHERE `+where, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Storage) BulkSetFlagByFilter(ctx context.Context, accountID, folderID string, flagged bool) (int64, error) {
	var affected int64
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		var err error
		affected, err = s.bulkSetFlagByFilter(ctx, accountID, folderID, flagged)
		return err
	})
	return affected, err
}

func (s *Storage) bulkSetFlagByFilter(ctx context.Context, accountID, folderID string, flagged bool) (int64, error) {
	where, args := buildFilterWhere(accountID, folderID)
	val := 0
	if flagged {
		val = 1
	}
	result, err := s.db.ExecContext(ctx, fmt.Sprintf(`UPDATE emails SET is_flagged = %d, is_dirty_locally = 1 WHERE `, val)+where, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Storage) BulkMoveByFilter(ctx context.Context, accountID, sourceFolderID, targetFolderID string) (int64, error) {
	var affected int64
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		var err error
		affected, err = s.bulkMoveByFilter(ctx, accountID, sourceFolderID, targetFolderID)
		return err
	})
	return affected, err
}

func (s *Storage) bulkMoveByFilter(ctx context.Context, accountID, sourceFolderID, targetFolderID string) (int64, error) {
	where, args := buildFilterWhere(accountID, sourceFolderID)
	result, err := s.db.ExecContext(ctx, `UPDATE emails SET folder_id = ?, is_dirty_locally = 1 WHERE `+where, append([]interface{}{targetFolderID}, args...)...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Storage) BulkMoveAndEnqueue(ctx context.Context, ids []string, accountID, targetFolderID, targetFolderName string, emails []models.Email) error {
	return s.withWriteRetry(ctx, func(ctx context.Context) error {
		return s.bulkMoveAndEnqueue(ctx, ids, accountID, targetFolderID, targetFolderName, emails)
	})
}

func (s *Storage) bulkMoveAndEnqueue(ctx context.Context, ids []string, accountID, targetFolderID, targetFolderName string, emails []models.Email) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(ids) > 0 {
		query := "UPDATE emails SET folder_id = ? WHERE account_id = ? AND id IN ("
		args := []interface{}{targetFolderID, accountID}
		for i, id := range ids {
			if i > 0 {
				query += ", "
			}
			query += "?"
			args = append(args, id)
		}
		query += ")"
		_, err = tx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
	}

	if len(emails) > 0 {
		// Карта имён папок-источников
		folderMap := make(map[string]string)
		rows, fErr := tx.QueryContext(ctx, "SELECT id, name FROM folders WHERE account_id = ?", accountID)
		if fErr == nil {
			for rows.Next() {
				var fid, fname string
				rows.Scan(&fid, &fname)
				folderMap[fid] = fname
			}
			rows.Close()
		}

		query := "INSERT OR REPLACE INTO imap_move_queue (id, email_id, account_id, target_folder_id, target_folder_name, source_folder_name, remote_uid, retry_count, created_at) VALUES "
		var args []interface{}
		validUIDs := 0
		for _, e := range emails {
			if e.UID <= 0 {
				continue
			}
			if validUIDs > 0 {
				query += ", "
			}
			query += "(?, ?, ?, ?, ?, ?, ?, 0, CURRENT_TIMESTAMP)"
			args = append(args, uuid.New().String(), e.ID, accountID, targetFolderID, targetFolderName, folderMap[e.FolderID], e.UID)
			validUIDs++
		}
		if validUIDs > 0 {
			_, err = tx.ExecContext(ctx, query, args...)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func isGmailSystemLabel(label string) bool {
	return strings.HasPrefix(label, "\\")
}

func (s *Storage) GetEmailByMsgIDAccount(ctx context.Context, msgID, accountID string) (*models.Email, error) {
	query := "SELECT id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, cc_address, date_sent, is_read, is_flagged, is_answered, has_attachments, is_dirty_locally, in_reply_to, thread_id, draft_reply, draft_remote_uid, snippet, body_path, COALESCE(spf_pass,0), COALESCE(dkim_pass,0), is_pinned, snooze_until, is_muted, status, first_response_at, resolved_at, created_at FROM emails WHERE msg_id = ? AND account_id = ? LIMIT 1"
	row := s.db.QueryRowContext(ctx, query, msgID, accountID)
	var e models.Email
	var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
	err := row.Scan(&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
		&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
		&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
		&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.BodyPath, &e.SpfPass, &e.DkimPass,
		&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	e.DateSent = parseTime(dateSent)
	if snoozeUntil.Valid {
		t := parseTime(snoozeUntil)
		e.SnoozeUntil = &t
	}
	if firstRespAt.Valid {
		t := parseTime(firstRespAt)
		e.FirstResponseAt = &t
	}
	if resolvedAt.Valid {
		t := parseTime(resolvedAt)
		e.ResolvedAt = &t
	}
	e.CreatedAt = parseTime(createdAt)
	return &e, nil
}

func (s *Storage) GetEmailsByLabel(ctx context.Context, accountID, label string, offset, limit int) ([]models.Email, error) {
	cols := "e.id, e.account_id, e.folder_id, e.msg_id, e.uid, e.subject, e.sender_name, e.sender_address, e.recipient_address, e.cc_address, e.date_sent, e.is_read, e.is_flagged, e.is_answered, e.has_attachments, e.is_dirty_locally, e.in_reply_to, e.thread_id, e.draft_reply, e.draft_remote_uid, e.snippet, COALESCE(e.spf_pass,0), COALESCE(e.dkim_pass,0), e.is_pinned, e.snooze_until, e.is_muted, e.status, e.first_response_at, e.resolved_at, e.created_at"
	query := "SELECT " + cols + " FROM emails e JOIN email_labels_junction j ON e.id = j.email_id WHERE e.account_id = ? AND j.label = ? AND (e.snooze_until IS NULL OR e.snooze_until <= datetime('now')) ORDER BY e.date_sent DESC LIMIT ? OFFSET ?"

	rows, err := s.db.QueryContext(ctx, query, accountID, label, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var e models.Email
		var dateSent, snoozeUntil, firstRespAt, resolvedAt, createdAt sql.NullString
		err := rows.Scan(
			&e.ID, &e.AccountID, &e.FolderID, &e.MsgID, &e.UID, &e.Subject,
			&e.SenderName, &e.SenderAddress, &e.RecipientAddress, &e.CcAddress, &dateSent,
			&e.IsRead, &e.IsFlagged, &e.IsAnswered, &e.HasAttachments, &e.IsDirtyLocally, &e.InReplyTo, &e.ThreadID,
			&e.DraftReply, &e.DraftRemoteUID, &e.Snippet, &e.SpfPass, &e.DkimPass,
			&e.IsPinned, &snoozeUntil, &e.IsMuted, &e.Status, &firstRespAt, &resolvedAt, &createdAt,
		)
		if err != nil {
			return nil, err
		}
		e.DateSent = parseTime(dateSent)
		if snoozeUntil.Valid {
			t := parseTime(snoozeUntil)
			e.SnoozeUntil = &t
		}
		if firstRespAt.Valid {
			t := parseTime(firstRespAt)
			e.FirstResponseAt = &t
		}
		if resolvedAt.Valid {
			t := parseTime(resolvedAt)
			e.ResolvedAt = &t
		}
		e.CreatedAt = parseTime(createdAt)
		emails = append(emails, e)
	}
	return emails, rows.Err()
}

func (s *Storage) DeleteEmailLabels(ctx context.Context, emailID, accountID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM email_labels_junction WHERE email_id = ? AND account_id = ?`, emailID, accountID)
	return err
}

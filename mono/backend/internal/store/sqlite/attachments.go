package sqlite

import (
	"context"
	"database/sql"

	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) SaveAttachment(ctx context.Context, att *models.Attachment) error {
	if att.ID == "" {
		att.ID = uuid.New().String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO attachments (id, email_id, account_id, filename, size, hash, content_id, path) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		att.ID, att.EmailID, att.AccountID, att.Filename, att.Size, att.Hash, att.ContentID, att.Path)
	return err
}

func (s *Storage) GetAttachmentByHash(ctx context.Context, hash string) (*models.Attachment, error) {
	query := "SELECT id, email_id, filename, size, hash, content_id, path, created_at FROM attachments WHERE hash = ? LIMIT 1"
	row := s.db.QueryRowContext(ctx, query, hash)
	var att models.Attachment
	var createdAt sql.NullString
	err := row.Scan(&att.ID, &att.EmailID, &att.Filename, &att.Size, &att.Hash, &att.ContentID, &att.Path, &createdAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	att.CreatedAt = parseTime(createdAt)
	return &att, nil
}

func (s *Storage) GetAllAttachments(ctx context.Context) ([]models.Attachment, error) {
	query := "SELECT id, email_id, filename, size, hash, content_id, path, created_at FROM attachments LIMIT 10000"
	rows, err := s.db.QueryContext(ctx, query)
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

// ============================================================================
// Tags
// ============================================================================

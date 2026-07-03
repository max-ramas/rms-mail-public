package sqlite

import (
	"context"
	"database/sql"
	"time"

	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) GetLabels(ctx context.Context, accountID string) ([]models.Label, error) {
	var query string
	var rows *sql.Rows
	var err error
	if accountID == "" {
		query = "SELECT id, COALESCE(account_id, ''), name, color, created_at FROM labels ORDER BY name"
		rows, err = s.db.QueryContext(ctx, query)
	} else {
		query = "SELECT id, COALESCE(account_id, ''), name, color, created_at FROM labels WHERE account_id = ? OR account_id IS NULL OR account_id = '' ORDER BY name"
		rows, err = s.db.QueryContext(ctx, query, accountID)
	}
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

func (s *Storage) CreateLabel(ctx context.Context, accountID, name, color string) (*models.Label, error) {
	id := uuid.New().String()
	now := formatTime(time.Now())
	var accID interface{}
	if accountID != "" && accountID != "unified" {
		accID = accountID
	}
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		_, err := s.db.ExecContext(ctx, `INSERT INTO labels (id, account_id, name, color, created_at) VALUES (?, ?, ?, ?, ?)`, id, accID, name, color, now)
		return err
	})
	if err != nil {
		return nil, err
	}
	var resAccountID string
	if accID != nil {
		resAccountID = accountID
	}
	return &models.Label{ID: id, AccountID: resAccountID, Name: name, Color: color, CreatedAt: time.Now()}, nil
}

func (s *Storage) UpdateLabel(ctx context.Context, id, name, color string) (*models.Label, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE labels SET name = ?, color = ? WHERE id = ?", name, color, id)
	if err != nil {
		return nil, err
	}
	var l models.Label
	var createdAt sql.NullString
	err = s.db.QueryRowContext(ctx, "SELECT id, COALESCE(account_id,''), name, color, created_at FROM labels WHERE id = ?", id).Scan(&l.ID, &l.AccountID, &l.Name, &l.Color, &createdAt)
	if err != nil {
		return nil, err
	}
	l.CreatedAt = parseTime(createdAt)
	return &l, nil
}

func (s *Storage) DeleteLabel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM labels WHERE id = ?", id)
	return err
}

func (s *Storage) GetLabel(ctx context.Context, id string) (*models.Label, error) {
	query := "SELECT id, COALESCE(account_id, ''), name, COALESCE(color, '#3b82f6'), created_at FROM labels WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var l models.Label
	var createdAt sql.NullString
	if err := row.Scan(&l.ID, &l.AccountID, &l.Name, &l.Color, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	l.CreatedAt = parseTime(createdAt)
	return &l, nil
}

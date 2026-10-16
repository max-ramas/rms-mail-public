package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) GetFolders(ctx context.Context, accountID string) ([]models.Folder, error) {
	var query string
	var args []interface{}

	if accountID == "" {
		// Return all folders (unified view / empty trash)
		query = `SELECT f.id, f.account_id, f.name, f.path, f.is_subscribed, f.last_sync_uid, f.created_at, COALESCE(f.unread_count, 0), COALESCE(f.uid_validity, 0), COALESCE(f.special_use, '')
					FROM folders f JOIN accounts a ON f.account_id = a.id
					WHERE f.is_subscribed = 1
					GROUP BY f.id
					ORDER BY CASE WHEN UPPER(f.name) = 'INBOX' THEN 1 WHEN UPPER(f.name) LIKE '%SENT%' OR UPPER(f.name) LIKE '%ОТПРАВЛ%' THEN 2
												WHEN UPPER(f.name) LIKE '%DRAFT%' OR UPPER(f.name) LIKE '%ЧЕРНОВ%' THEN 3
												WHEN UPPER(f.name) LIKE '%IMPORTANT%' OR UPPER(f.name) LIKE '%ВАЖНОЕ%' THEN 4
												WHEN UPPER(f.name) LIKE '%STARRED%' OR UPPER(f.name) LIKE '%ПОМЕЧЕННЫЕ%' THEN 5
												WHEN UPPER(f.name) LIKE '%ALL MAIL%' OR UPPER(f.name) LIKE '%ВСЯ ПОЧТА%' THEN 6
												WHEN UPPER(f.name) LIKE '%TRASH%' OR UPPER(f.name) LIKE '%SPAM%' OR UPPER(f.name) LIKE '%JUNK%' OR UPPER(f.name) LIKE '%КОРЗИНА%' THEN 8 ELSE 7 END ASC, f.name ASC`
	} else {
		query = `SELECT f.id, f.account_id, f.name, f.path, f.is_subscribed, f.last_sync_uid, f.created_at, COALESCE(f.unread_count, 0), COALESCE(f.uid_validity, 0), COALESCE(f.special_use, '')
				FROM folders f
				JOIN accounts a ON f.account_id = a.id
				WHERE f.account_id = ? AND f.is_subscribed = 1
				ORDER BY CASE WHEN UPPER(f.name) = 'INBOX' THEN 1 WHEN UPPER(f.name) LIKE '%SENT%' OR UPPER(f.name) LIKE '%ОТПРАВЛ%' THEN 2
												WHEN UPPER(f.name) LIKE '%DRAFT%' OR UPPER(f.name) LIKE '%ЧЕРНОВ%' THEN 3
												WHEN UPPER(f.name) LIKE '%IMPORTANT%' OR UPPER(f.name) LIKE '%ВАЖНОЕ%' THEN 4
												WHEN UPPER(f.name) LIKE '%STARRED%' OR UPPER(f.name) LIKE '%ПОМЕЧЕННЫЕ%' THEN 5
												WHEN UPPER(f.name) LIKE '%ALL MAIL%' OR UPPER(f.name) LIKE '%ВСЯ ПОЧТА%' THEN 6
												WHEN UPPER(f.name) LIKE '%TRASH%' OR UPPER(f.name) LIKE '%SPAM%' OR UPPER(f.name) LIKE '%JUNK%' OR UPPER(f.name) LIKE '%КОРЗИНА%' THEN 8 ELSE 7 END ASC, f.name ASC`
		args = append(args, accountID)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var folders []models.Folder
	for rows.Next() {
		var f models.Folder
		var createdAt sql.NullString
		err := rows.Scan(&f.ID, &f.AccountID, &f.Name, &f.Path, &f.IsSubscribed, &f.LastSyncUID, &createdAt, &f.UnreadCount, &f.UIDValidity, &f.SpecialUse)
		if err != nil {
			return nil, err
		}
		f.CreatedAt = parseTime(createdAt)
		folders = append(folders, f)
	}
	return folders, rows.Err()
}

func (s *Storage) GetFolderByID(ctx context.Context, id string) (*models.Folder, error) {
	query := "SELECT id, account_id, name, path, is_subscribed, COALESCE(last_sync_uid,0), created_at, COALESCE(uid_validity,0), COALESCE(special_use,'') FROM folders WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var f models.Folder
	var createdAt sql.NullString
	err := row.Scan(&f.ID, &f.AccountID, &f.Name, &f.Path, &f.IsSubscribed, &f.LastSyncUID, &createdAt, &f.UIDValidity, &f.SpecialUse)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	f.CreatedAt = parseTime(createdAt)
	return &f, nil
}

func (s *Storage) GetFolderByName(ctx context.Context, accountID, name string) (*models.Folder, error) {
	query := "SELECT id, account_id, name, path, is_subscribed, last_sync_uid, COALESCE(unread_count,0), created_at, COALESCE(uid_validity,0), COALESCE(special_use,'') FROM folders WHERE account_id = ? AND name_lower = LOWER(?) LIMIT 1"
	row := s.db.QueryRowContext(ctx, query, accountID, name)
	var f models.Folder
	var createdAt sql.NullString
	err := row.Scan(&f.ID, &f.AccountID, &f.Name, &f.Path, &f.IsSubscribed, &f.LastSyncUID, &f.UnreadCount, &createdAt, &f.UIDValidity, &f.SpecialUse)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	f.CreatedAt = parseTime(createdAt)
	return &f, nil
}

func (s *Storage) CreateFolder(ctx context.Context, accountID, name, path string, subscribed bool) (*models.Folder, error) {
	var f *models.Folder
	err := s.withWriteRetry(ctx, func(ctx context.Context) error {
		id := uuid.New().String()
		now := formatTime(time.Now())
		isInbox := 0
		if strings.ToLower(name) == "inbox" {
			isInbox = 1
		}
		_, err := s.db.ExecContext(ctx,
			`INSERT INTO folders (id, account_id, name, path, is_subscribed, created_at, is_inbox) VALUES (?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT (account_id, path) DO UPDATE SET is_subscribed = ?, is_inbox = ?`,
			id, accountID, name, path, boolToInt(subscribed), now, isInbox, boolToInt(subscribed), isInbox)
		if err != nil {
			return err
		}
		f, err = s.GetFolderByID(ctx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (s *Storage) RenameFolder(ctx context.Context, folderID, newName string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE folders SET name = ?, path = ? WHERE id = ?", newName, newName, folderID)
	return err
}

func (s *Storage) DeleteFolder(ctx context.Context, folderID string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM emails WHERE folder_id = ?", folderID)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "DELETE FROM folders WHERE id = ?", folderID)
	return err
}

func (s *Storage) UpdateFolderLastUID(ctx context.Context, folderID string, lastUID int) error {
	_, err := s.db.ExecContext(ctx, "UPDATE folders SET last_sync_uid = ? WHERE id = ?", lastUID, folderID)
	return err
}

func (s *Storage) UpdateFolderUIDValidity(ctx context.Context, folderID string, uidValidity uint32) error {
	_, err := s.db.ExecContext(ctx, "UPDATE folders SET uid_validity = ? WHERE id = ?", uidValidity, folderID)
	return err
}

func (s *Storage) UpdateFolderSpecialUse(ctx context.Context, folderID string, specialUse string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE folders SET special_use = ? WHERE id = ?", specialUse, folderID)
	return err
}

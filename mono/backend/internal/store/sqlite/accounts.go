package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"rmsmail/internal/models"
	"rmsmail/internal/store/shared"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) encryptPassword(password string, domain string) (string, error) {
	return shared.EncryptPassword(s.encKey, password, domain)
}

func (s *Storage) decryptPassword(encrypted string, domain string) (string, error) {
	return shared.DecryptPassword(s.encKeys, encrypted, domain)
}

// ============================================================================
// Emails
// ============================================================================

func (s *Storage) GetAccounts(ctx context.Context) ([]models.Account, error) {
	query := `SELECT COALESCE(id,''), COALESCE(email,''), COALESCE(name,''), COALESCE(provider,''), COALESCE(imap_host,''), COALESCE(imap_port,0), COALESCE(imap_ssl,0), COALESCE(imap_encryption,''), COALESCE(smtp_host,''), COALESCE(smtp_port,0), COALESCE(smtp_ssl,0), COALESCE(smtp_encryption,''), COALESCE(username,''), COALESCE(last_uid,0), COALESCE(uid_validity,0), COALESCE(ai_provider_config,'{}'), COALESCE(signature,''), COALESCE(is_active,1), COALESCE(last_sync_error,''), COALESCE(last_sync_at, '0001-01-01T00:00:00Z'), COALESCE(is_locked, 0), COALESCE(avatar_url, ''), COALESCE(color, ''), COALESCE(sort_order, 0), COALESCE(smart_categories, 1), COALESCE(is_gmail, 0) FROM accounts WHERE is_active = 1`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []models.Account
	for rows.Next() {
		var a models.Account
		var imapSSL, smtpSSL, isActive, isLocked, smartCategories, isGmail int
		var lastSyncAt sql.NullString
		err := rows.Scan(&a.ID, &a.Email, &a.Name, &a.Provider, &a.IMAPHost, &a.IMAPPort, &imapSSL, &a.IMAPEncryption, &a.SMTPHost, &a.SMTPPort, &smtpSSL, &a.SMTPEncryption, &a.Username, &a.LastUID, &a.UIDValidity, &a.AIProviderConfig, &a.Signature, &isActive, &a.LastSyncError, &lastSyncAt, &isLocked, &a.AvatarURL, &a.Color, &a.SortOrder, &smartCategories, &isGmail)
		if err != nil {
			return nil, err
		}
		a.IMAPSSL = imapSSL == 1
		a.SMTPSSL = smtpSSL == 1
		a.IsActive = isActive == 1
		a.IsLocked = isLocked == 1
		a.SmartCategories = smartCategories == 1
		a.IsGmail = isGmail == 1
		if lastSyncAt.Valid {
			a.LastSyncAt = parseTime(lastSyncAt)
		}
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func (s *Storage) GetFirstRegisteredAccountEmail(ctx context.Context) (string, error) {
	query := `SELECT COALESCE(email,'') FROM accounts ORDER BY created_at ASC LIMIT 1`
	var email string
	err := s.db.QueryRowContext(ctx, query).Scan(&email)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return "", nil
		}
		return "", err
	}
	return email, nil
}

func (s *Storage) GetAccount(ctx context.Context, id string) (*models.Account, error) {
	query := `SELECT COALESCE(id,''), COALESCE(email,''), COALESCE(name,''), COALESCE(provider,''), COALESCE(imap_host,''), COALESCE(imap_port,0), COALESCE(imap_ssl,0), COALESCE(imap_encryption,''), COALESCE(smtp_host,''), COALESCE(smtp_port,0), COALESCE(smtp_ssl,0), COALESCE(smtp_encryption,''), COALESCE(username,''), COALESCE(password_encrypted,''), COALESCE(oauth_access_token,''), COALESCE(oauth_refresh_token,''), COALESCE(last_uid,0), COALESCE(uid_validity,0), COALESCE(ai_provider_config,'{}'), COALESCE(signature,''), COALESCE(is_active,1), COALESCE(last_sync_error,''), COALESCE(last_sync_at, '0001-01-01T00:00:00Z'), COALESCE(is_locked, 0), COALESCE(avatar_url, ''), COALESCE(color, ''), COALESCE(sort_order, 0), COALESCE(smart_categories, 1), COALESCE(is_gmail, 0) FROM accounts WHERE id = ?`
	row := s.db.QueryRowContext(ctx, query, id)

	var a models.Account
	var imapSSL, smtpSSL, isActive, isLocked, smartCategories, isGmail int
	var lastSyncAt sql.NullString
	err := row.Scan(&a.ID, &a.Email, &a.Name, &a.Provider, &a.IMAPHost, &a.IMAPPort, &imapSSL, &a.IMAPEncryption, &a.SMTPHost, &a.SMTPPort, &smtpSSL, &a.SMTPEncryption, &a.Username, &a.PasswordEncrypted, &a.OAuthAccessToken, &a.OAuthRefreshToken, &a.LastUID, &a.UIDValidity, &a.AIProviderConfig, &a.Signature, &isActive, &a.LastSyncError, &lastSyncAt, &isLocked, &a.AvatarURL, &a.Color, &a.SortOrder, &smartCategories, &isGmail)
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
	a.IsGmail = isGmail == 1
	a.PasswordEncrypted = ""
	a.OAuthAccessToken = ""
	a.OAuthRefreshToken = ""
	if lastSyncAt.Valid {
		a.LastSyncAt = parseTime(lastSyncAt)
	}
	return &a, nil
}

func (s *Storage) GetAccountCredentials(ctx context.Context, id string) (*models.Account, error) {
	query := `SELECT COALESCE(id,''), COALESCE(email,''), COALESCE(name,''), COALESCE(provider,''), COALESCE(imap_host,''), COALESCE(imap_port,0), COALESCE(imap_ssl,0), COALESCE(imap_encryption,''), COALESCE(smtp_host,''), COALESCE(smtp_port,0), COALESCE(smtp_ssl,0), COALESCE(smtp_encryption,''), COALESCE(username,''), COALESCE(password_encrypted,''), COALESCE(oauth_access_token,''), COALESCE(oauth_refresh_token,''), COALESCE(last_uid,0), COALESCE(uid_validity,0), COALESCE(ai_provider_config,'{}'), COALESCE(signature,''), COALESCE(is_active,1), COALESCE(last_sync_error,''), COALESCE(last_sync_at, '0001-01-01T00:00:00Z'), COALESCE(is_locked, 0), COALESCE(avatar_url, ''), COALESCE(color, ''), COALESCE(sort_order, 0), COALESCE(smart_categories, 1), COALESCE(is_gmail, 0) FROM accounts WHERE id = ? AND is_active = 1`
	row := s.db.QueryRowContext(ctx, query, id)
	var a models.Account
	var imapSSL, smtpSSL, isActive, isLocked, smartCategories, isGmail int
	var lastSyncAt sql.NullString
	err := row.Scan(&a.ID, &a.Email, &a.Name, &a.Provider, &a.IMAPHost, &a.IMAPPort, &imapSSL, &a.IMAPEncryption, &a.SMTPHost, &a.SMTPPort, &smtpSSL, &a.SMTPEncryption, &a.Username, &a.PasswordEncrypted, &a.OAuthAccessToken, &a.OAuthRefreshToken, &a.LastUID, &a.UIDValidity, &a.AIProviderConfig, &a.Signature, &isActive, &a.LastSyncError, &lastSyncAt, &isLocked, &a.AvatarURL, &a.Color, &a.SortOrder, &smartCategories, &isGmail)
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
	a.IsGmail = isGmail == 1
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

func (s *Storage) CreateAccount(ctx context.Context, email, name, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username, password, aiConfig, signature string) (*models.Account, error) {
	encPassword, err := s.encryptPassword(password, "imap_password")
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	now := formatTime(time.Now())

	query := `INSERT INTO accounts (id, email, name, provider, imap_host, imap_port, imap_ssl, imap_encryption, smtp_host, smtp_port, smtp_ssl, smtp_encryption, username, password_encrypted, ai_provider_config, signature, created_at, updated_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.ExecContext(ctx, query, id, email, name, provider, imapHost, imapPort, boolToInt(imapSSL), imapEncryption, smtpHost, smtpPort, boolToInt(smtpSSL), smtpEncryption, username, encPassword, aiConfig, signature, now, now, now)
	if err != nil {
		return nil, err
	}

	return &models.Account{
		ID:              id,
		Email:           email,
		Name:            name,
		Provider:        provider,
		IMAPHost:        imapHost,
		IMAPPort:        int32(imapPort),
		IMAPSSL:         imapSSL,
		IMAPEncryption:  imapEncryption,
		SMTPHost:        smtpHost,
		SMTPPort:        int32(smtpPort),
		SMTPSSL:         smtpSSL,
		SMTPEncryption:  smtpEncryption,
		Username:        username,
		IsActive:        true,
		Signature:       signature,
		SmartCategories: true,
		LastSeenAt:      time.Now(),
	}, nil
}

func (s *Storage) UpdateAccountTimestamp(ctx context.Context, id string, field string) error {
	switch field {
	case "set_absent":
		_, err := s.db.ExecContext(ctx, "UPDATE accounts SET absent_since = datetime('now') WHERE id = ?", id)
		return err
	case "clear_absent":
		_, err := s.db.ExecContext(ctx, "UPDATE accounts SET absent_since = NULL WHERE id = ?", id)
		return err
	default:
		return nil
	}
}

func (s *Storage) UpdateAccount(ctx context.Context, id, email, name, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username, password, aiConfig, signature string) (*models.Account, error) {
	var err error
	if password != "" {
		encPassword, err := s.encryptPassword(password, "imap_password")
		if err != nil {
			return nil, err
		}
		_, err = s.db.ExecContext(ctx,
			`UPDATE accounts SET email=?, name=?, provider=?, imap_host=?, imap_port=?, imap_ssl=?, imap_encryption=?, smtp_host=?, smtp_port=?, smtp_ssl=?, smtp_encryption=?, username=?, password_encrypted=?, ai_provider_config=?, signature=?, updated_at=? WHERE id=? AND is_active=1`,
			email, name, provider, imapHost, imapPort, boolToInt(imapSSL), imapEncryption, smtpHost, smtpPort, boolToInt(smtpSSL), smtpEncryption, username, encPassword, aiConfig, signature, formatTime(time.Now()), id)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE accounts SET email=?, name=?, provider=?, imap_host=?, imap_port=?, imap_ssl=?, imap_encryption=?, smtp_host=?, smtp_port=?, smtp_ssl=?, smtp_encryption=?, username=?, ai_provider_config=?, signature=?, updated_at=? WHERE id=? AND is_active=1`,
			email, name, provider, imapHost, imapPort, boolToInt(imapSSL), imapEncryption, smtpHost, smtpPort, boolToInt(smtpSSL), smtpEncryption, username, aiConfig, signature, formatTime(time.Now()), id)
	}
	if err != nil {
		return nil, err
	}
	// Return updated account without credentials
	return s.GetAccount(ctx, id)
}

func (s *Storage) DeleteAccount(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE accounts SET is_active = 0 WHERE id = ?", id)
	return err
}

func (s *Storage) UpdateAccountTokens(ctx context.Context, id string, accessToken, refreshToken string) error {
	encAccess, err := s.encryptPassword(accessToken, "oauth_token")
	if err != nil {
		return err
	}
	encRefresh, err := s.encryptPassword(refreshToken, "oauth_token")
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, "UPDATE accounts SET oauth_access_token = ?, oauth_refresh_token = ?, updated_at = ? WHERE id = ?", encAccess, encRefresh, formatTime(time.Now()), id)
	return err
}

func (s *Storage) UpdateAccountOAuth(ctx context.Context, id, provider, imapHost string, imapPort int, imapSSL bool, imapEncryption, smtpHost string, smtpPort int, smtpSSL bool, smtpEncryption, username string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE accounts SET provider=?, imap_host=?, imap_port=?, imap_ssl=?, imap_encryption=?, smtp_host=?, smtp_port=?, smtp_ssl=?, smtp_encryption=?, username=?, is_active=1, updated_at=? WHERE id=?`,
		provider, imapHost, imapPort, boolToInt(imapSSL), imapEncryption, smtpHost, smtpPort, boolToInt(smtpSSL), smtpEncryption, username, formatTime(time.Now()), id)
	return err
}

func (s *Storage) UpdateAccountSyncError(ctx context.Context, id string, errText string) error {
	if errText == "" {
		_, err := s.db.ExecContext(ctx, `UPDATE accounts SET last_sync_error = '', last_sync_at = ? WHERE id = ?`, formatTime(time.Now()), id)
		return err
	}
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET last_sync_error = ? WHERE id = ?`, errText, id)
	return err
}

func (s *Storage) UpdateAccountUIDValidity(ctx context.Context, id string, uidValidity uint32) error {
	_, err := s.db.ExecContext(ctx, "UPDATE accounts SET uid_validity = ? WHERE id = ?", uidValidity, id)
	return err
}

func (s *Storage) UpdateAccountLastUID(ctx context.Context, id string, lastUID uint32) error {
	_, err := s.db.ExecContext(ctx, "UPDATE accounts SET last_uid = ? WHERE id = ?", lastUID, id)
	return err
}

func (s *Storage) UpdateAccountIsGmail(ctx context.Context, accountID string, isGmail bool) error {
	_, err := s.db.ExecContext(ctx, `UPDATE accounts SET is_gmail = ? WHERE id = ?`, boolToInt(isGmail), accountID)
	return err
}

// ============================================================================
// Folders
// ============================================================================

func (s *Storage) ResetAccountSync(ctx context.Context, accountID string) error {
	// 1. Delete physical files for attachments
	attRows, err := s.db.QueryContext(ctx, "SELECT path FROM attachments WHERE account_id = ? AND path IS NOT NULL AND path != ''", accountID)
	if err == nil {
		var paths []string
		for attRows.Next() {
			var p sql.NullString
			if err := attRows.Scan(&p); err == nil && p.Valid {
				paths = append(paths, p.String)
			}
		}
		attRows.Close()
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}

	// 2. Delete physical files for email bodies
	emailRows, err := s.db.QueryContext(ctx, "SELECT body_path FROM emails WHERE account_id = ? AND body_path IS NOT NULL AND body_path != ''", accountID)
	if err == nil {
		var paths []string
		for emailRows.Next() {
			var p sql.NullString
			if err := emailRows.Scan(&p); err == nil && p.Valid {
				paths = append(paths, p.String)
			}
		}
		emailRows.Close()
		for _, p := range paths {
			_ = os.Remove(p)
		}
	}

	// Delete existing emails and attachments so they get re-processed with current code
	if _, err := s.db.ExecContext(ctx, "DELETE FROM attachments WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("delete attachments: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM email_labels WHERE email_id IN (SELECT id FROM emails WHERE account_id = ?)", accountID); err != nil {
		return fmt.Errorf("delete email_labels: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM email_tags WHERE email_id IN (SELECT id FROM emails WHERE account_id = ?)", accountID); err != nil {
		return fmt.Errorf("delete email_tags: %w", err)
	}
	// Clean FTS5 virtual table before deleting emails (no FK cascade support).
	if _, err := s.db.ExecContext(ctx, "DELETE FROM emails_fts WHERE email_id IN (SELECT id FROM emails WHERE account_id = ?)", accountID); err != nil {
		return fmt.Errorf("clear FTS index: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM emails WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("delete emails: %w", err)
	}
	// Clear sync queue so completed tasks from the previous sync don't block re-enqueue.
	if _, err := s.db.ExecContext(ctx, "DELETE FROM email_sync_queue WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("clear sync queue: %w", err)
	}
	// Clear related tables that reference now-deleted emails or pending operations.
	if _, err := s.db.ExecContext(ctx, "DELETE FROM imap_move_queue WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("clear move queue: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM scheduled_emails WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("clear scheduled emails: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM email_comments WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("clear email comments: %w", err)
	}
	// Reset all folder sync positions for this account
	if _, err := s.db.ExecContext(ctx, "UPDATE folders SET last_sync_uid = 0, uid_validity = 0 WHERE account_id = ?", accountID); err != nil {
		return fmt.Errorf("reset folders: %w", err)
	}
	// Reset account-level UID tracking
	if _, err := s.db.ExecContext(ctx, "UPDATE accounts SET last_uid = 0, uid_validity = 0 WHERE id = ?", accountID); err != nil {
		return fmt.Errorf("reset account: %w", err)
	}
	return nil
}

// ============================================================================
// Attachments
// ============================================================================

func (s *Storage) SetGroupAccounts(ctx context.Context, groupID string, accountIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "DELETE FROM project_group_accounts WHERE group_id = ?", groupID); err != nil {
		return err
	}
	if len(accountIDs) > 0 {
		var placeholders []string
		var args []interface{}
		for _, aid := range accountIDs {
			placeholders = append(placeholders, "(?, ?)")
			args = append(args, groupID, aid)
		}
		query := fmt.Sprintf("INSERT OR IGNORE INTO project_group_accounts (group_id, account_id) VALUES %s", strings.Join(placeholders, ","))
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Storage) GetGroupAccounts(ctx context.Context, groupID string) ([]string, error) {
	// Lock enforcement is done at handler level via LicenseMgr (position-based, not DB flag)
	rows, err := s.db.QueryContext(ctx, "SELECT account_id FROM project_group_accounts WHERE group_id = ?", groupID)
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

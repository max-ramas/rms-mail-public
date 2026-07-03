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

func (s *Storage) GetRules(ctx context.Context, accountID string) ([]models.FilterRule, error) {
	query := "SELECT id, account_id, name, enabled, condition_field, condition_operator, condition_value, action_type, action_value, priority, ai_provider, ai_model, webhook_secret FROM filter_rules WHERE account_id = ? ORDER BY priority"
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

func (s *Storage) CreateRule(ctx context.Context, r models.FilterRule) (*models.FilterRule, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO filter_rules (id, account_id, name, enabled, condition_field, condition_operator, condition_value, action_type, action_value, priority, ai_provider, ai_model, webhook_secret)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, r.AccountID, r.Name, boolToInt(r.Enabled), r.ConditionField, r.ConditionOperator, r.ConditionValue, r.ActionType, r.ActionValue, r.Priority, r.AIProvider, r.AIModel, r.WebhookSecret)
	if err != nil {
		return nil, err
	}
	r.ID = id
	return &r, nil
}

func (s *Storage) UpdateRule(ctx context.Context, id string, r models.FilterRule) (*models.FilterRule, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE filter_rules SET name=?, enabled=?, condition_field=?, condition_operator=?, condition_value=?, action_type=?, action_value=?, priority=?, ai_provider=?, ai_model=?, webhook_secret=? WHERE id=?`,
		r.Name, boolToInt(r.Enabled), r.ConditionField, r.ConditionOperator, r.ConditionValue, r.ActionType, r.ActionValue, r.Priority, r.AIProvider, r.AIModel, r.WebhookSecret, id)
	if err != nil {
		return nil, err
	}
	r.ID = id
	return &r, nil
}

func (s *Storage) DeleteRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM filter_rules WHERE id = ?", id)
	return err
}

// ============================================================================
// Contacts
// ============================================================================

func (s *Storage) GetContacts(ctx context.Context, accountID string) ([]models.Contact, error) {
	var query string
	var rows *sql.Rows
	var err error
	if accountID == "" {
		query = `SELECT id, COALESCE(account_id, ''), email, name, COALESCE(phone,''), COALESCE(notes,''), company, position, tags FROM contacts ORDER BY name ASC, email ASC`
		rows, err = s.db.QueryContext(ctx, query)
	} else {
		query = `SELECT id, COALESCE(account_id, ''), email, name, COALESCE(phone,''), COALESCE(notes,''), company, position, tags FROM contacts WHERE account_id = ? ORDER BY name ASC, email ASC`
		rows, err = s.db.QueryContext(ctx, query, accountID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var contacts []models.Contact
	for rows.Next() {
		var c models.Contact
		if err := rows.Scan(&c.ID, &c.AccountID, &c.Address, &c.Name, &c.Phone, &c.Notes, &c.Company, &c.Position, &c.Tags); err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}
	return contacts, rows.Err()
}

func (s *Storage) GetContact(ctx context.Context, id string) (*models.Contact, error) {
	query := "SELECT id, COALESCE(account_id, ''), email, name, COALESCE(phone, ''), COALESCE(notes, ''), company, position, tags FROM contacts WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var c models.Contact
	if err := row.Scan(&c.ID, &c.AccountID, &c.Address, &c.Name, &c.Phone, &c.Notes, &c.Company, &c.Position, &c.Tags); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &c, nil
}

func (s *Storage) CreateContact(ctx context.Context, contact models.Contact) (*models.Contact, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO contacts (id, account_id, email, name, phone, notes, company, position, tags) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, contact.AccountID, contact.Address, contact.Name, contact.Phone, contact.Notes, contact.Company, contact.Position, contact.Tags)
	if err != nil {
		return nil, err
	}
	contact.ID = id
	return &contact, nil
}

func (s *Storage) UpdateContact(ctx context.Context, id string, contact models.Contact) (*models.Contact, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE contacts SET email = ?, name = ?, phone = ?, notes = ?, company = ?, position = ?, tags = ?, updated_at = ? WHERE id = ?`,
		contact.Address, contact.Name, contact.Phone, contact.Notes, contact.Company, contact.Position, contact.Tags, formatTime(time.Now()), id)
	if err != nil {
		return nil, err
	}
	contact.ID = id
	return &contact, nil
}

func (s *Storage) DeleteContact(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM contacts WHERE id = ?", id)
	return err
}

// ============================================================================
// Identities
// ============================================================================

func (s *Storage) CreateIdentity(ctx context.Context, accountID, email, name string) (*models.Identity, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO identities (id, account_id, email, name) VALUES (?, ?, ?, ?)`,
		id, accountID, email, name)
	if err != nil {
		return nil, err
	}
	return &models.Identity{ID: id, AccountID: accountID, Email: email, Name: name}, nil
}

func (s *Storage) DeleteIdentity(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM identities WHERE id = ?", id)
	return err
}

// ============================================================================
// Unread counts
// ============================================================================

// GetEmailsCursor returns emails using keyset pagination (O(1) regardless of page depth).

func (s *Storage) GetTemplates(ctx context.Context, accountID string) ([]models.Template, error) {
	query := "SELECT id, account_id, name, subject, body, created_at FROM templates WHERE account_id = ? ORDER BY name"
	rows, err := s.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []models.Template
	for rows.Next() {
		var t models.Template
		var createdAt sql.NullString
		if err := rows.Scan(&t.ID, &t.AccountID, &t.Name, &t.Subject, &t.Body, &createdAt); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(createdAt)
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (s *Storage) CreateTemplate(ctx context.Context, accountID, name, subject, body string) (*models.Template, error) {
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx, `INSERT INTO templates (id, account_id, name, subject, body, created_at) VALUES (?, ?, ?, ?, ?, ?)`, id, accountID, name, subject, body, now)
	if err != nil {
		return nil, err
	}
	return &models.Template{ID: id, AccountID: accountID, Name: name, Subject: subject, Body: body, CreatedAt: time.Now()}, nil
}

func (s *Storage) DeleteTemplate(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM templates WHERE id = ?", id)
	return err
}

// ============================================================================
// Sender Profiles
// ============================================================================

func (s *Storage) GetGroups(ctx context.Context) ([]models.ProjectGroupWithCount, error) {
	return []models.ProjectGroupWithCount{}, nil
}

func (s *Storage) CreateGroup(ctx context.Context, name, color string, sortOrder int) (*models.ProjectGroup, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `INSERT INTO project_groups (id, name, color, sort_order) VALUES (?, ?, ?, ?)`, id, name, color, sortOrder)
	if err != nil {
		return nil, err
	}
	return &models.ProjectGroup{ID: id, Name: name, Color: color, SortOrder: sortOrder}, nil
}

func (s *Storage) UpdateGroup(ctx context.Context, id, name, color string, sortOrder int) (*models.ProjectGroup, error) {
	_, err := s.db.ExecContext(ctx, `UPDATE project_groups SET name=?, color=?, sort_order=? WHERE id=?`, name, color, sortOrder, id)
	if err != nil {
		return nil, err
	}
	return &models.ProjectGroup{ID: id, Name: name, Color: color, SortOrder: sortOrder}, nil
}

func (s *Storage) DeleteGroup(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM project_groups WHERE id = ?", id)
	return err
}

func (s *Storage) GetUsers(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, email, name, role FROM users ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Storage) CreateUser(ctx context.Context, email, name, role string) (*models.User, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, email, name, role) VALUES (?, ?, ?, ?)`, id, email, name, role)
	if err != nil {
		return nil, err
	}
	return &models.User{ID: id, Email: email, Name: name, Role: role}, nil
}

func (s *Storage) UpsertUser(ctx context.Context, email, name, role string) (*models.User, error) {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, role) VALUES (?, ?, ?, ?)
		 ON CONFLICT(email) DO UPDATE SET role = excluded.role, name = excluded.name`,
		id, email, name, role)
	if err != nil {
		return nil, err
	}
	return &models.User{ID: id, Email: email, Name: name, Role: role}, nil
}

func (s *Storage) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, role FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Storage) DeleteUser(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

// ============================================================================
// Assignment (stubs for Mono)
// ============================================================================

func (s *Storage) GetComments(ctx context.Context, emailID string) ([]models.EmailComment, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, email_id, author_id, body, internal, created_at FROM email_comments WHERE email_id = ? ORDER BY created_at", emailID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.EmailComment
	for rows.Next() {
		var c models.EmailComment
		var createdAt sql.NullString
		if err := rows.Scan(&c.ID, &c.EmailID, &c.AuthorID, &c.Body, &c.Internal, &createdAt); err != nil {
			return nil, err
		}
		c.CreatedAt = parseTime(createdAt)
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

func (s *Storage) CreateComment(ctx context.Context, emailID, authorID, body string, internal bool) (*models.EmailComment, error) {
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO email_comments (id, email_id, author_id, body, internal, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, emailID, authorID, body, boolToInt(internal), now)
	if err != nil {
		return nil, err
	}
	return &models.EmailComment{ID: id, EmailID: emailID, AuthorID: authorID, Body: body, Internal: internal, CreatedAt: time.Now()}, nil
}

func (s *Storage) DeleteComment(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM email_comments WHERE id = ?", id)
	return err
}

// ============================================================================
// IMAP Move Queue (stub for Mono)
// ============================================================================

func (s *Storage) IsTelegramUserAllowed(ctx context.Context, userID int64) bool {
	if userID == 0 {
		return false
	}
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM admins WHERE telegram_user_id = ? AND telegram_ai_chat = 1", userID).Scan(&exists)
	if err == nil && exists == 1 {
		return true
	}
	// Also check in users table (for agents)
	err = s.db.QueryRowContext(ctx, "SELECT 1 FROM users WHERE telegram_user_id = ?", userID).Scan(&exists)
	return err == nil && exists == 1
}

func (s *Storage) GetTemplate(ctx context.Context, id string) (*models.Template, error) {
	query := "SELECT id, COALESCE(account_id, ''), name, COALESCE(subject, ''), COALESCE(body, ''), created_at FROM templates WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var t models.Template
	var createdAt sql.NullString
	if err := row.Scan(&t.ID, &t.AccountID, &t.Name, &t.Subject, &t.Body, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	t.CreatedAt = parseTime(createdAt)
	return &t, nil
}

func (s *Storage) GetRule(ctx context.Context, id string) (*models.FilterRule, error) {
	query := "SELECT id, COALESCE(account_id, ''), name, COALESCE(enabled, true), condition_field, condition_operator, condition_value, action_type, COALESCE(action_value, ''), COALESCE(priority, 0) FROM filter_rules WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var r models.FilterRule
	if err := row.Scan(&r.ID, &r.AccountID, &r.Name, &r.Enabled, &r.ConditionField, &r.ConditionOperator, &r.ConditionValue, &r.ActionType, &r.ActionValue, &r.Priority); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &r, nil
}

func (s *Storage) GetComment(ctx context.Context, id string) (*models.EmailComment, error) {
	query := "SELECT id, COALESCE(email_id, ''), COALESCE(account_id, ''), COALESCE(author_id, ''), body, COALESCE(internal, true), created_at FROM email_comments WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var c models.EmailComment
	var createdAt sql.NullString
	if err := row.Scan(&c.ID, &c.EmailID, &c.AccountID, &c.AuthorID, &c.Body, &c.Internal, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	c.CreatedAt = parseTime(createdAt)
	return &c, nil
}

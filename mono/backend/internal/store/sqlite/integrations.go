package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"rmsmail/internal/models"

	"github.com/google/uuid"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

func (s *Storage) LogAI(ctx context.Context, action, provider, model string, promptTokens, completionTokens, durationMs int, status string) error {
	total := promptTokens + completionTokens
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO ai_log (id, action, provider, model, prompt_tokens, completion_tokens, total_tokens, duration_ms, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, action, provider, model, promptTokens, completionTokens, total, durationMs, status, now)
	return err
}

func (s *Storage) GetAIStats(ctx context.Context) (*models.AILogStats, error) {
	var stats models.AILogStats
	stats.ByAction = make(map[string]int)
	stats.ByProvider = make(map[string]int)

	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(SUM(total_tokens),0) FROM ai_log").Scan(&stats.TotalActions, &stats.TotalTokens)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, "SELECT action, COUNT(*) FROM ai_log GROUP BY action")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var action string
		var count int
		if err := rows.Scan(&action, &count); err != nil {
			return nil, err
		}
		stats.ByAction[action] = count
	}

	rows2, err := s.db.QueryContext(ctx, "SELECT provider, COUNT(*) FROM ai_log GROUP BY provider")
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var provider string
		var count int
		if err := rows2.Scan(&provider, &count); err != nil {
			return nil, err
		}
		stats.ByProvider[provider] = count
	}

	return &stats, nil
}

func (s *Storage) ResetAIStats(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM ai_log")
	return err
}

func (s *Storage) GetAILog(ctx context.Context, limit int) ([]models.AILogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, action, provider, model, prompt_tokens, completion_tokens, total_tokens, duration_ms, status, created_at
		 FROM ai_log ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []models.AILogEntry
	for rows.Next() {
		var e models.AILogEntry
		var createdAt sql.NullString
		if err := rows.Scan(&e.ID, &e.Action, &e.Provider, &e.Model, &e.PromptTokens, &e.CompletionTokens, &e.TotalTokens, &e.DurationMs, &e.Status, &createdAt); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(createdAt)
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// ============================================================================
// MCP Keys (stubs for Mono)
// ============================================================================

func (s *Storage) CreateMCPKey(ctx context.Context, name, accountID, keyHash, keyPrefix, rawKey string) (*models.MCPKey, error) {
	encKey, err := s.encryptPassword(rawKey, "mcp_key")
	if err != nil {
		return nil, err
	}
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO mcp_keys (id, name, account_id, key_hash, key_prefix, key_encrypted, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, name, accountID, keyHash, keyPrefix, encKey, now)
	if err != nil {
		return nil, err
	}
	return &models.MCPKey{ID: id, Name: name, AccountID: accountID, KeyPrefix: keyPrefix, IsActive: true, CreatedAt: time.Now()}, nil
}

func (s *Storage) ListMCPKeys(ctx context.Context) ([]models.MCPKey, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(account_id, ''), key_prefix, is_active, COALESCE(last_used_at, ''), created_at FROM mcp_keys ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []models.MCPKey
	for rows.Next() {
		var k models.MCPKey
		var lastUsedAt, createdAt sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.AccountID, &k.KeyPrefix, &k.IsActive, &lastUsedAt, &createdAt); err != nil {
			return nil, err
		}
		if lastUsedAt.Valid && lastUsedAt.String != "" {
			t := parseTime(lastUsedAt)
			k.LastUsedAt = &t
		}
		k.CreatedAt = parseTime(createdAt)
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *Storage) DeleteMCPKey(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM mcp_keys WHERE id = ?", id)
	return err
}

func (s *Storage) GetMCPKey(ctx context.Context, id string) (*models.MCPKey, error) {
	var k models.MCPKey
	err := s.db.QueryRowContext(ctx, "SELECT id, COALESCE(account_id, ''), name, COALESCE(is_active, 1) FROM mcp_keys WHERE id = ?", id).Scan(&k.ID, &k.AccountID, &k.Name, &k.IsActive)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}

func (s *Storage) GetMCPKeyFull(ctx context.Context, id string) (string, error) {
	var encKey string
	err := s.db.QueryRowContext(ctx, "SELECT key_encrypted FROM mcp_keys WHERE id = ? AND is_active = 1", id).Scan(&encKey)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return s.decryptPassword(encKey, "mcp_key")
}

// GetMCPKeyByAPIKey finds the active MCP key record (with accountID) by matching the raw API key.

func (s *Storage) GetMCPKeyByAPIKey(ctx context.Context, apiKey string) (*models.MCPKey, error) {
	if len(apiKey) < 8 {
		return nil, nil
	}
	prefix := apiKey[:8]
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, COALESCE(account_id,''), key_prefix, key_encrypted, is_active,
		        COALESCE(last_used_at,''), created_at
		 FROM mcp_keys WHERE key_prefix = ? AND is_active = 1`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k models.MCPKey
		var encKey string
		var lastUsedAt, createdAt sql.NullString
		if err := rows.Scan(&k.ID, &k.Name, &k.AccountID, &k.KeyPrefix, &encKey, &k.IsActive, &lastUsedAt, &createdAt); err != nil {
			continue
		}
		if lastUsedAt.Valid && lastUsedAt.String != "" {
			t := parseTime(lastUsedAt)
			k.LastUsedAt = &t
		}
		k.CreatedAt = parseTime(createdAt)
		fullKey, err := s.decryptPassword(encKey, "mcp_key")
		if err == nil && fullKey == apiKey {
			return &k, nil
		}
	}
	return nil, rows.Err()
}

// ============================================================================
// Admin auth
// ============================================================================

func (s *Storage) GetTelegramSettings(ctx context.Context, email string) (userID int64, enabled bool, aiNotifications bool, aiChat bool, botToken string, err error) {
	var uid sql.NullInt64
	var en, aiNotif, aiCh sql.NullInt64
	var tok sql.NullString

	err = s.db.QueryRowContext(ctx,
		"SELECT telegram_user_id, telegram_enabled, telegram_ai_notifications, telegram_ai_chat, telegram_bot_token FROM admins WHERE email = ?",
		email).Scan(&uid, &en, &aiNotif, &aiCh, &tok)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, false, false, "", nil
		}
		return 0, false, false, false, "", err
	}
	tokenStr := tok.String
	if tokenStr != "" {
		dec, derr := s.decryptPassword(tokenStr, "telegram_token")
		if derr == nil {
			tokenStr = dec
		}
	}
	return uid.Int64, en.Int64 == 1, aiNotif.Int64 == 1, aiCh.Int64 == 1, tokenStr, nil
}

func (s *Storage) GetAnyTelegramSettings(ctx context.Context) (int64, bool, bool, bool, string, error) {
	var uid sql.NullInt64
	var en, aiNotif, aiCh sql.NullInt64
	var tok sql.NullString

	err := s.db.QueryRowContext(ctx,
		"SELECT telegram_user_id, telegram_enabled, telegram_ai_notifications, telegram_ai_chat, telegram_bot_token FROM admins WHERE telegram_enabled = 1 LIMIT 1",
	).Scan(&uid, &en, &aiNotif, &aiCh, &tok)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, false, false, false, "", nil
		}
		return 0, false, false, false, "", err
	}
	tokenStr := tok.String
	if tokenStr != "" {
		dec, derr := s.decryptPassword(tokenStr, "telegram_token")
		if derr == nil {
			tokenStr = dec
		}
	}
	return uid.Int64, en.Int64 == 1, aiNotif.Int64 == 1, aiCh.Int64 == 1, tokenStr, nil
}

func (s *Storage) UpdateTelegramSettings(ctx context.Context, email string, userID int64, enabled, aiNotifications, aiChat bool, botToken string) error {
	var uid interface{} = userID
	if userID == 0 {
		uid = nil
	}
	enVal := 0
	if enabled {
		enVal = 1
	}
	aiNotifVal := 0
	if aiNotifications {
		aiNotifVal = 1
	}
	aiChVal := 0
	if aiChat {
		aiChVal = 1
	}

	tokenVal := botToken
	if tokenVal != "" {
		enc, err := s.encryptPassword(tokenVal, "telegram_token")
		if err != nil {
			return fmt.Errorf("encrypt telegram token: %w", err)
		}
		tokenVal = enc
	}

	_, err := s.db.ExecContext(ctx,
		"UPDATE admins SET telegram_user_id = ?, telegram_enabled = ?, telegram_ai_notifications = ?, telegram_ai_chat = ?, telegram_bot_token = ? WHERE email = ?",
		uid, enVal, aiNotifVal, aiChVal, tokenVal, email)
	return err
}

func (s *Storage) GetGlobalTelegramBotToken(ctx context.Context) (string, error) {
	var token sql.NullString
	err := s.db.QueryRowContext(ctx, "SELECT telegram_bot_token FROM admins WHERE telegram_bot_token IS NOT NULL AND telegram_bot_token != '' LIMIT 1").Scan(&token)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return token.String, nil
}

func (s *Storage) GetAISettings(ctx context.Context, accountID string) (*models.AISetting, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, account_id, preset, config, prompts, api_keys_encrypted, created_at, updated_at FROM ai_settings WHERE account_id = ?", accountID)
	var setting models.AISetting
	var createdAt, updatedAt sql.NullString
	err := row.Scan(&setting.ID, &setting.AccountID, &setting.Preset, &setting.Config, &setting.Prompts, &setting.APIKeysEncrypted, &createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	setting.CreatedAt = parseTime(createdAt)
	setting.UpdatedAt = parseTime(updatedAt)
	return &setting, nil
}

// GetPresetSettings extracts the provider and model from the config JSONB for a specific preset name.

func (s *Storage) GetPresetSettings(ctx context.Context, accountID string, presetName string) (provider string, model string, err error) {
	setting, err := s.GetAISettings(ctx, accountID)
	if err != nil {
		return "", "", err
	}
	if setting == nil {
		return "", "", nil
	}
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(setting.Config), &config); err != nil {
		return "", "", err
	}
	presetsVal, ok := config["presets"]
	if !ok || presetsVal == nil {
		return "", "", nil
	}
	presets, ok := presetsVal.(map[string]interface{})
	if !ok {
		return "", "", nil
	}
	presetVal, ok := presets[presetName]
	if !ok || presetVal == nil {
		return "", "", nil
	}
	preset, ok := presetVal.(map[string]interface{})
	if !ok {
		return "", "", nil
	}
	if p, ok := preset["provider"].(string); ok {
		provider = p
	}
	if m, ok := preset["model"].(string); ok {
		model = m
	}
	return provider, model, nil
}

func (s *Storage) UpsertAISetting(ctx context.Context, setting models.AISetting) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO ai_settings (id, account_id, preset, config, prompts, api_keys_encrypted, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(account_id) DO UPDATE SET
			preset = excluded.preset,
			config = excluded.config,
			prompts = excluded.prompts,
			api_keys_encrypted = excluded.api_keys_encrypted,
			updated_at = datetime('now')
	`, setting.ID, setting.AccountID, setting.Preset, setting.Config, setting.Prompts, setting.APIKeysEncrypted)
	return err
}

// executeInChunks helps execute an IN query with a large number of parameters

func (s *Storage) GetWebhooks(ctx context.Context, accountID string) ([]models.Webhook, error) {
	query := "SELECT id, account_id, name, url, COALESCE(secret, ''), created_at FROM webhooks WHERE account_id = ? ORDER BY created_at DESC"
	rows, err := s.db.QueryContext(ctx, query, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var webhooks []models.Webhook
	for rows.Next() {
		var w models.Webhook
		var createdAt sql.NullString
		if err := rows.Scan(&w.ID, &w.AccountID, &w.Name, &w.URL, &w.Secret, &createdAt); err != nil {
			return nil, err
		}
		w.CreatedAt = parseTime(createdAt)
		webhooks = append(webhooks, w)
	}
	return webhooks, rows.Err()
}

// --- Singular Getters (for access control) ---

func (s *Storage) GetWebhook(ctx context.Context, id string) (*models.Webhook, error) {
	query := "SELECT id, account_id, name, url, COALESCE(secret, ''), created_at FROM webhooks WHERE id = ?"
	row := s.db.QueryRowContext(ctx, query, id)
	var w models.Webhook
	var createdAt sql.NullString
	if err := row.Scan(&w.ID, &w.AccountID, &w.Name, &w.URL, &w.Secret, &createdAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	w.CreatedAt = parseTime(createdAt)
	return &w, nil
}

func (s *Storage) CreateWebhook(ctx context.Context, accountID, name, urlStr, secret string) (*models.Webhook, error) {
	id := uuid.New().String()
	now := formatTime(time.Now())
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO webhooks (id, account_id, name, url, secret, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, accountID, name, urlStr, secret, now)
	if err != nil {
		return nil, err
	}
	return &models.Webhook{
		ID:        id,
		AccountID: accountID,
		Name:      name,
		URL:       urlStr,
		Secret:    secret,
		HasSecret: secret != "",
		CreatedAt: time.Now(),
	}, nil
}

func (s *Storage) DeleteWebhook(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM webhooks WHERE id = ?", id)
	return err
}

func (s *Storage) EnqueueWebhookRetry(ctx context.Context, id, urlStr, secret string, payload []byte, nextRetryAtUnix int64) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO webhook_retry_queue (id, url, secret, payload, attempt, next_retry_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, urlStr, secret, payload, 0, nextRetryAtUnix)
	return err
}

func (s *Storage) GetDueWebhookRetries(ctx context.Context, nowUnix int64, limit int) ([]models.WebhookRetry, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, url, secret, payload, attempt, next_retry_at FROM webhook_retry_queue WHERE next_retry_at <= ? ORDER BY next_retry_at ASC LIMIT ?",
		nowUnix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var retries []models.WebhookRetry
	for rows.Next() {
		var w models.WebhookRetry
		if err := rows.Scan(&w.ID, &w.URL, &w.Secret, &w.Payload, &w.Attempt, &w.NextRetryAt); err != nil {
			continue
		}
		retries = append(retries, w)
	}
	return retries, rows.Err()
}

func (s *Storage) DeleteWebhookRetry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM webhook_retry_queue WHERE id = ?", id)
	return err
}

func (s *Storage) UpdateWebhookRetryAttempt(ctx context.Context, id string, attempt int, nextRetryAtUnix int64) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE webhook_retry_queue SET attempt = ?, next_retry_at = ? WHERE id = ?",
		attempt, nextRetryAtUnix, id)
	return err
}

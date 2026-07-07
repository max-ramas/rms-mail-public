-- Mono Edition Schema (SQLite)
-- Simplified DDL without PostgreSQL-specific features

-- Admins table for app-level authentication
CREATE TABLE IF NOT EXISTS admins (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Accounts table
CREATE TABLE IF NOT EXISTS accounts (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT DEFAULT '',
    provider TEXT DEFAULT '',
    imap_host TEXT DEFAULT '',
    imap_port INTEGER DEFAULT 993,
    imap_ssl INTEGER DEFAULT 1,
    imap_encryption TEXT DEFAULT 'ssl',
    smtp_host TEXT DEFAULT '',
    smtp_port INTEGER DEFAULT 465,
    smtp_ssl INTEGER DEFAULT 1,
    smtp_encryption TEXT DEFAULT 'ssl',
    username TEXT DEFAULT '',
    password_encrypted TEXT DEFAULT '',
    ai_provider_config TEXT DEFAULT '{}',
    oauth_access_token TEXT DEFAULT '',
    oauth_refresh_token TEXT DEFAULT '',
    is_active INTEGER DEFAULT 1,
    is_locked INTEGER DEFAULT 0,
    last_uid INTEGER DEFAULT 0,
    uid_validity INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    signature TEXT DEFAULT '',
    last_sync_error TEXT DEFAULT '',
    last_sync_at TEXT DEFAULT '0001-01-01T00:00:00Z',
    smart_categories INTEGER DEFAULT 1
);

-- Seed system account for global AI settings (FK-safe sentinel UUID)
INSERT OR IGNORE INTO accounts (id, email, provider, is_active) VALUES ('00000000-0000-0000-0000-000000000000', 'system@rmsmail', 'system', 0);

-- Folders table
CREATE TABLE IF NOT EXISTS folders (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    path TEXT DEFAULT '',
    is_subscribed INTEGER DEFAULT 1,
    last_sync_uid INTEGER DEFAULT 0,
    uid_validity INTEGER DEFAULT 0,
    unread_count INTEGER DEFAULT 0,
    is_inbox INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(account_id, path)
);

CREATE INDEX IF NOT EXISTS idx_folders_is_inbox ON folders (is_inbox);
ALTER TABLE folders ADD COLUMN IF NOT EXISTS name_lower TEXT;
DROP INDEX IF EXISTS idx_folders_name_lower;
CREATE INDEX idx_folders_name_lower ON folders (account_id, name_lower);

-- Emails table
CREATE TABLE IF NOT EXISTS emails (
    id TEXT,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    folder_id TEXT REFERENCES folders(id) ON DELETE SET NULL,
    msg_id TEXT,
    uid INTEGER DEFAULT 0,
    subject TEXT DEFAULT '',
    sender_name TEXT DEFAULT '',
    sender_address TEXT DEFAULT '',
    recipient_address TEXT DEFAULT '',
    date_sent TEXT DEFAULT '0001-01-01T00:00:00Z',
    is_read INTEGER DEFAULT 0,
    is_flagged INTEGER DEFAULT 0,
    is_answered INTEGER DEFAULT 0,
    has_attachments INTEGER DEFAULT 0,
    is_dirty_locally INTEGER DEFAULT 0,
    in_reply_to TEXT DEFAULT '',
    thread_id TEXT DEFAULT '',
    snippet TEXT DEFAULT '',
    draft_reply TEXT DEFAULT '',
    draft_remote_uid INTEGER DEFAULT 0,
    body_path TEXT DEFAULT '',
    spf_pass INTEGER DEFAULT 0,
    dkim_pass INTEGER DEFAULT 0,
    is_pinned INTEGER DEFAULT 0,
    snooze_until TEXT,
    is_muted INTEGER DEFAULT 0,
    smart_category INTEGER DEFAULT 0,
    assigned_to TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (id, account_id)
);

-- Ensure (msg_id, account_id) uniqueness for cross-account support.
CREATE UNIQUE INDEX IF NOT EXISTS emails_msg_id_account_folder_key ON emails (msg_id, account_id, folder_id);

-- Attachments table
CREATE TABLE IF NOT EXISTS attachments (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    filename TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    hash TEXT DEFAULT '',
    path TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Email tags
CREATE TABLE IF NOT EXISTS email_tags (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    tag TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(email_id, tag)
);

-- Templates
CREATE TABLE IF NOT EXISTS templates (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    subject TEXT DEFAULT '',
    body TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Labels
CREATE TABLE IF NOT EXISTS labels (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT '' REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    color TEXT NOT NULL DEFAULT '#3b82f6',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Email-Label junction
CREATE TABLE IF NOT EXISTS email_labels (
    email_id TEXT,
    label_id TEXT REFERENCES labels(id) ON DELETE CASCADE,
    account_id TEXT,
    PRIMARY KEY (email_id, label_id, account_id)
);

-- Filter rules
CREATE TABLE IF NOT EXISTS filter_rules (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    name TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    condition_field TEXT NOT NULL,
    condition_operator TEXT NOT NULL,
    condition_value TEXT NOT NULL,
    action_type TEXT NOT NULL,
    action_value TEXT DEFAULT '',
    ai_provider TEXT DEFAULT '',
    ai_model TEXT DEFAULT '',
    priority INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Project groups
CREATE TABLE IF NOT EXISTS project_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#6366f1',
    sort_order INTEGER DEFAULT 0,
    is_locked INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Project group accounts
CREATE TABLE IF NOT EXISTS project_group_accounts (
    group_id TEXT REFERENCES project_groups(id) ON DELETE CASCADE,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    PRIMARY KEY (group_id, account_id)
);

-- Users
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    role TEXT DEFAULT 'agent',
    telegram_user_id INTEGER,
    whatsapp_phone TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Seed system user for anonymous comment authorship (always exists, FK-safe)
INSERT OR IGNORE INTO users (id, email, name, role) VALUES ('00000000-0000-0000-0000-000000000000', 'system@rmsmail', 'System', 'admin');

-- Email comments
CREATE TABLE IF NOT EXISTS email_comments (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    author_id TEXT REFERENCES users(id),
    body TEXT NOT NULL,
    internal INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Contacts
CREATE TABLE IF NOT EXISTS contacts (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL DEFAULT '' REFERENCES accounts(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    phone TEXT DEFAULT '',
    notes TEXT DEFAULT '',
    company TEXT NOT NULL DEFAULT '',
    position TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '[]',
    updated_at TEXT DEFAULT (datetime('now')),
    created_at TEXT DEFAULT (datetime('now'))
);

-- NOTE: contacts.account_id exists in CREATE TABLE above but SQLite ALTER TABLE
-- ADD COLUMN cannot carry REFERENCES / FK constraints. The column is re-added here
-- for migration safety; the FK is enforced only when the table is created fresh.
ALTER TABLE contacts ADD COLUMN account_id TEXT DEFAULT '';

-- Sender profiles (avatar cache)
CREATE TABLE IF NOT EXISTS sender_profiles (
    email TEXT PRIMARY KEY,
    name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    resolved_at TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- Identities / Aliases
CREATE TABLE IF NOT EXISTS identities (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    name TEXT DEFAULT ''
);

-- MCP keys
CREATE TABLE IF NOT EXISTS mcp_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    account_id TEXT NOT NULL DEFAULT '',
    key_hash TEXT NOT NULL UNIQUE,
    key_prefix TEXT NOT NULL,
    key_encrypted TEXT NOT NULL DEFAULT '',
    created_by TEXT DEFAULT 'admin',
    is_active INTEGER DEFAULT 1,
    last_used_at TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- AI log
CREATE TABLE IF NOT EXISTS ai_log (
    id TEXT PRIMARY KEY,
    action TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT NOT NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    duration_ms INTEGER DEFAULT 0,
    status TEXT DEFAULT 'success',
    created_at TEXT DEFAULT (datetime('now'))
);

-- IMAP move queue
CREATE TABLE IF NOT EXISTS imap_move_queue (
    id TEXT PRIMARY KEY,
    email_id TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    target_folder_id TEXT REFERENCES folders(id) ON DELETE CASCADE,
    target_folder_name TEXT DEFAULT '',
    source_folder_name TEXT DEFAULT '',
    remote_uid INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    last_error TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(email_id)
);

ALTER TABLE attachments ADD COLUMN content_id TEXT DEFAULT '';

ALTER TABLE emails ADD COLUMN status TEXT DEFAULT 'new';
ALTER TABLE emails ADD COLUMN first_response_at TEXT;
ALTER TABLE emails ADD COLUMN resolved_at TEXT;

-- Indexes
CREATE INDEX IF NOT EXISTS idx_emails_account_date ON emails(account_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_pagination ON emails(account_id, date_sent DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(folder_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_folder_isread ON emails(folder_id, is_read, is_muted);
CREATE INDEX IF NOT EXISTS idx_emails_folder_read_sent ON emails(folder_id, is_read, is_muted, is_pinned DESC, date_sent DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_folders_account ON folders(account_id);
CREATE INDEX IF NOT EXISTS idx_email_tags_email ON email_tags(email_id);
CREATE INDEX IF NOT EXISTS idx_email_tags_tag ON email_tags(tag);
CREATE INDEX IF NOT EXISTS idx_email_comments_email_account ON email_comments(email_id, account_id);
CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(email);
CREATE INDEX IF NOT EXISTS idx_emails_sender_address ON emails(sender_address);
CREATE INDEX IF NOT EXISTS idx_imap_move_queue_account ON imap_move_queue(account_id, retry_count);
CREATE INDEX IF NOT EXISTS idx_emails_thread ON emails(account_id, thread_id);

-- AI provider/model in filter rules
ALTER TABLE filter_rules ADD COLUMN ai_provider TEXT DEFAULT '';
ALTER TABLE filter_rules ADD COLUMN ai_model TEXT DEFAULT '';
ALTER TABLE filter_rules ADD COLUMN webhook_secret TEXT DEFAULT '';

-- Telegram settings for admins
ALTER TABLE admins ADD COLUMN telegram_user_id INTEGER;
ALTER TABLE admins ADD COLUMN telegram_enabled INTEGER DEFAULT 0;
ALTER TABLE admins ADD COLUMN telegram_ai_notifications INTEGER DEFAULT 0;
ALTER TABLE admins ADD COLUMN telegram_ai_chat INTEGER DEFAULT 0;
ALTER TABLE admins ADD COLUMN telegram_bot_token TEXT;

-- AI Settings (Phase P0.1 — secure backend storage for AI keys)
CREATE TABLE IF NOT EXISTS ai_settings (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    preset TEXT DEFAULT 'custom',
    config TEXT DEFAULT '{}',
    prompts TEXT DEFAULT '{}',
    api_keys_encrypted TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(account_id)
);

ALTER TABLE accounts ADD COLUMN absent_since TEXT;
ALTER TABLE accounts ADD COLUMN system_discovered INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN last_seen_at TEXT DEFAULT '0001-01-01T00:00:00Z';
ALTER TABLE accounts ADD COLUMN name TEXT DEFAULT '';
ALTER TABLE emails ADD COLUMN cc_address TEXT DEFAULT '';
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_manual INTEGER DEFAULT 0;

-- Background Jobs Queue
CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    payload TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    attempt INTEGER DEFAULT 0,
    next_run_at TEXT DEFAULT (datetime('now')),
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_next_run ON jobs(status, next_run_at);

CREATE TABLE IF NOT EXISTS webhooks (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    secret TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS scheduled_emails (
    id TEXT PRIMARY KEY,
    account_id TEXT REFERENCES accounts(id) ON DELETE CASCADE,
    email_data TEXT NOT NULL,
    status TEXT DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Schema migrations (safe due to InitSchema ignoring duplicate column errors)
ALTER TABLE accounts ADD COLUMN last_sync_error TEXT DEFAULT '';
ALTER TABLE accounts ADD COLUMN last_sync_at TEXT DEFAULT '0001-01-01';
ALTER TABLE accounts ADD COLUMN avatar_url TEXT DEFAULT '';
ALTER TABLE accounts ADD COLUMN color TEXT DEFAULT '';
ALTER TABLE accounts ADD COLUMN sort_order INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS webhook_retry_queue (
    id TEXT PRIMARY KEY,
    url TEXT NOT NULL,
    secret TEXT,
    payload BLOB NOT NULL,
    attempt INTEGER DEFAULT 0,
    next_retry_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_webhook_queue_retry ON webhook_retry_queue(next_retry_at);

ALTER TABLE accounts ADD COLUMN system_discovered INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN is_locked INTEGER DEFAULT 0;
ALTER TABLE accounts ADD COLUMN smart_categories INTEGER DEFAULT 1;
ALTER TABLE folders ADD COLUMN unread_count INTEGER DEFAULT 0;

CREATE TABLE IF NOT EXISTS email_sync_queue (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    folder_name TEXT NOT NULL,
    uid INTEGER NOT NULL,
    priority INTEGER DEFAULT 0,
    status TEXT DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_queue_fetch_active
ON email_sync_queue (account_id, priority DESC, created_at ASC)
WHERE status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_acc_folder_uid ON email_sync_queue (account_id, folder_name, uid);
ALTER TABLE emails ADD COLUMN smart_category INTEGER DEFAULT 0;
-- Fix existing rows where smart_category was added after data existed (NULL != 0 in SQL)
UPDATE emails SET smart_category = 0 WHERE smart_category IS NULL;

-- Migration: populate is_inbox
UPDATE folders SET is_inbox = 1 WHERE LOWER(name) = 'inbox';

-- Migration: dirty job to denormalize unread_count
UPDATE folders
SET unread_count = (
    SELECT COUNT(*)
    FROM emails e
    WHERE e.folder_id = folders.id AND e.is_read = 0 AND e.is_muted = 0
)
WHERE unread_count = 0;

-- Attachments performance indexes for CAS dedup and email-attachment lookups
CREATE INDEX IF NOT EXISTS idx_attachments_hash ON attachments(hash);
CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id, created_at);

-- Migration 025: Gmail labels support
-- Phase 1: Gmail detection flag
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS is_gmail INTEGER DEFAULT 0;

-- Phase 2: Email labels junction table (for Gmail multi-label tracking)
CREATE TABLE IF NOT EXISTS email_labels_junction (
    email_id  TEXT NOT NULL,
    account_id TEXT NOT NULL,
    label     TEXT NOT NULL,
    system    INTEGER DEFAULT 0,
    PRIMARY KEY (email_id, account_id, label)
);
CREATE INDEX IF NOT EXISTS idx_email_labels_junction_label ON email_labels_junction (account_id, label);

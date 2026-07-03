BEGIN TRANSACTION;

-- 1. Rename existing emails table
ALTER TABLE emails RENAME TO emails_legacy;

-- 2. Create the new emails table with composite primary key
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
    date_sent DATETIME,
    is_read BOOLEAN DEFAULT 0,
    is_flagged BOOLEAN DEFAULT 0,
    has_attachments BOOLEAN DEFAULT 0,
    is_dirty_locally BOOLEAN DEFAULT 0,
    in_reply_to TEXT DEFAULT '',
    thread_id TEXT DEFAULT '',
    snippet TEXT,
    draft_reply TEXT DEFAULT '',
    draft_remote_uid INTEGER DEFAULT 0,
    body_path TEXT,
    cc_address TEXT DEFAULT '',
    status TEXT DEFAULT 'new',
    first_response_at DATETIME,
    resolved_at DATETIME,
    spf_pass BOOLEAN,
    dkim_pass BOOLEAN,
    is_pinned BOOLEAN DEFAULT 0,
    snooze_until TEXT,
    is_muted INTEGER DEFAULT 0,
    assigned_to TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (id, account_id)
);

-- Ensure (msg_id, account_id) uniqueness for cross-account support
CREATE UNIQUE INDEX IF NOT EXISTS idx_emails_msg_account_folder ON emails(msg_id, account_id, folder_id);

-- 3. Copy data from legacy using COALESCE
INSERT INTO emails (
    id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, date_sent, is_read, is_flagged, has_attachments, is_dirty_locally, in_reply_to, thread_id, snippet, draft_reply, draft_remote_uid, body_path, cc_address, status, first_response_at, resolved_at, spf_pass, dkim_pass, is_pinned, snooze_until, is_muted, assigned_to, created_at
)
SELECT 
    id, 
    COALESCE(account_id, '00000000-0000-0000-0000-000000000000'), 
    folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, date_sent, is_read, is_flagged, has_attachments, is_dirty_locally, in_reply_to, thread_id, snippet, draft_reply, draft_remote_uid, body_path, cc_address, status, first_response_at, resolved_at, spf_pass, dkim_pass, is_pinned, snooze_until, is_muted, assigned_to, created_at
FROM emails_legacy;

-- 4. Recreate dependent tables to update FOREIGN KEYs

-- Attachments
ALTER TABLE attachments RENAME TO attachments_legacy;
CREATE TABLE IF NOT EXISTS attachments (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    filename TEXT DEFAULT '',
    size INTEGER DEFAULT 0,
    hash TEXT DEFAULT '',
    path TEXT DEFAULT '',
    content_id TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (email_id, account_id) REFERENCES emails(id, account_id) ON DELETE CASCADE
);
INSERT INTO attachments
SELECT id, email_id, COALESCE((SELECT account_id FROM emails WHERE emails.id = attachments_legacy.email_id), '00000000-0000-0000-0000-000000000000'), filename, size, hash, path, content_id, created_at FROM attachments_legacy;
DROP TABLE attachments_legacy;

-- Email Tags
ALTER TABLE email_tags RENAME TO email_tags_legacy;
CREATE TABLE IF NOT EXISTS email_tags (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    tag TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(email_id, tag),
    FOREIGN KEY (email_id, account_id) REFERENCES emails(id, account_id) ON DELETE CASCADE
);
INSERT INTO email_tags
SELECT id, email_id, COALESCE((SELECT account_id FROM emails WHERE emails.id = email_tags_legacy.email_id), '00000000-0000-0000-0000-000000000000'), tag, created_at FROM email_tags_legacy;
DROP TABLE email_tags_legacy;

-- Email Labels
ALTER TABLE email_labels RENAME TO email_labels_legacy;
CREATE TABLE IF NOT EXISTS email_labels (
    email_id TEXT,
    label_id TEXT REFERENCES labels(id) ON DELETE CASCADE,
    account_id TEXT,
    PRIMARY KEY (email_id, label_id, account_id),
    FOREIGN KEY (email_id, account_id) REFERENCES emails(id, account_id) ON DELETE CASCADE
);
INSERT INTO email_labels
SELECT email_id, label_id, COALESCE((SELECT account_id FROM emails WHERE emails.id = email_labels_legacy.email_id), '00000000-0000-0000-0000-000000000000') FROM email_labels_legacy;
DROP TABLE email_labels_legacy;

-- Email Comments
ALTER TABLE email_comments RENAME TO email_comments_legacy;
CREATE TABLE IF NOT EXISTS email_comments (
    id TEXT PRIMARY KEY,
    email_id TEXT,
    account_id TEXT,
    author_id TEXT REFERENCES users(id),
    body TEXT NOT NULL,
    internal INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    FOREIGN KEY (email_id, account_id) REFERENCES emails(id, account_id) ON DELETE CASCADE
);
INSERT INTO email_comments
SELECT id, email_id, COALESCE((SELECT account_id FROM emails WHERE emails.id = email_comments_legacy.email_id), '00000000-0000-0000-0000-000000000000'), author_id, body, internal, created_at FROM email_comments_legacy;
DROP TABLE email_comments_legacy;

-- IMAP Move Queue
ALTER TABLE imap_move_queue RENAME TO imap_move_queue_legacy;
CREATE TABLE IF NOT EXISTS imap_move_queue (
    id TEXT PRIMARY KEY,
    email_id TEXT NOT NULL,
    account_id TEXT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    target_folder_id TEXT REFERENCES folders(id) ON DELETE CASCADE,
    target_folder_name TEXT DEFAULT '',
    remote_uid INTEGER DEFAULT 0,
    retry_count INTEGER DEFAULT 0,
    last_error TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(email_id),
    FOREIGN KEY (email_id, account_id) REFERENCES emails(id, account_id) ON DELETE CASCADE
);
INSERT INTO imap_move_queue
SELECT id, email_id, COALESCE((SELECT account_id FROM emails WHERE emails.id = imap_move_queue_legacy.email_id), '00000000-0000-0000-0000-000000000000'), target_folder_id, target_folder_name, remote_uid, retry_count, last_error, created_at FROM imap_move_queue_legacy;
DROP TABLE imap_move_queue_legacy;

-- 5. Drop legacy
DROP TABLE emails_legacy;

-- Recreate index on new emails table
CREATE INDEX IF NOT EXISTS idx_emails_account_date ON emails(account_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(folder_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_sender_address ON emails(sender_address);

COMMIT;

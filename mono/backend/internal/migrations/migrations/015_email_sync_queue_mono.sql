-- Migration 015: Create email_sync_queue for independent Check Worker (SQLite)

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

-- Partial index to allow fast fetching of pending tasks
CREATE INDEX IF NOT EXISTS idx_sync_queue_fetch_active 
ON email_sync_queue (priority DESC, created_at ASC) 
WHERE status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_acc_folder_uid ON email_sync_queue (account_id, folder_name, uid);

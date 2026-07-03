-- Migration 015: Create email_sync_queue for independent Check Worker

CREATE TABLE IF NOT EXISTS email_sync_queue (
    id BIGSERIAL PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    folder_name VARCHAR(255) NOT NULL,
    uid INT NOT NULL,
    priority INT DEFAULT 0,
    status VARCHAR(50) DEFAULT 'pending',
    attempts INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Partial index to allow fast fetching of pending tasks
CREATE INDEX IF NOT EXISTS idx_sync_queue_fetch_active 
ON email_sync_queue (priority DESC, created_at ASC) 
WHERE status = 'pending';

CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_acc_folder_uid ON email_sync_queue (account_id, folder_name, uid);

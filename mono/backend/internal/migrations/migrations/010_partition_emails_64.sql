BEGIN;

-- 1. Drop foreign keys referencing emails(id)
ALTER TABLE IF EXISTS email_tags DROP CONSTRAINT IF EXISTS email_tags_email_id_fkey;
ALTER TABLE IF EXISTS email_labels DROP CONSTRAINT IF EXISTS email_labels_email_id_fkey;
ALTER TABLE IF EXISTS email_comments DROP CONSTRAINT IF EXISTS email_comments_email_id_fkey;
ALTER TABLE IF EXISTS attachments DROP CONSTRAINT IF EXISTS attachments_email_id_fkey;
ALTER TABLE IF EXISTS imap_move_queue DROP CONSTRAINT IF EXISTS imap_move_queue_email_id_fkey;

-- 2. Add account_id to child tables
ALTER TABLE email_tags ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE email_comments ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;

-- Backfill account_id
UPDATE email_tags et SET account_id = e.account_id FROM emails e WHERE et.email_id = e.id AND et.account_id IS NULL;
UPDATE email_comments ec SET account_id = e.account_id FROM emails e WHERE ec.email_id = e.id AND ec.account_id IS NULL;
UPDATE attachments a SET account_id = e.account_id FROM emails e WHERE a.email_id = e.id AND a.account_id IS NULL;

-- 3. Rename old table
ALTER TABLE emails RENAME TO emails_legacy;

-- 4. Create new partitioned table
CREATE TABLE emails (
    id UUID DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    folder_id UUID REFERENCES folders(id) ON DELETE SET NULL,
    msg_id VARCHAR(255),
    uid INT,
    subject TEXT,
    sender_name VARCHAR(255),
    sender_address VARCHAR(255),
    recipient_address TEXT,
    date_sent TIMESTAMP WITH TIME ZONE,
    is_read BOOLEAN DEFAULT false,
    is_flagged BOOLEAN DEFAULT false,
    has_attachments BOOLEAN DEFAULT false,
    is_dirty_locally BOOLEAN DEFAULT false,
    in_reply_to VARCHAR(255) DEFAULT '',
    thread_id VARCHAR(255) DEFAULT '',
    snippet TEXT,
    draft_reply TEXT DEFAULT '',
    draft_remote_uid INT DEFAULT 0,
    body_path TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    -- For B2B / shared / snooze 
    status VARCHAR(20) DEFAULT 'new',
    first_response_at TIMESTAMP WITH TIME ZONE,
    resolved_at TIMESTAMP WITH TIME ZONE,
    spf_pass BOOLEAN,
    dkim_pass BOOLEAN,
    is_pinned BOOLEAN DEFAULT false,
    snooze_until TIMESTAMPTZ,
    is_muted BOOLEAN DEFAULT false,
    assigned_to UUID REFERENCES users(id),
    cc_address TEXT DEFAULT '',
    PRIMARY KEY (id, account_id),
    CONSTRAINT emails_msg_id_account_id_folder_id_key UNIQUE (msg_id, account_id, folder_id)
) PARTITION BY HASH (account_id);

-- 5. Create 64 partitions
DO $$
DECLARE
    i INT;
BEGIN
    FOR i IN 0..63 LOOP
        EXECUTE format('CREATE TABLE IF NOT EXISTS emails_p%s PARTITION OF emails FOR VALUES WITH (MODULUS 64, REMAINDER %s)', LPAD(i::text, 2, '0'), i);
    END LOOP;
END $$;

-- 6. Insert data from legacy
INSERT INTO emails 
SELECT 
    id, account_id, folder_id, msg_id, uid, subject, sender_name, sender_address, recipient_address, date_sent, is_read, is_flagged, has_attachments, is_dirty_locally, in_reply_to, thread_id, snippet, draft_reply, draft_remote_uid, body_path, created_at, status, first_response_at, resolved_at, spf_pass, dkim_pass, is_pinned, snooze_until, is_muted, assigned_to, cc_address
FROM emails_legacy;

-- 7. Recreate Indexes
CREATE INDEX IF NOT EXISTS idx_emails_account_date ON emails(account_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_folder ON emails(folder_id, date_sent DESC);
CREATE INDEX IF NOT EXISTS idx_emails_sender_address ON emails(sender_address);

-- 8. Recreate Foreign Keys referencing emails
ALTER TABLE email_tags ADD CONSTRAINT email_tags_email_id_fkey FOREIGN KEY (email_id, account_id) REFERENCES emails (id, account_id) ON DELETE CASCADE;
ALTER TABLE email_labels ADD CONSTRAINT email_labels_email_id_fkey FOREIGN KEY (email_id, account_id) REFERENCES emails (id, account_id) ON DELETE CASCADE;
ALTER TABLE email_comments ADD CONSTRAINT email_comments_email_id_fkey FOREIGN KEY (email_id, account_id) REFERENCES emails (id, account_id) ON DELETE CASCADE;
ALTER TABLE attachments ADD CONSTRAINT attachments_email_id_fkey FOREIGN KEY (email_id, account_id) REFERENCES emails (id, account_id) ON DELETE CASCADE;
ALTER TABLE imap_move_queue ADD CONSTRAINT imap_move_queue_email_id_fkey FOREIGN KEY (email_id, account_id) REFERENCES emails (id, account_id) ON DELETE CASCADE;

-- 9. Drop legacy table
DROP TABLE emails_legacy CASCADE;

COMMIT;

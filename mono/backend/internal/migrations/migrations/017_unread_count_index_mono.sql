BEGIN TRANSACTION;

-- SQLite partial index for counting unread and unmuted emails
CREATE INDEX IF NOT EXISTS idx_emails_unread_count ON emails(folder_id) WHERE is_read = 0 AND is_muted = 0;

COMMIT;

-- Analyze table to update statistics for the new index
ANALYZE emails;

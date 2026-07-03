-- SQLite doesn't support partitioned tables or CONCURRENTLY, so standard CREATE INDEX is used.
CREATE INDEX IF NOT EXISTS idx_emails_folder_pinned_date 
ON emails (folder_id, is_pinned DESC, date_sent DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_emails_account_pinned_date 
ON emails (account_id, is_pinned DESC, date_sent DESC, id DESC);

-- Update query planner statistics immediately
ANALYZE emails;

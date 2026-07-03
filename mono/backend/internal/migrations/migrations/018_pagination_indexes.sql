-- We omit CONCURRENTLY because PostgreSQL does not support creating an index CONCURRENTLY 
-- directly on a partitioned table. A standard CREATE INDEX will automatically cascade 
-- and build the index on all 64 partitions.
CREATE INDEX IF NOT EXISTS idx_emails_folder_pinned_date 
ON emails (folder_id, is_pinned DESC, date_sent DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_emails_account_pinned_date 
ON emails (account_id, is_pinned DESC, date_sent DESC, id DESC);

-- Update query planner statistics immediately
ANALYZE emails;

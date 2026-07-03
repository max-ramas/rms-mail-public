-- Migration 024: Unified inbox covering index
-- The unified inbox query filters by folder_id IN (subquery) which can't use
-- the existing folder_id-prefixed indexes. This index covers the ORDER BY
-- without a folder_id prefix, enabling index-only scan for unified queries.

CREATE INDEX IF NOT EXISTS idx_emails_pinned_date_id
    ON emails (is_pinned DESC, date_sent DESC, id DESC);

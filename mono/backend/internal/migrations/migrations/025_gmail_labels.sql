-- Migration 025: Gmail labels support
-- Phase 1: Gmail detection flag
-- PostgreSQL-safe: use DO block since ADD COLUMN IF NOT EXISTS is unsupported
DO $$
BEGIN
    ALTER TABLE accounts ADD COLUMN is_gmail INTEGER DEFAULT 0;
EXCEPTION WHEN duplicate_column THEN
    -- already exists, skip
END $$;

-- Phase 2: Email labels junction table (for Gmail multi-label tracking)
-- Note: FK omitted intentionally — emails is partitioned in PostgreSQL,
-- and partitioned tables don't support foreign key references.
-- Label cleanup handled at application level via DeleteEmail.
CREATE TABLE IF NOT EXISTS email_labels_junction (
    email_id  TEXT NOT NULL,
    account_id TEXT NOT NULL,
    label     TEXT NOT NULL,
    system    INTEGER DEFAULT 0,
    PRIMARY KEY (email_id, account_id, label)
);
CREATE INDEX IF NOT EXISTS idx_email_labels_junction_label ON email_labels_junction (account_id, label);

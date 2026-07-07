-- Migration 025: Gmail labels support (SQLite/Mono version)
-- Phase 1: Gmail detection flag
-- SQLite doesn't support IF NOT EXISTS on ALTER TABLE ADD COLUMN.
-- The addColumnIfMissing function in InitSchema will add the column if missing.
ALTER TABLE accounts ADD COLUMN is_gmail INTEGER DEFAULT 0;

-- Phase 2: Email labels junction table (for Gmail multi-label tracking)
CREATE TABLE IF NOT EXISTS email_labels_junction (
    email_id  TEXT NOT NULL,
    account_id TEXT NOT NULL,
    label     TEXT NOT NULL,
    system    INTEGER DEFAULT 0,
    PRIMARY KEY (email_id, account_id, label)
);
CREATE INDEX IF NOT EXISTS idx_email_labels_junction_label ON email_labels_junction (account_id, label);

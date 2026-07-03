-- Migration 023 (Mono): IMAP Answered flag
-- Plain ADD COLUMN (no IF NOT EXISTS on LibSQL)
ALTER TABLE emails ADD COLUMN is_answered INTEGER DEFAULT 0;

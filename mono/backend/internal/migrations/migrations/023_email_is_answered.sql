-- Migration 023: IMAP \Answered flag (replied state)
ALTER TABLE emails ADD COLUMN IF NOT EXISTS is_answered BOOLEAN DEFAULT false;

-- Migration 026: IMAP SPECIAL-USE role per folder (\\Trash, \\Sent, …)
ALTER TABLE folders ADD COLUMN IF NOT EXISTS special_use TEXT DEFAULT '';

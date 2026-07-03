-- Migration 022: Per-folder IMAP UIDVALIDITY (UID namespace is per-mailbox, not per-account)
ALTER TABLE folders ADD COLUMN IF NOT EXISTS uid_validity BIGINT DEFAULT 0;

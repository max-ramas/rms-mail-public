-- Migration 007: Drop old msg_id UNIQUE constraint (conflicts with cross-account emails).
-- schema.sql created msg_id UNIQUE which prevents the same Message-ID from existing
-- in multiple accounts. Migration 005 added the correct (msg_id, account_id) unique index,
-- but the old single-column constraint was never dropped.
--
-- Impact: emails CC'd to multiple accounts are only saved for the FIRST syncing account.
-- Fix: drop the old constraint, keep only (msg_id, account_id).

ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_msg_id_key;

-- Migration 014: Allow the same Message-ID to exist in multiple folders
-- The previous constraint UNIQUE(msg_id, account_id) forced an email to exist in only one folder.
-- This caused the folder_id to be overwritten when syncing a second folder (e.g. INBOX then Promotions).
-- We now allow (msg_id, account_id, folder_id) to be UNIQUE, so an email can exist in both INBOX and Promotions.

ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_msg_id_account_id_key;

-- Since the table is partitioned, we must include the partition key (account_id) in the unique constraint.
ALTER TABLE emails ADD CONSTRAINT emails_msg_id_account_id_folder_id_key UNIQUE (msg_id, account_id, folder_id);

-- Also do the same for SQLite mono schema

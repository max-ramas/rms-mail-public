-- ===============================================================
-- Adds HASH partitioning on account_id for better query performance
-- and scalability with multi-account setups.
--
-- NOTE: This is a standalone migration script.
-- Run it manually AFTER the main schema.sql has been applied:
--   psql -d yourdb -f migrations/005_partition_emails.sql
-- ===============================================================

BEGIN;

-- ===============================================================
-- 1. Drop foreign keys from child tables referencing emails(id)
-- ===============================================================
-- We must remove these before we can drop the old `emails` table.

ALTER TABLE IF EXISTS email_tags
  DROP CONSTRAINT IF EXISTS email_tags_email_id_fkey;

ALTER TABLE IF EXISTS email_labels
  DROP CONSTRAINT IF EXISTS email_labels_email_id_fkey;

ALTER TABLE IF EXISTS email_comments
  DROP CONSTRAINT IF EXISTS email_comments_email_id_fkey;

ALTER TABLE IF EXISTS attachments
  DROP CONSTRAINT IF EXISTS attachments_email_id_fkey;

ALTER TABLE IF EXISTS imap_move_queue
  DROP CONSTRAINT IF EXISTS imap_move_queue_email_id_fkey;

-- ===============================================================
-- 1b. Drop old primary key on emails (will be replaced)
-- ===============================================================
ALTER TABLE emails DROP CONSTRAINT IF EXISTS emails_pkey CASCADE;

-- ===============================================================
-- 1c. Add account_id to child tables that need composite FK
-- ===============================================================
ALTER TABLE email_tags ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE email_comments ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE attachments ADD COLUMN IF NOT EXISTS account_id UUID REFERENCES accounts(id) ON DELETE CASCADE;

-- Update the new account_id columns from the emails table
UPDATE email_tags et SET account_id = e.account_id FROM emails e WHERE et.email_id = e.id AND et.account_id IS NULL;
UPDATE email_comments ec SET account_id = e.account_id FROM emails e WHERE ec.email_id = e.id AND ec.account_id IS NULL;
UPDATE attachments a SET account_id = e.account_id FROM emails e WHERE a.email_id = e.id AND a.account_id IS NULL;

-- ===============================================================
-- 2. Create the partitioned table
-- ===============================================================
-- `LIKE ... INCLUDING DEFAULTS INCLUDING GENERATED` copies column
-- definitions, NOT NULL, defaults, generated-column info — but NOT
-- primary key, foreign keys, unique constraints, or indexes.

CREATE TABLE IF NOT EXISTS emails_partitioned (
  LIKE emails
    INCLUDING DEFAULTS
    INCLUDING GENERATED
) PARTITION BY HASH (account_id);

-- ===============================================================
-- 3. Create 16 partitions for the hash space
-- ===============================================================

CREATE TABLE IF NOT EXISTS emails_p0 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE IF NOT EXISTS emails_p1 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE IF NOT EXISTS emails_p2 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE IF NOT EXISTS emails_p3 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE IF NOT EXISTS emails_p4 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE IF NOT EXISTS emails_p5 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE IF NOT EXISTS emails_p6 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE IF NOT EXISTS emails_p7 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE IF NOT EXISTS emails_p8 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE IF NOT EXISTS emails_p9 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE IF NOT EXISTS emails_p10 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE IF NOT EXISTS emails_p11 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE IF NOT EXISTS emails_p12 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE IF NOT EXISTS emails_p13 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE IF NOT EXISTS emails_p14 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE IF NOT EXISTS emails_p15 PARTITION OF emails_partitioned FOR VALUES WITH (MODULUS 16, REMAINDER 15);

-- ===============================================================
-- 4. Add primary key (MUST include the partition column)
-- ===============================================================
-- LIST partitioning requires the partition key to be part of
-- every unique constraint / primary key on the table.

ALTER TABLE emails_partitioned
  ADD CONSTRAINT emails_pkey
  PRIMARY KEY (id, account_id);

-- ===============================================================
-- 5. Copy existing data into the partitioned table
-- ===============================================================

INSERT INTO emails_partitioned
  SELECT * FROM emails;

-- ===============================================================
-- 6. Drop the old table
-- ===============================================================
-- CASCADE drops all remaining dependent objects (FK constraints
-- that pointed TO emails as well as FROM emails to accounts,
-- folders, users).

DROP TABLE IF EXISTS emails CASCADE;

-- ===============================================================
-- 7. Rename partitioned table → emails
-- ===============================================================

ALTER TABLE IF EXISTS emails_partitioned RENAME TO emails;

-- ===============================================================
-- 8. Recreate indexes on the partitioned table
-- ===============================================================

CREATE INDEX IF NOT EXISTS idx_emails_account_date
  ON emails (account_id, date_sent DESC);

CREATE INDEX IF NOT EXISTS idx_emails_folder
  ON emails (folder_id, date_sent DESC);

CREATE INDEX IF NOT EXISTS idx_emails_sender_address
  ON emails (sender_address);

-- Recreate the unique constraint on msg_id.
-- For a partitioned table, the constraint must include the
-- partition key, so we use (msg_id, account_id).

CREATE UNIQUE INDEX IF NOT EXISTS emails_msg_id_account_key
  ON emails (msg_id, account_id);

-- ===============================================================
-- 9. Recreate foreign keys from child tables TO emails
-- ===============================================================
-- Note: emails PK is now (id, account_id), so child FKs must match.
-- Tables with account_id use composite FK, tables without use (id)
-- with an additional uniqueness guarantee from the PK.

ALTER TABLE email_tags
  ADD CONSTRAINT email_tags_email_id_fkey
  FOREIGN KEY (email_id, account_id)
  REFERENCES emails (id, account_id) ON DELETE CASCADE;

ALTER TABLE email_labels
  ADD CONSTRAINT email_labels_email_id_fkey
  FOREIGN KEY (email_id, account_id)
  REFERENCES emails (id, account_id) ON DELETE CASCADE;

ALTER TABLE email_comments
  ADD CONSTRAINT email_comments_email_id_fkey
  FOREIGN KEY (email_id, account_id)
  REFERENCES emails (id, account_id) ON DELETE CASCADE;

ALTER TABLE attachments
  ADD CONSTRAINT attachments_email_id_fkey
  FOREIGN KEY (email_id, account_id)
  REFERENCES emails (id, account_id) ON DELETE CASCADE;

ALTER TABLE imap_move_queue
  ADD CONSTRAINT imap_move_queue_email_id_fkey
  FOREIGN KEY (email_id, account_id)
  REFERENCES emails (id, account_id) ON DELETE CASCADE;

-- ===============================================================
-- 10. Recreate foreign keys FROM emails to other tables
-- ===============================================================
-- These were dropped by DROP TABLE ... CASCADE above.

ALTER TABLE emails
  ADD CONSTRAINT emails_account_id_fkey
  FOREIGN KEY (account_id)
  REFERENCES accounts (id) ON DELETE CASCADE;

ALTER TABLE emails
  ADD CONSTRAINT emails_folder_id_fkey
  FOREIGN KEY (folder_id)
  REFERENCES folders (id) ON DELETE SET NULL;

ALTER TABLE emails
  ADD CONSTRAINT emails_assigned_to_fkey
  FOREIGN KEY (assigned_to)
  REFERENCES users (id);

COMMIT;

-- ===============================================================
-- Verification queries (run after migration):
-- ===============================================================
-- 1. SELECT count(*) FROM emails;
--    Should match the count before migration.
--
-- 2. SELECT relname, relkind
--    FROM pg_class WHERE relname LIKE 'emails%'
--    ORDER BY relname;
--    Should show: emails (p), emails_default (r).
--
-- 3. SELECT conname, confdeltype
--    FROM pg_constraint WHERE conrelid = 'emails'::regclass
--    ORDER BY conname;
--    Should show: emails_account_id_fkey, emails_assigned_to_fkey,
--                 emails_folder_id_fkey, emails_pkey.

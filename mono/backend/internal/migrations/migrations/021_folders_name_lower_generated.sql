-- Migration 021: Convert name_lower to a GENERATED column for auto-updates.
-- This ensures name_lower stays in sync with name without manual UPDATEs.
--
-- For zero-downtime on large tables, use the rename approach:
--   1. ALTER TABLE folders ADD COLUMN name_lower_new TEXT GENERATED ALWAYS AS (LOWER(name)) STORED;
--   2. CREATE INDEX idx_folders_name_lower_new ON folders (account_id, name_lower_new);
--   3. Deploy app code that reads from name_lower_new.
--   4. ALTER TABLE folders DROP COLUMN name_lower;
--   5. ALTER TABLE folders RENAME COLUMN name_lower_new TO name_lower;
--   6. ALTER INDEX idx_folders_name_lower_new RENAME TO idx_folders_name_lower;
--
-- For small tables (< 100K rows), the simple DROP + ADD is sufficient:
DO $$
BEGIN
    -- Only run if the column exists and is NOT already generated
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'folders'
          AND column_name = 'name_lower'
          AND is_generated = 'NEVER'
    ) THEN
        DROP INDEX IF EXISTS idx_folders_name_lower;
        ALTER TABLE folders DROP COLUMN name_lower;
        ALTER TABLE folders ADD COLUMN name_lower TEXT GENERATED ALWAYS AS (LOWER(name)) STORED;
        CREATE INDEX idx_folders_name_lower ON folders (account_id, name_lower);
    END IF;
END $$;

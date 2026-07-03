-- Migration 020: Add name_lower column to avoid LOWER() killing index usage
ALTER TABLE folders ADD COLUMN IF NOT EXISTS name_lower TEXT;
UPDATE folders SET name_lower = LOWER(name) WHERE name_lower IS NULL;
DROP INDEX IF EXISTS idx_folders_name_lower;
CREATE INDEX idx_folders_name_lower ON folders (account_id, name_lower);

-- +migrate Up
ALTER TABLE accounts ADD COLUMN smart_categories BOOLEAN DEFAULT true;

-- +migrate Down
ALTER TABLE accounts DROP COLUMN smart_categories;

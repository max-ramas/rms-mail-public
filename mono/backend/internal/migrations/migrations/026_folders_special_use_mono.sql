-- Migration 026 (Mono/SQLite): IMAP SPECIAL-USE role per folder
ALTER TABLE folders ADD COLUMN special_use TEXT DEFAULT '';

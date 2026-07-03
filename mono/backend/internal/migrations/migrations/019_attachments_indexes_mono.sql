-- Migration 019 (Mono/SQLite): Attachments performance indexes
-- SQLite variant without CONCURRENTLY (PostgreSQL-only syntax)

CREATE INDEX IF NOT EXISTS idx_attachments_hash ON attachments(hash);
CREATE INDEX IF NOT EXISTS idx_attachments_email_id ON attachments(email_id, created_at);

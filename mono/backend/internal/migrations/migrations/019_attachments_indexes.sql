-- Migration 019: Attachments performance indexes for CAS dedup and email-attachment lookups
-- NOTE: CONCURRENTLY is PostgreSQL-specific. For SQLite/Mono, see 019_attachments_indexes_mono.sql

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attachments_hash ON attachments(hash);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_attachments_email_id ON attachments(email_id, created_at);

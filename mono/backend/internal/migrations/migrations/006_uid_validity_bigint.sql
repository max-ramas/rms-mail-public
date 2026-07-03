-- Migration 006: Fix uid_validity int4 overflow
-- IMAP UIDValidity is a 32-bit unsigned integer (max 4,294,967,295)
-- PostgreSQL INT (int4) is signed (max 2,147,483,647) — overflow causes sync failures.
ALTER TABLE accounts ALTER COLUMN uid_validity TYPE BIGINT;

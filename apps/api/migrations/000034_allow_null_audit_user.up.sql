-- Audit events without a signed-in user (failed logins, OAuth client
-- registration, pipeline writes) were written with user_id = '', which
-- Postgres rejects for a UUID column, so they were silently dropped. Allow
-- NULL instead; resource_id is already nullable.
ALTER TABLE audit_logs ALTER COLUMN user_id DROP NOT NULL;

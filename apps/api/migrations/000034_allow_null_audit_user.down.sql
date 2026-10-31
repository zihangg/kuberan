-- Keep user-less audit rows by attributing them to the nil UUID rather than
-- deleting them, so NOT NULL can be restored.
UPDATE audit_logs SET user_id = '00000000-0000-0000-0000-000000000000' WHERE user_id IS NULL;
ALTER TABLE audit_logs ALTER COLUMN user_id SET NOT NULL;

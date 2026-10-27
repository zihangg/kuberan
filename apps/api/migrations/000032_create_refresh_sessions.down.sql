ALTER TABLE users ADD COLUMN IF NOT EXISTS refresh_token_hash VARCHAR(64) DEFAULT '';

DROP TABLE IF EXISTS refresh_sessions;

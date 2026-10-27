-- Per-device refresh sessions. Replaces the single users.refresh_token_hash
-- column, which only allowed one live refresh token per user: logging in on a
-- second device overwrote it and silently logged the first device out.
CREATE TABLE IF NOT EXISTS refresh_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash VARCHAR(64) NOT NULL,
    previous_token_hash VARCHAR(64) NOT NULL DEFAULT '',
    rotated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    user_agent VARCHAR(255) NOT NULL DEFAULT '',
    ip_address VARCHAR(45) NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_refresh_sessions_token_hash ON refresh_sessions (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_previous_token_hash ON refresh_sessions (previous_token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_user_id ON refresh_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_sessions_deleted_at ON refresh_sessions (deleted_at);

ALTER TABLE users DROP COLUMN IF EXISTS refresh_token_hash;

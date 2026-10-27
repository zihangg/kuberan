-- Plan 022: per-user, per-scope OAuth consent grants replace the global
-- trust-on-first-use table. Existing trust rows carry no user or scope, so
-- they are dropped; affected users see the consent screen once more.
CREATE TABLE IF NOT EXISTS oauth_client_grants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users(id),
    client_id VARCHAR(255) NOT NULL,
    client_name VARCHAR(255) NOT NULL DEFAULT '',
    approved_scopes TEXT NOT NULL DEFAULT '',
    reviewed_scopes TEXT NOT NULL DEFAULT '',
    auto_accept BOOLEAN NOT NULL DEFAULT FALSE,
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_client_grants_user_client
    ON oauth_client_grants (user_id, client_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_oauth_client_grants_deleted_at
    ON oauth_client_grants (deleted_at);

DROP TABLE IF EXISTS trusted_oauth_clients;

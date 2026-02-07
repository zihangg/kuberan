CREATE TABLE IF NOT EXISTS categories (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id BIGINT NOT NULL REFERENCES users(id),
    name VARCHAR(100) NOT NULL,
    type VARCHAR(20) NOT NULL,
    description VARCHAR(500) DEFAULT '',
    icon VARCHAR(50) DEFAULT '',
    color VARCHAR(20) DEFAULT '',
    parent_id BIGINT REFERENCES categories(id)
);

CREATE INDEX IF NOT EXISTS idx_categories_deleted_at ON categories (deleted_at);
CREATE INDEX IF NOT EXISTS idx_categories_user_id ON categories (user_id);

CREATE TABLE IF NOT EXISTS transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    user_id UUID NOT NULL REFERENCES users(id),
    account_id UUID NOT NULL REFERENCES accounts(id),
    category_id UUID REFERENCES categories(id),
    type VARCHAR(20) NOT NULL,
    amount BIGINT NOT NULL,
    description VARCHAR(500) DEFAULT '',
    date TIMESTAMPTZ NOT NULL,
    to_account_id UUID REFERENCES accounts(id)
);

CREATE INDEX IF NOT EXISTS idx_transactions_deleted_at ON transactions (deleted_at);
CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions (user_id);
CREATE INDEX IF NOT EXISTS idx_transactions_account_id ON transactions (account_id);

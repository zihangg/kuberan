CREATE TABLE IF NOT EXISTS investments (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    account_id BIGINT NOT NULL REFERENCES accounts(id),
    symbol VARCHAR(20) NOT NULL,
    asset_type VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 0,
    cost_basis BIGINT NOT NULL DEFAULT 0,
    current_price BIGINT DEFAULT 0,
    last_updated TIMESTAMPTZ,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    exchange VARCHAR(50) DEFAULT '',
    maturity_date TIMESTAMPTZ,
    yield_to_maturity DOUBLE PRECISION DEFAULT 0,
    coupon_rate DOUBLE PRECISION DEFAULT 0,
    network VARCHAR(50) DEFAULT '',
    wallet_address VARCHAR(255) DEFAULT '',
    property_type VARCHAR(50) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_investments_deleted_at ON investments (deleted_at);
CREATE INDEX IF NOT EXISTS idx_investments_account_id ON investments (account_id);

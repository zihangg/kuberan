CREATE TABLE IF NOT EXISTS investment_transactions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    investment_id UUID NOT NULL REFERENCES investments(id),
    type VARCHAR(20) NOT NULL,
    date TIMESTAMPTZ NOT NULL,
    quantity DOUBLE PRECISION NOT NULL DEFAULT 0,
    price_per_unit BIGINT NOT NULL DEFAULT 0,
    total_amount BIGINT NOT NULL DEFAULT 0,
    fee BIGINT DEFAULT 0,
    notes TEXT DEFAULT '',
    split_ratio DOUBLE PRECISION DEFAULT 0,
    dividend_type VARCHAR(20) DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_investment_transactions_deleted_at ON investment_transactions (deleted_at);
CREATE INDEX IF NOT EXISTS idx_investment_transactions_investment_id ON investment_transactions (investment_id);

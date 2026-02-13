CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id UUID NOT NULL REFERENCES users(id),
    recorded_at TIMESTAMPTZ NOT NULL,
    total_net_worth BIGINT NOT NULL DEFAULT 0,
    cash_balance BIGINT NOT NULL DEFAULT 0,
    investment_value BIGINT NOT NULL DEFAULT 0,
    debt_balance BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT uq_portfolio_snapshots_user_recorded UNIQUE (user_id, recorded_at)
);
CREATE INDEX idx_portfolio_snapshots_user_id ON portfolio_snapshots(user_id);
CREATE INDEX idx_portfolio_snapshots_recorded_at ON portfolio_snapshots(recorded_at);

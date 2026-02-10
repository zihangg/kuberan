CREATE TABLE IF NOT EXISTS securities (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    asset_type VARCHAR(20) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    exchange VARCHAR(50) DEFAULT '',
    maturity_date TIMESTAMPTZ,
    yield_to_maturity DOUBLE PRECISION DEFAULT 0,
    coupon_rate DOUBLE PRECISION DEFAULT 0,
    network VARCHAR(50) DEFAULT '',
    property_type VARCHAR(50) DEFAULT '',

    CONSTRAINT uq_securities_symbol_exchange UNIQUE (symbol, exchange)
);
CREATE INDEX idx_securities_deleted_at ON securities(deleted_at);
CREATE INDEX idx_securities_symbol ON securities(symbol);
CREATE INDEX idx_securities_asset_type ON securities(asset_type);

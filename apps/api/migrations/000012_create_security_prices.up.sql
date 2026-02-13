CREATE TABLE IF NOT EXISTS security_prices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    security_id UUID NOT NULL REFERENCES securities(id),
    price BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT uq_security_prices_security_recorded UNIQUE (security_id, recorded_at)
);
CREATE INDEX idx_security_prices_security_id ON security_prices(security_id);
CREATE INDEX idx_security_prices_recorded_at ON security_prices(recorded_at);

ALTER TABLE investments ADD COLUMN realized_gain_loss BIGINT NOT NULL DEFAULT 0;
ALTER TABLE investment_transactions ADD COLUMN realized_gain_loss BIGINT NOT NULL DEFAULT 0;

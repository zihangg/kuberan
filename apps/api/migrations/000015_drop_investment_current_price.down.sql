ALTER TABLE investments ADD COLUMN current_price BIGINT DEFAULT 0;
ALTER TABLE investments ADD COLUMN last_updated TIMESTAMPTZ;

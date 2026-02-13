ALTER TABLE investments ADD COLUMN security_id UUID REFERENCES securities(id);
CREATE INDEX idx_investments_security_id ON investments(security_id);

ALTER TABLE investments DROP COLUMN symbol;
ALTER TABLE investments DROP COLUMN name;
ALTER TABLE investments DROP COLUMN asset_type;
ALTER TABLE investments DROP COLUMN currency;
ALTER TABLE investments DROP COLUMN exchange;
ALTER TABLE investments DROP COLUMN maturity_date;
ALTER TABLE investments DROP COLUMN yield_to_maturity;
ALTER TABLE investments DROP COLUMN coupon_rate;
ALTER TABLE investments DROP COLUMN network;
ALTER TABLE investments DROP COLUMN property_type;

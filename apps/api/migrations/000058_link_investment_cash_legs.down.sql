DROP INDEX IF EXISTS idx_transactions_investment_transaction_id;
ALTER TABLE transactions DROP COLUMN IF EXISTS investment_transaction_id;

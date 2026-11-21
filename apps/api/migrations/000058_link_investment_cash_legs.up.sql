-- The cash row a buy, sell or dividend writes to its funding/deposit account
-- points at that investment transaction, so deleting or editing the trade can
-- reverse exactly that cash movement. NULL on rows written before this column
-- existed (those are found by date and description) and on every other row.
-- No foreign key: investment_transactions rows are soft deleted, and the demo
-- reset hard-deletes rows table by table in its own order.
ALTER TABLE transactions ADD COLUMN investment_transaction_id UUID;

CREATE INDEX idx_transactions_investment_transaction_id
    ON transactions (investment_transaction_id)
    WHERE investment_transaction_id IS NOT NULL;

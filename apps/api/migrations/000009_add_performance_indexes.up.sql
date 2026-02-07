CREATE INDEX IF NOT EXISTS idx_transactions_user_date ON transactions (user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions (category_id);
CREATE INDEX IF NOT EXISTS idx_accounts_user_active ON accounts (user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_categories_user_type ON categories (user_id, type);
CREATE INDEX IF NOT EXISTS idx_budgets_user_active ON budgets (user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC);

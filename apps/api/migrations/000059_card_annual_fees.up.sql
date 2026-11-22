-- Credit card annual fees (plan 025): one optional row per card holding the fee
-- and its waiver condition, plus the two reminder kinds and the Telegram switch.
CREATE TABLE IF NOT EXISTS card_annual_fees (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    account_id UUID NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL CHECK (amount > 0),
    fee_month SMALLINT NOT NULL CHECK (fee_month BETWEEN 1 AND 12),
    fee_day SMALLINT CHECK (fee_day BETWEEN 1 AND 31),
    waiver_type VARCHAR(16) CHECK (waiver_type IN ('spend', 'swipes', 'spend_or_swipes')),
    waiver_spend BIGINT CHECK (waiver_spend > 0),
    waiver_swipes INT CHECK (waiver_swipes > 0),
    waiver_window VARCHAR(16) NOT NULL DEFAULT 'card_year' CHECK (waiver_window IN ('card_year', 'calendar_year'))
);

CREATE INDEX IF NOT EXISTS idx_card_annual_fees_user_id ON card_annual_fees (user_id);
CREATE INDEX IF NOT EXISTS idx_card_annual_fees_deleted_at ON card_annual_fees (deleted_at);

-- The reminders.kind CHECK was created inline in 000056 (Postgres named it reminders_kind_check).
ALTER TABLE reminders DROP CONSTRAINT IF EXISTS reminders_kind_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_kind_check
    CHECK (kind IN ('card_due', 'loan_due', 'fd_maturity', 'card_annual_fee', 'fee_waiver_pace'));

-- One switch covers both new kinds.
ALTER TABLE telegram_links ADD COLUMN IF NOT EXISTS remind_annual_fee BOOLEAN NOT NULL DEFAULT TRUE;

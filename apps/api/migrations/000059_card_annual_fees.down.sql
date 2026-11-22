ALTER TABLE telegram_links DROP COLUMN IF EXISTS remind_annual_fee;

DELETE FROM reminders WHERE kind IN ('card_annual_fee', 'fee_waiver_pace');
ALTER TABLE reminders DROP CONSTRAINT IF EXISTS reminders_kind_check;
ALTER TABLE reminders ADD CONSTRAINT reminders_kind_check
    CHECK (kind IN ('card_due', 'loan_due', 'fd_maturity'));

DROP TABLE IF EXISTS card_annual_fees;

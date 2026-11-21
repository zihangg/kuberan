# Plan 025 — Credit card annual fees and fee waivers

**Status: agreed, not started.** Decisions recorded at the end; task plan in [`implementation-plan.md`](implementation-plan.md).

## Goal

Let a user record a credit card's annual fee and, when the card has one, the condition that waives it (a minimum
spend, a minimum number of transactions, or either). Kuberan then:

1. reminds them on Telegram about 30 days before the fee is charged,
2. works out waiver progress from the card's own transactions (never typed in), and
3. nudges them about 90 days before the waiver deadline, but only when they are behind pace.

## Context (what exists today)

- Reminders (plan 023, 5.6): `models.Reminder`, one row per occurrence, keyed by `(kind, ref_id, due_on)`.
  `services/reminder_sync.go` plans and syncs them; `services/reminder_service.go` serves the bot (`Due`,
  `MarkSent`), the web app (`ListUpcoming`, `SetEnabled`) and rolls stale ones on (`rollStale`).
- **Constraint 1:** `planAccountReminder` returns one plan per account, and `syncAccountReminders` hard-deletes
  every unsent reminder for the account that doesn't match it. A second kind on the same card would delete the
  monthly `card_due` reminder and the other way round. The sync has to plan and reconcile **per kind**.
- **Constraint 2:** `Due()` skips `card_due` / `loan_due` reminders when nothing is owed. The fee reminder must not
  be skipped for that reason.
- **Constraint 3:** `isRecurringReminder` means monthly. The fee reminder recurs yearly.
- Per-kind Telegram switches live on `telegram_links` (`remind_card_due`, ...), updated through
  `PUT /telegram/reminder-settings`.
- Web: `components/accounts/reminder-control.tsx` on `credit-card-view.tsx`; `lib/reminders.ts` mirrors the lead
  times and labels. Bot: `apps/bot/reminders.py` `format_reminder_message`.

## Data model — migration `000058_card_annual_fees`

New 1:1 table (mirrors `fixed_deposit_details` rather than adding seven nullable columns to `accounts`):

```sql
CREATE TABLE card_annual_fees (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    created_at, updated_at, deleted_at ...,
    account_id UUID NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount BIGINT NOT NULL CHECK (amount > 0),           -- cents, card currency (a free card has no row)
    fee_month SMALLINT NOT NULL CHECK (fee_month BETWEEN 1 AND 12),
    fee_day SMALLINT CHECK (fee_day BETWEEN 1 AND 31),   -- NULL = "sometime in the month" (use the 1st)
    waiver_type VARCHAR(16) CHECK (waiver_type IN ('spend', 'swipes', 'spend_or_swipes')),  -- NULL = no waiver
    waiver_spend BIGINT CHECK (waiver_spend > 0),        -- cents, card currency
    waiver_swipes INT CHECK (waiver_swipes > 0),
    waiver_window VARCHAR(16) NOT NULL DEFAULT 'card_year' CHECK (waiver_window IN ('card_year', 'calendar_year'))
);
```

- `reminders.kind` CHECK gains `card_annual_fee` and `fee_waiver_pace` (both fit `varchar(16)`).
- `telegram_links.remind_annual_fee BOOLEAN NOT NULL DEFAULT TRUE` (one switch covers both new kinds; each card
  still has its own per-kind switch).
- Down migration drops the table and column and restores the old CHECK (deleting rows of the new kinds first).
- Removing a fee **hard-deletes** its row (`Unscoped`): `account_id` is `UNIQUE`, so a soft-deleted row would block
  adding a fee to the same card again.
- The model is registered in `internal/testutil/database.go` (`allModels`), since service tests use AutoMigrate.
- Validation (service): credit cards only (`INVALID_ACCOUNT_TYPE` otherwise); `waiver_spend` required for
  `spend` / `spend_or_swipes`, `waiver_swipes` for `swipes` / `spend_or_swipes`, neither allowed without a type.

## Dates and the waiver window (pure functions, `services/card_fee.go`)

- `NextFeeDate(month, day *int, from)`: the next date on or after `from` that falls on `month`/`day`, clamping
  to the month's last day (29 Feb → 28 Feb in non-leap years). With no day, the 1st; the UI and bot then say
  "in March" rather than "on 1 Mar".
- `WaiverWindow(fee, from) (start, end)`, both inclusive UTC dates; `from` is "now" for progress and the
  reminder plan's start date for the next occurrence (so a sent pace reminder plans the *following* window):
  - `card_year`: `[nextFee − 1 year + 1 day, nextFee]`. The deadline is the fee date.
  - `calendar_year`: `[1 Jan, 31 Dec]` of `from`'s year. The deadline is 31 Dec.
- Transactions are matched on `date >= start AND date < end + 1 day` (UTC).

## Waiver progress (`GET /api/v1/accounts/:id/fee-waiver`)

Computed in SQL over the card's own transactions in the window (card currency, so no FX):

- **Spend** = Σ `expense` amounts. Credits on the card (`income`: refunds or cashback) are ignored, since banks
  don't subtract cashback; a refunded purchase therefore still counts.
- **Swipes** = count of `expense` rows.
- Excluded automatically by type: `transfer` (card payments), `opening_balance`, `investment`, `reimbursement`,
  soft-deleted rows.
- Excluded by description: the fee itself (`annual fee`, `yuran tahunan`, case-insensitive, via
  `LOWER(description) LIKE`, which works on both Postgres and the SQLite tests), from spend **and** swipes.
- Shared expenses count in full (`amount`, not `personal_amount`: the bank sees the whole charge).

Response:

```json
{
  "currency": "MYR",
  "fee": {"amount": 25000, "next_fee_date": "2027-03-15", "day_known": true},
  "waiver": {
    "type": "spend", "window": "card_year",
    "window_start": "2026-03-16", "window_end": "2027-03-15",
    "spend": {"target": 1200000, "actual": 842000, "remaining": 358000},
    "swipes": null,
    "status": "behind",              // met | on_track | behind
    "per_month_needed": 119333,      // remaining / months left (ceil), null when met
    "days_left": 114
  },
  "spent_last_12_months": 842000     // always present: used by the keep-or-cancel message
}
```

- `status`: `met` when the condition holds (either target for `spend_or_swipes`); otherwise `on_track` when
  actual ≥ 90% of the straight-line pace (target × elapsed fraction of the window); otherwise `behind`. During
  the first 30 days of a window it never reports `behind`.
- `waiver` is `null` when the card has no waiver condition; 404 `ACCOUNT_NOT_FOUND` for another user's card;
  `annual_fee` not set → 200 with `fee: null, waiver: null`.

## API surface

- `PUT /accounts/:id/annual-fee` sets or replaces the fee (`{amount, month, day?, waiver_type?, waiver_spend?,
  waiver_swipes?, waiver_window?}`) and `DELETE /accounts/:id/annual-fee` removes it. Both re-sync the card's
  reminders in the same DB transaction. (A dedicated resource rather than a field on `PUT /accounts/:id`: that
  request uses pointer fields, which can't tell "omitted" from `null`, and the create dialog doesn't need it.)
- `GET /accounts/:id` and `GET /accounts` preload and return `annual_fee`.
- `PUT /telegram/reminder-settings` accepts `annual_fee` (bool); `GET /telegram/link` returns it.
- Swagger regenerated; route list in `CLAUDE.md` updated.

## Reminders

1. **Refactor to per-kind planning.** `planAccountReminder` becomes `planAccountReminders` returning
   `[]reminderPlan`. `syncAccountReminders` loops over kinds: for each planned kind it uses that kind's latest
   sent occurrence and its on/off state, keeps the matching unsent row, deletes other unsent rows of that kind and
   creates the missing one; unsent rows of kinds no longer planned are deleted. Existing `card_due` / `loan_due` /
   `fd_maturity` behaviour is unchanged (covered by the existing tests, which must pass untouched).
2. **`card_annual_fee`.** `due_on` = next fee date, sendable 30 days before at 01:00 UTC. Recurs yearly: after
   `MarkSent`, the next plan starts the day after, which is next year's date. Never skipped for a zero balance.
3. **`fee_waiver_pace`.** Only when a waiver condition exists. `due_on` = window end, sendable from 90 days before.
   `Due()` evaluates the progress at poll time and returns it only when the status is `behind`. A card that is
   on track is left unsent and its `remind_at` moves to the next day at 01:00 UTC, so it is checked once a day
   (not on every 5-minute poll) and withheld rows never fill `Due()`'s 100-row batch ahead of real reminders. It
   fires on the first day the card is behind within the last 90 days, at most once per window; `rollStale` moves
   it to the next window after the deadline.
4. `kindEnabledOnLink`: both new kinds → `RemindAnnualFee`. `isRecurringReminder` covers both (their next
   occurrence comes from the plan, not from "+1 month").
5. `DueReminder` gains `annual_fee` (cents) and `waiver` (the progress object above) for the bot to word messages.

## Bot (`apps/bot/reminders.py`)

- `card_annual_fee`, with waiver met: "Waiver met — check the fee isn't charged on your statement."
- With waiver not met: shortfall, "spend it before {date}, call the bank to ask for a waiver, or cancel."
- No waiver condition: the fee plus "You spent MYR x on this card in the last 12 months."
- `fee_waiver_pace`: shortfall, days left, per-month figure.
- "in March" wording when the fee day is unknown. Tests in `tests/test_reminders.py`.
- (The bot has no per-card detail command, only the `/accounts` list, so no progress line is added there.)

## Web

- `types/models.ts`: `CardAnnualFee`, `FeeWaiverProgress`; `ReminderKind` gains the two kinds.
- `hooks/use-accounts.ts`: `useFeeWaiver(accountId)` with its own key in the factory; invalidated by transaction
  mutations that touch the account and by account updates.
- `GET /reminders?ref_id=` now returns several kinds for one card, and `ReminderControl` picks the soonest
  (`pickReminder`), so it would show the fee reminder as the payment reminder. `pickReminder` takes a kind filter;
  `ReminderControl` passes `card_due` / `loan_due`, the fee control `card_annual_fee`.
- `lib/reminders.ts`: lead days (30 / 90) and labels; `lib/card-fee.ts`: pure helpers (fee date label, status
  copy, progress fraction) with Vitest tests.
- `components/accounts/annual-fee-control.tsx` on `credit-card-view.tsx`, under `ReminderControl`:
  - nothing set → ghost button "Add annual fee";
  - set → "Annual fee · MYR 250.00 · around 15 Mar" + reminder switch + "Edit";
  - dialog: amount (`CurrencyInput`), month, optional day, "Can it be waived?" section (type, target(s), window).
- `components/accounts/fee-waiver-progress.tsx`: progress bar(s), status chip (Met / On track / Behind),
  "spend about MYR x/month to qualify", info tooltip: "Based on the transactions recorded on this card. Banks
  often exclude some spending (e-wallet top-ups, insurance, government payments)."
- `components/settings/telegram-settings.tsx`: "Annual fees" switch (`remind_annual_fee`), added to its own
  switch list (it isn't one-to-one with a reminder kind).
- Uses palette tokens (`pos` / `neg` / `accent`) only; checked light/dark at 1440 and 390.

## Also touched

- **MCP**: `list_accounts` (the only account tool) adds one line under each card with a fee: the fee, next date
  and waiver progress (`MYR` code formatting). No new write tool.
- **Demo** (`demo_service.go`): the sample card gets an annual fee with a spend waiver; its row is listed in
  `demo_rows` so `DELETE /demo` removes it.
- **Export**: `accounts.csv` gains the fee and waiver columns.
- **Docs**: `docs/telegram-bot.md` (reminder kinds), `CLAUDE.md` (endpoints, reminder kinds).

## Tests

- Service: `NextFeeDate` (no day, day 31, 29 Feb, the fee date itself, after it); `WaiverWindow` for both
  windows; progress (credits ignored, fee row excluded, transfers / opening balance / deleted excluded, shared expense in
  full, either-target logic, status thresholds, first-30-days rule); validation errors.
- Reminder sync: **`card_due` and `card_annual_fee` coexist on one card** (the regression from constraint 1);
  removing the fee deletes only its rows; yearly recurrence after `MarkSent`; per-kind on/off carries over;
  zero balance still sends the fee reminder; pace reminder withheld while on track, sent once behind, once per
  window; Telegram switch respected.
- Handlers: `PUT` / `DELETE /accounts/:id/annual-fee` (set, replace, remove then re-add, non-card rejected,
  another user's card 404), fee-waiver endpoint 200 / 404.
- Web: `ReminderControl` keeps showing the payment reminder when a sooner fee reminder exists.
- Integration: create card → add fee + waiver → add transactions → progress → due reminders.
- Bot: message wording for each case. Web: Vitest for `lib/card-fee.ts`.
- Gate: `./scripts/check-go.sh apps/api`, bot `pytest`, web `pnpm lint`, `tsc`, `pnpm test`, `pnpm build`.

## Delivery

Task-by-task in [`implementation-plan.md`](implementation-plan.md): one branch, one commit per task, each green
on its own.

## Decisions (agreed)

1. **Credits on the card are ignored**: spend counts expenses only, because Kuberan can't tell refunds from
   cashback and banks don't subtract cashback. A refunded purchase still counts.
2. **The fee charge is recognised by description** (`annual fee`, `yuran tahunan`); marking it by hand is later.
3. **Pace**: "behind" is below 90% of straight-line pace, the nudge window is the last 90 days, and nothing is
   reported as behind in a window's first 30 days.
4. **One Telegram switch** ("Annual fees") covers both new kinds; each card keeps its per-kind switch.
5. **Out of scope (v2)**: category exclusions per card; detecting the fee from transactions and offering to
   set it up; asking after the fee date whether it was waived.

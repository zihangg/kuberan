# Plan 025 — Credit card annual fees and fee waivers

**Status: agreed, not started.** Decisions recorded at the end.

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
    amount BIGINT NOT NULL CHECK (amount >= 0),          -- cents, card currency
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
- Validation (service): credit cards only (`ACCOUNT_TYPE_MISMATCH` otherwise); `waiver_spend` required for
  `spend` / `spend_or_swipes`, `waiver_swipes` for `swipes` / `spend_or_swipes`, neither allowed without a type.

## Dates and the waiver window (pure functions, `services/card_fee.go`)

- `NextFeeDate(month, day *int, from)`: the next date on or after `from` that falls on `month`/`day`, clamping
  to the month's last day (29 Feb → 28 Feb in non-leap years). With no day, the 1st; the UI and bot then say
  "in March" rather than "on 1 Mar".
- `WaiverWindow(fee, now) (start, end)`:
  - `card_year`: `(nextFee − 1 year, nextFee]`. The deadline is the fee date.
  - `calendar_year`: `[1 Jan, 31 Dec]` of `now`'s year. The deadline is 31 Dec.

## Waiver progress (`GET /api/v1/accounts/:id/fee-waiver`)

Computed in SQL over the card's own transactions in the window (card currency, so no FX):

- **Spend** = Σ `expense` amounts. Credits on the card (`income`: refunds or cashback) are ignored, since banks
  don't subtract cashback; a refunded purchase therefore still counts.
- **Swipes** = count of `expense` rows.
- Excluded automatically by type: `transfer` (card payments), `opening_balance`, `investment`, soft-deleted rows.
- Excluded by description: the fee itself (`annual fee`, `yuran tahunan`, case-insensitive), so a charged fee
  doesn't count towards its own waiver.
- Shared expenses count in full (the bank sees the whole charge); receivables settled later don't reduce it.

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

- `POST /accounts/credit-card` and `PATCH /accounts/:id` accept an optional `annual_fee` object
  (`{amount, month, day?, waiver_type?, waiver_spend?, waiver_swipes?, waiver_window?}`); `"annual_fee": null` on
  PATCH removes it. Saving it calls `SyncAccount` (already done after every account update).
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
   on track is left unsent and checked again on the next poll, so it fires on the first day it falls behind within
   the last 90 days. At most once per window; `rollStale` moves it to the next window after the deadline.
4. `kindEnabledOnLink`: both new kinds → `RemindAnnualFee`. `isRecurringReminder` covers both (their next
   occurrence comes from the plan, not from "+1 month").
5. `DueReminder` gains `annual_fee` (cents) and `waiver` (the progress object above) for the bot to word messages.

## Bot (`apps/bot/reminders.py`)

- `card_annual_fee`, with waiver met: "Waiver met — check the fee isn't charged on your statement."
- With waiver not met: shortfall, "spend it before {date}, call the bank to ask for a waiver, or cancel."
- No waiver condition: the fee plus "You spent MYR x on this card in the last 12 months."
- `fee_waiver_pace`: shortfall, days left, per-month figure.
- "in March" wording when the fee day is unknown. Tests in `tests/test_reminders.py`.
- The card detail in `handlers/accounts.py` adds one waiver progress line (new `api_client.get_fee_waiver`).

## Web

- `types/models.ts`: `CardAnnualFee`, `FeeWaiverProgress`; `ReminderKind` gains the two kinds.
- `hooks/use-accounts.ts`: `useFeeWaiver(accountId)` with its own key in the factory; invalidated by transaction
  mutations that touch the account and by account updates.
- `lib/reminders.ts`: lead days (30 / 90) and labels; `lib/card-fee.ts`: pure helpers (fee date label, status
  copy, progress fraction) with Vitest tests.
- `components/accounts/annual-fee-control.tsx` on `credit-card-view.tsx`, under `ReminderControl`:
  - nothing set → ghost button "Add annual fee";
  - set → "Annual fee · MYR 250.00 · around 15 Mar" + reminder switch + "Edit";
  - dialog: amount (`CurrencyInput`), month, optional day, "Can it be waived?" section (type, target(s), window).
- `components/accounts/fee-waiver-progress.tsx`: progress bar(s), status chip (Met / On track / Behind),
  "spend about MYR x/month to qualify", info tooltip: "Based on the transactions recorded on this card. Banks
  often exclude some spending (e-wallet top-ups, insurance, government payments)."
- `components/settings/telegram-settings.tsx`: "Annual fees" switch.
- Uses palette tokens (`pos` / `neg` / `accent`) only; checked light/dark at 1440 and 390.

## Also touched

- **MCP**: the account read tool includes the fee and waiver progress (`MYR` code formatting). No new write tool.
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
- Handlers: create / patch / clear `annual_fee`, non-card rejected, fee-waiver endpoint 200 / 404.
- Integration: create card → add fee + waiver → add transactions → progress → due reminders.
- Bot: message wording for each case. Web: Vitest for `lib/card-fee.ts`.
- Gate: `./scripts/check-go.sh apps/api`, bot `pytest`, web `pnpm lint`, `tsc`, `pnpm test`, `pnpm build`.

## Delivery

One branch, one commit per phase, each green on its own:

1. Migration, model, validation, create/patch/get, progress endpoint + tests.
2. Reminder refactor (per kind) + the two new kinds + Telegram switch + tests.
3. Bot messages and card detail line.
4. Web: types, hooks, fee control, progress, settings switch.
5. MCP, demo data, export, docs.

## Decisions (agreed)

1. **Credits on the card are ignored**: spend counts expenses only, because Kuberan can't tell refunds from
   cashback and banks don't subtract cashback. A refunded purchase still counts.
2. **The fee charge is recognised by description** (`annual fee`, `yuran tahunan`); marking it by hand is later.
3. **Pace**: "behind" is below 90% of straight-line pace, the nudge window is the last 90 days, and nothing is
   reported as behind in a window's first 30 days.
4. **One Telegram switch** ("Annual fees") covers both new kinds; each card keeps its per-kind switch.
5. **Out of scope (v2)**: category exclusions per card; detecting the fee from transactions and offering to
   set it up; asking after the fee date whether it was waived.

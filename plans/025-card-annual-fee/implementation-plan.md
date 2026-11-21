# Card Annual Fees and Fee Waivers Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Record a credit card's annual fee and waiver condition, compute waiver progress from the card's transactions, and send Telegram reminders before the fee and when the card is behind pace.

**Architecture:** A 1:1 `card_annual_fees` table beside `accounts`; pure date/window/status functions plus one SQL aggregate in `services`; the existing reminder sync made per-kind, then given two new kinds; the bot words the new messages; the web card page gets a fee control and a progress bar.

**Tech Stack:** Go 1.26, Gin, GORM, golang-migrate, PostgreSQL 16 (SQLite in tests); Python bot (python-telegram-bot, pytest); Next.js 15, React 19, React Query v5, react-hook-form + zod, Vitest.

**Spec:** [`025-card-annual-fee.md`](025-card-annual-fee.md) (read it first; this plan does not repeat its rules).

## Global Constraints

- Money is int64 cents in the card's own currency; no FX anywhere in this feature.
- All dates are UTC days; reminders become sendable at 01:00 UTC (`reminderSendHourUTC`).
- Lead times: fee reminder 30 days, pace reminder window 90 days. Pace "behind" = actual < 90% of straight-line pace; never `behind` within 30 days of the window start.
- Credits on the card (`income`, `reimbursement`) are ignored. Fee rows are matched by `LOWER(description) LIKE '%annual fee%' OR LIKE '%yuran tahunan%'`.
- Reminder kinds: `card_annual_fee`, `fee_waiver_pace`. Telegram switch: `remind_annual_fee` (one switch for both).
- Schema changes only via `apps/api/migrations/000058_card_annual_fees.{up,down}.sql`; register the model in `internal/testutil/database.go`.
- Every query is scoped by `user_id`. Exported Go identifiers get godoc. No `//nolint`. Zap only for logging.
- TypeScript strict, no `any`; money shown with `formatMoney(cents, currency)`; MCP amounts tagged with the ISO code.
- Per-task gate: `./scripts/check-go.sh apps/api` (Go tasks), `cd apps/bot && pytest` (bot), `cd apps/web && pnpm lint && pnpm exec tsc --noEmit && pnpm test && pnpm build` (web).

## Review Focus

1. **Deactivated card with a fee** — no fee or pace reminders are planned and pending ones are deleted (owner: Task 5, `TestCardFeeReminders_DeactivatedCard`).
2. **Transactions on the window's first and last day, and on the fee date itself** — both ends count; the day after the fee date belongs to the next window (owner: Task 2, `TestCardSpend_WindowBoundaries`).
3. **A non-MYR card** (e.g. USD) — progress and bot message use the card's currency, not the user's base (owner: Task 2 `TestCardFeeSummary_UsesCardCurrency`, Task 6 `test_annual_fee_message_uses_card_currency`).
4. **Waiver removed while a pace reminder is pending** — the pending `fee_waiver_pace` row is deleted; the `card_annual_fee` row stays (owner: Task 5, `TestCardFeeReminders_RemoveWaiverKeepsFeeReminder`).
5. **Fee date edited after this year's fee reminder was sent** — the next reminder follows the new date and the sent one is not resent for the old date (owner: Task 5, `TestCardFeeReminders_EditDateAfterSent`).

---

### Task 1: Model, migration and fee-date functions

**Files:**
- Create: `apps/api/migrations/000058_card_annual_fees.up.sql`, `.down.sql`
- Create: `apps/api/internal/models/card_annual_fee.go`
- Modify: `apps/api/internal/models/account.go` (relation), `models/reminder.go` (kinds), `models/telegram_link.go` (switch)
- Modify: `apps/api/internal/testutil/database.go` (`allModels`)
- Create: `apps/api/internal/services/card_fee.go`, `card_fee_test.go`

**Interfaces:**
- Produces (models): `type WaiverType string` (`WaiverTypeSpend`, `WaiverTypeSwipes`, `WaiverTypeSpendOrSwipes`); `type WaiverWindow string` (`WaiverWindowCardYear`, `WaiverWindowCalendarYear`); `type CardAnnualFee struct { Base; AccountID, UserID string; Amount int64; FeeMonth int; FeeDay *int; WaiverType *WaiverType; WaiverSpend *int64; WaiverSwipes *int; WaiverWindow WaiverWindow }` with json tags `account_id, user_id, amount, fee_month, fee_day, waiver_type, waiver_spend, waiver_swipes, waiver_window`; `Account.AnnualFee *CardAnnualFee` (`gorm:"foreignKey:AccountID" json:"annual_fee,omitempty"`); `ReminderKindCardAnnualFee`, `ReminderKindFeeWaiverPace` (and `Valid()`); `TelegramLink.RemindAnnualFee bool` (`json:"remind_annual_fee"`).
- Produces (services): `func NextFeeDate(month int, day *int, from time.Time) time.Time`; `func FeeWaiverWindow(fee *models.CardAnnualFee, from time.Time) (start, end time.Time)`.

- [ ] **Step 1: Write the failing tests** in `card_fee_test.go`:
  - `TestNextFeeDate` table: (3, 15, from 2026-11-21) → 2027-03-15; (3, nil, from 2026-11-21) → 2027-03-01; (11, 21, from 2026-11-21) → 2026-11-21 (the date itself); (2, 29, from 2026-11-21) → 2027-02-28; (2, 29, from 2027-03-01) → 2028-02-29; (4, 31, from 2026-01-01) → 2026-04-30.
  - `TestFeeWaiverWindow` table: card_year, fee 15 Mar, from 2026-11-21 → [2026-03-16, 2027-03-15]; card_year from 2027-03-16 → [2027-03-16, 2028-03-15]; calendar_year from 2026-11-21 → [2026-01-01, 2026-12-31].
- [ ] **Step 2: Run** `cd apps/api && go test ./internal/services -run 'TestNextFeeDate|TestFeeWaiverWindow' -v` — Expected: FAIL (undefined).
- [ ] **Step 3: Write the migration** per the spec's Data model section (table, the two `reminders.kind` CHECK values, `telegram_links.remind_annual_fee`). The down migration deletes reminders of the new kinds, then restores the old CHECK, drops the column and the table.
- [ ] **Step 4: Add the models, register `&models.CardAnnualFee{}` in `allModels`, implement both functions** (clamp the day like `NextDueDate` does).
- [ ] **Step 5: Run** the tests from Step 2 — Expected: PASS. Then `./scripts/check-go.sh apps/api` — Expected: all stages pass.
- [ ] **Step 6: Check the migration** against a dev Postgres if one is reachable: `cd apps/api && go run cmd/migrate/main.go up && go run cmd/migrate/main.go down 1 && go run cmd/migrate/main.go up` — Expected: no errors. If no database is reachable, say so in the task report.
- [ ] **Step 7: Commit** `feat(api): card annual fee model, migration and fee-date functions`.

### Task 2: Waiver progress and the card fee service

**Files:**
- Create: `apps/api/internal/services/card_fee_progress.go`, `card_fee_service.go`, `card_fee_progress_test.go`, `card_fee_service_test.go`
- Modify: `apps/api/internal/services/interfaces.go` (interface)

**Interfaces:**
- Consumes: Task 1.
- Produces:
  - `type FeeWaiverStatus string` — `FeeWaiverMet "met"`, `FeeWaiverOnTrack "on_track"`, `FeeWaiverBehind "behind"`.
  - `type FeeWaiverTarget struct { Target, Actual, Remaining int64 }` (json `target, actual, remaining`).
  - `type FeeWaiverProgress struct { Type models.WaiverType; Window models.WaiverWindow; WindowStart, WindowEnd string; Spend, Swipes *FeeWaiverTarget; Status FeeWaiverStatus; PerMonthNeeded *int64; DaysLeft int }` (json as in the spec's response).
  - `type CardFeeInfo struct { Amount int64; NextFeeDate string; DayKnown bool }`; `type CardFeeSummary struct { Currency string; Fee *CardFeeInfo; Waiver *FeeWaiverProgress; SpentLast12Months int64 }`.
  - `func cardSpend(tx *gorm.DB, userID, accountID string, start, end time.Time) (spend, swipes int64, err error)`.
  - `func waiverStatus(fee *models.CardAnnualFee, spend, swipes int64, start, end, now time.Time) FeeWaiverStatus`.
  - `func buildCardFeeSummary(tx *gorm.DB, acct *models.Account, now time.Time) (*CardFeeSummary, error)` — `acct.AnnualFee` preloaded; used by Tasks 3, 5 and 9.
  - `type CardAnnualFeeInput struct { Amount int64; Month int; Day *int; WaiverType *models.WaiverType; WaiverSpend *int64; WaiverSwipes *int; WaiverWindow models.WaiverWindow }`.
  - `type CardFeeServicer interface { Set(userID, accountID string, in CardAnnualFeeInput, now time.Time) (*models.CardAnnualFee, error); Delete(userID, accountID string, now time.Time) error; Summary(userID, accountID string, now time.Time) (*CardFeeSummary, error) }`; `func NewCardFeeService(db *gorm.DB) CardFeeServicer`.

- [ ] **Step 1: Write the failing tests:**
  - `TestCardSpend_CountsOnlyExpenses`: on one card, expenses 100_00 and 50_00, income 30_00, a transfer in 200_00, an opening balance, a deleted expense, a `reimbursement` → spend 150_00, swipes 2.
  - `TestCardSpend_ExcludesFeeRows`: expense "Annual Fee 2026" 250_00 and "YURAN TAHUNAN" 250_00 → spend 0, swipes 0.
  - `TestCardSpend_WindowBoundaries` (Review Focus 2): expenses on start, end and end+1 day → only the first two count.
  - `TestCardSpend_SharedExpenseCountsInFull`: expense 300_00 with `personal_amount` 100_00 → spend 300_00.
  - `TestWaiverStatus` table (window 2026-01-01..2026-04-10 = 100 days, spend target 12_000_00): met at 12_000_00; on 2026-02-19 (day 50) on_track with 5_400_00 and behind with 5_399_00; on 2026-01-20 (day 20) on_track with 0; `spend_or_swipes` (swipes target 12) met by 12 swipes and 0 spend. Pace in integer cents: `elapsed = days(now−start)+1`, `total = days(end−start)+1`, on_track iff `actual*total*100 >= target*elapsed*90`.
  - `TestCardFeeSummary_PerMonthNeeded`: remaining 3_580_00 with 114 days left → `ceil(358000 / ceil(114/30)) = 89500`; `nil` when met.
  - `TestCardFeeSummary_UsesCardCurrency` (Review Focus 3): USD card for a MYR user → `Currency == "USD"`, amounts unconverted.
  - `TestCardFeeService_Validation` table: non-card account → `ErrInvalidAccountType`; amount 0, month 13, day 32 → `ErrInvalidInput`; `spend` without `waiver_spend`, `swipes` without `waiver_swipes`, targets without a type → `ErrInvalidInput`; another user's card → `ErrAccountNotFound`.
  - `TestCardFeeService_SetReplaceDeleteReAdd`: set, set again (one row, new values), delete (row gone even `Unscoped`), set again succeeds.
- [ ] **Step 2: Run** `go test ./internal/services -run 'TestCardSpend|TestWaiverStatus|TestCardFee' -v` — Expected: FAIL.
- [ ] **Step 3: Implement.** `cardSpend` is one `SUM`/`COUNT` query with the spec's filters. `Set`/`Delete` run in one DB transaction and call `syncAccountReminders(tx, accountID, now)` before committing. `Delete` uses `Unscoped()`. "Months left" is `max(1, ceil(daysLeft/30))`. `SpentLast12Months` uses the window `[now − 1 year + 1 day, now]`.
- [ ] **Step 4: Run** the tests — Expected: PASS; then `./scripts/check-go.sh apps/api` — Expected: pass.
- [ ] **Step 5: Commit** `feat(api): card fee waiver progress and card fee service`.

### Task 3: HTTP endpoints and account preload

**Files:**
- Create: `apps/api/internal/handlers/card_fee_handler.go`, `card_fee_handler_test.go`
- Modify: `apps/api/internal/handlers/routes.go` (`registerAccountRoutes` and the handlers struct), the place that constructs `h.Account` (find with `grep -rn NewAccountHandler apps/api`)
- Modify: `apps/api/internal/services/account_service.go` (preload `AnnualFee` in get and list)
- Regenerate: `apps/api/internal/docs`

**Interfaces:**
- Consumes: `CardFeeServicer` (Task 2).
- Produces: `PUT /api/v1/accounts/:id/annual-fee` (body `{amount, month, day?, waiver_type?, waiver_spend?, waiver_swipes?, waiver_window?}` → 200 `{"annual_fee": CardAnnualFee}`), `DELETE /api/v1/accounts/:id/annual-fee` → 204, `GET /api/v1/accounts/:id/fee-waiver` → 200 `CardFeeSummary`; `annual_fee` on account responses.

- [ ] **Step 1: Write the failing handler tests** (mock service, BDD style like `reminder_handler_test.go`): PUT 200 with body echoed; PUT 400 for `month: 0` (binding `gte=1,lte=12`), `waiver_window: "weekly"` (binding `oneof`); service `ErrInvalidAccountType` → 400 with that code; DELETE 204; GET 200 shape matches the spec's JSON keys; GET 404 `ACCOUNT_NOT_FOUND`.
- [ ] **Step 2: Run** `go test ./internal/handlers -run CardFee -v` — Expected: FAIL.
- [ ] **Step 3: Implement** the handler (thin; `time.Now()` passed in), register routes, wire the service, add the preloads, run `cd apps/api && swag init -g cmd/api/main.go -d . --output internal/docs --parseDependency`.
- [ ] **Step 4: Run** the tests, then `./scripts/check-go.sh apps/api` — Expected: pass.
- [ ] **Step 5: Commit** `feat(api): annual fee and fee-waiver endpoints`.

### Task 4: Per-kind reminder sync (refactor, no new kinds)

**Files:**
- Modify: `apps/api/internal/services/reminder_sync.go`
- Test: `apps/api/internal/services/reminder_service_test.go` (unchanged; must pass as-is)

**Interfaces:**
- Produces: `func planAccountReminders(acct *models.Account, fd *models.FixedDepositDetails, now time.Time, sentAfter map[models.ReminderKind]time.Time) []reminderPlan`; `syncAccountReminders` keeps its signature.

- [ ] **Step 1: Run the existing reminder tests** `go test ./internal/services -run 'Reminder|NextDueDate|DueDay|DebtAccount' -v` — Expected: PASS (baseline).
- [ ] **Step 2: Refactor** per spec Reminders §1: the latest sent occurrence and the on/off state are read **per kind**; for each plan, delete that kind's other unsent rows and create the missing one; delete unsent rows of kinds not in the plan list. Behaviour for today's kinds is unchanged.
- [ ] **Step 3: Run** the Step 1 tests without editing them — Expected: PASS. Then `./scripts/check-go.sh apps/api` — Expected: pass.
- [ ] **Step 4: Commit** `refactor(api): plan and sync reminders per kind`.

### Task 5: Annual fee and pace reminders

**Files:**
- Modify: `apps/api/internal/services/reminder_sync.go`, `reminder_service.go`, `telegram_service.go`, `interfaces.go` (`ReminderSettings`)
- Modify: `apps/api/internal/handlers/telegram_handler.go` (+ its test)
- Create: `apps/api/internal/services/card_fee_reminder_test.go`
- Modify: `apps/api/tests/integration/` (one new workflow test file)

**Interfaces:**
- Consumes: Task 1 kinds and fields, Task 2 `buildCardFeeSummary`, Task 4 `planAccountReminders`.
- Produces: `planAccountReminders` gains `fee *models.CardAnnualFee` after `fd`; constants `annualFeeReminderLeadDays = 30`, `feeWaiverPaceLeadDays = 90`; `DueReminder` gains `AnnualFee *int64 \`json:"annual_fee,omitempty"\``, `FeeDayKnown *bool \`json:"fee_day_known,omitempty"\``, `FeeWaiver *FeeWaiverProgress \`json:"fee_waiver,omitempty"\``, `SpentLast12Months *int64 \`json:"spent_last_12_months,omitempty"\``; `ReminderSettings.AnnualFee *bool`; telegram settings request field `annual_fee`.

- [ ] **Step 1: Write the failing tests** in `card_fee_reminder_test.go`:
  - `TestCardFeeReminders_CoexistWithCardDue`: card with `due_day` 5 and a fee on 15 Mar → one unsent `card_due` and one `card_annual_fee` row; re-sync leaves both.
  - `TestCardFeeReminders_RemindAt`: fee 2027-03-15 → `remind_at` 2027-02-13 01:00 UTC; pace row `due_on` = window end, `remind_at` = end − 90 days 01:00.
  - `TestCardFeeReminders_YearlyAfterSent`: `MarkSent` on the 2027 fee row → a 2028-03-15 row exists.
  - `TestCardFeeReminders_ZeroBalanceStillDue`: balance 0 → `Due()` returns the fee reminder.
  - `TestCardFeeReminders_PaceWithheldWhenOnTrack`: calendar-year waiver, target 12_000_00, spend 9_000_00 in 2026. `Due(2026-10-15 02:00)` omits the pace row (on track: 9_000 ≥ 90% of 288/365 × 12_000) and its `remind_at` becomes 2026-10-16 01:00. `Due(2026-11-30 02:00)` returns it (behind: 334/365). After `MarkSent`, the next pace row has `due_on` 2027-12-31.
  - `TestCardFeeReminders_TelegramSwitch`: `remind_annual_fee = false` → neither kind returned; `card_due` still returned.
  - Review Focus: `TestCardFeeReminders_DeactivatedCard`, `TestCardFeeReminders_RemoveWaiverKeepsFeeReminder`, `TestCardFeeReminders_EditDateAfterSent` (behaviour as listed in Review Focus).
  - Integration (`TestAnnualFeeWorkflow`, real clock): link Telegram, create a card, PUT an annual fee dated today + 20 days with a card-year spend waiver of 12_000_00, POST one 100_00 expense → GET fee-waiver shows spend 100_00 and `behind`; `GET /internal/telegram/reminders/due` returns a `card_annual_fee` (inside 30 days) and a `fee_waiver_pace` (inside 90 days, behind) with `fee_waiver` set.
- [ ] **Step 2: Run** `go test ./internal/services -run CardFeeReminders -v` and `go test ./tests/integration/... -run AnnualFee -v` — Expected: FAIL.
- [ ] **Step 3: Implement** per spec Reminders §2–5: plan both kinds for active cards with a fee (pace only with a waiver type, windows from `FeeWaiverWindow(fee, from)`); preload `AnnualFee` in the sync and in `Due()`; keep the zero-balance skip for `card_due`/`loan_due` only; withhold-and-bump pace rows; map both kinds to `RemindAnnualFee`; add the telegram switch through service and handler.
- [ ] **Step 4: Run** the Step 2 commands and the Task 4 baseline — Expected: PASS. Then `./scripts/check-go.sh apps/api` — Expected: pass.
- [ ] **Step 5: Commit** `feat(api): annual fee and fee waiver pace reminders`.

### Task 6: Bot messages

**Files:**
- Modify: `apps/bot/reminders.py`
- Test: `apps/bot/tests/test_reminders.py`

**Interfaces:**
- Consumes: `DueReminder` JSON from Task 5 (`kind`, `annual_fee`, `fee_day_known`, `fee_waiver`, `spent_last_12_months`, `currency`, `due_on`).
- Produces: `format_reminder_message` handles both new kinds; helper `format_fee_date(due_on: str, day_known: bool, today: date | None = None) -> str` ("15 Mar" or "in March").

- [ ] **Step 1: Write the failing tests** (wording from the spec's Bot section): `test_annual_fee_message_waiver_met`, `test_annual_fee_message_waiver_short` (contains the shortfall and the date), `test_annual_fee_message_no_waiver` (contains `spent_last_12_months`), `test_annual_fee_message_day_unknown` ("in March"), `test_fee_waiver_pace_message` (shortfall, days left, per-month), `test_annual_fee_message_uses_card_currency` (Review Focus 3: `USD` amounts formatted as USD).
- [ ] **Step 2: Run** `cd apps/bot && pytest tests/test_reminders.py -v` — Expected: the new tests FAIL.
- [ ] **Step 3: Implement**, escaping names with `esc` and formatting money with `format_currency(cents, currency)`.
- [ ] **Step 4: Run** `cd apps/bot && pytest` — Expected: PASS.
- [ ] **Step 5: Commit** `feat(bot): annual fee and fee waiver reminder messages`.

### Task 7: Web data layer

**Files:**
- Modify: `apps/web/src/types/models.ts`, `types/api.ts`, `lib/reminders.ts`, `lib/reminders.test.ts`
- Create: `apps/web/src/hooks/use-card-fee.ts`, `lib/card-fee.ts`, `lib/card-fee.test.ts`
- Modify: the transaction mutation hooks in `apps/web/src/hooks/use-transactions.ts` (invalidate `cardFeeKeys.all`)

**Interfaces:**
- Consumes: Task 3 endpoints, Task 5 kinds.
- Produces: TS types `CardAnnualFee`, `CardFeeInfo`, `CardFeeSummary`, `FeeWaiverProgress`, `FeeWaiverTarget`, `FeeWaiverStatus` (snake_case fields, matching the Go json tags); `ReminderKind` gains the two kinds; `cardFeeKeys = { all, forAccount(id) }`; `useFeeWaiver(accountId)`, `useSetAnnualFee(accountId)`, `useDeleteAnnualFee(accountId)` (both mutations invalidate the account, reminder and card-fee keys); `pickReminder(reminders: Reminder[], kinds?: ReminderKind[]): Reminder | null`; `feeDateLabel(fee: CardFeeInfo): string`; `waiverStatusLabel(status: FeeWaiverStatus): string`; `waiverFraction(t: FeeWaiverTarget): number` (0..1, clamped).

- [ ] **Step 1: Write the failing Vitest tests:** `pickReminder` with a sooner `card_annual_fee` and kinds `["card_due"]` returns the `card_due` row; `feeDateLabel({next_fee_date: "2027-03-15", day_known: true})` → `"15 Mar"`, `day_known: false` → `"March"`; `waiverStatusLabel` → `"Met"`, `"On track"`, `"Behind"`; `waiverFraction({target: 100, actual: 150, remaining: 0})` → `1`, `({target: 100, actual: 25, remaining: 75})` → `0.25`.
- [ ] **Step 2: Run** `cd apps/web && pnpm test` — Expected: the new tests FAIL.
- [ ] **Step 3: Implement**; add the two kinds to `REMINDER_LEAD_DAYS` (30, 90) and `REMINDER_KIND_LABELS` ("Annual fee", "Fee waiver progress").
- [ ] **Step 4: Run** the web gate (Global Constraints) — Expected: pass.
- [ ] **Step 5: Commit** `feat(web): card fee types, hooks and helpers`.

### Task 8: Web UI

**Files:**
- Create: `apps/web/src/components/accounts/annual-fee-control.tsx`, `annual-fee-dialog.tsx`, `fee-waiver-progress.tsx`
- Modify: `components/accounts/credit-card-view.tsx`, `components/accounts/reminder-control.tsx` (pass kinds to `pickReminder`), `components/settings/telegram-settings.tsx`

**Interfaces:**
- Consumes: Task 7 hooks and helpers.
- Produces: `<AnnualFeeControl account={account} />`, `<FeeWaiverProgress summary={summary} />`.

- [ ] **Step 1: Build** the components per the spec's Web section; the dialog's zod schema mirrors the server rules (amount > 0, month 1–12, optional day 1–31, targets required by type); copy is exactly as in the spec.
- [ ] **Step 2: Run** the web gate — Expected: pass.
- [ ] **Step 3: Check the page** with the `run` skill against a seeded stack: a card with a fee and a behind-pace spend waiver at 1440 and 390 wide, light and dark; the payment reminder row still shows the payment reminder. Attach screenshots to the task report.
- [ ] **Step 4: Commit** `feat(web): annual fee control and waiver progress on the card page`.

### Task 9: MCP, demo, export and docs

**Files:**
- Modify: `apps/api/internal/mcp/tools_accounts.go` (+ a case in `e2e_reads_test.go`)
- Modify: `apps/api/internal/services/demo_service.go` (+ `demo_service_test.go`)
- Modify: `apps/api/internal/services/export_service.go` (+ its test)
- Modify: `docs/telegram-bot.md`, `CLAUDE.md`

**Interfaces:**
- Consumes: `buildCardFeeSummary` (Task 2).

- [ ] **Step 1: Write the failing tests:** `list_accounts` output for a card with a fee contains `Annual fee MYR 250.00` and `Waiver: MYR 8,420.00 of MYR 12,000.00 (behind)`; demo load creates one `card_annual_fees` row recorded in `demo_rows`, and `DELETE /demo` removes it; `accounts.csv` header gains `annual_fee, annual_fee_month, annual_fee_day, waiver_type, waiver_spend, waiver_swipes, waiver_window`.
- [ ] **Step 2: Run** `go test ./internal/mcp ./internal/services -run 'ListAccounts|Demo|Export' -v` — Expected: FAIL.
- [ ] **Step 3: Implement** (demo table name constant `demoTableCardFees = "card_annual_fees"`; export preloads `AnnualFee`); update the docs (reminder kinds, the three endpoints, the Telegram switch).
- [ ] **Step 4: Run** `./scripts/check-go.sh apps/api` — Expected: pass.
- [ ] **Step 5: Commit** `feat: annual fees in MCP, demo data, export and docs`.

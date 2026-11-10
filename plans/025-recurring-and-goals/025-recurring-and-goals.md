# Plan 025 — Recurring and Goals

**Status: draft.** Decisions Q1–Q6 at the end carry a recommended option; nothing is built yet.

## Context

The November 2026 recheck against Monarch found Kuberan ahead on self-hosting, multi-currency and investment
accounting, and behind on planning. The two biggest planning gaps are:

1. **Recurring bills, subscriptions and income.** Kuberan has no idea that Netflix comes every month. Reminders
   (plan 023, 5.6) cover only card statements, loan instalments and deposit maturities, which come from
   account fields (`due_day`, `monthly_payment`, `maturity_date`), not from spending.
2. **Goals.** There is no way to say "RM 30,000 for a house deposit by 2028" or "clear the car loan early" and
   see whether you're on track.

Budget rollover was considered and dropped: budgets stay the calendar-period caps of plan 016.

What we can build on:

- `transactions` already carries `source`, `notes` and `import_ref`. The CSV import reduces descriptions with
  `normalizeDescriptionKey` / `descriptionsMatch` (`services/csv_import_parse.go`). The rules engine
  (`services/rule_matcher.go`) already runs on every write path.
- `reminders` + `reminder_sync.go` + the bot's poller (`apps/bot/reminders.py`) deliver due-date nudges.
- `account_balance_snapshots` (one balance per account per UTC day, migration 53) gives account history, so
  goal history needs no new tracking.
- Loan accounts have `interest_rate`, `monthly_payment` and `due_day` (migration 55).
- `datedValuer` / `FXServicer.RatesByDay` convert per day; the receivables summary converts at the current rate.
- The oracle already calls a daily pipeline job (`POST /pipeline/fixed-deposits/process`).

## Principles

- **No bank sync, so we can't rely on it.** Monarch watches incoming bank data. Kuberan users type, forward to
  the bot or import CSVs. Recurring must therefore work as both a detector (find patterns in what was
  entered) and a schedule (optionally post the payment itself, Q2).
- **Derived, not duplicated.** Occurrences are computed from a series and its linked transactions; goal progress
  is computed from account balances. No stored "progress" or "occurrence" rows to drift.
- **Money rules unchanged.** Amounts are int64 cents in the account's currency; anything that adds across
  currencies converts to the user's base currency and reports `fx_missing`, never 1:1.
- **Read-only for apps in v1.** MCP gets read tools only; no new OAuth scopes (Q5).

### Non-goals

- Budget rollover or "flex" budgets.
- Linking individual transactions to goals (goals follow account balances, see G2).
- Cash-flow or retirement forecasting, Monte Carlo. A later plan can build on recurring income and bills.
- Receipt-based or email-based detection.

## Part R — Recurring

### R1. Model

New table `recurring_series` (migration numbers assigned at merge, after `000057`):

| Column | Notes |
|---|---|
| `id`, `user_id`, timestamps, `deleted_at` | `Base`, user-scoped |
| `name` | "Netflix", "Salary", "TNB electricity" |
| `type` | `expense` \| `income` \| `transfer` (transactions' types; no `opening_balance`) |
| `account_id` | source account; `to_account_id` for transfers |
| `category_id` | nullable; used when posting and as a matching hint |
| `kind` | `subscription` \| `bill` \| `income` \| `transfer`. Display grouping only; defaults from `type` |
| `amount` | expected amount, cents, account currency |
| `amount_mode` | `fixed` \| `variable`. Variable uses the median of the last 3 linked amounts |
| `frequency` | `weekly` \| `biweekly` \| `monthly` \| `quarterly` \| `yearly` |
| `anchor_date` | first or reference occurrence; monthly series keep its day of month (clamped, like `NextDueDate`) |
| `match_key` | normalised description key (`normalizeDescriptionKey`), editable |
| `status` | `active` \| `paused` \| `ended` \| `dismissed` (a rejected suggestion, so it isn't offered again) |
| `origin` | `manual` \| `detected` |
| `skipped_through` | nullable date: "skip this one" moves the next due date past it |
| `auto_post` | bool, default false (Q2) |
| `remind` | bool, default false; opt-in Telegram reminder |

`transactions.recurring_id` (nullable FK, indexed). Deleting a series sets it to NULL; it never deletes
transactions.

### R2. Matching and linking

- **On write.** Every path that creates an expense, income or transfer (web, bot, MCP, CSV import; IBKR rows
  excluded) runs `matchRecurring` after the rules engine: same user, same type, same account (and to-account
  for transfers), `descriptionsMatch(match_key, key(description))`, the amount within tolerance (fixed: ±10%,
  variable: ±50% of expected), and a date within the occurrence window (±3 days weekly/biweekly, ±7 monthly,
  ±15 quarterly/yearly) of an unpaid occurrence. The first match wins (oldest series). The link is a
  suggestion the user can undo. Cost: one indexed query per write for the user's active series.
- **By hand.** `PUT /transactions/:id` accepts `recurring_id` (or null). `POST /recurring/:id/link` takes
  `transaction_ids` for bulk linking from the transactions table.
- **Accepting a suggestion** links the transactions it was built from.

### R3. Occurrences (computed on read)

For a series, the next due date is `anchor_date` stepped by `frequency` past the latest of: the latest
linked transaction's matched occurrence, `skipped_through`, and the series' creation date. Each occurrence
is in one of these states:

- `paid`: a linked transaction falls in its window (shows the actual amount and any price change);
- `upcoming`: due in the future;
- `due`: inside the window, nothing linked yet;
- `missed`: the window has passed with nothing linked. It shows once, then the series moves on, so a
  cancelled subscription doesn't nag forever. Three misses in a row suggest ending the series.

`price_changed` is set when a fixed series' last linked amount differs from `amount` by more than 1%. The UI
offers "Update expected amount".

### R4. Detection (suggestions)

`GET /recurring/suggestions` is computed on read, like rule suggestions:

1. Take the last 13 months of expense/income transactions with no `recurring_id`, excluding opening balances,
   investment and fixed-deposit accounts.
2. Group by (account, type, normalised description key).
3. For groups with ≥3 rows (≥2 for yearly), take the gaps between dates. Assign the frequency whose nominal
   gap (7, 14, 30.4, 91.3, 365.25 days) is within tolerance of the **median** gap, with ≥75% of gaps within
   tolerance.
4. Amount: `fixed` if every amount is within 5% of the median, else `variable` if within 50%, else not
   recurring.
5. Skip groups that match a series with any status, `dismissed` included.
6. Rank by confidence (count × regularity) and return up to 20, with the sample transaction ids.

`POST /recurring/suggestions/accept` (`{match_key, account_id, type, ...overrides}`) creates the series and
links the sample. `POST /recurring/suggestions/dismiss` stores a `dismissed` series.

### R5. Upcoming view

`GET /recurring/upcoming?days=30` (max 90) lists occurrences in the window across all active series, **plus**
read-only entries derived from the existing account dues (card statement, loan instalment, FD maturity) so the
user sees one list. These entries are computed from the same functions `reminder_sync.go` uses; they are not
duplicated as series. Response:

- `items[]`: `{series_id | account_id, name, kind, due_on, amount, currency, state, price_changed}`
- `totals`: `expense_base`, `income_base` in the base currency at the current rate, plus `fx_missing` /
  `unconverted`.

`GET /recurring` lists series with `next_due_on`, `last_paid_on`, a 12-month `history` of linked amounts and
`monthly_equivalent_base` (weekly × 52/12, yearly ÷ 12, …). The page uses it for "Subscriptions cost you
RM 312 a month".

### R6. Auto-post (Q2)

When a series has `auto_post = true`, and only for `fixed` series, `POST /pipeline/recurring/process`
(called daily by the oracle, after fixed deposits) creates the transaction on its due date. The posted row has
`source = system`, the series' category, the description from `name` and `recurring_id` set. It runs through
the normal balance-writing service in one DB transaction per occurrence. It is idempotent via
`import_ref = recurring:<series_id>:<due_on>`. If a matching transaction was already entered by hand, the
occurrence is `paid` and nothing is posted. A cross-currency transfer series needs `to_amount` stored on the
series to auto-post; otherwise auto-post is refused (`RECURRING_AUTOPOST_UNSUPPORTED`).

### R7. Reminders

A new `ReminderKind` `bill_due` (ref = series id), synced by `reminder_sync.go` for series with `remind = true`,
3 days before (lead days per kind, as today). `telegram_links.remind_bill_due` (default true) is the per-user
switch, alongside the existing three. An auto-posted series reminds "will be recorded on …" instead.

### R8. API

```
GET    /api/v1/recurring                      # series + next_due_on, last_paid_on, history, monthly_equivalent_base
POST   /api/v1/recurring
GET    /api/v1/recurring/upcoming?days=30
GET    /api/v1/recurring/suggestions
POST   /api/v1/recurring/suggestions/accept
POST   /api/v1/recurring/suggestions/dismiss
GET    /api/v1/recurring/:id                  # + occurrences (last 12, next 3)
PUT    /api/v1/recurring/:id
DELETE /api/v1/recurring/:id                  # unlinks; transactions stay
POST   /api/v1/recurring/:id/link             # {transaction_ids}
POST   /api/v1/recurring/:id/skip             # skips the next occurrence
POST   /api/v1/pipeline/recurring/process     # auto-post (pipeline key)
```

Errors: `RECURRING_NOT_FOUND`, `RECURRING_ACCOUNT_TYPE` (investment / fixed deposit), `RECURRING_LINK_MISMATCH`
(wrong user, type or account), `RECURRING_AUTOPOST_UNSUPPORTED`.

### R9. Web

- **`/recurring` page** (Activity group; also in the mobile More tab and ⌘K). Header: monthly cost of
  subscriptions and bills, monthly recurring income. Sections: Subscriptions, Bills, Income, Transfers. Each
  row shows a merchant icon (category icon), cadence, next date, expected amount, a 12-month mini bar chart
  and badges (price changed, missed, auto). A "Review N suggestions" banner opens a sheet listing
  suggestions with their sample rows, plus accept, edit-then-accept and dismiss.
- **Calendar tab**: the month grid (reuse the spending calendar's layout) with dots on due days.
- **Home "Coming up" card**: the next 7 days from `/recurring/upcoming`, including card and loan dues.
- **Transaction detail and edit dialog**: "Part of Netflix · monthly" with unlink, or "Make recurring",
  which pre-fills the series from the transaction.
- **Record payment** on a `due` occurrence opens the transaction dialog pre-filled.

### R10. MCP, bot, export, demo

- MCP read tools `list_recurring` and `get_upcoming` under `read:transactions` (Q5), with amounts tagged by ISO
  code and totals in the base currency.
- Bot: `/upcoming` (next 14 days) and `bill_due` reminders. Quick-add replies say "Linked to Netflix" when a
  match happens.
- Export: `recurring.csv`; `transactions.csv` gains `recurring_id`.
- Demo data: four series (two subscriptions, one bill, salary), registered in `demo_rows`.

## Part G — Goals

### G1. Model

`goals`:

| Column | Notes |
|---|---|
| `id`, `user_id`, timestamps, `deleted_at` | |
| `name`, `icon` | icon validated like category icons |
| `type` | `save` \| `pay_off` |
| `target_amount` | cents, goal `currency`. Required for `save`; for `pay_off` it is 0 (debt cleared) |
| `currency` | default: the user's base currency |
| `target_date` | nullable |
| `monthly_plan` | nullable cents: what the user intends to put in each month |
| `start_amount`, `started_on` | progress at creation, so "saved since you started" works |
| `status` | `active` \| `achieved` \| `archived` |
| `achieved_on` | set when progress first reaches the target |

`goal_accounts(goal_id, account_id, share_pct)`: `share_pct` from 1 to 100. The sum over all active
goals for one account is ≤ 100 (`GOAL_ALLOCATION_EXCEEDED`), so a savings account can be 60% house and 40%
emergency fund (Q3).

### G2. Progress (computed on read)

- **Save:** `current` = Σ(linked account balance × share) converted to the goal currency at the current rate.
  Cash, investment (market value) and fixed-deposit accounts are allowed; credit cards and debt are not
  (`GOAL_ACCOUNT_TYPE`). Progress = `current / target_amount`, capped at 100% for display.
- **Pay off:** exactly one credit-card or debt account at 100%. Progress =
  `(start_balance − current_balance) / start_balance`.
- **History:** from `account_balance_snapshots × share`, converted at each snapshot's day (`datedValuer`),
  weekly points. Investment accounts only have snapshots from migration 53 onward (plan 023 note), so history
  starts there.
- Goals never change net worth or any aggregate: they are views over accounts.

### G3. Pace and projection

- `pace_monthly` = average monthly change over the last 3 months of history (fewer if the goal is newer;
  "not enough history yet" under 21 days).
- With a `target_date`: `required_monthly = remaining / months_left`. Status: `on_track` when
  `pace_monthly ≥ required_monthly` (or the projection lands by the target date), `behind`, or `achieved`.
- `projected_on` = today + remaining / pace (none when pace ≤ 0).
- **Pay off** uses amortisation when the account has `interest_rate` and `monthly_payment`. It gives the months
  to payoff and the total interest, computed in Go with the float rate and the balance rounded to cents each
  month, the same convention as `fdmath`. The calculator takes an `extra_monthly` amount and returns months
  and interest saved: `GET /goals/:id/payoff?extra_monthly=`.
- A goal reaching 100% sets `achieved` + `achieved_on` on read (one UPDATE). The UI celebrates once.

### G4. API

```
GET    /api/v1/goals                 # with progress, pace, status, projected_on
POST   /api/v1/goals                 # + accounts: [{account_id, share_pct}]
GET    /api/v1/goals/:id             # + history
PUT    /api/v1/goals/:id
DELETE /api/v1/goals/:id
GET    /api/v1/goals/:id/payoff      # pay_off only; ?extra_monthly=
```

Errors: `GOAL_NOT_FOUND`, `GOAL_ACCOUNT_TYPE`, `GOAL_ALLOCATION_EXCEEDED`, `GOAL_INVALID_TARGET`.

### G5. Web

- **`/goals` page** (Wealth group, next to Accounts and Investments). Each card shows a progress ring, current /
  target in `<Money>`, the projected date and on-track chip, and a sparkline. An empty state offers templates:
  emergency fund (3× typical monthly spend from `/analytics/typical`), house deposit, pay off a card or loan.
- **Detail page:** the history area chart (`ThemedAreaChart`) with the target line and, where there is a
  target date, a dashed "required pace" line. It also shows account allocations, and for pay-off goals the
  payoff calculator (an extra-per-month slider showing months and interest saved).
- **Create/edit dialog:** type, target, date, monthly plan, and account picker with share sliders. It shows how
  much of each account is already allocated.
- **Home:** a goals card (top 3, like the budgets card) shown when any goal exists.
- **Account page:** "Allocated to: House 60%".

### G6. MCP, bot, export, demo

- MCP `list_goals` under `read:accounts` (Q5).
- Bot `/goals`: progress and status per goal.
- Export: `goals.csv`, `goal_accounts.csv`.
- Demo: an emergency-fund goal on the savings account and a pay-off goal on the card.

## Phases

| Phase | Scope | Notes |
|---|---|---|
| 1 | **Recurring core**: R1–R3, R5, R8 (no suggestions or pipeline), R9 page + Home card + transaction link | Usable with manual series |
| 2 | **Detection**: R4, the suggestions UI, price-change prompt | The "it found my subscriptions" moment |
| 3 | **Automation and reach**: R6 auto-post + oracle call, R7 reminders, R10 (MCP, bot, export, demo) | |
| 4 | **Goals core**: G1–G2, G3 for `save`, G4, G5 page, dialog, Home card | Independent of 1–3 |
| 5 | **Pay-off goals**: G3 amortisation + calculator, G6 | |

Goals (4–5) don't depend on recurring and can run in parallel with phases 2–3. Each phase ends with
`./scripts/check-go.sh apps/api` (and `apps/oracle` for phase 3), plus `pnpm build`, `pnpm lint` and `pnpm test`,
and Playwright e2e for the new pages.

## Testing

- **Detection:** table tests with synthetic histories: monthly with ±3-day jitter, a month-end anchor
  (31 → 28/30), biweekly salary, yearly insurance (2 occurrences), variable utility bills, a one-off
  purchase at the same merchant, two different subscriptions at one merchant (Apple), a skipped month. Plus
  false-positive checks: weekly groceries at varying amounts must not be suggested as `fixed`.
- **Matching:** each write path (web, bot, MCP, CSV) links; IBKR rows and opening balances never link;
  user isolation; a transaction can't link to another user's series.
- **Occurrences:** paid, due, missed, skip, ended; the window boundaries; `skipped_through`; DST-free UTC
  dates.
- **Auto-post:** idempotency (run twice → one row), a manually entered payment prevents the post, the
  balance moves, a deleted series stops posting, cross-currency refusal.
- **Goals:** allocation sum ≤ 100 across goals, including on update; FX missing → `fx_missing` and no
  progress figure; history converted per day; pace with < 21 days; achieved transition happens once;
  amortisation against known schedules (a 0% rate, a payment smaller than interest → "never").
- Coverage target 95% on matching, occurrence and auto-post logic (it moves balances).

## Risks

- **False positives in detection** erode trust fast. Mitigations: suggestions are never applied without a
  click, the tolerances are conservative, and dismissals are remembered.
- **Auto-matching links the wrong row** (two Grab rides in one window). It only links within an unpaid
  occurrence's window and only to the closest-dated candidate; the link is visible and undoable.
- **Auto-post double counting** when the user also enters the payment by hand later. A later manual
  entry that matches an auto-posted occurrence gets a "this looks like a duplicate of …" warning in the
  dialog, not a silent link.
- **Write-path cost:** one extra query per transaction write; covered by an index on
  `recurring_series(user_id, status, account_id)`.
- **Goal history gaps** before snapshots existed; show history from the first snapshot and say so.
- **Migration numbering:** as in plan 023, assign numbers at merge.

## Decisions

| # | Question | Options | Recommended |
|---|---|---|---|
| Q1 | Where does recurring live in the nav? | (a) own page under Activity; (b) a tab on Transactions | **(a)**, plus the Home card |
| Q2 | Auto-post recurring transactions? | (a) never, detect only (Monarch's model); (b) per-series opt-in, fixed amounts only | **(b)**: without bank sync, posting rent or Netflix yourself every month is the chore this saves |
| Q3 | How goals measure progress | (a) linked accounts with % shares; (b) contributions linked transaction by transaction; (c) both | **(a)**: nothing to maintain after setup, and history comes from snapshots |
| Q4 | Pay-off goals in v1? | (a) yes (phase 5); (b) save goals only | **(a)**: the loan fields already exist and the calculator is cheap |
| Q5 | OAuth scopes for the new MCP tools | (a) reuse `read:transactions` / `read:accounts`; (b) new `read:recurring` / `read:goals` | **(a)**: new scopes would require existing connections to consent again |
| Q6 | Card and loan dues in the upcoming list | (a) merge them in, read-only; (b) migrate them into series | **(a)**: no data migration, one list for the user |

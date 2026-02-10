# Investments Frontend

## Context

The Kuberan backend (Plans 001-006 complete) has full investment tracking: users can create investment accounts, add holdings linked to normalized securities, record buy/sell/dividend/split transactions, update prices, view portfolio summaries, browse securities, query price history, and retrieve portfolio snapshots over time. The frontend (Plans 002-005) has a complete UI for accounts, transactions, categories, budgets, and dashboard charts.

However, the frontend has zero investment functionality:

1. **No investments page**: The sidebar links to `/investments` but the route does not exist (404).
2. **No securities browsing**: Securities exist in the backend but cannot be viewed from the UI. Users need to discover and select securities when adding investments.
3. **Stale frontend types**: The `Investment` model in `types/models.ts` still has the old denormalized schema (`symbol`, `name`, `asset_type` directly on Investment) instead of the normalized `security_id` + `Security` relation from Plan 006.
4. **No API types or hooks**: No request/response types, no React Query hooks for any investment, security, or snapshot endpoint.
5. **No investment action dialogs**: No UI for adding investments, recording buys/sells/dividends/splits.
6. **Placeholder on account detail**: The account detail page has a static placeholder card for investment accounts (lines 420-430 of `accounts/[id]/page.tsx`) that says "Investment holdings for this account will be shown here."
7. **Dashboard shows account balances, not portfolio value**: The "Investments" summary card on the dashboard sums `account.balance` for investment accounts, which is the cash balance of the brokerage account, not the actual portfolio value (holdings x current price).
8. **No search on ListSecurities**: The backend `ListSecurities` endpoint only supports pagination, not search filtering. The frontend needs search-as-you-type for the security selector in the Add Investment dialog.

This plan adds the complete investments frontend: types, hooks, pages, dialogs, account detail integration, dashboard enhancement, and one small backend change (search on ListSecurities).

**Out of scope**: Price charts using Recharts (deferred to a dedicated charting plan), security creation UI (pipeline-only), real-time price updates, portfolio performance analytics.

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Backend: ListSecurities search | Backend | Add `search` param to interface, service, handler, tests |
| Frontend types update | Frontend | Update Investment model, add Security/SecurityPrice/PortfolioSnapshot models, add all API request/response types |
| Investment hooks | Frontend | New hooks for portfolio, investments, buy/sell/dividend/split, price update |
| Security hooks | Frontend | New hooks for securities list (with search), detail, price history |
| Portfolio snapshot hooks | Frontend | New hooks for snapshot time-series |
| Securities browse page | Frontend | `/securities` — search, paginated table |
| Security detail page | Frontend | `/securities/[id]` — info cards, price history table |
| Portfolio/investments page | Frontend | `/investments` — portfolio summary cards, net worth chart, holdings table |
| Investment detail page | Frontend | `/investments/[id]` — stats, transaction history, action buttons |
| Investment action dialogs | Frontend | Add Investment, Record Buy, Record Sell, Record Dividend, Record Split |
| Account detail integration | Frontend | Replace placeholder card with real holdings table + Add Investment button |
| Sidebar update | Frontend | Add Securities nav item |
| Dashboard enhancement | Frontend | Replace account-balance sum with real portfolio value from `usePortfolio()` |

## Technology & Patterns

This plan follows all patterns established in Plans 002-005:
- Frontend: Next.js 15 App Router, TypeScript strict mode, Tailwind CSS v4, ShadCN UI components
- Data fetching: `@tanstack/react-query` with query key factories, hooks wrapping `apiClient`
- Charts: Recharts via ShadCN `ChartContainer` / `ChartTooltip`
- Dialogs: ShadCN `Dialog` components with form state, `toast` notifications via Sonner
- Monetary display: `formatCurrency()` from `@/lib/format` (converts cents to display string)
- Error handling: `ApiClientError` catch in mutation handlers

Backend change follows Plan 006 patterns:
- 3-layer architecture, interface-based services, table-driven tests

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Securities page location | `/securities` as top-level route | Securities are shared entities, not scoped under investments. Users may want to browse securities before owning any. |
| Security selector in Add Investment | Command/Popover with debounced search | Standard ShadCN combobox pattern. Debounce avoids excessive API calls on every keystroke. Requires backend search support. |
| Backend search implementation | `LOWER(symbol) LIKE ? OR LOWER(name) LIKE ?` | Simple, effective for the small-to-medium security catalog. No full-text search needed. |
| Net worth chart data source | `usePortfolioSnapshots` (GET /investments/snapshots) | Pipeline-generated snapshots provide historical data. If no snapshots exist, chart shows empty state. |
| Portfolio summary data source | `usePortfolio()` (GET /investments/portfolio) | Real-time computation from current holdings. More accurate than account balances. |
| Investment detail navigation | Clicking a holding row navigates to `/investments/[id]` | Consistent with account detail pattern. |
| Action buttons on investment detail | Buy, Sell, Dividend, Split as separate dialog triggers | Each action has different fields. Separate dialogs keep forms focused. |
| Account detail integration | Replace placeholder with holdings table + Add Investment button | Only shown for `account.type === "investment"`. Links to individual investment detail pages. |
| Dashboard investments card | Replace `account.balance` sum with `usePortfolio().total_value` and show gain/loss | Account balance is cash in the brokerage, not portfolio value. Portfolio value = sum(holdings x current_price). |
| Types update approach | Update `Investment` in-place, add new types | No existing code uses the old Investment fields. Clean update with no migration needed. |

---

## Phase 1: Backend — Add Search to ListSecurities

### 1.1 Update SecurityServicer Interface

**File**: `apps/api/internal/services/interfaces.go`

Change `ListSecurities(page pagination.PageRequest)` to `ListSecurities(search string, page pagination.PageRequest)`.

### 1.2 Update Security Service Implementation

**File**: `apps/api/internal/services/security_service.go`

In `ListSecurities`, when `search` is non-empty:
- Apply `LOWER(symbol) LIKE ? OR LOWER(name) LIKE ?` with `%search%` pattern.
- Both comparisons are case-insensitive via `LOWER()`.

When `search` is empty, query all securities (existing behavior).

### 1.3 Update Security Handler

**File**: `apps/api/internal/handlers/security_handler.go`

In `ListSecurities`:
- Parse `search` query parameter: `c.Query("search")`.
- Pass to service: `h.securityService.ListSecurities(search, page)`.
- Update Swagger annotation to document `search` query parameter.

### 1.4 Update Security Service Tests

**File**: `apps/api/internal/services/security_service_test.go`

- Update existing `ListSecurities` calls to pass `""` as search parameter.
- Add new subtests:
  - `search_by_symbol`: Create AAPL, GOOGL, MSFT; search "aapl" -> 1 result.
  - `search_by_name`: Search "apple" -> matches AAPL (Apple Inc.).
  - `search_case_insensitive`: Search "AAPL" and "aapl" both return same result.
  - `search_empty_returns_all`: Search "" -> all securities.

### 1.5 Update Security Handler Tests

**File**: `apps/api/internal/handlers/security_handler_test.go`

- Update `mockSecurityService.ListSecurities` signature to accept `search string`.
- Add test: `passes_search_to_service` — verify search query param is forwarded.

### 1.6 Verification

Run `./scripts/check.sh` from `apps/api/`. All 5 steps must pass.

---

## Phase 2: Frontend Types Update

### 2.1 Update models.ts

**File**: `apps/web/src/types/models.ts`

Add new types:

```typescript
// Security — shared entity for financial instruments
export interface Security extends BaseModel {
  symbol: string;
  name: string;
  asset_type: AssetType;
  currency: string; // ISO 4217
  exchange?: string;
  maturity_date?: string | null; // bonds, ISO 8601
  yield_to_maturity?: number; // bonds, float
  coupon_rate?: number; // bonds, float
  network?: string; // crypto
  property_type?: string; // REITs
}

// Security price (time-series, no soft deletes)
export interface SecurityPrice {
  id: number;
  security_id: number;
  price: number; // cents
  recorded_at: string; // ISO 8601
  security?: Security;
}

// Portfolio snapshot (time-series, no soft deletes)
export interface PortfolioSnapshot {
  id: number;
  user_id: number;
  recorded_at: string; // ISO 8601
  total_net_worth: number; // cents
  cash_balance: number; // cents
  investment_value: number; // cents
  debt_balance: number; // cents
}
```

Refactor `Investment`:
- Remove: `symbol`, `name`, `asset_type`, `currency`, `exchange`, `maturity_date`, `yield_to_maturity`, `coupon_rate`, `network`, `property_type`.
- Add: `security_id: number`, `security: Security` (preloaded relation), `account?: Account` (preloaded relation).
- Keep: `account_id`, `quantity`, `cost_basis`, `current_price`, `last_updated`, `wallet_address`.

Add `investment?: Investment` optional relation to `InvestmentTransaction`.

### 2.2 Update api.ts

**File**: `apps/web/src/types/api.ts`

Add import of new model types. Add:

```typescript
// Investment response wrappers
export interface InvestmentResponse {
  investment: Investment;
}

export interface InvestmentTransactionResponse {
  transaction: InvestmentTransaction;
}

export interface SecurityResponse {
  security: Security;
}

export interface PortfolioResponse {
  portfolio: PortfolioSummary;
}

// Investment requests
export interface AddInvestmentRequest {
  account_id: number;
  security_id: number;
  quantity: number; // float, > 0
  purchase_price: number; // cents, > 0
  wallet_address?: string;
}

export interface RecordBuyRequest {
  date: string; // ISO 8601
  quantity: number; // float, > 0
  price_per_unit: number; // cents, > 0
  fee?: number; // cents, >= 0
  notes?: string;
}

export interface RecordSellRequest {
  date: string; // ISO 8601
  quantity: number; // float, > 0
  price_per_unit: number; // cents, > 0
  fee?: number; // cents, >= 0
  notes?: string;
}

export interface RecordDividendRequest {
  date: string; // ISO 8601
  amount: number; // cents, > 0
  dividend_type?: string;
  notes?: string;
}

export interface RecordSplitRequest {
  date: string; // ISO 8601
  split_ratio: number; // float, > 0
  notes?: string;
}

export interface UpdatePriceRequest {
  current_price: number; // cents, > 0
}

// Security filters
export interface SecurityFilters extends PaginationParams {
  search?: string;
}

// Portfolio snapshot filters
export interface PortfolioSnapshotFilters extends PaginationParams {
  from_date: string;
  to_date: string;
}
```

### 2.3 Verification

Run `npm run build` from `apps/web/`. No type errors.

---

## Phase 3: React Query Hooks

### 3.1 Create use-investments.ts

**New file**: `apps/web/src/hooks/use-investments.ts`

Query key factory:
```typescript
export const investmentKeys = {
  all: ["investments"] as const,
  portfolio: () => [...investmentKeys.all, "portfolio"] as const,
  lists: () => [...investmentKeys.all, "list"] as const,
  list: (accountId: number, params?: PaginationParams) =>
    [...investmentKeys.lists(), accountId, params] as const,
  details: () => [...investmentKeys.all, "detail"] as const,
  detail: (id: number) => [...investmentKeys.details(), id] as const,
  transactions: (id: number, params?: PaginationParams) =>
    [...investmentKeys.all, "transactions", id, params] as const,
};
```

Hooks:
- `usePortfolio()` — GET /api/v1/investments/portfolio, unwrap `res.portfolio`.
- `useInvestment(id)` — GET /api/v1/investments/:id, unwrap `res.investment`, enabled when id > 0.
- `useAccountInvestments(accountId, params?)` — GET /api/v1/accounts/:accountId/investments, returns `PageResponse<Investment>`.
- `useInvestmentTransactions(investmentId, params?)` — GET /api/v1/investments/:id/transactions, returns `PageResponse<InvestmentTransaction>`.
- `useAddInvestment()` — POST /api/v1/investments, invalidates investment + account queries.
- `useRecordBuy(investmentId)` — POST /api/v1/investments/:id/buy, invalidates investment + account queries.
- `useRecordSell(investmentId)` — POST /api/v1/investments/:id/sell, invalidates investment + account queries.
- `useRecordDividend(investmentId)` — POST /api/v1/investments/:id/dividend, invalidates investment queries.
- `useRecordSplit(investmentId)` — POST /api/v1/investments/:id/split, invalidates investment queries.
- `useUpdateInvestmentPrice(investmentId)` — PUT /api/v1/investments/:id/price, invalidates investment + account queries.

### 3.2 Create use-securities.ts

**New file**: `apps/web/src/hooks/use-securities.ts`

Query key factory:
```typescript
export const securityKeys = {
  all: ["securities"] as const,
  lists: () => [...securityKeys.all, "list"] as const,
  list: (params?: SecurityFilters) =>
    [...securityKeys.lists(), params] as const,
  details: () => [...securityKeys.all, "detail"] as const,
  detail: (id: number) => [...securityKeys.details(), id] as const,
  prices: (id: number, from: string, to: string) =>
    [...securityKeys.all, "prices", id, from, to] as const,
};
```

Hooks:
- `useSecurities(params?)` — GET /api/v1/securities with search + pagination, returns `PageResponse<Security>`.
- `useSecurity(id)` — GET /api/v1/securities/:id, unwrap `res.security`, enabled when id > 0.
- `useSecurityPriceHistory(id, from, to, params?)` — GET /api/v1/securities/:id/prices with from_date/to_date, returns `PageResponse<SecurityPrice>`.

### 3.3 Create use-portfolio-snapshots.ts

**New file**: `apps/web/src/hooks/use-portfolio-snapshots.ts`

Query key factory and single hook:
- `usePortfolioSnapshots(params)` — GET /api/v1/investments/snapshots with from_date/to_date + pagination, returns `PageResponse<PortfolioSnapshot>`. Enabled when both dates are set.

### 3.4 Verification

Run `npm run build` from `apps/web/`. No type errors.

---

## Phase 4: Securities Pages

### 4.1 Securities Browse Page

**New file**: `apps/web/src/app/(dashboard)/securities/page.tsx`

"use client" page with:
- Debounced search input (300ms delay using `useEffect` timer pattern).
- `useSecurities({ search: debouncedSearch, page, page_size: 20 })` query.
- Three render states: loading (Skeleton rows), empty (friendly message), data.
- Table columns: Symbol (monospace, bold), Name, Type (Badge), Currency, Exchange.
- Row click navigates to `/securities/[id]`.
- Pagination controls (Previous/Next buttons, "Page X of Y").

### 4.2 Security Detail Page

**New file**: `apps/web/src/app/(dashboard)/securities/[id]/page.tsx`

"use client" page with:
- Back button to `/securities`.
- Security header: symbol, name, asset type badge.
- Info cards: Currency, Exchange (if present).
- Asset-specific fields shown conditionally:
  - Bonds: Maturity Date, Yield to Maturity, Coupon Rate.
  - Crypto: Network.
  - REITs: Property Type.
- Price history table with period selector (1M, 3M, 6M, 1Y).
  - Uses `useSecurityPriceHistory` with computed `from_date` based on selected period.
  - Table columns: Date, Price.
  - Paginated.
- Loading and empty states.

### 4.3 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 5: Portfolio & Investments Page

### 5.1 Portfolio/Investments Page

**New file**: `apps/web/src/app/(dashboard)/investments/page.tsx`

"use client" page with:
- `usePortfolio()` for summary data.
- Summary cards row (4 cards): Total Value, Cost Basis, Gain/Loss (green/red based on sign), Holdings Count.
- Net worth area chart:
  - Uses `usePortfolioSnapshots` with period selector (1M, 3M, 6M, 1Y, ALL).
  - Recharts `AreaChart` via ShadCN `ChartContainer`.
  - X-axis: dates, Y-axis: total_net_worth in formatted currency.
  - Shows empty state if no snapshots available.
- Holdings table:
  - Shows all investments from portfolio (or a dedicated query — `usePortfolio` returns `holdings_by_type` but not individual holdings). For individual holdings, use `useAccountInvestments` across all investment accounts, or add a dedicated endpoint. **Decision**: Query all accounts of type "investment" from `useAccounts`, then for the first investment account, use `useAccountInvestments`. If multiple investment accounts exist, aggregate or show by account.
  - **Revised approach**: Since there's no "all user investments" endpoint, the portfolio page shows:
    - Portfolio summary cards (from `usePortfolio()`).
    - Net worth chart (from `usePortfolioSnapshots()`).
    - Holdings breakdown by asset type from `usePortfolio().holdings_by_type` (total value and count per type).
    - Link to each investment account's detail page for per-holding views.
  - Table columns: Asset Type, Holdings Count, Total Value.
  - Clicking a row could filter/navigate to accounts of that type.

### 5.2 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 6: Investment Detail Page & Action Dialogs

### 6.1 Investment Detail Page

**New file**: `apps/web/src/app/(dashboard)/investments/[id]/page.tsx`

"use client" page with:
- Back button to `/investments`.
- Investment header: Security symbol + name, asset type badge.
- Stat cards (6): Current Price, Quantity, Market Value (quantity x current_price), Cost Basis, Gain/Loss (market value - cost basis, colored), Account (link to account).
- Action buttons row: Buy, Sell, Dividend, Split — each opens respective dialog.
- Transaction history table:
  - `useInvestmentTransactions(id, { page, page_size: 20 })`.
  - Columns: Date, Type (badge), Quantity, Price/Unit, Total, Fee, Notes.
  - Paginated.

### 6.2 Add Investment Dialog

**New file**: `apps/web/src/components/investments/add-investment-dialog.tsx`

Props: `accountId: number`, `open: boolean`, `onOpenChange: (open: boolean) => void`.

Form:
- Security selector: Command/Popover combobox with debounced search (300ms).
  - Uses `useSecurities({ search: debouncedSearch, page_size: 20 })`.
  - `shouldFilter={false}` on Command since server-side filtering.
  - Shows Symbol (monospace), Name, Asset Type badge, Exchange for each result.
- Quantity: number input (step="any", min=0).
- Purchase Price (per unit): CurrencyInput component.
- Wallet Address: text input, shown only when selected security is crypto.
- Submit calls `useAddInvestment()`.
- Success toast, form reset, dialog close.
- Error display for `ApiClientError`.

### 6.3 Record Buy Dialog

**New file**: `apps/web/src/components/investments/record-buy-dialog.tsx`

Props: `investmentId: number`, `open: boolean`, `onOpenChange`.

Form fields: Date (date input), Quantity (number), Price per Unit (CurrencyInput), Fee (CurrencyInput, optional), Notes (textarea).
- Submit calls `useRecordBuy(investmentId)`.
- Date converted to RFC3339 via `toRFC3339()`.

### 6.4 Record Sell Dialog

**New file**: `apps/web/src/components/investments/record-sell-dialog.tsx`

Same structure as Record Buy, but:
- Shows current quantity as hint ("You hold X units").
- Validates quantity <= current holdings client-side.
- Catches `INSUFFICIENT_SHARES` error from backend.
- Submit calls `useRecordSell(investmentId)`.

### 6.5 Record Dividend Dialog

**New file**: `apps/web/src/components/investments/record-dividend-dialog.tsx`

Form fields: Date, Amount (CurrencyInput), Dividend Type (Select: Cash, Stock, Special), Notes.
- Submit calls `useRecordDividend(investmentId)`.

### 6.6 Record Split Dialog

**New file**: `apps/web/src/components/investments/record-split-dialog.tsx`

Form fields: Date, Split Ratio (number input, step="any"), Notes.
- Submit calls `useRecordSplit(investmentId)`.

### 6.7 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 7: Account Detail Integration

### 7.1 Replace Investment Placeholder

**File**: `apps/web/src/app/(dashboard)/accounts/[id]/page.tsx`

For investment accounts (`account.type === "investment"`), replace the placeholder card (lines 420-430) with:

1. Add imports: `useAccountInvestments` from hooks, `AddInvestmentDialog` from components.
2. Add state: `addInvestmentOpen`, `investmentPage`.
3. Add query: `useAccountInvestments(accountId, { page: investmentPage, page_size: 20 })` — only enabled when account type is "investment" (pass `accountId` of 0 when not investment to disable).
4. Replace placeholder card with:
   - Card header with "Investments" title, item count, and "Add Investment" button.
   - Holdings table: Symbol (monospace, linked to `/investments/[id]`), Name, Quantity, Current Price, Market Value (quantity x current_price), Gain/Loss.
   - Empty state when no investments.
   - Pagination controls.
5. Render `AddInvestmentDialog` with `accountId`.

### 7.2 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 8: Sidebar & Dashboard Enhancement

### 8.1 Add Securities to Sidebar

**File**: `apps/web/src/components/layout/app-sidebar.tsx`

Add `{ title: "Securities", href: "/securities", icon: Database }` to `navItems` array between Budgets and Investments. Import `Database` icon from `lucide-react`.

### 8.2 Enhance Dashboard Investments Card

**File**: `apps/web/src/app/(dashboard)/page.tsx`

In the `SummaryCards` component:
- Import and call `usePortfolio()`.
- Replace the "Investments" card content:
  - Old: `formatCurrency(investmentTotal)` (sum of investment account balances).
  - New: `formatCurrency(portfolio.total_value)` (actual portfolio value from holdings).
  - Show gain/loss below: `+$X,XXX (+Y.YY%)` in green, or `-$X,XXX (-Y.YY%)` in red.
- Graceful fallback: if `usePortfolio()` is loading or errors, fall back to the existing account-balance-based value.

### 8.3 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 9: Final Verification

### 9.1 Frontend Lint

Run `npm run lint` from `apps/web/`. Zero warnings/errors.

### 9.2 Frontend Build

Run `npm run build` from `apps/web/`. All routes compile successfully.

### 9.3 Backend Check

Run `./scripts/check.sh` from `apps/api/`. All 5 steps pass.

### 9.4 Swagger Regeneration

Run `swag init -g cmd/api/main.go -d . --output internal/docs --parseDependency` from `apps/api/`. Verify the `search` query param appears on the `GET /securities` endpoint in swagger.json.

### 9.5 Verification Checklist

- [ ] `ListSecurities` backend endpoint supports `search` query parameter
- [ ] Frontend `Investment` type matches backend model (security_id + Security relation)
- [ ] All new hooks follow established pattern (query key factories, unwrap wrappers, invalidate on mutation)
- [ ] Securities page has working search with debounce
- [ ] Security detail page shows asset-specific fields conditionally
- [ ] Portfolio page shows summary cards, net worth chart, and holdings breakdown
- [ ] Investment detail page shows stats, action buttons, and transaction history
- [ ] All 5 action dialogs (Add, Buy, Sell, Dividend, Split) work with correct API calls
- [ ] Account detail page shows real holdings table for investment accounts
- [ ] Sidebar includes Securities nav item
- [ ] Dashboard investments card shows portfolio value (not account balance)
- [ ] No TypeScript `any` types
- [ ] No lint warnings
- [ ] Frontend builds successfully
- [ ] Backend passes full check.sh

---

## File Structure After This Plan

### New Files

```
apps/web/src/
├── hooks/
│   ├── use-investments.ts              # 10 hooks: portfolio, CRUD, buy/sell/dividend/split, price update
│   ├── use-securities.ts               # 3 hooks: list (with search), detail, price history
│   └── use-portfolio-snapshots.ts      # 1 hook: snapshots time-series
├── app/(dashboard)/
│   ├── securities/
│   │   ├── page.tsx                    # Securities browse page with search
│   │   └── [id]/
│   │       └── page.tsx                # Security detail page
│   └── investments/
│       ├── page.tsx                    # Portfolio overview page
│       └── [id]/
│           └── page.tsx                # Investment detail page
└── components/investments/
    ├── add-investment-dialog.tsx        # Security selector + quantity + price
    ├── record-buy-dialog.tsx           # Date + quantity + price + fee + notes
    ├── record-sell-dialog.tsx          # Same + max quantity validation
    ├── record-dividend-dialog.tsx      # Date + amount + type + notes
    └── record-split-dialog.tsx         # Date + ratio + notes
```

### Modified Files

```
apps/api/
├── internal/
│   ├── services/
│   │   ├── interfaces.go               # ListSecurities adds search param
│   │   ├── security_service.go         # LIKE filtering on symbol/name
│   │   └── security_service_test.go    # Updated calls + 4 new search tests
│   └── handlers/
│       ├── security_handler.go         # Parse search query param
│       └── security_handler_test.go    # Updated mock + search test

apps/web/src/
├── types/
│   ├── models.ts                       # Add Security, SecurityPrice, PortfolioSnapshot; refactor Investment
│   └── api.ts                          # Add all investment/security/snapshot request/response types
├── app/(dashboard)/
│   ├── accounts/[id]/page.tsx          # Replace placeholder with real holdings table
│   └── page.tsx                        # Dashboard investments card uses usePortfolio()
└── components/layout/
    └── app-sidebar.tsx                 # Add Securities nav item
```

---

## Implementation Order

```
Phase 1: Backend — Add Search to ListSecurities
  1.1  Update SecurityServicer interface (add search param)
  1.2  Update security service implementation (LIKE filtering)
  1.3  Update security handler (parse search query param)
  1.4  Update security service tests (pass "", add 4 search subtests)
  1.5  Update security handler tests (update mock, add search test)
  1.6  Verification (./scripts/check.sh)

Phase 2: Frontend Types Update
  2.1  Update models.ts (add Security, SecurityPrice, PortfolioSnapshot; refactor Investment)
  2.2  Update api.ts (add all investment/security/snapshot types)
  2.3  Verification (npm run build)

Phase 3: React Query Hooks
  3.1  Create use-investments.ts (10 hooks)
  3.2  Create use-securities.ts (3 hooks)
  3.3  Create use-portfolio-snapshots.ts (1 hook)
  3.4  Verification (npm run build)

Phase 4: Securities Pages
  4.1  Securities browse page with search
  4.2  Security detail page with price history
  4.3  Verification (npm run build)

Phase 5: Portfolio & Investments Page
  5.1  Portfolio overview page (summary cards, net worth chart, holdings breakdown)
  5.2  Verification (npm run build)

Phase 6: Investment Detail Page & Action Dialogs
  6.1  Investment detail page (stats, transactions, action buttons)
  6.2  Add Investment dialog (security selector + form)
  6.3  Record Buy dialog
  6.4  Record Sell dialog
  6.5  Record Dividend dialog
  6.6  Record Split dialog
  6.7  Verification (npm run build)

Phase 7: Account Detail Integration
  7.1  Replace investment placeholder with real holdings table
  7.2  Verification (npm run build)

Phase 8: Sidebar & Dashboard Enhancement
  8.1  Add Securities to sidebar
  8.2  Enhance dashboard investments card
  8.3  Verification (npm run build)

Phase 9: Final Verification
  9.1  Frontend lint (npm run lint)
  9.2  Frontend build (npm run build)
  9.3  Backend check (./scripts/check.sh)
  9.4  Swagger regeneration
  9.5  Verification checklist
```

## Verification

**Backend** — after code change:
```bash
cd apps/api && go build ./...
```
After completing Phase 1:
```bash
cd apps/api && ./scripts/check.sh
```

**Frontend** — after each phase:
```bash
cd apps/web && npm run build
```
After all phases:
```bash
cd apps/web && npm run lint && npm run build
```

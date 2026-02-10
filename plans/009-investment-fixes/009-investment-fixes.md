# Investment Fixes: Account Balances, Holdings Table, and Realized Gain/Loss

## Context

The Kuberan application (Plans 001-008 complete) has a fully functional investment tracking system with securities, live price lookups, portfolio summaries, and a frontend for managing holdings. However, three significant issues remain:

### Issue 1: Investment Account Balances Show Zero

The `Account.Balance` field for investment accounts is **always zero**. When an investment account is created (`CreateInvestmentAccount` in `account_service.go:84-99`), the `Balance` field defaults to `0` and is never updated thereafter. Unlike cash accounts where `UpdateAccountBalance()` adjusts the balance on every transaction, investment operations (`RecordBuy`, `RecordSell`, `RecordDividend`, `RecordSplit`) only update `investments.quantity` and `investments.cost_basis` — they never touch `accounts.balance`.

A dead method `CalculateInvestmentBalance()` exists on the Account model (`account.go:63-75`) but is never called from any service, handler, or test.

**Impact**: Investment account cards on the dashboard, the accounts list page, and account detail headers all display `MYR 0.00` for investment accounts, even when the user has thousands in holdings. The dashboard summary cards partially mitigate this by falling back to `portfolio.total_value` for the aggregate investment number, but individual account cards remain broken.

**Affected pages**:
- Dashboard (`page.tsx:210`): `AccountCard` shows `account.balance` = 0 for each investment account
- Accounts list (`accounts/page.tsx:57`): Table shows `formatCurrency(account.balance)` = 0
- Account detail (`accounts/[id]/page.tsx:235`): Header shows `formatCurrency(account.balance)` = 0

### Issue 2: Investments Page Has No Clickable Holdings

The `/investments` page (`investments/page.tsx`) shows only an aggregate portfolio overview: summary cards (Total Value, Cost Basis, Gain/Loss, Holdings count), a net worth chart, and a holdings-by-asset-type breakdown table. There is **no list of individual investments** that users can click into.

The only way to reach an individual investment detail page (`/investments/[id]`) is through a 3-step path: Accounts → specific investment account → investment row in the holdings table → investment detail. This is unintuitive and not discoverable.

**Missing**: A `GET /api/v1/investments` endpoint that returns all investments across all accounts (currently `GetAccountInvestments` is per-account only), and a corresponding holdings table on the investments page.

### Issue 3: No Realized Gain/Loss Tracking

When a position is partially or fully sold, the realized gain/loss from the sale is not tracked anywhere. The current `RecordSell` method (`investment_service.go:315-370`) adjusts quantity and cost basis proportionally but does not compute or store the realized P&L.

For fully liquidated positions (quantity = 0), the investment detail page shows all zeros: Market Value = 0, Cost Basis = 0, Gain/Loss = 0. This is misleading — the user may have realized a significant loss (e.g., bought SUI for MYR 22,996 and sold for MYR 8,975, a loss of ~MYR 14,021) but the page gives no indication.

**Impact**: Users have no way to see realized gains or losses from sales, whether the position is still open or fully closed.

**Out of scope**: Retroactive backfill of realized gains from historical sell transactions (can be a separate migration/script), tax lot accounting methods (FIFO, LIFO, etc.), and dividend reinvestment tracking.

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Enrich investment account balances | Backend | Modify `GetUserAccounts` and `GetAccountByID` to compute market value at query time |
| Remove dead `CalculateInvestmentBalance` | Backend | Remove unused method from Account model |
| Add `GET /api/v1/investments` endpoint | Backend | New service method, handler, route for listing all investments across accounts |
| Add realized gain/loss tracking | Backend + Migration | New columns on `investments` and `investment_transactions`, computed in `RecordSell` |
| Add realized gain/loss to portfolio | Backend | Include in `PortfolioSummary` response |
| Add holdings table to investments page | Frontend | New table with clickable rows on `/investments` |
| Improve closed position UX | Frontend | "Position Closed" banner, realized P&L display on investment detail |
| Frontend type updates | Frontend | Add `realized_gain_loss` to Investment and InvestmentTransaction types |

## Technology & Patterns

This plan follows all patterns established in Plans 001-008:
- Backend: 3-layer architecture (Handler → Service → Model), interface-based services, AppError types, table-driven tests with in-memory SQLite
- Monetary values as int64 cents
- Pagination via `PageRequest`/`PageResponse[T]`
- Computed fields use `gorm:"-"` pattern (established in Plan 008 for `CurrentPrice`)

Key technical decisions:
- **Query-time balance enrichment**: Investment account balances are computed at query time in the service layer, not persisted. This follows the same pattern as `Investment.CurrentPrice` — always fresh, never stale.
- **Realized gain/loss on sell**: Computed as `sellProceeds - proportionalCostBasis` and stored on both the `InvestmentTransaction` (per-sell breakdown) and accumulated on `Investment.RealizedGainLoss` (running total).
- **Cross-account investment listing**: New endpoint returns all investments across all active investment accounts, with pagination, `Security` and `Account` preloaded, and `CurrentPrice` populated.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Balance enrichment location | Service layer (`GetUserAccounts`, `GetAccountByID`) | Keeps handlers thin, consistent with existing service-layer patterns. Frontend requires zero changes — `account.balance` in the API response will now contain the computed value. |
| Balance persistence | Do not persist, compute at query time | Follows the `CurrentPrice` pattern from Plan 008. Always reflects latest prices. No cache invalidation needed. |
| `CalculateInvestmentBalance` | Remove entirely | Dead code that uses a different approach (model-level, persisted). Our service-level enrichment replaces it. |
| `GET /api/v1/investments` route placement | Before `GET /api/v1/investments/:id` | Gin matches routes in registration order. Static paths must come before parameterized paths. Consistent with existing `GET /portfolio` and `GET /snapshots`. |
| Realized gain/loss storage | Store on both `InvestmentTransaction` and `Investment` | Per-transaction storage gives audit trail. Running total on Investment gives quick access for display without aggregating transactions. |
| Realized gain/loss computation | `sellProceeds - proportionalCostBasis` | Simple average cost method. `sellProceeds = quantity * pricePerUnit - fee`. `proportionalCostBasis = costBasis * (sellQuantity / totalQuantity)`. Matches the existing proportional cost basis reduction logic. |
| Closed position UX | "Position Closed" banner + realized P&L stats | Clear visual distinction. Users see the realized outcome instead of confusing zeros. |
| Backfill of existing data | Out of scope | Existing sell transactions have `realized_gain_loss = 0` (the default). A separate script can retroactively compute these from transaction history, but it's not part of this plan. |

---

## Phase 1: Database Migration

### 1.1 Migration 000016: `add_realized_gain_loss`

**New file**: `apps/api/migrations/000016_add_realized_gain_loss.up.sql`

```sql
ALTER TABLE investments ADD COLUMN realized_gain_loss BIGINT NOT NULL DEFAULT 0;
ALTER TABLE investment_transactions ADD COLUMN realized_gain_loss BIGINT NOT NULL DEFAULT 0;
```

**New file**: `apps/api/migrations/000016_add_realized_gain_loss.down.sql`

```sql
ALTER TABLE investment_transactions DROP COLUMN IF EXISTS realized_gain_loss;
ALTER TABLE investments DROP COLUMN IF EXISTS realized_gain_loss;
```

**Notes**:
- Both columns default to `0`, which is correct for existing data (existing sells have unknown realized gain/loss; new sells will compute it).
- Down migration drops in reverse order for safety.
- `NOT NULL DEFAULT 0` ensures no nullable issues.

### 1.2 Verification

Run `go build ./...` from `apps/api/` to confirm migration file additions don't affect compilation.

---

## Phase 2: Model Changes

### 2.1 Add `RealizedGainLoss` to Investment Model

**File**: `apps/api/internal/models/investment.go`

Add to the `Investment` struct:

```go
RealizedGainLoss int64 `gorm:"type:bigint;not null;default:0" json:"realized_gain_loss"`
```

### 2.2 Add `RealizedGainLoss` to InvestmentTransaction Model

**File**: `apps/api/internal/models/investment.go`

Add to the `InvestmentTransaction` struct:

```go
RealizedGainLoss int64 `gorm:"type:bigint;not null;default:0" json:"realized_gain_loss"`
```

### 2.3 Remove Dead Code from Account Model

**File**: `apps/api/internal/models/account.go`

Remove the entire `CalculateInvestmentBalance` method (lines 63-75). This method is dead code — never called from any service, handler, or test. It also uses a stale approach (model-level persistence) that conflicts with our service-level query-time enrichment pattern.

Also remove the `"gorm.io/gorm"` import and `"time"` import if they are no longer used by other code in the file. (The `BeforeCreate` hook still uses `gorm.DB`, so the import stays.)

### 2.4 Verification

Run `go build ./...` from `apps/api/`. Should compile — the new fields have default values, and removing unused code shouldn't break anything.

---

## Phase 3: Backend — Enrich Investment Account Balances

### 3.1 Modify `GetUserAccounts` to Compute Investment Account Balances

**File**: `apps/api/internal/services/account_service.go`

After fetching the paginated list of accounts (existing lines 145-148), add enrichment logic:

1. Collect IDs of all investment-type accounts from the result set.
2. If none, return as-is (no overhead for users with only cash accounts).
3. Query investments for those account IDs: `SELECT account_id, security_id, quantity FROM investments WHERE account_id IN (?) AND deleted_at IS NULL`.
4. Collect distinct security IDs from the investments.
5. Call `getLatestPrices(s.db, securityIDs)` to batch-fetch latest prices.
6. For each investment, compute `value = int64(quantity * float64(price))` and accumulate by account ID.
7. Set `account.Balance` on each investment account in the result set to the computed total.

This is a transient enrichment — the balance is never written back to the database.

**Notes**:
- `getLatestPrices` is a package-level function already in `investment_service.go`, accessible from `account_service.go` since both are in the `services` package.
- For accounts with no investments, the balance remains `0`.
- For investments with no security price, they contribute `0` (consistent with Plan 008's zero-price fallback).

### 3.2 Modify `GetAccountByID` to Compute Investment Account Balance

**File**: `apps/api/internal/services/account_service.go`

After fetching a single account (existing lines 156-163), add enrichment:

1. If the account is not `AccountTypeInvestment`, return as-is.
2. Query investments for this account: `SELECT security_id, quantity FROM investments WHERE account_id = ? AND deleted_at IS NULL`.
3. Collect security IDs, call `getLatestPrices`, compute total value, set `account.Balance`.

### 3.3 Verification

Run `go build ./...` from `apps/api/`. Then run `make check-fast` (build + vet + lint).

---

## Phase 4: Backend — Add `GET /api/v1/investments` Endpoint

### 4.1 Add `GetAllInvestments` to Investment Service

**File**: `apps/api/internal/services/investment_service.go`

New method on `investmentService`:

```go
func (s *investmentService) GetAllInvestments(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error)
```

Implementation:
1. Find all active investment account IDs for the user.
2. If none, return empty paginated response.
3. Count total investments across those accounts.
4. Query investments with `Preload("Security")` and `Preload("Account")`, filtered by account IDs, with pagination.
5. Batch-populate `CurrentPrice` from `getLatestPrices`.
6. Return paginated response.

### 4.2 Add `GetAllInvestments` to `InvestmentServicer` Interface

**File**: `apps/api/internal/services/interfaces.go`

Add to the `InvestmentServicer` interface:

```go
GetAllInvestments(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error)
```

### 4.3 Add `GetAllInvestments` Handler

**File**: `apps/api/internal/handlers/investment_handler.go`

New handler method on `investmentHandler`:

```go
func (h *investmentHandler) GetAllInvestments(c *gin.Context)
```

Implementation:
1. Extract user ID from context (existing pattern).
2. Parse pagination query params (`page`, `page_size`).
3. Call `h.investmentService.GetAllInvestments(userID, pageReq)`.
4. Return JSON response with investments.
5. Add Swagger annotations.

### 4.4 Register Route

**File**: `apps/api/cmd/api/main.go`

Add route **before** `investments.GET("/:id", ...)` to avoid Gin route conflict:

```go
investments.GET("", investmentHandler.GetAllInvestments)
```

The existing route block currently has:
```go
investments.POST("", investmentHandler.AddInvestment)
investments.GET("/portfolio", investmentHandler.GetPortfolio)
investments.GET("/snapshots", snapshotHandler.GetSnapshots)
investments.GET("/:id", investmentHandler.GetInvestment)
```

The `GET ""` should go alongside `POST ""` near the top.

### 4.5 Register Route in Integration Test Setup

**File**: `apps/api/tests/integration/setup_test.go`

Add the same route registration to the test router setup.

### 4.6 Verification

Run `go build ./...` from `apps/api/`. Then run `make check-fast`.

---

## Phase 5: Backend — Realized Gain/Loss in RecordSell

### 5.1 Modify `RecordSell` to Compute and Store Realized Gain/Loss

**File**: `apps/api/internal/services/investment_service.go`

In the `RecordSell` method:

1. After computing `costBasisReduction` (existing line 336), compute realized gain/loss:
   ```go
   realizedGainLoss := totalAmount - costBasisReduction
   ```
   Where `totalAmount` = sell proceeds (quantity * pricePerUnit - fee) and `costBasisReduction` = proportional cost basis.

2. Set `RealizedGainLoss` on the `InvestmentTransaction`:
   ```go
   invTx = models.InvestmentTransaction{
       ...
       RealizedGainLoss: realizedGainLoss,
   }
   ```

3. Within the DB transaction, also update the investment's accumulated realized gain/loss:
   ```go
   newRealizedGainLoss := investment.RealizedGainLoss + realizedGainLoss
   if txErr := tx.Model(investment).Updates(map[string]interface{}{
       "quantity":            newQuantity,
       "cost_basis":          newCostBasis,
       "realized_gain_loss":  newRealizedGainLoss,
   }).Error; txErr != nil { ... }
   ```

### 5.2 Add Realized Gain/Loss to Portfolio Summary

**File**: `apps/api/internal/services/interfaces.go`

Add to `PortfolioSummary`:

```go
TotalRealizedGainLoss int64 `json:"total_realized_gain_loss"`
```

### 5.3 Update `GetPortfolio` to Include Realized Gain/Loss

**File**: `apps/api/internal/services/investment_service.go`

In the `GetPortfolio` method, after iterating investments to compute values, also sum `inv.RealizedGainLoss`:

```go
summary.TotalRealizedGainLoss += inv.RealizedGainLoss
```

### 5.4 Verification

Run `go build ./...` from `apps/api/`. Then run `make check-fast`.

---

## Phase 6: Test Fixture Updates

### 6.1 Update `CreateTestInvestment` Fixture

**File**: `apps/api/internal/testutil/fixtures.go`

If the `CreateTestInvestment` helper doesn't already include `RealizedGainLoss`, no change is needed (it defaults to 0, which is correct for new test investments).

### 6.2 Verification

Run `go build ./...` from `apps/api/`.

---

## Phase 7: Service Test Updates

### 7.1 Add Account Service Tests for Balance Enrichment

**File**: `apps/api/internal/services/account_service_test.go`

Add new test function `TestGetUserAccountsInvestmentBalance`:
- `enriches_investment_account_balance`: Create investment account, add investment, create security price. Call `GetUserAccounts`. Verify the investment account's balance is `quantity * latestPrice`, not 0.
- `leaves_cash_account_balance_unchanged`: Create cash account with known balance. Call `GetUserAccounts`. Verify balance is unchanged (not overwritten by enrichment logic).
- `handles_investment_account_with_no_investments`: Create investment account with no investments. Call `GetUserAccounts`. Verify balance is 0.
- `handles_investment_with_no_security_price`: Create investment account with investment, but no security price. Verify balance is 0.

Add new test function `TestGetAccountByIDInvestmentBalance`:
- `enriches_investment_account_balance`: Same pattern as above, but for single account.
- `leaves_cash_account_unchanged`: Verify cash account balance is not affected.

### 7.2 Add Investment Service Tests for `GetAllInvestments`

**File**: `apps/api/internal/services/investment_service_test.go`

Add new test function `TestGetAllInvestments`:
- `returns_investments_across_accounts`: Create 2 investment accounts, add investments to each, create security prices. Call `GetAllInvestments`. Verify all investments returned with correct `CurrentPrice`, `Security`, and `Account` preloaded.
- `returns_empty_for_no_investments`: User with no investment accounts. Verify empty response.
- `paginates_results`: Create enough investments to span multiple pages. Verify pagination works.
- `excludes_inactive_accounts`: Create an inactive investment account with investments. Verify those investments are excluded.

### 7.3 Update Investment Service Tests for Realized Gain/Loss

**File**: `apps/api/internal/services/investment_service_test.go`

Update `TestRecordSell` (or add new subtests):
- `computes_realized_gain_loss_on_sell`: Buy at price X, sell at price Y. Verify `InvestmentTransaction.RealizedGainLoss` = `(sellQty * Y - fee) - proportionalCostBasis`.
- `accumulates_realized_gain_loss_on_investment`: Perform two sells. Verify `Investment.RealizedGainLoss` is the sum of both.
- `full_sell_zeroes_quantity_and_cost_basis`: Sell entire position. Verify quantity = 0, costBasis = 0, and realized gain/loss is correct.
- `realized_gain_loss_for_losing_trade`: Buy high, sell low. Verify `RealizedGainLoss` is negative.

Update `TestGetPortfolio`:
- Verify `TotalRealizedGainLoss` is included in portfolio summary and sums correctly across investments.

### 7.4 Verification

Run `go test ./internal/services/ -v` from `apps/api/`.

---

## Phase 8: Handler Test Updates

### 8.1 Update Mock for `GetAllInvestments`

**File**: `apps/api/internal/handlers/investment_handler_test.go`

Add to `mockInvestmentService`:

```go
getAllInvestmentsFn func(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error)
```

And the corresponding method:

```go
func (m *mockInvestmentService) GetAllInvestments(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
    return m.getAllInvestmentsFn(userID, page)
}
```

### 8.2 Add Handler Tests for `GetAllInvestments`

**File**: `apps/api/internal/handlers/investment_handler_test.go`

Add `TestInvestmentHandler_GetAllInvestments`:
- `returns_200_with_investments`: Mock returns investments. Verify JSON response.
- `returns_200_empty_list`: Mock returns empty. Verify empty data array.
- `returns_500_on_service_error`: Mock returns error. Verify error response.

Add `GET /investments` to `setupInvestmentRouter`.

### 8.3 Verification

Run `go test ./internal/handlers/ -v` from `apps/api/`.

---

## Phase 9: Integration Test Updates

### 9.1 Update Integration Test Route Setup

**File**: `apps/api/tests/integration/setup_test.go`

Add `investments.GET("", investmentHandler.GetAllInvestments)` to the test router.

### 9.2 Add Integration Tests (Optional)

If `investment_flow_test.go` exists, consider adding a test step that calls `GET /api/v1/investments` after adding investments and verifies the response includes all holdings with correct prices.

### 9.3 Verification

Run `go test ./tests/integration/ -v` from `apps/api/`.

---

## Phase 10: Frontend — Type Updates

### 10.1 Add `realized_gain_loss` to Investment Type

**File**: `apps/web/src/types/models.ts`

Add to the `Investment` interface:

```typescript
realized_gain_loss: number; // cents — accumulated realized P&L from sells
```

### 10.2 Add `realized_gain_loss` to InvestmentTransaction Type

**File**: `apps/web/src/types/models.ts`

Add to the `InvestmentTransaction` interface:

```typescript
realized_gain_loss: number; // cents — realized P&L for this specific sell
```

### 10.3 Add `total_realized_gain_loss` to PortfolioSummary Type

**File**: `apps/web/src/types/models.ts`

Add to the `PortfolioSummary` interface:

```typescript
total_realized_gain_loss: number; // cents
```

### 10.4 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 11: Frontend — Add `useAllInvestments` Hook

### 11.1 Add Hook

**File**: `apps/web/src/hooks/use-investments.ts`

Add new hook:

```typescript
export function useAllInvestments(params?: PaginationParams) {
  return useQuery({
    queryKey: ["investments", "all", params],
    queryFn: () => api.get<PaginatedResponse<Investment>>("/investments", { params }),
  });
}
```

### 11.2 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 12: Frontend — Add Holdings Table to Investments Page

### 12.1 Add All Holdings Table

**File**: `apps/web/src/app/(dashboard)/investments/page.tsx`

Add a new section below the existing "Holdings by Asset Type" card. This is a new `Card` component containing:

1. **Card header**: "All Holdings" title with holding count description.
2. **Table** with columns:
   - Symbol (mono font, bold)
   - Name
   - Account (link to `/accounts/{account_id}`)
   - Qty (right-aligned, up to 6 decimal places)
   - Price (right-aligned, formatted currency)
   - Market Value (right-aligned, bold, formatted currency — computed as `Math.round(inv.quantity * inv.current_price)`)
   - Gain/Loss (right-aligned, colored green/red — `marketValue - inv.cost_basis`)
3. **Clickable rows**: Each row navigates to `/investments/{inv.id}` via `router.push`.
4. **Pagination**: Standard prev/next with page info, consistent with other tables in the app.

Uses the `useAllInvestments` hook. Show skeleton loading state. Show empty state message if no investments.

Import `useRouter` from `next/navigation` for row click navigation.

### 12.2 Verification

Run `npm run build` from `apps/web/`.

---

## Phase 13: Frontend — Improve Investment Detail Page for Closed/Active Positions

### 13.1 Add "Position Closed" Treatment for Fully Liquidated Investments

**File**: `apps/web/src/app/(dashboard)/investments/[id]/page.tsx`

When `investment.quantity === 0`:

1. Show a "Position Closed" badge next to the symbol in the header (use `Badge` component with `destructive` or `secondary` variant).
2. Replace the 6 stat cards with a layout appropriate for closed positions:
   - **Realized Gain/Loss**: Show `investment.realized_gain_loss` as the primary stat, with green/red coloring.
   - **Total Invested** (Cost Basis): Show the total cost basis from the original investment (this will be 0 after full sell due to proportional reduction, so compute from `realized_gain_loss + 0` or show as informational from transactions).
   - **Current Price**: Still show the latest security price (useful context).
   - **Account**: Link to account.
3. Disable or hide the "Sell" and "Split" buttons (can't sell/split with 0 quantity). Keep "Buy" enabled (user may want to re-open the position). "Dividend" may still be relevant for pending dividends.

### 13.2 Show Realized Gain/Loss for Active Positions with Partial Sells

**File**: `apps/web/src/app/(dashboard)/investments/[id]/page.tsx`

When `investment.quantity > 0` and `investment.realized_gain_loss !== 0`:

1. Add a 7th stat card (or modify the existing Gain/Loss card) to show both:
   - **Unrealized Gain/Loss**: Current `marketValue - costBasis` (existing calculation)
   - **Realized Gain/Loss**: `investment.realized_gain_loss`
2. This gives users a complete picture of their P&L for this investment.

### 13.3 Show Realized Gain/Loss in Transaction History

**File**: `apps/web/src/app/(dashboard)/investments/[id]/page.tsx`

For sell transactions in the transaction history table, show the `realized_gain_loss` value:
- Add a new column "Realized P&L" (or show it inline with the Total for sell rows).
- Only display for `type === "sell"` rows.
- Color green for positive, red for negative.

### 13.4 Verification

Run `npm run build && npm run lint` from `apps/web/`.

---

## Phase 14: Final Verification

### 14.1 Full Backend Check

Run `./scripts/check.sh` from `apps/api/`. This runs: `go build` → `go vet` → `golangci-lint` → `go test` → `go test -race`. All must pass.

### 14.2 Full Frontend Check

Run `npm run build && npm run lint` from `apps/web/`. Must be clean.

### 14.3 Verify Checklist

- [ ] Investment account cards on dashboard show computed market value (not 0)
- [ ] Accounts list page shows computed market value for investment accounts
- [ ] Account detail page header shows computed market value for investment accounts
- [ ] `CalculateInvestmentBalance` dead code removed from Account model
- [ ] `GET /api/v1/investments` endpoint returns all investments across accounts with pagination
- [ ] `GET /api/v1/investments` responses include `Security`, `Account`, and `CurrentPrice`
- [ ] Investments page has a clickable holdings table
- [ ] Clicking a holding row navigates to `/investments/{id}`
- [ ] `investments` table has `realized_gain_loss` column (migration applied)
- [ ] `investment_transactions` table has `realized_gain_loss` column (migration applied)
- [ ] `RecordSell` computes and stores `realized_gain_loss` on both transaction and investment
- [ ] `GetPortfolio` includes `total_realized_gain_loss` in response
- [ ] Frontend `Investment` type includes `realized_gain_loss`
- [ ] Frontend `InvestmentTransaction` type includes `realized_gain_loss`
- [ ] Frontend `PortfolioSummary` type includes `total_realized_gain_loss`
- [ ] Fully closed positions (quantity = 0) show "Position Closed" badge and realized P&L
- [ ] Active positions with partial sells show both unrealized and realized P&L
- [ ] Sell transactions in history show realized P&L
- [ ] All backend tests pass including race detection
- [ ] All frontend builds and lints pass
- [ ] No `//nolint` directives without justification

---

## API Changes

### New Endpoint

#### GET /api/v1/investments

Returns a paginated list of all investments across all active investment accounts for the authenticated user.

**Query Parameters**:
- `page` (int, optional, default 1): Page number
- `page_size` (int, optional, default 20): Items per page

**Response** (200):
```json
{
  "data": [
    {
      "id": 1,
      "account_id": 5,
      "security_id": 12,
      "quantity": 100.5,
      "cost_basis": 1505000,
      "realized_gain_loss": -142100,
      "current_price": 12500,
      "wallet_address": "",
      "security": {
        "id": 12,
        "symbol": "AAPL",
        "name": "Apple Inc.",
        "asset_type": "stock"
      },
      "account": {
        "id": 5,
        "name": "Brokerage Account",
        "type": "investment"
      }
    }
  ],
  "page": 1,
  "page_size": 20,
  "total_items": 15,
  "total_pages": 1
}
```

### Modified Responses

#### GET /api/v1/investments/:id

The `investment` object now includes `realized_gain_loss`:

```json
{
  "investment": {
    "id": 1,
    "quantity": 0,
    "cost_basis": 0,
    "realized_gain_loss": -1402100,
    "current_price": 369,
    ...
  }
}
```

#### GET /api/v1/investments/portfolio

The portfolio summary now includes `total_realized_gain_loss`:

```json
{
  "portfolio": {
    "total_value": 5000000,
    "total_cost_basis": 4500000,
    "total_gain_loss": 500000,
    "gain_loss_pct": 11.11,
    "total_realized_gain_loss": -1402100,
    "holdings_by_type": { ... }
  }
}
```

#### GET /api/v1/accounts and GET /api/v1/accounts/:id

Investment account `balance` field now contains the computed market value instead of `0`:

**Before**:
```json
{
  "id": 5,
  "type": "investment",
  "balance": 0,
  ...
}
```

**After**:
```json
{
  "id": 5,
  "type": "investment",
  "balance": 5000000,
  ...
}
```

#### POST /api/v1/investments/:id/sell

The returned transaction now includes `realized_gain_loss`:

```json
{
  "transaction": {
    "id": 42,
    "type": "sell",
    "quantity": 100,
    "price_per_unit": 15000,
    "total_amount": 1500000,
    "realized_gain_loss": 495000,
    ...
  }
}
```

---

## Files Changed

### New Files

```
apps/api/
├── migrations/
│   ├── 000016_add_realized_gain_loss.up.sql
│   └── 000016_add_realized_gain_loss.down.sql
```

### Modified Files — Backend

```
apps/api/
├── internal/
│   ├── models/
│   │   ├── account.go                              # Remove CalculateInvestmentBalance
│   │   └── investment.go                           # Add RealizedGainLoss to Investment and InvestmentTransaction
│   ├── services/
│   │   ├── interfaces.go                           # Add GetAllInvestments to interface; add TotalRealizedGainLoss to PortfolioSummary
│   │   ├── account_service.go                      # Enrich investment account balances in GetUserAccounts and GetAccountByID
│   │   ├── account_service_test.go                 # Add balance enrichment tests
│   │   ├── investment_service.go                   # Add GetAllInvestments; update RecordSell for realized P&L; update GetPortfolio
│   │   └── investment_service_test.go              # Add GetAllInvestments tests; add realized P&L tests
│   └── handlers/
│       ├── investment_handler.go                   # Add GetAllInvestments handler
│       └── investment_handler_test.go              # Add mock method and handler tests for GetAllInvestments
├── cmd/
│   └── api/main.go                                 # Register GET /api/v1/investments route
└── tests/
    └── integration/
        └── setup_test.go                           # Register route in test setup
```

### Modified Files — Frontend

```
apps/web/
├── src/
│   ├── types/
│   │   └── models.ts                               # Add realized_gain_loss to Investment, InvestmentTransaction, PortfolioSummary
│   ├── hooks/
│   │   └── use-investments.ts                      # Add useAllInvestments hook
│   └── app/
│       └── (dashboard)/
│           ├── investments/page.tsx                 # Add holdings table with clickable rows
│           └── investments/[id]/page.tsx            # Closed position UX + realized P&L display
```

### Unchanged Files — Frontend (no changes needed)

```
apps/web/
├── src/
│   └── app/
│       └── (dashboard)/
│           ├── page.tsx                             # Dashboard — AccountCard reads account.balance, now correct from API
│           ├── accounts/page.tsx                    # Accounts list — reads account.balance, now correct from API
│           └── accounts/[id]/page.tsx               # Account detail — reads account.balance, now correct from API
```

---

## Implementation Order

```
Phase 1: Database Migration
  1.1  Migration 000016: add realized_gain_loss columns
  1.2  Verification (go build)

Phase 2: Model Changes
  2.1  Add RealizedGainLoss to Investment model
  2.2  Add RealizedGainLoss to InvestmentTransaction model
  2.3  Remove CalculateInvestmentBalance from Account model
  2.4  Verification (go build)

Phase 3: Backend — Enrich Investment Account Balances
  3.1  Modify GetUserAccounts to compute investment account balances
  3.2  Modify GetAccountByID to compute investment account balance
  3.3  Verification (go build, make check-fast)

Phase 4: Backend — Add GET /api/v1/investments Endpoint
  4.1  Add GetAllInvestments to investment service
  4.2  Add GetAllInvestments to InvestmentServicer interface
  4.3  Add GetAllInvestments handler
  4.4  Register route in main.go
  4.5  Register route in integration test setup
  4.6  Verification (go build, make check-fast)

Phase 5: Backend — Realized Gain/Loss in RecordSell
  5.1  Modify RecordSell to compute and store realized gain/loss
  5.2  Add TotalRealizedGainLoss to PortfolioSummary
  5.3  Update GetPortfolio to include realized gain/loss
  5.4  Verification (go build, make check-fast)

Phase 6: Test Fixture Updates
  6.1  Update CreateTestInvestment if needed
  6.2  Verification (go build)

Phase 7: Service Test Updates
  7.1  Add account service tests for balance enrichment
  7.2  Add investment service tests for GetAllInvestments
  7.3  Update investment service tests for realized gain/loss
  7.4  Verification (go test services)

Phase 8: Handler Test Updates
  8.1  Update mock for GetAllInvestments
  8.2  Add handler tests for GetAllInvestments
  8.3  Verification (go test handlers)

Phase 9: Integration Test Updates
  9.1  Update integration test route setup
  9.2  Add integration tests (optional)
  9.3  Verification (go test integration)

Phase 10: Frontend — Type Updates
  10.1  Add realized_gain_loss to Investment type
  10.2  Add realized_gain_loss to InvestmentTransaction type
  10.3  Add total_realized_gain_loss to PortfolioSummary type
  10.4  Verification (npm run build)

Phase 11: Frontend — Add useAllInvestments Hook
  11.1  Add hook
  11.2  Verification (npm run build)

Phase 12: Frontend — Add Holdings Table to Investments Page
  12.1  Add all holdings table
  12.2  Verification (npm run build)

Phase 13: Frontend — Improve Investment Detail Page
  13.1  Add "Position Closed" treatment
  13.2  Show realized gain/loss for active positions with partial sells
  13.3  Show realized gain/loss in transaction history
  13.4  Verification (npm run build && npm run lint)

Phase 14: Final Verification
  14.1  Full backend check (./scripts/check.sh)
  14.2  Full frontend check (npm run build && npm run lint)
  14.3  Verify checklist
```

## Verification

**Backend** — after each code change:
```bash
cd apps/api && go build ./...
```
After completing each phase:
```bash
cd apps/api && ./scripts/check.sh
```

**Frontend** — after each change:
```bash
cd apps/web && npm run build && npm run lint
```

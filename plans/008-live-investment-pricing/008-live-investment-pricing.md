# Live Investment Pricing: Remove Stale `current_price` from Investments

## Context

The Kuberan backend (Plans 001-007 complete) has a fully functional investment tracking system with securities, price history via a pipeline, portfolio summaries, and a frontend for managing holdings. However, there is a fundamental design flaw in how investment prices are handled:

1. **Stale `current_price` on investments**: The `investments` table has a `current_price` column that is set to the `purchase_price` at creation time and only changes via a manual `PUT /api/v1/investments/:id/price` call (which has no UI). This means investment valuations are always stale — showing the original purchase price rather than the actual market price.

2. **Disconnected price systems**: The `security_prices` table (populated by the pipeline via `POST /api/v1/pipeline/securities/prices`) contains actual market prices, but **nothing propagates these to `investments.current_price`**. The two systems are completely independent.

3. **Misleading display**: The frontend shows `Current Price`, `Market Value`, and `Gain/Loss` all derived from the stale `current_price`. Users see `Gain/Loss = $0.00` because the "current" price never changes from the purchase price.

4. **Redundant `last_updated` field**: The `last_updated` column on investments exists solely to track when `current_price` was last manually set. It serves no purpose once `current_price` is derived from `security_prices`.

5. **Unused `UpdateInvestmentPrice` endpoint**: The `PUT /api/v1/investments/:id/price` endpoint and its associated handler, service method, and frontend hook exist but have no UI. They are made obsolete by this change.

This plan removes the `current_price` and `last_updated` columns from the `investments` table entirely. Instead, when investment data is served via the API, the latest price is looked up from `security_prices` at query time. The API response shape is unchanged — `current_price` becomes a computed field (`gorm:"-"`) populated before the response is sent. The frontend requires no structural changes.

**Out of scope**: Adding a UI for manually recording security prices (could be a future plan), real-time price streaming, or frontend chart changes.

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Remove `current_price` DB column | Backend + Migration | New migration to drop column |
| Remove `last_updated` DB column | Backend + Migration | Same migration |
| Live price lookup helper | Backend | New `getLatestPrices()` function |
| Refactor investment read paths | Backend | 4 service methods updated to populate prices from `security_prices` |
| Refactor portfolio snapshot | Backend | `computeSnapshot` uses live prices |
| Remove `UpdateInvestmentPrice` | Backend | Remove endpoint, handler, service method, interface, route |
| Frontend type cleanup | Frontend | Remove `last_updated`, `UpdatePriceRequest`, `useUpdateInvestmentPrice` |
| Frontend display unchanged | Frontend | `current_price` still in API response, pages need no changes |

## Technology & Patterns

This plan follows all patterns established in Plans 001-007:
- Backend: 3-layer architecture (Handler -> Service -> Model), interface-based services, AppError types, table-driven tests with in-memory SQLite
- Monetary values as int64 cents
- Pagination via `PageRequest`/`PageResponse[T]`

Key technical decisions:
- **`gorm:"-"` computed field**: `CurrentPrice` stays on the Go struct as a non-DB field. This preserves the API response shape while removing the DB column.
- **Batch price lookup**: A helper function `getLatestPrices(db, securityIDs)` batch-fetches the latest price per security in one query. Each read path collects security IDs, calls the helper, and populates `CurrentPrice` on each investment.
- **Fallback to zero**: If no `security_prices` entry exists for a security, `CurrentPrice` defaults to `0`. This is correct — if no market price has been recorded, the investment has no known market value.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Price source | Latest `security_prices` entry per security | Single source of truth. Prices are always fresh when the pipeline runs. |
| Lookup strategy | Query-time join, not write-time sync | Always shows the freshest price. No stale cache to worry about. Slight query overhead is negligible for typical portfolio sizes. |
| `CurrentPrice` field | Keep as `gorm:"-"` (computed, not DB-backed) | API response shape is unchanged. Frontend code needs zero changes to how it reads `current_price`. |
| `last_updated` field | Remove entirely | Only existed to track manual price updates. No longer relevant. |
| `UpdateInvestmentPrice` endpoint | Remove entirely | Obsolete — prices come from `security_prices` via the pipeline. Manual price entry should go through `POST /pipeline/securities/prices`. |
| `CalculateInvestmentBalance` method | Leave as-is | Defined on Account model but never called from anywhere. It references `CurrentPrice` which still exists as a struct field, so it compiles. Dead code can be cleaned up separately. |
| SQLite test compatibility | Use subquery pattern for latest price | Compatible with both PostgreSQL and SQLite. No DB-specific functions. |
| Zero-price fallback | Default `CurrentPrice` to 0 | If pipeline hasn't recorded prices yet, investment has no known market value. Frontend handles `formatCurrency(0)` = `MYR 0.00`. |

---

## Phase 1: Database Migration

### 1.1 Migration 000015: `drop_investment_current_price`

**New file**: `apps/api/migrations/000015_drop_investment_current_price.up.sql`

```sql
ALTER TABLE investments DROP COLUMN IF EXISTS current_price;
ALTER TABLE investments DROP COLUMN IF EXISTS last_updated;
```

**New file**: `apps/api/migrations/000015_drop_investment_current_price.down.sql`

```sql
ALTER TABLE investments ADD COLUMN current_price BIGINT DEFAULT 0;
ALTER TABLE investments ADD COLUMN last_updated TIMESTAMPTZ;
```

**Notes**:
- `IF EXISTS` for safety (idempotent).
- Down migration re-adds columns with safe defaults for reversibility. Data cannot be restored — this is a one-way data migration for the column values, but the columns themselves are reversible.

### 1.2 Verification

Run `go build ./...` from `apps/api/` to confirm migration file additions don't affect compilation.

---

## Phase 2: Model Changes

### 2.1 Modify Investment Model

**File**: `apps/api/internal/models/investment.go`

Change `CurrentPrice` from a DB-backed field to a computed field:

```go
// Before:
CurrentPrice  int64     `gorm:"type:bigint" json:"current_price"`
LastUpdated   time.Time `json:"last_updated"`

// After:
CurrentPrice  int64     `gorm:"-" json:"current_price"` // Populated at query time from security_prices
```

Remove `LastUpdated` field entirely.

### 2.2 Verification

Run `go build ./...` from `apps/api/` — expect compilation failures in services that reference `LastUpdated` or write `current_price` to the DB. These are fixed in Phase 3.

---

## Phase 3: Service Changes

### 3.1 Add Price Lookup Helper

**File**: `apps/api/internal/services/investment_service.go`

Add a new unexported helper function:

```go
// getLatestPrices fetches the most recent price for each security ID from security_prices.
// Returns a map of security_id -> price (int64 cents). Securities with no price entries
// are not included in the map.
func getLatestPrices(db *gorm.DB, securityIDs []uint) (map[uint]int64, error)
```

Implementation approach (SQLite-compatible):
1. If `securityIDs` is empty, return an empty map.
2. Query: for each security_id, get the row with the maximum `recorded_at` from `security_prices`.
3. Use a subquery: `SELECT sp.security_id, sp.price FROM security_prices sp INNER JOIN (SELECT security_id, MAX(recorded_at) AS max_recorded FROM security_prices WHERE security_id IN (?) GROUP BY security_id) latest ON sp.security_id = latest.security_id AND sp.recorded_at = latest.max_recorded`
4. Build and return the `map[uint]int64`.

### 3.2 Refactor `AddInvestment`

**File**: `apps/api/internal/services/investment_service.go`

Changes:
- Remove `CurrentPrice: purchasePrice` from the `Investment` struct literal.
- Remove `LastUpdated: time.Now()` from the `Investment` struct literal.
- After creating the investment and its initial buy transaction, look up the latest security price to populate `investment.CurrentPrice` for the response. If no price exists, leave it at `0`.

### 3.3 Refactor `GetInvestmentByID`

**File**: `apps/api/internal/services/investment_service.go`

After fetching the investment (with Preload("Security")), call `getLatestPrices` with the investment's `SecurityID` and set `investment.CurrentPrice`.

### 3.4 Refactor `GetAccountInvestments`

**File**: `apps/api/internal/services/investment_service.go`

After fetching the paginated list of investments:
1. Collect all distinct `SecurityID` values.
2. Call `getLatestPrices` with the batch.
3. Iterate investments and set `CurrentPrice` from the map.

### 3.5 Refactor `GetPortfolio`

**File**: `apps/api/internal/services/investment_service.go`

After fetching all investments across accounts:
1. Collect all distinct `SecurityID` values.
2. Call `getLatestPrices` with the batch.
3. Use the looked-up price (not `inv.CurrentPrice` which would be 0) for the value computation: `value := int64(inv.Quantity * float64(prices[inv.SecurityID]))`.

### 3.6 Remove `UpdateInvestmentPrice`

**File**: `apps/api/internal/services/investment_service.go`

Remove the entire `UpdateInvestmentPrice` method.

**File**: `apps/api/internal/services/interfaces.go`

Remove `UpdateInvestmentPrice` from the `InvestmentServicer` interface.

### 3.7 Refactor Portfolio Snapshot Service

**File**: `apps/api/internal/services/portfolio_snapshot_service.go`

In `computeSnapshot`, after fetching investments for a user:
1. Collect all distinct `SecurityID` values.
2. Call `getLatestPrices` with the batch.
3. Use looked-up prices for investment value computation instead of `investments[i].CurrentPrice`.
4. Update the comment on line 76 to reflect the new approach.

### 3.8 Verification

Run `go build ./...` from `apps/api/` — should compile. Handlers may still fail if they reference `UpdatePriceRequest` or `UpdatePrice`.

---

## Phase 4: Handler & Route Changes

### 4.1 Remove `UpdatePriceRequest` and `UpdatePrice` Handler

**File**: `apps/api/internal/handlers/investment_handler.go`

- Remove the `UpdatePriceRequest` struct.
- Remove the entire `UpdatePrice` handler method.
- Remove the Swagger annotations for the `UpdatePrice` endpoint.

### 4.2 Remove Route Registration

**File**: `apps/api/cmd/api/main.go`

Remove: `investments.PUT("/:id/price", investmentHandler.UpdatePrice)`

### 4.3 Verification

Run `go build ./...` from `apps/api/` — should compile. Run `make check-fast` (build + vet + lint).

---

## Phase 5: Test Fixture Updates

### 5.1 Update `CreateTestInvestment`

**File**: `apps/api/internal/testutil/fixtures.go`

Remove `CurrentPrice: 10000` and `LastUpdated: time.Now()` from the `CreateTestInvestment` helper.

### 5.2 Add `CreateTestSecurityPrice` Helper

**File**: `apps/api/internal/testutil/fixtures.go`

Add a new fixture function:

```go
// CreateTestSecurityPrice creates a security price record for testing.
func CreateTestSecurityPrice(t *testing.T, db *gorm.DB, securityID uint, price int64, recordedAt time.Time) *models.SecurityPrice
```

This allows tests to set up price data for securities so that `getLatestPrices` returns meaningful values.

### 5.3 Verification

Run `go build ./...` from `apps/api/`.

---

## Phase 6: Service Test Updates

### 6.1 Update Investment Service Tests

**File**: `apps/api/internal/services/investment_service_test.go`

- **Remove `TestUpdateInvestmentPrice`** entirely.
- **Update `TestAddInvestment`**: Remove assertion on `inv.CurrentPrice == 15000`. Instead, assert that `CurrentPrice` is `0` (no security price exists yet) or create a security price and verify it's picked up.
- **Update `TestGetPortfolio`**: Instead of setting `CurrentPrice` directly on investment fixtures, create `SecurityPrice` records via `CreateTestSecurityPrice`. Verify portfolio computation uses those prices.
- **Add `TestGetLatestPrices`** (if helper is exported for testing, or test indirectly via `GetInvestmentByID`):
  - `returns_latest_price`: Create multiple prices at different timestamps for same security. Verify the most recent is returned.
  - `returns_empty_for_no_prices`: Security has no price records. Verify map doesn't include it.
  - `batch_multiple_securities`: Create prices for 3 securities. Verify all returned in one call.
- **Update `TestGetInvestmentByID`**: After creating an investment, also create a security price. Verify `investment.CurrentPrice` matches the latest security price.
- **Update `TestGetAccountInvestments`**: Same pattern — create security prices and verify they appear on the returned investments.

### 6.2 Update Portfolio Snapshot Service Tests

**File**: `apps/api/internal/services/portfolio_snapshot_service_test.go`

- Remove `CurrentPrice` from all investment fixture literals.
- Add `CreateTestSecurityPrice` calls to set up prices before computing snapshots.
- Update assertions: `investment_value_computed_correctly` should verify value is computed from `security_prices`, not from a stale `CurrentPrice` column.
- Test the zero-price fallback: investment with no security price record should contribute `0` to investment value.

### 6.3 Verification

Run `go test ./internal/services/ -v` from `apps/api/`.

---

## Phase 7: Handler Test Updates

### 7.1 Update Investment Handler Tests

**File**: `apps/api/internal/handlers/investment_handler_test.go`

- Remove `updateInvestmentPriceFn` from `mockInvestmentService` struct.
- Remove `UpdateInvestmentPrice` method from mock.
- Remove `PUT /investments/:id/price` from `setupInvestmentRouter`.
- Remove the entire `TestInvestmentHandler_UpdatePrice` test function.
- Remove `CurrentPrice: price` from mock return values in `TestInvestmentHandler_AddInvestment` (the handler doesn't set `CurrentPrice` anymore — the service does).
- Verify `var _ services.InvestmentServicer = (*mockInvestmentService)(nil)` still compiles.

### 7.2 Verification

Run `go test ./internal/handlers/ -v` from `apps/api/`.

---

## Phase 8: Integration Test Updates

### 8.1 Update Investment Flow Integration Tests

**File**: `apps/api/tests/integration/investment_flow_test.go`

- Remove all `PUT /investments/:id/price` request steps.
- Remove assertions on `current_price` that expect manually-set values.
- Where portfolio value verification is needed, first record security prices via `POST /pipeline/securities/prices`, then verify portfolio values reflect those prices.
- Verify the update-price endpoint returns 404 (route no longer exists).

### 8.2 Verification

Run `go test ./tests/integration/ -v` from `apps/api/`.

---

## Phase 9: Frontend Cleanup

### 9.1 Update Investment Type

**File**: `apps/web/src/types/models.ts`

- Remove `last_updated: string` from the `Investment` interface.
- Keep `current_price: number` — it's still in the API response, just computed now.

### 9.2 Remove `UpdatePriceRequest`

**File**: `apps/web/src/types/api.ts`

Remove the `UpdatePriceRequest` interface.

### 9.3 Remove `useUpdateInvestmentPrice` Hook

**File**: `apps/web/src/hooks/use-investments.ts`

- Remove the `useUpdateInvestmentPrice` hook function.
- Remove the `UpdatePriceRequest` import from the import statement.

### 9.4 Verification

- Frontend pages (`investments/[id]/page.tsx`, `accounts/[id]/page.tsx`) need **no changes** — they read `investment.current_price` from the API response which is still present.
- Run `npm run build` from `apps/web/` — must compile.
- Run `npm run lint` from `apps/web/` — must be clean.

---

## Phase 10: Final Verification

### 10.1 Full Backend Check

Run `./scripts/check.sh` from `apps/api/`. This runs: `go build` -> `go vet` -> `golangci-lint` -> `go test` -> `go test -race`. All must pass.

### 10.2 Full Frontend Check

Run `npm run build && npm run lint` from `apps/web/`. Must be clean.

### 10.3 Swagger Regeneration

Run `swag init -g cmd/api/main.go -d . --output internal/docs --parseDependency` from `apps/api/` to regenerate Swagger docs. Verify:
- `PUT /investments/:id/price` endpoint is **removed**
- `current_price` still appears in the Investment schema (as a response field)
- `last_updated` no longer appears in the Investment schema

### 10.4 Verify Checklist

- [ ] `investments` table no longer has `current_price` or `last_updated` columns (migration applied)
- [ ] `Investment` Go struct has `CurrentPrice` as `gorm:"-"` (computed, not persisted)
- [ ] `Investment` Go struct no longer has `LastUpdated` field
- [ ] API response for investments still includes `current_price` (populated from `security_prices`)
- [ ] `GetInvestmentByID` returns investment with live price from `security_prices`
- [ ] `GetAccountInvestments` returns investments with live prices
- [ ] `GetPortfolio` computes value using live prices from `security_prices`
- [ ] `computeSnapshot` computes investment value using live prices
- [ ] `PUT /investments/:id/price` route no longer exists
- [ ] `UpdatePriceRequest` struct removed from handler
- [ ] `UpdateInvestmentPrice` removed from service interface and implementation
- [ ] Frontend `Investment` type no longer has `last_updated`
- [ ] Frontend `UpdatePriceRequest` type removed
- [ ] Frontend `useUpdateInvestmentPrice` hook removed
- [ ] Frontend pages display correct live prices (no code changes needed — same `current_price` field)
- [ ] Investments with no security price records show `CurrentPrice = 0`
- [ ] All backend tests pass including race detection
- [ ] All frontend builds and lints pass
- [ ] No `//nolint` directives without justification

---

## API Changes

### Removed Endpoint

#### ~~PUT /api/v1/investments/:id/price~~ (REMOVED)

This endpoint is removed. Security prices should be recorded via the pipeline endpoint `POST /api/v1/pipeline/securities/prices`.

### Modified Response

#### GET /api/v1/investments/:id

The `investment` object in the response still includes `current_price`, but the value now comes from the latest `security_prices` entry for the investment's security, rather than a stale column on the investments table.

**Before** (stale):
```json
{
  "investment": {
    "current_price": 806846,
    "last_updated": "2025-01-01T00:00:00Z",
    ...
  }
}
```

**After** (live):
```json
{
  "investment": {
    "current_price": 7866,
    ...
  }
}
```

The `last_updated` field is removed from the response. `current_price` is now the latest price from `security_prices`, or `0` if no price has been recorded.

---

## Files Changed

### New Files

```
apps/api/
├── migrations/
│   ├── 000015_drop_investment_current_price.up.sql
│   └── 000015_drop_investment_current_price.down.sql
```

### Modified Files — Backend

```
apps/api/
├── internal/
│   ├── models/
│   │   └── investment.go                          # CurrentPrice -> gorm:"-", remove LastUpdated
│   ├── services/
│   │   ├── interfaces.go                          # Remove UpdateInvestmentPrice from InvestmentServicer
│   │   ├── investment_service.go                  # Add getLatestPrices helper, refactor 4 read methods, remove UpdateInvestmentPrice
│   │   ├── investment_service_test.go             # Remove TestUpdateInvestmentPrice, add price lookup tests, update fixtures
│   │   ├── portfolio_snapshot_service.go          # Refactor computeSnapshot to use live prices
│   │   └── portfolio_snapshot_service_test.go     # Update fixtures to use SecurityPrice records
│   ├── handlers/
│   │   ├── investment_handler.go                  # Remove UpdatePriceRequest, remove UpdatePrice handler
│   │   └── investment_handler_test.go             # Remove mock/tests for UpdatePrice
│   └── testutil/
│       └── fixtures.go                            # Remove CurrentPrice/LastUpdated, add CreateTestSecurityPrice
├── cmd/
│   └── api/main.go                                # Remove PUT /:id/price route
└── tests/
    └── integration/
        └── investment_flow_test.go                # Remove update-price steps, use pipeline prices
```

### Modified Files — Frontend

```
apps/web/
├── src/
│   ├── types/
│   │   ├── models.ts                              # Remove last_updated from Investment
│   │   └── api.ts                                 # Remove UpdatePriceRequest
│   └── hooks/
│       └── use-investments.ts                     # Remove useUpdateInvestmentPrice hook
```

### Unchanged Files — Frontend (no changes needed)

```
apps/web/
├── src/
│   └── app/
│       └── (dashboard)/
│           ├── investments/[id]/page.tsx           # Still reads investment.current_price — now live
│           └── accounts/[id]/page.tsx              # Still reads inv.current_price — now live
```

---

## Implementation Order

```
Phase 1: Database Migration
  1.1  Migration 000015: drop current_price and last_updated columns
  1.2  Verification (go build)

Phase 2: Model Changes
  2.1  Investment model: CurrentPrice -> gorm:"-", remove LastUpdated
  2.2  Verification (go build — expect service failures)

Phase 3: Service Changes
  3.1  Add getLatestPrices helper function
  3.2  Refactor AddInvestment
  3.3  Refactor GetInvestmentByID
  3.4  Refactor GetAccountInvestments
  3.5  Refactor GetPortfolio
  3.6  Remove UpdateInvestmentPrice (service + interface)
  3.7  Refactor portfolio snapshot computeSnapshot
  3.8  Verification (go build)

Phase 4: Handler & Route Changes
  4.1  Remove UpdatePriceRequest and UpdatePrice handler
  4.2  Remove PUT /:id/price route from main.go
  4.3  Verification (make check-fast)

Phase 5: Test Fixture Updates
  5.1  Update CreateTestInvestment (remove CurrentPrice, LastUpdated)
  5.2  Add CreateTestSecurityPrice helper
  5.3  Verification (go build)

Phase 6: Service Test Updates
  6.1  Update investment service tests
  6.2  Update portfolio snapshot service tests
  6.3  Verification (go test services)

Phase 7: Handler Test Updates
  7.1  Update investment handler tests
  7.2  Verification (go test handlers)

Phase 8: Integration Test Updates
  8.1  Update investment flow integration tests
  8.2  Verification (go test integration)

Phase 9: Frontend Cleanup
  9.1  Remove last_updated from Investment type
  9.2  Remove UpdatePriceRequest type
  9.3  Remove useUpdateInvestmentPrice hook
  9.4  Verification (npm run build && npm run lint)

Phase 10: Final Verification
  10.1  Full backend check (./scripts/check.sh)
  10.2  Full frontend check (npm run build && npm run lint)
  10.3  Swagger regeneration
  10.4  Verify checklist
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

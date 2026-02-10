# Securities, Price History & Portfolio Snapshots

## Context

The Kuberan backend (Plans 001–005 complete) has full investment tracking: users can create investment accounts, add holdings (stocks, ETFs, bonds, crypto, REITs), record buy/sell/dividend/split transactions, update prices, and view portfolio summaries. However, the current design has fundamental limitations:

1. **No normalized securities**: Each `Investment` row stores `symbol`, `name`, `asset_type`, `currency`, `exchange`, and asset-specific fields directly. If two users hold AAPL, the data is duplicated. There is no shared entity for a financial instrument.

2. **No price history**: The `current_price` field on each `Investment` is overwritten on every update. There is no record of historical prices, making it impossible to chart investment performance over time.

3. **No portfolio snapshots**: Portfolio value is computed dynamically from current holdings × current prices. There is no way to see how total net worth (cash + investments - debt) has changed over time.

4. **No pipeline integration point**: A future Temporal pipeline will fetch prices hourly and push them to the API. The current endpoints are user-scoped (JWT auth) and don't support service-to-service authentication.

This plan introduces three new database entities (securities, security prices, portfolio snapshots), refactors the Investment model to reference a normalized Security, adds API key authentication for pipeline endpoints, and provides all the storage and API endpoints needed for historical price tracking and net worth visualization.

**Out of scope**: The Temporal pipeline itself, external price feed integrations, frontend charts, and data retention/downsampling policies.

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Securities table & CRUD | Backend | New model, service, handler, routes, migrations, tests |
| Security price history | Backend | New model, bulk insert endpoint, paginated query endpoint, tests |
| Portfolio snapshots | Backend | New model, compute-all endpoint, paginated query endpoint, tests |
| Investment model refactor | Backend | Remove denormalized security fields, add `security_id` FK |
| Pipeline API key auth | Backend | New middleware, config update, route group separation |
| Existing test updates | Backend | Update all investment tests for new model structure |

## Technology & Patterns

This plan follows all patterns established in Plans 001–005:
- Backend: 3-layer architecture (Handler → Service → Model), interface-based services, AppError types, Swagger annotations, table-driven tests with in-memory SQLite
- Monetary values as int64 cents
- Pagination via `PageRequest`/`PageResponse[T]`
- Audit logging for sensitive operations

New additions:
- **API key middleware**: `PipelineAuthMiddleware` using `X-API-Key` header with `crypto/subtle.ConstantTimeCompare` for timing-attack resistance
- **Bulk insert endpoint**: `POST /api/v1/pipeline/securities/prices` accepts an array of price entries
- **Compute endpoint**: `POST /api/v1/pipeline/snapshots/compute` triggers server-side portfolio snapshot computation for all active users
- **Time-series models**: `SecurityPrice` and `PortfolioSnapshot` do not embed the GORM `Base` model (no soft deletes for immutable time-series data)

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Security normalization | Full — move all security-level fields to `securities` table, add `security_id` FK on Investment | No production data exists yet. Clean schema now avoids a painful data migration later. |
| Price history granularity | Per-security, hourly, stored indefinitely | Per-security prices enable per-holding performance analysis and what-if scenarios. Portfolio snapshots can be derived but are also stored as a denormalized optimization. |
| Portfolio snapshot computation | API-side (`/pipeline/snapshots/compute`) | Keeps business logic (balance calculation) in one place. Pipeline's job is orchestration + price fetching, not duplicating financial logic. |
| Snapshot breakdown | `total_net_worth`, `cash_balance`, `investment_value`, `debt_balance` | Enables separate charting of cash growth vs investment growth vs net worth. Debt tracked separately for liability visibility. |
| Pipeline authentication | API key via `X-API-Key` header | Simple, appropriate for self-hosted app. Uses constant-time comparison to prevent timing attacks. Returns 503 if key not configured (not 500). |
| Pipeline route prefix | `/api/v1/pipeline/` | Clear separation in code, logs, and monitoring between user-facing and pipeline endpoints. |
| `current_price` on Investment | Keep as denormalized cache | Avoids changing `GetPortfolio()` query pattern. Pipeline updates both `security_prices` (append) and `investments.current_price` (overwrite). |
| `wallet_address` on Investment | Keep on Investment, not Security | Wallet addresses are user/holding-specific, not properties of the security itself. |
| Security unique constraint | `(symbol, exchange)` | Same ticker can exist on different exchanges. For crypto (empty exchange), symbol alone provides uniqueness. |
| Time-series models | No `Base` embed, no soft deletes | Immutable append-only data. Soft deletes add overhead with no benefit for time-series rows. |
| `AssetType` enum location | Move to `models/security.go` | The enum describes securities, not holdings. Constants and validation remain identical. |
| SQLite test compatibility | Standard SQL only, no db-specific date functions | Same approach as Plans 003–005. Per-iteration queries where needed. |

---

## Phase 1: Database Migrations

### 1.1 Migration 000011: `create_securities`

**New file**: `apps/api/migrations/000011_create_securities.up.sql`

```sql
CREATE TABLE IF NOT EXISTS securities (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    symbol VARCHAR(20) NOT NULL,
    name VARCHAR(200) NOT NULL,
    asset_type VARCHAR(20) NOT NULL,
    currency VARCHAR(10) NOT NULL DEFAULT 'USD',
    exchange VARCHAR(50) DEFAULT '',
    maturity_date TIMESTAMPTZ,
    yield_to_maturity DOUBLE PRECISION DEFAULT 0,
    coupon_rate DOUBLE PRECISION DEFAULT 0,
    network VARCHAR(50) DEFAULT '',
    property_type VARCHAR(50) DEFAULT '',

    CONSTRAINT uq_securities_symbol_exchange UNIQUE (symbol, exchange)
);
CREATE INDEX idx_securities_deleted_at ON securities(deleted_at);
CREATE INDEX idx_securities_symbol ON securities(symbol);
CREATE INDEX idx_securities_asset_type ON securities(asset_type);
```

**New file**: `apps/api/migrations/000011_create_securities.down.sql`

```sql
DROP TABLE IF EXISTS securities;
```

**Notes**:
- `wallet_address` is NOT included — it's a user/holding-level field that stays on Investment.
- `(symbol, exchange)` unique constraint handles the case where the same symbol exists on different exchanges. For crypto where exchange is empty string, the symbol alone provides uniqueness.
- Bond-specific fields (`maturity_date`, `yield_to_maturity`, `coupon_rate`), crypto-specific (`network`), and REIT-specific (`property_type`) are security-level properties.
- Soft deletes (`deleted_at`) supported via GORM `Base` model — securities can be deactivated without losing referential integrity.

### 1.2 Migration 000012: `create_security_prices`

**New file**: `apps/api/migrations/000012_create_security_prices.up.sql`

```sql
CREATE TABLE IF NOT EXISTS security_prices (
    id BIGSERIAL PRIMARY KEY,
    security_id BIGINT NOT NULL REFERENCES securities(id),
    price BIGINT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT uq_security_prices_security_recorded UNIQUE (security_id, recorded_at)
);
CREATE INDEX idx_security_prices_security_id ON security_prices(security_id);
CREATE INDEX idx_security_prices_recorded_at ON security_prices(recorded_at);
```

**New file**: `apps/api/migrations/000012_create_security_prices.down.sql`

```sql
DROP TABLE IF EXISTS security_prices;
```

**Notes**:
- No `created_at`, `updated_at`, or `deleted_at` — these are immutable time-series rows, not GORM-managed entities.
- The unique constraint on `(security_id, recorded_at)` prevents duplicate price entries for the same security at the same timestamp.
- `price` is `BIGINT` (int64 cents), consistent with all monetary values in the system.

### 1.3 Migration 000013: `create_portfolio_snapshots`

**New file**: `apps/api/migrations/000013_create_portfolio_snapshots.up.sql`

```sql
CREATE TABLE IF NOT EXISTS portfolio_snapshots (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    recorded_at TIMESTAMPTZ NOT NULL,
    total_net_worth BIGINT NOT NULL DEFAULT 0,
    cash_balance BIGINT NOT NULL DEFAULT 0,
    investment_value BIGINT NOT NULL DEFAULT 0,
    debt_balance BIGINT NOT NULL DEFAULT 0,

    CONSTRAINT uq_portfolio_snapshots_user_recorded UNIQUE (user_id, recorded_at)
);
CREATE INDEX idx_portfolio_snapshots_user_id ON portfolio_snapshots(user_id);
CREATE INDEX idx_portfolio_snapshots_recorded_at ON portfolio_snapshots(recorded_at);
```

**New file**: `apps/api/migrations/000013_create_portfolio_snapshots.down.sql`

```sql
DROP TABLE IF EXISTS portfolio_snapshots;
```

**Notes**:
- Same time-series pattern as `security_prices` — no soft deletes, immutable rows.
- Breakdown: `cash_balance` (sum of cash accounts), `investment_value` (sum of quantity × current_price across all investments), `debt_balance` (sum of debt + credit card balances), `total_net_worth` = cash + investment - debt.
- `(user_id, recorded_at)` unique constraint prevents duplicate snapshots for the same user at the same timestamp.

### 1.4 Migration 000014: `refactor_investments_add_security_id`

**New file**: `apps/api/migrations/000014_refactor_investments_add_security_id.up.sql`

```sql
ALTER TABLE investments ADD COLUMN security_id BIGINT REFERENCES securities(id);
CREATE INDEX idx_investments_security_id ON investments(security_id);

ALTER TABLE investments DROP COLUMN symbol;
ALTER TABLE investments DROP COLUMN name;
ALTER TABLE investments DROP COLUMN asset_type;
ALTER TABLE investments DROP COLUMN currency;
ALTER TABLE investments DROP COLUMN exchange;
ALTER TABLE investments DROP COLUMN maturity_date;
ALTER TABLE investments DROP COLUMN yield_to_maturity;
ALTER TABLE investments DROP COLUMN coupon_rate;
ALTER TABLE investments DROP COLUMN network;
ALTER TABLE investments DROP COLUMN property_type;
```

**New file**: `apps/api/migrations/000014_refactor_investments_add_security_id.down.sql`

```sql
ALTER TABLE investments ADD COLUMN symbol VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE investments ADD COLUMN name VARCHAR(200) NOT NULL DEFAULT '';
ALTER TABLE investments ADD COLUMN asset_type VARCHAR(20) NOT NULL DEFAULT '';
ALTER TABLE investments ADD COLUMN currency VARCHAR(10) NOT NULL DEFAULT 'USD';
ALTER TABLE investments ADD COLUMN exchange VARCHAR(50) DEFAULT '';
ALTER TABLE investments ADD COLUMN maturity_date TIMESTAMPTZ;
ALTER TABLE investments ADD COLUMN yield_to_maturity DOUBLE PRECISION DEFAULT 0;
ALTER TABLE investments ADD COLUMN coupon_rate DOUBLE PRECISION DEFAULT 0;
ALTER TABLE investments ADD COLUMN network VARCHAR(50) DEFAULT '';
ALTER TABLE investments ADD COLUMN property_type VARCHAR(50) DEFAULT '';

DROP INDEX IF EXISTS idx_investments_security_id;
ALTER TABLE investments DROP COLUMN security_id;
```

**Notes**:
- Since there is no production data, no data migration logic is needed — column drops are safe.
- `security_id` is nullable initially to allow the column to be added before data exists. In practice, all new investments will require a `security_id`.
- The down migration re-adds the dropped columns with safe defaults for reversibility.

### 1.5 Verification

Run `go build ./...` from `apps/api/` to confirm compilation is unaffected by migration file additions.

---

## Phase 2: New Models

### 2.1 Create Security Model

**New file**: `apps/api/internal/models/security.go`

Move `AssetType` type and constants (`AssetTypeStock`, `AssetTypeETF`, `AssetTypeBond`, `AssetTypeCrypto`, `AssetTypeREIT`) from `investment.go` to this file.

Define the `Security` struct:

```go
type Security struct {
    Base
    Symbol          string     `gorm:"not null;uniqueIndex:uq_securities_symbol_exchange" json:"symbol"`
    Name            string     `gorm:"not null" json:"name"`
    AssetType       AssetType  `gorm:"not null" json:"asset_type"`
    Currency        string     `gorm:"not null;default:'USD'" json:"currency"`
    Exchange        string     `gorm:"uniqueIndex:uq_securities_symbol_exchange" json:"exchange,omitempty"`
    MaturityDate    *time.Time `json:"maturity_date,omitempty"`
    YieldToMaturity float64    `json:"yield_to_maturity,omitempty"`
    CouponRate      float64    `json:"coupon_rate,omitempty"`
    Network         string     `json:"network,omitempty"`
    PropertyType    string     `json:"property_type,omitempty"`
}
```

### 2.2 Create SecurityPrice Model

**New file**: `apps/api/internal/models/security_price.go`

```go
type SecurityPrice struct {
    ID         uint      `gorm:"primarykey" json:"id"`
    SecurityID uint      `gorm:"not null" json:"security_id"`
    Price      int64     `gorm:"type:bigint;not null" json:"price"`
    RecordedAt time.Time `gorm:"not null" json:"recorded_at"`
    Security   Security  `gorm:"foreignKey:SecurityID" json:"security,omitempty"`
}
```

No `Base` embed — immutable time-series data, no soft deletes.

### 2.3 Create PortfolioSnapshot Model

**New file**: `apps/api/internal/models/portfolio_snapshot.go`

```go
type PortfolioSnapshot struct {
    ID              uint      `gorm:"primarykey" json:"id"`
    UserID          uint      `gorm:"not null" json:"user_id"`
    RecordedAt      time.Time `gorm:"not null" json:"recorded_at"`
    TotalNetWorth   int64     `gorm:"type:bigint;not null" json:"total_net_worth"`
    CashBalance     int64     `gorm:"type:bigint;not null" json:"cash_balance"`
    InvestmentValue int64     `gorm:"type:bigint;not null" json:"investment_value"`
    DebtBalance     int64     `gorm:"type:bigint;not null" json:"debt_balance"`
}
```

No `Base` embed — immutable time-series data, no soft deletes.

### 2.4 Modify Investment Model

**File**: `apps/api/internal/models/investment.go`

Remove these fields: `Symbol`, `Name`, `AssetType`, `Currency`, `Exchange`, `MaturityDate`, `YieldToMaturity`, `CouponRate`, `Network`, `PropertyType`.

Add:
```go
SecurityID uint     `gorm:"not null" json:"security_id"`
Security   Security `gorm:"foreignKey:SecurityID" json:"security"`
```

Keep: `AccountID`, `Quantity`, `CostBasis`, `CurrentPrice`, `LastUpdated`, `WalletAddress`, `Account`, `Transactions`.

Since `AssetType` moved to `security.go`, ensure `investment.go` still imports it (same package, no import needed).

### 2.5 Update Test Database Setup

**File**: `apps/api/internal/testutil/database.go`

Add `models.Security{}`, `models.SecurityPrice{}`, and `models.PortfolioSnapshot{}` to the `allModels` slice used by `SetupTestDB`'s `AutoMigrate` call.

### 2.6 Verification

Run `go build ./...` from `apps/api/`. This WILL fail because services reference removed Investment fields — that is expected and fixed in Phase 4.

---

## Phase 3: Configuration & Middleware

### 3.1 Add Pipeline API Key to Config

**File**: `apps/api/internal/config/config.go`

Add `PipelineAPIKey string` field to the `Config` struct. Load from `PIPELINE_API_KEY` environment variable.

No production enforcement — the key is optional. If not set, pipeline endpoints return 503 (handled by middleware).

### 3.2 Create Pipeline Auth Middleware

**New file**: `apps/api/internal/middleware/pipeline_auth.go`

```go
// PipelineAuthMiddleware creates a Gin middleware that validates the X-API-Key
// header against the configured pipeline API key.
func PipelineAuthMiddleware(apiKey string) gin.HandlerFunc {
    return func(c *gin.Context) {
        if apiKey == "" {
            c.AbortWithStatusJSON(http.StatusServiceUnavailable,
                gin.H{"error": gin.H{"code": "PIPELINE_NOT_CONFIGURED", "message": "Pipeline endpoints are not configured"}})
            return
        }
        key := c.GetHeader("X-API-Key")
        if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
            c.AbortWithStatusJSON(http.StatusUnauthorized,
                gin.H{"error": gin.H{"code": "INVALID_API_KEY", "message": "Invalid or missing API key"}})
            return
        }
        c.Next()
    }
}
```

Uses `crypto/subtle.ConstantTimeCompare` to prevent timing attacks. Returns 503 if the API key is not configured (rather than silently accepting or crashing). Returns 401 for invalid/missing keys.

### 3.3 Add Error Sentinels

**File**: `apps/api/internal/errors/errors.go`

Add new sentinel errors:

```go
// Security errors
var ErrSecurityNotFound = &AppError{Code: "SECURITY_NOT_FOUND", Message: "Security not found", StatusCode: http.StatusNotFound}
var ErrDuplicateSecurity = &AppError{Code: "DUPLICATE_SECURITY", Message: "A security with this symbol and exchange already exists", StatusCode: http.StatusConflict}
```

### 3.4 Middleware Tests

**New file**: `apps/api/internal/middleware/pipeline_auth_test.go`

Test cases:
- `valid_api_key`: Request with correct `X-API-Key` header → 200 (handler reached)
- `invalid_api_key`: Request with wrong key → 401
- `missing_api_key`: Request without `X-API-Key` header → 401
- `empty_configured_key`: Middleware created with empty key → 503
- `constant_time_comparison`: Verify the middleware doesn't short-circuit on partial matches (behavioral, not timing — just verify wrong keys always return 401)

### 3.5 Verification

Run `go build ./...` from `apps/api/`. May still fail due to service compilation errors — that is expected until Phase 4 completes.

---

## Phase 4: Service Interfaces & Implementations

### 4.1 Add SecurityServicer Interface

**File**: `apps/api/internal/services/interfaces.go`

```go
// SecurityPriceInput represents a single price entry for bulk recording.
type SecurityPriceInput struct {
    SecurityID uint      `json:"security_id"`
    Price      int64     `json:"price"`
    RecordedAt time.Time `json:"recorded_at"`
}

// SecurityServicer defines the interface for security-related operations.
type SecurityServicer interface {
    CreateSecurity(symbol, name string, assetType models.AssetType, currency, exchange string,
        extraFields map[string]interface{}) (*models.Security, error)
    GetSecurityByID(id uint) (*models.Security, error)
    ListSecurities(page pagination.PageRequest) (*pagination.PageResponse[models.Security], error)
    RecordPrices(prices []SecurityPriceInput) (int, error) // returns count of prices recorded
    GetPriceHistory(securityID uint, from, to time.Time,
        page pagination.PageRequest) (*pagination.PageResponse[models.SecurityPrice], error)
}
```

### 4.2 Add PortfolioSnapshotServicer Interface

**File**: `apps/api/internal/services/interfaces.go`

```go
// PortfolioSnapshotServicer defines the interface for portfolio snapshot operations.
type PortfolioSnapshotServicer interface {
    ComputeAndRecordSnapshots(recordedAt time.Time) (int, error) // returns count of snapshots created
    GetSnapshots(userID uint, from, to time.Time,
        page pagination.PageRequest) (*pagination.PageResponse[models.PortfolioSnapshot], error)
}
```

### 4.3 Modify InvestmentServicer Interface

**File**: `apps/api/internal/services/interfaces.go`

Change `AddInvestment` signature from:

```go
AddInvestment(userID, accountID uint, symbol, name string, assetType models.AssetType,
    quantity float64, purchasePrice int64, currency string,
    extraFields map[string]interface{}) (*models.Investment, error)
```

To:

```go
AddInvestment(userID, accountID, securityID uint, quantity float64,
    purchasePrice int64, walletAddress string) (*models.Investment, error)
```

### 4.4 Implement Security Service

**New file**: `apps/api/internal/services/security_service.go`

Struct: `securityService` with `db *gorm.DB`.

**CreateSecurity**:
1. Validate symbol and name are non-empty.
2. Build `Security` model with required fields + apply `extraFields` (same pattern as investment `applyExtraFields` — maps keys like `maturity_date`, `yield_to_maturity`, `coupon_rate`, `network`, `property_type`).
3. Attempt DB insert. If unique constraint violation (duplicate `symbol` + `exchange`), return `ErrDuplicateSecurity`.
4. Return created security.

**GetSecurityByID**:
1. Query by ID (no user-scoping — securities are shared).
2. Return `ErrSecurityNotFound` if not found.

**ListSecurities**:
1. Paginated query, ordered by `symbol ASC`.
2. No user-scoping — securities are shared.

**RecordPrices**:
1. Validate input array is non-empty.
2. Iterate and insert each `SecurityPrice` record. Use `db.Clauses(clause.OnConflict{DoNothing: true})` or simply skip on duplicate `(security_id, recorded_at)` — upsert semantics to handle idempotent pipeline retries.
3. Return count of records actually inserted.

**GetPriceHistory**:
1. Query `SecurityPrice` where `security_id = ? AND recorded_at BETWEEN ? AND ?`.
2. Paginated, ordered by `recorded_at DESC`.
3. Return paginated response.

### 4.5 Implement Portfolio Snapshot Service

**New file**: `apps/api/internal/services/portfolio_snapshot_service.go`

Struct: `portfolioSnapshotService` with `db *gorm.DB`.

**ComputeAndRecordSnapshots**:
1. Query all distinct active users: `SELECT DISTINCT user_id FROM accounts WHERE is_active = true AND deleted_at IS NULL`.
2. For each user:
   a. **Cash balance**: `SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ? AND type IN ('cash') AND is_active = true AND deleted_at IS NULL`
   b. **Investment value**: Query all investments for the user's active investment accounts, sum `quantity * current_price` for each holding.
   c. **Debt balance**: `SELECT COALESCE(SUM(balance), 0) FROM accounts WHERE user_id = ? AND type IN ('debt', 'credit_card') AND is_active = true AND deleted_at IS NULL`
   d. **Total net worth**: `cash_balance + investment_value - debt_balance`
   e. Insert `PortfolioSnapshot` record with `recorded_at` timestamp. Use upsert on `(user_id, recorded_at)` to handle idempotent retries.
3. Return count of snapshots created.

**GetSnapshots**:
1. Query `PortfolioSnapshot` where `user_id = ? AND recorded_at BETWEEN ? AND ?`.
2. Paginated, ordered by `recorded_at DESC`.
3. Return paginated response.

### 4.6 Modify Investment Service

**File**: `apps/api/internal/services/investment_service.go`

**AddInvestment** — refactor:
1. Remove parameters: `symbol`, `name`, `assetType`, `currency`, `extraFields`.
2. Add parameters: `securityID`, `walletAddress`.
3. Verify the security exists by querying the DB (or accept a `SecurityServicer` dependency — simpler to just query directly since it's in the same DB).
4. Create `Investment` with `SecurityID`, `AccountID`, `Quantity`, `CostBasis`, `CurrentPrice = purchasePrice`, `WalletAddress`.
5. Create initial `InvestmentTransaction` (type = buy) — `PricePerUnit` comes from `purchasePrice`.
6. Remove `applyExtraFields` call and the `applyExtraFields` helper function entirely.

**GetPortfolio** — refactor:
1. When loading investments, add `Preload("Security")` to access `AssetType` on the security.
2. Change `inv.AssetType` references to `inv.Security.AssetType`.

**GetInvestmentByID**, **GetAccountInvestments** — add `Preload("Security")` so responses include the nested security object.

### 4.7 Verification

Run `go build ./...` from `apps/api/`. This should now compile. If handlers reference removed fields, those are fixed in Phase 5.

---

## Phase 5: Handlers & Route Registration

### 5.1 Create Security Handler

**New file**: `apps/api/internal/handlers/security_handler.go`

Struct: `SecurityHandler` with `securityService services.SecurityServicer` and `auditService services.AuditServicer`.

**Request DTOs**:

```go
type CreateSecurityRequest struct {
    Symbol          string           `json:"symbol" binding:"required,min=1,max=20"`
    Name            string           `json:"name" binding:"required,min=1,max=200"`
    AssetType       models.AssetType `json:"asset_type" binding:"required,asset_type"`
    Currency        string           `json:"currency" binding:"omitempty,iso4217"`
    Exchange        string           `json:"exchange,omitempty"`
    MaturityDate    *time.Time       `json:"maturity_date,omitempty"`
    YieldToMaturity float64          `json:"yield_to_maturity,omitempty"`
    CouponRate      float64          `json:"coupon_rate,omitempty"`
    Network         string           `json:"network,omitempty"`
    PropertyType    string           `json:"property_type,omitempty"`
}

type RecordPricesRequest struct {
    Prices []RecordPriceEntry `json:"prices" binding:"required,min=1,dive"`
}

type RecordPriceEntry struct {
    SecurityID uint      `json:"security_id" binding:"required"`
    Price      int64     `json:"price" binding:"required,gt=0"`
    RecordedAt time.Time `json:"recorded_at" binding:"required"`
}
```

**Handler methods**:
- `CreateSecurity`: Bind request → call service → audit log `CREATE_SECURITY` → return 201 with security.
- `ListSecurities`: Parse pagination → call service → return 200 with paginated response.
- `GetSecurity`: Parse path ID → call service → return 200 with security.
- `RecordPrices`: Bind request → map entries to `SecurityPriceInput` → call service → return 200 with count.
- `GetPriceHistory`: Parse path ID + `from_date`/`to_date` query params + pagination → call service → return 200 with paginated response.

All methods include full Swagger annotations.

### 5.2 Create Portfolio Snapshot Handler

**New file**: `apps/api/internal/handlers/portfolio_snapshot_handler.go`

Struct: `PortfolioSnapshotHandler` with `snapshotService services.PortfolioSnapshotServicer`.

**Handler methods**:
- `ComputeSnapshots`: Call `snapshotService.ComputeAndRecordSnapshots(time.Now())` → return 200 with `{"snapshots_created": N, "recorded_at": "..."}`.
- `GetSnapshots`: Extract `userID` from JWT → parse `from_date`/`to_date` query params + pagination → call service → return 200 with paginated response.

All methods include full Swagger annotations.

### 5.3 Modify Investment Handler

**File**: `apps/api/internal/handlers/investment_handler.go`

Update `AddInvestmentRequest`:

```go
type AddInvestmentRequest struct {
    AccountID     uint    `json:"account_id" binding:"required"`
    SecurityID    uint    `json:"security_id" binding:"required"`
    Quantity      float64 `json:"quantity" binding:"required,gt=0"`
    PurchasePrice int64   `json:"purchase_price" binding:"required,gt=0"`
    WalletAddress string  `json:"wallet_address,omitempty"`
}
```

Remove: `Symbol`, `Name`, `AssetType`, `Currency`, `Exchange`, `MaturityDate`, `YieldToMaturity`, `CouponRate`, `Network`, `PropertyType`.

Update `AddInvestment` handler: call service with `securityID` and `walletAddress` instead of the old field set. Remove `buildExtraFields` helper function. Update audit log to reference `security_id` instead of `symbol`.

### 5.4 Register Routes

**File**: `apps/api/cmd/api/main.go`

Instantiate new services and handlers:

```go
securityService := services.NewSecurityService(db)
portfolioSnapshotService := services.NewPortfolioSnapshotService(db)
securityHandler := handlers.NewSecurityHandler(securityService, auditService)
portfolioSnapshotHandler := handlers.NewPortfolioSnapshotHandler(portfolioSnapshotService)
```

Register user-facing routes (JWT auth):

```go
securities := protected.Group("/securities")
securities.GET("", securityHandler.ListSecurities)
securities.GET("/:id", securityHandler.GetSecurity)
securities.GET("/:id/prices", securityHandler.GetPriceHistory)

portfolio := protected.Group("/portfolio")
portfolio.GET("/snapshots", portfolioSnapshotHandler.GetSnapshots)
```

Register pipeline routes (API key auth):

```go
pipeline := v1.Group("/pipeline").Use(middleware.PipelineAuthMiddleware(cfg.PipelineAPIKey))
pipeline.POST("/securities", securityHandler.CreateSecurity)
pipeline.POST("/securities/prices", securityHandler.RecordPrices)
pipeline.POST("/snapshots/compute", portfolioSnapshotHandler.ComputeSnapshots)
```

### 5.5 Verification

Run `go build ./...` from `apps/api/`. This should compile successfully. Run `make check-fast` (build + vet + lint).

---

## Phase 6: Test Utilities Update

### 6.1 Update Test Database

**File**: `apps/api/internal/testutil/database.go`

Add `models.Security{}`, `models.SecurityPrice{}`, and `models.PortfolioSnapshot{}` to the `allModels` slice (if not done in Phase 2.5).

### 6.2 Add Test Fixtures

**File**: `apps/api/internal/testutil/fixtures.go`

Add fixture functions:

**`CreateTestSecurity(t, db)`**: Creates a security with symbol `SEC{n}`, name `Test Security {n}`, asset type `stock`, currency `USD`, exchange `NYSE`. Returns `*models.Security`.

**`CreateTestSecurityWithParams(t, db, symbol, name, assetType, exchange)`**: Creates a security with specified parameters for more control. Returns `*models.Security`.

**Update `CreateTestInvestment(t, db, accountID)`**: Change to `CreateTestInvestment(t, db, accountID, securityID uint)`. Remove direct `Symbol`, `Name`, `AssetType`, `Currency` fields. Set `SecurityID = securityID` instead.

### 6.3 Verification

Run `go build ./...` and `go test ./internal/testutil/...` from `apps/api/`.

---

## Phase 7: New Service Tests

### 7.1 Security Service Tests

**New file**: `apps/api/internal/services/security_service_test.go`

Test functions with table-driven subtests:

**`TestCreateSecurity`**:
- `valid`: Creates security with all fields, verifies returned struct.
- `with_extra_fields`: Creates bond with maturity_date, yield_to_maturity, coupon_rate.
- `duplicate_symbol_exchange`: Creates two securities with same symbol+exchange → `ErrDuplicateSecurity`.
- `same_symbol_different_exchange`: Creates same symbol on NYSE and NASDAQ → both succeed.
- `empty_symbol`: → `ErrInvalidInput`.
- `empty_name`: → `ErrInvalidInput`.

**`TestGetSecurityByID`**:
- `found`: Verify returned security matches.
- `not_found`: → `ErrSecurityNotFound`.

**`TestListSecurities`**:
- `returns_paginated`: Create 5 securities, request page_size=2 → verify 2 items, total_items=5, total_pages=3.
- `ordered_by_symbol`: Verify alphabetical ordering.

**`TestRecordPrices`**:
- `valid_bulk_insert`: Record 3 prices for 2 securities, verify all inserted.
- `idempotent_retry`: Record same price twice (same security_id + recorded_at) → no error, no duplicate.
- `empty_input`: → error or 0 count.

**`TestGetPriceHistory`**:
- `returns_paginated`: Record 5 prices, request page_size=2 → verify pagination.
- `filters_by_date_range`: Record prices across dates, query subset → verify filtering.
- `filters_by_security`: Record prices for 2 securities, query for one → verify only that security's prices returned.
- `ordered_by_recorded_at_desc`: Verify most recent first.

### 7.2 Portfolio Snapshot Service Tests

**New file**: `apps/api/internal/services/portfolio_snapshot_service_test.go`

**`TestComputeAndRecordSnapshots`**:
- `creates_snapshots_for_all_users`: Create 2 users with accounts, call compute → verify 2 snapshots created with correct values.
- `cash_balance_computed_correctly`: Create user with 2 cash accounts (balances 10000, 5000) → verify snapshot cash_balance = 15000.
- `investment_value_computed_correctly`: Create user with investment account, 2 investments (10 shares @ $100, 5 shares @ $200) → verify investment_value = 200000.
- `debt_balance_computed_correctly`: Create user with debt account (balance 5000) and credit card (balance 2000) → verify debt_balance = 7000.
- `net_worth_computed_correctly`: Cash 15000 + investments 200000 - debt 7000 → verify total_net_worth = 208000.
- `excludes_inactive_accounts`: Create inactive account → verify its balance is not included.
- `idempotent_retry`: Call compute twice with same `recorded_at` → verify no duplicate snapshots.

**`TestGetSnapshots`**:
- `returns_paginated`: Create 5 snapshots, request page_size=2 → verify pagination.
- `filters_by_date_range`: Create snapshots across dates, query subset → verify filtering.
- `user_isolation`: Create snapshots for 2 users, query for one → verify only that user's snapshots.
- `ordered_by_recorded_at_desc`: Verify most recent first.

### 7.3 Verification

Run `go test ./internal/services/ -v` from `apps/api/`.

---

## Phase 8: Modified Service Tests

### 8.1 Update Investment Service Tests

**File**: `apps/api/internal/services/investment_service_test.go`

All tests that call `AddInvestment` must be updated:

1. Create a `Security` first using `testutil.CreateTestSecurity(t, db)`.
2. Pass `security.ID` to `AddInvestment` instead of `symbol`, `name`, `assetType`, `currency`, `extraFields`.
3. Update assertions: instead of checking `investment.Symbol`, check `investment.SecurityID == security.ID`.
4. Tests for `GetPortfolio` that check `HoldingsByType` must account for the fact that `AssetType` now comes from the `Security` relationship — ensure investments are loaded with `Preload("Security")`.

Specific tests to update:
- `TestAddInvestment`: Update fixture calls and assertions.
- `TestGetInvestmentByID`: Verify `Security` is preloaded.
- `TestGetAccountInvestments`: Verify `Security` is preloaded.
- `TestGetPortfolio`: Update to create securities, verify `HoldingsByType` still groups correctly.
- All record tests (buy, sell, dividend, split): Update `AddInvestment` calls.

### 8.2 Verification

Run `go test ./internal/services/ -v` from `apps/api/`.

---

## Phase 9: New Handler Tests

### 9.1 Security Handler Tests

**New file**: `apps/api/internal/handlers/security_handler_test.go`

Mock: `mockSecurityService` with function fields for each interface method.

**`TestSecurityHandler_CreateSecurity`**:
- `returns_201_on_success`: Valid request → 201 with security JSON.
- `returns_400_missing_symbol`: → 400.
- `returns_400_invalid_asset_type`: → 400.
- `returns_409_duplicate`: Service returns `ErrDuplicateSecurity` → 409.

**`TestSecurityHandler_ListSecurities`**:
- `returns_200_with_data`: → 200 with paginated response.
- `returns_200_with_pagination_params`: Verify page/page_size passed to service.

**`TestSecurityHandler_GetSecurity`**:
- `returns_200_on_success`: → 200 with security.
- `returns_404_not_found`: → 404.
- `returns_400_invalid_id`: → 400.

**`TestSecurityHandler_RecordPrices`**:
- `returns_200_on_success`: Valid bulk input → 200 with count.
- `returns_400_empty_prices`: → 400.
- `returns_400_invalid_price`: Price = 0 → 400.

**`TestSecurityHandler_GetPriceHistory`**:
- `returns_200_with_data`: → 200 with paginated response.
- `returns_400_missing_from_date`: → 400.
- `returns_400_missing_to_date`: → 400.
- `returns_404_invalid_security`: → 404.

### 9.2 Portfolio Snapshot Handler Tests

**New file**: `apps/api/internal/handlers/portfolio_snapshot_handler_test.go`

Mock: `mockPortfolioSnapshotService` with function fields.

**`TestPortfolioSnapshotHandler_ComputeSnapshots`**:
- `returns_200_on_success`: → 200 with `{"snapshots_created": 3, "recorded_at": "..."}`.
- `returns_500_on_service_error`: Service returns error → 500.

**`TestPortfolioSnapshotHandler_GetSnapshots`**:
- `returns_200_with_data`: → 200 with paginated response.
- `returns_400_missing_from_date`: → 400.
- `returns_400_missing_to_date`: → 400.
- `returns_200_empty_data`: → 200 with empty data.

### 9.3 Verification

Run `go test ./internal/handlers/ -v` from `apps/api/`.

---

## Phase 10: Modified Handler Tests

### 10.1 Update Investment Handler Tests

**File**: `apps/api/internal/handlers/investment_handler_test.go`

Update `mockInvestmentService.addInvestmentFn` signature to match the new `AddInvestment` interface (accepts `securityID`, `walletAddress` instead of `symbol`, `name`, `assetType`, `currency`, `extraFields`).

Update test request bodies:
- `TestInvestmentHandler_AddInvestment`: Change from `{"symbol": "AAPL", "name": "Apple", "asset_type": "stock", ...}` to `{"account_id": 1, "security_id": 5, "quantity": 10, "purchase_price": 15000}`.
- Update validation tests: `returns_400_missing_symbol` → `returns_400_missing_security_id`.

### 10.2 Pipeline Auth Handler Tests

**New file**: `apps/api/internal/middleware/pipeline_auth_test.go` (if not created in Phase 3.4)

Verify pipeline routes return 401 without key, 200 with key, 503 when key not configured.

### 10.3 Verification

Run `go test ./internal/handlers/ -v` from `apps/api/`.

---

## Phase 11: Integration Tests

### 11.1 Security Flow Integration Test

**New file**: `apps/api/tests/integration/security_flow_test.go`

Update `setupApp` to wire security service, security handler, portfolio snapshot service, portfolio snapshot handler, and pipeline routes.

**`TestSecurityFlow_FullLifecycle`**:
1. Create security via pipeline endpoint (API key auth).
2. List securities (JWT auth) → verify 1 result.
3. Get security by ID → verify fields.
4. Record prices via pipeline endpoint (3 price entries).
5. Get price history → verify 3 entries, ordered by recorded_at DESC.

**`TestSecurityFlow_DuplicateSymbolExchange`**:
1. Create security (AAPL, NYSE).
2. Create same security again → 409.
3. Create AAPL on NASDAQ → 201 (different exchange, allowed).

### 11.2 Portfolio Snapshot Integration Test

**New file**: `apps/api/tests/integration/portfolio_snapshot_flow_test.go`

**`TestPortfolioSnapshotFlow_ComputeAndQuery`**:
1. Register user, create cash account ($5000 balance), create investment account.
2. Create security, add investment (10 shares @ $150).
3. Call compute snapshots via pipeline endpoint.
4. Get snapshots (JWT auth) → verify 1 snapshot with correct cash_balance (5000), investment_value (150000), total_net_worth (155000).

**`TestPortfolioSnapshotFlow_MultipleUsersComputed`**:
1. Register 2 users with different account balances.
2. Call compute once.
3. Query snapshots for each user → verify each has their own snapshot.

### 11.3 Update Investment Flow Integration Tests

**File**: `apps/api/tests/integration/investment_flow_test.go`

Update all tests to:
1. Create a security via pipeline endpoint (or directly via DB) before adding investments.
2. Use `security_id` in add-investment requests instead of `symbol`, `name`, `asset_type`.
3. Verify investment responses include nested `security` object.

### 11.4 Update Integration Test Setup

**File**: `apps/api/tests/integration/setup_test.go`

Add to `testApp`:
- `securityService`, `portfolioSnapshotService`
- `securityHandler`, `portfolioSnapshotHandler`
- Pipeline route group with test API key
- Helper methods for pipeline requests (with API key header)

### 11.5 Verification

Run `go test ./tests/integration/ -v` from `apps/api/`.

---

## Phase 12: Final Verification

### 12.1 Full Check

Run `./scripts/check.sh` from `apps/api/`. This runs: `go build` → `go vet` → `golangci-lint` → `go test` → `go test -race`. All must pass.

### 12.2 Swagger Regeneration

Run `swag init -g cmd/api/main.go -d . --output internal/docs --parseDependency` to regenerate Swagger docs with new endpoints.

### 12.3 Verify Checklist

- [ ] No `Investment.Symbol`, `Investment.Name`, `Investment.AssetType` references remain outside of down-migration files
- [ ] All new endpoints have Swagger annotations
- [ ] All new services have interface definitions and tests
- [ ] All handler tests pass with mocks
- [ ] All integration tests pass end-to-end
- [ ] Pipeline endpoints return 503 when `PIPELINE_API_KEY` is not set
- [ ] Pipeline endpoints return 401 with wrong API key
- [ ] Pipeline endpoints work with correct API key
- [ ] Securities are shared (no user-scoping)
- [ ] Portfolio snapshots are user-scoped
- [ ] Time-series models have no soft deletes
- [ ] `GetPortfolio` still works correctly with `Security.AssetType`

---

## API Reference

### New Public Endpoints (JWT Auth)

#### GET /api/v1/securities

List all securities (paginated).

| Param | Type | Required | Description |
|---|---|---|---|
| `page` | int | No | Page number (default: 1) |
| `page_size` | int | No | Items per page (default: 20, max: 100) |

**Response**: `200`
```json
{
  "data": [
    {
      "id": 1,
      "symbol": "AAPL",
      "name": "Apple Inc.",
      "asset_type": "stock",
      "currency": "USD",
      "exchange": "NASDAQ"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total_items": 50,
  "total_pages": 3
}
```

#### GET /api/v1/securities/:id

Get a security by ID.

**Response**: `200` — Security object. `404` — `SECURITY_NOT_FOUND`.

#### GET /api/v1/securities/:id/prices

Get price history for a security (paginated).

| Param | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | Yes | Start date (RFC3339 or YYYY-MM-DD) |
| `to_date` | string | Yes | End date (RFC3339 or YYYY-MM-DD) |
| `page` | int | No | Page number (default: 1) |
| `page_size` | int | No | Items per page (default: 20, max: 100) |

**Response**: `200`
```json
{
  "data": [
    {
      "id": 1,
      "security_id": 5,
      "price": 17500,
      "recorded_at": "2026-02-09T12:00:00Z"
    }
  ],
  "page": 1,
  "page_size": 20,
  "total_items": 720,
  "total_pages": 36
}
```

#### GET /api/v1/portfolio/snapshots

Get portfolio snapshots for the authenticated user (paginated).

| Param | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | Yes | Start date (RFC3339 or YYYY-MM-DD) |
| `to_date` | string | Yes | End date (RFC3339 or YYYY-MM-DD) |
| `page` | int | No | Page number (default: 1) |
| `page_size` | int | No | Items per page (default: 20, max: 100) |

**Response**: `200`
```json
{
  "data": [
    {
      "id": 1,
      "user_id": 1,
      "recorded_at": "2026-02-09T12:00:00Z",
      "total_net_worth": 15500000,
      "cash_balance": 5000000,
      "investment_value": 11000000,
      "debt_balance": 500000
    }
  ],
  "page": 1,
  "page_size": 20,
  "total_items": 168,
  "total_pages": 9
}
```

### New Pipeline Endpoints (API Key Auth)

#### POST /api/v1/pipeline/securities

Create a new security. Requires `X-API-Key` header.

**Request**:
```json
{
  "symbol": "AAPL",
  "name": "Apple Inc.",
  "asset_type": "stock",
  "currency": "USD",
  "exchange": "NASDAQ"
}
```

**Response**: `201` — Security object. `409` — `DUPLICATE_SECURITY`. `401` — Invalid API key. `503` — Pipeline not configured.

#### POST /api/v1/pipeline/securities/prices

Bulk record prices for securities. Requires `X-API-Key` header.

**Request**:
```json
{
  "prices": [
    {"security_id": 1, "price": 17500, "recorded_at": "2026-02-09T12:00:00Z"},
    {"security_id": 2, "price": 4200, "recorded_at": "2026-02-09T12:00:00Z"}
  ]
}
```

**Response**: `200`
```json
{"prices_recorded": 2}
```

#### POST /api/v1/pipeline/snapshots/compute

Compute and store portfolio snapshots for all active users. Requires `X-API-Key` header.

**Response**: `200`
```json
{
  "snapshots_created": 5,
  "recorded_at": "2026-02-09T12:00:00Z"
}
```

### Modified Endpoints

#### POST /api/v1/investments (JWT Auth)

**New request body**:
```json
{
  "account_id": 1,
  "security_id": 5,
  "quantity": 10.0,
  "purchase_price": 15000,
  "wallet_address": "0x..."
}
```

**Old fields removed**: `symbol`, `name`, `asset_type`, `currency`, `exchange`, `maturity_date`, `yield_to_maturity`, `coupon_rate`, `network`, `property_type`.

**Response**: Investment object now includes nested `security` object.

---

## File Structure After This Plan

### New Files

```
apps/api/
├── internal/
│   ├── models/
│   │   ├── security.go                       # Security model + AssetType enum (moved from investment.go)
│   │   ├── security_price.go                 # SecurityPrice model (no Base embed)
│   │   └── portfolio_snapshot.go             # PortfolioSnapshot model (no Base embed)
│   ├── services/
│   │   ├── security_service.go               # CRUD + price recording + history
│   │   ├── security_service_test.go          # Service tests
│   │   ├── portfolio_snapshot_service.go     # Compute + query snapshots
│   │   └── portfolio_snapshot_service_test.go # Service tests
│   ├── handlers/
│   │   ├── security_handler.go               # HTTP handlers for securities + prices
│   │   ├── security_handler_test.go          # Handler tests
│   │   ├── portfolio_snapshot_handler.go     # HTTP handlers for snapshots
│   │   └── portfolio_snapshot_handler_test.go # Handler tests
│   └── middleware/
│       ├── pipeline_auth.go                  # API key middleware
│       └── pipeline_auth_test.go             # Middleware tests
├── migrations/
│   ├── 000011_create_securities.up.sql
│   ├── 000011_create_securities.down.sql
│   ├── 000012_create_security_prices.up.sql
│   ├── 000012_create_security_prices.down.sql
│   ├── 000013_create_portfolio_snapshots.up.sql
│   ├── 000013_create_portfolio_snapshots.down.sql
│   ├── 000014_refactor_investments_add_security_id.up.sql
│   └── 000014_refactor_investments_add_security_id.down.sql
└── tests/
    └── integration/
        ├── security_flow_test.go             # Security + price history e2e
        └── portfolio_snapshot_flow_test.go   # Snapshot computation e2e
```

### Modified Files

```
apps/api/
├── internal/
│   ├── config/config.go                      # Add PipelineAPIKey field
│   ├── errors/errors.go                      # Add ErrSecurityNotFound, ErrDuplicateSecurity
│   ├── models/investment.go                  # Remove security fields, add SecurityID
│   ├── services/
│   │   ├── interfaces.go                     # Add SecurityServicer, PortfolioSnapshotServicer, modify InvestmentServicer
│   │   ├── investment_service.go             # Refactor AddInvestment, GetPortfolio
│   │   └── investment_service_test.go        # Update for security references
│   ├── handlers/
│   │   ├── investment_handler.go             # Refactor AddInvestment request/response
│   │   └── investment_handler_test.go        # Update mocks and request bodies
│   └── testutil/
│       ├── database.go                       # Add new models to AutoMigrate
│       └── fixtures.go                       # Add CreateTestSecurity, update CreateTestInvestment
├── cmd/api/main.go                           # Wire new services/handlers, register pipeline routes
└── tests/
    └── integration/
        ├── setup_test.go                     # Add pipeline routes and helpers
        └── investment_flow_test.go           # Create security before investments
```

---

## Implementation Order

```
Phase 1: Database Migrations
  1.1  Migration 000011: create_securities
  1.2  Migration 000012: create_security_prices
  1.3  Migration 000013: create_portfolio_snapshots
  1.4  Migration 000014: refactor_investments_add_security_id
  1.5  Verification (go build)

Phase 2: New Models
  2.1  Create Security model (+ move AssetType enum)
  2.2  Create SecurityPrice model
  2.3  Create PortfolioSnapshot model
  2.4  Modify Investment model (remove security fields, add SecurityID)
  2.5  Update test database setup (add new models to AutoMigrate)
  2.6  Verification (go build — expect failures in services)

Phase 3: Configuration & Middleware
  3.1  Add PipelineAPIKey to config
  3.2  Create PipelineAuthMiddleware
  3.3  Add error sentinels (ErrSecurityNotFound, ErrDuplicateSecurity)
  3.4  Middleware tests
  3.5  Verification

Phase 4: Service Interfaces & Implementations
  4.1  Add SecurityServicer interface
  4.2  Add PortfolioSnapshotServicer interface
  4.3  Modify InvestmentServicer interface (AddInvestment signature)
  4.4  Implement SecurityService
  4.5  Implement PortfolioSnapshotService
  4.6  Modify InvestmentService (AddInvestment, GetPortfolio)
  4.7  Verification (go build)

Phase 5: Handlers & Route Registration
  5.1  Create SecurityHandler
  5.2  Create PortfolioSnapshotHandler
  5.3  Modify InvestmentHandler (AddInvestment request/response)
  5.4  Register routes (JWT + pipeline groups in main.go)
  5.5  Verification (make check-fast)

Phase 6: Test Utilities Update
  6.1  Update test database (add models)
  6.2  Add test fixtures (CreateTestSecurity, update CreateTestInvestment)
  6.3  Verification

Phase 7: New Service Tests
  7.1  Security service tests
  7.2  Portfolio snapshot service tests
  7.3  Verification (go test services)

Phase 8: Modified Service Tests
  8.1  Update investment service tests
  8.2  Verification (go test services)

Phase 9: New Handler Tests
  9.1  Security handler tests
  9.2  Portfolio snapshot handler tests
  9.3  Verification (go test handlers)

Phase 10: Modified Handler Tests
  10.1  Update investment handler tests
  10.2  Pipeline auth handler tests
  10.3  Verification (go test handlers)

Phase 11: Integration Tests
  11.1  Security flow integration test
  11.2  Portfolio snapshot integration test
  11.3  Update investment flow integration tests
  11.4  Update integration test setup
  11.5  Verification (go test integration)

Phase 12: Final Verification
  12.1  Full check (./scripts/check.sh)
  12.2  Swagger regeneration
  12.3  Verify checklist
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

## Environment Variables

New variable added:
```
PIPELINE_API_KEY=<any-strong-secret>   # Required for pipeline endpoints. If empty, pipeline routes return 503.
```

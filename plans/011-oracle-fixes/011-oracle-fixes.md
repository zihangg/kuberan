# Oracle Fixes: Provider Symbol Support & Yahoo v8 Migration

## Context

The price oracle (Plan 010) was recently built and deployed. Two issues were discovered during the first production runs:

### Issue 1: Yahoo Finance v7 API returns 401 Unauthorized

Yahoo Finance has restricted access to the `/v7/finance/quote` batch endpoint. All 20 Yahoo-routed securities fail with `unexpected status 401`. The v8 `/chart/{symbol}` endpoint was tested and works without authentication, but it only supports one symbol per request (no batching).

### Issue 2: BURSA (Malaysian) stocks use numeric codes on Yahoo Finance

Malaysian stocks on BURSA use numeric stock codes on Yahoo Finance (e.g., CIMB → `1023.KL`, MAYBANK → `1155.KL`), not the human-readable ticker names stored in the `symbol` column. The current `buildYahooSymbol` function simply appends `.KL` to the symbol, producing invalid tickers like `CIMB.KL` which return "No data found, symbol may be delisted". This also affects JPX (Tokyo) and potentially other exchanges that use numeric codes.

### Current State

1. **CoinGecko provider works** — crypto prices fetch successfully via the free `/simple/price` bulk endpoint.
2. **Yahoo provider is broken** — all requests return 401 on v7.
3. **v8 endpoint confirmed working** — tested manually for AAPL (200, price in `chart.result[0].meta.regularMarketPrice`), VUAA.L (200), and CIMB.KL (404 — symbol mismatch, not auth issue).
4. **Asset type normalization fixed** — a separate fix already handles the case mismatch (`Stock` → `stock`, `Cryptocurrency` → `crypto`).
5. **Lint errors fixed** — all pre-existing errcheck violations in the oracle codebase have been resolved.

### What This Plan Adds

1. **`provider_symbol` field on Security model** — an optional field that stores the provider-specific symbol (e.g., `1023.KL` for CIMB on BURSA). When set, the oracle uses this instead of computing the symbol from `symbol + exchange suffix`. This is a generic solution that works for any exchange with non-standard ticker formats.

2. **Yahoo Finance v7 → v8 migration** — switches the Yahoo provider from the broken batch v7 endpoint to the working per-symbol v8 chart endpoint, with concurrent fetching via a semaphore.

**Out of scope**: Populating `provider_symbol` for existing BURSA securities (manual data update), adding new exchanges, frontend changes (the field is API-only for now).

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Add `provider_symbol` to Security model | Backend (API) | Migration, model, handler, service, tests |
| Propagate `provider_symbol` to oracle | Oracle | Client, provider, orchestrator structs |
| Use `provider_symbol` in Yahoo provider | Oracle | `buildYahooSymbol` logic change |
| Switch Yahoo from v7 to v8 endpoint | Oracle | Rewrite fetch logic, update response parsing, add concurrency |

## Technology & Patterns

- **API changes** follow existing patterns: migration, model field, extra fields map, handler request struct
- **Oracle changes** follow existing patterns: struct field additions, provider interface unchanged
- **Concurrency**: Yahoo v8 requires one HTTP request per symbol. A buffered channel semaphore limits concurrent requests (default 10) to avoid rate limiting.
- **No new dependencies**: Both apps continue to use only the Go standard library (oracle) and existing dependencies (API).

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Symbol override field | `provider_symbol` on Security model | Generic solution — works for BURSA, JPX, or any exchange where Yahoo's symbol differs from the display symbol. No hardcoded mapping tables needed. |
| Field location | API model + DB column | The oracle fetches securities from the API. The field must be in the DB and returned by `GET /pipeline/securities`. |
| Override semantics | Use `provider_symbol` when non-empty, fall back to `symbol + exchange suffix` | Backward-compatible — existing securities that work fine today need no changes. Only securities with non-standard Yahoo tickers need the field set. |
| Yahoo v8 concurrency | Semaphore (buffered channel, cap 10) | v8 is one-symbol-per-request, so we need concurrency to avoid sequential HTTP calls for 20+ securities. Cap at 10 to respect Yahoo's rate limits. |
| Yahoo v8 query params | `interval=1d&range=1d` | Minimal data request — we only need the current price from `meta.regularMarketPrice`, not historical candles. |
| Batch logic removal | Remove `yahooBatchMax`, `fetchBatch`, `batchErrors` | v8 doesn't support batching. Replaced by per-symbol `fetchOne` with concurrent dispatch. |

---

## Phase 1: Add `provider_symbol` to API

### 1.1 Database Migration

**New files**: `apps/api/migrations/000017_add_provider_symbol.up.sql`, `000017_add_provider_symbol.down.sql`

**Up migration**:
```sql
ALTER TABLE securities ADD COLUMN provider_symbol VARCHAR(50) DEFAULT '';
```

**Down migration**:
```sql
ALTER TABLE securities DROP COLUMN provider_symbol;
```

### 1.2 Update Security Model

**File**: `apps/api/internal/models/security.go`

Add field to `Security` struct:
```go
ProviderSymbol string `gorm:"default:''" json:"provider_symbol,omitempty"`
```

Place it after `Exchange` — it's related to exchange/provider-specific symbol overrides.

### 1.3 Update Security Handler

**File**: `apps/api/internal/handlers/security_handler.go`

Add to `CreateSecurityRequest` struct:
```go
ProviderSymbol string `json:"provider_symbol,omitempty"`
```

Update `buildSecurityExtraFields` to extract `provider_symbol`:
```go
if req.ProviderSymbol != "" {
    fields["provider_symbol"] = req.ProviderSymbol
}
```

### 1.4 Update Security Service

**File**: `apps/api/internal/services/security_service.go`

Update `applySecurityExtraFields` to handle `provider_symbol`:
```go
if v, ok := fields["provider_symbol"].(string); ok {
    sec.ProviderSymbol = v
}
```

### 1.5 Update API Tests

**Handler tests** (`apps/api/internal/handlers/security_handler_test.go`):
- Add a subtest to `TestSecurityHandler_CreateSecurity` that includes `provider_symbol` in the request JSON and verifies the service receives it via `extraFields`

**Service tests** (`apps/api/internal/services/security_service_test.go`):
- Update the `with_extra_fields` subtest in `TestCreateSecurity` to include `provider_symbol` and verify it's persisted

**Integration tests** (`apps/api/tests/integration/security_flow_test.go`):
- Verify `provider_symbol` round-trips through create → get

### 1.6 Verification

Run `./scripts/check-go.sh apps/api`. All 5 steps must pass.

---

## Phase 2: Propagate `provider_symbol` to Oracle

### 2.1 Update Oracle Client Struct

**File**: `apps/oracle/internal/client/kuberan.go`

Add to `Security` struct:
```go
ProviderSymbol string `json:"provider_symbol"`
```

### 2.2 Update Oracle Provider Struct

**File**: `apps/oracle/internal/provider/provider.go`

Add to `Security` struct:
```go
ProviderSymbol string
```

### 2.3 Update Oracle Orchestrator

**File**: `apps/oracle/internal/oracle/oracle.go`

In the client→provider conversion loop, map the new field:
```go
ProviderSymbol: s.ProviderSymbol,
```

### 2.4 Update Yahoo Provider — Use `provider_symbol`

**File**: `apps/oracle/internal/provider/yahoo.go`

Modify `buildYahooSymbol` to accept a `Security` (or add `providerSymbol` parameter):
```go
func buildYahooSymbol(sec Security) string {
    if sec.ProviderSymbol != "" {
        return sec.ProviderSymbol
    }
    if suffix, ok := exchangeSuffixes[sec.Exchange]; ok {
        return sec.Symbol + suffix
    }
    return sec.Symbol
}
```

Update all call sites to pass the full security instead of just `symbol, exchange`.

### 2.5 Update Oracle Tests

**Yahoo tests** (`apps/oracle/internal/provider/yahoo_test.go`):
- Add `TestYahooProvider_FetchPrices_ProviderSymbol`: security with `ProviderSymbol: "1023.KL"` and `Exchange: "BURSA"`. Verify HTTP request uses `1023.KL` (not `CIMB.KL`).
- Update `TestYahooProvider_FetchPrices_ExchangeSuffix` to pass `Security` struct.

**Oracle orchestrator tests** (`apps/oracle/internal/oracle/oracle_test.go`):
- Add `ProviderSymbol` to at least one test case to verify it flows through the conversion.

**Client tests** (`apps/oracle/internal/client/kuberan_test.go`):
- Update `TestGetSecurities_Success` mock response to include `provider_symbol` field and verify it's parsed.

### 2.6 Verification

Run `./scripts/check-go.sh apps/oracle`. All 5 steps must pass.

---

## Phase 3: Switch Yahoo from v7 to v8 Endpoint

### 3.1 Update Yahoo Response Types

**File**: `apps/oracle/internal/provider/yahoo.go`

Replace v7 response types:
```go
// Remove:
type yahooQuoteResponse struct { ... }
type yahooQuoteResult struct { ... }

// Add:
type yahooChartResponse struct {
    Chart struct {
        Result []struct {
            Meta struct {
                Symbol             string  `json:"symbol"`
                RegularMarketPrice float64 `json:"regularMarketPrice"`
            } `json:"meta"`
        } `json:"result"`
        Error *struct {
            Code        string `json:"code"`
            Description string `json:"description"`
        } `json:"error"`
    } `json:"chart"`
}
```

### 3.2 Update Constants

**File**: `apps/oracle/internal/provider/yahoo.go`

```go
// Change:
yahooBaseURL = "https://query1.finance.yahoo.com/v7/finance/quote"

// To:
yahooBaseURL = "https://query1.finance.yahoo.com/v8/finance/chart"

// Remove:
yahooBatchMax = 50

// Add:
yahooMaxConcurrent = 10
```

### 3.3 Rewrite FetchPrices

**File**: `apps/oracle/internal/provider/yahoo.go`

Replace the batch-based `FetchPrices` with concurrent per-symbol fetching:

1. Build Yahoo ticker for each security using `buildYahooSymbol`.
2. Create a semaphore (buffered channel, capacity `yahooMaxConcurrent`).
3. Launch a goroutine per security that:
   a. Acquires semaphore slot.
   b. Calls `fetchOne(ctx, yahooTicker)` → `GET {baseURL}/{ticker}?interval=1d&range=1d`.
   c. Parses `yahooChartResponse`.
   d. Extracts `chart.result[0].meta.regularMarketPrice`.
   e. Converts to cents via `math.Round(price * 100)`.
   f. Returns `PriceResult` or `FetchError`.
   g. Releases semaphore slot.
4. Collect all results and errors via mutex-protected slices (same pattern as orchestrator).

### 3.4 Implement `fetchOne` Method

**File**: `apps/oracle/internal/provider/yahoo.go`

New method:
```go
func (p *YahooProvider) fetchOne(ctx context.Context, ticker string, now time.Time) (*PriceResult, error)
```

- Build URL: `{baseURL}/{ticker}?interval=1d&range=1d`
- Set User-Agent header.
- Parse response. If `chart.result` is nil or empty, return error.
- If `chart.error` is non-nil, return error with code and description.
- Extract `regularMarketPrice` from `chart.result[0].meta`.
- If price is 0, return error.
- Return `PriceResult` with cents conversion.

### 3.5 Remove Batch Logic

**File**: `apps/oracle/internal/provider/yahoo.go`

Remove:
- `fetchBatch` method
- `batchErrors` helper function
- `yahooBatchMax` constant
- `yahooQuoteResponse` and `yahooQuoteResult` types

### 3.6 Update Yahoo Tests

**File**: `apps/oracle/internal/provider/yahoo_test.go`

Update mock server:
- v7 mock served all symbols at one URL path with `?symbols=` query param.
- v8 mock must serve per-symbol at `/{ticker}` path. Use `r.URL.Path` to extract the ticker and return the appropriate v8 chart response.

Update response format in all tests to v8 structure:
```json
{
  "chart": {
    "result": [{
      "meta": {
        "symbol": "AAPL",
        "regularMarketPrice": 178.72
      }
    }],
    "error": null
  }
}
```

Test changes:
- `TestYahooProvider_Supports`: No changes needed (doesn't depend on endpoint).
- `TestYahooProvider_FetchPrices_Success`: Update mock to v8 format. Verify 3 results.
- `TestYahooProvider_FetchPrices_PartialFailure`: Mock returns v8 error response for one symbol, success for others. Verify partial results.
- `TestYahooProvider_FetchPrices_ExchangeSuffix`: Update mock to v8 format. Verify request path includes `SHOP.TO`.
- `TestYahooProvider_FetchPrices_BatchSplit`: **Remove** — batching no longer exists. Replace with a concurrency test.
- `TestYahooProvider_FetchPrices_Concurrent`: New test — create 15 securities. Verify all fetched successfully. Optionally verify concurrency limit via atomic counter of in-flight requests.
- `TestYahooProvider_FetchPrices_HTTPError`: Update mock. Verify FetchError for all securities.
- `TestYahooProvider_FetchPrices_ZeroPrice`: Update mock to v8 format. Verify FetchError.
- `TestYahooProvider_FetchPrices_ChartError`: New test — mock returns v8 error response `{"chart": {"result": null, "error": {"code": "Not Found", "description": "No data found"}}}`. Verify FetchError.

### 3.7 Verification

Run `./scripts/check-go.sh apps/oracle`. All 5 steps must pass.

---

## Phase 4: Final Verification

### 4.1 Full API Verification

Run `./scripts/check-go.sh apps/api`. All 5 steps must pass.

### 4.2 Full Oracle Verification

Run `./scripts/check-go.sh apps/oracle`. All 5 steps must pass.

### 4.3 Docker Build & Live Test

```bash
docker compose up -d
docker compose --profile oracle build oracle
docker compose --profile oracle run --rm oracle
```

Verify:
- Oracle successfully fetches securities from API
- CoinGecko prices are recorded (crypto securities)
- Yahoo v8 prices are recorded (stock/ETF/REIT securities with valid Yahoo symbols)
- BURSA securities with `provider_symbol` set fetch correctly
- BURSA securities without `provider_symbol` fail gracefully (FetchError, not crash)
- Portfolio snapshots are computed

### 4.4 Verification Checklist

- [ ] Migration applies cleanly: `provider_symbol` column added to `securities` table
- [ ] Migration rolls back cleanly: column dropped
- [ ] `POST /api/v1/pipeline/securities` accepts `provider_symbol` in request body
- [ ] `GET /api/v1/pipeline/securities` returns `provider_symbol` in response
- [ ] `provider_symbol` is optional — omitting it defaults to empty string
- [ ] Oracle receives `provider_symbol` from API and passes it to Yahoo provider
- [ ] Yahoo provider uses `provider_symbol` when non-empty (e.g., `1023.KL`)
- [ ] Yahoo provider falls back to `symbol + exchange suffix` when `provider_symbol` is empty
- [ ] Yahoo v8 endpoint returns prices for US stocks (AAPL, MSFT, etc.)
- [ ] Yahoo v8 endpoint returns prices for international stocks (SHOP.TO, VUAA.L, etc.)
- [ ] Yahoo v8 concurrent fetching works for 20+ symbols without rate limiting
- [ ] Yahoo v8 handles "Not Found" errors gracefully (FetchError per symbol)
- [ ] CoinGecko provider continues to work unchanged
- [ ] All API tests pass (build, vet, lint, test, test -race)
- [ ] All oracle tests pass (build, vet, lint, test, test -race)
- [ ] Docker image builds successfully
- [ ] No `//nolint` directives without justification

---

## Files Changed

### New Files

```
apps/api/migrations/
├── 000017_add_provider_symbol.up.sql
└── 000017_add_provider_symbol.down.sql
```

### Modified Files — API

```
apps/api/
├── internal/
│   ├── models/
│   │   └── security.go                    # Add ProviderSymbol field
│   ├── handlers/
│   │   ├── security_handler.go            # Add ProviderSymbol to CreateSecurityRequest, buildSecurityExtraFields
│   │   └── security_handler_test.go       # Add provider_symbol test cases
│   └── services/
│       ├── security_service.go            # Add provider_symbol to applySecurityExtraFields
│       └── security_service_test.go       # Add provider_symbol test cases
└── tests/
    └── integration/
        └── security_flow_test.go          # Add provider_symbol round-trip test
```

### Modified Files — Oracle

```
apps/oracle/
├── internal/
│   ├── client/
│   │   ├── kuberan.go                     # Add ProviderSymbol to Security struct
│   │   └── kuberan_test.go                # Add provider_symbol to mock response
│   ├── provider/
│   │   ├── provider.go                    # Add ProviderSymbol to Security struct
│   │   ├── yahoo.go                       # v7→v8 rewrite, provider_symbol support
│   │   └── yahoo_test.go                  # v8 response format, new tests
│   └── oracle/
│       ├── oracle.go                      # Map ProviderSymbol in conversion
│       └── oracle_test.go                 # Add ProviderSymbol test case
```

---

## Implementation Order

```
Phase 1: Add provider_symbol to API
  1.1  Database migration (000017)
  1.2  Update Security model
  1.3  Update Security handler (CreateSecurityRequest + buildSecurityExtraFields)
  1.4  Update Security service (applySecurityExtraFields)
  1.5  Update API tests (handler, service, integration)
  1.6  Verification (./scripts/check-go.sh apps/api)

Phase 2: Propagate provider_symbol to Oracle
  2.1  Update oracle client struct
  2.2  Update oracle provider struct
  2.3  Update oracle orchestrator (client→provider mapping)
  2.4  Update Yahoo provider (buildYahooSymbol uses provider_symbol)
  2.5  Update oracle tests
  2.6  Verification (./scripts/check-go.sh apps/oracle)

Phase 3: Switch Yahoo from v7 to v8
  3.1  Update Yahoo response types
  3.2  Update constants (base URL, remove batch max, add concurrency limit)
  3.3  Rewrite FetchPrices (concurrent per-symbol)
  3.4  Implement fetchOne method
  3.5  Remove batch logic
  3.6  Update Yahoo tests (v8 format, concurrency test)
  3.7  Verification (./scripts/check-go.sh apps/oracle)

Phase 4: Final Verification
  4.1  Full API verification
  4.2  Full oracle verification
  4.3  Docker build & live test
  4.4  Verification checklist
```

Note: Phases 2 and 3 both modify `yahoo.go`, so they should be done together in practice. The phases are separated for clarity in the plan.

## Verification

**API** — after each code change:
```bash
cd apps/api && go build ./...
```
After completing Phase 1:
```bash
./scripts/check-go.sh apps/api
```

**Oracle** — after each code change:
```bash
cd apps/oracle && go build ./...
```
After completing each phase:
```bash
cd apps/oracle && go test ./... -v
```
After completing all phases:
```bash
./scripts/check-go.sh apps/oracle
```

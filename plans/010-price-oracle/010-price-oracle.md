# Price Oracle: Automated Security Price Updates

## Context

The Kuberan application (Plans 001-009 complete) has a fully functional investment tracking system with securities, price history, portfolio snapshots, and a pipeline API for external data ingestion. However, there is no automated process to keep security prices up to date. Prices are only recorded when something calls `POST /api/v1/pipeline/securities/prices`, but nothing does — the pipeline exists as infrastructure with no data flowing through it.

### Current State

1. **Pipeline API exists but is idle**: Three pipeline endpoints are registered and functional — `POST /api/v1/pipeline/securities` (create securities), `POST /api/v1/pipeline/securities/prices` (record prices), and `POST /api/v1/pipeline/snapshots` (compute portfolio snapshots). All are guarded by `X-API-Key` auth via `PipelineAuthMiddleware`. The middleware returns 503 if `PIPELINE_API_KEY` is not configured.

2. **No way to list securities via pipeline**: The pipeline can create securities and record prices, but there is no `GET` endpoint for an external process to discover which securities exist. The authenticated `GET /api/v1/securities` endpoint exists but requires JWT auth, which is user-specific and inappropriate for a service-to-service context.

3. **Price staleness**: Investment valuations (`CurrentPrice`, market value, gain/loss) are computed from the latest entry in `security_prices` (Plan 008). If no prices are recorded, everything shows `MYR 0.00`. If prices were recorded once but never updated, valuations become stale indefinitely.

4. **Portfolio snapshots are never triggered**: `POST /api/v1/pipeline/snapshots` computes net worth snapshots for all users, but nothing calls it. The dashboard's net worth chart has no data points accumulating over time.

### What This Plan Adds

A standalone Go binary (`apps/oracle/`) that runs on a cron schedule (every 10-15 minutes), fetches current market prices from free data sources, and pushes them into Kuberan via the pipeline API. This completes the data flow:

```
Securities in DB ──GET──> Oracle ──fetch──> Yahoo Finance / CoinGecko
                                     │
                                     ▼
Kuberan API <──POST prices──────── Oracle
Kuberan API <──POST snapshots───── Oracle
```

**Out of scope**: Paid data source integrations, real-time price streaming (WebSocket), bond pricing (no reliable free source), historical backfill, and frontend changes (the frontend already handles prices via existing hooks).

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Add `GET /api/v1/pipeline/securities` endpoint | Backend (API) | New handler method, service method, route — returns all active securities |
| Price oracle binary | New app (`apps/oracle/`) | Go binary with provider abstraction, Yahoo Finance + CoinGecko providers |
| Docker Compose integration | Infrastructure | New service definition for the oracle |

## Technology & Patterns

### Oracle (`apps/oracle/`)
- **Language**: Go 1.24 (same as `apps/api/`)
- **Go module**: Separate `go.mod` — the oracle does not import anything from `apps/api/`. Communication is purely over HTTP.
- **HTTP client**: Standard `net/http` — no third-party HTTP libraries needed
- **Data sources**: Yahoo Finance (unofficial v8 API, no key required) for stocks/ETFs/REITs, CoinGecko (free public API, no key required) for crypto
- **Logging**: `log/slog` (Go standard library structured logging) — lightweight, no external dependencies. The oracle is a short-lived cron job, not a long-running service; Zap would be overkill.
- **Configuration**: Environment variables only, no CLI flags
- **Scheduling**: External cron/systemd timer — the binary is stateless and exits after each run
- **No database**: The oracle has no local storage. It reads from Kuberan's API and writes back to Kuberan's API.

### API Changes (`apps/api/`)
- Follows all existing patterns: 3-layer architecture, interface-based services, AppError types
- New endpoint follows pipeline auth pattern (API key, not JWT)

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Oracle language | Go | Same language as Kuberan backend. Simple deployment as a static binary. No runtime dependencies. |
| Oracle module | Separate `go.mod` in `apps/oracle/` | Clean separation. The oracle is a client of Kuberan, not a library consumer. No shared Go code. |
| Security list source | New `GET /api/v1/pipeline/securities` endpoint | Oracle talks to Kuberan only via HTTP. Clean API boundary. No direct DB access. |
| Data source: stocks/ETFs/REITs | Yahoo Finance unofficial API | Free, no API key, real-time to 15-min delayed, covers all equity-like instruments. Community-maintained, stable for years. |
| Data source: crypto | CoinGecko free API | Free tier allows 10-30 calls/min. Supports bulk queries. Best free crypto price API. |
| Data source: bonds | Not supported (out of scope) | No reliable free source for individual bond pricing. Users can manually record via pipeline API. |
| CoinGecko symbol mapping | Embedded lookup table in oracle | Top ~100 crypto ticker-to-slug mappings (BTC→bitcoin, ETH→ethereum, etc.). Covers 99% of personal portfolios. Simple, no API call overhead. |
| Scheduling mechanism | Host cron / systemd timer | Simplest approach for self-hosted. `*/15 * * * * docker compose run --rm oracle`. No in-container scheduler complexity. |
| Error handling | Partial success model | If Yahoo Finance returns 48 of 50 prices, record the 48 and log the 2 failures. Don't fail the entire run for individual symbol errors. |
| Snapshot trigger | Optional, on by default | After recording prices, trigger `POST /api/v1/pipeline/snapshots` to recompute portfolio net worth. Configurable via `COMPUTE_SNAPSHOTS` env var. |
| Logging | `log/slog` (stdlib) | Structured JSON logging. Lightweight for a cron job. No external dependencies. |
| Retry logic | None | Cron runs every 10-15 min. If a run fails, the next one will succeed. Retry adds complexity with no benefit for this cadence. |

---

## Phase 1: API Change — Add `GET /api/v1/pipeline/securities`

### 1.1 Add `ListAllSecurities` to Security Service

**File**: `apps/api/internal/services/security_service.go`

New method on `securityService`:

```go
func (s *securityService) ListAllSecurities() ([]models.Security, error)
```

Implementation:
1. Query all securities with no pagination and no soft-delete filter (GORM default excludes soft-deleted): `db.Find(&securities)`.
2. Order by `symbol ASC` for deterministic output.
3. Return the full list.

This is intentionally simple — the pipeline endpoint serves a machine client (the oracle) that needs all securities, not a paginated subset.

### 1.2 Add `ListAllSecurities` to `SecurityServicer` Interface

**File**: `apps/api/internal/services/interfaces.go`

Add to the `SecurityServicer` interface:

```go
ListAllSecurities() ([]models.Security, error)
```

### 1.3 Add `ListAllSecurities` Handler

**File**: `apps/api/internal/handlers/security_handler.go`

New handler method on `securityHandler`:

```go
func (h *securityHandler) ListAllSecurities(c *gin.Context)
```

Implementation:
1. Call `h.securityService.ListAllSecurities()`.
2. On error, return 500 with internal error.
3. On success, return `200` with `{"securities": [...]}`.
4. Add Swagger annotations.

No user context needed — this is a pipeline endpoint, not a user-scoped query. Securities are global (shared across all users).

### 1.4 Register Route

**File**: `apps/api/cmd/api/main.go`

Add to the pipeline route group (around line 222):

```go
pipeline.GET("/securities", securityHandler.ListAllSecurities)
```

This sits alongside the existing `pipeline.POST("/securities", ...)`.

### 1.5 Add Service Tests

**File**: `apps/api/internal/services/security_service_test.go`

Add `TestListAllSecurities`:
- `returns_all_securities`: Create 3 securities via DB. Call `ListAllSecurities`. Verify all 3 returned, ordered by symbol.
- `returns_empty_when_none`: No securities. Verify empty slice (not nil).
- `excludes_soft_deleted`: Create 2 securities, soft-delete one. Verify only the active one is returned.

### 1.6 Add Handler Tests

**File**: `apps/api/internal/handlers/security_handler_test.go`

Update `mockSecurityService` with `listAllSecuritiesFn` field and `ListAllSecurities` method.

Add `TestSecurityHandler_ListAllSecurities`:
- `returns_200_with_securities`: Mock returns securities list. Verify JSON response shape.
- `returns_200_empty_list`: Mock returns empty. Verify `{"securities": []}`.
- `returns_500_on_service_error`: Mock returns error. Verify error response.

Add `GET /pipeline/securities` to handler test router setup with pipeline auth middleware (or test without middleware, since middleware has its own tests).

### 1.7 Verification

Run `./scripts/check.sh` from `apps/api/`. All 5 steps must pass.

---

## Phase 2: Oracle — Project Skeleton

### 2.1 Initialize Go Module

Create `apps/oracle/go.mod`:

```
module github.com/kuberan/oracle
go 1.24
```

The module path doesn't need to match any real GitHub path — it's a local module in a monorepo.

### 2.2 Create Directory Structure

```
apps/oracle/
├── main.go
├── go.mod
└── internal/
    ├── config/
    │   └── config.go
    ├── client/
    │   └── kuberan.go
    ├── provider/
    │   ├── provider.go
    │   ├── yahoo.go
    │   ├── coingecko.go
    │   └── coingecko_symbols.go
    └── oracle/
        └── oracle.go
```

### 2.3 Configuration (`internal/config/config.go`)

Environment variables:

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `KUBERAN_API_URL` | Yes | — | Base URL of Kuberan API (e.g., `http://localhost:8080`) |
| `PIPELINE_API_KEY` | Yes | — | Shared secret for `X-API-Key` header |
| `LOG_LEVEL` | No | `info` | Structured log level: `debug`, `info`, `warn`, `error` |
| `REQUEST_TIMEOUT` | No | `30s` | HTTP timeout per provider request |
| `COMPUTE_SNAPSHOTS` | No | `true` | Whether to trigger snapshot computation after price update |

Implementation:
- Read from `os.Getenv`.
- Validate that required variables are set; exit with error if not.
- Parse `REQUEST_TIMEOUT` as `time.Duration`.
- Parse `COMPUTE_SNAPSHOTS` as bool (accept `true`/`false`/`1`/`0`).
- Parse `LOG_LEVEL` and configure `slog` accordingly.

### 2.4 Verification

Run `go build ./...` from `apps/oracle/`. Should compile (empty main is fine at this stage).

---

## Phase 3: Oracle — Kuberan API Client

### 3.1 Implement Kuberan Client (`internal/client/kuberan.go`)

Define types mirroring the API responses:

```go
type Security struct {
    ID        uint   `json:"id"`
    Symbol    string `json:"symbol"`
    Name      string `json:"name"`
    AssetType string `json:"asset_type"`
    Currency  string `json:"currency"`
    Exchange  string `json:"exchange"`
    Network   string `json:"network"`
}

type RecordPriceEntry struct {
    SecurityID uint   `json:"security_id"`
    Price      int64  `json:"price"`
    RecordedAt string `json:"recorded_at"` // RFC3339
}
```

Client struct:

```go
type KuberanClient struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
}
```

Three methods:

**`GetSecurities(ctx context.Context) ([]Security, error)`**
- `GET {baseURL}/api/v1/pipeline/securities` with `X-API-Key` header.
- Parse JSON response `{"securities": [...]}`.
- Return the securities slice.

**`RecordPrices(ctx context.Context, prices []RecordPriceEntry) (int, error)`**
- `POST {baseURL}/api/v1/pipeline/securities/prices` with `X-API-Key` header.
- Body: `{"prices": [...]}`.
- Parse JSON response `{"prices_recorded": N}`.
- Return N.

**`ComputeSnapshots(ctx context.Context) (int, error)`**
- `POST {baseURL}/api/v1/pipeline/snapshots` with `X-API-Key` header.
- Body: `{"recorded_at": "RFC3339 now"}`.
- Parse JSON response `{"snapshots_recorded": N}`.
- Return N.

All methods:
- Set `Content-Type: application/json` and `X-API-Key` headers.
- Check response status code; return descriptive errors for non-2xx.
- Use the provided `context.Context` for cancellation/timeout.

### 3.2 Add Client Tests (`internal/client/kuberan_test.go`)

Use `httptest.NewServer` to mock the Kuberan API:

- `TestGetSecurities_Success`: Mock returns 200 with 3 securities. Verify all parsed correctly.
- `TestGetSecurities_Unauthorized`: Mock returns 401. Verify error message includes "401".
- `TestGetSecurities_ServerError`: Mock returns 500. Verify error.
- `TestRecordPrices_Success`: Mock returns 200 with `{"prices_recorded": 5}`. Verify return value.
- `TestRecordPrices_ValidatesRequestBody`: Mock captures request body. Verify JSON structure matches expectations.
- `TestComputeSnapshots_Success`: Mock returns 200 with `{"snapshots_recorded": 3}`. Verify return value.

### 3.3 Verification

Run `go test ./...` from `apps/oracle/`. All tests must pass.

---

## Phase 4: Oracle — Provider Interface

### 4.1 Define Provider Types (`internal/provider/provider.go`)

```go
// Security represents a security from the Kuberan API, containing
// the fields needed by price providers to fetch quotes.
type Security struct {
    ID        uint
    Symbol    string
    AssetType string
    Exchange  string
    Network   string
    Currency  string
}

// PriceResult represents a successfully fetched price for a security.
type PriceResult struct {
    SecurityID uint
    Price      int64     // cents
    RecordedAt time.Time
}

// FetchError represents a failed price fetch for a specific security.
type FetchError struct {
    SecurityID uint
    Symbol     string
    Err        error
}

func (e *FetchError) Error() string {
    return fmt.Sprintf("failed to fetch price for %s (ID %d): %v", e.Symbol, e.SecurityID, e.Err)
}

// Provider fetches current market prices for a set of securities.
type Provider interface {
    // Name returns the provider's display name (e.g., "Yahoo Finance", "CoinGecko").
    Name() string

    // Supports returns true if this provider can fetch prices for the given asset type.
    Supports(assetType string) bool

    // FetchPrices fetches current prices for the given securities.
    // Returns successful results and any per-security errors.
    // A provider should return as many prices as possible, even if some fail.
    FetchPrices(ctx context.Context, securities []Security) ([]PriceResult, []FetchError)
}
```

Key design points:
- `FetchPrices` returns `([]PriceResult, []FetchError)` — not a single `error`. This enables partial success: if 48 of 50 symbols resolve, we get 48 prices and 2 errors. The orchestrator records the successes and logs the failures.
- `FetchError` includes the symbol and security ID for logging context.
- `Security` is a separate type from the client's `Security` — the provider doesn't need to know about the API response shape. The orchestrator maps between them.

### 4.2 Verification

Run `go build ./...` from `apps/oracle/`.

---

## Phase 5: Oracle — Yahoo Finance Provider

### 5.1 Implement Yahoo Finance Provider (`internal/provider/yahoo.go`)

Struct:

```go
type YahooProvider struct {
    httpClient *http.Client
}
```

**`Supports(assetType string) bool`**: Returns true for `stock`, `etf`, `bond`, `reit`.

**`FetchPrices(ctx context.Context, securities []Security) ([]PriceResult, []FetchError)`**:

1. Build symbol list with exchange suffixes:
   - Map each security's `(Symbol, Exchange)` to a Yahoo-compatible ticker.
   - Exchange suffix mapping: `TSX` → `.TO`, `LSE` → `.L`, `HKEX` → `.HK`, `ASX` → `.AX`, etc.
   - If `Exchange` is empty or `NYSE`/`NASDAQ`, use the symbol as-is.
   - Maintain a map from Yahoo ticker back to security ID for result mapping.

2. Call Yahoo Finance v8 quote API:
   - URL: `https://query1.finance.yahoo.com/v7/finance/quote?symbols={comma-separated}`
   - Yahoo supports batch queries — up to ~50 symbols per request.
   - If more than 50 securities, split into batches and make multiple requests.
   - Set a `User-Agent` header (Yahoo may reject requests without one).

3. Parse the JSON response:
   - Response shape: `{"quoteResponse": {"result": [{"symbol": "AAPL", "regularMarketPrice": 178.72, ...}], "error": null}}`
   - For each result, extract `regularMarketPrice` (float64).
   - Convert to int64 cents: `int64(math.Round(price * 100))`.
   - Map back to security ID using the ticker→ID map.

4. Handle errors:
   - If a symbol is not found in the response, create a `FetchError` for it.
   - If the HTTP request fails entirely, create `FetchError` for all securities in that batch.
   - If a symbol returns `regularMarketPrice = 0` or missing, create a `FetchError` (zero price is likely an error, not a real quote).

### 5.2 Exchange Suffix Mapping

The exchange-to-suffix mapping should be a package-level `map[string]string`:

```go
var exchangeSuffixes = map[string]string{
    "TSX":   ".TO",
    "TSXV":  ".V",
    "LSE":   ".L",
    "HKEX":  ".HK",
    "ASX":   ".AX",
    "NSE":   ".NS",
    "BSE":   ".BO",
    "SGX":   ".SI",
    "KRX":   ".KS",
    "KOSDAQ": ".KQ",
    "BURSA": ".KL",
    "JPX":   ".T",
    "FRA":   ".F",
    "XETRA": ".DE",
    "SIX":   ".SW",
    "EURONEXT": ".PA",
    // NYSE, NASDAQ, AMEX: no suffix needed
}
```

This list covers major global exchanges. Securities with unlisted exchanges use the symbol as-is (assumes US market).

### 5.3 Add Yahoo Provider Tests (`internal/provider/yahoo_test.go`)

Use `httptest.NewServer` to mock Yahoo Finance:

- `TestYahooProvider_Supports`: Verify returns true for stock/etf/bond/reit, false for crypto.
- `TestYahooProvider_FetchPrices_Success`: Mock returns valid quote response for 3 symbols. Verify 3 PriceResults with correct cent conversion.
- `TestYahooProvider_FetchPrices_PartialFailure`: Mock returns 2 of 3 symbols. Verify 2 results + 1 FetchError.
- `TestYahooProvider_FetchPrices_ExchangeSuffix`: Security with Exchange="TSX" and Symbol="SHOP". Verify request URL includes "SHOP.TO".
- `TestYahooProvider_FetchPrices_BatchSplit`: Create 60 securities. Verify 2 HTTP requests are made (batch of 50 + batch of 10).
- `TestYahooProvider_FetchPrices_HTTPError`: Mock returns 500. Verify all securities return FetchErrors.
- `TestYahooProvider_FetchPrices_ZeroPrice`: Mock returns `regularMarketPrice: 0`. Verify FetchError (not a PriceResult with 0).

### 5.4 Verification

Run `go test ./...` from `apps/oracle/`.

---

## Phase 6: Oracle — CoinGecko Provider

### 6.1 Implement CoinGecko Symbol Mapping (`internal/provider/coingecko_symbols.go`)

A package-level map from uppercase ticker symbol to CoinGecko API slug:

```go
var coinGeckoIDs = map[string]string{
    "BTC":   "bitcoin",
    "ETH":   "ethereum",
    "USDT":  "tether",
    "BNB":   "binancecoin",
    "SOL":   "solana",
    "XRP":   "ripple",
    "USDC":  "usd-coin",
    "ADA":   "cardano",
    "DOGE":  "dogecoin",
    "AVAX":  "avalanche-2",
    "TRX":   "tron",
    "DOT":   "polkadot",
    "LINK":  "chainlink",
    "MATIC": "matic-network",
    "POL":   "matic-network",
    "SHIB":  "shiba-inu",
    "TON":   "the-open-network",
    "DAI":   "dai",
    "LTC":   "litecoin",
    "BCH":   "bitcoin-cash",
    "UNI":   "uniswap",
    "ATOM":  "cosmos",
    "XLM":   "stellar",
    "ETC":   "ethereum-classic",
    "XMR":   "monero",
    "FIL":   "filecoin",
    "ARB":   "arbitrum",
    "OP":    "optimism",
    "APT":   "aptos",
    "SUI":   "sui",
    "NEAR":  "near",
    "AAVE":  "aave",
    "MKR":   "maker",
    "GRT":   "the-graph",
    "ALGO":  "algorand",
    "FTM":   "fantom",
    "SAND":  "the-sandbox",
    "MANA":  "decentraland",
    "AXS":   "axie-infinity",
    "HBAR":  "hedera-hashgraph",
    "ICP":   "internet-computer",
    "VET":   "vechain",
    "THETA": "theta-token",
    "EGLD":  "elrond-erd-2",
    "FLOW":  "flow",
    "XTZ":   "tezos",
    "NEO":   "neo",
    "KLAY":  "klay-token",
    "QNT":   "quant-network",
    "CRV":   "curve-dao-token",
    "SNX":   "havven",
    "RPL":   "rocket-pool",
    "COMP":  "compound-governance-token",
    "1INCH": "1inch",
    "ENS":   "ethereum-name-service",
    "LDO":   "lido-dao",
    "IMX":   "immutable-x",
    "RNDR":  "render-token",
    "INJ":   "injective-protocol",
    "FET":   "fetch-ai",
    "PEPE":  "pepe",
    "WIF":   "dogwifcoin",
    "BONK":  "bonk",
    "FLOKI": "floki",
    // Add more as needed
}
```

Also export a lookup function:

```go
// LookupCoinGeckoID returns the CoinGecko API slug for a ticker symbol.
// Returns the slug and true if found, or empty string and false if not.
func LookupCoinGeckoID(symbol string) (string, bool) {
    id, ok := coinGeckoIDs[strings.ToUpper(symbol)]
    return id, ok
}
```

### 6.2 Implement CoinGecko Provider (`internal/provider/coingecko.go`)

Struct:

```go
type CoinGeckoProvider struct {
    httpClient *http.Client
}
```

**`Supports(assetType string) bool`**: Returns true for `crypto` only.

**`FetchPrices(ctx context.Context, securities []Security) ([]PriceResult, []FetchError)`**:

1. Map securities to CoinGecko IDs:
   - For each security, call `LookupCoinGeckoID(security.Symbol)`.
   - If not found, create a `FetchError` for that security.
   - Build a comma-separated list of CoinGecko IDs.
   - Maintain a map from CoinGecko ID back to security (for result mapping).

2. Determine the target currency:
   - Use the first security's `Currency` field (lowercased) as `vs_currencies`.
   - If securities have mixed currencies, group by currency and make separate requests.
   - For simplicity in V1, assume all crypto securities share the same currency (typically `USD`).

3. Call CoinGecko simple price API:
   - URL: `https://api.coingecko.com/api/v3/simple/price?ids={comma-separated}&vs_currencies={currency}`
   - This is a bulk endpoint — one call for all IDs.
   - CoinGecko free tier: 10-30 calls/min, more than enough for a single bulk call.

4. Parse the JSON response:
   - Response shape: `{"bitcoin": {"usd": 67234.56}, "ethereum": {"usd": 3456.78}}`
   - For each result, extract the price for the target currency.
   - Convert to int64 cents: `int64(math.Round(price * 100))`.
   - Map back to security ID.

5. Handle errors:
   - If a CoinGecko ID is not in the response, create a `FetchError`.
   - If the HTTP request fails, create `FetchError` for all securities.

### 6.3 Add CoinGecko Provider Tests (`internal/provider/coingecko_test.go`)

Use `httptest.NewServer`:

- `TestCoinGeckoProvider_Supports`: Verify returns true for crypto, false for stock/etf/bond/reit.
- `TestCoinGeckoProvider_FetchPrices_Success`: Mock returns prices for BTC and ETH. Verify 2 PriceResults with correct cent conversion.
- `TestCoinGeckoProvider_FetchPrices_UnknownSymbol`: Security with unmapped symbol "OBSCURECOIN". Verify FetchError.
- `TestCoinGeckoProvider_FetchPrices_PartialResponse`: Mock returns price for BTC but not ETH. Verify 1 result + 1 error.
- `TestCoinGeckoProvider_FetchPrices_HTTPError`: Mock returns 429 (rate limit). Verify all FetchErrors.

### 6.4 Test Symbol Lookup

- `TestLookupCoinGeckoID_Found`: Verify BTC → "bitcoin", ETH → "ethereum".
- `TestLookupCoinGeckoID_CaseInsensitive`: Verify "btc" → "bitcoin" (lowercase input).
- `TestLookupCoinGeckoID_NotFound`: Verify unknown symbol returns false.

### 6.5 Verification

Run `go test ./...` from `apps/oracle/`.

---

## Phase 7: Oracle — Orchestrator

### 7.1 Implement Orchestrator (`internal/oracle/oracle.go`)

Struct:

```go
type Oracle struct {
    client    *client.KuberanClient
    providers []provider.Provider
    config    *config.Config
    logger    *slog.Logger
}

type RunResult struct {
    SecuritiesFetched int
    PricesRecorded    int
    SnapshotsRecorded int
    Errors            []provider.FetchError
    Duration          time.Duration
}
```

**`Run(ctx context.Context) (*RunResult, error)`**:

1. **Fetch securities**: Call `client.GetSecurities(ctx)`. If this fails, return error (fatal — can't proceed without knowing what to fetch).

2. **Map to provider types**: Convert `[]client.Security` to `[]provider.Security` for the provider interface.

3. **Group by provider**: For each security, find the first provider where `provider.Supports(security.AssetType)` is true. Group securities by provider. Securities with no matching provider are logged as warnings (e.g., bonds with no bond provider).

4. **Fetch prices in parallel**: Use `sync.WaitGroup` (or `errgroup.Group`) to call each provider's `FetchPrices` concurrently. Collect all `PriceResult`s and `FetchError`s.

5. **Convert and push prices**: Map `[]provider.PriceResult` to `[]client.RecordPriceEntry` (converting `time.Time` to RFC3339 string). Call `client.RecordPrices(ctx, entries)`. If this fails, return error (fatal — prices were fetched but couldn't be recorded).

6. **Trigger snapshots** (if configured): Call `client.ComputeSnapshots(ctx)`. If this fails, log a warning but don't fail the run (prices were already recorded successfully).

7. **Return summary**: Populate `RunResult` with counts and duration.

### 7.2 Add Orchestrator Tests (`internal/oracle/oracle_test.go`)

Use interfaces/mocks to test the orchestrator without real HTTP calls:

- Define a `SecurityClient` interface in the oracle package that `KuberanClient` satisfies. This allows injecting a mock client in tests.
- Define mock providers that return canned results.

Tests:
- `TestOracle_Run_FullFlow`: 3 stocks + 2 crypto securities. Mock Yahoo returns 3 prices, mock CoinGecko returns 2 prices. Verify client receives all 5 prices and snapshots are triggered.
- `TestOracle_Run_PartialProviderFailure`: Yahoo returns 2 of 3, CoinGecko returns both. Verify 4 prices recorded, 1 error in result.
- `TestOracle_Run_NoSecurities`: Mock client returns empty list. Verify early exit, 0 prices recorded.
- `TestOracle_Run_UnsupportedAssetType`: 1 bond security with no bond provider. Verify logged warning, 0 prices recorded for it, no crash.
- `TestOracle_Run_GetSecuritiesFails`: Mock client returns error. Verify oracle returns error.
- `TestOracle_Run_RecordPricesFails`: Mock client `RecordPrices` returns error. Verify oracle returns error (fatal).
- `TestOracle_Run_SnapshotFailureNonFatal`: Mock client `ComputeSnapshots` returns error. Verify oracle returns success (prices were recorded).
- `TestOracle_Run_SnapshotsDisabled`: Config has `COMPUTE_SNAPSHOTS=false`. Verify `ComputeSnapshots` is never called.

### 7.3 Verification

Run `go test ./...` from `apps/oracle/`.

---

## Phase 8: Oracle — Main Entrypoint

### 8.1 Implement `main.go`

**File**: `apps/oracle/main.go`

```go
func main() {
    // 1. Load config
    cfg, err := config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
        os.Exit(1)
    }

    // 2. Set up structured logger
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: cfg.LogLevel,
    }))

    // 3. Create HTTP client with timeout
    httpClient := &http.Client{Timeout: cfg.RequestTimeout}

    // 4. Create Kuberan client
    kuberanClient := client.NewKuberanClient(cfg.KuberanAPIURL, cfg.PipelineAPIKey, httpClient)

    // 5. Create providers
    providers := []provider.Provider{
        provider.NewYahooProvider(httpClient),
        provider.NewCoinGeckoProvider(httpClient),
    }

    // 6. Create and run oracle
    orc := oracle.NewOracle(kuberanClient, providers, cfg, logger)
    ctx := context.Background()
    result, err := orc.Run(ctx)
    if err != nil {
        logger.Error("oracle run failed", "error", err)
        os.Exit(1)
    }

    // 7. Log summary
    logger.Info("oracle run completed",
        "securities_fetched", result.SecuritiesFetched,
        "prices_recorded", result.PricesRecorded,
        "snapshots_recorded", result.SnapshotsRecorded,
        "errors", len(result.Errors),
        "duration", result.Duration.String(),
    )

    // 8. Log individual errors at warn level
    for _, fetchErr := range result.Errors {
        logger.Warn("price fetch failed",
            "symbol", fetchErr.Symbol,
            "security_id", fetchErr.SecurityID,
            "error", fetchErr.Err.Error(),
        )
    }

    // 9. Exit with non-zero if there were errors (but prices were still recorded)
    if len(result.Errors) > 0 {
        os.Exit(2) // Partial success
    }
}
```

Exit codes:
- `0`: Full success — all prices fetched and recorded.
- `1`: Fatal error — couldn't reach API, couldn't record prices, or config error.
- `2`: Partial success — some prices recorded, some failed. Cron can alert on this if desired.

### 8.2 Verification

Run `go build ./...` from `apps/oracle/`. Binary should compile.
Run `go test ./...` from `apps/oracle/`. All tests should pass.
Run `go vet ./...` from `apps/oracle/`. No issues.

---

## Phase 9: Docker Compose Integration

### 9.1 Create Dockerfile (`apps/oracle/Dockerfile`)

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /oracle ./main.go

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /oracle /oracle
ENTRYPOINT ["/oracle"]
```

Notes:
- Multi-stage build for minimal image size.
- `CGO_ENABLED=0` for static binary (no libc dependency).
- `ca-certificates` needed for HTTPS calls to Yahoo Finance and CoinGecko.

### 9.2 Add to Docker Compose

**File**: `docker-compose.yml` (repo root)

Add the oracle service:

```yaml
oracle:
  build:
    context: ./apps/oracle
    dockerfile: Dockerfile
  environment:
    - KUBERAN_API_URL=http://api:8080
    - PIPELINE_API_KEY=${PIPELINE_API_KEY}
    - COMPUTE_SNAPSHOTS=true
    - LOG_LEVEL=info
  depends_on:
    - api
  profiles:
    - oracle
  restart: "no"
```

Notes:
- Uses Docker Compose `profiles` so it doesn't run during normal `docker compose up`. Run explicitly with `docker compose --profile oracle run --rm oracle`.
- `restart: "no"` — it's a one-shot process, not a long-running service.
- `PIPELINE_API_KEY` is read from the host environment (or `.env` file).
- `depends_on: api` ensures the API is running before the oracle starts.

### 9.3 Update `.env.example` (if it exists)

Add:

```
PIPELINE_API_KEY=your-secret-key-here
```

### 9.4 Verification

Run `docker compose --profile oracle build oracle` from the repo root. Should build successfully.

---

## Phase 10: Final Verification

### 10.1 Full Backend Verification

Run `./scripts/check.sh` from `apps/api/`. All 5 steps must pass.

### 10.2 Full Oracle Verification

Run the following from `apps/oracle/`:
- `go build ./...`
- `go vet ./...`
- `go test ./... -v`
- `go test ./... -race`

All must pass.

### 10.3 Docker Build Verification

Run `docker compose --profile oracle build oracle` from the repo root.

### 10.4 Verification Checklist

- [ ] `GET /api/v1/pipeline/securities` returns all active securities with API key auth
- [ ] `GET /api/v1/pipeline/securities` returns 401 without valid API key
- [ ] `GET /api/v1/pipeline/securities` returns 503 when `PIPELINE_API_KEY` is not configured
- [ ] Oracle binary compiles and runs with valid config
- [ ] Oracle exits with code 1 on missing required config
- [ ] Oracle fetches securities from Kuberan API
- [ ] Yahoo Finance provider fetches prices for stocks, ETFs, REITs
- [ ] Yahoo Finance provider handles exchange suffixes correctly (e.g., SHOP.TO for TSX)
- [ ] Yahoo Finance provider batches requests for >50 symbols
- [ ] CoinGecko provider fetches prices for crypto
- [ ] CoinGecko symbol mapping covers top 60+ cryptocurrencies
- [ ] CoinGecko provider handles unmapped symbols gracefully (FetchError, not crash)
- [ ] Oracle records fetched prices via pipeline API
- [ ] Oracle triggers portfolio snapshot computation (when configured)
- [ ] Oracle handles partial failures — records successful prices, logs failures
- [ ] Oracle exit code 0 for full success, 1 for fatal, 2 for partial success
- [ ] All service and handler tests pass for new pipeline endpoint
- [ ] All oracle unit tests pass (client, providers, orchestrator)
- [ ] All oracle tests pass with race detector
- [ ] Docker image builds successfully
- [ ] No `//nolint` directives without justification

---

## API Changes

### New Endpoint

#### GET /api/v1/pipeline/securities

Returns all active (non-deleted) securities. Requires pipeline API key auth.

**Headers**:
- `X-API-Key: {pipeline_api_key}` (required)

**Response** (200):
```json
{
  "securities": [
    {
      "id": 1,
      "symbol": "AAPL",
      "name": "Apple Inc.",
      "asset_type": "stock",
      "currency": "USD",
      "exchange": "NASDAQ",
      "network": "",
      "maturity_date": null,
      "yield_to_maturity": 0,
      "coupon_rate": 0,
      "property_type": ""
    },
    {
      "id": 7,
      "symbol": "BTC",
      "name": "Bitcoin",
      "asset_type": "crypto",
      "currency": "USD",
      "exchange": "",
      "network": "bitcoin"
    }
  ]
}
```

**Error Responses**:
- `401 Unauthorized`: Invalid or missing API key.
- `503 Service Unavailable`: Pipeline not configured (`PIPELINE_API_KEY` not set).

---

## Files Changed

### New Files — Backend

```
apps/api/
  (no new files — modifications only)
```

### Modified Files — Backend

```
apps/api/
├── internal/
│   ├── services/
│   │   ├── interfaces.go                 # Add ListAllSecurities to SecurityServicer
│   │   ├── security_service.go           # Implement ListAllSecurities
│   │   └── security_service_test.go      # Add ListAllSecurities tests
│   └── handlers/
│       ├── security_handler.go           # Add ListAllSecurities handler
│       └── security_handler_test.go      # Add mock method and handler tests
├── cmd/
│   └── api/main.go                       # Register GET /pipeline/securities route
```

### New Files — Oracle

```
apps/oracle/
├── main.go
├── go.mod
├── go.sum
├── Dockerfile
└── internal/
    ├── config/
    │   └── config.go
    ├── client/
    │   ├── kuberan.go
    │   └── kuberan_test.go
    ├── provider/
    │   ├── provider.go
    │   ├── yahoo.go
    │   ├── yahoo_test.go
    │   ├── coingecko.go
    │   ├── coingecko_test.go
    │   └── coingecko_symbols.go
    └── oracle/
        ├── oracle.go
        └── oracle_test.go
```

### Modified Files — Infrastructure

```
docker-compose.yml                        # Add oracle service with profile
```

---

## Implementation Order

```
Phase 1: API Change — Add GET /api/v1/pipeline/securities
  1.1  Add ListAllSecurities to security service
  1.2  Add ListAllSecurities to SecurityServicer interface
  1.3  Add ListAllSecurities handler
  1.4  Register route in main.go
  1.5  Add service tests
  1.6  Add handler tests
  1.7  Verification (./scripts/check.sh)

Phase 2: Oracle — Project Skeleton
  2.1  Initialize Go module (go.mod)
  2.2  Create directory structure
  2.3  Implement configuration (config.go)
  2.4  Verification (go build)

Phase 3: Oracle — Kuberan API Client
  3.1  Implement Kuberan client (kuberan.go)
  3.2  Add client tests (kuberan_test.go)
  3.3  Verification (go test)

Phase 4: Oracle — Provider Interface
  4.1  Define provider types and interface (provider.go)
  4.2  Verification (go build)

Phase 5: Oracle — Yahoo Finance Provider
  5.1  Implement Yahoo Finance provider (yahoo.go)
  5.2  Exchange suffix mapping
  5.3  Add Yahoo provider tests (yahoo_test.go)
  5.4  Verification (go test)

Phase 6: Oracle — CoinGecko Provider
  6.1  Implement CoinGecko symbol mapping (coingecko_symbols.go)
  6.2  Implement CoinGecko provider (coingecko.go)
  6.3  Add CoinGecko provider tests (coingecko_test.go)
  6.4  Test symbol lookup
  6.5  Verification (go test)

Phase 7: Oracle — Orchestrator
  7.1  Implement orchestrator (oracle.go)
  7.2  Add orchestrator tests (oracle_test.go)
  7.3  Verification (go test)

Phase 8: Oracle — Main Entrypoint
  8.1  Implement main.go
  8.2  Verification (go build, go test, go vet)

Phase 9: Docker Compose Integration
  9.1  Create Dockerfile
  9.2  Add to docker-compose.yml
  9.3  Update .env.example
  9.4  Verification (docker compose build)

Phase 10: Final Verification
  10.1  Full backend verification (check.sh)
  10.2  Full oracle verification (build, vet, test, test -race)
  10.3  Docker build verification
  10.4  Verification checklist
```

## Verification

**Backend** — after each code change:
```bash
cd apps/api && go build ./...
```
After completing Phase 1:
```bash
cd apps/api && ./scripts/check.sh
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
cd apps/oracle && go build ./... && go vet ./... && go test ./... -v && go test ./... -race
```

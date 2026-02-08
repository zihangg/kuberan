# Kuberan S-Tier Backend Upgrade Plan

## Context

The Kuberan backend (`apps/api/`) is a Go API built with Gin + GORM + PostgreSQL. It has a working foundation for user auth, accounts, categories, and transactions, but has significant quality and architecture issues that need to be resolved before it can be considered production-ready.

This plan describes 14 phases of improvements to bring the codebase to S-tier quality: clean architecture, proper error handling, comprehensive testing, full feature implementation, and production hardening.

## Current State (Before This Plan)

### What Exists
- User registration/login with JWT
- Cash account CRUD
- Category CRUD with hierarchical parent/child support
- Transaction create/get/delete with atomic balance updates
- Swagger documentation
- Docker Compose dev environment

### Known Issues
1. **Duplicate model definitions**: Models defined in both `internal/models/` (used) and domain packages (`internal/account/`, `internal/auth/`, etc.) (unused). The domain packages have import cycle errors.
2. **Dead code**: `internal/app/` (abandoned bootstrap), `internal/router/` (empty).
3. **Data consistency bug**: Account creation with initial balance is not atomic. The initial transaction insert can fail silently.
4. **Nested DB transactions**: `TransactionService.CreateTransaction` opens a transaction inside another transaction context unnecessarily.
5. **float64 for money**: All monetary values use `float64`, which causes floating-point rounding errors.
6. **No custom error types**: Services return `errors.New("string")`, which handlers pass directly to clients, leaking internal error details.
7. **No input validation beyond `binding:"required"`**: No length limits, no enum validation, no currency code validation.
8. **No logging framework**: Uses `log.Println` and `fmt.Printf`.
9. **No tests**: Zero test files exist.
10. **No migrations**: Uses GORM AutoMigrate.
11. **No pagination**: List endpoints return all records.
12. **No service interfaces**: Handlers depend on concrete structs, making testing impossible without a real DB.
13. **Incomplete features**: Budget and Investment models exist but have no service/handler/endpoint implementations.
14. **No transfer support**: `TransactionTypeTransfer` and `ToAccountID` exist in the model but are not implemented.
15. **Weak security**: No token refresh, no login lockout, no audit logging, JWT secret falls back silently.
16. **No graceful shutdown**: Server uses `router.Run()` directly.
17. **`main.go` in wrong location**: At `apps/api/main.go` instead of `apps/api/cmd/api/main.go`.
18. **No linting or CI feedback loop**: No `golangci-lint` configuration, no `go vet` integration, no automated checks that an agent or developer can run to verify correctness.

---

## Phase 1: Cleanup & Foundation

**Goal**: Remove all dead/duplicate code, fix critical bugs, add structured logging.

### 1.1 Delete Duplicate Domain Packages

Delete these directories entirely. They are unused duplicates of `internal/models/` with import cycle errors:

```
internal/account/     -> duplicate of models/account.go
internal/auth/        -> duplicate of models/user.go
internal/budget/      -> duplicate of models/budget.go
internal/category/    -> duplicate of models/category.go
internal/transaction/ -> duplicate of models/transaction.go
internal/investment/  -> duplicate of models/investment.go
```

### 1.2 Delete Abandoned Code

```
internal/app/         -> abandoned application bootstrap (app.go references non-existent SetupRouter, router.go is a stub with empty function)
internal/router/      -> empty directory
```

### 1.3 Move main.go

Move `apps/api/main.go` to `apps/api/cmd/api/main.go`.

Update these files to reflect the new path:
- `apps/api/air.toml`: Change `cmd = "go build -o ./tmp/api ."` to `cmd = "go build -o ./tmp/api ./cmd/api"`
- `apps/api/dockerfiles/Dockerfile.dev`: Update if it references main.go path
- Root `package.json`: Update `build:backend` script to `go build -o ../../bin/api ./cmd/api`

### 1.4 Fix Account Creation Atomicity Bug

**File**: `internal/services/account_service.go`, method `CreateCashAccount`

**Current behavior** (lines 48-64):
```go
if err := s.db.Create(account).Error; err != nil {
    return nil, err
}
if initialBalance > 0 {
    transaction := &models.Transaction{...}
    if err := s.db.Create(transaction).Error; err != nil {
        return account, nil  // BUG: silently swallows error
    }
}
```

**Required fix**: Wrap both operations in a single `s.db.Transaction()`:
```go
err := s.db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(account).Error; err != nil {
        return err
    }
    if initialBalance > 0 {
        transaction := &models.Transaction{...}
        if err := tx.Create(transaction).Error; err != nil {
            return err  // Rolls back account creation too
        }
    }
    return nil
})
```

### 1.5 Fix Nested Transaction Pattern

**File**: `internal/services/transaction_service.go`

**Current behavior**: `CreateTransaction` (line 27) calls `createTransactionWithDB(s.db, ...)` which internally opens its own `tx.Transaction(...)`. This creates an unnecessary GORM savepoint.

**Required fix**: `CreateTransaction` should open the DB transaction and pass `tx` to `createTransactionWithDB`. The inner method should operate directly on the passed `tx` without opening another transaction.

```go
func (s *TransactionService) CreateTransaction(...) (*models.Transaction, error) {
    // ... validation ...
    var result *models.Transaction
    err := s.db.Transaction(func(tx *gorm.DB) error {
        var err error
        result, err = s.createTransactionWithDB(tx, ...)
        return err
    })
    return result, err
}

func (s *TransactionService) createTransactionWithDB(tx *gorm.DB, ...) (*models.Transaction, error) {
    // Use tx directly for all operations, do NOT call tx.Transaction()
    if err := tx.Create(transaction).Error; err != nil {
        return nil, err
    }
    if err := s.accountService.UpdateAccountBalance(tx, account, ...); err != nil {
        return nil, err
    }
    return transaction, nil
}
```

### 1.6 Integrate Zap Logger

**New dependency**: `go.uber.org/zap`

**New file**: `internal/logger/logger.go`
- `Init(env string)` function: Creates Zap logger (JSON encoder for production, console encoder for development)
- `Get() *zap.SugaredLogger` function: Returns the global logger instance
- Logger should be initialized in `main.go` before anything else

**New file**: `internal/middleware/logging.go`
- Request logging middleware for Gin
- Generates UUID request IDs (new dependency: `github.com/google/uuid`)
- Sets request ID in Gin context and `X-Request-ID` response header
- Logs: method, path, status code, latency, client IP, request ID
- Uses Zap for all log output

**Files to update** (replace `log.Println`, `log.Fatalf`, `fmt.Printf`):
- `internal/config/config.go`: 3 occurrences
- `internal/database/database.go`: 2 occurrences
- `cmd/api/main.go`: 3 occurrences

After this phase, zero calls to `log.Println`, `log.Fatalf`, or `fmt.Printf` should remain.

### 1.7 Verification

- `go build ./cmd/api` succeeds
- `go vet ./...` passes
- No references to deleted packages remain
- All logging goes through Zap

---

## Phase 2: Linting & Feedback Loops

**Goal**: Establish automated code quality checks that both developers and agentic coding tools can run to verify correctness after every change.

This phase is critical for agentic development. Every subsequent phase will produce code changes, and the agent needs a fast, deterministic way to verify that changes compile, pass linting, and don't introduce regressions. Without this, errors compound silently across phases.

### 2.1 Install and Configure golangci-lint

**New file**: `apps/api/.golangci.yml`

```yaml
run:
  timeout: 5m
  modules-download-mode: readonly

linters:
  enable:
    - errcheck        # Check that errors are handled
    - govet           # Reports suspicious constructs (shadows, printf args, etc.)
    - ineffassign     # Detects unused variable assignments
    - staticcheck     # Comprehensive static analysis
    - unused          # Finds unused code
    - gosimple        # Suggests code simplifications
    - gocritic        # Opinionated linter with many useful checks
    - revive          # Replacement for golint, configurable
    - misspell        # Finds misspelled English words in comments/strings
    - unconvert       # Removes unnecessary type conversions
    - unparam         # Finds unused function parameters
    - gofmt           # Checks code is gofmt-ed
    - goimports       # Checks imports are sorted and grouped
    - bodyclose       # Checks HTTP response bodies are closed
    - nilerr          # Finds code that returns nil when err is not nil
    - exportloopref   # Checks for pointers to loop variables
    - sqlclosecheck   # Checks that sql.Rows/sql.Stmt are closed

linters-settings:
  revive:
    rules:
      - name: exported
        arguments:
          - "checkPrivateReceivers"
          - "sayRepetitiveInsteadOfStutters"
      - name: unused-parameter
        disabled: true  # Covered by unparam
  gocritic:
    enabled-tags:
      - diagnostic
      - style
      - performance
    disabled-checks:
      - ifElseChain  # Sometimes if-else is clearer than switch
      - hugeParam    # Too aggressive for DTOs

issues:
  exclude-rules:
    # Allow dot imports in test files (for testify etc.)
    - path: _test\.go
      linters:
        - revive
      text: "dot-imports"
    # Ignore exported function comment requirements in test files
    - path: _test\.go
      linters:
        - revive
      text: "exported"
    # Allow unused parameters in interface implementations that are stubs/mocks
    - path: _test\.go
      linters:
        - unparam
    # Swagger docs file is auto-generated
    - path: internal/docs/
      linters:
        - goimports
        - gofmt
        - misspell
  max-issues-per-linter: 0
  max-same-issues: 0
```

### 2.2 Create Verification Script

**New file**: `apps/api/scripts/check.sh`

A single script that runs all quality checks in sequence. This is the primary feedback loop for both developers and agentic tools. If this script passes, the change is good.

```bash
#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

echo "=== Step 1/5: go build ==="
go build ./...

echo "=== Step 2/5: go vet ==="
go vet ./...

echo "=== Step 3/5: golangci-lint ==="
golangci-lint run ./...

echo "=== Step 4/5: go test ==="
go test ./... -count=1 -timeout 120s

echo "=== Step 5/5: go test -race ==="
go test ./... -race -count=1 -timeout 120s

echo ""
echo "All checks passed."
```

Make it executable: `chmod +x apps/api/scripts/check.sh`

This script is the **single command** an agent should run after making changes:
```bash
cd apps/api && ./scripts/check.sh
```

### 2.3 Update Makefile with Check Targets

The Makefile (created in Phase 14) should include these targets. Define them here so they can be used immediately if the Makefile is created early:

```makefile
# Quick check: compile + vet + lint (fast, no tests)
check-fast:
	go build ./...
	go vet ./...
	golangci-lint run ./...

# Full check: compile + vet + lint + test (complete verification)
check:
	./scripts/check.sh

# Format all Go files
fmt:
	gofmt -w .
	goimports -w .
```

### 2.4 Agentic Development Protocol

When an agent (Claude, Cursor, Copilot, etc.) is implementing any phase of this plan, it MUST follow this protocol:

1. **Before starting a phase**: Run `go build ./...` to confirm the codebase compiles.
2. **After each logical unit of work** (e.g., after creating a new file, after refactoring a service): Run `go build ./...` to catch compile errors immediately. Do NOT batch up multiple files and hope they all work.
3. **After completing a phase**: Run `./scripts/check.sh` (or the individual steps if the script doesn't exist yet). Fix ALL errors before moving to the next phase.
4. **If tests exist**: Run `go test ./... -count=1` after any change to service/handler code. Never skip tests.
5. **If lint fails**: Fix the lint issues. Do not add `//nolint` directives unless there is a genuinely good reason (document it in a comment).

The feedback loop order of priority:
1. `go build ./...` — must pass (compilation)
2. `go vet ./...` — must pass (correctness)
3. `golangci-lint run ./...` — must pass (code quality)
4. `go test ./... -count=1` — must pass (behavior)

If step 1 fails, do NOT proceed to step 2. Fix compilation first.

### 2.5 Initial Lint Fixes

After installing `golangci-lint` and creating the config, run it against the current codebase:

```bash
cd apps/api && golangci-lint run ./...
```

Fix all reported issues. Common issues expected:
- Unused variables or imports (from dead code that survived Phase 1 cleanup)
- Unhandled errors (`errcheck`)
- Ineffective assignments (`ineffassign`)
- Missing godoc comments on exported types (`revive`)

Do NOT suppress these with `//nolint`. Fix them properly.

### 2.6 Verification

- `go build ./...` passes
- `go vet ./...` passes
- `golangci-lint run ./...` passes with zero issues
- `.golangci.yml` exists and configures all listed linters
- `scripts/check.sh` exists, is executable, and runs all checks in sequence
- Running `scripts/check.sh` exits 0

---

## Phase 3: Custom Error Handling

**Goal**: Consistent, secure error responses. Never leak internal errors to clients.

### 3.1 Create AppError Type

**New file**: `internal/errors/errors.go`

```go
package errors

type AppError struct {
    Code       string `json:"code"`       // Machine-readable: "ACCOUNT_NOT_FOUND"
    Message    string `json:"message"`    // Human-readable: "Account not found"
    StatusCode int    `json:"-"`          // HTTP status code (not serialized)
    Internal   error  `json:"-"`          // Original error (not serialized, logged only)
}

func (e *AppError) Error() string { return e.Message }
func (e *AppError) Unwrap() error { return e.Internal }

// Wrap creates a new AppError with the same code/message/status but wraps an internal error
func Wrap(sentinel *AppError, internal error) *AppError {
    return &AppError{
        Code:       sentinel.Code,
        Message:    sentinel.Message,
        StatusCode: sentinel.StatusCode,
        Internal:   internal,
    }
}

// WithMessage creates a new AppError with a custom message
func WithMessage(sentinel *AppError, message string) *AppError {
    return &AppError{
        Code:       sentinel.Code,
        Message:    message,
        StatusCode: sentinel.StatusCode,
        Internal:   sentinel.Internal,
    }
}
```

**Predefined sentinel errors** (define all in the same file):

| Variable | Code | HTTP Status | Message |
|---|---|---|---|
| `ErrUnauthorized` | `UNAUTHORIZED` | 401 | "Authentication required" |
| `ErrInvalidCredentials` | `INVALID_CREDENTIALS` | 401 | "Invalid email or password" |
| `ErrForbidden` | `FORBIDDEN` | 403 | "Access denied" |
| `ErrAccountLocked` | `ACCOUNT_LOCKED` | 423 | "Account is temporarily locked" |
| `ErrInvalidInput` | `INVALID_INPUT` | 400 | "Invalid input" |
| `ErrNotFound` | `NOT_FOUND` | 404 | "Resource not found" |
| `ErrUserNotFound` | `USER_NOT_FOUND` | 404 | "User not found" |
| `ErrDuplicateEmail` | `DUPLICATE_EMAIL` | 409 | "A user with this email already exists" |
| `ErrAccountNotFound` | `ACCOUNT_NOT_FOUND` | 404 | "Account not found" |
| `ErrNotCashAccount` | `NOT_CASH_ACCOUNT` | 400 | "Operation only allowed on cash accounts" |
| `ErrCategoryNotFound` | `CATEGORY_NOT_FOUND` | 404 | "Category not found" |
| `ErrCategoryInUse` | `CATEGORY_IN_USE` | 409 | "Category is used by existing transactions" |
| `ErrCategoryHasChildren` | `CATEGORY_HAS_CHILDREN` | 409 | "Category has child categories" |
| `ErrSelfParentCategory` | `SELF_PARENT_CATEGORY` | 400 | "A category cannot be its own parent" |
| `ErrTransactionNotFound` | `TRANSACTION_NOT_FOUND` | 404 | "Transaction not found" |
| `ErrInvalidTransactionType` | `INVALID_TRANSACTION_TYPE` | 400 | "Unsupported transaction type" |
| `ErrInsufficientBalance` | `INSUFFICIENT_BALANCE` | 400 | "Insufficient account balance" |
| `ErrSameAccountTransfer` | `SAME_ACCOUNT_TRANSFER` | 400 | "Cannot transfer to the same account" |
| `ErrBudgetNotFound` | `BUDGET_NOT_FOUND` | 404 | "Budget not found" |
| `ErrInvestmentNotFound` | `INVESTMENT_NOT_FOUND` | 404 | "Investment not found" |
| `ErrInsufficientShares` | `INSUFFICIENT_SHARES` | 400 | "Insufficient shares for this sale" |
| `ErrInternalServer` | `INTERNAL_ERROR` | 500 | "An internal error occurred" |

### 3.2 Create Error Handling Middleware

**New file**: `internal/middleware/error.go`

Gin middleware that:
1. Calls `c.Next()` to execute the handler chain
2. After handler execution, checks if there are any errors in the Gin context
3. If the error is an `*AppError`: responds with `{"error": {"code": "...", "message": "..."}}`
4. If the error is unexpected: logs the full error with Zap, responds with generic `ErrInternalServer`

Register this middleware on the router in `main.go`.

### 3.3 Refactor All Services

Replace every `errors.New("some message")` return with the appropriate `AppError` sentinel.

**Files to update**:
- `services/user_service.go`: Replace "email and password are required" -> `ErrInvalidInput`, "user with this email already exists" -> `ErrDuplicateEmail`, "user not found" -> `ErrUserNotFound`
- `services/account_service.go`: Replace "account name is required" -> `ErrInvalidInput`, "account not found" -> `ErrAccountNotFound`, "not a cash account" -> `ErrNotCashAccount`
- `services/category_service.go`: Replace "category name is required" -> `ErrInvalidInput`, "category with this name already exists" -> `WithMessage(ErrInvalidInput, "...")`, "parent category not found" -> `ErrCategoryNotFound`, "category not found" -> `ErrCategoryNotFound`, "category cannot be its own parent" -> `ErrSelfParentCategory`, "cannot delete category with child categories" -> `ErrCategoryHasChildren`, "cannot delete category that is used by transactions" -> `ErrCategoryInUse`
- `services/transaction_service.go`: Replace "amount must be greater than zero" -> `ErrInvalidInput`, "account ID is required" -> `ErrInvalidInput`, "transaction not found" -> `ErrTransactionNotFound`, "unsupported transaction type for deletion" -> `ErrInvalidTransactionType`

When wrapping GORM errors, use `Wrap(sentinel, gormErr)` so the internal error is logged but not exposed.

### 3.4 Refactor All Handlers

Replace all `c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})` patterns.

Handlers should check if the error is an `*AppError` using `errors.As()`:
```go
var appErr *apperrors.AppError
if errors.As(err, &appErr) {
    c.JSON(appErr.StatusCode, gin.H{"error": gin.H{"code": appErr.Code, "message": appErr.Message}})
    return
}
// Unexpected error
logger.Get().Errorw("unexpected error", "error", err)
c.JSON(500, gin.H{"error": gin.H{"code": "INTERNAL_ERROR", "message": "An internal error occurred"}})
```

Alternatively, if using the error middleware from 2.2, handlers can set errors on the context and let middleware handle formatting.

**Files to update**: `auth_handler.go`, `account_handler.go`, `category_handler.go`, `transaction_handler.go`

### 3.5 Verification

- `./scripts/check.sh` passes
- No handler returns `err.Error()` directly to clients
- All error responses follow the format `{"error": {"code": "...", "message": "..."}}`
- Internal/GORM errors are logged but return generic messages to clients

---

## Phase 4: Input Validation

**Goal**: Reject invalid input at the handler boundary with clear error messages.

### 4.1 Register Custom Gin Validators

**New file**: `internal/validator/validator.go`

Create and register custom validation functions with Gin's validator engine:

| Tag | Logic |
|---|---|
| `iso4217` | Check value against list of valid currency codes (USD, EUR, GBP, INR, JPY, etc.) |
| `hex_color` | Match regex `^#([0-9a-fA-F]{3}\|[0-9a-fA-F]{6})$` |
| `transaction_type` | Value must be one of: "income", "expense", "transfer", "investment" |
| `category_type` | Value must be one of: "income", "expense" |
| `account_type` | Value must be one of: "cash", "investment", "debt" |
| `budget_period` | Value must be one of: "monthly", "yearly" |
| `asset_type` | Value must be one of: "stock", "etf", "bond", "crypto", "reit" |

Call `validator.Register()` from `main.go` before setting up routes.

### 4.2 Update All Request DTOs

**`handlers/auth_handler.go`**:
```go
type RegisterRequest struct {
    Email     string `json:"email" binding:"required,email,max=255"`
    Password  string `json:"password" binding:"required,min=8,max=128"`
    FirstName string `json:"first_name" binding:"max=100"`
    LastName  string `json:"last_name" binding:"max=100"`
}
```

**`handlers/account_handler.go`**:
```go
type CreateCashAccountRequest struct {
    Name           string `json:"name" binding:"required,min=1,max=100"`
    Description    string `json:"description" binding:"max=500"`
    Currency       string `json:"currency" binding:"omitempty,iso4217"`
    InitialBalance int64  `json:"initial_balance" binding:"gte=0"`
}

type UpdateCashAccountRequest struct {
    Name        string `json:"name" binding:"omitempty,min=1,max=100"`
    Description string `json:"description" binding:"max=500"`
}
```

**`handlers/category_handler.go`**:
```go
type CreateCategoryRequest struct {
    Name        string              `json:"name" binding:"required,min=1,max=100"`
    Type        models.CategoryType `json:"type" binding:"required,category_type"`
    Description string              `json:"description" binding:"max=500"`
    Icon        string              `json:"icon" binding:"max=50"`
    Color       string              `json:"color" binding:"omitempty,hex_color"`
    ParentID    *uint               `json:"parent_id"`
}

type UpdateCategoryRequest struct {
    Name        string `json:"name" binding:"omitempty,min=1,max=100"`
    Description string `json:"description" binding:"max=500"`
    Icon        string `json:"icon" binding:"max=50"`
    Color       string `json:"color" binding:"omitempty,hex_color"`
    ParentID    *uint  `json:"parent_id"`
}
```

**`handlers/transaction_handler.go`**:
```go
type CreateTransactionRequest struct {
    AccountID   uint                   `json:"account_id" binding:"required"`
    CategoryID  *uint                  `json:"category_id"`
    Type        models.TransactionType `json:"type" binding:"required,transaction_type"`
    Amount      int64                  `json:"amount" binding:"required,gt=0"`
    Description string                 `json:"description" binding:"max=500"`
    Date        *time.Time             `json:"date"`
}
```

### 4.3 Validate Query Parameters

**File**: `handlers/category_handler.go`, `GetUserCategories` method

Currently `categoryType := c.Query("type")` is passed directly to the service with no validation. If someone sends `?type=invalid`, it silently returns empty results.

Fix: Validate the query param is either empty, "income", or "expense". Return `400 INVALID_INPUT` for anything else.

### 4.4 Verification

- `./scripts/check.sh` passes
- Requests with missing required fields return 400 with clear validation errors
- Requests with values exceeding length limits are rejected
- Invalid enum values are rejected
- Invalid currency codes are rejected

---

## Phase 5: float64 to int64 Cents Conversion

**Goal**: Eliminate floating-point rounding errors for all monetary values.

### 5.1 Model Changes

Change these fields from `float64` to `int64`:

| Model | Field |
|---|---|
| `Account` | `Balance` |
| `Transaction` | `Amount` |
| `Budget` | `Amount` |
| `Investment` | `CostBasis` |
| `Investment` | `CurrentPrice` |
| `InvestmentTransaction` | `PricePerUnit` |
| `InvestmentTransaction` | `TotalAmount` |
| `InvestmentTransaction` | `Fee` |

**Fields that remain float64** (not monetary):
| Model | Field | Reason |
|---|---|---|
| `Investment` | `Quantity` | Fractional shares |
| `Investment` | `YieldToMaturity` | Percentage |
| `Investment` | `CouponRate` | Percentage |
| `InvestmentTransaction` | `Quantity` | Fractional shares |
| `InvestmentTransaction` | `SplitRatio` | Ratio |
| `Account` | `InterestRate` | Percentage |

### 5.2 Update GORM Tags

Change `gorm:"not null;default:0"` on Balance from implying float to explicitly `gorm:"type:bigint;not null;default:0"`.

### 5.3 Update All Services

Every method that accepts or returns monetary values must use `int64`:

- `AccountService.CreateCashAccount(... initialBalance int64)`
- `AccountService.UpdateAccountBalance(... amount int64)`
- `TransactionService.CreateTransaction(... amount int64)`
- `TransactionService.CreateTransfer(... amount int64)` (Phase 9)
- `BudgetService.CreateBudget(... amount int64)` (Phase 10)
- All investment service methods (Phase 11)

All arithmetic in services operates on `int64`. No conversions needed.

### 5.4 Update All Handlers/DTOs

Request DTOs change `float64` fields to `int64`. Response DTOs similarly.

### 5.5 API Contract

The API accepts and returns **cents as integers**. The frontend divides by 100 for display.

Example: Creating a $25.50 expense:
```json
POST /api/v1/transactions
{"account_id": 1, "type": "expense", "amount": 2550, "description": "Lunch"}
```

### 5.6 Database Migration

A migration file will handle converting existing float data to int cents:

```sql
-- 000010_convert_money_to_cents.up.sql
ALTER TABLE accounts ALTER COLUMN balance TYPE bigint USING (balance * 100)::bigint;
ALTER TABLE transactions ALTER COLUMN amount TYPE bigint USING (amount * 100)::bigint;
ALTER TABLE budgets ALTER COLUMN amount TYPE bigint USING (amount * 100)::bigint;
ALTER TABLE investments ALTER COLUMN cost_basis TYPE bigint USING (cost_basis * 100)::bigint;
ALTER TABLE investments ALTER COLUMN current_price TYPE bigint USING (current_price * 100)::bigint;
ALTER TABLE investment_transactions ALTER COLUMN price_per_unit TYPE bigint USING (price_per_unit * 100)::bigint;
ALTER TABLE investment_transactions ALTER COLUMN total_amount TYPE bigint USING (total_amount * 100)::bigint;
ALTER TABLE investment_transactions ALTER COLUMN fee TYPE bigint USING (fee * 100)::bigint;
```

### 5.7 Verification

- `./scripts/check.sh` passes
- Zero `float64` fields remain for monetary values in models
- All services/handlers use `int64` for money
- Existing balance update logic (add/subtract) works correctly with int64

---

## Phase 6: Service Interfaces & Handler Refactoring

**Goal**: Testable service layer, reduced handler boilerplate.

### 6.1 Define Service Interfaces

**New file**: `internal/services/interfaces.go`

Define an interface for each service. The interface includes all public methods.

```go
type UserServicer interface {
    CreateUser(email, password, firstName, lastName string) (*models.User, error)
    GetUserByEmail(email string) (*models.User, error)
    GetUserByID(id uint) (*models.User, error)
    VerifyPassword(user *models.User, password string) bool
}

type AccountServicer interface {
    CreateCashAccount(userID uint, name, description, currency string, initialBalance int64) (*models.Account, error)
    CreateInvestmentAccount(userID uint, name, description, currency, broker, accountNumber string) (*models.Account, error)
    GetUserAccounts(userID uint) ([]models.Account, error)
    GetAccountByID(userID, accountID uint) (*models.Account, error)
    UpdateCashAccount(userID, accountID uint, name, description string) (*models.Account, error)
    UpdateAccountBalance(tx *gorm.DB, account *models.Account, transactionType models.TransactionType, amount int64) error
}

type CategoryServicer interface {
    CreateCategory(userID uint, name string, categoryType models.CategoryType, description, icon, color string, parentID *uint) (*models.Category, error)
    GetUserCategories(userID uint) ([]models.Category, error)
    GetUserCategoriesByType(userID uint, categoryType models.CategoryType) ([]models.Category, error)
    GetCategoryByID(userID, categoryID uint) (*models.Category, error)
    UpdateCategory(userID, categoryID uint, name, description, icon, color string, parentID *uint) (*models.Category, error)
    DeleteCategory(userID, categoryID uint) error
}

type TransactionServicer interface {
    CreateTransaction(userID, accountID uint, categoryID *uint, transactionType models.TransactionType, amount int64, description string, date time.Time) (*models.Transaction, error)
    CreateTransfer(userID, fromAccountID, toAccountID uint, amount int64, description string, date time.Time) (*models.Transaction, error)
    GetAccountTransactions(userID, accountID uint) ([]models.Transaction, error)
    GetTransactionByID(userID, transactionID uint) (*models.Transaction, error)
    DeleteTransaction(userID, transactionID uint) error
}

type BudgetServicer interface {
    CreateBudget(userID, categoryID uint, name string, amount int64, period models.BudgetPeriod, startDate time.Time, endDate *time.Time) (*models.Budget, error)
    GetUserBudgets(userID uint) ([]models.Budget, error)
    GetBudgetByID(userID, budgetID uint) (*models.Budget, error)
    UpdateBudget(userID, budgetID uint, name string, amount int64, period models.BudgetPeriod, endDate *time.Time) (*models.Budget, error)
    DeleteBudget(userID, budgetID uint) error
    GetBudgetProgress(userID, budgetID uint) (*BudgetProgress, error)
}

type InvestmentServicer interface {
    // Defined fully in Phase 11
}

type AuditServicer interface {
    Log(userID uint, action, resourceType string, resourceID uint, ipAddress string, changes map[string]interface{}) error
}
```

Note: Method signatures will include pagination parameters once Phase 8 is implemented. The interfaces should be updated at that point.

### 6.2 Rename Implementations

Rename concrete service structs to unexported names:
- `UserService` -> `userService`
- `AccountService` -> `accountService`
- etc.

Update constructors to return interfaces:
```go
func NewUserService(db *gorm.DB) UserServicer {
    return &userService{db: db}
}
```

### 6.3 Update Handler Constructors

All handlers accept interfaces instead of concrete types:

```go
type AuthHandler struct {
    userService services.UserServicer
}

func NewAuthHandler(userService services.UserServicer) *AuthHandler {
    return &AuthHandler{userService: userService}
}
```

Update all 4 existing handlers: `AuthHandler`, `AccountHandler`, `CategoryHandler`, `TransactionHandler`.

### 6.4 Extract Handler Helpers

**New file**: `internal/handlers/helpers.go`

```go
// getUserID extracts the authenticated user ID from the Gin context.
// Returns ErrUnauthorized if not present.
func getUserID(c *gin.Context) (uint, error) {
    userID, exists := c.Get("userID")
    if !exists {
        return 0, apperrors.ErrUnauthorized
    }
    return userID.(uint), nil
}

// parsePathID parses a uint path parameter.
// Returns ErrInvalidInput if the parameter is not a valid positive integer.
func parsePathID(c *gin.Context, param string) (uint, error) {
    id, err := strconv.ParseUint(c.Param(param), 10, 32)
    if err != nil {
        return 0, apperrors.WithMessage(apperrors.ErrInvalidInput, "Invalid "+param)
    }
    return uint(id), nil
}
```

Replace all 12+ occurrences of the `userID, exists := c.Get("userID")` block and all 10+ occurrences of the `strconv.ParseUint(c.Param("id"), 10, 32)` block across all handlers.

### 6.5 Verification

- All handlers use interfaces for service dependencies
- Helper functions eliminate duplicated boilerplate
- `go build ./cmd/api` succeeds
- `./scripts/check.sh` passes
- Handlers are significantly shorter and cleaner

---

## Phase 7: Security Improvements

**Goal**: Production-grade authentication, login security, audit trail.

### 7.1 JWT Secret Enforcement

**File**: `internal/config/config.go`

Add validation after loading config:
```go
if config.Env == "production" {
    if config.JWTSecret == "" || config.JWTSecret == "fallback-secret-key-for-dev-only" || config.JWTSecret == "your-super-secret-key-change-in-production" {
        return nil, fmt.Errorf("JWT_SECRET must be explicitly set in production")
    }
    if config.DBPassword == "kuberan" {
        return nil, fmt.Errorf("DB_PASSWORD must not be the default in production")
    }
}
```

### 7.2 Token Refresh

**Model change**: Add `RefreshTokenHash string` field to `User` model.

**New endpoint**: `POST /api/v1/auth/refresh`

**Login flow change**:
- Login now returns both `access_token` (15min expiry) and `refresh_token` (7d expiry)
- `refresh_token` is a separate JWT with different claims
- The hash of the refresh token is stored in `users.refresh_token_hash`

**Refresh flow**:
1. Client sends `{"refresh_token": "..."}` to `/api/v1/auth/refresh`
2. Server validates the refresh token JWT
3. Server verifies the token hash matches `users.refresh_token_hash`
4. Server generates new `access_token` + new `refresh_token`
5. Server updates `refresh_token_hash` in DB (token rotation)
6. Server returns both new tokens

**Update `middleware/auth.go`**: `GenerateToken` becomes `GenerateAccessToken`. Add `GenerateRefreshToken`. Add `ValidateRefreshToken`.

### 7.3 Login Security

**Model changes to `User`**:
```go
FailedLoginAttempts int        `gorm:"default:0" json:"-"`
LockedUntil         *time.Time `json:"-"`
LastLoginAt         *time.Time `json:"last_login_at,omitempty"`
```

**Logic in `UserService`** (or a new method `AttemptLogin`):
1. Find user by email
2. Check if `LockedUntil` is set and in the future -> return `ErrAccountLocked`
3. Verify password
4. If password wrong: increment `FailedLoginAttempts`. If attempts >= 5, set `LockedUntil = now + 15min`. Return `ErrInvalidCredentials`
5. If password correct: set `FailedLoginAttempts = 0`, `LockedUntil = nil`, `LastLoginAt = now`. Return user

### 7.4 Audit Logging

**New model**: `internal/models/audit_log.go`
```go
type AuditLog struct {
    Base
    UserID       uint   `gorm:"not null;index" json:"user_id"`
    Action       string `gorm:"not null" json:"action"`
    ResourceType string `gorm:"not null" json:"resource_type"`
    ResourceID   uint   `json:"resource_id"`
    IPAddress    string `json:"ip_address"`
    Changes      string `json:"changes,omitempty"` // JSON string
}
```

**New service**: `internal/services/audit_service.go`

Method: `Log(userID uint, action, resourceType string, resourceID uint, ipAddress string, changes map[string]interface{}) error`

Serializes `changes` to JSON string and inserts an `AuditLog` record. This should be fire-and-forget (log errors but don't fail the main operation).

**Actions to audit**:
- `LOGIN`, `LOGIN_FAILED`, `REGISTER`
- `CREATE_ACCOUNT`, `UPDATE_ACCOUNT`
- `CREATE_TRANSACTION`, `DELETE_TRANSACTION`, `CREATE_TRANSFER`
- `CREATE_CATEGORY`, `UPDATE_CATEGORY`, `DELETE_CATEGORY`
- `CREATE_BUDGET`, `UPDATE_BUDGET`, `DELETE_BUDGET`
- `CREATE_INVESTMENT`, `UPDATE_INVESTMENT_PRICE`
- `INVESTMENT_BUY`, `INVESTMENT_SELL`, `INVESTMENT_DIVIDEND`, `INVESTMENT_SPLIT`

### 7.5 Verification

- `./scripts/check.sh` passes
- Login with wrong password 5 times -> account locked for 15min
- Login with correct password resets failed attempts
- Refresh token endpoint works and rotates tokens
- Audit logs are created for all tracked operations
- Production config rejects default secrets

---

## Phase 8: Database Migrations (golang-migrate)

**Goal**: Version-controlled, reversible schema changes.

### 8.1 Setup

**New dependency**: `github.com/golang-migrate/migrate/v4`

**New directory**: `apps/api/migrations/`

### 8.2 Migration Files

Create SQL migration files for the full schema. Each migration is a pair of `.up.sql` and `.down.sql` files.

```
migrations/
├── 000001_create_users.up.sql
├── 000001_create_users.down.sql
├── 000002_create_accounts.up.sql
├── 000002_create_accounts.down.sql
├── 000003_create_categories.up.sql
├── 000003_create_categories.down.sql
├── 000004_create_transactions.up.sql
├── 000004_create_transactions.down.sql
├── 000005_create_budgets.up.sql
├── 000005_create_budgets.down.sql
├── 000006_create_investments.up.sql
├── 000006_create_investments.down.sql
├── 000007_create_investment_transactions.up.sql
├── 000007_create_investment_transactions.down.sql
├── 000008_create_audit_logs.up.sql
├── 000008_create_audit_logs.down.sql
├── 000009_add_user_security_fields.up.sql
├── 000009_add_user_security_fields.down.sql
├── 000010_convert_money_to_cents.up.sql
├── 000010_convert_money_to_cents.down.sql
├── 000011_add_performance_indexes.up.sql
└── 000011_add_performance_indexes.down.sql
```

The SQL should define tables explicitly (column types, constraints, foreign keys) rather than relying on GORM tags. The GORM model tags serve as documentation but the migrations are the source of truth for schema.

**Important**: `down` migrations must reverse the `up` operations cleanly (DROP TABLE, DROP INDEX, ALTER COLUMN back, etc.).

### 8.3 Migration CLI

**New file**: `apps/api/cmd/migrate/main.go`

Standalone CLI tool:
```bash
go run cmd/migrate/main.go up          # Apply all pending migrations
go run cmd/migrate/main.go down 1      # Rollback last migration
go run cmd/migrate/main.go version     # Show current migration version
```

Reads database config from env vars / `.env` file (reuse `config` and `database` packages).

### 8.4 Remove AutoMigrate

**File**: `internal/database/database.go`

Remove the `Migrate()` method that calls `db.AutoMigrate(...)`.

**File**: `cmd/api/main.go`

Remove the `dbManager.Migrate()` call. In development mode (`ENV=development`), optionally run golang-migrate automatically on server start. In production, migrations must be run explicitly before deployment.

### 8.5 Verification

- `go run cmd/migrate/main.go up` creates all tables successfully
- `go run cmd/migrate/main.go down 1` rolls back the last migration cleanly
- `go run cmd/migrate/main.go version` shows correct version
- Server starts without AutoMigrate

---

## Phase 9: Pagination & Query Optimization

**Goal**: Scalable data retrieval, performant queries.

### 9.1 Pagination Utility

**New file**: `internal/pagination/pagination.go`

```go
type PageRequest struct {
    Page     int `form:"page" binding:"omitempty,min=1"`
    PageSize int `form:"page_size" binding:"omitempty,min=1,max=100"`
}

// Defaults returns a PageRequest with default values if not provided
func (p *PageRequest) Defaults() {
    if p.Page == 0 { p.Page = 1 }
    if p.PageSize == 0 { p.PageSize = 20 }
}

func (p *PageRequest) Offset() int {
    return (p.Page - 1) * p.PageSize
}

type PageResponse[T any] struct {
    Data       []T   `json:"data"`
    Page       int   `json:"page"`
    PageSize   int   `json:"page_size"`
    TotalItems int64 `json:"total_items"`
    TotalPages int   `json:"total_pages"`
}

// Paginate returns a GORM scope that applies offset and limit
func Paginate(req PageRequest) func(db *gorm.DB) *gorm.DB {
    return func(db *gorm.DB) *gorm.DB {
        return db.Offset(req.Offset()).Limit(req.PageSize)
    }
}
```

### 9.2 Update List Endpoints

Add pagination to all list endpoints. Service method signatures change to accept `PageRequest` and return `PageResponse`.

| Endpoint | Additional Filters |
|---|---|
| `GET /accounts` | pagination |
| `GET /accounts/:id/transactions` | pagination, `from_date`, `to_date`, `type`, `category_id`, `min_amount`, `max_amount` |
| `GET /categories` | pagination, `type` (already exists) |
| `GET /budgets` | pagination, `is_active`, `period` |
| `GET /investments` | pagination, `asset_type`, `account_id` |
| `GET /investments/:id/transactions` | pagination |

Update service interfaces (from Phase 5) to include pagination parameters.

### 9.3 Database Indexes

**Migration file**: `000011_add_performance_indexes.up.sql`

```sql
CREATE INDEX IF NOT EXISTS idx_transactions_user_date ON transactions(user_id, date DESC);
CREATE INDEX IF NOT EXISTS idx_transactions_account ON transactions(account_id);
CREATE INDEX IF NOT EXISTS idx_transactions_category ON transactions(category_id);
CREATE INDEX IF NOT EXISTS idx_accounts_user_active ON accounts(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_categories_user_type ON categories(user_id, type);
CREATE INDEX IF NOT EXISTS idx_budgets_user_active ON budgets(user_id, is_active);
CREATE INDEX IF NOT EXISTS idx_investments_account ON investments(account_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_user ON audit_logs(user_id, created_at DESC);
```

### 9.4 Database Connection Pooling

**File**: `internal/database/database.go`

After `gorm.Open(...)`:
```go
sqlDB, err := db.DB()
if err != nil {
    return nil, fmt.Errorf("failed to get underlying DB: %w", err)
}
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

### 9.5 Health Check with DB Ping

**File**: `cmd/api/main.go`

Replace the current health check:
```go
router.GET("/api/health", func(c *gin.Context) {
    sqlDB, err := db.DB()
    if err != nil || sqlDB.Ping() != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "database": "unavailable"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "ok", "database": "connected"})
})
```

### 9.6 Verification

- List endpoints accept `page` and `page_size` query params
- Default pagination is applied when params are omitted
- Response includes `total_items` and `total_pages`
- Transaction list supports date range and type filters
- Health check returns 503 when DB is down

---

## Phase 10: Transfer Transaction Implementation

**Goal**: Account-to-account transfers.

### 10.1 New Service Method

**File**: `internal/services/transaction_service.go`

New method: `CreateTransfer(userID, fromAccountID, toAccountID uint, amount int64, description string, date time.Time) (*models.Transaction, error)`

Within a single DB transaction:
1. Validate `fromAccountID != toAccountID` (return `ErrSameAccountTransfer`)
2. Verify both accounts exist and belong to user (return `ErrAccountNotFound` if not)
3. Check from-account has sufficient balance (return `ErrInsufficientBalance` if not)
4. Create transaction record: `Type = TransactionTypeTransfer`, `AccountID = fromAccountID`, `ToAccountID = &toAccountID`
5. Decrease from-account balance by `amount`
6. Increase to-account balance by `amount`

### 10.2 Update Delete Transfer Logic

**File**: `internal/services/transaction_service.go`, `DeleteTransaction` method

Currently errors on non-income/expense types:
```go
} else {
    return errors.New("unsupported transaction type for deletion")
}
```

Add handling for `TransactionTypeTransfer`:
1. Get the transaction (verify user ownership)
2. Get both accounts (from and to)
3. Within DB transaction: delete the transaction, add amount back to from-account, subtract amount from to-account

### 10.3 New Endpoint

**File**: `internal/handlers/transaction_handler.go`

New handler method: `CreateTransfer`

Request DTO:
```go
type CreateTransferRequest struct {
    FromAccountID uint       `json:"from_account_id" binding:"required"`
    ToAccountID   uint       `json:"to_account_id" binding:"required"`
    Amount        int64      `json:"amount" binding:"required,gt=0"`
    Description   string     `json:"description" binding:"max=500"`
    Date          *time.Time `json:"date"`
}
```

**Route**: `POST /api/v1/transactions/transfer` (add to protected routes in main.go)

### 10.4 Verification

- `./scripts/check.sh` passes
- Transfer between two accounts updates both balances atomically
- Transfer to same account is rejected
- Transfer with insufficient balance is rejected
- Deleting a transfer reverses both account balances
- Transfer appears in transaction history for both accounts

---

## Phase 11: Budget Feature

**Goal**: Complete budget management with progress tracking.

### 11.1 Budget Service

**New file**: `internal/services/budget_service.go`

Methods:

**CreateBudget**: Validates category exists and belongs to user. Creates budget record.

**GetUserBudgets**: Returns paginated list of budgets for the user. Supports filtering by `is_active` and `period`.

**GetBudgetByID**: Returns budget if it exists and belongs to user.

**UpdateBudget**: Updates budget fields. Only updatable: `name`, `amount`, `period`, `end_date`, `is_active`.

**DeleteBudget**: Soft-deletes the budget.

**GetBudgetProgress**: The key analytics method.
1. Get the budget (verify user ownership)
2. Determine the current period window:
   - Monthly: from 1st of current month to last day of current month
   - Yearly: from Jan 1 to Dec 31 of current year
3. Query `SUM(amount)` from `transactions` WHERE `category_id = budget.category_id AND user_id = budget.user_id AND date BETWEEN period_start AND period_end AND type = 'expense'`
4. Return:
```go
type BudgetProgress struct {
    BudgetID    uint  `json:"budget_id"`
    Budgeted    int64 `json:"budgeted"`      // Budget amount in cents
    Spent       int64 `json:"spent"`         // Total spent in cents
    Remaining   int64 `json:"remaining"`     // Budgeted - Spent
    Percentage  float64 `json:"percentage"`  // (Spent / Budgeted) * 100
}
```

### 11.2 Budget Handler

**New file**: `internal/handlers/budget_handler.go`

Request DTOs:
```go
type CreateBudgetRequest struct {
    CategoryID uint               `json:"category_id" binding:"required"`
    Name       string             `json:"name" binding:"required,min=1,max=100"`
    Amount     int64              `json:"amount" binding:"required,gt=0"`
    Period     models.BudgetPeriod `json:"period" binding:"required,budget_period"`
    StartDate  time.Time          `json:"start_date" binding:"required"`
    EndDate    *time.Time         `json:"end_date"`
}

type UpdateBudgetRequest struct {
    Name    string              `json:"name" binding:"omitempty,min=1,max=100"`
    Amount  int64               `json:"amount" binding:"omitempty,gt=0"`
    Period  models.BudgetPeriod `json:"period" binding:"omitempty,budget_period"`
    EndDate *time.Time          `json:"end_date"`
}
```

**Routes** (add to protected routes in main.go):
```
POST   /api/v1/budgets
GET    /api/v1/budgets
GET    /api/v1/budgets/:id
PUT    /api/v1/budgets/:id
DELETE /api/v1/budgets/:id
GET    /api/v1/budgets/:id/progress
```

### 11.3 Verification

- `./scripts/check.sh` passes
- CRUD operations work for budgets
- Budget progress correctly sums transactions for the category within the period
- Deleting a budget is a soft delete
- Invalid category ID is rejected
- Pagination works on budget list

---

## Phase 12: Investment Feature

**Goal**: Complete investment tracking with buy/sell/dividend/split support.

### 12.1 Investment Service

**New file**: `internal/services/investment_service.go`

**Account management:**

`CreateInvestmentAccount(userID uint, name, description, currency, broker, accountNumber string) (*models.Account, error)`:
- Creates an Account with `Type = AccountTypeInvestment`
- Sets `Broker` and `AccountNumber` fields

**Holdings:**

`AddInvestment(userID, accountID uint, symbol, name string, assetType models.AssetType, quantity float64, purchasePrice int64, currency string, extraFields map[string]interface{}) (*models.Investment, error)`:
- Verify account exists, belongs to user, and is an investment account
- Create Investment record with initial `CostBasis = int64(quantity * float64(purchasePrice))`
- Create corresponding InvestmentTransaction (type = buy)
- `extraFields` handles asset-type-specific fields (exchange, maturity_date, network, etc.)

`GetAccountInvestments(userID, accountID uint, page PageRequest) (*PageResponse[models.Investment], error)`:
- Verify account ownership, return paginated holdings

`GetInvestmentByID(userID, investmentID uint) (*models.Investment, error)`:
- Get investment, verify the parent account belongs to the user

`UpdateInvestmentPrice(userID, investmentID uint, currentPrice int64) (*models.Investment, error)`:
- Verify ownership
- Update `CurrentPrice` and `LastUpdated = time.Now()`
- Recalculate parent account's investment balance

`GetPortfolio(userID uint) (*PortfolioSummary, error)`:
- Query all investments across all investment accounts for the user
- Calculate total value (`SUM(quantity * current_price)`), total cost basis, total gain/loss, gain/loss percentage
- Group by asset type

**Transactions:**

`RecordBuy(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)`:
- Within DB transaction:
  1. Create InvestmentTransaction record (type = buy)
  2. Update Investment: `Quantity += quantity`, `CostBasis += (quantity * pricePerUnit) + fee`
  3. No account balance change (money moves from cash to investment - handled externally if needed)

`RecordSell(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)`:
- Verify `quantity <= investment.Quantity` (return `ErrInsufficientShares` if not)
- Within DB transaction:
  1. Create InvestmentTransaction record (type = sell)
  2. Adjust CostBasis proportionally: `costBasisReduction = investment.CostBasis * (quantity / investment.Quantity)`
  3. Update Investment: `Quantity -= quantity`, `CostBasis -= costBasisReduction`

`RecordDividend(userID, investmentID uint, date time.Time, amount int64, dividendType string, notes string) (*models.InvestmentTransaction, error)`:
- Create InvestmentTransaction record (type = dividend)
- No quantity or cost basis change
- `TotalAmount = amount`

`RecordSplit(userID, investmentID uint, date time.Time, splitRatio float64, notes string) (*models.InvestmentTransaction, error)`:
- Create InvestmentTransaction record (type = split, `SplitRatio = splitRatio`)
- Update Investment: `Quantity *= splitRatio`
- Cost basis does not change (same total investment, more shares)

`GetInvestmentTransactions(userID, investmentID uint, page PageRequest) (*PageResponse[models.InvestmentTransaction], error)`:
- Verify ownership, return paginated transaction history

### 12.2 Investment Handler

**New file**: `internal/handlers/investment_handler.go`

Request DTOs:
```go
type CreateInvestmentAccountRequest struct {
    Name          string `json:"name" binding:"required,min=1,max=100"`
    Description   string `json:"description" binding:"max=500"`
    Currency      string `json:"currency" binding:"omitempty,iso4217"`
    Broker        string `json:"broker" binding:"max=100"`
    AccountNumber string `json:"account_number" binding:"max=50"`
}

type AddInvestmentRequest struct {
    AccountID     uint             `json:"account_id" binding:"required"`
    Symbol        string           `json:"symbol" binding:"required,min=1,max=20"`
    Name          string           `json:"name" binding:"required,min=1,max=200"`
    AssetType     models.AssetType `json:"asset_type" binding:"required,asset_type"`
    Quantity      float64          `json:"quantity" binding:"required,gt=0"`
    PurchasePrice int64            `json:"purchase_price" binding:"required,gt=0"`
    Currency      string           `json:"currency" binding:"omitempty,iso4217"`
    // Asset-type-specific optional fields
    Exchange        string     `json:"exchange,omitempty"`
    MaturityDate    *time.Time `json:"maturity_date,omitempty"`
    YieldToMaturity float64    `json:"yield_to_maturity,omitempty"`
    CouponRate      float64    `json:"coupon_rate,omitempty"`
    Network         string     `json:"network,omitempty"`
    WalletAddress   string     `json:"wallet_address,omitempty"`
    PropertyType    string     `json:"property_type,omitempty"`
}

type UpdatePriceRequest struct {
    CurrentPrice int64 `json:"current_price" binding:"required,gt=0"`
}

type RecordInvestmentTransactionRequest struct {
    Type         models.InvestmentTransactionType `json:"type" binding:"required"`
    Date         time.Time                        `json:"date" binding:"required"`
    Quantity     float64                          `json:"quantity" binding:"required_unless=Type dividend,gt=0"`
    PricePerUnit int64                            `json:"price_per_unit" binding:"required_unless=Type split Type dividend"`
    Fee          int64                            `json:"fee" binding:"gte=0"`
    Notes        string                           `json:"notes" binding:"max=500"`
    SplitRatio   float64                          `json:"split_ratio,omitempty"`
    DividendType string                           `json:"dividend_type,omitempty"`
}
```

**Routes** (add to protected routes in main.go):
```
POST   /api/v1/accounts/investment
GET    /api/v1/investments
GET    /api/v1/investments/portfolio    (must be before /:id to avoid route conflict)
GET    /api/v1/investments/:id
PUT    /api/v1/investments/:id/price
POST   /api/v1/investments/:id/transactions
GET    /api/v1/investments/:id/transactions
```

### 12.3 Portfolio Response Shape

```go
type PortfolioSummary struct {
    TotalValue      int64                          `json:"total_value"`
    TotalCostBasis  int64                          `json:"total_cost_basis"`
    TotalGainLoss   int64                          `json:"total_gain_loss"`
    GainLossPct     float64                        `json:"gain_loss_pct"`
    HoldingsByType  map[models.AssetType]TypeSummary `json:"holdings_by_type"`
}

type TypeSummary struct {
    Value int64 `json:"value"`
    Count int   `json:"count"`
}
```

### 12.4 Verification

- `./scripts/check.sh` passes
- Investment account creation works
- Adding a holding creates the investment and an initial buy transaction
- Buy increases quantity and cost basis
- Sell decreases quantity and adjusts cost basis proportionally
- Selling more than held is rejected
- Dividend creates transaction without changing quantity
- Split multiplies quantity without changing cost basis
- Portfolio summary aggregates correctly across accounts
- Price update changes current price and last updated timestamp

---

## Phase 13: Testing

**Goal**: Comprehensive test coverage (80% overall, 95% on critical paths).

### 13.1 Test Infrastructure

**New file**: `internal/testutil/database.go`
```go
// SetupTestDB creates an in-memory SQLite database with all models migrated.
// Returns a *gorm.DB instance ready for testing.
func SetupTestDB(t *testing.T) *gorm.DB

// TeardownTestDB closes the database connection.
func TeardownTestDB(t *testing.T, db *gorm.DB)
```

Uses `gorm.io/driver/sqlite` with `file::memory:?cache=shared`. Runs `db.AutoMigrate(...)` for all models (AutoMigrate is fine for tests).

**New file**: `internal/testutil/fixtures.go`
```go
// Factory functions that create test data with sensible defaults.
// All return the created model instance.

func CreateTestUser(t *testing.T, db *gorm.DB) *models.User
func CreateTestUserWithEmail(t *testing.T, db *gorm.DB, email string) *models.User
func CreateTestCashAccount(t *testing.T, db *gorm.DB, userID uint) *models.Account
func CreateTestCashAccountWithBalance(t *testing.T, db *gorm.DB, userID uint, balance int64) *models.Account
func CreateTestInvestmentAccount(t *testing.T, db *gorm.DB, userID uint) *models.Account
func CreateTestCategory(t *testing.T, db *gorm.DB, userID uint, categoryType models.CategoryType) *models.Category
func CreateTestTransaction(t *testing.T, db *gorm.DB, userID, accountID uint, txType models.TransactionType, amount int64) *models.Transaction
func CreateTestBudget(t *testing.T, db *gorm.DB, userID, categoryID uint) *models.Budget
func CreateTestInvestment(t *testing.T, db *gorm.DB, accountID uint) *models.Investment
```

**New file**: `internal/testutil/assertions.go`
```go
// AssertAppError checks that an error is an AppError with the expected code.
func AssertAppError(t *testing.T, err error, expectedCode string)

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error)
```

**New dependency**: `gorm.io/driver/sqlite`

### 13.2 Service Tests (Table-Driven)

Create one test file per service. Each uses `testutil.SetupTestDB()` for an isolated in-memory DB.

**`internal/services/user_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateUser/valid` | Creates user successfully |
| `TestCreateUser/duplicate_email` | Returns `ErrDuplicateEmail` |
| `TestCreateUser/empty_email` | Returns `ErrInvalidInput` |
| `TestCreateUser/empty_password` | Returns `ErrInvalidInput` |
| `TestCreateUser/email_normalized` | Email stored as lowercase |
| `TestGetUserByEmail/found` | Returns user |
| `TestGetUserByEmail/not_found` | Returns `ErrUserNotFound` |
| `TestGetUserByEmail/inactive_user` | Returns `ErrUserNotFound` |
| `TestGetUserByID/found` | Returns user |
| `TestGetUserByID/not_found` | Returns `ErrUserNotFound` |
| `TestVerifyPassword/correct` | Returns true |
| `TestVerifyPassword/incorrect` | Returns false |
| `TestAttemptLogin/success` | Resets failed attempts, updates LastLoginAt |
| `TestAttemptLogin/wrong_password` | Increments failed attempts |
| `TestAttemptLogin/lockout_after_5_failures` | Sets LockedUntil |
| `TestAttemptLogin/locked_account` | Returns `ErrAccountLocked` |

**`internal/services/account_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateCashAccount/valid` | Creates account |
| `TestCreateCashAccount/with_initial_balance` | Creates account AND initial transaction atomically |
| `TestCreateCashAccount/empty_name` | Returns error |
| `TestCreateCashAccount/default_currency` | Uses USD |
| `TestGetUserAccounts/returns_user_accounts_only` | Other users' accounts not returned |
| `TestGetUserAccounts/excludes_inactive` | Inactive accounts filtered |
| `TestGetAccountByID/found` | Returns account |
| `TestGetAccountByID/not_found` | Returns `ErrAccountNotFound` |
| `TestGetAccountByID/wrong_user` | Returns `ErrAccountNotFound` |
| `TestUpdateCashAccount/valid` | Updates name/description |
| `TestUpdateCashAccount/not_cash_account` | Returns `ErrNotCashAccount` |
| `TestUpdateAccountBalance/income` | Adds to balance |
| `TestUpdateAccountBalance/expense` | Subtracts from balance |

**`internal/services/category_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateCategory/valid` | Creates category |
| `TestCreateCategory/duplicate_name` | Returns error |
| `TestCreateCategory/with_parent` | Sets parent relationship |
| `TestCreateCategory/invalid_parent` | Returns `ErrCategoryNotFound` |
| `TestGetUserCategories/returns_user_categories_only` | Isolation check |
| `TestGetUserCategoriesByType/filters_correctly` | Only matching type returned |
| `TestUpdateCategory/valid` | Updates fields |
| `TestUpdateCategory/self_parent` | Returns `ErrSelfParentCategory` |
| `TestDeleteCategory/valid` | Soft-deletes |
| `TestDeleteCategory/has_children` | Returns `ErrCategoryHasChildren` |
| `TestDeleteCategory/has_transactions` | Allowed (soft delete, categories remain as references) |

**Important note on category deletion**: Per project requirements, soft-deleted categories remain as references for existing transactions. Update the `DeleteCategory` service method to allow deletion even when transactions reference the category (remove the check that blocks this). The soft delete ensures data integrity.

**`internal/services/transaction_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateTransaction/income` | Balance increases |
| `TestCreateTransaction/expense` | Balance decreases |
| `TestCreateTransaction/zero_amount` | Returns error |
| `TestCreateTransaction/invalid_account` | Returns `ErrAccountNotFound` |
| `TestCreateTransaction/atomicity` | If balance update fails, transaction not created |
| `TestCreateTransfer/valid` | Both accounts updated |
| `TestCreateTransfer/same_account` | Returns `ErrSameAccountTransfer` |
| `TestCreateTransfer/insufficient_balance` | Returns `ErrInsufficientBalance` |
| `TestDeleteTransaction/income` | Balance reversed (decreased) |
| `TestDeleteTransaction/expense` | Balance reversed (increased) |
| `TestDeleteTransaction/transfer` | Both accounts reversed |
| `TestDeleteTransaction/not_found` | Returns `ErrTransactionNotFound` |
| `TestDeleteTransaction/wrong_user` | Returns `ErrTransactionNotFound` |

**`internal/services/budget_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateBudget/valid` | Creates budget |
| `TestCreateBudget/invalid_category` | Returns error |
| `TestGetBudgetProgress/no_spending` | Spent = 0, Remaining = Budgeted |
| `TestGetBudgetProgress/partial_spending` | Correct calculation |
| `TestGetBudgetProgress/over_budget` | Remaining is negative, percentage > 100 |
| `TestDeleteBudget/valid` | Soft-deletes |

**`internal/services/investment_service_test.go`**:
| Test Case | Description |
|---|---|
| `TestCreateInvestmentAccount/valid` | Creates account with type=investment |
| `TestAddInvestment/valid` | Creates holding + initial buy transaction |
| `TestUpdatePrice/valid` | Updates price and last_updated |
| `TestRecordBuy/valid` | Quantity and cost basis increase |
| `TestRecordSell/valid` | Quantity and cost basis decrease proportionally |
| `TestRecordSell/insufficient_shares` | Returns `ErrInsufficientShares` |
| `TestRecordDividend/valid` | Transaction created, no quantity change |
| `TestRecordSplit/valid` | Quantity multiplied, cost basis unchanged |
| `TestGetPortfolio/aggregation` | Values summed correctly across accounts |

### 13.3 Handler Tests (BDD-Style with httptest)

For each handler, create mock service implementations (structs that implement the interfaces with configurable return values).

Test pattern:
```go
func TestCreateCashAccount(t *testing.T) {
    t.Run("returns 201 on success", func(t *testing.T) {
        mockService := &mockAccountService{
            createCashAccountFn: func(...) (*models.Account, error) {
                return &models.Account{...}, nil
            },
        }
        handler := handlers.NewAccountHandler(mockService)
        router := setupTestRouter(handler)

        body := `{"name": "Savings", "currency": "USD"}`
        req := httptest.NewRequest("POST", "/api/v1/accounts/cash", strings.NewReader(body))
        req.Header.Set("Content-Type", "application/json")
        rec := httptest.NewRecorder()
        router.ServeHTTP(rec, req)

        assert.Equal(t, 201, rec.Code)
        // assert response body
    })

    t.Run("returns 400 on missing name", func(t *testing.T) { ... })
    t.Run("returns 401 without auth token", func(t *testing.T) { ... })
}
```

**Files**:
- `internal/handlers/auth_handler_test.go`
- `internal/handlers/account_handler_test.go`
- `internal/handlers/category_handler_test.go`
- `internal/handlers/transaction_handler_test.go`
- `internal/handlers/budget_handler_test.go`
- `internal/handlers/investment_handler_test.go`

### 13.4 Integration Tests

**Directory**: `apps/api/tests/integration/`

Full workflow tests using real in-memory SQLite DB. Each test sets up the full application stack (services + handlers + router) and makes HTTP requests.

**`tests/integration/auth_test.go`**:
- Register -> Login -> Access profile -> Refresh token -> Access profile with new token
- Register with duplicate email fails
- Login with wrong password -> repeated failures -> account lockout

**`tests/integration/account_flow_test.go`**:
- Create account with initial balance -> Verify initial transaction exists -> Create income -> Create expense -> Verify final balance is correct

**`tests/integration/transfer_flow_test.go`**:
- Create 2 accounts -> Transfer from A to B -> Verify A balance decreased, B balance increased -> Delete transfer -> Verify both balances restored

**`tests/integration/budget_flow_test.go`**:
- Create category -> Create budget for category -> Add expense transactions -> Check budget progress matches

**`tests/integration/investment_flow_test.go`**:
- Create investment account -> Add holding -> Record buy -> Update price -> Record sell -> Verify quantities and cost basis -> Check portfolio summary

### 13.5 Verification

- `./scripts/check.sh` passes
- `go test ./... -v` passes all tests
- `go test ./... -coverprofile=coverage.out` shows >= 80% overall coverage
- Critical paths (auth, transaction balance, transfer) have >= 95% coverage

---

## Phase 14: Polish & Final Touches

**Goal**: Production hardening and documentation.

### 14.1 Graceful Shutdown

**File**: `cmd/api/main.go`

Replace `router.Run()` with:
```go
srv := &http.Server{
    Addr:    ":" + appConfig.Port,
    Handler: router,
}

go func() {
    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        logger.Get().Fatalf("Failed to start server: %v", err)
    }
}()

quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit
logger.Get().Info("Shutting down server...")

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

if err := srv.Shutdown(ctx); err != nil {
    logger.Get().Fatalf("Server forced to shutdown: %v", err)
}

// Close database connections
sqlDB, _ := db.DB()
sqlDB.Close()

logger.Get().Info("Server exited cleanly")
```

### 14.2 Environment-Based Config

**File**: `internal/config/config.go`

Add `Env` field:
```go
type Environment string

const (
    Development Environment = "development"
    Staging     Environment = "staging"
    Production  Environment = "production"
)

type Config struct {
    Env Environment
    // ... existing fields
}
```

Load from `ENV` env var, default to `development`.

Gin mode: set `gin.SetMode(gin.ReleaseMode)` for production, `gin.SetMode(gin.DebugMode)` for development.

### 14.3 Swagger Regeneration

After all endpoint changes, regenerate docs:
```bash
swag init -g cmd/api/main.go -d . --output internal/docs
```

Ensure all new endpoints (budget, investment, transfer, refresh) have complete Swagger annotations with:
- Summary and description
- All request parameters and body schemas
- All possible response codes and schemas
- Security requirements (BearerAuth) where applicable
- Example values where helpful

### 14.4 Makefile

**New file**: `apps/api/Makefile`

```makefile
.PHONY: dev build test test-cover test-race lint check check-fast fmt migrate-up migrate-down migrate-version swagger clean

# Development
dev:
	air

build:
	go build -o bin/api ./cmd/api

# Testing
test:
	go test ./... -v -count=1 -timeout 120s

test-cover:
	go test ./... -coverprofile=coverage.out -count=1 -timeout 120s
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test ./... -race -count=1 -timeout 120s

# Code quality
lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

# Verification (primary feedback loop for agents and developers)
check-fast:
	go build ./...
	go vet ./...
	golangci-lint run ./...

check:
	./scripts/check.sh

# Database
migrate-up:
	go run cmd/migrate/main.go up

migrate-down:
	go run cmd/migrate/main.go down 1

migrate-version:
	go run cmd/migrate/main.go version

# Documentation
swagger:
	swag init -g cmd/api/main.go -d . --output internal/docs

clean:
	rm -rf bin/ tmp/ coverage.out coverage.html
```

### 14.5 Category Soft Delete Update

**File**: `internal/services/category_service.go`, `DeleteCategory` method

Currently blocks deletion if transactions reference the category. Per design decision, this should be changed to allow soft deletion. Existing transactions keep their `category_id` reference to the soft-deleted category. Remove the transaction count check.

Keep the child category check (cannot delete a category that has non-deleted children).

### 14.6 Final Verification Checklist

- [ ] `./scripts/check.sh` passes (build + vet + lint + test + race)
- [ ] `go build ./cmd/api` succeeds
- [ ] `go vet ./...` passes
- [ ] `golangci-lint run ./...` passes with zero issues
- [ ] `go test ./... -v` - all tests pass
- [ ] `go test ./... -race -count=1` - no race conditions
- [ ] `go test ./... -coverprofile=coverage.out` - coverage >= 80%
- [ ] Swagger docs generate without errors
- [ ] No `fmt.Printf`, `log.Println`, or `log.Fatalf` calls remain
- [ ] No `float64` monetary values remain
- [ ] No `errors.New("...")` in services (all use AppError)
- [ ] No `err.Error()` exposed to clients in handlers
- [ ] No duplicate model definitions
- [ ] No empty directories
- [ ] No `//nolint` directives without justification comments
- [ ] All exported functions have godoc comments
- [ ] All endpoints have Swagger annotations

---

## Final Directory Structure

```
apps/api/
├── cmd/
│   ├── api/
│   │   └── main.go
│   └── migrate/
│       └── main.go
├── migrations/
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   └── ... (11 migration pairs)
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   ├── config.go
│   │   └── database.go
│   ├── errors/
│   │   └── errors.go
│   ├── handlers/
│   │   ├── helpers.go
│   │   ├── auth_handler.go
│   │   ├── account_handler.go
│   │   ├── category_handler.go
│   │   ├── transaction_handler.go
│   │   ├── budget_handler.go
│   │   └── investment_handler.go
│   ├── logger/
│   │   └── logger.go
│   ├── middleware/
│   │   ├── auth.go
│   │   ├── error.go
│   │   └── logging.go
│   ├── models/
│   │   ├── base.go
│   │   ├── user.go
│   │   ├── account.go
│   │   ├── category.go
│   │   ├── transaction.go
│   │   ├── budget.go
│   │   ├── investment.go
│   │   └── audit_log.go
│   ├── pagination/
│   │   └── pagination.go
│   ├── services/
│   │   ├── interfaces.go
│   │   ├── user_service.go
│   │   ├── account_service.go
│   │   ├── category_service.go
│   │   ├── transaction_service.go
│   │   ├── budget_service.go
│   │   ├── investment_service.go
│   │   └── audit_service.go
│   ├── validator/
│   │   └── validator.go
│   ├── testutil/
│   │   ├── database.go
│   │   ├── fixtures.go
│   │   └── assertions.go
│   └── docs/
│       └── docs.go
├── tests/
│   └── integration/
│       ├── auth_test.go
│       ├── account_flow_test.go
│       ├── transfer_flow_test.go
│       ├── budget_flow_test.go
│       └── investment_flow_test.go
├── scripts/
│   └── check.sh
├── .golangci.yml
├── Makefile
├── air.toml
├── go.mod
└── go.sum
```

## Dependencies Added

| Package | Purpose |
|---|---|
| `go.uber.org/zap` | Structured logging |
| `github.com/golang-migrate/migrate/v4` | Database migrations |
| `gorm.io/driver/sqlite` | SQLite driver for tests |
| `github.com/google/uuid` | Request ID generation |

## Dependencies Removed

None. All existing dependencies remain.

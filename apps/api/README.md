# Kuberan API

The backend service for Kuberan, built with Go 1.24, Gin, GORM, and PostgreSQL 16.

## Architecture

3-layer architecture with interface-based services:

```
Handlers (HTTP) → Services (Business Logic) → Models (Database/GORM)
```

```
apps/api/
├── cmd/
│   ├── api/main.go           # Application entrypoint
│   └── migrate/main.go       # Migration CLI tool
├── migrations/               # SQL migration files (golang-migrate)
├── scripts/
│   └── check.sh              # Full verification (build+vet+lint+test+race)
├── Makefile                  # Dev targets
├── internal/
│   ├── config/               # Environment-based configuration
│   ├── database/             # DB connection, pooling, health check
│   ├── docs/                 # Generated Swagger docs
│   ├── errors/               # Custom AppError types with codes
│   ├── handlers/             # HTTP handlers (thin, delegate to services)
│   ├── logger/               # Zap structured logger
│   ├── middleware/            # Auth, error handling, request logging
│   ├── models/               # GORM models (single source of truth)
│   ├── pagination/           # Generic PageRequest/PageResponse[T]
│   ├── services/             # Business logic layer (interface-based)
│   ├── testutil/             # Test helpers (DB setup, fixtures, assertions)
│   └── validator/            # Custom Gin validators
└── tests/
    └── integration/          # End-to-end workflow tests
```

## Development

### Prerequisites

- Go 1.24+
- PostgreSQL 16 (or use Docker Compose from the repo root)
- [Air](https://github.com/air-verse/air) for hot reload
- [golangci-lint](https://golangci-lint.run/) for linting
- [swag](https://github.com/swaggo/swag) for Swagger doc generation

### Running

```bash
# Via Docker Compose (from repo root, starts API + PostgreSQL)
npm run dev

# Locally with Air hot reload (requires PostgreSQL running)
make dev

# Or directly
go run cmd/api/main.go
```

The server runs at http://localhost:8080. Swagger UI is available at http://localhost:8080/swagger/index.html.

### Makefile Targets

| Target            | Description                                      |
|-------------------|--------------------------------------------------|
| `make dev`        | Run with Air hot reload                          |
| `make build`      | Build binary to `bin/api`                        |
| `make test`       | Run all tests                                    |
| `make test-cover` | Run tests with HTML coverage report              |
| `make test-race`  | Run tests with race detector                     |
| `make lint`       | Run golangci-lint                                |
| `make fmt`        | Format code (gofmt + goimports)                  |
| `make check-fast` | Quick check: build + vet + lint (no tests)       |
| `make check`      | Full verification: build + vet + lint + test + race |
| `make migrate-up`   | Run all pending migrations                     |
| `make migrate-down` | Roll back the last migration                   |
| `make migrate-version` | Show current migration version              |
| `make swagger`    | Regenerate Swagger docs                          |
| `make clean`      | Remove build artifacts and coverage files        |

### Verification

After any code change, run:

```bash
# Quick feedback (compile + lint)
make check-fast

# Full verification (must pass before merging)
make check
```

## API Endpoints

### Public

```
POST /api/v1/auth/register     # Register new user
POST /api/v1/auth/login        # Login, returns access + refresh tokens
POST /api/v1/auth/refresh      # Refresh access token
GET  /api/health               # Health check (includes DB ping)
GET  /swagger/*                # Swagger UI
```

### Protected (require Bearer token)

```
# User
GET    /api/v1/profile

# Accounts
POST   /api/v1/accounts/cash
POST   /api/v1/accounts/investment
GET    /api/v1/accounts
GET    /api/v1/accounts/:id
PUT    /api/v1/accounts/:id
GET    /api/v1/accounts/:id/transactions
GET    /api/v1/accounts/:id/investments

# Transactions
POST   /api/v1/transactions
POST   /api/v1/transactions/transfer
GET    /api/v1/transactions/:id
DELETE /api/v1/transactions/:id

# Categories
POST   /api/v1/categories
GET    /api/v1/categories
GET    /api/v1/categories/:id
PUT    /api/v1/categories/:id
DELETE /api/v1/categories/:id

# Budgets
POST   /api/v1/budgets
GET    /api/v1/budgets
GET    /api/v1/budgets/:id
PUT    /api/v1/budgets/:id
DELETE /api/v1/budgets/:id
GET    /api/v1/budgets/:id/progress

# Investments
POST   /api/v1/investments
GET    /api/v1/investments/portfolio
GET    /api/v1/investments/:id
PUT    /api/v1/investments/:id/price
POST   /api/v1/investments/:id/buy
POST   /api/v1/investments/:id/sell
POST   /api/v1/investments/:id/dividend
POST   /api/v1/investments/:id/split
GET    /api/v1/investments/:id/transactions
```

## Key Design Decisions

- **Monetary values as int64 cents** -- `$10.50` = `1050`. No floating-point rounding errors.
- **SQL migrations** via golang-migrate, not GORM AutoMigrate. Version-controlled and reversible.
- **Soft deletes** on all models. Deleted categories remain as references for existing transactions.
- **User-scoped queries** -- every data query includes `user_id` for data isolation.
- **Atomic operations** -- all balance-affecting operations wrapped in DB transactions.
- **Audit logging** -- sensitive operations logged to `audit_logs` table.
- **JWT auth** -- short-lived access tokens (15min) + refresh tokens (7d) with rotation.
- **Account lockout** -- 5 failed login attempts triggers a 15-minute lockout.

## Testing

```bash
make test          # Run all tests
make test-cover    # Tests with HTML coverage report
make test-race     # Tests with race detector
```

Three levels of tests:

- **Service tests** -- table-driven unit tests with in-memory SQLite (`internal/services/*_test.go`)
- **Handler tests** -- HTTP tests with mock services via interfaces (`internal/handlers/*_test.go`)
- **Integration tests** -- full workflow tests with real SQLite DB (`tests/integration/`)

## Database Migrations

Migrations live in `migrations/` as numbered SQL pairs:

```
000001_create_users.up.sql / .down.sql
000002_create_accounts.up.sql / .down.sql
...
000009_add_performance_indexes.up.sql / .down.sql
```

```bash
make migrate-up        # Apply all pending migrations
make migrate-down      # Roll back last migration
make migrate-version   # Check current version
```

In development, migrations run automatically on server start.

## Environment Variables

Configured via `apps/api/.env`:

| Variable       | Description                          | Default       |
|----------------|--------------------------------------|---------------|
| `ENV`          | Environment                          | `development` |
| `PORT`         | Server port                          | `8080`        |
| `DB_HOST`      | PostgreSQL host                      | `localhost`   |
| `DB_PORT`      | PostgreSQL port                      | `5433`        |
| `DB_USER`      | Database user                        | `kuberan`     |
| `DB_PASSWORD`  | Database password                    | `kuberan`     |
| `DB_NAME`      | Database name                        | `kuberan`     |
| `DB_SSLMODE`   | SSL mode                             | `disable`     |
| `JWT_SECRET`   | JWT signing key (required in prod)   | dev default   |
| `JWT_EXPIRES_IN` | Token expiration                   | `15m`         |

In production, `JWT_SECRET` must be explicitly set and `DB_PASSWORD` must not be the development default.

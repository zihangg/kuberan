# Kuberan Frontend Expansion Plan

## Context

The frontend foundation (Plan 002) is complete — the app has auth, dashboard, accounts, and transaction management. The sidebar navigation links to four pages that don't exist yet: Transactions, Categories, Budgets, and Investments. This plan builds out Categories, Transactions, and Budgets pages, adds a backend endpoint for cross-account transaction listing, and delivers several polish items: dark mode, edit account dialog, and dashboard improvements.

Investments is deferred to Plan 004 as it's the most complex domain with unique UI requirements (portfolio charts, buy/sell/dividend/split flows).

## Scope Summary

| Feature | Type | Backend Changes? |
|---|---|---|
| Categories page (CRUD) | Frontend | No — 5 endpoints exist |
| Transactions page (cross-account) | Frontend + Backend | Yes — new `GET /api/v1/transactions` endpoint |
| Budgets page (CRUD + progress) | Frontend | No — 6 endpoints exist |
| Dashboard improvements | Frontend | No — client-side aggregation |
| Dark mode toggle | Frontend | No — pure theming |
| Edit account dialog | Frontend | No — `PUT /accounts/:id` exists |

## Technology & Patterns

This plan follows all patterns established in Plan 002:
- ShadCN UI components (Dialog, Table, Tabs, Select, Badge, etc.)
- React Query hooks with query key factories and cache invalidation
- Controlled dialog pattern (`open`/`onOpenChange` props)
- `ApiClientError` mapping to user-friendly messages
- Sonner toast notifications on success
- Inline destructive banner for errors
- Skeleton loading states
- Client-side validation matching backend constraints

Backend additions follow patterns from Plan 001:
- 3-layer architecture (Handler → Service → Model)
- Interface-based services with mock-based handler tests
- Table-driven subtests with in-memory SQLite for service tests
- AppError types with error codes
- Swagger annotations on all endpoints
- Full verification via `scripts/check.sh`

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Category layout | Table with indentation | Consistent with accounts page; child categories indented under parents |
| Category color field | Predefined palette (~16 colors) | Quick, consistent UX; no color picker library needed |
| Category icon field | Plain text input | Simplest approach; user can type emoji or text; upgrade later |
| Parent type matching | Frontend enforces same-type | Prevents confusing mixed hierarchies even though backend allows cross-type parenting |
| Transactions listing | New backend endpoint | Client-side merging across accounts has broken pagination and N API calls; proper endpoint is cleaner |
| Dark mode | next-themes | Sonner already uses it; just need ThemeProvider + toggle |
| Budget progress display | Progress bars with color coding | Green < 75%, yellow 75-90%, red > 90% of budget |

---

## Phase 1: Backend — Cross-Account Transactions Endpoint

### 1.1 Add `GetUserTransactions` to Transaction Service

**Files**: `internal/services/interfaces.go`, `internal/services/transaction_service.go`

Add `AccountID *uint` to `TransactionFilter` struct so the cross-account endpoint can optionally filter by a specific account.

Add new method to `TransactionServicer` interface:
```go
GetUserTransactions(userID uint, page pagination.PageRequest, filter TransactionFilter) (*pagination.PageResponse[models.Transaction], error)
```

Implementation follows `GetAccountTransactions` pattern:
- Base query: `WHERE user_id = ?` (no account_id required)
- Apply `applyTransactionFilters` (same filter function, extended with `AccountID`)
- Add `.Preload("Category")` to include category data in response
- Order by `date DESC`
- Paginate via `pagination.Paginate(page)`

### 1.2 Add `GetUserTransactions` Handler

**File**: `internal/handlers/transaction_handler.go`

New handler `GetUserTransactions`:
- Extract user ID from context via `getUserID(c)`
- Bind pagination from query params
- Parse transaction filter from query params via `parseTransactionFilter(c)` (reuse existing)
- Add optional `account_id` query param parsing
- Call `transactionService.GetUserTransactions(userID, page, filter)`
- Return `200` with paginated response
- Full Swagger annotations

### 1.3 Register Route

**File**: `cmd/api/main.go`

Add `transactions.GET("", transactionHandler.GetUserTransactions)` to the transactions route group.

### 1.4 Service Tests

**File**: `internal/services/transaction_service_test.go`

Add `TestGetUserTransactions` with subtests:
- `lists_all_transactions_across_accounts` — create transactions in multiple accounts, verify all returned
- `filters_by_type` — only income/expense transactions
- `filters_by_date_range` — from_date and to_date
- `filters_by_category` — category_id filter
- `filters_by_amount_range` — min/max amount
- `filters_by_account_id` — optional account filter
- `paginates_correctly` — verify page/page_size/total_items
- `user_isolation` — user A cannot see user B's transactions
- `orders_by_date_desc` — newest first

### 1.5 Handler Tests

**File**: `internal/handlers/transaction_handler_test.go`

Add mock method to `mockTransactionService` for `GetUserTransactions`. Add `TestTransactionHandler_GetUserTransactions` with subtests:
- `returns_200_with_transactions`
- `returns_200_empty_when_no_transactions`
- `passes_filters_to_service` — capture filter args, verify
- `returns_400_for_invalid_date`
- `returns_400_for_invalid_type`
- `returns_401_without_auth`

### 1.6 Backend Verification

Run `./scripts/check.sh` from `apps/api/` — build, vet, lint, test, race detection must all pass.

---

## Phase 2: Frontend — Categories

### 2.1 Extend Category React Query Hooks

**File**: `src/hooks/use-categories.ts` (modify existing)

Extend `categoryKeys` factory with `details` and `detail` entries. Add:
- `useCategory(id)` — GET single category, unwrap `CategoryResponse`
- `useCreateCategory()` — POST mutation, invalidate lists
- `useUpdateCategory(id)` — PUT mutation, invalidate lists + detail
- `useDeleteCategory()` — DELETE mutation, invalidate lists

### 2.2 Color Palette Component

**File**: `src/components/ui/color-palette.tsx` (new)

Grid of ~16 preset Tailwind colors as small circles. Props: `value`, `onChange`, `disabled?`. Selected state with ring indicator. Click to select/deselect.

Preset colors:
```
#EF4444 (red)      #F97316 (orange)   #F59E0B (amber)    #EAB308 (yellow)
#84CC16 (lime)     #22C55E (green)    #10B981 (emerald)  #14B8A6 (teal)
#06B6D4 (cyan)     #3B82F6 (blue)     #6366F1 (indigo)   #8B5CF6 (violet)
#A855F7 (purple)   #EC4899 (pink)     #F43F5E (rose)     #6B7280 (gray)
```

### 2.3 Categories List Page

**File**: `src/app/(dashboard)/categories/page.tsx` (new)

- Page header: "Categories" + "Create Category" button
- Type filter tabs: All | Income | Expense
- Table: Name (indented for children), Type badge, Color swatch, Icon, Description (truncated), Actions (edit/delete)
- Client-side tree ordering from flat list (parents first, children indented after)
- Pagination, loading skeletons, empty state

### 2.4 Create Category Dialog

**File**: `src/components/categories/create-category-dialog.tsx` (new)

Fields: name (required, max 100), type (income/expense, required), description (opt, max 500), icon (opt, max 50), color (ColorPalette), parent category (select, filtered to same type, resets on type change).

Error mapping: `INVALID_INPUT` with "already exists" → "A category with this name already exists"

### 2.5 Edit Category Dialog

**File**: `src/components/categories/edit-category-dialog.tsx` (new)

Same as create but: type is read-only badge, fields pre-populated, parent dropdown excludes self, sends only changed fields.

Error mapping: `SELF_PARENT_CATEGORY` → "A category cannot be its own parent"

### 2.6 Delete Category Dialog

**File**: `src/components/categories/delete-category-dialog.tsx` (new)

Confirmation dialog with category name. Error mapping: `CATEGORY_HAS_CHILDREN` → "This category has child categories. Delete them first."

### 2.7 Wire Category Dialogs & Verify

Wire all dialogs into categories page via `useState`. Run `npm run build`.

---

## Phase 3: Frontend — Transactions Page

### 3.1 Add Transaction Types & Hooks

**File**: `src/types/api.ts` (modify)

Add `UserTransactionFilters` extending `PaginationParams` with all filter fields including `account_id`.

**File**: `src/hooks/use-transactions.ts` (modify)

Add `useTransactions(filters?)` hook that calls `GET /api/v1/transactions` with query params. Returns `PageResponse<Transaction>`.

### 3.2 Transactions List Page

**File**: `src/app/(dashboard)/transactions/page.tsx` (new)

- Page header: "Transactions" + "Add Transaction" button + "Transfer" button
- Filter bar (collapsible/expandable):
  - Account dropdown (all accounts, optional)
  - Type dropdown (all/income/expense/transfer)
  - Category dropdown (all categories, optional)
  - Date range: from date, to date
  - Amount range: min, max
- Transactions table:
  - Columns: Date, Description, Account name, Type badge (colored), Category, Amount (signed, colored)
  - Pagination
- Loading skeletons, empty state
- Reuse existing `CreateTransactionDialog` and `CreateTransferDialog` wired via useState

### 3.3 Verify Transactions Page Build

Run `npm run build` to verify.

---

## Phase 4: Frontend — Budgets Page

### 4.1 Add Budget API Types

**File**: `src/types/api.ts` (modify)

Add types:
- `BudgetResponse` — `{ budget: Budget }`
- `BudgetProgressResponse` — `{ progress: BudgetProgress }`
- `CreateBudgetRequest` — `{ category_id, name, amount, period, start_date, end_date? }`
- `UpdateBudgetRequest` — `{ name?, amount?, period?, end_date? }` (category_id and start_date immutable)
- `BudgetFilters` extending `PaginationParams` — `{ is_active?, period? }`

### 4.2 Budget React Query Hooks

**File**: `src/hooks/use-budgets.ts` (new)

- `budgetKeys` factory with list/detail/progress keys
- `useBudgets(filters?)` — GET /api/v1/budgets with optional is_active and period filters
- `useBudget(id)` — GET /api/v1/budgets/:id, unwrap `BudgetResponse`
- `useBudgetProgress(id)` — GET /api/v1/budgets/:id/progress, unwrap `BudgetProgressResponse`
- `useCreateBudget()` — POST mutation
- `useUpdateBudget(id)` — PUT mutation
- `useDeleteBudget()` — DELETE mutation

### 4.3 Budgets List Page

**File**: `src/app/(dashboard)/budgets/page.tsx` (new)

- Page header: "Budgets" + "Create Budget" button
- Filter tabs or controls: All | Active | Inactive (via `is_active` param), Monthly | Yearly (via `period` param)
- Budget cards (not table — cards work better for budget visualization):
  - Each card shows: budget name, category badge, period badge, amount (formatted)
  - Progress bar: spent/budgeted with percentage
  - Color coding: green < 75%, yellow 75-90%, red > 90%
  - Remaining amount displayed
  - Edit/Delete action buttons
- Each card uses `useBudgetProgress(id)` to fetch current progress
- Pagination, loading skeletons, empty state

### 4.4 Create Budget Dialog

**File**: `src/components/budgets/create-budget-dialog.tsx` (new)

Fields:
- Name (required, max 100)
- Category (required, select from user's categories)
- Amount (required, CurrencyInput, > 0, in cents)
- Period (required, select: Monthly / Yearly)
- Start Date (required, date picker)
- End Date (optional, date picker, must be after start date if set)

Error mapping: `CATEGORY_NOT_FOUND` → "Selected category not found"

### 4.5 Edit Budget Dialog

**File**: `src/components/budgets/edit-budget-dialog.tsx` (new)

Same as create but: category and start date are read-only (not updatable per backend). Pre-populated fields, sends only changed values.

### 4.6 Delete Budget Dialog

**File**: `src/components/budgets/delete-budget-dialog.tsx` (new)

Simple confirmation. Error mapping: `BUDGET_NOT_FOUND` → "Budget not found"

### 4.7 Wire Budget Dialogs & Verify

Wire dialogs, run `npm run build`.

---

## Phase 5: Frontend — Polish & Improvements

### 5.1 Dark Mode

**File**: `src/providers/theme-provider.tsx` (new)

Install `next-themes`. Create `ThemeProvider` wrapping `NextThemesProvider` with `attribute="class"`, `defaultTheme="system"`, `enableSystem`.

**File**: `src/app/layout.tsx` (modify)

Wrap children with `ThemeProvider` (outermost provider).

**File**: `src/components/layout/app-header.tsx` (modify)

Add theme toggle button in the header (Sun/Moon icon). Uses `useTheme()` from `next-themes` to cycle between light/dark/system.

**File**: `src/app/globals.css` (modify if needed)

Ensure ShadCN dark mode CSS variables are present (they should be from ShadCN init, but verify).

### 5.2 Edit Account Dialog

**File**: `src/components/accounts/edit-account-dialog.tsx` (new)

Controlled dialog with `account: Account` prop. Fields: name (required, max 100), description (optional, max 500). Pre-populated from account. Uses `useUpdateAccount(id)` mutation (already exists in hooks). Success: toast, close. Error: inline banner.

**File**: `src/app/(dashboard)/accounts/[id]/page.tsx` (modify)

Wire Edit button in account header to open `EditAccountDialog`.

### 5.3 Dashboard Improvements

**File**: `src/app/(dashboard)/page.tsx` (modify)

Enhance the existing dashboard:

1. **Budget progress summary**: Show top 3-4 active budgets with mini progress bars on the dashboard. Uses `useBudgets({ is_active: true })` + `useBudgetProgress(id)` for each. Shows budget name, category, and spent/budgeted bar.

2. **Net summary cards**: In addition to total balance, add:
   - Cash accounts total
   - Investment accounts total
   - Calculated from the already-loaded accounts data (no new API call)

3. **Recent transactions from all accounts**: Use the new `useTransactions({ page_size: 5 })` hook (from Phase 3) to show 5 most recent transactions across all accounts instead of just the first account.

### 5.4 Final Verification

Run full lint + build:
```bash
cd apps/web && npm run lint && npm run build
```

---

## Backend API Reference

### New Endpoint: GET /api/v1/transactions

**Query Parameters**:
| Param | Type | Required | Description |
|---|---|---|---|
| `page` | int | No | Page number (default 1) |
| `page_size` | int | No | Items per page (default 20, max 100) |
| `account_id` | int | No | Filter by specific account |
| `from_date` | string | No | RFC3339, filter start date |
| `to_date` | string | No | RFC3339, filter end date |
| `type` | string | No | income, expense, transfer, investment |
| `category_id` | int | No | Filter by category |
| `min_amount` | int | No | Min amount in cents |
| `max_amount` | int | No | Max amount in cents |

**Response**: `PageResponse[Transaction]` with `Category` preloaded.

### Existing Categories Endpoints
```
POST   /api/v1/categories          # Create (name, type, description, icon, color, parent_id)
GET    /api/v1/categories           # List (?type=income|expense, pagination)
GET    /api/v1/categories/:id       # Get single
PUT    /api/v1/categories/:id       # Update (type immutable)
DELETE /api/v1/categories/:id       # Soft delete (blocked if has children)
```

Key behaviors:
- Type immutable after creation
- Flat response (client builds tree from parent_id)
- Duplicate name rejected (INVALID_INPUT)
- Self-parent rejected (SELF_PARENT_CATEGORY)
- Delete blocked by children (CATEGORY_HAS_CHILDREN, 409)
- Delete allowed with transactions (soft delete, transactions keep reference)

### Existing Budget Endpoints
```
POST   /api/v1/budgets              # Create (category_id, name, amount, period, start_date, end_date?)
GET    /api/v1/budgets              # List (?is_active=true|false, ?period=monthly|yearly, pagination)
GET    /api/v1/budgets/:id          # Get single (preloads Category)
PUT    /api/v1/budgets/:id          # Update (name?, amount?, period?, end_date? — category_id and start_date immutable)
DELETE /api/v1/budgets/:id          # Soft delete
GET    /api/v1/budgets/:id/progress # Current period progress (budgeted, spent, remaining, percentage)
```

Key behaviors:
- category_id and start_date cannot be changed after creation
- Progress is always for current period (monthly = current month, yearly = current year)
- Progress sums expense transactions matching category_id within the period
- Remaining can be negative if overspent
- Categories are preloaded on GET single and GET list

### Error Codes Reference
| Code | HTTP | Feature | When |
|---|---|---|---|
| `INVALID_INPUT` | 400 | Categories | Empty/duplicate name, invalid type |
| `SELF_PARENT_CATEGORY` | 400 | Categories | Setting category as its own parent |
| `CATEGORY_NOT_FOUND` | 404 | Categories, Budgets | Category doesn't exist |
| `CATEGORY_HAS_CHILDREN` | 409 | Categories | Deleting category with children |
| `BUDGET_NOT_FOUND` | 404 | Budgets | Budget doesn't exist |
| `ACCOUNT_NOT_FOUND` | 404 | Transactions | Account doesn't exist (if filtering) |
| `TRANSACTION_NOT_FOUND` | 404 | Transactions | Transaction doesn't exist |

---

## File Structure After This Plan

### Backend (new/modified)
```
apps/api/
├── cmd/api/main.go                            # MODIFIED: add GET /transactions route
├── internal/
│   ├── handlers/
│   │   └── transaction_handler.go             # MODIFIED: add GetUserTransactions handler
│   ├── services/
│   │   ├── interfaces.go                      # MODIFIED: add AccountID to filter, new method
│   │   └── transaction_service.go             # MODIFIED: add GetUserTransactions, extend filters
│   │   └── transaction_service_test.go        # MODIFIED: add tests
│   └── handlers/
│       └── transaction_handler_test.go        # MODIFIED: add handler tests
```

### Frontend (new/modified)
```
apps/web/src/
├── app/
│   ├── layout.tsx                              # MODIFIED: add ThemeProvider
│   ├── globals.css                             # VERIFY: dark mode vars
│   └── (dashboard)/
│       ├── page.tsx                            # MODIFIED: dashboard improvements
│       ├── categories/
│       │   └── page.tsx                        # NEW
│       ├── transactions/
│       │   └── page.tsx                        # NEW
│       ├── budgets/
│       │   └── page.tsx                        # NEW
│       └── accounts/[id]/
│           └── page.tsx                        # MODIFIED: wire edit dialog
├── components/
│   ├── categories/
│   │   ├── create-category-dialog.tsx          # NEW
│   │   ├── edit-category-dialog.tsx            # NEW
│   │   └── delete-category-dialog.tsx          # NEW
│   ├── budgets/
│   │   ├── create-budget-dialog.tsx            # NEW
│   │   ├── edit-budget-dialog.tsx              # NEW
│   │   └── delete-budget-dialog.tsx            # NEW
│   ├── accounts/
│   │   └── edit-account-dialog.tsx             # NEW
│   ├── layout/
│   │   └── app-header.tsx                      # MODIFIED: theme toggle
│   └── ui/
│       └── color-palette.tsx                   # NEW
├── hooks/
│   ├── use-categories.ts                       # MODIFIED: add CRUD hooks
│   ├── use-transactions.ts                     # MODIFIED: add useTransactions
│   └── use-budgets.ts                          # NEW
├── providers/
│   └── theme-provider.tsx                      # NEW
└── types/
    └── api.ts                                  # MODIFIED: add budget + transaction types
```

## Implementation Order

```
Phase 1: Backend
  1.1  Add GetUserTransactions to service interface + implementation
  1.2  Add GetUserTransactions handler with Swagger
  1.3  Register route in main.go
  1.4  Service tests for GetUserTransactions
  1.5  Handler tests for GetUserTransactions
  1.6  Backend verification (./scripts/check.sh)

Phase 2: Frontend — Categories
  2.1  Extend category hooks (CRUD mutations)
  2.2  Color palette component
  2.3  Categories list page
  2.4  Create Category dialog
  2.5  Edit Category dialog
  2.6  Delete Category dialog
  2.7  Wire dialogs + build verification

Phase 3: Frontend — Transactions
  3.1  Add transaction types + useTransactions hook
  3.2  Transactions list page with filters
  3.3  Build verification

Phase 4: Frontend — Budgets
  4.1  Add budget API types
  4.2  Budget hooks (CRUD + progress)
  4.3  Budgets list page with progress bars
  4.4  Create Budget dialog
  4.5  Edit Budget dialog
  4.6  Delete Budget dialog
  4.7  Wire dialogs + build verification

Phase 5: Frontend — Polish
  5.1  Dark mode (ThemeProvider + toggle)
  5.2  Edit account dialog
  5.3  Dashboard improvements (budget summary, net totals, cross-account transactions)
  5.4  Final verification (lint + build)
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

**Frontend** — after each task:
```bash
cd apps/web && npm run build
```
After completing all phases:
```bash
cd apps/web && npm run lint && npm run build
```

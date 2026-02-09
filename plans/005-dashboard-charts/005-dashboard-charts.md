# Kuberan Dashboard Charts

## Context

Plan 004 (Post-Expansion Fixes) is complete — the app has full CRUD for accounts (cash, investment, credit card), transactions (with editing), categories, and budgets. The dashboard shows summary cards, account cards, budget progress bars, and recent transactions. However, it's all boxes and numbers — there are no data visualizations.

This plan adds three charts to the dashboard to make it more aesthetic and informative:

1. **Expenditure Breakdown (Donut Chart)**: Where is my money going this month?
2. **Income vs. Expenses (Bar Chart)**: Am I saving or overspending over the last 6 months?
3. **Spending Trend (Area Chart)**: How fast am I spending this month?

All three require new backend aggregation endpoints (SQL GROUP BY queries) and new frontend chart components using ShadCN Charts (Recharts wrapper).

## Scope Summary

| Feature | Type | Backend Changes? |
|---|---|---|
| Spending by category endpoint | Backend | Yes — new service method, handler, route, tests |
| Monthly income/expenses endpoint | Backend | Yes — new service method, handler, route, tests |
| Daily spending endpoint | Backend | Yes — new service method, handler, route, tests |
| ShadCN Charts installation | Frontend | Yes — install recharts + chart component |
| Expenditure donut chart | Frontend | No backend — consumes spending-by-category endpoint |
| Income vs. expenses bar chart | Frontend | No backend — consumes monthly-summary endpoint |
| Spending trend area chart | Frontend | No backend — consumes daily-spending endpoint |
| Dashboard layout update | Frontend | No — rearrange existing sections, insert chart grid |

## Technology & Patterns

This plan follows all patterns established in Plans 001–004:
- Backend: 3-layer architecture (Handler → Service → Model), interface-based services, AppError types, Swagger annotations, table-driven tests with in-memory SQLite
- Frontend: ShadCN UI components, React Query hooks with query key factories and cache invalidation, skeleton loading states, Sonner toasts

New additions:
- **ShadCN Charts**: Wraps Recharts with `ChartContainer`, `ChartTooltip`, `ChartTooltipContent`, `ChartLegend`, `ChartLegendContent` components. Installed via `npx shadcn@latest add chart`.
- **Recharts**: Underlying charting library. Uses `PieChart`, `Pie`, `Cell` (donut), `BarChart`, `Bar`, `XAxis`, `YAxis`, `CartesianGrid` (bar chart), `AreaChart`, `Area` (area chart).
- **Chart theming**: Category colors come from the database (`category.color` hex field). Generic chart colors use existing `--chart-1` through `--chart-5` CSS variables defined in `globals.css`.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Charting library | ShadCN Charts (Recharts) | Matches existing ShadCN design system. Uses `--chart-*` CSS vars already in `globals.css`. No new styling paradigm. |
| Data strategy | Server-side aggregation endpoints | Client-side aggregation breaks with >100 transactions per page (API pagination cap). SQL GROUP BY is cleaner and more efficient. |
| Expenditure chart type | Donut chart | Clean, modern look. Center shows total. Category colors from DB make each segment immediately recognizable. |
| Income vs. expenses chart type | Grouped bar chart | Side-by-side bars per month are the clearest way to compare two values over time. |
| Spending trend chart type | Area chart with cumulative line | Gradient fill is aesthetically appealing. Cumulative view shows spending velocity vs. a simple daily bar. |
| Default time period | Current calendar month (donut + area), last 6 months (bar) | Most useful default for personal finance at-a-glance. No period selector to keep the dashboard clean. |
| Chart layout | 2-column grid (donut + bar) + full-width (area) | Balanced information density. Donut is compact, bar needs horizontal space, area benefits from full width. |
| Chart placement | Between Budget Overview and Recent Transactions | Natural flow: summaries → accounts → budgets → charts → recent activity. |
| Category colors | From DB `category.color` field | Users already assign colors to categories. Chart segments match what they see elsewhere in the app. Uncategorized expenses get `#9CA3AF` (neutral gray). |
| SQLite compatibility | Use `strftime` for date truncation in tests | Production uses PostgreSQL `DATE_TRUNC`, but service tests run on SQLite. The service layer must use raw SQL compatible with both, or use GORM-abstracted queries where possible. |
| Empty state | Show "No data" message with subtle icon | Don't render an empty chart — show a helpful message encouraging the user to add transactions. |

---

## Phase 1: Backend — Spending by Category Endpoint

### 1.1 Add SpendingByCategoryItem and GetSpendingByCategory to Interface

**File**: `apps/api/internal/services/interfaces.go`

Add response structs:
```go
// SpendingByCategoryItem represents spending total for a single category.
type SpendingByCategoryItem struct {
    CategoryID    *uint   `json:"category_id"`
    CategoryName  string  `json:"category_name"`
    CategoryColor string  `json:"category_color"`
    CategoryIcon  string  `json:"category_icon"`
    Total         int64   `json:"total"`
}

// SpendingByCategory represents the full spending breakdown response.
type SpendingByCategory struct {
    Items      []SpendingByCategoryItem `json:"items"`
    TotalSpent int64                    `json:"total_spent"`
    FromDate   time.Time                `json:"from_date"`
    ToDate     time.Time                `json:"to_date"`
}
```

Add to `TransactionServicer` interface:
```go
GetSpendingByCategory(userID uint, from, to time.Time) (*SpendingByCategory, error)
```

### 1.2 Implement GetSpendingByCategory Service

**File**: `apps/api/internal/services/transaction_service.go`

Implementation:
1. Define a local scan struct for the GROUP BY result: `categorySpend { CategoryID *uint, Total int64 }`
2. Query: `SELECT category_id, COALESCE(SUM(amount), 0) as total FROM transactions WHERE user_id = ? AND type = 'expense' AND deleted_at IS NULL AND date BETWEEN ? AND ? GROUP BY category_id`
3. Scan results into `[]categorySpend`
4. For each result, if `CategoryID` is non-nil, fetch the category from DB to get name/color/icon. If nil, use "Uncategorized" with color `#9CA3AF` and empty icon.
5. Sum all totals into `TotalSpent`
6. Return `SpendingByCategory` with items sorted by total descending

**Note on SQLite compatibility**: The query uses standard SQL (no PostgreSQL-specific functions), so it works in both production (PostgreSQL) and tests (SQLite).

### 1.3 Add GetSpendingByCategory Handler

**File**: `apps/api/internal/handlers/transaction_handler.go`

Add `GetSpendingByCategory` handler:
- Extract `from_date` and `to_date` query params (required, RFC3339 or date-only format)
- If missing, return 400 with `INVALID_INPUT` error
- Parse dates via `parseFlexibleTime` (already exists in the handler)
- Call `h.transactionService.GetSpendingByCategory(userID, from, to)`
- Return `c.JSON(http.StatusOK, result)`
- Full Swagger annotations: `@Summary Get spending by category`, `@Tags transactions`, `@Security BearerAuth`

### 1.4 Register Route

**File**: `apps/api/cmd/api/main.go`

Add `transactions.GET("/spending-by-category", transactionHandler.GetSpendingByCategory)` — place before the `/:id` routes to avoid path conflicts.

### 1.5 Service Tests

**File**: `apps/api/internal/services/transaction_service_test.go`

Add `TestGetSpendingByCategory` with subtests:
- `groups_by_category`: Create 2 categories + expenses across both, verify items contain correct totals per category
- `handles_uncategorized`: Create expense with nil category_id, verify "Uncategorized" item with gray color
- `filters_by_date_range`: Create expenses in and out of range, verify only in-range expenses counted
- `excludes_non_expense_types`: Create income + transfer transactions, verify they don't appear in results
- `returns_empty_for_no_expenses`: No expense transactions exist, verify empty items array and total_spent = 0
- `user_isolation`: User A's expenses don't appear in user B's results
- `sorts_by_total_descending`: Multiple categories, verify largest spending first

### 1.6 Handler Tests

**File**: `apps/api/internal/handlers/transaction_handler_test.go`

Add mock method `getSpendingByCategoryFn` to `mockTransactionService`. Add `TestTransactionHandler_GetSpendingByCategory` with subtests:
- `returns_200_with_data`: Mock returns items, verify 200 + JSON structure
- `returns_400_missing_from_date`: No from_date param, verify 400
- `returns_400_missing_to_date`: No to_date param, verify 400
- `returns_200_empty_array`: Mock returns no items, verify 200 with empty items

### 1.7 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

---

## Phase 2: Backend — Monthly Summary Endpoint

### 2.1 Add MonthlySummaryItem and GetMonthlySummary to Interface

**File**: `apps/api/internal/services/interfaces.go`

Add:
```go
// MonthlySummaryItem represents income and expense totals for a single month.
type MonthlySummaryItem struct {
    Month    string `json:"month"`    // "2026-02" format
    Income   int64  `json:"income"`   // cents
    Expenses int64  `json:"expenses"` // cents
}
```

Add to `TransactionServicer` interface:
```go
GetMonthlySummary(userID uint, months int) ([]MonthlySummaryItem, error)
```

### 2.2 Implement GetMonthlySummary Service

**File**: `apps/api/internal/services/transaction_service.go`

Implementation:
1. Calculate the start date as `months` months ago from the 1st of the current month
2. Define a local scan struct: `monthTypeSum { Month string, Type string, Total int64 }`
3. Query differs between PostgreSQL and SQLite:
   - The service should use `strftime('%Y-%m', date)` for date truncation (works in SQLite for tests)
   - In production, PostgreSQL also supports `TO_CHAR(date, 'YYYY-MM')` but `strftime` is not available
   - **Solution**: Use GORM raw SQL with `TO_CHAR(date, 'YYYY-MM')` for the main query. For the test, use a helper approach — either run multiple targeted queries per month, or use a build-tag strategy
   - **Simpler solution**: Iterate over each month in the range, run two separate SUM queries (one for income, one for expenses) per month. This avoids database-specific date functions entirely and uses the same BETWEEN pattern proven in `GetBudgetProgress`.
4. For each month in range (from `startMonth` to current month):
   - Calculate `monthStart` and `monthEnd` timestamps
   - Query: `SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id = ? AND type = ? AND deleted_at IS NULL AND date BETWEEN ? AND ?`
   - Run once for `type = 'income'`, once for `type = 'expense'`
5. Return `[]MonthlySummaryItem` ordered chronologically

### 2.3 Add GetMonthlySummary Handler

**File**: `apps/api/internal/handlers/transaction_handler.go`

Add `GetMonthlySummary` handler:
- Extract optional `months` query param (default: 6, min: 1, max: 24)
- Call `h.transactionService.GetMonthlySummary(userID, months)`
- Return `c.JSON(http.StatusOK, gin.H{"data": result})`
- Full Swagger annotations

### 2.4 Register Route

**File**: `apps/api/cmd/api/main.go`

Add `transactions.GET("/monthly-summary", transactionHandler.GetMonthlySummary)` — place before `/:id` routes.

### 2.5 Service Tests

**File**: `apps/api/internal/services/transaction_service_test.go`

Add `TestGetMonthlySummary` with subtests:
- `returns_monthly_totals`: Create income + expenses across 3 months, verify correct totals per month
- `defaults_to_6_months`: Pass months=6, verify 6 items returned (even if some are zero)
- `excludes_transfers_and_investments`: Create transfer + investment transactions, verify not counted
- `returns_zero_for_empty_months`: No transactions in a month, verify income=0 and expenses=0
- `user_isolation`: User A's transactions don't appear in user B's results

### 2.6 Handler Tests

**File**: `apps/api/internal/handlers/transaction_handler_test.go`

Add mock method `getMonthlySummaryFn`. Add `TestTransactionHandler_GetMonthlySummary` with subtests:
- `returns_200_with_default_months`: No months param, verify 200 + data array
- `returns_200_with_custom_months`: months=3, verify mock called with 3
- `returns_200_empty_months`: Mock returns items with zero values, verify 200

### 2.7 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

---

## Phase 3: Backend — Daily Spending Endpoint

### 3.1 Add DailySpendingItem and GetDailySpending to Interface

**File**: `apps/api/internal/services/interfaces.go`

Add:
```go
// DailySpendingItem represents expense total for a single day.
type DailySpendingItem struct {
    Date  string `json:"date"`  // "2026-02-01" format
    Total int64  `json:"total"` // cents
}
```

Add to `TransactionServicer` interface:
```go
GetDailySpending(userID uint, from, to time.Time) ([]DailySpendingItem, error)
```

### 3.2 Implement GetDailySpending Service

**File**: `apps/api/internal/services/transaction_service.go`

Implementation:
1. Iterate over each day from `from` to `to`:
   - Calculate `dayStart` (00:00:00) and `dayEnd` (23:59:59)
   - Query: `SELECT COALESCE(SUM(amount), 0) FROM transactions WHERE user_id = ? AND type = 'expense' AND deleted_at IS NULL AND date BETWEEN ? AND ?`
   - Scan into `int64`
2. Build `[]DailySpendingItem` for each day, format date as `"2006-01-02"`
3. Return chronologically ordered array (includes zero-amount days)

**Note**: This day-by-day iteration approach avoids database-specific date truncation functions, maintaining SQLite test compatibility. For a 31-day month, this is 31 simple queries — acceptable for a single-user self-hosted app.

### 3.3 Add GetDailySpending Handler

**File**: `apps/api/internal/handlers/transaction_handler.go`

Add `GetDailySpending` handler:
- Extract `from_date` and `to_date` query params (required)
- Validate date range doesn't exceed 366 days (prevent abuse)
- Call `h.transactionService.GetDailySpending(userID, from, to)`
- Return `c.JSON(http.StatusOK, gin.H{"data": result})`
- Full Swagger annotations

### 3.4 Register Route

**File**: `apps/api/cmd/api/main.go`

Add `transactions.GET("/daily-spending", transactionHandler.GetDailySpending)` — place before `/:id` routes.

### 3.5 Service Tests

**File**: `apps/api/internal/services/transaction_service_test.go`

Add `TestGetDailySpending` with subtests:
- `returns_daily_totals`: Create expenses on different days, verify correct per-day totals
- `includes_zero_days`: Days with no expenses still appear with total=0
- `excludes_non_expense_types`: Income/transfer/investment transactions not counted
- `filters_by_date_range`: Expenses outside range not counted
- `user_isolation`: User A's expenses don't appear in user B's results

### 3.6 Handler Tests

**File**: `apps/api/internal/handlers/transaction_handler_test.go`

Add mock method `getDailySpendingFn`. Add `TestTransactionHandler_GetDailySpending` with subtests:
- `returns_200_with_data`: Mock returns daily items, verify 200 + JSON
- `returns_400_missing_from_date`: No from_date, verify 400
- `returns_400_missing_to_date`: No to_date, verify 400
- `returns_400_for_excessive_range`: Range > 366 days, verify 400

### 3.7 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

---

## Phase 4: Frontend — ShadCN Charts Setup

### 4.1 Install ShadCN Chart Component

Run `npx shadcn@latest add chart` from `apps/web/`.

This will:
- Install `recharts` as a dependency
- Create `src/components/ui/chart.tsx` with `ChartContainer`, `ChartTooltip`, `ChartTooltipContent`, `ChartLegend`, `ChartLegendContent`, and `ChartStyle` components

### 4.2 Add API Types for Chart Data

**File**: `apps/web/src/types/api.ts`

Add:
```ts
// Chart/analytics response types
export interface SpendingByCategoryItem {
  category_id: number | null;
  category_name: string;
  category_color: string;
  category_icon: string;
  total: number; // cents
}

export interface SpendingByCategory {
  items: SpendingByCategoryItem[];
  total_spent: number; // cents
  from_date: string; // ISO 8601
  to_date: string; // ISO 8601
}

export interface MonthlySummaryItem {
  month: string; // "2026-02"
  income: number; // cents
  expenses: number; // cents
}

export interface DailySpendingItem {
  date: string; // "2026-02-01"
  total: number; // cents
}
```

### 4.3 Add React Query Hooks for Chart Data

**File**: `apps/web/src/hooks/use-transactions.ts`

Add query key entries:
```ts
spendingByCategory: (from: string, to: string) =>
  [...transactionKeys.all, "spendingByCategory", from, to] as const,
monthlySummary: (months: number) =>
  [...transactionKeys.all, "monthlySummary", months] as const,
dailySpending: (from: string, to: string) =>
  [...transactionKeys.all, "dailySpending", from, to] as const,
```

Add hooks:
```ts
export function useSpendingByCategory(from: string, to: string) {
  return useQuery({
    queryKey: transactionKeys.spendingByCategory(from, to),
    queryFn: () =>
      apiClient.get<SpendingByCategory>(
        "/api/v1/transactions/spending-by-category",
        { from_date: from, to_date: to }
      ),
  });
}

export function useMonthlySummary(months = 6) {
  return useQuery({
    queryKey: transactionKeys.monthlySummary(months),
    queryFn: () =>
      apiClient.get<{ data: MonthlySummaryItem[] }>(
        "/api/v1/transactions/monthly-summary",
        { months }
      ),
    select: (res) => res.data,
  });
}

export function useDailySpending(from: string, to: string) {
  return useQuery({
    queryKey: transactionKeys.dailySpending(from, to),
    queryFn: () =>
      apiClient.get<{ data: DailySpendingItem[] }>(
        "/api/v1/transactions/daily-spending",
        { from_date: from, to_date: to }
      ),
    select: (res) => res.data,
  });
}
```

### 4.4 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 5: Frontend — Expenditure Breakdown Donut Chart

### 5.1 Create ExpenditureChart Component

**File**: `apps/web/src/components/dashboard/expenditure-chart.tsx` (new)

Component: `ExpenditureChart`

Behavior:
- Computes current month start/end dates for the query
- Calls `useSpendingByCategory(fromDate, toDate)`
- Shows `Skeleton` while loading
- Shows empty state ("No expenses this month") if no items
- Renders a ShadCN `ChartContainer` wrapping a Recharts `PieChart` with `Pie` component:
  - `innerRadius={60}` and `outerRadius={80}` for donut appearance
  - `paddingAngle={2}` for segment gaps
  - Each `Cell` uses `fill={item.category_color}` from the API response
  - `dataKey="total"`, `nameKey="category_name"`
- Center label: `<text>` SVG element showing `formatCurrency(totalSpent)` and "This Month" subtitle using Recharts `customized` prop or a Recharts `Label`
- `ChartTooltip` with `ChartTooltipContent` showing category name + formatted amount + percentage
- Legend below the chart (on mobile) or to the right (on desktop) showing each category with its color dot, icon, name, formatted amount, and percentage of total

Wrapped in a `Card` with `CardHeader` ("Spending Breakdown") and `CardContent`.

### 5.2 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 6: Frontend — Income vs. Expenses Bar Chart

### 6.1 Create IncomeExpensesChart Component

**File**: `apps/web/src/components/dashboard/income-expenses-chart.tsx` (new)

Component: `IncomeExpensesChart`

Behavior:
- Calls `useMonthlySummary(6)` for last 6 months
- Shows `Skeleton` while loading
- Shows empty state if all months have zero income and zero expenses
- Renders a ShadCN `ChartContainer` wrapping a Recharts `BarChart`:
  - `CartesianGrid` with `vertical={false}`
  - `XAxis` with `dataKey="month"` and `tickFormatter` to show short month name (e.g., "Feb")
  - `YAxis` hidden or with formatted currency ticks
  - Two `Bar` components: `income` (green, `var(--color-income)` or `#22C55E`) and `expenses` (red, `var(--color-expenses)` or `#EF4444`)
  - `radius={4}` for rounded bar tops
  - `barSize={20}` for appropriate bar width
- `ChartTooltip` with `ChartTooltipContent` showing month name, income amount, expenses amount (formatted as currency)
- `ChartLegend` with `ChartLegendContent` showing "Income" and "Expenses"
- Subtitle text below chart title: net savings for the period (total income - total expenses), colored green if positive, red if negative

Wrapped in a `Card` with `CardHeader` ("Income vs. Expenses") and `CardContent`.

Chart config:
```ts
const chartConfig = {
  income: { label: "Income", color: "#22C55E" },
  expenses: { label: "Expenses", color: "#EF4444" },
} satisfies ChartConfig
```

### 6.2 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 7: Frontend — Spending Trend Area Chart

### 7.1 Create SpendingTrendChart Component

**File**: `apps/web/src/components/dashboard/spending-trend-chart.tsx` (new)

Component: `SpendingTrendChart`

Behavior:
- Computes current month start/end dates (same as donut chart)
- Calls `useDailySpending(fromDate, toDate)`
- Transforms daily data into cumulative data: each day's value = sum of all previous days' totals + current day's total
- Shows `Skeleton` while loading
- Shows empty state if no spending data
- Renders a ShadCN `ChartContainer` wrapping a Recharts `AreaChart`:
  - `CartesianGrid` with `vertical={false}`
  - `XAxis` with `dataKey="date"` and `tickFormatter` to show day number (e.g., "1", "5", "10")
  - `YAxis` hidden (clean look) or with formatted currency
  - `Area` component with `type="monotone"` for smooth curve, `dataKey="cumulative"`, `stroke` using primary color, `fill` using primary color with gradient
  - `defs` with `linearGradient` for the fill (solid at top → transparent at bottom)
- `ChartTooltip` with `ChartTooltipContent` showing date and cumulative total (formatted as currency)
- Current total displayed in the card header as supplementary text

Wrapped in a `Card` with `CardHeader` ("Spending Trend") and subtitle ("Cumulative spending this month") and `CardContent`.

Chart config:
```ts
const chartConfig = {
  cumulative: { label: "Total Spent", color: "var(--chart-1)" },
} satisfies ChartConfig
```

### 7.2 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 8: Frontend — Dashboard Layout Integration

### 8.1 Import and Place Charts on Dashboard

**File**: `apps/web/src/app/(dashboard)/page.tsx`

Import all three chart components:
```ts
import { ExpenditureChart } from "@/components/dashboard/expenditure-chart";
import { IncomeExpensesChart } from "@/components/dashboard/income-expenses-chart";
import { SpendingTrendChart } from "@/components/dashboard/spending-trend-chart";
```

Insert between `<BudgetOverview>` and the Recent Transactions `<Card>`:

```tsx
{/* Charts Section */}
<div className="grid gap-4 lg:grid-cols-2">
  <ExpenditureChart />
  <IncomeExpensesChart />
</div>
<SpendingTrendChart />
```

Layout result:
```
[Summary Cards — 4 columns]
[Account Cards — 3 columns]
[Budget Overview — 4 columns]
[Expenditure Donut  |  Income vs Expenses Bar]  ← NEW
[Spending Trend — full width area chart]         ← NEW
[Recent Transactions]
```

### 8.2 Update DashboardSkeleton

Add skeleton placeholders for the chart cards in `DashboardSkeleton` to match the new layout.

### 8.3 Final Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Backend API Reference

### New Endpoints

#### GET /api/v1/transactions/spending-by-category

Get expense totals grouped by category for a date range.

| Param | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | Yes | Start date (RFC3339 or YYYY-MM-DD) |
| `to_date` | string | Yes | End date (RFC3339 or YYYY-MM-DD) |

**Response**: `200`
```json
{
  "items": [
    {
      "category_id": 3,
      "category_name": "Groceries",
      "category_color": "#22C55E",
      "category_icon": "🛒",
      "total": 45000
    },
    {
      "category_id": null,
      "category_name": "Uncategorized",
      "category_color": "#9CA3AF",
      "category_icon": "",
      "total": 12500
    }
  ],
  "total_spent": 57500,
  "from_date": "2026-02-01T00:00:00Z",
  "to_date": "2026-02-28T23:59:59Z"
}
```

#### GET /api/v1/transactions/monthly-summary

Get monthly income and expense totals for the last N months.

| Param | Type | Required | Description |
|---|---|---|---|
| `months` | int | No | Number of months back (default: 6, min: 1, max: 24) |

**Response**: `200`
```json
{
  "data": [
    { "month": "2025-09", "income": 500000, "expenses": 320000 },
    { "month": "2025-10", "income": 480000, "expenses": 350000 }
  ]
}
```

#### GET /api/v1/transactions/daily-spending

Get daily expense totals for a date range.

| Param | Type | Required | Description |
|---|---|---|---|
| `from_date` | string | Yes | Start date (RFC3339 or YYYY-MM-DD) |
| `to_date` | string | Yes | End date (RFC3339 or YYYY-MM-DD) |

**Response**: `200`
```json
{
  "data": [
    { "date": "2026-02-01", "total": 15000 },
    { "date": "2026-02-02", "total": 0 },
    { "date": "2026-02-03", "total": 8500 }
  ]
}
```

---

## File Structure After This Plan

### Backend (new/modified)

```
apps/api/
├── internal/
│   ├── services/
│   │   ├── interfaces.go                    # MODIFIED: add 3 response structs + 3 methods to TransactionServicer
│   │   ├── transaction_service.go           # MODIFIED: implement 3 aggregation methods
│   │   └── transaction_service_test.go      # MODIFIED: add tests for 3 new methods
│   └── handlers/
│       ├── transaction_handler.go           # MODIFIED: add 3 handler functions
│       └── transaction_handler_test.go      # MODIFIED: add 3 mock methods + handler tests
├── cmd/api/
│   └── main.go                              # MODIFIED: register 3 new routes
```

### Frontend (new/modified)

```
apps/web/
├── package.json                              # MODIFIED: recharts added
├── src/
│   ├── components/
│   │   ├── ui/
│   │   │   └── chart.tsx                    # NEW: ShadCN chart component (auto-generated)
│   │   └── dashboard/
│   │       ├── expenditure-chart.tsx         # NEW: donut chart component
│   │       ├── income-expenses-chart.tsx     # NEW: bar chart component
│   │       └── spending-trend-chart.tsx      # NEW: area chart component
│   ├── types/
│   │   └── api.ts                           # MODIFIED: add chart response types
│   ├── hooks/
│   │   └── use-transactions.ts              # MODIFIED: add 3 query hooks + key entries
│   └── app/(dashboard)/
│       └── page.tsx                         # MODIFIED: import charts, insert chart grid
```

---

## Implementation Order

```
Phase 1: Backend — Spending by Category
  1.1  Add SpendingByCategoryItem, SpendingByCategory structs + GetSpendingByCategory to interface
  1.2  Implement GetSpendingByCategory service method
  1.3  Add GetSpendingByCategory handler
  1.4  Register GET /transactions/spending-by-category route
  1.5  Service tests
  1.6  Handler tests
  1.7  Backend verification (./scripts/check.sh)

Phase 2: Backend — Monthly Summary
  2.1  Add MonthlySummaryItem struct + GetMonthlySummary to interface
  2.2  Implement GetMonthlySummary service method
  2.3  Add GetMonthlySummary handler
  2.4  Register GET /transactions/monthly-summary route
  2.5  Service tests
  2.6  Handler tests
  2.7  Backend verification

Phase 3: Backend — Daily Spending
  3.1  Add DailySpendingItem struct + GetDailySpending to interface
  3.2  Implement GetDailySpending service method
  3.3  Add GetDailySpending handler
  3.4  Register GET /transactions/daily-spending route
  3.5  Service tests
  3.6  Handler tests
  3.7  Backend verification

Phase 4: Frontend — ShadCN Charts Setup
  4.1  Install ShadCN chart component (npx shadcn@latest add chart)
  4.2  Add API types for chart data
  4.3  Add React Query hooks for chart data
  4.4  Frontend verification (npm run build)

Phase 5: Frontend — Expenditure Donut Chart
  5.1  Create ExpenditureChart component
  5.2  Frontend verification

Phase 6: Frontend — Income vs. Expenses Bar Chart
  6.1  Create IncomeExpensesChart component
  6.2  Frontend verification

Phase 7: Frontend — Spending Trend Area Chart
  7.1  Create SpendingTrendChart component
  7.2  Frontend verification

Phase 8: Frontend — Dashboard Integration
  8.1  Import and place charts on dashboard page
  8.2  Update DashboardSkeleton
  8.3  Final frontend verification (npm run build)
```

## Verification

**Backend** — after each code change:
```bash
cd apps/api && go build ./...
```
After completing each backend phase:
```bash
cd apps/api && ./scripts/check.sh
```

**Frontend** — after each task:
```bash
cd apps/web && npm run build
```
After completing all phases:
```bash
cd apps/web && npm run build
```

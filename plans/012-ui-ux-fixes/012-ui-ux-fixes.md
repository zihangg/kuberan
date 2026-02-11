# UI/UX Fixes: Investment Charts, Sortable Tables, Securities Types & Dashboard Net Worth

## Context

Several UI/UX issues and missing visualizations have been identified across three pages:

### Issue 1: Investment Page — Missing Charts & Text Misalignment

The investments page shows a portfolio summary with 5 stat cards, a net worth over time chart, a "Holdings by Asset Type" plain table, and an "All Holdings" table. Two problems:

1. **Text misalignment in summary cards** — The ShadCN Card component uses `gap-6` (24px) between `CardHeader` and `CardContent`, causing the subtitle text ("Current market value", "Total invested", etc.) to appear visually misaligned across cards — especially when card titles vary in height (e.g., the Unrealized G/L card has a colored value with a percentage in its CardContent).

2. **No visual charts for portfolio composition** — The "Holdings by Asset Type" section is a plain HTML table. There's no visual chart showing asset allocation or portfolio composition breakdown (cash vs investments vs debt).

### Issue 2: Securities Page — Types Not Showing

The securities list page renders a `Badge` for each security's type using `ASSET_TYPE_LABELS[security.asset_type]`. The label map keys are lowercase (`stock`, `etf`, `bond`, `crypto`, `reit`), but the database stores some `asset_type` values with initial capitals (e.g., `"Stock"` instead of `"stock"`). When the key doesn't match, `ASSET_TYPE_LABELS[security.asset_type]` returns `undefined`, and React renders nothing — the badge appears empty.

The same issue affects the security detail page, where `asset_type` is used both for badge display and for conditional rendering of asset-type-specific fields (bond maturity, crypto network, REIT property type). These string comparisons (`security.asset_type === "bond"`) also fail with capitalized values.

**Root cause**: Securities were inserted into the database with capitalized asset types before the API validator (which enforces lowercase) was added, or via direct SQL. The API validator at `internal/validator/validator.go:102-107` now correctly rejects non-lowercase values, so new securities created via the API are fine. The oracle already has a `normalizeAssetType()` helper for its own internal use, confirming this was a known concern.

**Chosen fix**: Frontend-only — normalize the `asset_type` to lowercase before label lookup and conditional comparisons. The validator already prevents new capitalized entries via the API.

### Issue 3: Dashboard — No Net Worth Over Time Chart

The dashboard shows summary cards (Net Worth, Cash, Investments, Credit Cards), account cards, budget overview, and three charts (expenditure donut, income/expenses bar, spending trend). Now that portfolio snapshots exist (tracking `total_net_worth`, `cash_balance`, `investment_value`, `debt_balance` over time), a net worth over time chart should be added to the dashboard.

The investments page already has a `NetWorthChart` component (defined inline) using `usePortfolioSnapshots`, but it's not available on the dashboard — the most natural place for a high-level financial overview.

### Issue 4: Holdings Table Not Sortable

The "All Holdings" table on the investments page has 8 columns (Symbol, Name, Account, Qty, Price, Market Value, Unrealized G/L, Realized G/L) but no sorting capability. Users can't quickly find their largest positions or best/worst performers.

**Chosen approach**: Client-side sorting of the current page. Data is server-paginated (20 per page), but most users have fewer than 100 holdings, so a single page typically covers all holdings. No backend changes required.

### Current State

1. **Portfolio data is available** — `usePortfolio()` returns `PortfolioSummary` with `holdings_by_type` data (value and count per asset type), and `usePortfolioSnapshots()` returns historical `PortfolioSnapshot` records with cash/investment/debt breakdowns.
2. **Chart infrastructure exists** — Three dashboard chart components already follow the same pattern: ShadCN `Card` > `CardHeader` > `CardContent` > `ChartContainer` (Recharts). The `expenditure-chart.tsx` donut chart is a direct template for the asset allocation chart.
3. **ShadCN Table has no built-in sorting** — The ShadCN `Table` component is a thin styled wrapper around native HTML table elements. Sorting must be implemented manually with state management.
4. **The `usePortfolioSnapshots` hook already exists** and is fully functional with period-based date filtering.

### What This Plan Adds

1. **Investment page: Fix summary card text alignment** — Tighten card spacing to eliminate visual misalignment.
2. **Investment page: Asset allocation donut chart** — Donut/pie chart showing portfolio allocation by asset type (stocks, ETFs, bonds, crypto, REITs).
3. **Investment page: Portfolio composition bar chart** — Horizontal stacked bar showing Cash vs Investments vs Debt breakdown from snapshot data.
4. **Investment page: Sortable holdings table** — Click-to-sort column headers with ascending/descending/default cycling.
5. **Securities page: Fix type display** — Case-insensitive label lookup so capitalized DB values render correctly.
6. **Dashboard: Net worth over time chart** — Area chart with period selector, adapted from the investments page's inline `NetWorthChart`.

**Out of scope**: Backend changes, database migration to fix capitalized asset types, server-side sorting for the holdings table, adding `@tanstack/react-table`.

## Scope Summary

| Feature | Type | Changes |
|---|---|---|
| Fix summary card text alignment | Frontend (investments) | CSS class adjustment on Card components |
| Asset allocation donut chart | Frontend (investments) | New component + page layout change |
| Portfolio composition bar chart | Frontend (investments) | New component + page layout change |
| Sortable holdings table | Frontend (investments) | State management + sorting logic + UI indicators |
| Fix securities type display | Frontend (securities) | Case-insensitive label lookup |
| Dashboard net worth chart | Frontend (dashboard) | New component + dashboard page layout change |

## Technology & Patterns

- **Charts** follow existing patterns: Recharts (`PieChart`, `BarChart`, `AreaChart`) wrapped in ShadCN `ChartContainer` with `ChartTooltip` and `ChartConfig`.
- **Donut chart** follows `expenditure-chart.tsx` pattern: `PieChart` + `Pie` + `Label` for center text.
- **Sorting** uses React `useState` + `useMemo` — no new dependencies.
- **Icons** use Lucide (`ArrowUpDown`, `ArrowUp`, `ArrowDown`) for sort indicators.
- **All data hooks already exist** — `usePortfolio()`, `usePortfolioSnapshots()`, `useAllInvestments()`. No new API calls needed.

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Card alignment fix | Reduce gap with `className="gap-2"` on summary cards | The default `gap-6` between `CardHeader` and `CardContent` is too large for compact stat cards. `gap-2` tightens the spacing so all cards align visually regardless of content height. |
| Securities type fix | Frontend `toLowerCase()` normalization | The API validator already prevents new capitalized entries. A DB migration is cleaner but out of scope. Frontend normalization is safe and handles any existing mixed-case data. |
| Holdings sort approach | Client-side sort of current page | Most users have <100 holdings (single page). Client-side is simpler, no backend changes. Sort state resets on page change. |
| Sort cycling | none → ascending → descending → none | Three-state toggle. Default (none) preserves server order. Visual indicators make current state clear. |
| Dashboard chart placement | Between budget overview and existing charts row | Net worth is the most important high-level metric — it deserves prominence above the transaction-level charts. |
| Net worth chart | Extract as reusable component from investments page | The investments page already has the exact same chart inline. Extract it so both pages can use the same component, or create a separate dashboard-specific version that's simpler. |
| Donut chart data | `portfolio.holdings_by_type` from `usePortfolio()` | Already fetched — no additional API call. Provides value + count per asset type. |
| Composition bar data | Latest snapshot from `usePortfolioSnapshots()` | Snapshot has `cash_balance`, `investment_value`, `debt_balance` — exactly the breakdown needed. |

---

## Phase 1: Fix Summary Card Alignment

### 1.1 Tighten Card Spacing

**File**: `apps/web/src/app/(dashboard)/investments/page.tsx`

Add `className="gap-2"` to each of the 5 summary `<Card>` components in the grid (lines 429-525). This overrides the default `gap-6` from the Card component's base styles, tightening the space between `CardHeader` (label + big number) and `CardContent` (subtitle text).

Before:
```tsx
<Card>
  <CardHeader>...</CardHeader>
  <CardContent>...</CardContent>
</Card>
```

After:
```tsx
<Card className="gap-2">
  <CardHeader>...</CardHeader>
  <CardContent>...</CardContent>
</Card>
```

Apply to all 5 cards: Total Value, Cost Basis, Unrealized G/L, Realized G/L, Holdings.

### 1.2 Verification

Run `pnpm build` from `apps/web/` to confirm no TypeScript errors. Visually verify card alignment in the browser.

---

## Phase 2: Investment Page Charts

### 2.1 Asset Allocation Donut Chart

**New file**: `apps/web/src/components/investments/asset-allocation-chart.tsx`

A donut/pie chart showing portfolio allocation by asset type. Pattern follows `expenditure-chart.tsx`.

**Props**:
```tsx
interface AssetAllocationChartProps {
  holdingsByType: Record<AssetType, { value: number; count: number }>;
  totalValue: number;
}
```

**Implementation**:
- Filter to types with `count > 0`.
- Map each type to `{ name: ASSET_TYPE_LABELS[type], value: holding.value / 100, fill: chartColor }`.
- Assign distinct colors per asset type via `ChartConfig` (e.g., stocks = blue, ETFs = green, bonds = amber, crypto = purple, REITs = orange).
- Use Recharts `PieChart` + `Pie` with `innerRadius` and `outerRadius` for donut shape.
- Center `Label` shows total portfolio value formatted via `formatCurrency`.
- Wrap in ShadCN `Card` with title "Asset Allocation" and description "Portfolio breakdown by asset type".
- Use `ChartContainer` + `ChartTooltip` with custom tooltip showing formatted currency + percentage.

### 2.2 Portfolio Composition Bar Chart

**New file**: `apps/web/src/components/investments/portfolio-composition-chart.tsx`

A horizontal stacked bar chart showing Cash vs Investments vs Debt from the latest snapshot.

**Props**: None — fetches its own data via `usePortfolioSnapshots()`.

**Implementation**:
- Fetch latest snapshot using `usePortfolioSnapshots()` with `from_date` = 10 years ago, `to_date` = today, `page_size: 1`.
- Extract `cash_balance`, `investment_value`, `debt_balance` from the most recent snapshot.
- Convert to dollars (divide by 100).
- Render a Recharts `BarChart` with `layout="vertical"` and 3 stacked `Bar` components.
- Colors: green for cash, blue for investments, orange/red for debt.
- ShadCN `Card` wrapper with title "Portfolio Composition" and description "Breakdown of your net worth".
- Include legend via `ChartLegend` + `ChartLegendContent`.
- Custom tooltip showing formatted currency values.
- Handle loading and empty states.

### 2.3 Integrate Charts Into Investments Page

**File**: `apps/web/src/app/(dashboard)/investments/page.tsx`

- Import both new chart components.
- Add a 2-column grid row between the `NetWorthChart` and the "Holdings by Asset Type" table:
  ```tsx
  <div className="grid gap-4 lg:grid-cols-2">
    <AssetAllocationChart
      holdingsByType={portfolio.holdings_by_type}
      totalValue={portfolio.total_value}
    />
    <PortfolioCompositionChart />
  </div>
  ```

### 2.4 Verification

Run `pnpm build` from `apps/web/`. Visually verify charts render with correct data.

---

## Phase 3: Sortable Holdings Table

### 3.1 Add Sort State

**File**: `apps/web/src/app/(dashboard)/investments/page.tsx`

In the `AllHoldingsTable` component, add state:
```tsx
type SortColumn = "symbol" | "name" | "qty" | "price" | "market_value" | "unrealized_gl" | "realized_gl";
type SortDirection = "asc" | "desc" | null;

const [sortColumn, setSortColumn] = useState<SortColumn | null>(null);
const [sortDirection, setSortDirection] = useState<SortDirection>(null);
```

Add a `handleSort` function that cycles: `null` → `"asc"` → `"desc"` → `null`.

### 3.2 Sort Logic

Add a `useMemo` that sorts the `investments` array based on `sortColumn` and `sortDirection`:
```tsx
const sortedInvestments = useMemo(() => {
  if (!sortColumn || !sortDirection) return investments;
  return [...investments].sort((a, b) => {
    let aVal, bVal;
    switch (sortColumn) {
      case "symbol": aVal = a.security.symbol; bVal = b.security.symbol; break;
      case "name": aVal = a.security.name; bVal = b.security.name; break;
      case "qty": aVal = a.quantity; bVal = b.quantity; break;
      case "price": aVal = a.current_price; bVal = b.current_price; break;
      case "market_value":
        aVal = Math.round(a.quantity * a.current_price);
        bVal = Math.round(b.quantity * b.current_price);
        break;
      case "unrealized_gl":
        aVal = Math.round(a.quantity * a.current_price) - a.cost_basis;
        bVal = Math.round(b.quantity * b.current_price) - b.cost_basis;
        break;
      case "realized_gl": aVal = a.realized_gain_loss; bVal = b.realized_gain_loss; break;
    }
    if (typeof aVal === "string") return sortDirection === "asc" ? aVal.localeCompare(bVal) : bVal.localeCompare(aVal);
    return sortDirection === "asc" ? aVal - bVal : bVal - aVal;
  });
}, [investments, sortColumn, sortDirection]);
```

### 3.3 Sortable Column Headers

Replace static `<TableHead>` elements with clickable headers. Create a `SortableHeader` helper:

```tsx
function SortableHeader({
  column, label, align, currentColumn, currentDirection, onSort,
}: { ... }) {
  const isActive = currentColumn === column;
  const Icon = isActive && currentDirection === "asc" ? ArrowUp
    : isActive && currentDirection === "desc" ? ArrowDown
    : ArrowUpDown;
  return (
    <TableHead className={`cursor-pointer select-none ${align}`} onClick={() => onSort(column)}>
      <div className={`flex items-center gap-1 ${align === "text-right" ? "justify-end" : ""}`}>
        {label}
        <Icon className={`h-3 w-3 ${isActive ? "text-foreground" : "text-muted-foreground/50"}`} />
      </div>
    </TableHead>
  );
}
```

Apply to all 7 sortable columns (Symbol, Name, Qty, Price, Market Value, Unrealized G/L, Realized G/L). The Account column remains non-sortable.

Add `ArrowUp`, `ArrowDown`, `ArrowUpDown` to the Lucide imports.

### 3.4 Render Sorted Data

Replace `investments.map(...)` with `sortedInvestments.map(...)` in the table body.

### 3.5 Verification

Run `pnpm build` from `apps/web/`. Verify sorting works correctly for each column in both directions.

---

## Phase 4: Fix Securities Type Display

### 4.1 Securities List Page

**File**: `apps/web/src/app/(dashboard)/securities/page.tsx`

In the `SecurityRow` component, change the badge rendering from:
```tsx
{ASSET_TYPE_LABELS[security.asset_type]}
```
to:
```tsx
{ASSET_TYPE_LABELS[security.asset_type.toLowerCase() as AssetType] ?? security.asset_type}
```

This normalizes the key to lowercase before lookup, and falls back to the raw value if it's still not in the map.

### 4.2 Security Detail Page

**File**: `apps/web/src/app/(dashboard)/securities/[id]/page.tsx`

Add a normalized asset type variable near the top of the component body:
```tsx
const assetType = security.asset_type.toLowerCase() as AssetType;
```

Use `assetType` for:
1. **Badge display**: `ASSET_TYPE_LABELS[assetType] ?? security.asset_type`
2. **Bond-specific fields**: `assetType === "bond" && ...`
3. **Crypto-specific fields**: `assetType === "crypto" && ...`
4. **REIT-specific fields**: `assetType === "reit" && ...`

### 4.3 Verification

Run `pnpm build` from `apps/web/`. Verify that securities with capitalized types now display correctly on both list and detail pages.

---

## Phase 5: Dashboard Net Worth Over Time Chart

### 5.1 Create Net Worth Chart Component

**New file**: `apps/web/src/components/dashboard/net-worth-chart.tsx`

An area chart showing net worth over time with period selector tabs. Adapted from the investments page's inline `NetWorthChart` component.

**Implementation**:
- Period tabs: 1M, 3M, 6M, 1Y, ALL (same as investments page).
- Uses `usePortfolioSnapshots()` with computed date range based on selected period.
- Maps snapshot data to `{ date: formatted_date, net_worth: total_net_worth / 100 }`.
- Recharts `AreaChart` with gradient fill (same pattern as investments page).
- `ChartContainer` + `ChartTooltip` with custom tooltip showing formatted currency.
- ShadCN `Card` wrapper with title "Net Worth Over Time" and description showing the latest net worth value.
- Handle loading state (skeleton) and empty state ("No snapshot data available").

### 5.2 Integrate Into Dashboard Page

**File**: `apps/web/src/app/(dashboard)/page.tsx`

- Import the new `NetWorthChart` component.
- Add it between the `BudgetOverview` section and the existing 2-column charts row:

```tsx
<BudgetOverview budgets={activeBudgets} />

<NetWorthChart />

<div className="grid gap-4 lg:grid-cols-2">
  <ExpenditureChart />
  <IncomeExpensesChart />
</div>
```

- Update the `DashboardSkeleton` to include a skeleton placeholder for the new chart.

### 5.3 Verification

Run `pnpm build` from `apps/web/`. Verify the net worth chart appears on the dashboard with correct data and period switching.

---

## Phase 6: Final Verification

### 6.1 Full Build

Run `pnpm build` from `apps/web/`. Must complete with no errors.

### 6.2 Visual Verification Checklist

- [ ] **Investment page — Card alignment**: All 5 summary cards have consistent vertical spacing between the big number and subtitle text
- [ ] **Investment page — Asset allocation donut**: Donut chart renders with correct colors per asset type, center label shows total value, tooltip shows value + percentage
- [ ] **Investment page — Portfolio composition bar**: Stacked bar renders showing cash/investments/debt breakdown, legend and tooltip display correctly
- [ ] **Investment page — Sortable table**: Clicking column headers toggles ascending/descending/default, sort icons reflect state, data re-orders correctly
- [ ] **Securities list — Type badges**: All securities show their asset type badge (stock, ETF, bond, crypto, REIT) regardless of database casing
- [ ] **Security detail — Type badge + conditional fields**: Type badge renders correctly, bond/crypto/REIT-specific fields still appear when applicable
- [ ] **Dashboard — Net worth chart**: Area chart renders between budget overview and existing charts, period tabs (1M/3M/6M/1Y/ALL) switch correctly, tooltip shows formatted currency
- [ ] **Dashboard — Empty state**: If no snapshots exist, chart shows "No snapshot data available" message (not a blank card or error)

---

## Files Changed

### New Files

```
apps/web/src/components/
├── investments/
│   ├── asset-allocation-chart.tsx
│   └── portfolio-composition-chart.tsx
└── dashboard/
    └── net-worth-chart.tsx
```

### Modified Files

```
apps/web/src/app/(dashboard)/
├── investments/
│   └── page.tsx                          # Card alignment, chart integration, sortable table
├── securities/
│   ├── page.tsx                          # Case-insensitive asset type label lookup
│   └── [id]/
│       └── page.tsx                      # Case-insensitive asset type for badge + conditionals
└── page.tsx                              # Add net worth chart to dashboard
```

---

## Implementation Order

```
Phase 1: Fix Summary Card Alignment
  1.1  Add gap-2 class to summary Card components
  1.2  Verification (pnpm build)

Phase 2: Investment Page Charts
  2.1  Create asset-allocation-chart.tsx (donut chart)
  2.2  Create portfolio-composition-chart.tsx (stacked bar chart)
  2.3  Integrate charts into investments page layout
  2.4  Verification (pnpm build)

Phase 3: Sortable Holdings Table
  3.1  Add sort state (column + direction)
  3.2  Sort logic (useMemo with comparator)
  3.3  Sortable column headers (SortableHeader component + icons)
  3.4  Render sorted data
  3.5  Verification (pnpm build)

Phase 4: Fix Securities Type Display
  4.1  Securities list page — case-insensitive label lookup
  4.2  Security detail page — normalize asset_type for badge + conditionals
  4.3  Verification (pnpm build)

Phase 5: Dashboard Net Worth Over Time Chart
  5.1  Create net-worth-chart.tsx component
  5.2  Integrate into dashboard page
  5.3  Verification (pnpm build)

Phase 6: Final Verification
  6.1  Full build (pnpm build from apps/web/)
  6.2  Visual verification checklist
```

## Verification

**After each file change**:
```bash
cd apps/web && pnpm build
```

**After completing all phases**:
```bash
cd apps/web && pnpm build
```

No backend changes — no need to run `./scripts/check-go.sh`.

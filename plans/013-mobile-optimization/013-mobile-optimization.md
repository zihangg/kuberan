# Mobile Optimization: Complete Responsive Design for 375px-768px Devices

## Context

The Kuberan MVP is complete and functional on desktop, but mobile compatibility has not been thoroughly tested or optimized. Users accessing the application on mobile devices (375px-768px) will encounter several UX issues that make the app difficult or frustrating to use.

### Target Devices

Per user requirements, we're targeting the standard mobile breakpoint range:
- **iPhone SE**: 375px × 667px (smallest target)
- **iPhone 12-14**: 390px × 844px
- **iPhone 14 Pro Max**: 428px × 926px
- **Small tablets**: 768px × 1024px

### Known Issues (All Categories Per User Feedback)

1. **Tables overflow and are hard to use** — All list pages (accounts, transactions, categories) use desktop-optimized table layouts that don't work well on small screens
2. **Dialogs and forms feel cramped** — Button groups, form fields, and dialog content need better spacing for touch interaction
3. **Charts don't display well** — Dashboard and investment charts may be too small or have overlapping labels on mobile
4. **Touch targets too small** — Some icon buttons and interactive elements are below the 44px minimum touch target size
5. **Filter rows problematic** — Multi-select filter rows (5 filters on transactions page) stack poorly on mobile
6. **Navigation needs mobile drawer** — Sidebar should work as a mobile drawer (user confirmed keeping sidebar approach vs. bottom nav)

### Current State

**Strengths**:
- ✅ Mobile breakpoint hook exists (`use-mobile.ts` with 768px threshold)
- ✅ ShadCN Sidebar has built-in responsive drawer support
- ✅ Tailwind CSS v4 provides modern responsive utilities
- ✅ Tables already wrapped in `overflow-x-auto` containers
- ✅ Dialogs use responsive max-width: `max-w-[calc(100%-2rem)]`
- ✅ Header actions already hide text on mobile (`hidden xl:inline` for search button text)
- ✅ User name hides on small screens (`hidden sm:inline`)

**Weaknesses**:
- ❌ Button touch targets: `size="icon"` is 36px (should be 44px minimum)
- ❌ No card/list alternative for tables on mobile
- ❌ Form button groups (3 buttons side-by-side) too cramped at 375px
- ❌ No collapsible filter panel for mobile (5 filters stack vertically = tall page)
- ❌ Chart heights and responsive sizing not optimized
- ❌ No touch-action CSS optimization (300ms tap delay)
- ❌ Pagination buttons may have text overflow on small screens

### What This Plan Adds

This plan transforms Kuberan into a fully mobile-responsive application by:

1. **Foundation**: Touch optimizations, viewport compliance, button size fixes
2. **Tables**: Card/list layouts on mobile for accounts, transactions, categories
3. **Charts**: Responsive sizing, readable tooltips, optimized heights for mobile
4. **Dialogs/Forms**: Better spacing, stacked button groups, improved touch targets
5. **Filters**: Collapsible filter panels with active filter badges
6. **Detail Pages**: Mobile-optimized layouts for account and investment detail views
7. **Component Audit**: Ensure all UI primitives meet mobile accessibility standards

**Out of scope**: Bottom navigation bar, PWA features (offline support, install prompts), swipe gestures (pull-to-refresh, swipe-to-delete), infinite scroll pagination.

## Scope Summary

| Feature | Type | Changes | Priority |
|---------|------|---------|----------|
| Touch optimization & button fixes | Foundation | globals.css, button.tsx | CRITICAL |
| Viewport & horizontal scroll fixes | Foundation | layout.tsx, all pages | CRITICAL |
| Accounts page mobile cards | Tables | accounts/page.tsx | HIGH |
| Transactions page mobile list | Tables | transactions/page.tsx | HIGH |
| Categories page mobile cards | Tables | categories/page.tsx | HIGH |
| Dashboard charts responsive | Charts | 4 chart components | HIGH |
| Transaction dialog form stacking | Forms | create/edit dialogs | HIGH |
| Collapsible filter panels | Filters | transactions/page.tsx | HIGH |
| Account detail mobile layout | Detail Pages | accounts/[id]/page.tsx | MEDIUM |
| Investment pages mobile layout | Detail Pages | investments/, securities/ | MEDIUM |
| Budgets page filter stacking | Budgets | budgets/page.tsx | MEDIUM |
| Form component touch targets | Components | input, select, textarea | MEDIUM |

## Technology & Patterns

- **Responsive patterns**: `<md` for mobile, `md:` for tablet/desktop breakpoint (768px)
- **Card/Table switching**: Render mobile cards below md breakpoint, table above
- **Collapsible filters**: Toggle button + conditional render, active filter badge count
- **Touch targets**: Minimum 44×44px for all interactive elements
- **Button stacking**: `flex-col sm:flex-row` for button groups
- **Chart responsiveness**: Conditional heights via `max-h-[200px] md:max-h-[250px]`
- **Touch optimization**: `touch-action: manipulation` to eliminate 300ms tap delay
- **No horizontal scroll**: `overflow-x-hidden` on body, `max-w-full` on containers

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Mobile breakpoint | 768px (md:) | Matches ShadCN sidebar breakpoint and standard tablet size. Most phones are <768px, most tablets ≥768px. |
| Navigation pattern | Keep sidebar as mobile drawer | User requirement. ShadCN sidebar has built-in drawer support. No bottom nav needed. |
| Table alternative | Card layout | Cards stack vertically, show essential info, are touch-friendly. Better than horizontal scroll tables. |
| Filter approach | Collapsible panel behind button | Saves vertical space. Badge shows active filter count so users know filters are applied. |
| Button touch size | Increase icon buttons to 44px | WCAG 2.1 Level AAA guideline. Critical for usability on touch devices. |
| Chart heights | Reduce on mobile (200px vs 250px) | Prevents charts from dominating small screens while keeping them readable. |
| Dialog button groups | Stack on <sm (640px) | 3 buttons side-by-side are too cramped at 375px width. Stacking is clearer. |
| Form field spacing | Increase gap from gap-4 to gap-5 | Better touch spacing between adjacent form fields (20px minimum recommended). |
| Pagination | Icon-only on mobile | Text like "Previous"/"Next" can overflow. Icons (ChevronLeft/Right) are universal. |

---

## Phase 1: Foundation & Core Layout (CRITICAL)

### 1.1 Global Touch Optimizations

**File**: `apps/web/src/app/globals.css`

Add touch-action CSS to eliminate 300ms tap delay and prevent unwanted pull-to-refresh:

```css
@layer base {
  * {
    @apply border-border outline-ring/50;
    touch-action: manipulation; /* Eliminate 300ms tap delay */
  }
  body {
    @apply bg-background text-foreground;
    overscroll-behavior-y: contain; /* Prevent pull-to-refresh */
    overflow-x: hidden; /* Prevent horizontal scroll */
  }
}
```

### 1.2 Viewport Verification

**File**: `apps/web/src/app/layout.tsx`

Verify viewport meta tag is present in Next.js config. If using App Router, Next.js adds this automatically. No changes needed unless missing.

Verify by checking page source for:
```html
<meta name="viewport" content="width=device-width, initial-scale=1">
```

### 1.3 Fix Button Touch Target Sizes

**File**: `apps/web/src/components/ui/button.tsx`

Update button size variants to meet 44px minimum:

**Before**:
```tsx
size: {
  default: "h-9 px-4 py-2 has-[>svg]:px-3",
  sm: "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
  icon: "size-9", // 36px — too small
  "icon-sm": "size-8", // 32px — too small
}
```

**After**:
```tsx
size: {
  default: "h-10 px-4 py-2 has-[>svg]:px-3", // 40px (acceptable)
  sm: "h-9 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5", // 36px (acceptable for secondary)
  icon: "size-11", // 44px — meets minimum
  "icon-sm": "size-9", // 36px (only for secondary icon buttons)
  "icon-lg": "size-12", // 48px (for prominent actions)
}
```

### 1.4 Header Responsiveness Check

**File**: `apps/web/src/components/layout/app-header.tsx`

Verify existing responsive patterns are working:
- ✅ Line 40-49: Search button shows icon-only on mobile, text on xl+
- ✅ Line 82: User name hidden on <sm, visible on sm+
- ✅ Line 35: SidebarTrigger (hamburger menu) for mobile drawer

**Action**: No changes needed. Verify in browser at 375px width.

### 1.5 Sidebar Mobile Drawer Verification

**File**: `apps/web/src/app/(dashboard)/layout.tsx`

The ShadCN `SidebarProvider` + `SidebarInset` pattern should automatically convert sidebar to drawer on mobile (<md).

**Action**: Test sidebar drawer functionality at 375px, 428px, and 768px widths. Ensure:
- Sidebar is hidden by default on mobile
- SidebarTrigger (hamburger) toggles drawer open/closed
- Drawer overlays content with backdrop
- Drawer closes on navigation or backdrop click

### 1.6 Verification

```bash
cd apps/web
pnpm build
```

Test in browser at 375px, 390px, 428px, 768px widths. Verify:
- No horizontal scroll on any page
- All buttons are tappable (no 300ms delay)
- Icon buttons are large enough to tap accurately
- Sidebar works as mobile drawer

---

## Phase 2: Table to Card/List Conversions (HIGH)

### 2.1 Accounts Page Mobile Cards

**File**: `apps/web/src/app/(dashboard)/accounts/page.tsx`

**Current issue**: 6-column table (Name, Type, Balance, Currency, Status, Created) is too wide for mobile.

**Solution**: Card layout on mobile, table on desktop.

**Implementation**:

1. Create `AccountCard` component (reusable):
```tsx
function AccountCard({ account }: { account: Account }) {
  return (
    <Link href={`/accounts/${account.id}`}>
      <Card className="transition-colors hover:bg-accent/50 cursor-pointer">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-2">
            <CardTitle className="text-base truncate">{account.name}</CardTitle>
            <Badge variant="secondary" className="shrink-0">
              {ACCOUNT_TYPE_LABELS[account.type]}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-2">
          <p className="text-2xl font-semibold">
            {formatCurrency(account.balance, account.currency)}
          </p>
          <div className="flex items-center justify-between text-sm text-muted-foreground">
            <div className="flex items-center gap-2">
              <Badge variant={account.is_active ? "outline" : "secondary"} className="text-xs">
                {account.is_active ? "Active" : "Inactive"}
              </Badge>
              <span className="text-xs">{account.currency}</span>
            </div>
            <span className="text-xs">{formatDate(account.created_at)}</span>
          </div>
        </CardContent>
      </Card>
    </Link>
  );
}
```

2. Implement responsive rendering in `AccountsPage` component:
```tsx
// Replace current table rendering with:

{/* Mobile: Card Grid */}
<div className="md:hidden">
  <div className="grid gap-3">
    {accounts.map((account) => (
      <AccountCard key={account.id} account={account} />
    ))}
  </div>
</div>

{/* Desktop: Table */}
<div className="hidden md:block">
  <Table>
    {/* Existing table structure */}
  </Table>
</div>
```

3. Update loading skeleton to show card skeletons on mobile:
```tsx
function AccountsTableSkeleton() {
  return (
    <>
      {/* Mobile: Card skeletons */}
      <div className="md:hidden grid gap-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-32 w-full rounded-lg" />
        ))}
      </div>
      
      {/* Desktop: Table skeleton */}
      <div className="hidden md:block space-y-3">
        <Skeleton className="h-10 w-full" />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full" />
        ))}
      </div>
    </>
  );
}
```

### 2.2 Transactions Page Mobile List

**File**: `apps/web/src/app/(dashboard)/transactions/page.tsx`

**Current issue**: 6-column table + 5-filter row is not mobile-friendly.

**Solution**: 
1. Mobile list view with compact transaction cards
2. Collapsible filter panel behind "Filters" button
3. Icon-only pagination buttons

**Implementation**:

1. Create `TransactionListItem` component:
```tsx
function TransactionListItem({
  transaction,
  accountName,
  onClick,
}: {
  transaction: Transaction;
  accountName: string;
  onClick?: () => void;
}) {
  const config = TRANSACTION_TYPE_CONFIG[transaction.type];
  const Icon = config.icon;
  const isNegative = transaction.type === "expense" || transaction.type === "transfer";

  return (
    <div
      className="flex items-center justify-between py-3 px-3 -mx-3 rounded-md cursor-pointer hover:bg-accent/50 transition-colors"
      onClick={onClick}
    >
      <div className="flex items-center gap-3 min-w-0 flex-1">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-muted">
          <Icon className={`h-5 w-5 ${config.color}`} />
        </div>
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium truncate">
            {transaction.description || config.label}
          </p>
          <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <span>{formatDate(transaction.date)}</span>
            <span>·</span>
            <span className="truncate">{accountName}</span>
            {transaction.category && (
              <>
                <span>·</span>
                <span className="truncate">{transaction.category.name}</span>
              </>
            )}
          </div>
        </div>
      </div>
      <span className={`text-sm font-medium shrink-0 ml-3 ${config.color}`}>
        {isNegative ? "-" : "+"}
        {formatCurrency(transaction.amount)}
      </span>
    </div>
  );
}
```

2. Add collapsible filter state:
```tsx
const [showFilters, setShowFilters] = useState(false);
const activeFilterCount = [
  accountFilter !== "all",
  typeFilter !== "all",
  categoryFilter !== "all",
  fromDate !== "",
  toDate !== "",
].filter(Boolean).length;
```

3. Implement filter panel toggle button:
```tsx
{/* Mobile: Filter toggle button */}
<div className="md:hidden">
  <Button
    variant="outline"
    size="sm"
    onClick={() => setShowFilters(!showFilters)}
    className="w-full"
  >
    <Filter className="h-4 w-4 mr-2" />
    Filters
    {activeFilterCount > 0 && (
      <Badge variant="secondary" className="ml-2">
        {activeFilterCount}
      </Badge>
    )}
  </Button>
  {showFilters && (
    <div className="mt-3 grid gap-3 grid-cols-2">
      {/* All 5 filter selects/inputs, 2 per row */}
      {/* Date inputs span full width: col-span-2 */}
    </div>
  )}
</div>

{/* Desktop: Always visible filters */}
<div className="hidden md:grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
  {/* Existing filter structure */}
</div>
```

4. Responsive transaction list rendering:
```tsx
{/* Mobile: List view */}
<div className="md:hidden">
  <div className="divide-y">
    {transactions.map((tx) => (
      <TransactionListItem
        key={tx.id}
        transaction={tx}
        accountName={accountNameMap.get(tx.account_id) ?? `Account #${tx.account_id}`}
        onClick={() => {
          setSelectedTransaction(tx);
          setEditTxOpen(true);
        }}
      />
    ))}
  </div>
</div>

{/* Desktop: Table view */}
<div className="hidden md:block">
  <Table>
    {/* Existing table */}
  </Table>
</div>
```

5. Make pagination mobile-friendly:
```tsx
{/* Replace existing pagination buttons */}
<Button
  variant="outline"
  size="sm"
  disabled={page <= 1}
  onClick={() => setPage((p) => p - 1)}
>
  <ChevronLeft className="h-4 w-4" />
  <span className="ml-1 hidden sm:inline">Previous</span>
</Button>
<Button
  variant="outline"
  size="sm"
  disabled={page >= totalPages}
  onClick={() => setPage((p) => p + 1)}
>
  <span className="mr-1 hidden sm:inline">Next</span>
  <ChevronRight className="h-4 w-4" />
</Button>
```

6. Add Filter icon to imports:
```tsx
import { ..., Filter } from "lucide-react";
```

### 2.3 Categories Page Mobile Cards

**File**: `apps/web/src/app/(dashboard)/categories/page.tsx`

Similar pattern to accounts page. Create `CategoryCard` component showing:
- Name + Type badge (Income/Expense)
- Icon/color indicator
- Transaction count or other metadata

Implementation follows 2.1 pattern with adjusted card content.

### 2.4 Verification

```bash
cd apps/web
pnpm build
```

Test at 375px, 428px, 768px:
- [ ] Accounts show as cards on mobile, table on desktop
- [ ] Transactions show as list items on mobile with tappable rows
- [ ] Filter button shows/hides filter panel on mobile
- [ ] Active filter badge shows correct count
- [ ] Pagination buttons show icons only on mobile
- [ ] Categories show as cards on mobile

---

## Phase 3: Dashboard & Chart Responsiveness (HIGH)

### 3.1 Dashboard Header Actions

**File**: `apps/web/src/app/(dashboard)/page.tsx`

Lines 363-378 have two action buttons in the header. Stack them on very small screens:

```tsx
<div className="flex flex-col sm:flex-row gap-2">
  <Button asChild variant="outline" size="sm">
    <Link href="/accounts">
      <Wallet className="h-4 w-4" />
      <span className="ml-2">Add Account</span>
    </Link>
  </Button>
  <Button variant="outline" size="sm" onClick={() => setTxDialogOpen(true)}>
    <Plus className="h-4 w-4" />
    <span className="ml-2">Add Transaction</span>
  </Button>
</div>
```

Alternatively, for very cramped screens, consider icon-only buttons with tooltips:

```tsx
<div className="flex gap-2">
  <Tooltip>
    <TooltipTrigger asChild>
      <Button asChild variant="outline" size="icon" className="sm:hidden">
        <Link href="/accounts">
          <Wallet className="h-4 w-4" />
        </Link>
      </Button>
    </TooltipTrigger>
    <TooltipContent>Add Account</TooltipContent>
  </Tooltip>
  
  {/* Desktop: Button with text */}
  <Button asChild variant="outline" size="sm" className="hidden sm:flex">
    <Link href="/accounts">
      <Wallet className="h-4 w-4" />
      <span className="ml-2">Add Account</span>
    </Link>
  </Button>
  
  {/* Repeat for Add Transaction */}
</div>
```

Choose stacking approach for simplicity. Icon-only is optional enhancement.

### 3.2 Expenditure Chart (Donut)

**File**: `apps/web/src/components/dashboard/expenditure-chart.tsx`

**Changes**:
1. Reduce chart max-height on mobile:
```tsx
<ChartContainer
  config={chartConfig}
  className="mx-auto aspect-square max-h-[200px] md:max-h-[250px]"
>
```

2. Reduce font sizes in center label for mobile:
```tsx
<tspan
  x={viewBox.cx}
  y={viewBox.cy}
  className="fill-foreground text-sm md:text-base font-bold"
>
  {formatCurrency(data.total_spent)}
</tspan>
<tspan
  x={viewBox.cx}
  y={(viewBox.cy || 0) + 18}
  className="fill-muted-foreground text-[10px] md:text-xs"
>
  This Month
</tspan>
```

3. Ensure tooltip is readable on small screens (current custom tooltip is already concise).

### 3.3 Income/Expenses Chart (Bar)

**File**: `apps/web/src/components/dashboard/income-expenses-chart.tsx`

**Changes**:
1. Reduce chart max-height on mobile:
```tsx
<ChartContainer config={chartConfig} className="h-[200px] md:h-[250px]">
```

2. Reduce bar radius on mobile for better visibility:
```tsx
<Bar dataKey="income" fill="var(--chart-1)" radius={[4, 4, 0, 0]} />
<Bar dataKey="expense" fill="var(--chart-2)" radius={[4, 4, 0, 0]} />
```

3. Consider hiding Y-axis labels on very small screens (optional):
```tsx
<YAxis
  tickFormatter={formatCurrency}
  className="text-xs"
  width={60} // Reduce from default 80 to save space
/>
```

### 3.4 Spending Trend Chart (Area)

**File**: `apps/web/src/components/dashboard/spending-trend-chart.tsx`

**Changes**:
1. Reduce chart max-height:
```tsx
<ChartContainer config={chartConfig} className="h-[250px] md:h-[300px]">
```

2. Reduce X-axis tick count on mobile to prevent label overlap:
```tsx
<XAxis
  dataKey="date"
  tickFormatter={(value) => new Date(value).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
  className="text-xs"
  interval={isMobile ? "preserveStartEnd" : "preserveStartEnd"}
  minTickGap={isMobile ? 50 : 30}
/>
```

3. Add `useIsMobile` hook import:
```tsx
import { useIsMobile } from "@/hooks/use-mobile";

export function SpendingTrendChart() {
  const isMobile = useIsMobile();
  // ... rest of component
}
```

### 3.5 Net Worth Chart (Area)

**File**: `apps/web/src/components/dashboard/net-worth-chart.tsx`

Apply same changes as Spending Trend Chart:
- Reduce height on mobile
- Reduce tick count/increase minTickGap
- Use `useIsMobile` hook

### 3.6 Dashboard Summary Cards

**File**: `apps/web/src/app/(dashboard)/page.tsx`

Verify summary cards (lines 116-196) stack correctly:
- Current grid: `sm:grid-cols-2 lg:grid-cols-4` ✅
- On mobile (<640px): 1 column (stacked)
- On tablet (640-1024px): 2 columns
- On desktop (1024px+): 4 columns

**No changes needed** — already responsive.

### 3.7 Verification

```bash
cd apps/web
pnpm build
```

Test at 375px, 428px, 768px:
- [ ] Dashboard header buttons stack or show as icon-only on mobile
- [ ] Expenditure donut chart readable at 200px height
- [ ] Income/expenses bar chart readable with reduced height
- [ ] Spending trend chart labels don't overlap
- [ ] Net worth chart labels don't overlap
- [ ] Summary cards stack properly (1 column on mobile)

---

## Phase 4: Dialogs & Forms (HIGH)

### 4.1 Transaction Dialog Type Buttons

**File**: `apps/web/src/components/transactions/create-transaction-dialog.tsx`

Lines 228-259 have 3 type buttons (Expense, Income, Transfer) side-by-side. Stack on small screens:

```tsx
<div className="flex flex-col sm:flex-row gap-2">
  <Button
    type="button"
    variant={type === "expense" ? "default" : "outline"}
    size="sm"
    className="flex-1"
    onClick={() => handleTypeChange("expense")}
    disabled={isSubmitting}
  >
    Expense
  </Button>
  <Button
    type="button"
    variant={type === "income" ? "default" : "outline"}
    size="sm"
    className="flex-1"
    onClick={() => handleTypeChange("income")}
    disabled={isSubmitting}
  >
    Income
  </Button>
  <Button
    type="button"
    variant={type === "transfer" ? "default" : "outline"}
    size="sm"
    className="flex-1"
    onClick={() => handleTypeChange("transfer")}
    disabled={isSubmitting}
  >
    Transfer
  </Button>
</div>
```

### 4.2 Transaction Dialog Form Spacing

**File**: `apps/web/src/components/transactions/create-transaction-dialog.tsx`

Line 224: Increase form gap for better touch spacing:

```tsx
<form onSubmit={handleSubmit} className="flex flex-col gap-5">
  {/* Changed from gap-4 to gap-5 for 20px spacing */}
```

### 4.3 Edit Transaction Dialog

**File**: `apps/web/src/components/transactions/edit-transaction-dialog.tsx`

Apply same changes as create dialog:
- Stack type buttons on mobile
- Increase form gap to gap-5

### 4.4 Account Dialogs

**Files**:
- `apps/web/src/components/accounts/create-account-dialog.tsx`
- `apps/web/src/components/accounts/edit-account-dialog.tsx`

If these dialogs have button groups (e.g., account type selector), apply same stacking pattern:
```tsx
<div className="flex flex-col sm:flex-row gap-2">
  {/* Account type buttons */}
</div>
```

Increase form gap to gap-5 for all forms.

### 4.5 Budget Dialogs

**Files**:
- `apps/web/src/components/budgets/create-budget-dialog.tsx`
- `apps/web/src/components/budgets/edit-budget-dialog.tsx`

Period selector (Monthly/Yearly) likely uses toggle buttons. If side-by-side, verify they fit at 375px width. If cramped, stack:

```tsx
<div className="flex flex-col sm:flex-row gap-2">
  <Button
    type="button"
    variant={period === "monthly" ? "default" : "outline"}
    className="flex-1"
    onClick={() => setPeriod("monthly")}
  >
    Monthly
  </Button>
  <Button
    type="button"
    variant={period === "yearly" ? "default" : "outline"}
    className="flex-1"
    onClick={() => setPeriod("yearly")}
  >
    Yearly
  </Button>
</div>
```

Increase form gap to gap-5.

### 4.6 Category Dialogs

**Files**:
- `apps/web/src/components/categories/create-category-dialog.tsx`
- `apps/web/src/components/categories/edit-category-dialog.tsx`

If type selector (Income/Expense) is present as buttons, stack on mobile.
Increase form gap to gap-5.

### 4.7 Investment Dialogs

**Files**:
- `apps/web/src/components/investments/add-investment-dialog.tsx`
- `apps/web/src/components/investments/record-buy-dialog.tsx`
- `apps/web/src/components/investments/record-sell-dialog.tsx`
- `apps/web/src/components/investments/record-dividend-dialog.tsx`
- `apps/web/src/components/investments/record-split-dialog.tsx`

These have more complex forms with quantity + price fields. Changes:
1. Increase form gap to gap-5
2. Ensure `CurrencyInput` fields have adequate height (inherits from `Input`, should be h-10 = 40px)
3. If any button groups exist, stack on mobile

### 4.8 Dialog Width Verification

**File**: `apps/web/src/components/ui/dialog.tsx`

Line 64: Verify max-width calculation provides adequate margin on mobile:
```tsx
className={cn(
  "... max-w-[calc(100%-2rem)] ...",
  className
)}
```

2rem = 32px total margin (16px on each side). At 375px width:
- Dialog max-width = 375 - 32 = 343px ✅ (adequate)

**No changes needed** — already responsive.

### 4.9 Verification

```bash
cd apps/web
pnpm build
```

Test all dialogs at 375px width:
- [ ] Transaction type buttons stack vertically
- [ ] All form fields have 20px vertical spacing
- [ ] Account type buttons stack if present
- [ ] Budget period buttons stack if cramped
- [ ] Category type buttons stack if present
- [ ] Investment dialogs have adequate spacing
- [ ] All dialogs have 16px margin on each side

---

## Phase 5: Detail Pages (MEDIUM)

### 5.1 Account Detail Page

**File**: `apps/web/src/app/(dashboard)/accounts/[id]/page.tsx`

Expected structure (verify in codebase):
- Account header with name, type, balance
- Action buttons (Edit, Add Transaction, possibly Delete)
- Transaction list for this account

**Mobile optimizations**:

1. Stack account header info vertically:
```tsx
<div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
  <div>
    <h1 className="text-2xl font-bold">{account.name}</h1>
    <div className="flex items-center gap-2 mt-1">
      <Badge variant="secondary">{ACCOUNT_TYPE_LABELS[account.type]}</Badge>
      <Badge variant={account.is_active ? "outline" : "secondary"}>
        {account.is_active ? "Active" : "Inactive"}
      </Badge>
    </div>
  </div>
  <div className="flex flex-col sm:flex-row gap-2">
    <Button variant="outline" size="sm">
      <Pencil className="h-4 w-4" />
      <span className="ml-2">Edit</span>
    </Button>
    <Button variant="outline" size="sm">
      <Plus className="h-4 w-4" />
      <span className="ml-2">Add Transaction</span>
    </Button>
  </div>
</div>
```

2. Display balance prominently:
```tsx
<Card className="mb-6">
  <CardContent className="pt-6">
    <p className="text-sm text-muted-foreground">Current Balance</p>
    <p className="text-3xl font-bold mt-1">{formatCurrency(account.balance, account.currency)}</p>
  </CardContent>
</Card>
```

3. Transaction list uses mobile list pattern from Phase 2.2.

### 5.2 Investment Detail Page

**File**: `apps/web/src/app/(dashboard)/investments/[id]/page.tsx`

Expected structure:
- Investment header (security name, symbol, account)
- Current position stats (quantity, price, market value, G/L)
- Action buttons (Buy, Sell, Dividend, Split)
- Transaction history

**Mobile optimizations**:

1. Stack header vertically with security info:
```tsx
<div className="space-y-4">
  <div>
    <h1 className="text-2xl font-bold">{investment.security.name}</h1>
    <p className="text-muted-foreground">{investment.security.symbol}</p>
  </div>
  
  <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
    {/* Stats cards: Quantity, Price, Market Value, G/L */}
  </div>
  
  <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
    <Button variant="outline" size="sm">Buy</Button>
    <Button variant="outline" size="sm">Sell</Button>
    <Button variant="outline" size="sm">Dividend</Button>
    <Button variant="outline" size="sm">Split</Button>
  </div>
</div>
```

2. Transaction history uses mobile list pattern.

### 5.3 Security Detail Page

**File**: `apps/web/src/app/(dashboard)/securities/[id]/page.tsx`

Expected structure:
- Security header (name, symbol, type)
- Asset-specific fields (bond maturity, crypto network, etc.)
- Price history chart
- Related investments list

**Mobile optimizations**:

1. Stack header and metadata:
```tsx
<div className="space-y-4">
  <div>
    <h1 className="text-2xl font-bold">{security.name}</h1>
    <div className="flex items-center gap-2 mt-1">
      <Badge variant="secondary">{ASSET_TYPE_LABELS[assetType]}</Badge>
      <span className="text-muted-foreground">{security.symbol}</span>
    </div>
  </div>
  
  <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
    {/* Asset-specific fields in cards */}
  </div>
</div>
```

2. Price history chart: Reduce height on mobile (same as dashboard charts).

3. Related investments: Use card layout on mobile, table on desktop.

### 5.4 Verification

```bash
cd apps/web
pnpm build
```

Test detail pages at 375px:
- [ ] Account detail header stacks vertically
- [ ] Account action buttons stack or flow to new line
- [ ] Investment detail stats cards in 2-column grid on mobile
- [ ] Investment action buttons in 2-column grid on mobile
- [ ] Security detail metadata stacks properly
- [ ] Price history chart readable on mobile

---

## Phase 6: Remaining Pages (MEDIUM)

### 6.1 Budgets Page

**File**: `apps/web/src/app/(dashboard)/budgets/page.tsx`

Current state (already reviewed in earlier analysis):
- Card grid: `sm:grid-cols-1 md:grid-cols-2 lg:grid-cols-3` ✅
- Tabs for status filter ✅
- Period filter dropdown ✅

**Mobile optimization**:

Stack filter controls on very small screens:
```tsx
<div className="flex flex-col sm:flex-row flex-wrap items-start sm:items-center gap-3 sm:gap-4">
  <Tabs value={statusFilter} onValueChange={handleStatusChange}>
    <TabsList>
      <TabsTrigger value="all">All</TabsTrigger>
      <TabsTrigger value="active">Active</TabsTrigger>
      <TabsTrigger value="inactive">Inactive</TabsTrigger>
    </TabsList>
  </Tabs>
  <Select value={periodFilter} onValueChange={handlePeriodChange}>
    <SelectTrigger className="w-full sm:w-[140px]">
      <SelectValue placeholder="Period" />
    </SelectTrigger>
    <SelectContent>
      <SelectItem value="all">All Periods</SelectItem>
      <SelectItem value="monthly">Monthly</SelectItem>
      <SelectItem value="yearly">Yearly</SelectItem>
    </SelectContent>
  </Select>
</div>
```

Budget cards already responsive. Ensure edit/delete icon buttons in cards meet 44px minimum:

```tsx
<Button
  variant="ghost"
  size="icon-sm" // 36px — consider changing to "icon" (44px)
  onClick={() => setEditBudget(budget)}
>
  <Pencil className="h-4 w-4" />
</Button>
```

Change to `size="icon"` for 44px touch target.

### 6.2 Investments Page

**File**: `apps/web/src/app/(dashboard)/investments/page.tsx`

Contains:
- Portfolio summary cards (5 cards)
- Net worth over time chart
- Holdings by asset type table
- All holdings table (sortable from plan 012)

**Mobile optimizations**:

1. Summary cards already in responsive grid ✅

2. Asset allocation and portfolio composition charts (from plan 012): Reduce heights on mobile:
```tsx
<ChartContainer className="mx-auto aspect-square max-h-[200px] md:max-h-[250px]">
```

3. Holdings by asset type table: Convert to card layout on mobile:
```tsx
{/* Mobile: Cards */}
<div className="md:hidden grid gap-3">
  {Object.entries(portfolio.holdings_by_type)
    .filter(([_, holding]) => holding.count > 0)
    .map(([type, holding]) => (
      <Card key={type}>
        <CardHeader className="pb-2">
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm">
              {ASSET_TYPE_LABELS[type as AssetType]}
            </CardTitle>
            <Badge variant="outline">{holding.count} holding{holding.count !== 1 ? 's' : ''}</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <p className="text-xl font-semibold">{formatCurrency(holding.value)}</p>
        </CardContent>
      </Card>
    ))}
</div>

{/* Desktop: Table */}
<div className="hidden md:block">
  <Table>{/* existing table */}</Table>
</div>
```

4. All holdings table: Already has sortable columns. Add mobile card view:
```tsx
function HoldingCard({ investment }: { investment: Investment }) {
  const marketValue = Math.round(investment.quantity * investment.current_price);
  const unrealizedGL = marketValue - investment.cost_basis;
  const glPercent = investment.cost_basis > 0
    ? ((unrealizedGL / investment.cost_basis) * 100).toFixed(2)
    : "0.00";

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-2">
          <div className="min-w-0 flex-1">
            <CardTitle className="text-base truncate">{investment.security.symbol}</CardTitle>
            <p className="text-sm text-muted-foreground truncate">{investment.security.name}</p>
          </div>
          <div className="text-right shrink-0">
            <p className="text-lg font-semibold">{formatCurrency(marketValue)}</p>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-2">
        <div className="grid grid-cols-2 gap-2 text-sm">
          <div>
            <p className="text-muted-foreground text-xs">Quantity</p>
            <p className="font-medium">{investment.quantity.toFixed(4)}</p>
          </div>
          <div className="text-right">
            <p className="text-muted-foreground text-xs">Price</p>
            <p className="font-medium">{formatCurrency(investment.current_price)}</p>
          </div>
        </div>
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Unrealized G/L</span>
          <span className={unrealizedGL >= 0 ? "text-green-600" : "text-red-600"}>
            {unrealizedGL >= 0 ? "+" : ""}{formatCurrency(unrealizedGL)} ({glPercent}%)
          </span>
        </div>
        <div className="flex items-center justify-between text-sm">
          <span className="text-muted-foreground">Realized G/L</span>
          <span className={investment.realized_gain_loss >= 0 ? "text-green-600" : "text-red-600"}>
            {investment.realized_gain_loss >= 0 ? "+" : ""}{formatCurrency(investment.realized_gain_loss)}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
```

Render mobile cards:
```tsx
{/* Mobile: Cards */}
<div className="md:hidden grid gap-3">
  {sortedInvestments.map((investment) => (
    <HoldingCard key={investment.id} investment={investment} />
  ))}
</div>

{/* Desktop: Sortable table */}
<div className="hidden md:block">
  <Table>{/* existing sortable table */}</Table>
</div>
```

### 6.3 Securities Page

**File**: `apps/web/src/app/(dashboard)/securities/page.tsx`

Contains securities list table.

**Mobile optimization**:

Convert to card layout on mobile:
```tsx
function SecurityCard({ security }: { security: Security }) {
  const assetType = security.asset_type.toLowerCase() as AssetType;
  
  return (
    <Link href={`/securities/${security.id}`}>
      <Card className="transition-colors hover:bg-accent/50 cursor-pointer">
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between gap-2">
            <div className="min-w-0 flex-1">
              <CardTitle className="text-base truncate">{security.symbol}</CardTitle>
              <p className="text-sm text-muted-foreground truncate">{security.name}</p>
            </div>
            <Badge variant="secondary" className="shrink-0">
              {ASSET_TYPE_LABELS[assetType] ?? security.asset_type}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <p className="text-xl font-semibold">{formatCurrency(security.current_price)}</p>
          <p className="text-xs text-muted-foreground mt-1">{security.currency}</p>
        </CardContent>
      </Card>
    </Link>
  );
}
```

Render pattern:
```tsx
{/* Mobile: Cards */}
<div className="md:hidden grid gap-3">
  {securities.map((security) => (
    <SecurityCard key={security.id} security={security} />
  ))}
</div>

{/* Desktop: Table */}
<div className="hidden md:block">
  <Table>{/* existing table */}</Table>
</div>
```

### 6.4 Verification

```bash
cd apps/web
pnpm build
```

Test remaining pages at 375px:
- [ ] Budgets page filters stack properly
- [ ] Budget card action buttons meet 44px minimum
- [ ] Investments page charts reduced height on mobile
- [ ] Holdings by type show as cards on mobile
- [ ] All holdings show as cards on mobile (instead of table)
- [ ] Securities show as cards on mobile

---

## Phase 7: Component Audit (MEDIUM)

### 7.1 Form Input Components

**File**: `apps/web/src/components/ui/input.tsx`

Verify minimum height for touch targets. Standard height should be h-10 (40px):

```tsx
const Input = ({ className, type, ...props }: React.ComponentProps<"input">) => {
  return (
    <input
      type={type}
      className={cn(
        "flex h-10 w-full rounded-md border ...",
        className
      )}
      {...props}
    />
  )
}
```

If current height is h-9 (36px), change to h-10 (40px).

### 7.2 Select Component

**File**: `apps/web/src/components/ui/select.tsx`

Verify `SelectTrigger` has minimum h-10 height:

```tsx
const SelectTrigger = React.forwardRef<...>(({ className, children, ...props }, ref) => (
  <SelectPrimitive.Trigger
    ref={ref}
    className={cn(
      "flex h-10 w-full items-center justify-between ...",
      className
    )}
    {...props}
  >
    {children}
    <SelectPrimitive.Icon asChild>
      <ChevronDown className="h-4 w-4 opacity-50" />
    </SelectPrimitive.Icon>
  </SelectPrimitive.Trigger>
))
```

### 7.3 Textarea Component

**File**: `apps/web/src/components/ui/textarea.tsx`

Verify minimum height. Default textarea should have min-h-[80px] for adequate tap area:

```tsx
const Textarea = React.forwardRef<...>(({ className, ...props }, ref) => {
  return (
    <textarea
      className={cn(
        "flex min-h-[80px] w-full rounded-md border ...",
        className
      )}
      ref={ref}
      {...props}
    />
  )
})
```

### 7.4 Currency Input

**File**: `apps/web/src/components/ui/currency-input.tsx`

Should inherit height from `Input` component. Verify it uses `Input` internally or has h-10:

```tsx
// If using Input component:
<Input
  type="text"
  inputMode="decimal"
  value={displayValue}
  onChange={handleChange}
  {...props}
/>

// If custom implementation, ensure h-10 class
```

### 7.5 Card Component

**File**: `apps/web/src/components/ui/card.tsx`

Verify adequate padding on mobile. ShadCN default is p-6 (24px). Consider reducing slightly for mobile:

```tsx
const Card = React.forwardRef<...>(({ className, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      "rounded-lg border bg-card text-card-foreground shadow-sm p-4 sm:p-6",
      className
    )}
    {...props}
  />
))
```

Change from fixed p-6 to responsive p-4 sm:p-6 for tighter mobile padding.

### 7.6 Badge Component

**File**: `apps/web/src/components/ui/badge.tsx`

Verify text size is at least 12px. ShadCN uses text-xs (12px) which is acceptable ✅.

Verify minimum height for touch if badges are clickable. If used as buttons, wrap in Button component.

### 7.7 Tooltip Component

**File**: `apps/web/src/components/ui/tooltip.tsx`

Tooltips should not be the primary interaction method on mobile (hover doesn't work).

**Guideline**: Use tooltips only for supplementary info. Primary actions should have visible labels or be wrapped in disclosure patterns (sheets, popovers) on mobile.

**No code changes needed** — verification only.

### 7.8 Verification

```bash
cd apps/web
pnpm build
```

Test form components at 375px:
- [ ] Input fields are h-10 (40px minimum)
- [ ] Select triggers are h-10 (40px minimum)
- [ ] Textareas have min-h-[80px]
- [ ] Currency inputs inherit proper height
- [ ] Card padding reduced on mobile (p-4 vs p-6)
- [ ] Badges readable at text-xs size
- [ ] No critical functionality hidden behind tooltips

---

## Phase 8: Final Verification & Testing (CRITICAL)

### 8.1 Full Build

```bash
cd apps/web
pnpm build
```

Must complete with zero TypeScript errors.

### 8.2 Visual Verification Checklist

Test all pages at 375px, 390px, 428px, and 768px widths.

#### Layout & Navigation
- [ ] No horizontal scroll on any page at any breakpoint
- [ ] Sidebar works as mobile drawer (< 768px)
- [ ] Sidebar trigger (hamburger menu) visible and tappable
- [ ] Drawer closes on navigation or backdrop click
- [ ] Header search button shows icon-only on mobile
- [ ] User name hidden on < sm, visible on sm+

#### Touch Targets & Accessibility
- [ ] All interactive elements meet 44×44px minimum
- [ ] Icon buttons use size="icon" (44px)
- [ ] Adjacent touch targets have 8px+ spacing
- [ ] No 300ms tap delay (touch-action: manipulation)
- [ ] Form fields are 40px+ tall
- [ ] Select dropdowns are 40px+ tall

#### Tables & Lists
- [ ] Accounts page shows cards on mobile, table on desktop (breakpoint: md)
- [ ] Transactions page shows list items on mobile, table on desktop
- [ ] Categories page shows cards on mobile, table on desktop
- [ ] Securities page shows cards on mobile, table on desktop
- [ ] Holdings tables (investments page) show cards on mobile
- [ ] All mobile cards are tappable and show relevant info
- [ ] All mobile lists have clear visual separation between items

#### Filters & Pagination
- [ ] Transactions filter button shows/hides filter panel on mobile
- [ ] Active filter count badge displays correctly
- [ ] Filter panel stacks filters in 2-column grid on mobile
- [ ] Pagination buttons show icon-only on mobile, text on sm+
- [ ] Budget filter tabs and dropdown stack properly on mobile

#### Charts
- [ ] Dashboard expenditure chart: max-h-[200px] on mobile, 250px on md+
- [ ] Dashboard income/expenses chart: h-[200px] on mobile, 250px on md+
- [ ] Dashboard spending trend chart: readable with no label overlap
- [ ] Dashboard net worth chart: readable with no label overlap
- [ ] Investment charts: reduced height on mobile, readable tooltips
- [ ] All chart center labels and axis labels have responsive font sizes

#### Dialogs & Forms
- [ ] Transaction type buttons stack vertically on mobile (<sm)
- [ ] All forms have gap-5 (20px) between fields
- [ ] Account type buttons stack if present
- [ ] Budget period buttons stack if cramped
- [ ] Category type buttons stack if present
- [ ] Investment dialogs have adequate spacing
- [ ] All dialogs have 16px margin on each side
- [ ] Dialog content not cut off at 375px width

#### Detail Pages
- [ ] Account detail header stacks vertically on mobile
- [ ] Account action buttons stack or wrap properly
- [ ] Investment detail stats cards in 2-column grid on mobile
- [ ] Investment action buttons in 2-column grid on mobile
- [ ] Security detail metadata stacks properly
- [ ] Price history chart readable on mobile
- [ ] Transaction lists on detail pages use mobile list pattern

#### Dashboard
- [ ] Header action buttons (Add Account, Add Transaction) stack or work as icon-only
- [ ] Summary cards stack 1 column on mobile, 2 on sm, 4 on lg
- [ ] Budget overview cards stack properly
- [ ] All charts render without overlapping content

#### Edge Cases
- [ ] Empty states readable and properly spaced
- [ ] Loading skeletons appropriate size for mobile
- [ ] Error messages visible and not cut off
- [ ] Long text truncates with ellipsis where appropriate
- [ ] Currency values don't overflow containers
- [ ] Badge text doesn't overflow or wrap awkwardly

### 8.3 Accessibility Checks

- [ ] Minimum 16px font size for body text (text-base or larger)
- [ ] Color contrast meets 4.5:1 minimum for text
- [ ] Focus states visible on all interactive elements
- [ ] Form labels associated with inputs (htmlFor)
- [ ] Error messages announced (aria-live or role="alert")
- [ ] Alt text on images (if any)
- [ ] Keyboard navigation works (tab order logical)

### 8.4 Performance Checks

- [ ] No layout shift when loading charts/data
- [ ] Smooth transitions (150-300ms duration)
- [ ] No janky animations or scrolling
- [ ] Images optimized (SVG icons, WebP for photos)
- [ ] Bundle size reasonable (check Next.js build output)

### 8.5 Browser Testing

Test on actual devices or browser dev tools device emulation:
- [ ] Safari iOS (most critical for iPhone users)
- [ ] Chrome Android
- [ ] Chrome iOS
- [ ] Firefox Mobile (optional)

### 8.6 Regression Testing

Verify desktop experience still works:
- [ ] All pages render correctly at 1024px, 1440px, 1920px
- [ ] Tables visible and functional on desktop
- [ ] Charts have proper desktop heights
- [ ] Sidebar visible and functional at md+ breakpoints
- [ ] Filter rows horizontal on desktop
- [ ] Button groups horizontal on desktop

---

## Files Changed Summary

### Phase 1: Foundation (6 files)
```
apps/web/src/
├── app/
│   ├── globals.css                          # Touch optimization, overscroll, horizontal scroll prevention
│   └── layout.tsx                           # Viewport verification (no change if already correct)
├── components/
│   ├── ui/
│   │   └── button.tsx                       # Fix icon button sizes (36px → 44px)
│   └── layout/
│       ├── app-header.tsx                   # Verification only (already responsive)
│       └── app-sidebar.tsx                  # Verification only (already responsive)
└── app/(dashboard)/
    └── layout.tsx                           # Verification only (sidebar drawer)
```

### Phase 2: Tables (3 files)
```
apps/web/src/app/(dashboard)/
├── accounts/
│   └── page.tsx                             # Mobile card layout, responsive rendering
├── transactions/
│   └── page.tsx                             # Mobile list layout, collapsible filters, icon-only pagination
└── categories/
    └── page.tsx                             # Mobile card layout
```

### Phase 3: Charts (5 files)
```
apps/web/src/
├── app/(dashboard)/
│   └── page.tsx                             # Dashboard header button stacking
└── components/
    └── dashboard/
        ├── expenditure-chart.tsx            # Responsive height, font sizes
        ├── income-expenses-chart.tsx        # Responsive height
        ├── spending-trend-chart.tsx         # Responsive height, tick count
        └── net-worth-chart.tsx              # Responsive height, tick count
```

### Phase 4: Dialogs & Forms (11+ files)
```
apps/web/src/components/
├── transactions/
│   ├── create-transaction-dialog.tsx        # Stack type buttons, increase gap
│   └── edit-transaction-dialog.tsx          # Stack type buttons, increase gap
├── accounts/
│   ├── create-account-dialog.tsx            # Stack buttons if needed, increase gap
│   └── edit-account-dialog.tsx              # Stack buttons if needed, increase gap
├── budgets/
│   ├── create-budget-dialog.tsx             # Stack period buttons if needed, increase gap
│   └── edit-budget-dialog.tsx               # Stack period buttons if needed, increase gap
├── categories/
│   ├── create-category-dialog.tsx           # Stack type buttons if needed, increase gap
│   └── edit-category-dialog.tsx             # Stack type buttons if needed, increase gap
└── investments/
    ├── add-investment-dialog.tsx            # Increase gap
    ├── record-buy-dialog.tsx                # Increase gap
    ├── record-sell-dialog.tsx               # Increase gap
    ├── record-dividend-dialog.tsx           # Increase gap
    └── record-split-dialog.tsx              # Increase gap
```

### Phase 5: Detail Pages (3 files)
```
apps/web/src/app/(dashboard)/
├── accounts/
│   └── [id]/
│       └── page.tsx                         # Stack header, mobile transaction list
├── investments/
│   └── [id]/
│       └── page.tsx                         # Stack header, 2-col stats grid, mobile list
└── securities/
    └── [id]/
        └── page.tsx                         # Stack header, responsive metadata, chart height
```

### Phase 6: Remaining Pages (3 files)
```
apps/web/src/app/(dashboard)/
├── budgets/
│   └── page.tsx                             # Stack filters, fix button sizes
├── investments/
│   └── page.tsx                             # Mobile cards for holdings tables, chart heights
└── securities/
    └── page.tsx                             # Mobile card layout
```

### Phase 7: Component Audit (6 files)
```
apps/web/src/components/ui/
├── input.tsx                                # Verify h-10 height
├── select.tsx                               # Verify h-10 height
├── textarea.tsx                             # Verify min-h-[80px]
├── currency-input.tsx                       # Verify height inheritance
├── card.tsx                                 # Responsive padding (p-4 sm:p-6)
├── badge.tsx                                # Verification only (already text-xs)
└── tooltip.tsx                              # Verification only (guideline check)
```

### Total Files Modified
- **New files**: 0 (all modifications to existing files)
- **Modified files**: ~40-45 files
- **Backend changes**: 0 (frontend-only plan)

---

## Implementation Order

```
Phase 1: Foundation & Core Layout (CRITICAL)
  1.1  Global touch optimizations (globals.css)
  1.2  Viewport verification (layout.tsx)
  1.3  Fix button touch target sizes (button.tsx)
  1.4  Header responsiveness check (app-header.tsx)
  1.5  Sidebar mobile drawer verification (layout.tsx, app-sidebar.tsx)
  1.6  Verification (pnpm build + browser testing at 375px)

Phase 2: Table to Card/List Conversions (HIGH)
  2.1  Accounts page mobile cards (accounts/page.tsx)
  2.2  Transactions page mobile list + collapsible filters (transactions/page.tsx)
  2.3  Categories page mobile cards (categories/page.tsx)
  2.4  Verification (pnpm build + browser testing)

Phase 3: Dashboard & Chart Responsiveness (HIGH)
  3.1  Dashboard header actions (page.tsx)
  3.2  Expenditure chart responsive (expenditure-chart.tsx)
  3.3  Income/expenses chart responsive (income-expenses-chart.tsx)
  3.4  Spending trend chart responsive (spending-trend-chart.tsx)
  3.5  Net worth chart responsive (net-worth-chart.tsx)
  3.6  Dashboard summary cards verification (page.tsx)
  3.7  Verification (pnpm build + browser testing)

Phase 4: Dialogs & Forms (HIGH)
  4.1  Transaction dialog type buttons (create/edit)
  4.2  Transaction dialog form spacing
  4.3  Edit transaction dialog
  4.4  Account dialogs (create/edit)
  4.5  Budget dialogs (create/edit)
  4.6  Category dialogs (create/edit)
  4.7  Investment dialogs (all 5)
  4.8  Dialog width verification
  4.9  Verification (pnpm build + browser testing)

Phase 5: Detail Pages (MEDIUM)
  5.1  Account detail page (accounts/[id]/page.tsx)
  5.2  Investment detail page (investments/[id]/page.tsx)
  5.3  Security detail page (securities/[id]/page.tsx)
  5.4  Verification (pnpm build + browser testing)

Phase 6: Remaining Pages (MEDIUM)
  6.1  Budgets page (budgets/page.tsx)
  6.2  Investments page (investments/page.tsx)
  6.3  Securities page (securities/page.tsx)
  6.4  Verification (pnpm build + browser testing)

Phase 7: Component Audit (MEDIUM)
  7.1  Form input components (input.tsx)
  7.2  Select component (select.tsx)
  7.3  Textarea component (textarea.tsx)
  7.4  Currency input (currency-input.tsx)
  7.5  Card component (card.tsx)
  7.6  Badge component (badge.tsx)
  7.7  Tooltip component (tooltip.tsx)
  7.8  Verification (pnpm build + browser testing)

Phase 8: Final Verification & Testing (CRITICAL)
  8.1  Full build (pnpm build)
  8.2  Visual verification checklist (all pages, all breakpoints)
  8.3  Accessibility checks
  8.4  Performance checks
  8.5  Browser testing (Safari iOS, Chrome Android)
  8.6  Regression testing (desktop breakpoints)
```

## Verification

**After each phase**:
```bash
cd apps/web
pnpm build
```

**Device testing** (use browser DevTools device emulation or real devices):
- iPhone SE (375px × 667px)
- iPhone 12-14 (390px × 844px)
- iPhone 14 Pro Max (428px × 926px)
- iPad Mini (768px × 1024px)

**No backend changes** — no need to run `./scripts/check-go.sh`.

**Final acceptance criteria**:
1. Zero TypeScript build errors
2. All pages load without horizontal scroll (375px-768px)
3. All interactive elements meet 44×44px touch target minimum
4. Charts readable and interactive on mobile
5. Forms and dialogs have adequate spacing
6. Filters accessible via collapsible panels
7. No critical UX issues on 4 target device sizes
8. Desktop experience still fully functional (regression test)

# Kuberan Frontend Foundation Plan

## Context

The Kuberan backend (`apps/api/`) is a fully functional Go API with JWT auth, accounts (cash/investment), transactions, transfers, categories (hierarchical), budgets with progress tracking, and investments with portfolio summary. All monetary values are int64 cents.

The frontend (`apps/web/`) is a pristine `create-next-app` scaffold — Next.js 15, React 19, TypeScript strict mode, Tailwind CSS v4, Turbopack — with zero custom application code. This plan covers building the first three phases of the frontend to get a working authenticated app with dashboard and account management.

## Technology Choices

| Decision | Choice | Rationale |
|---|---|---|
| Component Library | ShadCN UI | As planned in CLAUDE.md. Radix primitives, fully customizable, Tailwind-native. |
| HTTP Client | Native `fetch` wrapper | Lightweight, no extra dependency. Typed generics for request/response. |
| Data Fetching | `@tanstack/react-query` | Server state management with caching, refetching, mutations. |
| Icons | `lucide-react` | ShadCN's default icon library. |
| API Communication | Direct browser calls + CORS | Backend already has `Access-Control-Allow-Origin: *`. `NEXT_PUBLIC_API_URL` env var. |

## Current State (Before This Plan)

### What Exists
- Next.js 15 with App Router (`src/app/`)
- React 19, TypeScript strict mode
- Tailwind CSS v4 with `@tailwindcss/postcss`
- Geist Sans + Mono fonts loaded in root layout
- Default `create-next-app` boilerplate page
- Docker Compose with `NEXT_PUBLIC_API_URL=http://localhost:8080`
- Backend CORS middleware (`Access-Control-Allow-Origin: *`)

### What Does NOT Exist
- No components, services, hooks, or API client
- No authentication logic or token management
- No application pages (login, dashboard, accounts)
- No ShadCN UI, `@tanstack/react-query`, or `lucide-react`
- No route groups, loading states, or error boundaries
- Metadata still reads "Create Next App"

---

## Phase 1: Foundation Layer

**Goal**: Set up the core infrastructure — dependencies, API client, auth system, app shell — so all subsequent work can build on a solid base.

### 1.1 Install Dependencies and Initialize ShadCN

Install and configure:
- `npx shadcn@latest init` (New York style, default color theme)
- `@tanstack/react-query` for data fetching
- `lucide-react` for icons

ShadCN components to install initially:
- `button`, `input`, `label`, `card`, `dialog`, `form`, `dropdown-menu`, `sidebar`, `toast`, `sonner`, `separator`, `avatar`, `badge`, `skeleton`, `table`, `select`, `tabs`, `popover`, `command`, `sheet`

This creates:
- `src/components/ui/` directory with all ShadCN components
- `src/lib/utils.ts` with the `cn()` utility function
- Updated `globals.css` with ShadCN CSS variables

### 1.2 TypeScript Types

Create type definitions matching the backend API exactly.

**`src/types/models.ts`** — Domain models:
```typescript
// Base model fields (all entities have these)
interface BaseModel {
  id: number;
  created_at: string; // ISO 8601
  updated_at: string;
}

// User (from GET /profile, auth responses)
interface User {
  id: number;
  email: string;
  first_name: string;
  last_name: string;
}

// Account types: "cash" | "investment" | "debt"
type AccountType = "cash" | "investment" | "debt";

interface Account extends BaseModel {
  user_id: number;
  name: string;
  type: AccountType;
  description: string;
  balance: number;      // int64 cents
  currency: string;     // ISO 4217
  is_active: boolean;
  broker: string;         // investment accounts
  account_number: string; // investment accounts
  interest_rate: number;  // debt accounts (float)
  due_date: string;       // debt accounts
}

// Transaction types: "income" | "expense" | "transfer" | "investment"
type TransactionType = "income" | "expense" | "transfer" | "investment";

interface Transaction extends BaseModel {
  user_id: number;
  account_id: number;
  category_id: number | null;
  type: TransactionType;
  amount: number;        // int64 cents, always positive
  description: string;
  date: string;          // ISO 8601
  to_account_id: number | null; // for transfers
}

// Category types: "income" | "expense"
type CategoryType = "income" | "expense";

interface Category extends BaseModel {
  user_id: number;
  name: string;
  type: CategoryType;
  description: string;
  icon: string;
  color: string;         // hex (#RGB or #RRGGBB)
  parent_id: number | null;
}

// Budget periods: "monthly" | "yearly"
type BudgetPeriod = "monthly" | "yearly";

interface Budget extends BaseModel {
  user_id: number;
  category_id: number;
  name: string;
  amount: number;       // int64 cents
  period: BudgetPeriod;
  start_date: string;
  end_date: string | null;
  is_active: boolean;
  category?: Category;  // preloaded on some endpoints
}

interface BudgetProgress {
  budget_id: number;
  budgeted: number;     // cents
  spent: number;        // cents
  remaining: number;    // cents
  percentage: number;   // float, (spent/budgeted)*100
}

// Asset types: "stock" | "etf" | "bond" | "crypto" | "reit"
type AssetType = "stock" | "etf" | "bond" | "crypto" | "reit";

interface Investment extends BaseModel {
  account_id: number;
  symbol: string;
  asset_type: AssetType;
  name: string;
  quantity: number;      // float
  cost_basis: number;    // int64 cents
  current_price: number; // int64 cents (per unit)
  last_updated: string;
  currency: string;
  exchange: string;
  maturity_date: string | null;
  yield_to_maturity: number;
  coupon_rate: number;
  network: string;
  wallet_address: string;
  property_type: string;
}

type InvestmentTransactionType = "buy" | "sell" | "dividend" | "split" | "transfer";

interface InvestmentTransaction extends BaseModel {
  investment_id: number;
  type: InvestmentTransactionType;
  date: string;
  quantity: number;      // float
  price_per_unit: number; // int64 cents
  total_amount: number;   // int64 cents
  fee: number;            // int64 cents
  notes: string;
  split_ratio: number;    // float
  dividend_type: string;
}

interface PortfolioSummary {
  total_value: number;       // cents
  total_cost_basis: number;  // cents
  total_gain_loss: number;   // cents
  gain_loss_pct: number;     // float percentage
  holdings_by_type: Record<AssetType, { value: number; count: number }>;
}
```

**`src/types/api.ts`** — API request/response types:
```typescript
// Pagination
interface PageResponse<T> {
  data: T[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

interface PaginationParams {
  page?: number;
  page_size?: number;
}

// Error response
interface ApiError {
  error: {
    code: string;
    message: string;
  };
}

// Auth
interface LoginRequest {
  email: string;
  password: string;
}

interface RegisterRequest {
  email: string;
  password: string;
  first_name?: string;
  last_name?: string;
}

interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

interface RefreshRequest {
  refresh_token: string;
}

// Accounts
interface CreateCashAccountRequest {
  name: string;
  description?: string;
  currency?: string;
  initial_balance?: number; // cents
}

interface CreateInvestmentAccountRequest {
  name: string;
  description?: string;
  currency?: string;
  broker?: string;
  account_number?: string;
}

interface UpdateAccountRequest {
  name?: string;
  description?: string;
}

// Transactions
interface CreateTransactionRequest {
  account_id: number;
  category_id?: number;
  type: TransactionType;
  amount: number;        // cents, > 0
  description?: string;
  date?: string;
}

interface CreateTransferRequest {
  from_account_id: number;
  to_account_id: number;
  amount: number;        // cents, > 0
  description?: string;
  date?: string;
}

interface TransactionFilters extends PaginationParams {
  from_date?: string;
  to_date?: string;
  type?: TransactionType;
  category_id?: number;
  min_amount?: number;
  max_amount?: number;
}

// Categories
interface CreateCategoryRequest {
  name: string;
  type: CategoryType;
  description?: string;
  icon?: string;
  color?: string;
  parent_id?: number;
}

interface UpdateCategoryRequest {
  name?: string;
  description?: string;
  icon?: string;
  color?: string;
  parent_id?: number;
}
```

### 1.3 API Client

**`src/lib/api-client.ts`** — Typed fetch wrapper:

Key features:
- Base URL from `process.env.NEXT_PUBLIC_API_URL`
- Generic methods: `get<T>()`, `post<T>()`, `put<T>()`, `del<T>()`
- Automatic `Authorization: Bearer <token>` header from stored access token
- Automatic 401 handling: attempt token refresh, retry original request once
- If refresh also fails, clear tokens and redirect to `/login`
- Typed error handling: parse `{ error: { code, message } }` response into `ApiError`
- Query parameter serialization for GET requests with filters/pagination
- No external dependency (uses native `fetch`)

Error flow:
1. API returns non-2xx -> parse JSON body for `ApiError`
2. If 401 and not a refresh/login request -> attempt refresh
3. On refresh success -> retry original request with new access token
4. On refresh failure -> clear auth state, redirect to login
5. Throw typed `ApiError` for callers to handle

### 1.4 Auth Infrastructure

**`src/lib/auth.ts`** — Token management:
- `getAccessToken()` / `setAccessToken()` / `clearTokens()` using `localStorage`
- `getRefreshToken()` / `setRefreshToken()` using `localStorage`
- `isAuthenticated()` — checks if access token exists (not expired)
- Token parsing helper to read JWT claims (expiry check)

**`src/providers/auth-provider.tsx`** — Auth context (client component):
- State: `user: User | null`, `isLoading: boolean`, `isAuthenticated: boolean`
- On mount: check for stored tokens, attempt to fetch profile (`GET /api/v1/profile`)
- If profile fetch succeeds: set user state
- If profile fetch fails (401): attempt refresh, retry profile
- If still fails: clear tokens, set unauthenticated
- Methods exposed via context:
  - `login(email, password)` -> calls `POST /api/v1/auth/login`, stores tokens, sets user
  - `register(data)` -> calls `POST /api/v1/auth/register`, stores tokens, sets user
  - `logout()` -> clears tokens, sets user to null, redirects to `/login`
  - `refreshAuth()` -> calls `POST /api/v1/auth/refresh` with stored refresh token

**`src/hooks/use-auth.ts`** — Convenience hook:
```typescript
const { user, isLoading, isAuthenticated, login, register, logout } = useAuth();
```

### 1.5 React Query Provider

**`src/providers/query-provider.tsx`** — Client component wrapping `QueryClientProvider`:
- `staleTime: 5 * 60 * 1000` (5 minutes)
- `retry: 1`
- Default error handler that could surface toast notifications

### 1.6 App Shell & Layout

**Root layout (`src/app/layout.tsx`)** updates:
- Update metadata: `title: "Kuberan"`, `description: "Personal finance tracker"`
- Wrap children with `<QueryProvider>` then `<AuthProvider>`
- Add `<Toaster />` for toast notifications

**Auth layout (`src/app/(auth)/layout.tsx`)**:
- Centered card layout for login/register pages
- No sidebar, minimal branding
- Redirect to `/` if already authenticated

**Dashboard layout (`src/app/(dashboard)/layout.tsx`)**:
- ShadCN Sidebar with navigation:
  - Dashboard (home icon)
  - Accounts (wallet icon)
  - Transactions (arrow-left-right icon)
  - Categories (tag icon)
  - Budgets (pie-chart icon)
  - Investments (trending-up icon)
- App header with:
  - App title / logo
  - User avatar + name dropdown (profile, logout)
- Protected: redirects to `/login` if not authenticated
- Shows loading skeleton while auth state is being determined

**`src/components/layout/app-sidebar.tsx`**:
- Uses ShadCN Sidebar component
- Navigation items with icons and active state highlighting
- Collapsible on mobile

**`src/components/layout/app-header.tsx`**:
- Breadcrumb or page title area
- User dropdown menu (avatar, name, logout action)

### 1.7 Utility Components

**`src/lib/format.ts`** — Formatting utilities:
- `formatCurrency(cents: number, currency?: string): string` — e.g., `1050` -> `"$10.50"`
- `formatDate(iso: string): string` — human-readable date
- `formatDateTime(iso: string): string` — human-readable date + time
- `formatPercentage(value: number): string` — e.g., `65.5` -> `"65.5%"`

### 1.8 Environment Configuration

**`.env.local`** (gitignored, for local dev outside Docker):
```
NEXT_PUBLIC_API_URL=http://localhost:8080
```

This is already set in `docker-compose.yml` for containerized dev.

---

## Phase 2: Auth Pages

**Goal**: Login and register pages, plus Next.js middleware for route protection.

### 2.1 Login Page

**`src/app/(auth)/login/page.tsx`**:
- Form fields: email, password
- Client-side validation:
  - Email: required, valid email format
  - Password: required, min 8 characters
- Submit calls `login()` from auth context
- Error display:
  - `INVALID_CREDENTIALS` -> "Invalid email or password"
  - `ACCOUNT_LOCKED` -> "Account locked due to too many failed attempts. Try again later."
  - Generic error fallback
- Link to register page: "Don't have an account? Register"
- On success: redirect to `/` (dashboard)
- Loading state on submit button

### 2.2 Register Page

**`src/app/(auth)/register/page.tsx`**:
- Form fields: email, password, confirm password, first name, last name
- Client-side validation:
  - Email: required, valid email format, max 255
  - Password: required, min 8, max 128
  - Confirm password: must match password
  - First name: optional, max 100
  - Last name: optional, max 100
- Submit calls `register()` from auth context
- Error display:
  - `DUPLICATE_EMAIL` -> "An account with this email already exists"
  - Generic error fallback
- Link to login page: "Already have an account? Login"
- On success: auto-login, redirect to `/` (dashboard)

### 2.3 Route Protection via Next.js Middleware

**`src/middleware.ts`**:
- Matcher: applies to all routes except `/login`, `/register`, `/_next/`, `/api/`, static files
- Check: look for auth token in cookies (we'll store a simple flag cookie alongside localStorage)
- If no token -> redirect to `/login`
- If token exists and route is `/login` or `/register` -> redirect to `/`
- Note: The real auth check happens client-side in the AuthProvider; the middleware provides a fast redirect to avoid flash of wrong page

---

## Phase 3: Dashboard & Accounts

**Goal**: Working dashboard with account overview, accounts list, account detail with transactions, and create/add forms.

### 3.1 React Query Service Hooks

All hooks use `@tanstack/react-query` for caching, refetching, and mutations.

**`src/hooks/use-accounts.ts`**:
```typescript
useAccounts(params?: PaginationParams)         // GET /api/v1/accounts -> PageResponse<Account>
useAccount(id: number)                          // GET /api/v1/accounts/:id -> Account
useCreateCashAccount()                          // POST /api/v1/accounts/cash -> mutation
useCreateInvestmentAccount()                    // POST /api/v1/accounts/investment -> mutation
useUpdateAccount(id: number)                    // PUT /api/v1/accounts/:id -> mutation
```

**`src/hooks/use-transactions.ts`**:
```typescript
useAccountTransactions(accountId: number, filters?: TransactionFilters)
                                                 // GET /api/v1/accounts/:id/transactions -> PageResponse<Transaction>
useTransaction(id: number)                       // GET /api/v1/transactions/:id -> Transaction
useCreateTransaction()                           // POST /api/v1/transactions -> mutation
useCreateTransfer()                              // POST /api/v1/transactions/transfer -> mutation
useDeleteTransaction()                           // DELETE /api/v1/transactions/:id -> mutation
```

**`src/hooks/use-categories.ts`**:
```typescript
useCategories(params?: PaginationParams & { type?: CategoryType })
                                                 // GET /api/v1/categories -> PageResponse<Category>
```

**`src/hooks/use-profile.ts`**:
```typescript
useProfile()                                     // GET /api/v1/profile -> User
```

All mutations invalidate relevant queries on success (e.g., creating a transaction invalidates account and transactions queries).

### 3.2 Dashboard Page

**`src/app/(dashboard)/page.tsx`**:

Layout:
- **Welcome header**: "Welcome back, {firstName}" or "Dashboard"
- **Account summary section**:
  - Total balance card: sum of all account balances, formatted as currency
  - Grid of account cards (max 4-6 visible, "View all" link):
    - Account name, type badge (Cash/Investment/Debt), balance
    - Click navigates to `/accounts/[id]`
- **Recent transactions section**:
  - Last 5 transactions across all accounts (from the first account, or needs a dedicated endpoint — we'll use the first account's transactions as a start)
  - Each row: date, description, type badge, amount (green/red)
  - "View all" link to `/accounts/[id]` or future transactions page
- **Quick actions**:
  - "Add Account" button -> opens create account dialog
  - "Add Transaction" button -> opens create transaction dialog

Uses `useAccounts()` and `useAccountTransactions()` hooks.

### 3.3 Accounts List Page

**`src/app/(dashboard)/accounts/page.tsx`**:

- Page header: "Accounts" with "Create Account" button
- Table or card grid of accounts:
  - Columns: Name, Type (badge), Balance (formatted), Currency, Status (active/inactive), Created
  - Click row -> navigate to `/accounts/[id]`
- Pagination controls if > 20 accounts
- Empty state: "No accounts yet. Create your first account to get started."

### 3.4 Create Account Dialog

**`src/components/accounts/create-account-dialog.tsx`**:

- ShadCN Dialog triggered by "Create Account" button
- Tabs: "Cash Account" | "Investment Account"
- **Cash Account tab**:
  - Name (required, 1-100 chars)
  - Description (optional, max 500)
  - Currency (select, defaults to USD)
  - Initial Balance (currency input, optional, >= 0)
- **Investment Account tab**:
  - Name (required, 1-100 chars)
  - Description (optional, max 500)
  - Currency (select, defaults to USD)
  - Broker (optional, max 100)
  - Account Number (optional, max 50)
- On submit: calls `useCreateCashAccount` or `useCreateInvestmentAccount` mutation
- On success: close dialog, show success toast, invalidate accounts query
- On error: show error message inline

### 3.5 Account Detail Page

**`src/app/(dashboard)/accounts/[id]/page.tsx`**:

Layout:
- **Account header**:
  - Account name (editable inline or via edit button)
  - Type badge, currency, status
  - Balance prominently displayed
  - Description (if any)
  - "Edit" button -> opens edit dialog
- **Transactions section**:
  - "Add Transaction" button (+ "Transfer" button if cash account)
  - Filter bar:
    - Date range picker (from_date, to_date)
    - Type dropdown (all, income, expense, transfer)
    - Category dropdown (loaded from categories API)
    - Amount range (min, max)
  - Transactions table:
    - Columns: Date, Description, Type (badge), Category, Amount
    - Amount: green with `+` for income, red with `-` for expense, blue for transfer
    - Click to view detail or delete
    - Pagination
  - Empty state: "No transactions yet."
- **For investment accounts**: additional tab showing investments list (links to future investments page)

### 3.6 Create Transaction Dialog

**`src/components/transactions/create-transaction-dialog.tsx`**:

- Account pre-selected if opened from account detail page, otherwise dropdown
- Type: income or expense (radio/segmented control)
- Amount: currency input (user types dollars, we convert to cents)
- Category: dropdown loaded from categories (filtered by income/expense type)
- Description: optional text
- Date: date picker, defaults to today
- On submit: calls `useCreateTransaction` mutation
- On success: close, toast, invalidate queries
- On error: show error (e.g., insufficient balance for expense)

### 3.7 Transfer Dialog

**`src/components/transactions/create-transfer-dialog.tsx`**:

- From Account: dropdown (all cash accounts)
- To Account: dropdown (all cash accounts, excludes selected from-account)
- Amount: currency input
- Description: optional
- Date: date picker, defaults to today
- Validation: from !== to, amount > 0
- On submit: calls `useCreateTransfer` mutation
- On success: close, toast, invalidate both accounts' queries

### 3.8 Currency Input Component

**`src/components/ui/currency-input.tsx`**:

- Accepts value in cents, displays formatted dollars
- User types dollar amount (e.g., "10.50"), component stores as cents (1050)
- Handles decimal input properly (max 2 decimal places)
- Prefix with currency symbol ($)

---

## File Structure After Phase 3

```
src/
├── app/
│   ├── (auth)/
│   │   ├── layout.tsx                  # Centered auth layout
│   │   ├── login/
│   │   │   └── page.tsx
│   │   └── register/
│   │       └── page.tsx
│   ├── (dashboard)/
│   │   ├── layout.tsx                  # Sidebar + header layout
│   │   ├── page.tsx                    # Dashboard
│   │   └── accounts/
│   │       ├── page.tsx                # Accounts list
│   │       └── [id]/
│   │           └── page.tsx            # Account detail
│   ├── globals.css
│   ├── layout.tsx                      # Root layout (providers)
│   └── favicon.ico
├── components/
│   ├── layout/
│   │   ├── app-sidebar.tsx
│   │   └── app-header.tsx
│   ├── accounts/
│   │   └── create-account-dialog.tsx
│   ├── transactions/
│   │   ├── create-transaction-dialog.tsx
│   │   └── create-transfer-dialog.tsx
│   └── ui/                             # ShadCN components (auto-generated)
│       ├── button.tsx
│       ├── input.tsx
│       ├── card.tsx
│       ├── dialog.tsx
│       ├── table.tsx
│       ├── badge.tsx
│       ├── skeleton.tsx
│       ├── currency-input.tsx          # Custom
│       └── ... (other ShadCN components)
├── hooks/
│   ├── use-auth.ts
│   ├── use-accounts.ts
│   ├── use-transactions.ts
│   ├── use-categories.ts
│   └── use-profile.ts
├── lib/
│   ├── api-client.ts
│   ├── auth.ts
│   ├── format.ts
│   └── utils.ts                        # ShadCN cn() utility
├── middleware.ts                        # Route protection
├── providers/
│   ├── auth-provider.tsx
│   └── query-provider.tsx
└── types/
    ├── api.ts
    └── models.ts
```

---

## Implementation Order

The tasks should be executed in this sequence (dependencies shown):

```
1.1  Install dependencies & ShadCN init
 |
1.2  TypeScript types (models.ts, api.ts)
 |
1.3  API client (api-client.ts)
 |
1.4  Auth infrastructure (auth.ts, auth-provider.tsx, use-auth.ts)
 |
1.5  React Query provider
 |
1.6  App shell & layouts (root, auth, dashboard)
 |
1.7  Utility components (format.ts, currency-input)
 |
1.8  Environment config
 |
2.1  Login page
 |
2.2  Register page
 |
2.3  Next.js middleware
 |
3.1  React Query service hooks
 |
3.2  Dashboard page
 |
3.3  Accounts list page
 |
3.4  Create account dialog
 |
3.5  Account detail page
 |
3.6  Create transaction dialog
 |
3.7  Transfer dialog
 |
3.8  Currency input component
```

## Verification

After each logical unit of work, run:
```bash
cd apps/web && npm run build
```

This runs the Next.js TypeScript build, catching:
- Type errors (strict mode)
- Import errors
- Missing exports
- JSX errors
- ESLint issues (via next build)

After completing all three phases:
```bash
cd apps/web && npm run lint && npm run build
```

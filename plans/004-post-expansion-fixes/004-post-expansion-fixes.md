# Kuberan Post-Expansion Fixes & Enhancements

## Context

Plan 003 (Frontend Expansion) is complete — the app has categories, transactions, budgets pages, dark mode, edit account dialog, and dashboard improvements. However, several UX issues and missing features were identified:

1. **Dashboard stale data**: Adding a transaction doesn't refresh the dashboard's "Recent Transactions" section (query invalidation bug).
2. **No credit card accounts**: Only cash and investment accounts can be created. Credit cards (liabilities) are missing.
3. **No transaction editing**: Transactions can only be created or deleted — no way to fix mistakes without deleting and recreating.
4. **Limited account editing**: Only cash accounts can be edited, and only name/description. Investment and credit card accounts need editable type-specific fields.
5. **Transfer UX**: Transfers require a separate button and dialog. They should be an option within the Add Transaction dialog.

## Scope Summary

| Feature | Type | Backend Changes? |
|---|---|---|
| Dashboard query invalidation fix | Frontend | No — cache key fix only |
| Credit card account type | Backend + Frontend | Yes — migration, model, service, handler, tests |
| Generic account update | Backend + Frontend | Yes — refactor service/handler, extend edit dialog |
| Transaction editing | Backend + Frontend | Yes — new service method, handler, route, tests |
| Unified transaction dialog | Frontend | No — refactor existing dialogs |

## Technology & Patterns

This plan follows all patterns established in Plans 001–003:
- Backend: 3-layer architecture (Handler → Service → Model), interface-based services, AppError types, Swagger annotations, table-driven tests
- Frontend: ShadCN UI, React Query with query key factories, controlled dialog pattern, Sonner toasts, skeleton loading states

## Design Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Credit card account type | New `credit_card` type (not reuse `debt`) | Credit cards have unique semantics (credit limit, statement cycles) distinct from generic debt (loans, mortgages). Keeps the door open for a future `debt` type for loans. |
| Credit card balance semantics | Positive balance = amount owed | Simpler math: expense increases balance, payment decreases it. Avoids negative number display complexity. Balance represents "what you owe." |
| Transaction edit scope | All fields editable (amount, type, account, category, description, date) | Maximum flexibility for fixing mistakes. Complex but essential for a finance app. |
| Transaction edit restrictions | Transfers and investment types cannot be edited | Transfer edits require coordinating two accounts + to_account_id changes. Investment transactions are managed through the investment system. Users should delete and recreate instead. |
| Type change allowed | Income ↔ Expense only | Changing to/from transfer or investment types introduces too much complexity (to_account_id management, investment system coordination). |
| Account update endpoint | Generic `PUT /accounts/:id` for all types | Single endpoint with type-specific optional fields. Cleaner API surface, less duplication, scales naturally with new account types. |
| Currency editing | Not allowed after creation | Changing currency on an account with transactions would misrepresent historical amounts. Currency is immutable. |
| Transfer in transaction dialog | Inline type toggle (third button) | Keeps the flow seamless — user picks Expense, Income, or Transfer within the same dialog. Form fields adapt dynamically. No separate dialog needed. |
| Credit card fields | Minimal: credit_limit, interest_rate, due_date | Covers the essential tracking needs without overcomplicating the model. Can be extended later. |

---

## Phase 1: Fix Dashboard Query Invalidation (Bug Fix)

### Problem

After creating a transaction, the dashboard's "Recent Transactions" section doesn't update. Root cause: mutation `onSuccess` handlers invalidate `transactionKeys.lists()` (`["transactions", "list"]`), but the dashboard uses `useTransactions()` which has key `transactionKeys.userList(...)` (`["transactions", "userList", ...]`). React Query's prefix matching means `["transactions", "list"]` does NOT match `["transactions", "userList"]`.

### 1.1 Fix Transaction Mutation Invalidation

**File**: `apps/web/src/hooks/use-transactions.ts`

In all three mutation hooks (`useCreateTransaction`, `useCreateTransfer`, `useDeleteTransaction`), change:
```ts
queryClient.invalidateQueries({ queryKey: transactionKeys.lists() });
```
to:
```ts
queryClient.invalidateQueries({ queryKey: transactionKeys.all });
```

This invalidates ALL transaction queries (both `["transactions", "list", ...]` and `["transactions", "userList", ...]`) ensuring the dashboard, transactions page, and account detail pages all refresh.

### 1.2 Verify

Run `npm run build` from `apps/web/`.

---

## Phase 2: Credit Card Account Type

### Backend Changes

#### 2.1 Database Migration

**Files**: `apps/api/migrations/000010_add_credit_card_support.up.sql`, `000010_add_credit_card_support.down.sql`

**Up**: Add `credit_limit BIGINT DEFAULT 0` column to `accounts` table.

The existing `interest_rate` (DOUBLE PRECISION) and `due_date` (TIMESTAMPTZ) columns from migration 000002 are reused for credit cards.

**Down**: Drop `credit_limit` column.

#### 2.2 Update Account Model

**File**: `apps/api/internal/models/account.go`

- Add `AccountTypeCreditCard AccountType = "credit_card"` constant
- Add `CreditLimit int64` field with `gorm:"type:bigint;default:0" json:"credit_limit,omitempty"`
- Update `BeforeCreate` hook: for `credit_card` type, clear `Broker` and `AccountNumber`

#### 2.3 Update Account Validator

**File**: `apps/api/internal/validator/validator.go`

Add `"credit_card"` to the `account_type` validator's accepted values.

#### 2.4 Add CreateCreditCardAccount to Service

**File**: `apps/api/internal/services/interfaces.go`

Add to `AccountServicer`:
```go
CreateCreditCardAccount(userID uint, name, description, currency string, creditLimit int64, interestRate float64, dueDate *time.Time) (*models.Account, error)
```

**File**: `apps/api/internal/services/account_service.go`

Implementation:
- Validate name not empty, default currency to "USD"
- Create account with `Type: AccountTypeCreditCard`, `Balance: 0`, `IsActive: true`
- Set `CreditLimit`, `InterestRate`, `DueDate` from parameters
- No initial balance — credit cards start at 0 (nothing owed)

#### 2.5 Update Balance Logic for Credit Cards

**File**: `apps/api/internal/services/account_service.go`

Update `UpdateAccountBalance` to handle credit card semantics (positive balance = amount owed):
- Credit card + Expense → `balance += amount` (spending adds to debt)
- Credit card + Income → `balance -= amount` (payment reduces debt)
- Cash/Investment + Income → `balance += amount` (unchanged)
- Cash/Investment + Expense → `balance -= amount` (unchanged)

#### 2.6 Update Transfer Validation

**File**: `apps/api/internal/services/transaction_service.go`

In `CreateTransfer`, the insufficient balance check (`fromAccount.Balance < amount`) only applies to non-credit-card accounts. For credit card source accounts, skip this check (credit cards can go up to their limit, or even beyond for the MVP).

#### 2.7 Add Credit Card Handler

**File**: `apps/api/internal/handlers/account_handler.go`

Add `CreateCreditCardAccountRequest` struct:
```go
type CreateCreditCardAccountRequest struct {
    Name         string  `json:"name" binding:"required,min=1,max=100"`
    Description  string  `json:"description" binding:"max=500"`
    Currency     string  `json:"currency" binding:"omitempty,iso4217"`
    CreditLimit  int64   `json:"credit_limit" binding:"gte=0"`
    InterestRate float64 `json:"interest_rate" binding:"gte=0,lte=100"`
    DueDate      *string `json:"due_date"`
}
```

Add `CreateCreditCardAccount` handler method with full Swagger annotations.

#### 2.8 Register Route

**File**: `apps/api/cmd/api/main.go`

Add `accounts.POST("/credit-card", accountHandler.CreateCreditCardAccount)`.

#### 2.9 Service Tests

**File**: `apps/api/internal/services/account_service_test.go`

Add tests:
- `TestCreateCreditCardAccount`: valid creation, empty name error, default currency, credit limit set
- `TestUpdateAccountBalance_CreditCard`: expense increases balance, income decreases balance

#### 2.10 Handler Tests

**File**: `apps/api/internal/handlers/account_handler_test.go`

Add mock method and tests for `CreateCreditCardAccount` handler.

#### 2.11 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

### Frontend Changes

#### 2.12 Update Frontend Types

**File**: `apps/web/src/types/models.ts`

- Update `AccountType` to `"cash" | "investment" | "debt" | "credit_card"`
- Add `credit_limit?: number` to `Account` interface

**File**: `apps/web/src/types/api.ts`

Add:
```ts
export interface CreateCreditCardAccountRequest {
  name: string;
  description?: string;
  currency?: string;
  credit_limit?: number;
  interest_rate?: number;
  due_date?: string;
}
```

#### 2.13 Add Frontend Hook

**File**: `apps/web/src/hooks/use-accounts.ts`

Add `useCreateCreditCardAccount()` mutation hook: POST `/api/v1/accounts/credit-card`, invalidate account lists on success.

#### 2.14 Update Create Account Dialog

**File**: `apps/web/src/components/accounts/create-account-dialog.tsx`

Add a third tab: "Credit Card" with fields:
- Name (required, max 100)
- Description (optional, max 500)
- Currency (dropdown, same list as cash/investment)
- Credit Limit (CurrencyInput)
- Interest Rate (number input, 0–100, %)
- Due Date (date input, optional)

#### 2.15 Update Dashboard Summary Cards

**File**: `apps/web/src/app/(dashboard)/page.tsx`

Update `SummaryCards` to show 4 cards:
- **Net Worth**: (cash + investment) − credit card balances
- **Cash**: sum of cash account balances
- **Investments**: sum of investment account balances
- **Credit Cards**: sum of credit card balances (displayed as liability)

#### 2.16 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 3: Generic Account Update

### Backend Changes

#### 3.1 Add AccountUpdateFields and Refactor Interface

**File**: `apps/api/internal/services/interfaces.go`

Replace `UpdateCashAccount` with:
```go
UpdateAccount(userID, accountID uint, updates AccountUpdateFields) (*models.Account, error)
```

Add:
```go
type AccountUpdateFields struct {
    Name          *string
    Description   *string
    IsActive      *bool
    Broker        *string   // investment only
    AccountNumber *string   // investment only
    InterestRate  *float64  // credit_card only
    DueDate       *time.Time // credit_card only
    CreditLimit   *int64    // credit_card only
}
```

#### 3.2 Implement Generic UpdateAccount

**File**: `apps/api/internal/services/account_service.go`

Replace `UpdateCashAccount` with `UpdateAccount`:
1. Fetch account by ID + user ID
2. Build updates map from non-nil fields
3. Type-specific field validation: only apply fields relevant to the account's type
   - Cash: only `Name`, `Description`, `IsActive`
   - Investment: + `Broker`, `AccountNumber`
   - Credit card: + `InterestRate`, `DueDate`, `CreditLimit`
4. Apply updates atomically, return updated account

#### 3.3 Update Handler

**File**: `apps/api/internal/handlers/account_handler.go`

Replace `UpdateCashAccountRequest` and `UpdateCashAccount` handler with:

```go
type UpdateAccountRequest struct {
    Name          *string  `json:"name" binding:"omitempty,min=1,max=100"`
    Description   *string  `json:"description" binding:"omitempty,max=500"`
    IsActive      *bool    `json:"is_active"`
    Broker        *string  `json:"broker" binding:"omitempty,max=100"`
    AccountNumber *string  `json:"account_number" binding:"omitempty,max=50"`
    InterestRate  *float64 `json:"interest_rate" binding:"omitempty,gte=0,lte=100"`
    DueDate       *string  `json:"due_date"`
    CreditLimit   *int64   `json:"credit_limit" binding:"omitempty,gte=0"`
}
```

Rename handler to `UpdateAccount`. Map request to `AccountUpdateFields`, call `UpdateAccount` service.

#### 3.4 Update Error Codes

**File**: `apps/api/internal/errors/errors.go`

Remove `ErrNotCashAccount` (no longer needed since all account types can be updated).

#### 3.5 Service Tests

- Update existing `TestUpdateCashAccount` tests to use new `UpdateAccount` method
- Add `TestUpdateAccount_Investment`: update broker, account_number
- Add `TestUpdateAccount_CreditCard`: update interest_rate, due_date, credit_limit
- Add `TestUpdateAccount_IgnoresIrrelevantFields`: sending broker when updating a cash account is ignored
- Add `TestUpdateAccount_IsActive`: toggle active status

#### 3.6 Handler Tests

Update mock to implement new `UpdateAccount` method. Update existing handler tests. Add tests for the new fields.

#### 3.7 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

### Frontend Changes

#### 3.8 Update Frontend Types

**File**: `apps/web/src/types/api.ts`

Update `UpdateAccountRequest`:
```ts
export interface UpdateAccountRequest {
  name?: string;
  description?: string;
  is_active?: boolean;
  broker?: string;
  account_number?: string;
  interest_rate?: number;
  due_date?: string;
  credit_limit?: number;
}
```

#### 3.9 Update Edit Account Dialog

**File**: `apps/web/src/components/accounts/edit-account-dialog.tsx`

Extend to show type-specific editable fields:
- **All types**: Name, Description, Active/Inactive toggle
- **Investment**: + Broker, Account Number
- **Credit card**: + Interest Rate, Due Date, Credit Limit
- **Cash**: Name and Description only
- **Currency**: displayed read-only
- **Type**: displayed read-only as Badge

#### 3.10 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 4: Transaction Editing

### Backend Changes

#### 4.1 Add TransactionUpdateFields and UpdateTransaction to Interface

**File**: `apps/api/internal/services/interfaces.go`

Add to `TransactionServicer`:
```go
UpdateTransaction(userID, transactionID uint, updates TransactionUpdateFields) (*models.Transaction, error)
```

Add:
```go
type TransactionUpdateFields struct {
    AccountID   *uint
    CategoryID  **uint   // nil = don't change, *nil = clear, *val = set
    Type        *models.TransactionType
    Amount      *int64
    Description *string
    Date        *time.Time
}
```

#### 4.2 Add Error Codes

**File**: `apps/api/internal/errors/errors.go`

Add:
```go
ErrTransactionNotEditable = &AppError{Code: "TRANSACTION_NOT_EDITABLE", Message: "This transaction type cannot be edited", StatusCode: http.StatusBadRequest}
ErrInvalidTypeChange      = &AppError{Code: "INVALID_TYPE_CHANGE", Message: "Cannot change transaction type to or from transfer/investment", StatusCode: http.StatusBadRequest}
```

#### 4.3 Implement UpdateTransaction Service

**File**: `apps/api/internal/services/transaction_service.go`

Logic:
1. Fetch existing transaction by ID + user ID
2. Reject if transaction type is `transfer` or `investment` → return `ErrTransactionNotEditable`
3. If type change requested, reject changes to/from `transfer` or `investment` → return `ErrInvalidTypeChange`
4. Within a DB transaction:
   a. **Reverse old impact**: undo the balance on the old account (income → subtract, expense → add)
   b. If `AccountID` changed, fetch and validate the new account
   c. Apply field updates to the transaction record
   d. **Apply new impact**: apply the balance on the (possibly new) account with the (possibly new) type and amount
   e. Save the updated transaction

#### 4.4 Add UpdateTransaction Handler

**File**: `apps/api/internal/handlers/transaction_handler.go`

Add:
```go
type UpdateTransactionRequest struct {
    AccountID   *uint                   `json:"account_id"`
    CategoryID  *int64                  `json:"category_id"`  // nullable: -1 or null to clear
    Type        *models.TransactionType `json:"type" binding:"omitempty,transaction_type"`
    Amount      *int64                  `json:"amount" binding:"omitempty,gt=0"`
    Description *string                 `json:"description" binding:"omitempty,max=500"`
    Date        *string                 `json:"date"`
}
```

Add `UpdateTransaction` handler with Swagger annotations. Map `CategoryID` handling: JSON `null` → clear category, positive number → set category.

#### 4.5 Register Route

**File**: `apps/api/cmd/api/main.go`

Add `transactions.PUT("/:id", transactionHandler.UpdateTransaction)`.

#### 4.6 Service Tests

**File**: `apps/api/internal/services/transaction_service_test.go`

Add `TestUpdateTransaction` with subtests:
- `updates_amount_adjusts_balance`: change amount, verify old balance reversed and new applied
- `updates_type_income_to_expense`: change type, verify balance impact reversal + new impact
- `updates_account_id`: move transaction to different account, verify both accounts' balances adjusted
- `updates_category_description_date`: metadata changes, no balance impact
- `clears_category`: set category_id to nil
- `rejects_transfer_transaction`: attempt to edit a transfer, get `TRANSACTION_NOT_EDITABLE`
- `rejects_investment_transaction`: attempt to edit an investment tx, get `TRANSACTION_NOT_EDITABLE`
- `rejects_type_change_to_transfer`: attempt to change type to transfer, get `INVALID_TYPE_CHANGE`
- `rejects_type_change_to_investment`: attempt to change type to investment, get `INVALID_TYPE_CHANGE`
- `user_isolation`: user A cannot edit user B's transaction

#### 4.7 Handler Tests

Add mock method and handler tests for `UpdateTransaction`.

#### 4.8 Backend Verification

Run `./scripts/check.sh` from `apps/api/`.

### Frontend Changes

#### 4.9 Update Frontend Types

**File**: `apps/web/src/types/api.ts`

Add:
```ts
export interface UpdateTransactionRequest {
  account_id?: number;
  category_id?: number | null;
  type?: TransactionType;
  amount?: number;
  description?: string;
  date?: string;
}
```

#### 4.10 Add Frontend Hook

**File**: `apps/web/src/hooks/use-transactions.ts`

Add `useUpdateTransaction(id: number)` mutation: PUT `/api/v1/transactions/:id`. On success, invalidate `transactionKeys.all` and `accountKeys.all` (balances may have changed on multiple accounts).

#### 4.11 Create Edit Transaction Dialog

**File**: `apps/web/src/components/transactions/edit-transaction-dialog.tsx` (new)

Props: `open`, `onOpenChange`, `transaction` (Transaction object).

Behavior:
- If transaction type is `transfer` or `investment`: show read-only display of all fields + a "Delete" button only. Show info message: "Transfer/investment transactions cannot be edited. You can delete and recreate if needed."
- If transaction type is `income` or `expense`:
  - Type toggle: Expense / Income buttons (pre-selected from transaction)
  - Account dropdown (all accounts, pre-selected)
  - Amount (CurrencyInput, pre-populated)
  - Category dropdown (filtered by type, pre-selected)
  - Description (pre-populated)
  - Date (pre-populated)
  - "Save Changes" button: sends only changed fields via `useUpdateTransaction`
  - "Delete" button: opens delete confirmation

Error mapping:
- `TRANSACTION_NOT_EDITABLE` → "This transaction type cannot be edited"
- `INVALID_TYPE_CHANGE` → "Cannot change to this transaction type"
- `ACCOUNT_NOT_FOUND` → "Selected account not found"

#### 4.12 Add Delete Confirmation to Edit Dialog

Within the edit dialog (or as a nested alert dialog):
- "Delete Transaction" button (destructive variant)
- Confirmation: "Are you sure? This will reverse the balance impact."
- Uses `useDeleteTransaction` hook
- On success: toast + close both dialogs

#### 4.13 Wire Edit Dialog into Transaction Views

**Files**:
- `apps/web/src/app/(dashboard)/transactions/page.tsx`: Add click handler on transaction rows → open edit dialog with selected transaction
- `apps/web/src/app/(dashboard)/accounts/[id]/page.tsx`: Same for account detail transaction rows
- `apps/web/src/app/(dashboard)/page.tsx`: Same for dashboard recent transaction rows

Each page: add `useState` for `editTxOpen` + `selectedTransaction`, render `EditTransactionDialog`.

#### 4.14 Frontend Verification

Run `npm run build` from `apps/web/`.

---

## Phase 5: Unified Transaction Dialog with Transfer

### 5.1 Merge Transfer into Create Transaction Dialog

**File**: `apps/web/src/components/transactions/create-transaction-dialog.tsx`

Redesign the type selector to show three options: **Expense** | **Income** | **Transfer**.

When **Transfer** is selected:
- Replace single "Account" dropdown with "From Account" and "To Account" dropdowns
- Hide Category field (transfers have no category)
- Keep: Amount, Description, Date
- On submit: call `useCreateTransfer()` instead of `useCreateTransaction()`
- Validation: both accounts required, must be different, amount > 0

When **Expense** or **Income** is selected:
- Show current form unchanged (single Account, Category, Amount, Description, Date)
- On submit: call `useCreateTransaction()` as before

Update dialog title/description dynamically based on selected type.

### 5.2 Remove Separate Transfer Button & Dialog

**Files**:
- `apps/web/src/app/(dashboard)/transactions/page.tsx`: Remove "Transfer" button from header. Remove `CreateTransferDialog` import and rendering.
- `apps/web/src/app/(dashboard)/accounts/[id]/page.tsx`: Remove "Transfer" button. Remove `CreateTransferDialog` import and rendering. (The `defaultAccountId` prop still works for the unified dialog's "From Account" pre-selection when transfer is chosen.)

### 5.3 Delete Transfer Dialog Component

**File**: `apps/web/src/components/transactions/create-transfer-dialog.tsx` — delete this file. Its functionality is now absorbed into the unified `CreateTransactionDialog`.

### 5.4 Verification

Run `npm run lint && npm run build` from `apps/web/`.

---

## Backend API Reference

### New Endpoints

#### POST /api/v1/accounts/credit-card

Create a credit card account.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Max 100 chars |
| `description` | string | No | Max 500 chars |
| `currency` | string | No | ISO 4217, defaults to "USD" |
| `credit_limit` | int64 | No | In cents, >= 0 |
| `interest_rate` | float64 | No | APR percentage, 0–100 |
| `due_date` | string | No | ISO 8601 date |

**Response**: `201 { account: Account }`

#### PUT /api/v1/transactions/:id

Update a transaction. Only income/expense transactions can be edited.

| Field | Type | Required | Description |
|---|---|---|---|
| `account_id` | uint | No | Move to different account |
| `category_id` | int/null | No | Set or clear category |
| `type` | string | No | "income" or "expense" only |
| `amount` | int64 | No | In cents, > 0 |
| `description` | string | No | Max 500 chars |
| `date` | string | No | ISO 8601 |

**Response**: `200 { transaction: Transaction }`

### Modified Endpoints

#### PUT /api/v1/accounts/:id (expanded)

Now accepts all account types (not just cash). Additional optional fields:

| Field | Type | Applies To | Description |
|---|---|---|---|
| `name` | string | All | Max 100 chars |
| `description` | string | All | Max 500 chars |
| `is_active` | bool | All | Toggle active status |
| `broker` | string | Investment | Max 100 chars |
| `account_number` | string | Investment | Max 50 chars |
| `interest_rate` | float64 | Credit card | APR percentage |
| `due_date` | string | Credit card | ISO 8601 date |
| `credit_limit` | int64 | Credit card | In cents |

### New Error Codes

| Code | HTTP | When |
|---|---|---|
| `TRANSACTION_NOT_EDITABLE` | 400 | Attempting to edit a transfer or investment transaction |
| `INVALID_TYPE_CHANGE` | 400 | Attempting to change type to/from transfer or investment |

### Removed Error Codes

| Code | Replacement |
|---|---|
| `NOT_CASH_ACCOUNT` | Removed — generic update now supports all types |

---

## Migration

**000010_add_credit_card_support**

```sql
-- Up
ALTER TABLE accounts ADD COLUMN credit_limit BIGINT DEFAULT 0;

-- Down
ALTER TABLE accounts DROP COLUMN credit_limit;
```

---

## File Structure After This Plan

### Backend (new/modified)

```
apps/api/
├── migrations/
│   ├── 000010_add_credit_card_support.up.sql        # NEW
│   └── 000010_add_credit_card_support.down.sql      # NEW
├── internal/
│   ├── errors/
│   │   └── errors.go                                # MODIFIED: add new errors, remove ErrNotCashAccount
│   ├── models/
│   │   └── account.go                               # MODIFIED: add credit_card type + CreditLimit field
│   ├── validator/
│   │   └── validator.go                             # MODIFIED: add credit_card to account_type validator
│   ├── services/
│   │   ├── interfaces.go                            # MODIFIED: add CreateCreditCardAccount, UpdateAccount (replace UpdateCashAccount), UpdateTransaction, new structs
│   │   ├── account_service.go                       # MODIFIED: add CreateCreditCardAccount, replace UpdateCashAccount with UpdateAccount, update balance logic
│   │   ├── account_service_test.go                  # MODIFIED: add credit card tests, update existing tests
│   │   ├── transaction_service.go                   # MODIFIED: add UpdateTransaction, update transfer validation
│   │   └── transaction_service_test.go              # MODIFIED: add UpdateTransaction tests
│   └── handlers/
│       ├── account_handler.go                       # MODIFIED: add CreateCreditCardAccount handler, replace UpdateCashAccount with UpdateAccount
│       ├── account_handler_test.go                  # MODIFIED: add tests, update mocks
│       ├── transaction_handler.go                   # MODIFIED: add UpdateTransaction handler
│       └── transaction_handler_test.go              # MODIFIED: add tests, update mocks
├── cmd/api/
│   └── main.go                                      # MODIFIED: add new routes
```

### Frontend (new/modified)

```
apps/web/src/
├── types/
│   ├── models.ts                                    # MODIFIED: add credit_card type, credit_limit field
│   └── api.ts                                       # MODIFIED: add new request types, update UpdateAccountRequest
├── hooks/
│   ├── use-accounts.ts                              # MODIFIED: add useCreateCreditCardAccount
│   └── use-transactions.ts                          # MODIFIED: fix invalidation, add useUpdateTransaction
├── components/
│   ├── accounts/
│   │   ├── create-account-dialog.tsx                # MODIFIED: add Credit Card tab
│   │   └── edit-account-dialog.tsx                  # MODIFIED: add type-specific fields
│   └── transactions/
│       ├── create-transaction-dialog.tsx             # MODIFIED: add Transfer as inline type option
│       ├── create-transfer-dialog.tsx                # DELETED: absorbed into create-transaction-dialog
│       └── edit-transaction-dialog.tsx               # NEW: edit/delete transaction dialog
├── app/(dashboard)/
│   ├── page.tsx                                     # MODIFIED: fix query invalidation effect, update summary cards, wire edit tx dialog
│   ├── transactions/
│   │   └── page.tsx                                 # MODIFIED: remove transfer button, wire edit tx dialog
│   └── accounts/[id]/
│       └── page.tsx                                 # MODIFIED: remove transfer button, wire edit tx dialog
```

---

## Implementation Order

```
Phase 1: Dashboard Query Fix
  1.1  Fix transaction mutation invalidation keys
  1.2  Frontend build verification

Phase 2: Credit Card Accounts
  2.1   Database migration (000010)
  2.2   Update account model
  2.3   Update account validator
  2.4   Add CreateCreditCardAccount to service
  2.5   Update balance logic for credit cards
  2.6   Update transfer validation
  2.7   Add credit card handler
  2.8   Register route
  2.9   Service tests
  2.10  Handler tests
  2.11  Backend verification
  2.12  Update frontend types
  2.13  Add frontend hook
  2.14  Update create account dialog
  2.15  Update dashboard summary cards
  2.16  Frontend verification

Phase 3: Generic Account Update
  3.1   Add AccountUpdateFields, refactor interface
  3.2   Implement generic UpdateAccount
  3.3   Update handler
  3.4   Update error codes
  3.5   Service tests
  3.6   Handler tests
  3.7   Backend verification
  3.8   Update frontend types
  3.9   Update edit account dialog
  3.10  Frontend verification

Phase 4: Transaction Editing
  4.1   Add TransactionUpdateFields to interface
  4.2   Add error codes
  4.3   Implement UpdateTransaction service
  4.4   Add UpdateTransaction handler
  4.5   Register route
  4.6   Service tests
  4.7   Handler tests
  4.8   Backend verification
  4.9   Update frontend types
  4.10  Add frontend hook
  4.11  Create edit transaction dialog
  4.12  Add delete confirmation
  4.13  Wire edit dialog into transaction views
  4.14  Frontend verification

Phase 5: Unified Transaction Dialog
  5.1   Merge transfer into create transaction dialog
  5.2   Remove separate transfer button & dialog references
  5.3   Delete transfer dialog component
  5.4   Final verification (lint + build)
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
cd apps/web && npm run lint && npm run build
```

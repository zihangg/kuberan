import type {
  Account,
  Budget,
  BudgetPeriod,
  BudgetProgress,
  Category,
  Transaction,
  User,
  TransactionType,
  CategoryType,
} from "./models";

// Pagination
export interface PageResponse<T> {
  data: T[];
  page: number;
  page_size: number;
  total_items: number;
  total_pages: number;
}

export interface PaginationParams {
  page?: number;
  page_size?: number;
}

// Error response (matches backend { error: { code, message } })
export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}

// Auth requests
export interface LoginRequest {
  email: string;
  password: string;
}

export interface RegisterRequest {
  email: string;
  password: string;
  first_name?: string;
  last_name?: string;
}

export interface RefreshRequest {
  refresh_token: string;
}

// Auth response (login, register, refresh all return this)
export interface AuthResponse {
  access_token: string;
  refresh_token: string;
  user: User;
}

// Profile response
export interface ProfileResponse {
  user: User;
}

// Single-item response wrappers (backend wraps single items in a key)
export interface AccountResponse {
  account: Account;
}

export interface TransactionResponse {
  transaction: Transaction;
}

export interface CategoryResponse {
  category: Category;
}

export interface DeleteResponse {
  message: string;
}

// Account requests
export interface CreateCashAccountRequest {
  name: string;
  description?: string;
  currency?: string; // ISO 4217, defaults to USD
  initial_balance?: number; // cents, >= 0
}

export interface CreateInvestmentAccountRequest {
  name: string;
  description?: string;
  currency?: string; // ISO 4217
  broker?: string;
  account_number?: string;
}

export interface CreateCreditCardAccountRequest {
  name: string;
  description?: string;
  currency?: string;
  credit_limit?: number;
  interest_rate?: number;
  due_date?: string;
}

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

// Transaction requests
export interface CreateTransactionRequest {
  account_id: number;
  category_id?: number;
  type: TransactionType;
  amount: number; // cents, > 0
  description?: string;
  date?: string; // ISO 8601
}

export interface CreateTransferRequest {
  from_account_id: number;
  to_account_id: number;
  amount: number; // cents, > 0
  description?: string;
  date?: string; // ISO 8601
}

export interface UpdateTransactionRequest {
  account_id?: number;
  category_id?: number | null;
  type?: TransactionType;
  amount?: number; // cents, > 0
  description?: string;
  date?: string; // ISO 8601
}

export interface TransactionFilters extends PaginationParams {
  from_date?: string;
  to_date?: string;
  type?: TransactionType;
  category_id?: number;
  min_amount?: number;
  max_amount?: number;
}

export interface UserTransactionFilters extends TransactionFilters {
  account_id?: number;
}

// Category requests
export interface CreateCategoryRequest {
  name: string;
  type: CategoryType;
  description?: string;
  icon?: string;
  color?: string; // hex color
  parent_id?: number;
}

export interface UpdateCategoryRequest {
  name?: string;
  description?: string;
  icon?: string;
  color?: string;
  parent_id?: number;
}

// Budget responses
export interface BudgetResponse {
  budget: Budget;
}

export interface BudgetProgressResponse {
  progress: BudgetProgress;
}

// Budget requests
export interface CreateBudgetRequest {
  category_id: number;
  name: string;
  amount: number; // cents, > 0
  period: BudgetPeriod;
  start_date: string; // ISO 8601
  end_date?: string; // ISO 8601
}

export interface UpdateBudgetRequest {
  name?: string;
  amount?: number; // cents, > 0
  period?: BudgetPeriod;
  end_date?: string; // ISO 8601
}

// Budget filters
export interface BudgetFilters extends PaginationParams {
  is_active?: boolean;
  period?: BudgetPeriod;
}

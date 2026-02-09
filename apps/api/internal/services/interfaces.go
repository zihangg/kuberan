package services

import (
	"time"

	"gorm.io/gorm"

	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// UserServicer defines the contract for user-related business logic.
type UserServicer interface {
	CreateUser(email, password, firstName, lastName string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uint) (*models.User, error)
	VerifyPassword(user *models.User, password string) bool
	AttemptLogin(email, password string) (*models.User, error)
	StoreRefreshTokenHash(userID uint, tokenHash string) error
	GetRefreshTokenHash(userID uint) (string, error)
}

// AccountUpdateFields holds optional fields for updating an account.
// Nil pointer means "don't change"; non-nil means "set to this value".
type AccountUpdateFields struct {
	Name          *string
	Description   *string
	IsActive      *bool
	Broker        *string    // investment only
	AccountNumber *string    // investment only
	InterestRate  *float64   // credit_card only
	DueDate       *time.Time // credit_card only
	CreditLimit   *int64     // credit_card only
}

// AccountServicer defines the contract for account-related business logic.
type AccountServicer interface {
	CreateCashAccount(userID uint, name, description, currency string, initialBalance int64) (*models.Account, error)
	CreateInvestmentAccount(userID uint, name, description, currency, broker, accountNumber string) (*models.Account, error)
	CreateCreditCardAccount(userID uint, name, description, currency string, creditLimit int64, interestRate float64, dueDate *time.Time) (*models.Account, error)
	GetUserAccounts(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Account], error)
	GetAccountByID(userID, accountID uint) (*models.Account, error)
	UpdateAccount(userID, accountID uint, updates AccountUpdateFields) (*models.Account, error)
	UpdateAccountBalance(tx *gorm.DB, account *models.Account, transactionType models.TransactionType, amount int64) error
}

// CategoryServicer defines the contract for category-related business logic.
type CategoryServicer interface {
	CreateCategory(userID uint, name string, categoryType models.CategoryType, description, icon, color string, parentID *uint) (*models.Category, error)
	GetUserCategories(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Category], error)
	GetUserCategoriesByType(userID uint, categoryType models.CategoryType, page pagination.PageRequest) (*pagination.PageResponse[models.Category], error)
	GetCategoryByID(userID, categoryID uint) (*models.Category, error)
	UpdateCategory(userID, categoryID uint, name, description, icon, color string, parentID *uint) (*models.Category, error)
	DeleteCategory(userID, categoryID uint) error
}

// TransactionUpdateFields holds optional fields for updating a transaction.
// Nil pointer means "don't change"; non-nil means "set to this value".
// CategoryID uses a double pointer: nil=no change, *nil=clear, *value=set.
type TransactionUpdateFields struct {
	AccountID   *uint
	CategoryID  **uint
	Type        *models.TransactionType
	Amount      *int64
	Description *string
	Date        *time.Time
}

// TransactionFilter holds optional filter parameters for listing transactions.
type TransactionFilter struct {
	FromDate   *time.Time
	ToDate     *time.Time
	Type       *models.TransactionType
	CategoryID *uint
	MinAmount  *int64
	MaxAmount  *int64
	AccountID  *uint
}

// TransactionServicer defines the contract for transaction-related business logic.
type TransactionServicer interface {
	CreateTransaction(userID, accountID uint, categoryID *uint, transactionType models.TransactionType, amount int64, description string, date time.Time) (*models.Transaction, error)
	CreateTransfer(userID, fromAccountID, toAccountID uint, amount int64, description string, date time.Time) (*models.Transaction, error)
	GetAccountTransactions(userID, accountID uint, page pagination.PageRequest, filter TransactionFilter) (*pagination.PageResponse[models.Transaction], error)
	GetUserTransactions(userID uint, page pagination.PageRequest, filter TransactionFilter) (*pagination.PageResponse[models.Transaction], error)
	GetTransactionByID(userID, transactionID uint) (*models.Transaction, error)
	UpdateTransaction(userID, transactionID uint, updates TransactionUpdateFields) (*models.Transaction, error)
	DeleteTransaction(userID, transactionID uint) error
}

// BudgetProgress contains spending vs budget data for a budget's current period.
type BudgetProgress struct {
	BudgetID   uint    `json:"budget_id"`
	Budgeted   int64   `json:"budgeted"`
	Spent      int64   `json:"spent"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
}

// BudgetServicer defines the contract for budget-related business logic.
type BudgetServicer interface {
	CreateBudget(userID, categoryID uint, name string, amount int64, period models.BudgetPeriod, startDate time.Time, endDate *time.Time) (*models.Budget, error)
	GetUserBudgets(userID uint, page pagination.PageRequest, isActive *bool, period *models.BudgetPeriod) (*pagination.PageResponse[models.Budget], error)
	GetBudgetByID(userID, budgetID uint) (*models.Budget, error)
	UpdateBudget(userID, budgetID uint, name string, amount *int64, period *models.BudgetPeriod, endDate *time.Time) (*models.Budget, error)
	DeleteBudget(userID, budgetID uint) error
	GetBudgetProgress(userID, budgetID uint) (*BudgetProgress, error)
}

// PortfolioSummary contains aggregated portfolio data across all investment accounts.
type PortfolioSummary struct {
	TotalValue     int64                            `json:"total_value"`
	TotalCostBasis int64                            `json:"total_cost_basis"`
	TotalGainLoss  int64                            `json:"total_gain_loss"`
	GainLossPct    float64                          `json:"gain_loss_pct"`
	HoldingsByType map[models.AssetType]TypeSummary `json:"holdings_by_type"`
}

// TypeSummary contains summary data for a single asset type.
type TypeSummary struct {
	Value int64 `json:"value"`
	Count int   `json:"count"`
}

// InvestmentServicer defines the contract for investment-related business logic.
type InvestmentServicer interface {
	AddInvestment(userID, accountID uint, symbol, name string, assetType models.AssetType, quantity float64, purchasePrice int64, currency string, extraFields map[string]interface{}) (*models.Investment, error)
	GetAccountInvestments(userID, accountID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error)
	GetInvestmentByID(userID, investmentID uint) (*models.Investment, error)
	UpdateInvestmentPrice(userID, investmentID uint, currentPrice int64) (*models.Investment, error)
	GetPortfolio(userID uint) (*PortfolioSummary, error)
	RecordBuy(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)
	RecordSell(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)
	RecordDividend(userID, investmentID uint, date time.Time, amount int64, dividendType, notes string) (*models.InvestmentTransaction, error)
	RecordSplit(userID, investmentID uint, date time.Time, splitRatio float64, notes string) (*models.InvestmentTransaction, error)
	GetInvestmentTransactions(userID, investmentID uint, page pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error)
}

// AuditServicer defines the contract for audit logging.
type AuditServicer interface {
	Log(userID uint, action, resourceType string, resourceID uint, ipAddress string, changes map[string]interface{})
}

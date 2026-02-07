package services

import (
	"time"

	"gorm.io/gorm"

	"kuberan/internal/models"
)

// UserServicer defines the contract for user-related business logic.
type UserServicer interface {
	CreateUser(email, password, firstName, lastName string) (*models.User, error)
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uint) (*models.User, error)
	VerifyPassword(user *models.User, password string) bool
	AttemptLogin(email, password string) (*models.User, error)
}

// AccountServicer defines the contract for account-related business logic.
type AccountServicer interface {
	CreateCashAccount(userID uint, name, description, currency string, initialBalance int64) (*models.Account, error)
	GetUserAccounts(userID uint) ([]models.Account, error)
	GetAccountByID(userID, accountID uint) (*models.Account, error)
	UpdateCashAccount(userID, accountID uint, name, description string) (*models.Account, error)
	UpdateAccountBalance(tx *gorm.DB, account *models.Account, transactionType models.TransactionType, amount int64) error
}

// CategoryServicer defines the contract for category-related business logic.
type CategoryServicer interface {
	CreateCategory(userID uint, name string, categoryType models.CategoryType, description, icon, color string, parentID *uint) (*models.Category, error)
	GetUserCategories(userID uint) ([]models.Category, error)
	GetUserCategoriesByType(userID uint, categoryType models.CategoryType) ([]models.Category, error)
	GetCategoryByID(userID, categoryID uint) (*models.Category, error)
	UpdateCategory(userID, categoryID uint, name, description, icon, color string, parentID *uint) (*models.Category, error)
	DeleteCategory(userID, categoryID uint) error
}

// TransactionServicer defines the contract for transaction-related business logic.
type TransactionServicer interface {
	CreateTransaction(userID, accountID uint, categoryID *uint, transactionType models.TransactionType, amount int64, description string, date time.Time) (*models.Transaction, error)
	GetAccountTransactions(userID, accountID uint) ([]models.Transaction, error)
	GetTransactionByID(userID, transactionID uint) (*models.Transaction, error)
	DeleteTransaction(userID, transactionID uint) error
}

package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// TransactionService handles transaction-related business logic
type TransactionService struct {
	db             *gorm.DB
	accountService *AccountService
}

// NewTransactionService creates a new TransactionService
func NewTransactionService(db *gorm.DB, accountService *AccountService) *TransactionService {
	return &TransactionService{
		db:             db,
		accountService: accountService,
	}
}

// CreateTransaction creates a new transaction for a user's account
func (s *TransactionService) CreateTransaction(
	userID uint,
	accountID uint,
	categoryID *uint,
	transactionType models.TransactionType,
	amount float64,
	description string,
	date time.Time,
) (*models.Transaction, error) {
	// Validate input
	if amount <= 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "amount must be greater than zero")
	}

	if accountID == 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "account ID is required")
	}

	// Default date to now if not provided
	if date.IsZero() {
		date = time.Now()
	}

	// Get the account to ensure it exists and belongs to the user
	account, err := s.accountService.GetAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}

	var result *models.Transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var txErr error
		result, txErr = s.createTransactionWithDB(tx, userID, account, categoryID, transactionType, amount, description, date)
		return txErr
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// createTransactionWithDB creates a transaction with a given database connection (useful for transactions)
func (s *TransactionService) createTransactionWithDB(
	tx *gorm.DB,
	userID uint,
	account *models.Account,
	categoryID *uint,
	transactionType models.TransactionType,
	amount float64,
	description string,
	date time.Time,
) (*models.Transaction, error) {
	// Create transaction record
	transaction := &models.Transaction{
		UserID:      userID,
		AccountID:   account.ID,
		CategoryID:  categoryID,
		Type:        transactionType,
		Amount:      amount,
		Description: description,
		Date:        date,
	}

	if err := tx.Create(transaction).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	if err := s.accountService.UpdateAccountBalance(tx, account, transactionType, amount); err != nil {
		return nil, err
	}

	return transaction, nil
}

// GetAccountTransactions retrieves transactions for a specific account
func (s *TransactionService) GetAccountTransactions(userID, accountID uint) ([]models.Transaction, error) {
	// First verify the account belongs to the user
	_, err := s.accountService.GetAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}

	var transactions []models.Transaction
	if err := s.db.Where("user_id = ? AND account_id = ?", userID, accountID).
		Order("date DESC").
		Find(&transactions).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return transactions, nil
}

// GetTransactionByID retrieves a transaction by ID for a specific user
func (s *TransactionService) GetTransactionByID(userID, transactionID uint) (*models.Transaction, error) {
	var transaction models.Transaction
	if err := s.db.Where("id = ? AND user_id = ?", transactionID, userID).First(&transaction).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrTransactionNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &transaction, nil
}

// DeleteTransaction deletes a transaction and updates the account balance
func (s *TransactionService) DeleteTransaction(userID, transactionID uint) error {
	// Get the transaction
	transaction, err := s.GetTransactionByID(userID, transactionID)
	if err != nil {
		return err
	}

	// Get the account
	account, err := s.accountService.GetAccountByID(userID, transaction.AccountID)
	if err != nil {
		return err
	}

	// Within a database transaction to ensure consistency
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete the transaction
		if err := tx.Delete(transaction).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}

		// Reverse the effect on the account balance
		var reverseType models.TransactionType
		switch transaction.Type {
		case models.TransactionTypeIncome:
			reverseType = models.TransactionTypeExpense
		case models.TransactionTypeExpense:
			reverseType = models.TransactionTypeIncome
		default:
			return apperrors.ErrInvalidTransactionType
		}

		// Update account balance
		if err := s.accountService.UpdateAccountBalance(tx, account, reverseType, transaction.Amount); err != nil {
			return err
		}

		return nil
	})
}

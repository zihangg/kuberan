package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// transactionService handles transaction-related business logic.
type transactionService struct {
	db             *gorm.DB
	accountService AccountServicer
}

// NewTransactionService creates a new TransactionServicer.
func NewTransactionService(db *gorm.DB, accountService AccountServicer) TransactionServicer {
	return &transactionService{
		db:             db,
		accountService: accountService,
	}
}

// CreateTransaction creates a new transaction for a user's account
func (s *transactionService) CreateTransaction(
	userID uint,
	accountID uint,
	categoryID *uint,
	transactionType models.TransactionType,
	amount int64,
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
func (s *transactionService) createTransactionWithDB(
	tx *gorm.DB,
	userID uint,
	account *models.Account,
	categoryID *uint,
	transactionType models.TransactionType,
	amount int64,
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

// CreateTransfer creates an account-to-account transfer within a single DB transaction.
func (s *transactionService) CreateTransfer(
	userID, fromAccountID, toAccountID uint,
	amount int64,
	description string,
	date time.Time,
) (*models.Transaction, error) {
	if fromAccountID == toAccountID {
		return nil, apperrors.ErrSameAccountTransfer
	}

	if amount <= 0 {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "amount must be greater than zero")
	}

	if date.IsZero() {
		date = time.Now()
	}

	fromAccount, err := s.accountService.GetAccountByID(userID, fromAccountID)
	if err != nil {
		return nil, err
	}

	toAccount, err := s.accountService.GetAccountByID(userID, toAccountID)
	if err != nil {
		return nil, err
	}

	if fromAccount.Balance < amount {
		return nil, apperrors.ErrInsufficientBalance
	}

	var result *models.Transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		transaction := &models.Transaction{
			UserID:      userID,
			AccountID:   fromAccountID,
			ToAccountID: &toAccountID,
			Type:        models.TransactionTypeTransfer,
			Amount:      amount,
			Description: description,
			Date:        date,
		}
		if txErr := tx.Create(transaction).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		if txErr := s.accountService.UpdateAccountBalance(tx, fromAccount, models.TransactionTypeExpense, amount); txErr != nil {
			return txErr
		}
		if txErr := s.accountService.UpdateAccountBalance(tx, toAccount, models.TransactionTypeIncome, amount); txErr != nil {
			return txErr
		}

		result = transaction
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// GetAccountTransactions retrieves a paginated, filtered list of transactions for a specific account.
func (s *transactionService) GetAccountTransactions(userID, accountID uint, page pagination.PageRequest, filter TransactionFilter) (*pagination.PageResponse[models.Transaction], error) {
	// First verify the account belongs to the user
	_, err := s.accountService.GetAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}

	page.Defaults()

	base := s.db.Model(&models.Transaction{}).Where("user_id = ? AND account_id = ?", userID, accountID)
	base = applyTransactionFilters(base, filter)

	var totalItems int64
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var transactions []models.Transaction
	if err := base.Scopes(pagination.Paginate(page)).
		Order("date DESC").
		Find(&transactions).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(transactions, page.Page, page.PageSize, totalItems)
	return &result, nil
}

func applyTransactionFilters(q *gorm.DB, f TransactionFilter) *gorm.DB {
	if f.FromDate != nil {
		q = q.Where("date >= ?", *f.FromDate)
	}
	if f.ToDate != nil {
		q = q.Where("date <= ?", *f.ToDate)
	}
	if f.Type != nil {
		q = q.Where("type = ?", *f.Type)
	}
	if f.CategoryID != nil {
		q = q.Where("category_id = ?", *f.CategoryID)
	}
	if f.MinAmount != nil {
		q = q.Where("amount >= ?", *f.MinAmount)
	}
	if f.MaxAmount != nil {
		q = q.Where("amount <= ?", *f.MaxAmount)
	}
	return q
}

// GetTransactionByID retrieves a transaction by ID for a specific user
func (s *transactionService) GetTransactionByID(userID, transactionID uint) (*models.Transaction, error) {
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
func (s *transactionService) DeleteTransaction(userID, transactionID uint) error {
	transaction, err := s.GetTransactionByID(userID, transactionID)
	if err != nil {
		return err
	}

	account, err := s.accountService.GetAccountByID(userID, transaction.AccountID)
	if err != nil {
		return err
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if txErr := tx.Delete(transaction).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		switch transaction.Type {
		case models.TransactionTypeIncome:
			return s.accountService.UpdateAccountBalance(tx, account, models.TransactionTypeExpense, transaction.Amount)
		case models.TransactionTypeExpense:
			return s.accountService.UpdateAccountBalance(tx, account, models.TransactionTypeIncome, transaction.Amount)
		case models.TransactionTypeTransfer:
			if transaction.ToAccountID == nil {
				return apperrors.ErrInvalidTransactionType
			}
			toAccount, toErr := s.accountService.GetAccountByID(userID, *transaction.ToAccountID)
			if toErr != nil {
				return toErr
			}
			// Reverse: add back to from-account, subtract from to-account
			if txErr := s.accountService.UpdateAccountBalance(tx, account, models.TransactionTypeIncome, transaction.Amount); txErr != nil {
				return txErr
			}
			return s.accountService.UpdateAccountBalance(tx, toAccount, models.TransactionTypeExpense, transaction.Amount)
		default:
			return apperrors.ErrInvalidTransactionType
		}
	})
}

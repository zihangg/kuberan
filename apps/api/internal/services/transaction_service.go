package services

import (
	"errors"
	"sort"
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

	if fromAccount.Type != models.AccountTypeCreditCard && fromAccount.Balance < amount {
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

// reverseType flips income↔expense for balance reversal.
func reverseType(t models.TransactionType) models.TransactionType {
	if t == models.TransactionTypeIncome {
		return models.TransactionTypeExpense
	}
	return models.TransactionTypeIncome
}

// UpdateTransaction updates an existing income/expense transaction.
// Transfer and investment transactions cannot be edited.
func (s *transactionService) UpdateTransaction(userID, transactionID uint, updates TransactionUpdateFields) (*models.Transaction, error) {
	transaction, err := s.GetTransactionByID(userID, transactionID)
	if err != nil {
		return nil, err
	}

	// Reject transfers and investment transactions
	if transaction.Type == models.TransactionTypeTransfer || transaction.Type == models.TransactionTypeInvestment {
		return nil, apperrors.ErrTransactionNotEditable
	}

	// If type change requested, reject changes to/from transfer or investment
	if updates.Type != nil {
		newType := *updates.Type
		if newType == models.TransactionTypeTransfer || newType == models.TransactionTypeInvestment {
			return nil, apperrors.ErrInvalidTypeChange
		}
	}

	// Capture old values
	oldAccountID := transaction.AccountID
	oldType := transaction.Type
	oldAmount := transaction.Amount

	// Determine new values
	newAccountID := oldAccountID
	if updates.AccountID != nil {
		newAccountID = *updates.AccountID
	}
	newType := oldType
	if updates.Type != nil {
		newType = *updates.Type
	}
	newAmount := oldAmount
	if updates.Amount != nil {
		newAmount = *updates.Amount
	}

	// Fetch old account
	oldAccount, err := s.accountService.GetAccountByID(userID, oldAccountID)
	if err != nil {
		return nil, err
	}

	// If account is changing, fetch the new account
	var targetAccount *models.Account
	if newAccountID != oldAccountID {
		targetAccount, err = s.accountService.GetAccountByID(userID, newAccountID)
		if err != nil {
			return nil, err
		}
	} else {
		targetAccount = oldAccount
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Reverse old impact on old account
		if txErr := s.accountService.UpdateAccountBalance(tx, oldAccount, reverseType(oldType), oldAmount); txErr != nil {
			return txErr
		}

		// Apply field updates
		if updates.AccountID != nil {
			transaction.AccountID = *updates.AccountID
		}
		if updates.Type != nil {
			transaction.Type = *updates.Type
		}
		if updates.Amount != nil {
			transaction.Amount = *updates.Amount
		}
		if updates.Description != nil {
			transaction.Description = *updates.Description
		}
		if updates.Date != nil {
			transaction.Date = *updates.Date
		}
		if updates.CategoryID != nil {
			transaction.CategoryID = *updates.CategoryID
		}

		if txErr := tx.Save(transaction).Error; txErr != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, txErr)
		}

		// Apply new impact on target account
		if txErr := s.accountService.UpdateAccountBalance(tx, targetAccount, newType, newAmount); txErr != nil {
			return txErr
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return transaction, nil
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
	if f.AccountID != nil {
		q = q.Where("account_id = ?", *f.AccountID)
	}
	return q
}

// GetUserTransactions retrieves a paginated, filtered list of all transactions for a user across all accounts.
func (s *transactionService) GetUserTransactions(userID uint, page pagination.PageRequest, filter TransactionFilter) (*pagination.PageResponse[models.Transaction], error) {
	page.Defaults()

	base := s.db.Model(&models.Transaction{}).Where("user_id = ?", userID)
	base = applyTransactionFilters(base, filter)

	var totalItems int64
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var transactions []models.Transaction
	if err := base.Preload("Category").
		Scopes(pagination.Paginate(page)).
		Order("date DESC").
		Find(&transactions).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(transactions, page.Page, page.PageSize, totalItems)
	return &result, nil
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

// GetMonthlySummary returns monthly income and expense totals for the last N months.
func (s *transactionService) GetMonthlySummary(userID uint, months int) ([]MonthlySummaryItem, error) {
	now := time.Now()
	startMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -(months - 1), 0)

	items := make([]MonthlySummaryItem, 0, months)

	current := startMonth
	for i := 0; i < months; i++ {
		monthStart := current
		monthEnd := current.AddDate(0, 1, 0).Add(-time.Nanosecond)

		var income int64
		if err := s.db.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND type = ? AND deleted_at IS NULL AND date BETWEEN ? AND ?",
				userID, models.TransactionTypeIncome, monthStart, monthEnd).
			Scan(&income).Error; err != nil {
			return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
		}

		var expenses int64
		if err := s.db.Model(&models.Transaction{}).
			Select("COALESCE(SUM(amount), 0)").
			Where("user_id = ? AND type = ? AND deleted_at IS NULL AND date BETWEEN ? AND ?",
				userID, models.TransactionTypeExpense, monthStart, monthEnd).
			Scan(&expenses).Error; err != nil {
			return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
		}

		items = append(items, MonthlySummaryItem{
			Month:    monthStart.Format("2006-01"),
			Income:   income,
			Expenses: expenses,
		})

		current = current.AddDate(0, 1, 0)
	}

	return items, nil
}

// GetSpendingByCategory returns expense totals grouped by category for a date range.
func (s *transactionService) GetSpendingByCategory(userID uint, from, to time.Time) (*SpendingByCategory, error) {
	type categorySpend struct {
		CategoryID *uint
		Total      int64
	}

	var results []categorySpend
	err := s.db.Model(&models.Transaction{}).
		Select("category_id, COALESCE(SUM(amount), 0) as total").
		Where("user_id = ? AND type = ? AND deleted_at IS NULL AND date BETWEEN ? AND ?",
			userID, models.TransactionTypeExpense, from, to).
		Group("category_id").
		Scan(&results).Error
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var items []SpendingByCategoryItem
	var totalSpent int64

	for _, r := range results {
		item := SpendingByCategoryItem{
			CategoryID: r.CategoryID,
			Total:      r.Total,
		}

		if r.CategoryID != nil {
			var category models.Category
			if catErr := s.db.First(&category, *r.CategoryID).Error; catErr != nil {
				item.CategoryName = "Unknown Category"
				item.CategoryColor = "#9CA3AF"
			} else {
				item.CategoryName = category.Name
				item.CategoryColor = category.Color
				item.CategoryIcon = category.Icon
			}
		} else {
			item.CategoryName = "Uncategorized"
			item.CategoryColor = "#9CA3AF"
		}

		totalSpent += r.Total
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Total > items[j].Total
	})

	if items == nil {
		items = []SpendingByCategoryItem{}
	}

	return &SpendingByCategory{
		Items:      items,
		TotalSpent: totalSpent,
		FromDate:   from,
		ToDate:     to,
	}, nil
}

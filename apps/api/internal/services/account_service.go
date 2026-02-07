package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// accountService handles account-related business logic.
type accountService struct {
	db *gorm.DB
}

// NewAccountService creates a new AccountServicer.
func NewAccountService(db *gorm.DB) AccountServicer {
	return &accountService{db: db}
}

// CreateCashAccount creates a new cash account for a user
func (s *accountService) CreateCashAccount(userID uint, name, description, currency string, initialBalance int64) (*models.Account, error) {
	// Validate input
	if name == "" {
		return nil, apperrors.WithMessage(apperrors.ErrInvalidInput, "account name is required")
	}

	if currency == "" {
		currency = "USD" // Default currency
	}

	// Create account
	account := &models.Account{
		UserID:      userID,
		Name:        name,
		Type:        models.AccountTypeCash,
		Description: description,
		Balance:     initialBalance,
		Currency:    currency,
		IsActive:    true,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(account).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}

		if initialBalance > 0 {
			transaction := &models.Transaction{
				UserID:      userID,
				AccountID:   account.ID,
				Type:        models.TransactionTypeIncome,
				Amount:      initialBalance,
				Description: "Initial balance",
				Date:        time.Now(),
			}
			if err := tx.Create(transaction).Error; err != nil {
				return apperrors.Wrap(apperrors.ErrInternalServer, err)
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return account, nil
}

// GetUserAccounts retrieves a paginated list of accounts for a user.
func (s *accountService) GetUserAccounts(userID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Account], error) {
	page.Defaults()

	var totalItems int64
	base := s.db.Model(&models.Account{}).Where("user_id = ? AND is_active = ?", userID, true)
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var accounts []models.Account
	if err := base.Scopes(pagination.Paginate(page)).Find(&accounts).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(accounts, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// GetAccountByID retrieves an account by ID for a specific user
func (s *accountService) GetAccountByID(userID, accountID uint) (*models.Account, error) {
	var account models.Account
	if err := s.db.Where("id = ? AND user_id = ? AND is_active = ?", accountID, userID, true).First(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrAccountNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &account, nil
}

// UpdateCashAccount updates an existing cash account
func (s *accountService) UpdateCashAccount(userID, accountID uint, name, description string) (*models.Account, error) {
	// Get the account
	account, err := s.GetAccountByID(userID, accountID)
	if err != nil {
		return nil, err
	}

	// Ensure it's a cash account
	if account.Type != models.AccountTypeCash {
		return nil, apperrors.ErrNotCashAccount
	}

	// Update fields if provided
	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}
	if description != "" {
		updates["description"] = description
	}

	// Apply updates if any
	if len(updates) > 0 {
		if err := s.db.Model(account).Updates(updates).Error; err != nil {
			return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
	}

	return account, nil
}

// UpdateAccountBalance updates the balance of an account based on transaction
func (s *accountService) UpdateAccountBalance(tx *gorm.DB, account *models.Account, transactionType models.TransactionType, amount int64) error {
	// Update the balance based on transaction type
	switch transactionType {
	case models.TransactionTypeIncome:
		account.Balance += amount
	case models.TransactionTypeExpense:
		account.Balance -= amount
	}

	// Save the updated balance
	if err := tx.Model(account).Update("balance", account.Balance).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

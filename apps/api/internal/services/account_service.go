package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// AccountService handles account-related business logic
type AccountService struct {
	db *gorm.DB
}

// NewAccountService creates a new AccountService
func NewAccountService(db *gorm.DB) *AccountService {
	return &AccountService{db: db}
}

// CreateCashAccount creates a new cash account for a user
func (s *AccountService) CreateCashAccount(userID uint, name, description, currency string, initialBalance float64) (*models.Account, error) {
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

// GetUserAccounts retrieves all accounts for a user
func (s *AccountService) GetUserAccounts(userID uint) ([]models.Account, error) {
	var accounts []models.Account
	if err := s.db.Where("user_id = ? AND is_active = ?", userID, true).Find(&accounts).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return accounts, nil
}

// GetAccountByID retrieves an account by ID for a specific user
func (s *AccountService) GetAccountByID(userID, accountID uint) (*models.Account, error) {
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
func (s *AccountService) UpdateCashAccount(userID, accountID uint, name, description string) (*models.Account, error) {
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
func (s *AccountService) UpdateAccountBalance(tx *gorm.DB, account *models.Account, transactionType models.TransactionType, amount float64) error {
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

package models

import "time"

// TransactionType represents the type of transaction
type TransactionType string

const (
	TransactionTypeIncome     TransactionType = "income"
	TransactionTypeExpense    TransactionType = "expense"
	TransactionTypeTransfer   TransactionType = "transfer"
	TransactionTypeInvestment TransactionType = "investment"
)

// Transaction represents a financial transaction in the system
type Transaction struct {
	Base
	UserID      uint            `gorm:"not null" json:"user_id"`
	AccountID   uint            `gorm:"not null" json:"account_id"`
	CategoryID  *uint           `json:"category_id,omitempty"`
	Type        TransactionType `gorm:"not null" json:"type"`
	Amount      int64           `gorm:"type:bigint;not null" json:"amount"`
	Description string          `json:"description"`
	Date        time.Time       `gorm:"not null" json:"date"`

	// For transfers
	ToAccountID *uint `json:"to_account_id,omitempty"`

	// Relationships
	Account   Account   `gorm:"foreignKey:AccountID" json:"account"`
	ToAccount *Account  `gorm:"foreignKey:ToAccountID" json:"to_account,omitempty"`
	Category  *Category `gorm:"foreignKey:CategoryID" json:"category,omitempty"`
}

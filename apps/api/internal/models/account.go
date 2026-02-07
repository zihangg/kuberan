package models

import (
	"time"

	"gorm.io/gorm"
)

// AccountType represents the type of account
type AccountType string

const (
	AccountTypeCash       AccountType = "cash"
	AccountTypeInvestment AccountType = "investment"
	AccountTypeDebt       AccountType = "debt"
)

// Account represents a financial account in the system
type Account struct {
	Base
	UserID      uint        `gorm:"not null" json:"user_id"`
	Name        string      `gorm:"not null" json:"name"`
	Type        AccountType `gorm:"not null" json:"type"`
	Description string      `json:"description"`
	Balance     float64     `gorm:"not null;default:0" json:"balance"`
	Currency    string      `gorm:"not null;default:'USD'" json:"currency"`
	IsActive    bool        `gorm:"default:true" json:"is_active"`

	// For investment accounts
	Broker        string       `json:"broker,omitempty"` // E.g., Robinhood, Fidelity, etc.
	AccountNumber string       `json:"account_number,omitempty"`
	Investments   []Investment `gorm:"foreignKey:AccountID" json:"investments,omitempty"`

	// For debt accounts
	InterestRate float64   `json:"interest_rate,omitempty"`
	DueDate      time.Time `json:"due_date,omitempty"`

	// Relationships
	Transactions []Transaction `gorm:"foreignKey:AccountID" json:"transactions,omitempty"`
}

// BeforeCreate hook to set default values based on account type
func (a *Account) BeforeCreate(tx *gorm.DB) error {
	switch a.Type {
	case AccountTypeCash:
		a.Broker = ""
		a.AccountNumber = ""
		a.InterestRate = 0
	case AccountTypeInvestment:
		a.InterestRate = 0
	case AccountTypeDebt:
		a.Broker = ""
		a.AccountNumber = ""
	}
	return nil
}

// CalculateInvestmentBalance calculates the total value of all investments in the account
func (a *Account) CalculateInvestmentBalance(tx *gorm.DB) error {
	if a.Type != AccountTypeInvestment {
		return nil
	}

	var total float64
	for i := range a.Investments {
		total += a.Investments[i].Quantity * a.Investments[i].CurrentPrice
	}
	a.Balance = total
	return tx.Save(a).Error
}

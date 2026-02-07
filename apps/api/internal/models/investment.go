package models

import "time"

// AssetType represents the type of investment asset
type AssetType string

const (
	AssetTypeStock  AssetType = "stock"
	AssetTypeETF    AssetType = "etf"
	AssetTypeBond   AssetType = "bond"
	AssetTypeCrypto AssetType = "crypto"
	AssetTypeREIT   AssetType = "reit"
)

// Investment represents a holding of a specific investment asset
type Investment struct {
	Base
	AccountID    uint      `gorm:"not null" json:"account_id"`
	Symbol       string    `gorm:"not null" json:"symbol"` // Stock/ETF symbol or unique identifier
	AssetType    AssetType `gorm:"not null" json:"asset_type"`
	Name         string    `gorm:"not null" json:"name"`       // Full name of the investment
	Quantity     float64   `gorm:"not null" json:"quantity"`   // Number of shares/units held
	CostBasis    float64   `gorm:"not null" json:"cost_basis"` // Total cost basis for this holding
	CurrentPrice float64   `json:"current_price"`              // Current market price per unit
	LastUpdated  time.Time `json:"last_updated"`               // Last time the price was updated
	Currency     string    `gorm:"not null;default:'USD'" json:"currency"`

	// Stock/ETF specific fields
	Exchange string `json:"exchange,omitempty"` // NYSE, NASDAQ, etc.

	// Bond specific fields
	MaturityDate    *time.Time `json:"maturity_date,omitempty"`
	YieldToMaturity float64    `json:"yield_to_maturity,omitempty"`
	CouponRate      float64    `json:"coupon_rate,omitempty"`

	// Crypto specific fields
	Network       string `json:"network,omitempty"` // Ethereum, Bitcoin, etc.
	WalletAddress string `json:"wallet_address,omitempty"`

	// REIT specific fields
	PropertyType string `json:"property_type,omitempty"` // Residential, Commercial, etc.

	// Relationships
	Account      Account                 `gorm:"foreignKey:AccountID" json:"account"`
	Transactions []InvestmentTransaction `gorm:"foreignKey:InvestmentID" json:"transactions,omitempty"`
}

// InvestmentTransactionType represents the type of investment transaction
type InvestmentTransactionType string

const (
	InvestmentTransactionBuy      InvestmentTransactionType = "buy"
	InvestmentTransactionSell     InvestmentTransactionType = "sell"
	InvestmentTransactionDividend InvestmentTransactionType = "dividend"
	InvestmentTransactionSplit    InvestmentTransactionType = "split"
	InvestmentTransactionTransfer InvestmentTransactionType = "transfer"
)

// InvestmentTransaction represents a transaction for an investment
type InvestmentTransaction struct {
	Base
	InvestmentID uint                      `gorm:"not null" json:"investment_id"`
	Type         InvestmentTransactionType `gorm:"not null" json:"type"`
	Date         time.Time                 `gorm:"not null" json:"date"`
	Quantity     float64                   `gorm:"not null" json:"quantity"`
	PricePerUnit float64                   `gorm:"not null" json:"price_per_unit"`
	TotalAmount  float64                   `gorm:"not null" json:"total_amount"`
	Fee          float64                   `json:"fee"`
	Notes        string                    `json:"notes"`

	// For splits
	SplitRatio float64 `json:"split_ratio,omitempty"`

	// For dividends
	DividendType string `json:"dividend_type,omitempty"` // Cash, Stock, Special

	// Relationships
	Investment Investment `gorm:"foreignKey:InvestmentID" json:"investment"`
}

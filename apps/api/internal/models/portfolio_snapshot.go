package models

import "time"

// PortfolioSnapshot represents a point-in-time snapshot of a user's net worth.
// This is immutable time-series data — no Base embed, no soft deletes.
type PortfolioSnapshot struct {
	ID              uint      `gorm:"primarykey" json:"id"`
	UserID          uint      `gorm:"not null" json:"user_id"`
	RecordedAt      time.Time `gorm:"not null" json:"recorded_at"`
	TotalNetWorth   int64     `gorm:"type:bigint;not null" json:"total_net_worth"`
	CashBalance     int64     `gorm:"type:bigint;not null" json:"cash_balance"`
	InvestmentValue int64     `gorm:"type:bigint;not null" json:"investment_value"`
	DebtBalance     int64     `gorm:"type:bigint;not null" json:"debt_balance"`
}

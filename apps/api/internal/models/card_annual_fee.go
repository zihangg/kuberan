package models

// WaiverType says what a card's annual-fee waiver asks for.
type WaiverType string

// Waiver types (plan 025).
const (
	// WaiverTypeSpend waives the fee once enough is spent.
	WaiverTypeSpend WaiverType = "spend"
	// WaiverTypeSwipes waives the fee once enough purchases are made.
	WaiverTypeSwipes WaiverType = "swipes"
	// WaiverTypeSpendOrSwipes waives the fee when either target is met.
	WaiverTypeSpendOrSwipes WaiverType = "spend_or_swipes"
)

// WaiverWindow is the period in which the waiver target must be met.
type WaiverWindow string

// Waiver windows (plan 025).
const (
	// WaiverWindowCardYear runs for the year ending on the fee date.
	WaiverWindowCardYear WaiverWindow = "card_year"
	// WaiverWindowCalendarYear runs from 1 January to 31 December.
	WaiverWindowCalendarYear WaiverWindow = "calendar_year"
)

// CardAnnualFee is the annual fee of a credit card and its optional waiver
// condition. It is a 1:1 extension of the Account row; a card without a fee has
// no row.
type CardAnnualFee struct {
	Base

	// AccountID is the FK to the credit card account (unique 1:1).
	AccountID string `gorm:"type:uuid;not null;uniqueIndex" json:"account_id"`
	// UserID owns the row (every query is user-scoped).
	UserID string `gorm:"type:uuid;not null;index" json:"user_id"`
	// Amount is the fee in cents, in the card's currency.
	Amount int64 `gorm:"type:bigint;not null" json:"amount"`
	// FeeMonth is the month (1-12) the fee is charged.
	FeeMonth int `gorm:"type:smallint;not null" json:"fee_month"`
	// FeeDay is the day of the month (1-31); nil means "sometime in the month" (the 1st).
	FeeDay *int `gorm:"type:smallint" json:"fee_day"`
	// WaiverType is nil when the fee cannot be waived.
	WaiverType *WaiverType `gorm:"type:varchar(16)" json:"waiver_type"`
	// WaiverSpend is the spend target in cents, in the card's currency.
	WaiverSpend *int64 `gorm:"type:bigint" json:"waiver_spend"`
	// WaiverSwipes is the number of purchases that waives the fee.
	WaiverSwipes *int `gorm:"type:int" json:"waiver_swipes"`
	// WaiverWindow is the period the targets are measured over.
	WaiverWindow WaiverWindow `gorm:"type:varchar(16);not null;default:'card_year'" json:"waiver_window"`
}

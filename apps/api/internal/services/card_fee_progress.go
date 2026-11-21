package services

import (
	"fmt"
	"time"

	"gorm.io/gorm"

	"kuberan/internal/models"
)

// FeeWaiverStatus is how a card stands against its fee-waiver condition.
type FeeWaiverStatus string

// Waiver statuses (plan 025).
const (
	// FeeWaiverMet means a waiver target has been reached.
	FeeWaiverMet FeeWaiverStatus = "met"
	// FeeWaiverOnTrack means the card is at or above 90% of the straight-line pace.
	FeeWaiverOnTrack FeeWaiverStatus = "on_track"
	// FeeWaiverBehind means the card is below 90% of the straight-line pace.
	FeeWaiverBehind FeeWaiverStatus = "behind"
)

// FeeWaiverTarget is one waiver target and how far along the card is.
// Spend is in cents of the card's currency; swipes is a count.
type FeeWaiverTarget struct {
	Target    int64 `json:"target"`
	Actual    int64 `json:"actual"`
	Remaining int64 `json:"remaining"`
}

// FeeWaiverProgress is the progress against a card's waiver condition.
type FeeWaiverProgress struct {
	Type           models.WaiverType   `json:"type"`
	Window         models.WaiverWindow `json:"window"`
	WindowStart    string              `json:"window_start"`
	WindowEnd      string              `json:"window_end"`
	Spend          *FeeWaiverTarget    `json:"spend"`
	Swipes         *FeeWaiverTarget    `json:"swipes"`
	Status         FeeWaiverStatus     `json:"status"`
	PerMonthNeeded *int64              `json:"per_month_needed"`
	DaysLeft       int                 `json:"days_left"`
}

// CardFeeInfo is a card's annual fee and when it is next charged.
type CardFeeInfo struct {
	Amount      int64  `json:"amount"`
	NextFeeDate string `json:"next_fee_date"`
	DayKnown    bool   `json:"day_known"`
}

// CardFeeSummary is the fee, the waiver progress and the trailing 12 months of
// spend, all in the card's own currency.
type CardFeeSummary struct {
	Currency          string             `json:"currency"`
	Fee               *CardFeeInfo       `json:"fee"`
	Waiver            *FeeWaiverProgress `json:"waiver"`
	SpentLast12Months int64              `json:"spent_last_12_months"`
}

const feeDateLayout = "2006-01-02"

// cardSpend sums and counts the card's expenses dated from start through end
// (inclusive UTC dates). Credits, transfers, opening balances and deleted rows
// are not purchases, and the annual fee itself is excluded by description.
// Shared expenses count in full: the bank sees the whole charge.
func cardSpend(tx *gorm.DB, userID, accountID string, start, end time.Time) (spend, swipes int64, err error) {
	var row struct {
		Spend  int64
		Swipes int64
	}
	err = tx.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0) AS spend, COUNT(*) AS swipes").
		Where("user_id = ? AND account_id = ? AND type = ?", userID, accountID, models.TransactionTypeExpense).
		Where("date >= ? AND date < ?", utcDay(start), utcDay(end).AddDate(0, 0, 1)).
		Where("LOWER(COALESCE(description, '')) NOT LIKE ? AND LOWER(COALESCE(description, '')) NOT LIKE ?",
			"%annual fee%", "%yuran tahunan%").
		Scan(&row).Error
	if err != nil {
		return 0, 0, fmt.Errorf("sum card spend: %w", err)
	}
	return row.Spend, row.Swipes, nil
}

func daysBetween(a, b time.Time) int {
	return int(utcDay(b).Sub(utcDay(a)) / (24 * time.Hour))
}

// waiverStatus rates progress against the condition: met when a target is
// reached, otherwise on_track when at least 90% of the straight-line pace, and
// behind below that. The first 30 days of a window never report behind.
func waiverStatus(fee *models.CardAnnualFee, spend, swipes int64, start, end, now time.Time) FeeWaiverStatus {
	useSpend := fee.WaiverSpend != nil && fee.WaiverType != nil && *fee.WaiverType != models.WaiverTypeSwipes
	useSwipes := fee.WaiverSwipes != nil && fee.WaiverType != nil && *fee.WaiverType != models.WaiverTypeSpend
	if useSpend && spend >= *fee.WaiverSpend {
		return FeeWaiverMet
	}
	if useSwipes && swipes >= int64(*fee.WaiverSwipes) {
		return FeeWaiverMet
	}
	elapsed := int64(daysBetween(start, now) + 1)
	if elapsed <= 30 {
		return FeeWaiverOnTrack
	}
	total := int64(daysBetween(start, end) + 1)
	onPace := func(actual, target int64) bool { return actual*total*100 >= target*elapsed*90 }
	if (useSpend && onPace(spend, *fee.WaiverSpend)) || (useSwipes && onPace(swipes, int64(*fee.WaiverSwipes))) {
		return FeeWaiverOnTrack
	}
	return FeeWaiverBehind
}

func newWaiverTarget(target, actual int64) *FeeWaiverTarget {
	remaining := target - actual
	if remaining < 0 {
		remaining = 0
	}
	return &FeeWaiverTarget{Target: target, Actual: actual, Remaining: remaining}
}

// buildCardFeeSummary computes the fee summary of a card whose AnnualFee is
// preloaded. A card without a fee gets a nil Fee and Waiver; one without a
// waiver condition gets a nil Waiver.
func buildCardFeeSummary(tx *gorm.DB, acct *models.Account, now time.Time) (*CardFeeSummary, error) {
	today := utcDay(now)
	summary := &CardFeeSummary{Currency: acct.Currency}

	spent, _, err := cardSpend(tx, acct.UserID, acct.ID, today.AddDate(-1, 0, 1), today)
	if err != nil {
		return nil, err
	}
	summary.SpentLast12Months = spent

	fee := acct.AnnualFee
	if fee == nil {
		return summary, nil
	}
	summary.Fee = &CardFeeInfo{
		Amount:      fee.Amount,
		NextFeeDate: NextFeeDate(fee.FeeMonth, fee.FeeDay, today).Format(feeDateLayout),
		DayKnown:    fee.FeeDay != nil,
	}
	if fee.WaiverType == nil {
		return summary, nil
	}

	start, end := FeeWaiverWindow(fee, today)
	spend, swipes, err := cardSpend(tx, acct.UserID, acct.ID, start, end)
	if err != nil {
		return nil, err
	}
	w := &FeeWaiverProgress{
		Type:        *fee.WaiverType,
		Window:      fee.WaiverWindow,
		WindowStart: start.Format(feeDateLayout),
		WindowEnd:   end.Format(feeDateLayout),
		Status:      waiverStatus(fee, spend, swipes, start, end, today),
		DaysLeft:    daysBetween(today, end),
	}
	if fee.WaiverSpend != nil && *fee.WaiverType != models.WaiverTypeSwipes {
		w.Spend = newWaiverTarget(*fee.WaiverSpend, spend)
	}
	if fee.WaiverSwipes != nil && *fee.WaiverType != models.WaiverTypeSpend {
		w.Swipes = newWaiverTarget(int64(*fee.WaiverSwipes), swipes)
	}
	if w.Status != FeeWaiverMet && w.Spend != nil {
		months := int64((w.DaysLeft + 29) / 30)
		if months < 1 {
			months = 1
		}
		per := (w.Spend.Remaining + months - 1) / months
		w.PerMonthNeeded = &per
	}
	summary.Waiver = w
	return summary, nil
}

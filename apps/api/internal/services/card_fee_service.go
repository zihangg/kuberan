package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// CardAnnualFeeInput is the fee a card is set to.
type CardAnnualFeeInput struct {
	Amount       int64
	Month        int
	Day          *int
	WaiverType   *models.WaiverType
	WaiverSpend  *int64
	WaiverSwipes *int
	WaiverWindow models.WaiverWindow
}

type cardFeeService struct {
	db *gorm.DB
}

// NewCardFeeService creates a CardFeeServicer.
func NewCardFeeService(db *gorm.DB) CardFeeServicer {
	return &cardFeeService{db: db}
}

func invalidFee(msg string) error {
	return apperrors.WithMessage(apperrors.ErrInvalidInput, msg)
}

func validateCardFeeInput(in *CardAnnualFeeInput) error {
	if in.Amount <= 0 {
		return invalidFee("amount must be greater than zero")
	}
	if in.Month < 1 || in.Month > 12 {
		return invalidFee("month must be between 1 and 12")
	}
	if in.Day != nil && (*in.Day < 1 || *in.Day > 31) {
		return invalidFee("day must be between 1 and 31")
	}
	switch in.WaiverWindow {
	case "":
		in.WaiverWindow = models.WaiverWindowCardYear
	case models.WaiverWindowCardYear, models.WaiverWindowCalendarYear:
	default:
		return invalidFee("waiver_window must be card_year or calendar_year")
	}
	if in.WaiverType == nil {
		if in.WaiverSpend != nil || in.WaiverSwipes != nil {
			return invalidFee("waiver_type is required when a waiver target is given")
		}
		return nil
	}
	wantSpend, wantSwipes := false, false
	switch *in.WaiverType {
	case models.WaiverTypeSpend:
		wantSpend = true
	case models.WaiverTypeSwipes:
		wantSwipes = true
	case models.WaiverTypeSpendOrSwipes:
		wantSpend, wantSwipes = true, true
	default:
		return invalidFee("waiver_type must be spend, swipes or spend_or_swipes")
	}
	if wantSpend && (in.WaiverSpend == nil || *in.WaiverSpend <= 0) {
		return invalidFee("waiver_spend must be greater than zero for this waiver type")
	}
	if wantSwipes && (in.WaiverSwipes == nil || *in.WaiverSwipes <= 0) {
		return invalidFee("waiver_swipes must be greater than zero for this waiver type")
	}
	if !wantSpend && in.WaiverSpend != nil {
		return invalidFee("waiver_spend is not allowed for this waiver type")
	}
	if !wantSwipes && in.WaiverSwipes != nil {
		return invalidFee("waiver_swipes is not allowed for this waiver type")
	}
	return nil
}

// loadCard fetches the user's credit card, or the matching AppError.
func loadCard(tx *gorm.DB, userID, accountID string) (*models.Account, error) {
	var acct models.Account
	if err := tx.Preload("AnnualFee").Where("id = ? AND user_id = ?", accountID, userID).First(&acct).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrAccountNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if acct.Type != models.AccountTypeCreditCard {
		return nil, apperrors.ErrInvalidAccountType
	}
	return &acct, nil
}

// Set implements CardFeeServicer.
func (s *cardFeeService) Set(userID, accountID string, in CardAnnualFeeInput, now time.Time) (*models.CardAnnualFee, error) {
	var out models.CardAnnualFee
	err := s.db.Transaction(func(tx *gorm.DB) error {
		acct, err := loadCard(tx, userID, accountID)
		if err != nil {
			return err
		}
		if err := validateCardFeeInput(&in); err != nil {
			return err
		}
		fee := acct.AnnualFee
		if fee == nil {
			fee = &models.CardAnnualFee{AccountID: accountID, UserID: userID}
		}
		fee.Amount = in.Amount
		fee.FeeMonth = in.Month
		fee.FeeDay = in.Day
		fee.WaiverType = in.WaiverType
		fee.WaiverSpend = in.WaiverSpend
		fee.WaiverSwipes = in.WaiverSwipes
		fee.WaiverWindow = in.WaiverWindow
		// Save writes every column, so cleared (nil) fields reach the row.
		if err := tx.Save(fee).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
		if err := syncAccountReminders(tx, accountID, now); err != nil {
			return err
		}
		out = *fee
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Delete implements CardFeeServicer.
func (s *cardFeeService) Delete(userID, accountID string, now time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if _, err := loadCard(tx, userID, accountID); err != nil {
			return err
		}
		// Hard delete: account_id is UNIQUE, so a soft-deleted row would block a re-add.
		if err := tx.Unscoped().Where("account_id = ? AND user_id = ?", accountID, userID).
			Delete(&models.CardAnnualFee{}).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
		return syncAccountReminders(tx, accountID, now)
	})
}

// Summary implements CardFeeServicer.
func (s *cardFeeService) Summary(userID, accountID string, now time.Time) (*CardFeeSummary, error) {
	acct, err := loadCard(s.db, userID, accountID)
	if err != nil {
		return nil, err
	}
	return buildCardFeeSummary(s.db, acct, now)
}

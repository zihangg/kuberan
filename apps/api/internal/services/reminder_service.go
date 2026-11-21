package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// dueReminderBatch caps how many reminders one poll of the bot returns.
const dueReminderBatch = 100

// DueReminder is a reminder ready to be sent, with what the bot needs to word
// the message. Amount is in cents of Currency, the account's own currency
// (plan 024): what is owed on a card or loan, the maturity value of a
// deposit, or, for card_annual_fee and fee_waiver_pace, the annual fee amount
// (plan 025).
type DueReminder struct {
	ID             string              `json:"id"`
	Kind           models.ReminderKind `json:"kind"`
	TelegramUserID int64               `json:"telegram_user_id"`
	AccountID      string              `json:"account_id"`
	AccountName    string              `json:"account_name"`
	DueOn          string              `json:"due_on"` // YYYY-MM-DD
	Amount         int64               `json:"amount"` // the fee amount for the two annual-fee kinds
	Currency       string              `json:"currency"`
	MonthlyPayment *int64              `json:"monthly_payment,omitempty"`
	AccountType    models.AccountType  `json:"account_type"`
	RemindAt       time.Time           `json:"remind_at"`

	// The fields below are set only for card_annual_fee and fee_waiver_pace
	// (plan 025), all in the card's currency.

	// AnnualFee is the fee in cents.
	AnnualFee *int64 `json:"annual_fee,omitempty"`
	// FeeDayKnown is false when only the fee month is known ("in March").
	FeeDayKnown *bool `json:"fee_day_known,omitempty"`
	// FeeWaiver is the waiver progress; nil when the fee cannot be waived.
	FeeWaiver *FeeWaiverProgress `json:"fee_waiver,omitempty"`
	// SpentLast12Months is the card's spend over the trailing year, in cents.
	SpentLast12Months *int64 `json:"spent_last_12_months,omitempty"`
}

// ReminderServicer defines the contract for reminder business logic.
type ReminderServicer interface {
	// ListUpcoming returns the user's reminders that are unsent or not yet due,
	// soonest first. refID, when non-empty, limits it to one account.
	ListUpcoming(userID, refID string, now time.Time) ([]models.Reminder, error)
	// SetEnabled switches an account's reminder on or off (all occurrences of
	// the reminder's kind for its account) and returns the reminder. The two
	// annual-fee kinds (card_annual_fee, fee_waiver_pace) share one switch:
	// either one switches both for the card.
	SetEnabled(userID, reminderID string, enabled bool) (*models.Reminder, error)
	// SyncAccount brings the reminders of an account in line with its due day
	// or maturity date.
	SyncAccount(accountID string, now time.Time) error
	// Due returns the reminders the bot should send now: unsent, enabled,
	// remind_at reached, for users with an active Telegram link that has the
	// reminder's kind switched on. Reminders whose due date has passed unsent
	// are dropped and the next occurrence is created. A fee_waiver_pace
	// reminder is sent only while the card is behind pace and no fee reminder
	// in the same poll covers its deadline; otherwise it is withheld and
	// checked again the next day, as is an annual-fee reminder that cannot be
	// sent (no active link, switched off, account inactive).
	Due(now time.Time) ([]DueReminder, error)
	// MarkSent records that a reminder was delivered. Idempotent: marking a
	// sent reminder again changes nothing. A recurring reminder gets its next
	// occurrence.
	MarkSent(reminderID string, now time.Time) (*models.Reminder, error)
}

type reminderService struct {
	db *gorm.DB
}

// NewReminderService creates a new ReminderServicer.
func NewReminderService(db *gorm.DB) ReminderServicer {
	return &reminderService{db: db}
}

// ListUpcoming implements ReminderServicer.
func (s *reminderService) ListUpcoming(userID, refID string, now time.Time) ([]models.Reminder, error) {
	q := s.db.Where("user_id = ? AND (sent_at IS NULL OR due_on >= ?)", userID, utcDay(now))
	if refID != "" {
		q = q.Where("ref_id = ?", refID)
	}
	var out []models.Reminder
	if err := q.Order("due_on ASC").Find(&out).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return out, nil
}

// SetEnabled implements ReminderServicer.
func (s *reminderService) SetEnabled(userID, reminderID string, enabled bool) (*models.Reminder, error) {
	var rem models.Reminder
	if err := s.db.Where("id = ? AND user_id = ?", reminderID, userID).First(&rem).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	kinds := []models.ReminderKind{rem.Kind}
	if rem.Kind == models.ReminderKindCardAnnualFee || rem.Kind == models.ReminderKindFeeWaiverPace {
		// One switch on the card covers the fee reminder and the waiver nudge.
		kinds = []models.ReminderKind{models.ReminderKindCardAnnualFee, models.ReminderKindFeeWaiverPace}
	}
	if err := s.db.Model(&models.Reminder{}).
		Where("user_id = ? AND ref_id = ? AND kind IN ?", userID, rem.RefID, kinds).
		Update("enabled", enabled).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	rem.Enabled = enabled
	return &rem, nil
}

// SyncAccount implements ReminderServicer.
func (s *reminderService) SyncAccount(accountID string, now time.Time) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		return syncAccountReminders(tx, accountID, now)
	})
}

// Due implements ReminderServicer.
func (s *reminderService) Due(now time.Time) ([]DueReminder, error) {
	if err := s.rollStale(now); err != nil {
		return nil, err
	}

	var reminders []models.Reminder
	if err := s.db.Where("sent_at IS NULL AND enabled = ? AND remind_at <= ?", true, now).
		Order("remind_at ASC").Limit(dueReminderBatch).Find(&reminders).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if len(reminders) == 0 {
		return []DueReminder{}, nil
	}

	userIDs := make([]string, 0, len(reminders))
	accountIDs := make([]string, 0, len(reminders))
	for i := range reminders {
		userIDs = append(userIDs, reminders[i].UserID)
		accountIDs = append(accountIDs, reminders[i].RefID)
	}
	var links []models.TelegramLink
	if err := s.db.Where("user_id IN ? AND is_active = ? AND telegram_user_id <> 0", userIDs, true).
		Find(&links).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	linkByUser := make(map[string]models.TelegramLink, len(links))
	for i := range links {
		linkByUser[links[i].UserID] = links[i]
	}
	var accounts []models.Account
	if err := s.db.Preload("FixedDeposit").Preload("AnnualFee").
		Where("id IN ? AND is_active = ?", accountIDs, true).Find(&accounts).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	accountByID := make(map[string]models.Account, len(accounts))
	for i := range accounts {
		accountByID[accounts[i].ID] = accounts[i]
	}

	out := make([]DueReminder, 0, len(reminders))
	var withheld []string
	for i := range reminders {
		r := &reminders[i]
		link, linked := linkByUser[r.UserID]
		acct, ok := accountByID[r.RefID]
		if !linked || !ok || !kindEnabledOnLink(&link, r.Kind) {
			if isAnnualFeeKind(r.Kind) {
				withheld = append(withheld, r.ID)
			}
			continue
		}
		item := DueReminder{
			ID:             r.ID,
			Kind:           r.Kind,
			TelegramUserID: link.TelegramUserID,
			AccountID:      acct.ID,
			AccountName:    acct.Name,
			DueOn:          utcDay(r.DueOn).Format("2006-01-02"),
			Amount:         acct.Balance,
			Currency:       acct.Currency,
			MonthlyPayment: acct.MonthlyPayment,
			AccountType:    acct.Type,
			RemindAt:       r.RemindAt,
		}
		if r.Kind == models.ReminderKindFDMaturity && acct.FixedDeposit != nil {
			item.Amount = acct.FixedDeposit.MaturityValue
			if item.Amount == 0 {
				item.Amount = acct.FixedDeposit.Principal
			}
		}
		switch r.Kind {
		case models.ReminderKindCardDue, models.ReminderKindLoanDue:
			// Nothing owed: a settled card or loan needs no nudge.
			if item.Amount <= 0 {
				continue
			}
		case models.ReminderKindCardAnnualFee, models.ReminderKindFeeWaiverPace:
			send, err := s.addCardFeeDetails(&item, &acct, r.DueOn, now)
			if err != nil {
				return nil, err
			}
			if !send {
				withheld = append(withheld, r.ID)
				continue
			}
		}
		out = append(out, item)
	}
	out, covered := withholdCoveredPace(out)
	withheld = append(withheld, covered...)
	if err := s.postponeWithheld(withheld, now); err != nil {
		return nil, err
	}
	return out, nil
}

// isAnnualFeeKind reports whether a kind is one of the two annual-fee
// reminders (plan 025).
func isAnnualFeeKind(k models.ReminderKind) bool {
	return k == models.ReminderKindCardAnnualFee || k == models.ReminderKindFeeWaiverPace
}

// withholdCoveredPace drops each fee_waiver_pace reminder whose card also has
// a card_annual_fee reminder in the same batch for the same deadline (the fee
// reminder's waiver window ends on the pace reminder's due date): the fee
// reminder already carries the shortfall. It returns the remaining reminders
// and the IDs of the dropped pace reminders.
func withholdCoveredPace(items []DueReminder) (kept []DueReminder, dropped []string) {
	feeDeadline := make(map[string]string)
	for i := range items {
		if items[i].Kind == models.ReminderKindCardAnnualFee && items[i].FeeWaiver != nil {
			feeDeadline[items[i].AccountID] = items[i].FeeWaiver.WindowEnd
		}
	}
	if len(feeDeadline) == 0 {
		return items, nil
	}
	kept = items[:0]
	for i := range items {
		it := items[i]
		if end, ok := feeDeadline[it.AccountID]; ok && it.Kind == models.ReminderKindFeeWaiverPace && end == it.DueOn {
			dropped = append(dropped, it.ID)
			continue
		}
		kept = append(kept, it)
	}
	return kept, dropped
}

// addCardFeeDetails fills in the annual-fee fields of a card_annual_fee or
// fee_waiver_pace reminder from the card's fee summary as of now. The fee
// reminder's waiver progress is over the window that decides the fee due on
// dueOn (for a calendar-year waiver, the year before the fee); the pace
// reminder's is over the current window. It reports false when the reminder
// should not be sent: the card has no fee, or (pace) it has no waiver
// condition or is not behind pace.
func (s *reminderService) addCardFeeDetails(item *DueReminder, acct *models.Account, dueOn, now time.Time) (bool, error) {
	if acct.AnnualFee == nil {
		return false, nil
	}
	window := func(fee *models.CardAnnualFee) (time.Time, time.Time) { return FeeWaiverWindow(fee, now) }
	if item.Kind == models.ReminderKindCardAnnualFee {
		window = func(fee *models.CardAnnualFee) (time.Time, time.Time) { return FeeDecidingWindow(fee, dueOn) }
	}
	summary, err := buildCardFeeSummaryIn(s.db, acct, now, window)
	if err != nil {
		return false, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if item.Kind == models.ReminderKindFeeWaiverPace &&
		(summary.Waiver == nil || summary.Waiver.Status != FeeWaiverBehind) {
		return false, nil
	}
	fee := acct.AnnualFee.Amount
	dayKnown := acct.AnnualFee.FeeDay != nil
	spent := summary.SpentLast12Months
	item.Amount = fee
	item.AnnualFee = &fee
	item.FeeDayKnown = &dayKnown
	item.FeeWaiver = summary.Waiver
	item.SpentLast12Months = &spent
	return true, nil
}

// postponeWithheld moves the annual-fee reminders (card_annual_fee and
// fee_waiver_pace) Due() read but did not return (pace on track or met or
// covered by the fee reminder, no fee or waiver, no active link, annual-fee
// reminders switched off, account inactive) to 01:00 UTC the next day, so they
// are checked once a day and never fill Due()'s batch.
func (s *reminderService) postponeWithheld(ids []string, now time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	next := utcDay(now).AddDate(0, 0, 1).Add(reminderSendHourUTC * time.Hour)
	if err := s.db.Model(&models.Reminder{}).
		Where("id IN ? AND sent_at IS NULL", ids).
		Update("remind_at", next).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// kindEnabledOnLink reports whether the user switched a reminder kind on.
func kindEnabledOnLink(l *models.TelegramLink, k models.ReminderKind) bool {
	switch k {
	case models.ReminderKindCardDue:
		return l.RemindCardDue
	case models.ReminderKindLoanDue:
		return l.RemindLoanDue
	case models.ReminderKindFDMaturity:
		return l.RemindFDMaturity
	case models.ReminderKindCardAnnualFee, models.ReminderKindFeeWaiverPace:
		return l.RemindAnnualFee
	default:
		return false
	}
}

// rollStale drops unsent reminders whose due date has passed (the bot was
// down, or the user had them switched off) and re-syncs those accounts so a
// recurring reminder moves on to its next occurrence.
func (s *reminderService) rollStale(now time.Time) error {
	var refs []string
	if err := s.db.Model(&models.Reminder{}).
		Where("sent_at IS NULL AND due_on < ?", utcDay(now)).
		Distinct().Pluck("ref_id", &refs).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	for _, ref := range refs {
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().
				Where("ref_id = ? AND sent_at IS NULL AND due_on < ?", ref, utcDay(now)).
				Delete(&models.Reminder{}).Error; err != nil {
				return apperrors.Wrap(apperrors.ErrInternalServer, err)
			}
			return syncAccountReminders(tx, ref, now)
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// MarkSent implements ReminderServicer.
func (s *reminderService) MarkSent(reminderID string, now time.Time) (*models.Reminder, error) {
	var rem models.Reminder
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", reminderID).First(&rem).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return apperrors.ErrNotFound
			}
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
		if rem.SentAt != nil {
			return nil // already confirmed
		}
		res := tx.Model(&models.Reminder{}).
			Where("id = ? AND sent_at IS NULL", reminderID).
			Update("sent_at", now)
		if res.Error != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, res.Error)
		}
		if res.RowsAffected == 0 {
			// Lost a race with another confirmation: same outcome.
			return tx.Where("id = ?", reminderID).First(&rem).Error
		}
		rem.SentAt = &now
		if isRecurringReminder(rem.Kind) {
			return syncAccountReminders(tx, rem.RefID, now)
		}
		return nil
	})
	if err != nil {
		var appErr *apperrors.AppError
		if errors.As(err, &appErr) {
			return nil, err
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &rem, nil
}

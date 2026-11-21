package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
)

// Reminder lead times (plan 023, 5.6): how long before the due date the
// reminder becomes sendable, and the hour (UTC) it is sent at (09:00 in
// Malaysia, the app's home market).
const (
	cardReminderLeadDays = 3
	loanReminderLeadDays = 3
	fdReminderLeadDays   = 7
	reminderSendHourUTC  = 1
)

// NextDueDate returns the first date on or after from (compared by UTC date)
// that falls on dueDay (1-31) of its month. Months with fewer days use their
// last day, so dueDay 31 gives Feb 28/29, Apr 30 and so on.
func NextDueDate(dueDay int, from time.Time) time.Time {
	from = utcDay(from)
	for i := 0; i < 2; i++ {
		first := time.Date(from.Year(), from.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		last := time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		day := dueDay
		if day > last {
			day = last
		}
		candidate := time.Date(first.Year(), first.Month(), day, 0, 0, 0, 0, time.UTC)
		if !candidate.Before(from) {
			return candidate
		}
	}
	// Unreachable: the second month's candidate is always after from.
	return from
}

// reminderRemindAt is when the reminder for a due date becomes sendable.
func reminderRemindAt(dueOn time.Time, leadDays int) time.Time {
	d := utcDay(dueOn).AddDate(0, 0, -leadDays)
	return d.Add(reminderSendHourUTC * time.Hour)
}

// isRecurringReminder reports whether a kind repeats every month (the next
// occurrence is created once the current one has been sent).
func isRecurringReminder(k models.ReminderKind) bool {
	return k == models.ReminderKindCardDue || k == models.ReminderKindLoanDue
}

// reminderPlan is one reminder an account wants: the kind and the first due
// date that still needs a reminder for that kind.
type reminderPlan struct {
	kind     models.ReminderKind
	dueOn    time.Time
	leadDays int
}

// reminderPlanFrom is the first day a kind's plan may fall on: today, or the
// day after the due date of that kind's latest sent occurrence (after), so a
// sent occurrence is not repeated.
func reminderPlanFrom(now, after time.Time) time.Time {
	from := utcDay(now)
	if !after.IsZero() && !utcDay(after).Before(from) {
		from = utcDay(after).AddDate(0, 0, 1)
	}
	return from
}

// planAccountReminders works out the reminders an account should have as of
// now, at most one per kind. sentAfter holds, per kind, the due date of the
// latest reminder of that kind already sent: that kind's plan then starts the
// day after it.
func planAccountReminders(acct *models.Account, fd *models.FixedDepositDetails, now time.Time, sentAfter map[models.ReminderKind]time.Time) []reminderPlan {
	if !acct.IsActive {
		return nil
	}
	var plans []reminderPlan
	switch acct.Type {
	case models.AccountTypeCreditCard, models.AccountTypeDebt:
		if acct.DueDay == nil || *acct.DueDay < 1 || *acct.DueDay > 31 {
			break
		}
		kind, lead := models.ReminderKindCardDue, cardReminderLeadDays
		if acct.Type == models.AccountTypeDebt {
			kind, lead = models.ReminderKindLoanDue, loanReminderLeadDays
		}
		from := reminderPlanFrom(now, sentAfter[kind])
		plans = append(plans, reminderPlan{kind: kind, dueOn: NextDueDate(*acct.DueDay, from), leadDays: lead})
	case models.AccountTypeFixedDeposit:
		if fd == nil || fd.Status != models.FixedDepositStatusActive {
			break
		}
		kind := models.ReminderKindFDMaturity
		due := utcDay(fd.MaturityDate)
		if due.Before(reminderPlanFrom(now, sentAfter[kind])) {
			break
		}
		plans = append(plans, reminderPlan{kind: kind, dueOn: due, leadDays: fdReminderLeadDays})
	}
	return plans
}

// syncAccountReminders brings the reminders of one account in line with its
// due_day / maturity date, inside tx, kind by kind. For each planned kind it
// keeps the matching unsent reminder, removes that kind's other unsent ones
// (due day changed) and creates the missing one; unsent reminders of kinds no
// longer planned (account deactivated, due day cleared, deposit closed) are
// removed. A reminder switched off for the account stays off across
// occurrences of its kind. Idempotent.
func syncAccountReminders(tx *gorm.DB, accountID string, now time.Time) error {
	var acct models.Account
	err := tx.Preload("FixedDeposit").Where("id = ?", accountID).First(&acct).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return deleteUnsentReminders(tx, accountID, nil)
	}
	if err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	// Per kind: the latest occurrence (for the on/off switch, which carries
	// over) and the latest one already sent (the plan starts after it).
	var rows []models.Reminder
	if err := tx.Where("ref_id = ? AND user_id = ?", accountID, acct.UserID).
		Select("kind", "due_on", "enabled", "sent_at").Order("due_on DESC").Find(&rows).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	enabledByKind := make(map[models.ReminderKind]bool)
	sentAfter := make(map[models.ReminderKind]time.Time)
	for i := range rows {
		r := &rows[i]
		if _, seen := enabledByKind[r.Kind]; !seen {
			enabledByKind[r.Kind] = r.Enabled
		}
		if _, seen := sentAfter[r.Kind]; !seen && r.SentAt != nil {
			sentAfter[r.Kind] = r.DueOn
		}
	}

	plans := planAccountReminders(&acct, acct.FixedDeposit, now, sentAfter)
	planned := make([]models.ReminderKind, 0, len(plans))
	for _, plan := range plans {
		planned = append(planned, plan.kind)
		enabled, seen := enabledByKind[plan.kind]
		if !seen {
			enabled = true
		}
		if err := syncPlannedReminder(tx, &acct, plan, enabled); err != nil {
			return err
		}
	}
	return deleteUnsentReminders(tx, accountID, planned)
}

// syncPlannedReminder reconciles one kind of an account's reminders with its
// plan: other unsent occurrences of that kind are hard-deleted and the planned
// one is created, with the given on/off state, unless it already exists.
func syncPlannedReminder(tx *gorm.DB, acct *models.Account, plan reminderPlan, enabled bool) error {
	if err := tx.Unscoped().
		Where("ref_id = ? AND kind = ? AND sent_at IS NULL AND due_on <> ?", acct.ID, plan.kind, plan.dueOn).
		Delete(&models.Reminder{}).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	var existing int64
	if err := tx.Model(&models.Reminder{}).
		Where("kind = ? AND ref_id = ? AND due_on = ?", plan.kind, acct.ID, plan.dueOn).
		Count(&existing).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if existing > 0 {
		return nil
	}
	rem := &models.Reminder{
		UserID:   acct.UserID,
		Kind:     plan.kind,
		RefID:    acct.ID,
		DueOn:    plan.dueOn,
		RemindAt: reminderRemindAt(plan.dueOn, plan.leadDays),
		Enabled:  enabled,
	}
	if err := tx.Create(rem).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	if !enabled {
		// GORM writes the column default (true) for a zero-value false on insert.
		if err := tx.Model(rem).Update("enabled", false).Error; err != nil {
			return apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
	}
	return nil
}

// deleteUnsentReminders hard-deletes an account's unsent reminders of every
// kind except keepKinds (nil or empty keeps none). They are hard deleted so
// the unique (kind, ref_id, due_on) index never blocks recreating the same
// occurrence later.
func deleteUnsentReminders(tx *gorm.DB, accountID string, keepKinds []models.ReminderKind) error {
	q := tx.Unscoped().Where("ref_id = ? AND sent_at IS NULL", accountID)
	if len(keepKinds) > 0 {
		q = q.Where("kind NOT IN ?", keepKinds)
	}
	if err := q.Delete(&models.Reminder{}).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

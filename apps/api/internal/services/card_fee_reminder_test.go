package services

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"kuberan/internal/models"
	"kuberan/internal/testutil"
)

// feeEnv is a user with an active Telegram link and one credit card, plus the
// services the annual-fee reminder tests drive.
type feeEnv struct {
	db    *gorm.DB
	svc   ReminderServicer
	fees  CardFeeServicer
	accts AccountServicer
	user  *models.User
	card  *models.Account
}

func feeFixture(t *testing.T, dueDay int, balance int64) feeEnv {
	t.Helper()
	db := testutil.SetupTestDB(t)
	t.Cleanup(func() { testutil.TeardownTestDB(t, db) })
	env := feeEnv{db: db, svc: NewReminderService(db), fees: NewCardFeeService(db), accts: NewAccountService(db)}
	env.user = testutil.CreateTestUser(t, db)
	testutil.CreateTestTelegramLink(t, db, env.user.ID, 777)
	var err error
	env.card, err = env.accts.CreateCreditCardAccountWithParams(env.user.ID, CreateCreditCardParams{
		Name: "Visa Infinite", Currency: "MYR", CurrentBalance: balance, DueDay: dueDay,
	})
	testutil.AssertNoError(t, err)
	return env
}

func intp(v int) *int                            { return &v }
func i64p(v int64) *int64                        { return &v }
func wtp(v models.WaiverType) *models.WaiverType { return &v }

// spendWaiver is a 250.00 fee on 15 March with a 12,000.00 spend waiver.
func spendWaiver(window models.WaiverWindow) CardAnnualFeeInput {
	return CardAnnualFeeInput{
		Amount: 250_00, Month: 3, Day: intp(15),
		WaiverType: wtp(models.WaiverTypeSpend), WaiverSpend: i64p(12_000_00), WaiverWindow: window,
	}
}

// reminderRows returns the card's reminders of a kind, soonest first.
func (env feeEnv) reminderRows(t *testing.T, kind models.ReminderKind) []models.Reminder {
	t.Helper()
	var rows []models.Reminder
	testutil.AssertNoError(t, env.db.Where("ref_id = ? AND kind = ?", env.card.ID, kind).Order("due_on ASC").Find(&rows).Error)
	return rows
}

// unsent returns the single unsent reminder of a kind, failing otherwise.
func (env feeEnv) unsent(t *testing.T, kind models.ReminderKind) models.Reminder {
	t.Helper()
	var rows []models.Reminder
	testutil.AssertNoError(t, env.db.Where("ref_id = ? AND kind = ? AND sent_at IS NULL", env.card.ID, kind).Find(&rows).Error)
	if len(rows) != 1 {
		t.Fatalf("expected one unsent %s reminder, got %+v", kind, rows)
	}
	return rows[0]
}

func dueOfKind(due []DueReminder, kind models.ReminderKind) *DueReminder {
	for i := range due {
		if due[i].Kind == kind {
			return &due[i]
		}
	}
	return nil
}

func TestCardFeeReminders_CoexistWithCardDue(t *testing.T) {
	env := feeFixture(t, 5, 100_00)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
	testutil.AssertNoError(t, err)

	due := env.unsent(t, models.ReminderKindCardDue)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(fee.DueOn).Equal(utcYMD(2027, 3, 15)) {
		t.Fatalf("fee reminder due %v, want 2027-03-15", fee.DueOn)
	}
	if len(env.reminderRows(t, models.ReminderKindFeeWaiverPace)) != 0 {
		t.Fatal("no waiver condition, so no pace reminder")
	}

	testutil.AssertNoError(t, env.svc.SyncAccount(env.card.ID, now))
	if got := env.unsent(t, models.ReminderKindCardDue); got.ID != due.ID {
		t.Fatalf("re-sync replaced the card_due reminder: %+v", got)
	}
	if got := env.unsent(t, models.ReminderKindCardAnnualFee); got.ID != fee.ID {
		t.Fatalf("re-sync replaced the annual fee reminder: %+v", got)
	}
}

func TestCardFeeReminders_RemindAt(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)

	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	if want := time.Date(2027, 2, 13, 1, 0, 0, 0, time.UTC); !fee.RemindAt.Equal(want) {
		t.Fatalf("fee remind_at %v, want %v", fee.RemindAt, want)
	}
	pace := env.unsent(t, models.ReminderKindFeeWaiverPace)
	_, end := FeeWaiverWindow(&models.CardAnnualFee{FeeMonth: 3, FeeDay: intp(15), WaiverWindow: models.WaiverWindowCardYear}, now)
	if !utcDay(pace.DueOn).Equal(end) || !end.Equal(utcYMD(2027, 3, 15)) {
		t.Fatalf("pace due %v, want window end %v", pace.DueOn, end)
	}
	if want := end.AddDate(0, 0, -90).Add(time.Hour); !pace.RemindAt.Equal(want) {
		t.Fatalf("pace remind_at %v, want %v", pace.RemindAt, want)
	}
}

func TestCardFeeReminders_YearlyAfterSent(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)},
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)

	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	_, err = env.svc.MarkSent(fee.ID, time.Date(2027, 2, 13, 1, 5, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)

	next := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(next.DueOn).Equal(utcYMD(2028, 3, 15)) {
		t.Fatalf("next fee reminder due %v, want 2028-03-15", next.DueOn)
	}
}

func TestCardFeeReminders_ZeroBalanceStillDue(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)},
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)

	got, err := env.svc.Due(time.Date(2027, 2, 14, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	d := dueOfKind(got, models.ReminderKindCardAnnualFee)
	if d == nil {
		t.Fatalf("a card with nothing owed still gets the fee reminder: %+v", got)
	}
	if d.AnnualFee == nil || *d.AnnualFee != 250_00 || d.FeeDayKnown == nil || !*d.FeeDayKnown {
		t.Fatalf("fee fields missing: %+v", d)
	}
	if d.SpentLast12Months == nil || *d.SpentLast12Months != 0 || d.FeeWaiver != nil {
		t.Fatalf("expected spend 0 and no waiver: %+v", d)
	}
	if d.DueOn != "2027-03-15" || d.Currency != "MYR" || d.TelegramUserID != 777 {
		t.Fatalf("unexpected due reminder: %+v", d)
	}
}

func TestCardFeeReminders_PaceWithheldWhenOnTrack(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear),
		time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	addCardTx(t, env.db, env.user.ID, env.card.ID, models.TransactionTypeExpense, 5_000_00, "flights", utcYMD(2026, 4, 2))
	addCardTx(t, env.db, env.user.ID, env.card.ID, models.TransactionTypeExpense, 4_000_00, "laptop", utcYMD(2026, 9, 20))

	pace := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if !utcDay(pace.DueOn).Equal(utcYMD(2026, 12, 31)) {
		t.Fatalf("pace due %v, want 2026-12-31", pace.DueOn)
	}

	got, err := env.svc.Due(time.Date(2026, 10, 15, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if d := dueOfKind(got, models.ReminderKindFeeWaiverPace); d != nil {
		t.Fatalf("on track: the pace reminder must be withheld, got %+v", d)
	}
	pace = env.unsent(t, models.ReminderKindFeeWaiverPace)
	if want := time.Date(2026, 10, 16, 1, 0, 0, 0, time.UTC); !pace.RemindAt.Equal(want) {
		t.Fatalf("withheld pace remind_at %v, want %v", pace.RemindAt, want)
	}
	// Later the same day it is not even considered.
	got, err = env.svc.Due(time.Date(2026, 10, 15, 23, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if len(got) != 0 {
		t.Fatalf("nothing due before the next check: %+v", got)
	}

	got, err = env.svc.Due(time.Date(2026, 11, 30, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	d := dueOfKind(got, models.ReminderKindFeeWaiverPace)
	if d == nil {
		t.Fatalf("behind: the pace reminder must be sent, got %+v", got)
	}
	if d.FeeWaiver == nil || d.FeeWaiver.Status != FeeWaiverBehind || d.FeeWaiver.Spend.Actual != 9_000_00 {
		t.Fatalf("unexpected waiver progress: %+v", d.FeeWaiver)
	}
	if d.DueOn != "2026-12-31" || d.AnnualFee == nil || *d.AnnualFee != 250_00 || d.SpentLast12Months == nil {
		t.Fatalf("unexpected pace payload: %+v", d)
	}

	_, err = env.svc.MarkSent(d.ID, time.Date(2026, 11, 30, 2, 1, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if !utcDay(next.DueOn).Equal(utcYMD(2027, 12, 31)) {
		t.Fatalf("next pace reminder due %v, want 2027-12-31", next.DueOn)
	}
	got, err = env.svc.Due(time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if d := dueOfKind(got, models.ReminderKindFeeWaiverPace); d != nil {
		t.Fatalf("at most one pace reminder per window: %+v", d)
	}
}

func TestCardFeeReminders_TelegramSwitch(t *testing.T) {
	env := feeFixture(t, 5, 100_00)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)

	got, err := env.svc.Due(now)
	testutil.AssertNoError(t, err)
	// The pace nudge shares the fee reminder's deadline, so only the fee
	// reminder goes out for it (F3).
	if len(got) != 2 || dueOfKind(got, models.ReminderKindCardDue) == nil || dueOfKind(got, models.ReminderKindCardAnnualFee) == nil {
		t.Fatalf("expected card_due and card_annual_fee due, got %+v", got)
	}
	testutil.AssertNoError(t, env.db.Model(&models.Reminder{}).
		Where("ref_id = ? AND kind = ?", env.card.ID, models.ReminderKindFeeWaiverPace).
		Update("remind_at", now.Add(-time.Hour)).Error)

	off := false
	_, err = NewTelegramService(env.db).UpdateReminderSettings(env.user.ID, ReminderSettings{AnnualFee: &off})
	testutil.AssertNoError(t, err)
	got, err = env.svc.Due(now)
	testutil.AssertNoError(t, err)
	if len(got) != 1 || got[0].Kind != models.ReminderKindCardDue {
		t.Fatalf("annual fee reminders are off; only card_due is due, got %+v", got)
	}
	if got[0].AnnualFee != nil || got[0].FeeWaiver != nil || got[0].SpentLast12Months != nil || got[0].FeeDayKnown != nil {
		t.Fatalf("card_due must not carry fee fields: %+v", got[0])
	}
	// Both withheld annual-fee rows move to the next day, so they never fill
	// the batch (F4).
	nextDay := time.Date(2027, 3, 4, 1, 0, 0, 0, time.UTC)
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); !pace.RemindAt.Equal(nextDay) {
		t.Fatalf("withheld pace remind_at %v, want 2027-03-04 01:00", pace.RemindAt)
	}
	if fee := env.unsent(t, models.ReminderKindCardAnnualFee); !fee.RemindAt.Equal(nextDay) {
		t.Fatalf("withheld fee remind_at %v, want 2027-03-04 01:00", fee.RemindAt)
	}
}

// A pace row for a card that has been deactivated (read by Due but not
// returned) is also moved to the next day.
func TestCardFeeReminders_PaceInactiveAccountRescheduled(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)
	// Flip is_active directly, bypassing the sync that would delete the rows.
	testutil.AssertNoError(t, env.db.Model(&models.Account{}).Where("id = ?", env.card.ID).Update("is_active", false).Error)

	got, err := env.svc.Due(now)
	testutil.AssertNoError(t, err)
	if len(got) != 0 {
		t.Fatalf("an inactive card gets no reminders: %+v", got)
	}
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); !pace.RemindAt.Equal(time.Date(2027, 3, 4, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("pace remind_at %v, want 2027-03-04 01:00", pace.RemindAt)
	}
}

// A sent fee reminder covers its fee year: moving the date later in the same
// year plans next year's reminder, not a second one this year.
func TestCardFeeReminders_MoveDateLaterAfterSent(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)},
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	sent := env.unsent(t, models.ReminderKindCardAnnualFee)
	_, err = env.svc.MarkSent(sent.ID, time.Date(2027, 2, 14, 1, 5, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)

	_, err = env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(25)},
		time.Date(2027, 2, 20, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(next.DueOn).Equal(utcYMD(2028, 3, 25)) {
		t.Fatalf("next fee reminder %v, want 2028-03-25", next.DueOn)
	}
	if rows := env.reminderRows(t, models.ReminderKindCardAnnualFee); len(rows) != 2 || rows[0].ID != sent.ID {
		t.Fatalf("expected the sent 2027 reminder plus the 2028 one, got %+v", rows)
	}
}

// Two kinds on one card keep their own on/off state and sent dates.
func TestCardFeeReminders_KindsAreIndependent(t *testing.T) {
	env := feeFixture(t, 5, 100_00)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
	testutil.AssertNoError(t, err)

	due := env.unsent(t, models.ReminderKindCardDue)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)

	// Switching card_due off leaves the fee reminder on, across a re-sync.
	_, err = env.svc.SetEnabled(env.user.ID, due.ID, false)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, env.svc.SyncAccount(env.card.ID, now))
	if !env.unsent(t, models.ReminderKindCardAnnualFee).Enabled || env.unsent(t, models.ReminderKindCardDue).Enabled {
		t.Fatal("switching card_due off must not switch the fee reminder off")
	}
	_, err = env.svc.SetEnabled(env.user.ID, due.ID, true)
	testutil.AssertNoError(t, err)

	// And the other way round.
	_, err = env.svc.SetEnabled(env.user.ID, fee.ID, false)
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, env.svc.SyncAccount(env.card.ID, now))
	if env.unsent(t, models.ReminderKindCardAnnualFee).Enabled || !env.unsent(t, models.ReminderKindCardDue).Enabled {
		t.Fatal("switching the fee reminder off must not switch card_due off")
	}
	_, err = env.svc.SetEnabled(env.user.ID, fee.ID, true)
	testutil.AssertNoError(t, err)

	// Sending card_due moves only card_due on.
	_, err = env.svc.MarkSent(due.ID, now)
	testutil.AssertNoError(t, err)
	if got := env.unsent(t, models.ReminderKindCardAnnualFee); got.ID != fee.ID || !utcDay(got.DueOn).Equal(utcYMD(2027, 3, 15)) {
		t.Fatalf("sending card_due moved the fee reminder: %+v", got)
	}
	nextDue := env.unsent(t, models.ReminderKindCardDue)
	if !utcDay(nextDue.DueOn).Equal(utcYMD(2027, 4, 5)) {
		t.Fatalf("next card_due %v, want 2027-04-05", nextDue.DueOn)
	}

	// Sending the fee reminder moves only the fee reminder on.
	_, err = env.svc.MarkSent(fee.ID, now)
	testutil.AssertNoError(t, err)
	if got := env.unsent(t, models.ReminderKindCardDue); got.ID != nextDue.ID {
		t.Fatalf("sending the fee reminder moved card_due: %+v", got)
	}
	if got := env.unsent(t, models.ReminderKindCardAnnualFee); !utcDay(got.DueOn).Equal(utcYMD(2028, 3, 15)) {
		t.Fatalf("next fee reminder %v, want 2028-03-15", got.DueOn)
	}
}

// Review Focus 1: a deactivated card plans no fee or pace reminders.
func TestCardFeeReminders_DeactivatedCard(t *testing.T) {
	env := feeFixture(t, 5, 100_00)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), time.Now())
	testutil.AssertNoError(t, err)
	if len(env.reminderRows(t, models.ReminderKindCardAnnualFee)) != 1 || len(env.reminderRows(t, models.ReminderKindFeeWaiverPace)) != 1 {
		t.Fatal("expected a fee and a pace reminder before deactivating")
	}

	off := false
	_, err = env.accts.UpdateAccount(env.user.ID, env.card.ID, AccountUpdateFields{IsActive: &off})
	testutil.AssertNoError(t, err)
	var n int64
	testutil.AssertNoError(t, env.db.Model(&models.Reminder{}).Where("ref_id = ?", env.card.ID).Count(&n).Error)
	if n != 0 {
		t.Fatalf("expected no reminders for a deactivated card, got %d", n)
	}
	// Editing the fee of the inactive card plans nothing either.
	_, err = env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear), time.Now())
	testutil.AssertNoError(t, err)
	testutil.AssertNoError(t, env.db.Model(&models.Reminder{}).Where("ref_id = ?", env.card.ID).Count(&n).Error)
	if n != 0 {
		t.Fatalf("an inactive card must stay without reminders, got %d", n)
	}
}

// Review Focus 4: removing the waiver drops the pending pace reminder only.
func TestCardFeeReminders_RemoveWaiverKeepsFeeReminder(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	env.unsent(t, models.ReminderKindFeeWaiverPace)

	_, err = env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
	testutil.AssertNoError(t, err)
	var n int64
	testutil.AssertNoError(t, env.db.Unscoped().Model(&models.Reminder{}).
		Where("ref_id = ? AND kind = ?", env.card.ID, models.ReminderKindFeeWaiverPace).Count(&n).Error)
	if n != 0 {
		t.Fatalf("the pace reminder must be deleted with the waiver, got %d", n)
	}
	if got := env.unsent(t, models.ReminderKindCardAnnualFee); got.ID != fee.ID {
		t.Fatalf("the fee reminder must stay as it was: %+v", got)
	}

	// Removing the fee removes its reminder too.
	testutil.AssertNoError(t, env.fees.Delete(env.user.ID, env.card.ID, now))
	if rows := env.reminderRows(t, models.ReminderKindCardAnnualFee); len(rows) != 0 {
		t.Fatalf("removing the fee must remove its reminder: %+v", rows)
	}
}

// Review Focus 5: a fee date edited after this year's reminder was sent.
func TestCardFeeReminders_EditDateAfterSent(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)},
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)

	// Before it is sent, an edit moves the pending reminder to the new date.
	_, err = env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(20)},
		time.Date(2026, 12, 1, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	pending := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(pending.DueOn).Equal(utcYMD(2027, 3, 20)) {
		t.Fatalf("pending fee reminder %v, want 2027-03-20", pending.DueOn)
	}

	sentAt := time.Date(2027, 2, 18, 1, 5, 0, 0, time.UTC)
	_, err = env.svc.MarkSent(pending.ID, sentAt)
	testutil.AssertNoError(t, err)

	// The user corrects the date to 1 March: next year's reminder follows it,
	// and nothing is planned again for this year's fee.
	_, err = env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(1)},
		time.Date(2027, 2, 25, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(next.DueOn).Equal(utcYMD(2028, 3, 1)) {
		t.Fatalf("next fee reminder %v, want 2028-03-01", next.DueOn)
	}
	rows := env.reminderRows(t, models.ReminderKindCardAnnualFee)
	if len(rows) != 2 || rows[0].ID != pending.ID || rows[0].SentAt == nil || !rows[0].SentAt.Equal(sentAt) {
		t.Fatalf("expected the sent 2027 reminder untouched plus the 2028 one, got %+v", rows)
	}
	got, err := env.svc.Due(time.Date(2027, 2, 26, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if d := dueOfKind(got, models.ReminderKindCardAnnualFee); d != nil {
		t.Fatalf("this year's fee reminder must not be sent again: %+v", d)
	}
}

// F1: a calendar-year waiver decides the fee charged the following year, so
// the fee reminder for 15 Mar 2027 is judged on 2026's spend, not 2027's.
func TestCardFeeReminders_CalendarYearFeeUsesPreviousYear(t *testing.T) {
	tests := []struct {
		name       string
		spend      int64
		wantStatus FeeWaiverStatus
		wantRemain int64
	}{
		{"met in 2026", 12_500_00, FeeWaiverMet, 0},
		{"missed in 2026", 8_420_00, FeeWaiverBehind, 3_580_00},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := feeFixture(t, 0, 0)
			_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear),
				time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
			testutil.AssertNoError(t, err)
			addCardTx(t, env.db, env.user.ID, env.card.ID, models.TransactionTypeExpense, tt.spend, "flights", utcYMD(2026, 6, 2))
			// 2027 spend must not count towards the 2027 fee.
			addCardTx(t, env.db, env.user.ID, env.card.ID, models.TransactionTypeExpense, 20_000_00, "car", utcYMD(2027, 1, 10))

			got, err := env.svc.Due(time.Date(2027, 2, 14, 2, 0, 0, 0, time.UTC))
			testutil.AssertNoError(t, err)
			d := dueOfKind(got, models.ReminderKindCardAnnualFee)
			if d == nil || d.FeeWaiver == nil {
				t.Fatalf("expected the fee reminder with waiver progress, got %+v", got)
			}
			w := d.FeeWaiver
			if w.WindowStart != "2026-01-01" || w.WindowEnd != "2026-12-31" {
				t.Fatalf("window %s..%s, want 2026-01-01..2026-12-31", w.WindowStart, w.WindowEnd)
			}
			if w.Status != tt.wantStatus || w.Spend == nil || w.Spend.Actual != tt.spend || w.Spend.Remaining != tt.wantRemain {
				t.Fatalf("unexpected progress: %+v (spend %+v)", w, w.Spend)
			}
			if w.DaysLeft != 0 || w.PerMonthNeeded != nil {
				t.Fatalf("a closed window has no days left and no per-month figure: %+v", w)
			}
		})
	}
}

// F2: the card's one fee switch covers both annual-fee reminders, across
// occurrences, and leaves card_due alone.
func TestCardFeeReminders_SwitchCoversFeeAndPace(t *testing.T) {
	env := feeFixture(t, 5, 100_00)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear), now)
	testutil.AssertNoError(t, err)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)

	_, err = env.svc.SetEnabled(env.user.ID, fee.ID, false)
	testutil.AssertNoError(t, err)
	pace := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if pace.Enabled {
		t.Fatal("switching the fee reminder off must switch the waiver nudge off too")
	}
	if !env.unsent(t, models.ReminderKindCardDue).Enabled {
		t.Fatal("card_due must stay on")
	}

	// The pace reminder's next occurrence stays off.
	_, err = env.svc.MarkSent(pace.ID, time.Date(2026, 12, 1, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if next.ID == pace.ID || next.Enabled {
		t.Fatalf("the next waiver nudge must stay off: %+v", next)
	}

	// Switching the pace row back on switches the fee reminder on too.
	_, err = env.svc.SetEnabled(env.user.ID, next.ID, true)
	testutil.AssertNoError(t, err)
	if !env.unsent(t, models.ReminderKindCardAnnualFee).Enabled || !env.unsent(t, models.ReminderKindFeeWaiverPace).Enabled {
		t.Fatal("switching back on must cover both annual-fee kinds")
	}
}

// F3: when the fee reminder goes out in the same poll and covers the same
// deadline, the pace reminder is withheld (the fee reminder already carries
// the shortfall) and checked again the next day.
func TestCardFeeReminders_PaceWithheldWhenFeeReminderCoversDeadline(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)

	got, err := env.svc.Due(now)
	testutil.AssertNoError(t, err)
	if len(got) != 1 || got[0].Kind != models.ReminderKindCardAnnualFee {
		t.Fatalf("only the fee reminder goes out for the shared deadline, got %+v", got)
	}
	if got[0].FeeWaiver == nil || got[0].FeeWaiver.Status != FeeWaiverBehind {
		t.Fatalf("the fee reminder carries the shortfall: %+v", got[0].FeeWaiver)
	}
	pace := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if !pace.RemindAt.Equal(time.Date(2027, 3, 4, 1, 0, 0, 0, time.UTC)) {
		t.Fatalf("withheld pace remind_at %v, want 2027-03-04 01:00", pace.RemindAt)
	}
}

// F3: a pace reminder for a different deadline than the fee reminder's still
// goes out in the same poll.
func TestCardFeeReminders_PaceSentWithFeeReminderForOtherDeadline(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear),
		time.Date(2027, 1, 20, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	// The pace row (31 Dec 2027) is not normally sendable in February; bring it
	// into the fee reminder's poll by hand.
	testutil.AssertNoError(t, env.db.Model(&models.Reminder{}).
		Where("ref_id = ? AND kind = ?", env.card.ID, models.ReminderKindFeeWaiverPace).
		Update("remind_at", time.Date(2027, 1, 1, 1, 0, 0, 0, time.UTC)).Error)
	// The fee due 15 Mar 2027 is decided by 2026; the pace row is for 2027.
	got, err := env.svc.Due(time.Date(2027, 2, 20, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if dueOfKind(got, models.ReminderKindCardAnnualFee) == nil {
		t.Fatalf("expected the fee reminder, got %+v", got)
	}
	if d := dueOfKind(got, models.ReminderKindFeeWaiverPace); d == nil || d.DueOn != "2027-12-31" {
		t.Fatalf("the pace reminder for 2027 is a different deadline and goes out too, got %+v", got)
	}
}

// F4: a fee reminder Due() reads but does not return is moved to the next day
// like a withheld pace row, so it never fills the batch.
func TestCardFeeReminders_WithheldFeeReminderPostponed(t *testing.T) {
	t.Run("no telegram link", func(t *testing.T) {
		env := feeFixture(t, 0, 0)
		now := time.Date(2027, 2, 20, 2, 0, 0, 0, time.UTC)
		_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
		testutil.AssertNoError(t, err)
		testutil.AssertNoError(t, env.db.Model(&models.TelegramLink{}).Where("user_id = ?", env.user.ID).
			Update("is_active", false).Error)

		got, err := env.svc.Due(now)
		testutil.AssertNoError(t, err)
		if len(got) != 0 {
			t.Fatalf("no active link, nothing to send: %+v", got)
		}
		if fee := env.unsent(t, models.ReminderKindCardAnnualFee); !fee.RemindAt.Equal(time.Date(2027, 2, 21, 1, 0, 0, 0, time.UTC)) {
			t.Fatalf("withheld fee remind_at %v, want 2027-02-21 01:00", fee.RemindAt)
		}
	})
	t.Run("inactive account", func(t *testing.T) {
		env := feeFixture(t, 0, 0)
		now := time.Date(2027, 2, 20, 2, 0, 0, 0, time.UTC)
		_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
		testutil.AssertNoError(t, err)
		testutil.AssertNoError(t, env.db.Model(&models.Account{}).Where("id = ?", env.card.ID).Update("is_active", false).Error)

		got, err := env.svc.Due(now)
		testutil.AssertNoError(t, err)
		if len(got) != 0 {
			t.Fatalf("an inactive card gets no reminders: %+v", got)
		}
		if fee := env.unsent(t, models.ReminderKindCardAnnualFee); !fee.RemindAt.Equal(time.Date(2027, 2, 21, 1, 0, 0, 0, time.UTC)) {
			t.Fatalf("withheld fee remind_at %v, want 2027-02-21 01:00", fee.RemindAt)
		}
	})
}

// R1: a sent fee reminder covers the pace nudge for the same deadline, also
// across polls: once the fee reminder has gone out, the next day's poll does
// not send the covered nudge, and the pace row moves on to the next window.
func TestCardFeeReminders_SentFeeReminderCoversPaceNextDay(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)

	got, err := env.svc.Due(now)
	testutil.AssertNoError(t, err)
	if len(got) != 1 || got[0].Kind != models.ReminderKindCardAnnualFee {
		t.Fatalf("expected only the fee reminder, got %+v", got)
	}
	_, err = env.svc.MarkSent(got[0].ID, now.Add(time.Minute))
	testutil.AssertNoError(t, err)

	got, err = env.svc.Due(time.Date(2027, 3, 4, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if len(got) != 0 {
		t.Fatalf("the sent fee reminder covers the 15 Mar 2027 nudge, got %+v", got)
	}
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); !utcDay(pace.DueOn).Equal(utcYMD(2028, 3, 15)) {
		t.Fatalf("pace row due %v, want the next window's end 2028-03-15", pace.DueOn)
	}
}

// R1: for a calendar-year waiver the fee reminder covers the window that
// decides it (the previous year), so the current year's nudge is kept.
func TestCardFeeReminders_SentCalendarYearFeeKeepsCurrentPace(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear),
		time.Date(2027, 1, 20, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	_, err = env.svc.MarkSent(fee.ID, time.Date(2027, 2, 13, 1, 5, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); !utcDay(pace.DueOn).Equal(utcYMD(2027, 12, 31)) {
		t.Fatalf("pace row due %v, want 2027-12-31 (not decided by the 2027 fee)", pace.DueOn)
	}
}

// R2: the card's one fee switch carries over to a waiver nudge planned after
// the switch was turned off (a waiver added later).
func TestCardFeeReminders_SwitchOffCarriesToNewWaiver(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)}, now)
	testutil.AssertNoError(t, err)
	fee := env.unsent(t, models.ReminderKindCardAnnualFee)
	_, err = env.svc.SetEnabled(env.user.ID, fee.ID, false)
	testutil.AssertNoError(t, err)

	_, err = env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); pace.Enabled {
		t.Fatal("the new waiver nudge must follow the card's fee switch (off)")
	}
	if env.unsent(t, models.ReminderKindCardAnnualFee).Enabled {
		t.Fatal("the fee reminder must stay off")
	}
}

// R3: the fee reminder switched off stays off after its date passes unsent.
func TestCardFeeReminders_SwitchOffSurvivesStaleFeeReminder(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: intp(15)},
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	_, err = env.svc.SetEnabled(env.user.ID, env.unsent(t, models.ReminderKindCardAnnualFee).ID, false)
	testutil.AssertNoError(t, err)

	_, err = env.svc.Due(time.Date(2027, 3, 20, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindCardAnnualFee)
	if !utcDay(next.DueOn).Equal(utcYMD(2028, 3, 15)) || next.Enabled {
		t.Fatalf("next fee reminder must be 2028-03-15 and off, got %+v", next)
	}
}

// R3: the waiver nudge switched off (by the card's shared switch) stays off
// once its window passes unsent.
func TestCardFeeReminders_SwitchOffSurvivesStalePace(t *testing.T) {
	env := feeFixture(t, 0, 0)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCalendarYear),
		time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	_, err = env.svc.SetEnabled(env.user.ID, env.unsent(t, models.ReminderKindCardAnnualFee).ID, false)
	testutil.AssertNoError(t, err)

	// Both the 2026 nudge and the 15 Mar 2027 fee reminder have passed unsent,
	// so no row of either kind is left to carry the switch.
	_, err = env.svc.Due(time.Date(2027, 3, 20, 2, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	next := env.unsent(t, models.ReminderKindFeeWaiverPace)
	if !utcDay(next.DueOn).Equal(utcYMD(2027, 12, 31)) || next.Enabled {
		t.Fatalf("next pace reminder must be 2027-12-31 and off, got %+v", next)
	}
	if fee := env.unsent(t, models.ReminderKindCardAnnualFee); fee.Enabled {
		t.Fatalf("next fee reminder must stay off, got %+v", fee)
	}
}

// Fix round 2: moving a card-year fee date earlier after this year's reminder
// was sent must not treat next year's deadline as covered.
func TestCardFeeReminders_MoveDateEarlierAfterSentKeepsNextPace(t *testing.T) {
	env := feeFixture(t, 0, 0)
	now := time.Date(2027, 3, 3, 2, 0, 0, 0, time.UTC)
	_, err := env.fees.Set(env.user.ID, env.card.ID, spendWaiver(models.WaiverWindowCardYear), now)
	testutil.AssertNoError(t, err)
	got, err := env.svc.Due(now)
	testutil.AssertNoError(t, err)
	if len(got) != 1 || got[0].Kind != models.ReminderKindCardAnnualFee {
		t.Fatalf("expected only the fee reminder, got %+v", got)
	}
	_, err = env.svc.MarkSent(got[0].ID, now.Add(time.Minute))
	testutil.AssertNoError(t, err)

	moved := spendWaiver(models.WaiverWindowCardYear)
	moved.Day = intp(10)
	_, err = env.fees.Set(env.user.ID, env.card.ID, moved, time.Date(2027, 6, 1, 12, 0, 0, 0, time.UTC))
	testutil.AssertNoError(t, err)
	if fee := env.unsent(t, models.ReminderKindCardAnnualFee); !utcDay(fee.DueOn).Equal(utcYMD(2028, 3, 10)) {
		t.Fatalf("fee reminder due %v, want 2028-03-10", fee.DueOn)
	}
	if pace := env.unsent(t, models.ReminderKindFeeWaiverPace); !utcDay(pace.DueOn).Equal(utcYMD(2028, 3, 10)) {
		t.Fatalf("pace row due %v, want 2028-03-10 (not covered by the 2027 fee reminder)", pace.DueOn)
	}
}

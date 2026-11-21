package services

import (
	"testing"
	"time"

	"gorm.io/gorm"

	"kuberan/internal/models"
	"kuberan/internal/testutil"
)

func addCardTx(t *testing.T, db *gorm.DB, userID, accountID string, typ models.TransactionType, amount int64, desc string, date time.Time) *models.Transaction {
	t.Helper()
	tx := &models.Transaction{UserID: userID, AccountID: accountID, Type: typ, Amount: amount, Description: desc, Date: date}
	testutil.AssertNoError(t, db.Create(tx).Error)
	return tx
}

func TestCardSpend_CountsOnlyExpenses(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	d := utcYMD(2026, 3, 10)

	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 100_00, "a", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 50_00, "b", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeIncome, 30_00, "refund", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeTransfer, 200_00, "payment", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeOpeningBalance, 999_00, "opening", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeReimbursement, 10_00, "reimb", d)
	del := addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 77_00, "deleted", d)
	testutil.AssertNoError(t, db.Delete(del).Error)

	spend, swipes, err := cardSpend(db, user.ID, card.ID, utcYMD(2026, 1, 1), utcYMD(2026, 12, 31))
	testutil.AssertNoError(t, err)
	if spend != 150_00 || swipes != 2 {
		t.Fatalf("spend=%d swipes=%d, want 15000 and 2", spend, swipes)
	}
}

func TestCardSpend_ExcludesFeeRows(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	d := utcYMD(2026, 3, 10)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 250_00, "Annual Fee 2026", d)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 250_00, "YURAN TAHUNAN", d)

	spend, swipes, err := cardSpend(db, user.ID, card.ID, utcYMD(2026, 1, 1), utcYMD(2026, 12, 31))
	testutil.AssertNoError(t, err)
	if spend != 0 || swipes != 0 {
		t.Fatalf("spend=%d swipes=%d, want 0 and 0", spend, swipes)
	}
}

func TestCardSpend_WindowBoundaries(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	start, end := utcYMD(2026, 3, 16), utcYMD(2027, 3, 15)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 1_00, "on start", start)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 2_00, "on end", end.Add(23*time.Hour))
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 4_00, "after end", end.AddDate(0, 0, 1))
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 8_00, "before start", start.AddDate(0, 0, -1))

	spend, swipes, err := cardSpend(db, user.ID, card.ID, start, end)
	testutil.AssertNoError(t, err)
	if spend != 3_00 || swipes != 2 {
		t.Fatalf("spend=%d swipes=%d, want 300 and 2", spend, swipes)
	}
}

func TestCardSpend_SharedExpenseCountsInFull(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	tx := addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 300_00, "dinner", utcYMD(2026, 3, 10))
	personal := int64(100_00)
	testutil.AssertNoError(t, db.Model(tx).Update("personal_amount", personal).Error)

	spend, _, err := cardSpend(db, user.ID, card.ID, utcYMD(2026, 1, 1), utcYMD(2026, 12, 31))
	testutil.AssertNoError(t, err)
	if spend != 300_00 {
		t.Fatalf("spend=%d, want 30000", spend)
	}
}

func feeWith(typ models.WaiverType, spend *int64, swipes *int) *models.CardAnnualFee {
	return &models.CardAnnualFee{WaiverType: &typ, WaiverSpend: spend, WaiverSwipes: swipes, WaiverWindow: models.WaiverWindowCardYear}
}

func TestWaiverStatus(t *testing.T) {
	start, end := utcYMD(2026, 1, 1), utcYMD(2026, 4, 10) // 100 days
	target := int64(12_000_00)
	swipeTarget := 12
	tests := []struct {
		name   string
		fee    *models.CardAnnualFee
		spend  int64
		swipes int64
		now    time.Time
		want   FeeWaiverStatus
	}{
		{"met", feeWith(models.WaiverTypeSpend, &target, nil), 12_000_00, 0, utcYMD(2026, 2, 19), FeeWaiverMet},
		{"on pace boundary", feeWith(models.WaiverTypeSpend, &target, nil), 5_400_00, 0, utcYMD(2026, 2, 19), FeeWaiverOnTrack},
		{"just behind", feeWith(models.WaiverTypeSpend, &target, nil), 5_399_00, 0, utcYMD(2026, 2, 19), FeeWaiverBehind},
		{"first 30 days never behind", feeWith(models.WaiverTypeSpend, &target, nil), 0, 0, utcYMD(2026, 1, 20), FeeWaiverOnTrack},
		{"swipes meet either target", feeWith(models.WaiverTypeSpendOrSwipes, &target, &swipeTarget), 0, 12, utcYMD(2026, 2, 19), FeeWaiverMet},
		{"swipes only ignores spend", feeWith(models.WaiverTypeSwipes, nil, &swipeTarget), 99_000_00, 0, utcYMD(2026, 2, 19), FeeWaiverBehind},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waiverStatus(tt.fee, tt.spend, tt.swipes, start, end, tt.now); got != tt.want {
				t.Fatalf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestCardFeeSummary_PerMonthNeeded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	svc := NewCardFeeService(db)
	day, spendT, typ := 15, int64(12_000_00), models.WaiverTypeSpend
	// Window 2026-03-16..2027-03-15; now is 114 days before the end.
	now := utcYMD(2026, 11, 21)
	_, err := svc.Set(user.ID, card.ID, CardAnnualFeeInput{Amount: 250_00, Month: 3, Day: &day, WaiverType: &typ, WaiverSpend: &spendT}, now)
	testutil.AssertNoError(t, err)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 8_420_00, "spend", utcYMD(2026, 6, 1))

	got, err := svc.Summary(user.ID, card.ID, now)
	testutil.AssertNoError(t, err)
	w := got.Waiver
	if w == nil || w.DaysLeft != 114 || w.Spend.Remaining != 3_580_00 || w.WindowStart != "2026-03-16" || w.WindowEnd != "2027-03-15" {
		t.Fatalf("unexpected waiver: %+v", w)
	}
	if w.PerMonthNeeded == nil || *w.PerMonthNeeded != 89_500 {
		t.Fatalf("per month = %v, want 89500", w.PerMonthNeeded)
	}
	if got.Fee == nil || got.Fee.NextFeeDate != "2027-03-15" || !got.Fee.DayKnown || got.SpentLast12Months != 8_420_00 {
		t.Fatalf("unexpected summary: %+v", got)
	}

	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 3_580_00, "more", utcYMD(2026, 11, 1))
	got, err = svc.Summary(user.ID, card.ID, now)
	testutil.AssertNoError(t, err)
	if got.Waiver.Status != FeeWaiverMet || got.Waiver.PerMonthNeeded != nil {
		t.Fatalf("expected met with no per-month figure, got %+v", got.Waiver)
	}
}

func TestCardFeeSummary_UsesCardCurrency(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	user := testutil.CreateTestUser(t, db)
	testutil.AssertNoError(t, db.Model(user).Update("base_currency", "MYR").Error)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0) // USD
	svc := NewCardFeeService(db)
	now := utcYMD(2026, 6, 1)
	addCardTx(t, db, user.ID, card.ID, models.TransactionTypeExpense, 123_45, "x", utcYMD(2026, 5, 1))

	got, err := svc.Summary(user.ID, card.ID, now)
	testutil.AssertNoError(t, err)
	if got.Currency != "USD" || got.SpentLast12Months != 123_45 || got.Fee != nil || got.Waiver != nil {
		t.Fatalf("unexpected summary: %+v", got)
	}
}

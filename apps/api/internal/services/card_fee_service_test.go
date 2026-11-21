package services

import (
	"testing"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/testutil"
)

func TestCardFeeService_Validation(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	svc := NewCardFeeService(db)
	user := testutil.CreateTestUser(t, db)
	other := testutil.CreateTestUserWithEmail(t, db, "other@example.com")
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	cash := testutil.CreateTestCashAccount(t, db, user.ID)
	now := utcYMD(2026, 6, 1)

	typSpend, typSwipes := models.WaiverTypeSpend, models.WaiverTypeSwipes
	d32, target, count := 32, int64(1000_00), 5
	ok := CardAnnualFeeInput{Amount: 100_00, Month: 3}

	tests := []struct {
		name    string
		user    string
		account string
		mutate  func(in *CardAnnualFeeInput)
		code    string
	}{
		{"non-card account", user.ID, cash.ID, nil, apperrors.ErrInvalidAccountType.Code},
		{"another user's card", other.ID, card.ID, nil, apperrors.ErrAccountNotFound.Code},
		{"zero amount", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.Amount = 0 }, apperrors.ErrInvalidInput.Code},
		{"month 13", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.Month = 13 }, apperrors.ErrInvalidInput.Code},
		{"day 32", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.Day = &d32 }, apperrors.ErrInvalidInput.Code},
		{"spend without target", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.WaiverType = &typSpend }, apperrors.ErrInvalidInput.Code},
		{"swipes without target", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.WaiverType = &typSwipes }, apperrors.ErrInvalidInput.Code},
		{"targets without a type", user.ID, card.ID, func(in *CardAnnualFeeInput) { in.WaiverSpend = &target; in.WaiverSwipes = &count }, apperrors.ErrInvalidInput.Code},
		{"swipes target on spend type", user.ID, card.ID, func(in *CardAnnualFeeInput) {
			in.WaiverType = &typSpend
			in.WaiverSpend = &target
			in.WaiverSwipes = &count
		}, apperrors.ErrInvalidInput.Code},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ok
			if tt.mutate != nil {
				tt.mutate(&in)
			}
			_, err := svc.Set(tt.user, tt.account, in, now)
			testutil.AssertAppError(t, err, tt.code)
		})
	}
	var n int64
	db.Model(&models.CardAnnualFee{}).Count(&n)
	if n != 0 {
		t.Fatalf("invalid input must not write rows, got %d", n)
	}
}

func TestCardFeeService_SetReplaceDeleteReAdd(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	svc := NewCardFeeService(db)
	user := testutil.CreateTestUser(t, db)
	card := testutil.CreateTestCreditCardAccount(t, db, user.ID, 0)
	now := utcYMD(2026, 6, 1)
	typ, target := models.WaiverTypeSpend, int64(5000_00)

	first, err := svc.Set(user.ID, card.ID, CardAnnualFeeInput{Amount: 100_00, Month: 3, WaiverType: &typ, WaiverSpend: &target}, now)
	testutil.AssertNoError(t, err)
	if first.WaiverWindow != models.WaiverWindowCardYear {
		t.Fatalf("window should default to card_year, got %q", first.WaiverWindow)
	}

	// Replacing keeps one row and clears the waiver.
	second, err := svc.Set(user.ID, card.ID, CardAnnualFeeInput{Amount: 200_00, Month: 4}, now)
	testutil.AssertNoError(t, err)
	if second.ID != first.ID || second.Amount != 200_00 || second.WaiverType != nil || second.WaiverSpend != nil {
		t.Fatalf("not replaced in place: %+v", second)
	}
	var n int64
	db.Unscoped().Model(&models.CardAnnualFee{}).Where("account_id = ?", card.ID).Count(&n)
	if n != 1 {
		t.Fatalf("want one row, got %d", n)
	}

	testutil.AssertNoError(t, svc.Delete(user.ID, card.ID, now))
	db.Unscoped().Model(&models.CardAnnualFee{}).Where("account_id = ?", card.ID).Count(&n)
	if n != 0 {
		t.Fatalf("row should be hard-deleted, got %d", n)
	}

	_, err = svc.Set(user.ID, card.ID, CardAnnualFeeInput{Amount: 300_00, Month: 5}, now)
	testutil.AssertNoError(t, err)
}

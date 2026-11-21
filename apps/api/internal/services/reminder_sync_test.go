package services

import (
	"testing"
	"time"

	"kuberan/internal/models"
	"kuberan/internal/testutil"
)

// A reminder of a kind the account no longer plans is removed on sync, while
// the planned kind's reminder is kept as it was (per-kind reconcile).
func TestSyncAccountReminders_DropsUnplannedKindKeepsPlanned(t *testing.T) {
	db := testutil.SetupTestDB(t)
	defer testutil.TeardownTestDB(t, db)
	accts := NewAccountService(db)
	user := testutil.CreateTestUser(t, db)
	card, err := accts.CreateCreditCardAccountWithParams(user.ID, CreateCreditCardParams{Name: "Card", Currency: "MYR", DueDay: 5})
	testutil.AssertNoError(t, err)

	var due models.Reminder
	testutil.AssertNoError(t, db.Where("ref_id = ? AND kind = ?", card.ID, models.ReminderKindCardDue).First(&due).Error)

	stray := &models.Reminder{
		UserID: user.ID, Kind: models.ReminderKindLoanDue, RefID: card.ID,
		DueOn: due.DueOn, RemindAt: due.RemindAt, Enabled: true,
	}
	testutil.AssertNoError(t, db.Create(stray).Error)

	testutil.AssertNoError(t, syncAccountReminders(db, card.ID, time.Now()))

	var rows []models.Reminder
	testutil.AssertNoError(t, db.Unscoped().Where("ref_id = ?", card.ID).Find(&rows).Error)
	if len(rows) != 1 || rows[0].ID != due.ID || rows[0].Kind != models.ReminderKindCardDue {
		t.Fatalf("expected only the original card_due reminder to remain, got %+v", rows)
	}
}

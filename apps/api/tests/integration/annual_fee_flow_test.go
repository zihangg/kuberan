package integration

// Integration test for credit card annual fees (plan 025): set a fee with a
// spend waiver, record a purchase, read the waiver progress and let the bot
// pull the fee and pace reminders.

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"kuberan/internal/models"
)

func TestAnnualFeeWorkflow(t *testing.T) {
	app := setupApp(t)
	token, _, userID := app.registerUser(t, "annual-fee@test.com", "password123")
	if err := app.DB.Create(&models.TelegramLink{UserID: userID, TelegramUserID: 5150, IsActive: true}).Error; err != nil {
		t.Fatalf("link: %v", err)
	}

	rec := app.request("POST", "/api/v1/accounts/credit-card",
		`{"name":"Visa Infinite","currency":"MYR","credit_limit":2000000}`, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create card: %d %s", rec.Code, rec.Body.String())
	}
	cardID := accountOf(t, rec)["id"].(string)

	feeDate := time.Now().UTC().AddDate(0, 0, 20)
	rec = app.request("PUT", "/api/v1/accounts/"+cardID+"/annual-fee", fmt.Sprintf(
		`{"amount":25000,"month":%d,"day":%d,"waiver_type":"spend","waiver_spend":1200000,"waiver_window":"card_year"}`,
		int(feeDate.Month()), feeDate.Day()), token)
	if rec.Code != http.StatusOK {
		t.Fatalf("set annual fee: %d %s", rec.Code, rec.Body.String())
	}

	rec = app.request("POST", "/api/v1/transactions",
		fmt.Sprintf(`{"account_id":%q,"type":"expense","amount":10000,"description":"Groceries"}`, cardID), token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expense: %d %s", rec.Code, rec.Body.String())
	}

	rec = app.request("GET", "/api/v1/accounts/"+cardID+"/fee-waiver", "", token)
	if rec.Code != http.StatusOK {
		t.Fatalf("fee-waiver: %d %s", rec.Code, rec.Body.String())
	}
	waiver := parseJSON(t, rec)["waiver"].(map[string]interface{})
	if spend := waiver["spend"].(map[string]interface{}); spend["actual"].(float64) != 10000 {
		t.Fatalf("expected spend 10000, got %v", spend)
	}
	if waiver["status"] != "behind" {
		t.Fatalf("expected behind, got %v", waiver["status"])
	}

	rec = app.botRequest("GET", "/api/v1/internal/telegram/reminders/due")
	if rec.Code != http.StatusOK {
		t.Fatalf("due: %d %s", rec.Code, rec.Body.String())
	}
	due := parseJSON(t, rec)["reminders"].([]interface{})
	byKind := map[string]map[string]interface{}{}
	for _, item := range due {
		d := item.(map[string]interface{})
		byKind[d["kind"].(string)] = d
	}
	if len(due) != 2 {
		t.Fatalf("expected the fee and pace reminders, got %v", due)
	}
	wantDue := feeDate.Format("2006-01-02")

	fee, ok := byKind["card_annual_fee"]
	if !ok {
		t.Fatalf("no card_annual_fee reminder in %v", due)
	}
	if fee["due_on"] != wantDue || fee["annual_fee"].(float64) != 25000 || fee["fee_day_known"] != true ||
		fee["spent_last_12_months"].(float64) != 10000 || fee["telegram_user_id"].(float64) != 5150 {
		t.Fatalf("unexpected fee reminder: %v", fee)
	}
	if fee["fee_waiver"] == nil {
		t.Fatalf("the fee reminder carries the waiver progress: %v", fee)
	}

	pace, ok := byKind["fee_waiver_pace"]
	if !ok {
		t.Fatalf("no fee_waiver_pace reminder in %v", due)
	}
	pw, ok := pace["fee_waiver"].(map[string]interface{})
	if !ok || pw["status"] != "behind" || pace["due_on"] != wantDue {
		t.Fatalf("unexpected pace reminder: %v", pace)
	}
}

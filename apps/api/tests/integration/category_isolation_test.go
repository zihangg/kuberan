package integration

import (
	"fmt"
	"net/http"
	"testing"
)

// TestTransactionCategoryIsolation checks that a user cannot attach another
// user's category to their own transaction, on create or on update.
func TestTransactionCategoryIsolation(t *testing.T) {
	app := setupApp(t)
	victimToken, _, _ := app.registerUser(t, "victim@test.com", "password123")
	attackerToken, _, _ := app.registerUser(t, "attacker@test.com", "password123")

	rec := app.request("POST", "/api/v1/categories", `{"name":"Therapy","type":"expense"}`, victimToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create category failed: %d %s", rec.Code, rec.Body.String())
	}
	foreignCategory := parseJSON(t, rec)["category"].(map[string]interface{})["id"].(string)

	rec = app.request("POST", "/api/v1/accounts/cash", `{"name":"Wallet","initial_balance":10000}`, attackerToken)
	accountID := parseJSON(t, rec)["account"].(map[string]interface{})["id"].(string)

	rec = app.request("POST", "/api/v1/transactions",
		fmt.Sprintf(`{"account_id":%q,"category_id":%q,"type":"expense","amount":100}`, accountID, foreignCategory), attackerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("create with another user's category: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
	if code := parseJSON(t, rec)["error"].(map[string]interface{})["code"]; code != "CATEGORY_NOT_FOUND" {
		t.Errorf("expected CATEGORY_NOT_FOUND, got %v", code)
	}

	rec = app.request("POST", "/api/v1/transactions",
		fmt.Sprintf(`{"account_id":%q,"type":"expense","amount":100}`, accountID), attackerToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create transaction failed: %d %s", rec.Code, rec.Body.String())
	}
	txnID := parseJSON(t, rec)["transaction"].(map[string]interface{})["id"].(string)

	rec = app.request("PUT", "/api/v1/transactions/"+txnID,
		fmt.Sprintf(`{"category_id":%q}`, foreignCategory), attackerToken)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("update to another user's category: expected 404, got %d: %s", rec.Code, rec.Body.String())
	}

	rec = app.request("GET", "/api/v1/transactions/"+txnID, "", attackerToken)
	txn := parseJSON(t, rec)["transaction"].(map[string]interface{})
	if txn["category_id"] != nil || txn["category"] != nil {
		t.Errorf("transaction must stay uncategorized, got category_id=%v category=%v", txn["category_id"], txn["category"])
	}
}

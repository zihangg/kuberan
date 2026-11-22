package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/services"
)

type mockCardFeeService struct {
	setFn     func(userID, accountID string, in services.CardAnnualFeeInput) (*models.CardAnnualFee, error)
	deleteFn  func(userID, accountID string) error
	summaryFn func(userID, accountID string) (*services.CardFeeSummary, error)
}

func (m *mockCardFeeService) Set(userID, accountID string, in services.CardAnnualFeeInput, _ time.Time) (*models.CardAnnualFee, error) {
	if m.setFn != nil {
		return m.setFn(userID, accountID, in)
	}
	return &models.CardAnnualFee{}, nil
}

func (m *mockCardFeeService) Delete(userID, accountID string, _ time.Time) error {
	if m.deleteFn != nil {
		return m.deleteFn(userID, accountID)
	}
	return nil
}

func (m *mockCardFeeService) Summary(userID, accountID string, _ time.Time) (*services.CardFeeSummary, error) {
	if m.summaryFn != nil {
		return m.summaryFn(userID, accountID)
	}
	return &services.CardFeeSummary{}, nil
}

var _ services.CardFeeServicer = (*mockCardFeeService)(nil)

func setupCardFeeRouter(h *CardFeeHandler) *gin.Engine {
	r := gin.New()
	auth := r.Group("", injectUserID("u1"))
	auth.PUT("/accounts/:id/annual-fee", h.Set)
	auth.DELETE("/accounts/:id/annual-fee", h.Delete)
	auth.GET("/accounts/:id/fee-waiver", h.Summary)
	return r
}

func TestCardFeeHandler_Set(t *testing.T) {
	t.Run("sets the fee and echoes it", func(t *testing.T) {
		var gotUser, gotAcct string
		var gotIn services.CardAnnualFeeInput
		h := NewCardFeeHandler(&mockCardFeeService{setFn: func(u, a string, in services.CardAnnualFeeInput) (*models.CardAnnualFee, error) {
			gotUser, gotAcct, gotIn = u, a, in
			return &models.CardAnnualFee{AccountID: a, Amount: in.Amount, FeeMonth: in.Month, FeeDay: in.Day, WaiverType: in.WaiverType, WaiverSpend: in.WaiverSpend, WaiverWindow: in.WaiverWindow}, nil
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "PUT", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee",
			`{"amount":25000,"month":3,"day":15,"waiver_type":"spend","waiver_spend":1200000}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if gotUser != "u1" || gotAcct != "11111111-1111-4111-8111-111111111111" {
			t.Fatalf("service got user=%q account=%q", gotUser, gotAcct)
		}
		if gotIn.Amount != 25000 || gotIn.Month != 3 || gotIn.Day == nil || *gotIn.Day != 15 {
			t.Fatalf("unexpected input: %+v", gotIn)
		}
		if gotIn.WaiverWindow != models.WaiverWindowCardYear {
			t.Fatalf("omitted window should default to card_year, got %q", gotIn.WaiverWindow)
		}
		fee := parseJSON(t, rec)["annual_fee"].(map[string]interface{})
		if fee["amount"] != float64(25000) || fee["fee_month"] != float64(3) || fee["waiver_type"] != "spend" {
			t.Fatalf("unexpected body: %v", fee)
		}
	})
	t.Run("passes an explicit window", func(t *testing.T) {
		var got models.WaiverWindow
		h := NewCardFeeHandler(&mockCardFeeService{setFn: func(_, _ string, in services.CardAnnualFeeInput) (*models.CardAnnualFee, error) {
			got = in.WaiverWindow
			return &models.CardAnnualFee{}, nil
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "PUT", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee", `{"amount":100,"month":1,"waiver_window":"calendar_year"}`)
		if rec.Code != http.StatusOK || got != models.WaiverWindowCalendarYear {
			t.Fatalf("expected 200 and calendar_year, got %d window=%q", rec.Code, got)
		}
	})
	t.Run("month 0 is rejected", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "PUT", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee", `{"amount":100,"month":0}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
	t.Run("unknown window is rejected", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "PUT", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee", `{"amount":100,"month":1,"waiver_window":"weekly"}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
	t.Run("service error keeps its code", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{setFn: func(string, string, services.CardAnnualFeeInput) (*models.CardAnnualFee, error) {
			return nil, apperrors.ErrInvalidAccountType
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "PUT", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee", `{"amount":100,"month":1}`)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		if code := parseJSON(t, rec)["error"].(map[string]interface{})["code"]; code != "INVALID_ACCOUNT_TYPE" {
			t.Fatalf("expected INVALID_ACCOUNT_TYPE, got %v", code)
		}
	})
}

func TestCardFeeHandler_Delete(t *testing.T) {
	var gotAcct string
	h := NewCardFeeHandler(&mockCardFeeService{deleteFn: func(_, a string) error { gotAcct = a; return nil }}, &mockAuditService{})
	rec := doRequest(setupCardFeeRouter(h), "DELETE", "/accounts/11111111-1111-4111-8111-111111111111/annual-fee", "")
	if rec.Code != http.StatusNoContent || gotAcct != "11111111-1111-4111-8111-111111111111" {
		t.Fatalf("expected 204 for 11111111-1111-4111-8111-111111111111, got %d account=%q", rec.Code, gotAcct)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected empty body, got %q", rec.Body.String())
	}
}

func TestCardFeeHandler_Summary(t *testing.T) {
	t.Run("returns the progress", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{summaryFn: func(_, _ string) (*services.CardFeeSummary, error) {
			return &services.CardFeeSummary{
				Currency: "MYR",
				Fee:      &services.CardFeeInfo{Amount: 25000, NextFeeDate: "2027-03-15", DayKnown: true},
				Waiver:   &services.FeeWaiverProgress{Type: "spend", Window: "card_year", Status: "behind"},
			}, nil
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "GET", "/accounts/11111111-1111-4111-8111-111111111111/fee-waiver", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := parseJSON(t, rec)
		fee := body["fee"].(map[string]interface{})
		waiver := body["waiver"].(map[string]interface{})
		if body["currency"] != "MYR" || fee["next_fee_date"] != "2027-03-15" || fee["day_known"] != true ||
			waiver["status"] != "behind" || waiver["window"] != "card_year" {
			t.Fatalf("unexpected body: %v", body)
		}
		if _, ok := body["spent_last_12_months"]; !ok {
			t.Fatalf("missing spent_last_12_months: %v", body)
		}
	})
	t.Run("card without a fee has null fee and waiver", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{summaryFn: func(_, _ string) (*services.CardFeeSummary, error) {
			return &services.CardFeeSummary{Currency: "MYR"}, nil
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "GET", "/accounts/11111111-1111-4111-8111-111111111111/fee-waiver", "")
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		body := parseJSON(t, rec)
		fee, feeOK := body["fee"]
		waiver, waiverOK := body["waiver"]
		if !feeOK || fee != nil || !waiverOK || waiver != nil {
			t.Fatalf("expected explicit null fee and waiver, got %v", body)
		}
	})
	t.Run("unknown account is 404", func(t *testing.T) {
		h := NewCardFeeHandler(&mockCardFeeService{summaryFn: func(_, _ string) (*services.CardFeeSummary, error) {
			return nil, apperrors.ErrAccountNotFound
		}}, &mockAuditService{})
		rec := doRequest(setupCardFeeRouter(h), "GET", "/accounts/11111111-1111-4111-8111-111111111111/fee-waiver", "")
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		if code := parseJSON(t, rec)["error"].(map[string]interface{})["code"]; code != "ACCOUNT_NOT_FOUND" {
			t.Fatalf("expected ACCOUNT_NOT_FOUND, got %v", code)
		}
	})
}

func TestCardFeeHandler_RejectsMalformedAccountID(t *testing.T) {
	called := false
	h := NewCardFeeHandler(&mockCardFeeService{
		setFn: func(string, string, services.CardAnnualFeeInput) (*models.CardAnnualFee, error) {
			called = true
			return &models.CardAnnualFee{}, nil
		},
		deleteFn: func(string, string) error { called = true; return nil },
		summaryFn: func(string, string) (*services.CardFeeSummary, error) {
			called = true
			return &services.CardFeeSummary{}, nil
		},
	}, &mockAuditService{})
	r := setupCardFeeRouter(h)
	cases := []struct{ method, path, body string }{
		{"PUT", "/accounts/not-a-uuid/annual-fee", `{"amount":100,"month":1}`},
		{"DELETE", "/accounts/not-a-uuid/annual-fee", ""},
		{"GET", "/accounts/not-a-uuid/fee-waiver", ""},
	}
	for _, tc := range cases {
		rec := doRequest(r, tc.method, tc.path, tc.body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s %s: expected 400, got %d", tc.method, tc.path, rec.Code)
		}
		if code := parseJSON(t, rec)["error"].(map[string]interface{})["code"]; code != "INVALID_INPUT" {
			t.Fatalf("%s %s: expected INVALID_INPUT, got %v", tc.method, tc.path, code)
		}
	}
	if called {
		t.Fatal("service must not be called for a malformed id")
	}
}

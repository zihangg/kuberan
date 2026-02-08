package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/services"
)

// --- mock transaction service ---

type mockTransactionService struct {
	createTransactionFn      func(userID, accountID uint, categoryID *uint, transactionType models.TransactionType, amount int64, description string, date time.Time) (*models.Transaction, error)
	createTransferFn         func(userID, fromAccountID, toAccountID uint, amount int64, description string, date time.Time) (*models.Transaction, error)
	getAccountTransactionsFn func(userID, accountID uint, page pagination.PageRequest, filter services.TransactionFilter) (*pagination.PageResponse[models.Transaction], error)
	getTransactionByIDFn     func(userID, transactionID uint) (*models.Transaction, error)
	deleteTransactionFn      func(userID, transactionID uint) error
}

func (m *mockTransactionService) CreateTransaction(userID, accountID uint, categoryID *uint, transactionType models.TransactionType, amount int64, description string, date time.Time) (*models.Transaction, error) {
	if m.createTransactionFn != nil {
		return m.createTransactionFn(userID, accountID, categoryID, transactionType, amount, description, date)
	}
	return &models.Transaction{}, nil
}

func (m *mockTransactionService) CreateTransfer(userID, fromAccountID, toAccountID uint, amount int64, description string, date time.Time) (*models.Transaction, error) {
	if m.createTransferFn != nil {
		return m.createTransferFn(userID, fromAccountID, toAccountID, amount, description, date)
	}
	return &models.Transaction{}, nil
}

func (m *mockTransactionService) GetAccountTransactions(userID, accountID uint, page pagination.PageRequest, filter services.TransactionFilter) (*pagination.PageResponse[models.Transaction], error) {
	if m.getAccountTransactionsFn != nil {
		return m.getAccountTransactionsFn(userID, accountID, page, filter)
	}
	resp := pagination.NewPageResponse([]models.Transaction{}, 1, 20, 0)
	return &resp, nil
}

func (m *mockTransactionService) GetTransactionByID(userID, transactionID uint) (*models.Transaction, error) {
	if m.getTransactionByIDFn != nil {
		return m.getTransactionByIDFn(userID, transactionID)
	}
	return &models.Transaction{}, nil
}

func (m *mockTransactionService) DeleteTransaction(userID, transactionID uint) error {
	if m.deleteTransactionFn != nil {
		return m.deleteTransactionFn(userID, transactionID)
	}
	return nil
}

var _ services.TransactionServicer = (*mockTransactionService)(nil)

func setupTransactionRouter(handler *TransactionHandler) *gin.Engine {
	r := gin.New()
	auth := r.Group("", injectUserID(1))
	auth.POST("/transactions", handler.CreateTransaction)
	auth.POST("/transactions/transfer", handler.CreateTransfer)
	auth.GET("/accounts/:id/transactions", handler.GetAccountTransactions)
	auth.GET("/transactions/:id", handler.GetTransactionByID)
	auth.DELETE("/transactions/:id", handler.DeleteTransaction)
	return r
}

func TestTransactionHandler_CreateTransaction(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		txSvc := &mockTransactionService{
			createTransactionFn: func(userID, accountID uint, _ *uint, txType models.TransactionType, amount int64, desc string, _ time.Time) (*models.Transaction, error) {
				return &models.Transaction{
					Base:      models.Base{ID: 1},
					UserID:    userID,
					AccountID: accountID,
					Type:      txType,
					Amount:    amount,
				}, nil
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions",
			`{"account_id":1,"type":"income","amount":5000,"description":"Salary"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		tx := result["transaction"].(map[string]interface{})
		if tx["amount"].(float64) != 5000 {
			t.Errorf("expected amount 5000, got %v", tx["amount"])
		}
	})

	t.Run("returns 400 on missing account_id", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions",
			`{"type":"income","amount":5000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on zero amount", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions",
			`{"account_id":1,"type":"expense","amount":0}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on invalid type", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions",
			`{"account_id":1,"type":"invalid","amount":1000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when account not found", func(t *testing.T) {
		txSvc := &mockTransactionService{
			createTransactionFn: func(_, _ uint, _ *uint, _ models.TransactionType, _ int64, _ string, _ time.Time) (*models.Transaction, error) {
				return nil, apperrors.ErrAccountNotFound
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions",
			`{"account_id":999,"type":"income","amount":1000}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := gin.New()
		r.POST("/transactions", handler.CreateTransaction)

		rec := doRequest(r, "POST", "/transactions",
			`{"account_id":1,"type":"income","amount":1000}`)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_CreateTransfer(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		txSvc := &mockTransactionService{
			createTransferFn: func(userID, from, to uint, amount int64, _ string, _ time.Time) (*models.Transaction, error) {
				toAcct := to
				return &models.Transaction{
					Base:        models.Base{ID: 1},
					UserID:      userID,
					AccountID:   from,
					ToAccountID: &toAcct,
					Type:        models.TransactionTypeTransfer,
					Amount:      amount,
				}, nil
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions/transfer",
			`{"from_account_id":1,"to_account_id":2,"amount":1000,"description":"Transfer"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("returns 400 on same account", func(t *testing.T) {
		txSvc := &mockTransactionService{
			createTransferFn: func(_, _, _ uint, _ int64, _ string, _ time.Time) (*models.Transaction, error) {
				return nil, apperrors.ErrSameAccountTransfer
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions/transfer",
			`{"from_account_id":1,"to_account_id":1,"amount":1000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "SAME_ACCOUNT_TRANSFER")
	})

	t.Run("returns 400 on insufficient balance", func(t *testing.T) {
		txSvc := &mockTransactionService{
			createTransferFn: func(_, _, _ uint, _ int64, _ string, _ time.Time) (*models.Transaction, error) {
				return nil, apperrors.ErrInsufficientBalance
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions/transfer",
			`{"from_account_id":1,"to_account_id":2,"amount":999999}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "INSUFFICIENT_BALANCE")
	})

	t.Run("returns 400 on missing required fields", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "POST", "/transactions/transfer", `{"amount":1000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_GetAccountTransactions(t *testing.T) {
	t.Run("returns 200 with paginated transactions", func(t *testing.T) {
		now := time.Now()
		txSvc := &mockTransactionService{
			getAccountTransactionsFn: func(_, _ uint, _ pagination.PageRequest, _ services.TransactionFilter) (*pagination.PageResponse[models.Transaction], error) {
				resp := pagination.NewPageResponse([]models.Transaction{
					{Base: models.Base{ID: 1}, Amount: 5000, Type: "income", Date: now},
				}, 1, 20, 1)
				return &resp, nil
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/accounts/1/transactions", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		result := parseJSON(t, rec)
		data := result["data"].([]interface{})
		if len(data) != 1 {
			t.Errorf("expected 1 transaction, got %d", len(data))
		}
	})

	t.Run("passes filter params to service", func(t *testing.T) {
		var capturedFilter services.TransactionFilter
		txSvc := &mockTransactionService{
			getAccountTransactionsFn: func(_, _ uint, _ pagination.PageRequest, filter services.TransactionFilter) (*pagination.PageResponse[models.Transaction], error) {
				capturedFilter = filter
				resp := pagination.NewPageResponse([]models.Transaction{}, 1, 20, 0)
				return &resp, nil
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		doRequest(r, "GET", "/accounts/1/transactions?type=income&min_amount=100&max_amount=5000", "")

		if capturedFilter.Type == nil || *capturedFilter.Type != models.TransactionTypeIncome {
			t.Errorf("expected type=income filter, got %v", capturedFilter.Type)
		}
		if capturedFilter.MinAmount == nil || *capturedFilter.MinAmount != 100 {
			t.Errorf("expected min_amount=100, got %v", capturedFilter.MinAmount)
		}
		if capturedFilter.MaxAmount == nil || *capturedFilter.MaxAmount != 5000 {
			t.Errorf("expected max_amount=5000, got %v", capturedFilter.MaxAmount)
		}
	})

	t.Run("returns 400 on invalid type filter", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/accounts/1/transactions?type=invalid", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on invalid date format", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/accounts/1/transactions?from_date=not-a-date", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on invalid min_amount", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/accounts/1/transactions?min_amount=abc", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on invalid account ID", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/accounts/abc/transactions", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_GetTransactionByID(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		txSvc := &mockTransactionService{
			getTransactionByIDFn: func(_, txID uint) (*models.Transaction, error) {
				return &models.Transaction{
					Base:   models.Base{ID: txID},
					Amount: 5000,
					Type:   "income",
				}, nil
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/transactions/1", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		txSvc := &mockTransactionService{
			getTransactionByIDFn: func(_, _ uint) (*models.Transaction, error) {
				return nil, apperrors.ErrTransactionNotFound
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "GET", "/transactions/999", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestTransactionHandler_DeleteTransaction(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "DELETE", "/transactions/1", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		result := parseJSON(t, rec)
		if result["message"] != "Transaction deleted successfully" {
			t.Errorf("unexpected message: %v", result["message"])
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		txSvc := &mockTransactionService{
			deleteTransactionFn: func(_, _ uint) error {
				return apperrors.ErrTransactionNotFound
			},
		}
		handler := NewTransactionHandler(txSvc, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "DELETE", "/transactions/999", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on invalid ID", func(t *testing.T) {
		handler := NewTransactionHandler(&mockTransactionService{}, &mockAuditService{})
		r := setupTransactionRouter(handler)

		rec := doRequest(r, "DELETE", "/transactions/abc", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

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

// --- mock investment service ---

type mockInvestmentService struct {
	addInvestmentFn             func(userID, accountID, securityID uint, quantity float64, purchasePrice int64, walletAddress string) (*models.Investment, error)
	getAccountInvestmentsFn     func(userID, accountID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error)
	getInvestmentByIDFn         func(userID, investmentID uint) (*models.Investment, error)
	updateInvestmentPriceFn     func(userID, investmentID uint, currentPrice int64) (*models.Investment, error)
	getPortfolioFn              func(userID uint) (*services.PortfolioSummary, error)
	recordBuyFn                 func(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)
	recordSellFn                func(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit int64, fee int64, notes string) (*models.InvestmentTransaction, error)
	recordDividendFn            func(userID, investmentID uint, date time.Time, amount int64, dividendType, notes string) (*models.InvestmentTransaction, error)
	recordSplitFn               func(userID, investmentID uint, date time.Time, splitRatio float64, notes string) (*models.InvestmentTransaction, error)
	getInvestmentTransactionsFn func(userID, investmentID uint, page pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error)
}

func (m *mockInvestmentService) AddInvestment(userID, accountID, securityID uint, quantity float64, purchasePrice int64, walletAddress string) (*models.Investment, error) {
	if m.addInvestmentFn != nil {
		return m.addInvestmentFn(userID, accountID, securityID, quantity, purchasePrice, walletAddress)
	}
	return &models.Investment{}, nil
}

func (m *mockInvestmentService) GetAccountInvestments(userID, accountID uint, page pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
	if m.getAccountInvestmentsFn != nil {
		return m.getAccountInvestmentsFn(userID, accountID, page)
	}
	resp := pagination.NewPageResponse([]models.Investment{}, 1, 20, 0)
	return &resp, nil
}

func (m *mockInvestmentService) GetInvestmentByID(userID, investmentID uint) (*models.Investment, error) {
	if m.getInvestmentByIDFn != nil {
		return m.getInvestmentByIDFn(userID, investmentID)
	}
	return &models.Investment{}, nil
}

func (m *mockInvestmentService) UpdateInvestmentPrice(userID, investmentID uint, currentPrice int64) (*models.Investment, error) {
	if m.updateInvestmentPriceFn != nil {
		return m.updateInvestmentPriceFn(userID, investmentID, currentPrice)
	}
	return &models.Investment{}, nil
}

func (m *mockInvestmentService) GetPortfolio(userID uint) (*services.PortfolioSummary, error) {
	if m.getPortfolioFn != nil {
		return m.getPortfolioFn(userID)
	}
	return &services.PortfolioSummary{HoldingsByType: map[models.AssetType]services.TypeSummary{}}, nil
}

func (m *mockInvestmentService) RecordBuy(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit, fee int64, notes string) (*models.InvestmentTransaction, error) {
	if m.recordBuyFn != nil {
		return m.recordBuyFn(userID, investmentID, date, quantity, pricePerUnit, fee, notes)
	}
	return &models.InvestmentTransaction{}, nil
}

func (m *mockInvestmentService) RecordSell(userID, investmentID uint, date time.Time, quantity float64, pricePerUnit, fee int64, notes string) (*models.InvestmentTransaction, error) {
	if m.recordSellFn != nil {
		return m.recordSellFn(userID, investmentID, date, quantity, pricePerUnit, fee, notes)
	}
	return &models.InvestmentTransaction{}, nil
}

func (m *mockInvestmentService) RecordDividend(userID, investmentID uint, date time.Time, amount int64, dividendType, notes string) (*models.InvestmentTransaction, error) {
	if m.recordDividendFn != nil {
		return m.recordDividendFn(userID, investmentID, date, amount, dividendType, notes)
	}
	return &models.InvestmentTransaction{}, nil
}

func (m *mockInvestmentService) RecordSplit(userID, investmentID uint, date time.Time, splitRatio float64, notes string) (*models.InvestmentTransaction, error) {
	if m.recordSplitFn != nil {
		return m.recordSplitFn(userID, investmentID, date, splitRatio, notes)
	}
	return &models.InvestmentTransaction{}, nil
}

func (m *mockInvestmentService) GetInvestmentTransactions(userID, investmentID uint, page pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error) {
	if m.getInvestmentTransactionsFn != nil {
		return m.getInvestmentTransactionsFn(userID, investmentID, page)
	}
	resp := pagination.NewPageResponse([]models.InvestmentTransaction{}, 1, 20, 0)
	return &resp, nil
}

var _ services.InvestmentServicer = (*mockInvestmentService)(nil)

func setupInvestmentRouter(handler *InvestmentHandler) *gin.Engine {
	r := gin.New()
	auth := r.Group("", injectUserID(1))
	auth.POST("/investments", handler.AddInvestment)
	auth.GET("/investments/portfolio", handler.GetPortfolio)
	auth.GET("/investments/:id", handler.GetInvestment)
	auth.PUT("/investments/:id/price", handler.UpdatePrice)
	auth.POST("/investments/:id/buy", handler.RecordBuy)
	auth.POST("/investments/:id/sell", handler.RecordSell)
	auth.POST("/investments/:id/dividend", handler.RecordDividend)
	auth.POST("/investments/:id/split", handler.RecordSplit)
	auth.GET("/investments/:id/transactions", handler.GetInvestmentTransactions)
	auth.GET("/accounts/:id/investments", handler.GetAccountInvestments)
	return r
}

func TestInvestmentHandler_AddInvestment(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			addInvestmentFn: func(_ uint, accountID, securityID uint, quantity float64, price int64, _ string) (*models.Investment, error) {
				return &models.Investment{
					Base:         models.Base{ID: 1},
					AccountID:    accountID,
					SecurityID:   securityID,
					Quantity:     quantity,
					CurrentPrice: price,
					CostBasis:    int64(quantity * float64(price)),
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments",
			`{"account_id":1,"security_id":1,"quantity":10,"purchase_price":15000}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		inv := result["investment"].(map[string]interface{})
		if inv["quantity"].(float64) != 10 {
			t.Errorf("expected quantity=10, got %v", inv["quantity"])
		}
	})

	t.Run("returns 400 on missing security_id", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments",
			`{"account_id":1,"quantity":10,"purchase_price":15000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "INVALID_INPUT")
	})

	t.Run("returns 400 on zero quantity", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments",
			`{"account_id":1,"security_id":1,"quantity":0,"purchase_price":15000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 on invalid account", func(t *testing.T) {
		svc := &mockInvestmentService{
			addInvestmentFn: func(_, _, _ uint, _ float64, _ int64, _ string) (*models.Investment, error) {
				return nil, apperrors.ErrAccountNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments",
			`{"account_id":999,"security_id":1,"quantity":10,"purchase_price":15000}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "ACCOUNT_NOT_FOUND")
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := gin.New()
		r.POST("/investments", handler.AddInvestment)

		rec := doRequest(r, "POST", "/investments",
			`{"account_id":1,"security_id":1,"quantity":10,"purchase_price":15000}`)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_GetInvestment(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			getInvestmentByIDFn: func(_, investmentID uint) (*models.Investment, error) {
				return &models.Investment{
					Base:       models.Base{ID: investmentID},
					SecurityID: 1,
					Quantity:   10,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/1", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		result := parseJSON(t, rec)
		inv := result["investment"].(map[string]interface{})
		if inv["security_id"].(float64) != 1 {
			t.Errorf("expected security_id=1, got %v", inv["security_id"])
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			getInvestmentByIDFn: func(_, _ uint) (*models.Investment, error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/999", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "INVESTMENT_NOT_FOUND")
	})

	t.Run("returns 400 on invalid ID", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/abc", "")

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_UpdatePrice(t *testing.T) {
	t.Run("returns 200 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			updateInvestmentPriceFn: func(_, investmentID uint, price int64) (*models.Investment, error) {
				return &models.Investment{
					Base:         models.Base{ID: investmentID},
					CurrentPrice: price,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "PUT", "/investments/1/price", `{"current_price":17500}`)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		inv := result["investment"].(map[string]interface{})
		if inv["current_price"].(float64) != 17500 {
			t.Errorf("expected current_price=17500, got %v", inv["current_price"])
		}
	})

	t.Run("returns 400 on zero price", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "PUT", "/investments/1/price", `{"current_price":0}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			updateInvestmentPriceFn: func(_, _ uint, _ int64) (*models.Investment, error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "PUT", "/investments/999/price", `{"current_price":17500}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_GetPortfolio(t *testing.T) {
	t.Run("returns 200 with portfolio summary", func(t *testing.T) {
		svc := &mockInvestmentService{
			getPortfolioFn: func(_ uint) (*services.PortfolioSummary, error) {
				return &services.PortfolioSummary{
					TotalValue:     500000,
					TotalCostBasis: 400000,
					TotalGainLoss:  100000,
					GainLossPct:    25.0,
					HoldingsByType: map[models.AssetType]services.TypeSummary{
						models.AssetTypeStock: {Value: 300000, Count: 2},
						models.AssetTypeETF:   {Value: 200000, Count: 1},
					},
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/portfolio", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		portfolio := result["portfolio"].(map[string]interface{})
		if portfolio["total_value"].(float64) != 500000 {
			t.Errorf("expected total_value=500000, got %v", portfolio["total_value"])
		}
		if portfolio["gain_loss_pct"].(float64) != 25.0 {
			t.Errorf("expected gain_loss_pct=25, got %v", portfolio["gain_loss_pct"])
		}
	})

	t.Run("returns 401 without auth", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := gin.New()
		r.GET("/investments/portfolio", handler.GetPortfolio)

		rec := doRequest(r, "GET", "/investments/portfolio", "")

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_RecordBuy(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordBuyFn: func(_, investmentID uint, _ time.Time, qty float64, price int64, fee int64, notes string) (*models.InvestmentTransaction, error) {
				return &models.InvestmentTransaction{
					Base:         models.Base{ID: 1},
					InvestmentID: investmentID,
					Type:         models.InvestmentTransactionBuy,
					Quantity:     qty,
					PricePerUnit: price,
					Fee:          fee,
					Notes:        notes,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/buy",
			`{"date":"2025-01-15T00:00:00Z","quantity":5,"price_per_unit":15000,"fee":999,"notes":"Buy more"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		tx := result["transaction"].(map[string]interface{})
		if tx["type"] != "buy" {
			t.Errorf("expected type=buy, got %v", tx["type"])
		}
		if tx["quantity"].(float64) != 5 {
			t.Errorf("expected quantity=5, got %v", tx["quantity"])
		}
	})

	t.Run("returns 400 on missing date", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/buy",
			`{"quantity":5,"price_per_unit":15000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 400 on zero quantity", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/buy",
			`{"date":"2025-01-15T00:00:00Z","quantity":0,"price_per_unit":15000}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when investment not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordBuyFn: func(_, _ uint, _ time.Time, _ float64, _ int64, _ int64, _ string) (*models.InvestmentTransaction, error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/999/buy",
			`{"date":"2025-01-15T00:00:00Z","quantity":5,"price_per_unit":15000}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_RecordSell(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordSellFn: func(_, investmentID uint, _ time.Time, qty float64, price int64, _ int64, _ string) (*models.InvestmentTransaction, error) {
				return &models.InvestmentTransaction{
					Base:         models.Base{ID: 2},
					InvestmentID: investmentID,
					Type:         models.InvestmentTransactionSell,
					Quantity:     qty,
					PricePerUnit: price,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/sell",
			`{"date":"2025-02-01T00:00:00Z","quantity":3,"price_per_unit":17500}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		tx := result["transaction"].(map[string]interface{})
		if tx["type"] != "sell" {
			t.Errorf("expected type=sell, got %v", tx["type"])
		}
	})

	t.Run("returns 400 on insufficient shares", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordSellFn: func(_, _ uint, _ time.Time, _ float64, _ int64, _ int64, _ string) (*models.InvestmentTransaction, error) {
				return nil, apperrors.ErrInsufficientShares
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/sell",
			`{"date":"2025-02-01T00:00:00Z","quantity":100,"price_per_unit":17500}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "INSUFFICIENT_SHARES")
	})
}

func TestInvestmentHandler_RecordDividend(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordDividendFn: func(_, investmentID uint, _ time.Time, amount int64, divType, _ string) (*models.InvestmentTransaction, error) {
				return &models.InvestmentTransaction{
					Base:         models.Base{ID: 3},
					InvestmentID: investmentID,
					Type:         models.InvestmentTransactionDividend,
					TotalAmount:  amount,
					DividendType: divType,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/dividend",
			`{"date":"2025-03-15T00:00:00Z","amount":500,"dividend_type":"Cash"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		tx := result["transaction"].(map[string]interface{})
		if tx["type"] != "dividend" {
			t.Errorf("expected type=dividend, got %v", tx["type"])
		}
		if tx["total_amount"].(float64) != 500 {
			t.Errorf("expected total_amount=500, got %v", tx["total_amount"])
		}
	})

	t.Run("returns 400 on zero amount", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/dividend",
			`{"date":"2025-03-15T00:00:00Z","amount":0}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordDividendFn: func(_, _ uint, _ time.Time, _ int64, _, _ string) (*models.InvestmentTransaction, error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/999/dividend",
			`{"date":"2025-03-15T00:00:00Z","amount":500}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_RecordSplit(t *testing.T) {
	t.Run("returns 201 on success", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordSplitFn: func(_, investmentID uint, _ time.Time, ratio float64, _ string) (*models.InvestmentTransaction, error) {
				return &models.InvestmentTransaction{
					Base:         models.Base{ID: 4},
					InvestmentID: investmentID,
					Type:         models.InvestmentTransactionSplit,
					SplitRatio:   ratio,
				}, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/split",
			`{"date":"2025-06-01T00:00:00Z","split_ratio":2.0,"notes":"2:1 split"}`)

		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		tx := result["transaction"].(map[string]interface{})
		if tx["type"] != "split" {
			t.Errorf("expected type=split, got %v", tx["type"])
		}
		if tx["split_ratio"].(float64) != 2.0 {
			t.Errorf("expected split_ratio=2.0, got %v", tx["split_ratio"])
		}
	})

	t.Run("returns 400 on zero split ratio", func(t *testing.T) {
		handler := NewInvestmentHandler(&mockInvestmentService{}, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/1/split",
			`{"date":"2025-06-01T00:00:00Z","split_ratio":0}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			recordSplitFn: func(_, _ uint, _ time.Time, _ float64, _ string) (*models.InvestmentTransaction, error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "POST", "/investments/999/split",
			`{"date":"2025-06-01T00:00:00Z","split_ratio":2.0}`)

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

func TestInvestmentHandler_GetAccountInvestments(t *testing.T) {
	t.Run("returns 200 with paginated investments", func(t *testing.T) {
		svc := &mockInvestmentService{
			getAccountInvestmentsFn: func(_, _ uint, _ pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
				resp := pagination.NewPageResponse([]models.Investment{
					{Base: models.Base{ID: 1}, SecurityID: 1},
					{Base: models.Base{ID: 2}, SecurityID: 2},
				}, 1, 20, 2)
				return &resp, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/accounts/1/investments", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		data := result["data"].([]interface{})
		if len(data) != 2 {
			t.Errorf("expected 2 investments, got %d", len(data))
		}
		if result["total_items"].(float64) != 2 {
			t.Errorf("expected total_items=2, got %v", result["total_items"])
		}
	})

	t.Run("returns 404 on invalid account", func(t *testing.T) {
		svc := &mockInvestmentService{
			getAccountInvestmentsFn: func(_, _ uint, _ pagination.PageRequest) (*pagination.PageResponse[models.Investment], error) {
				return nil, apperrors.ErrAccountNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/accounts/999/investments", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		assertErrorCode(t, parseJSON(t, rec), "ACCOUNT_NOT_FOUND")
	})
}

func TestInvestmentHandler_GetInvestmentTransactions(t *testing.T) {
	t.Run("returns 200 with paginated transactions", func(t *testing.T) {
		svc := &mockInvestmentService{
			getInvestmentTransactionsFn: func(_, _ uint, _ pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error) {
				resp := pagination.NewPageResponse([]models.InvestmentTransaction{
					{Base: models.Base{ID: 1}, Type: models.InvestmentTransactionBuy},
					{Base: models.Base{ID: 2}, Type: models.InvestmentTransactionDividend},
				}, 1, 20, 2)
				return &resp, nil
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/1/transactions", "")

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
		result := parseJSON(t, rec)
		data := result["data"].([]interface{})
		if len(data) != 2 {
			t.Errorf("expected 2 transactions, got %d", len(data))
		}
	})

	t.Run("returns 404 when not found", func(t *testing.T) {
		svc := &mockInvestmentService{
			getInvestmentTransactionsFn: func(_, _ uint, _ pagination.PageRequest) (*pagination.PageResponse[models.InvestmentTransaction], error) {
				return nil, apperrors.ErrInvestmentNotFound
			},
		}
		handler := NewInvestmentHandler(svc, &mockAuditService{})
		r := setupInvestmentRouter(handler)

		rec := doRequest(r, "GET", "/investments/999/transactions", "")

		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
	})
}

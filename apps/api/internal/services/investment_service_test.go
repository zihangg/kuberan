package services

import (
	"testing"
	"time"

	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/testutil"
)

func TestAddInvestment(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)

		inv, err := svc.AddInvestment(user.ID, account.ID, "AAPL", "Apple Inc", models.AssetTypeStock, 10.0, 15000, "USD", nil)
		testutil.AssertNoError(t, err)

		if inv.ID == 0 {
			t.Fatal("expected non-zero investment ID")
		}
		if inv.Symbol != "AAPL" {
			t.Errorf("expected symbol AAPL, got %s", inv.Symbol)
		}
		if inv.Quantity != 10.0 {
			t.Errorf("expected quantity 10.0, got %f", inv.Quantity)
		}
		// CostBasis = 10 * 15000 = 150000
		if inv.CostBasis != 150000 {
			t.Errorf("expected cost basis 150000, got %d", inv.CostBasis)
		}
		if inv.CurrentPrice != 15000 {
			t.Errorf("expected current price 15000, got %d", inv.CurrentPrice)
		}

		// Verify initial buy transaction was created
		var txCount int64
		db.Model(&models.InvestmentTransaction{}).Where("investment_id = ?", inv.ID).Count(&txCount)
		if txCount != 1 {
			t.Errorf("expected 1 initial buy transaction, got %d", txCount)
		}

		var buyTx models.InvestmentTransaction
		db.Where("investment_id = ?", inv.ID).First(&buyTx)
		if buyTx.Type != models.InvestmentTransactionBuy {
			t.Errorf("expected buy transaction, got %s", buyTx.Type)
		}
		if buyTx.Quantity != 10.0 {
			t.Errorf("expected buy quantity 10.0, got %f", buyTx.Quantity)
		}
		if buyTx.TotalAmount != 150000 {
			t.Errorf("expected buy total 150000, got %d", buyTx.TotalAmount)
		}
	})

	t.Run("with_extra_fields", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)

		extras := map[string]interface{}{
			"exchange": "NASDAQ",
		}
		inv, err := svc.AddInvestment(user.ID, account.ID, "MSFT", "Microsoft", models.AssetTypeStock, 5.0, 30000, "", extras)
		testutil.AssertNoError(t, err)

		if inv.Exchange != "NASDAQ" {
			t.Errorf("expected exchange NASDAQ, got %s", inv.Exchange)
		}
		// Empty currency should inherit from account
		if inv.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", inv.Currency)
		}
	})

	t.Run("not_investment_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		cashAcct := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := svc.AddInvestment(user.ID, cashAcct.ID, "AAPL", "Apple", models.AssetTypeStock, 10.0, 15000, "USD", nil)
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("invalid_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.AddInvestment(user.ID, 9999, "AAPL", "Apple", models.AssetTypeStock, 10.0, 15000, "USD", nil)
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})
}

func TestGetInvestmentByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID)

		result, err := svc.GetInvestmentByID(user.ID, inv.ID)
		testutil.AssertNoError(t, err)

		if result.ID != inv.ID {
			t.Errorf("expected ID %d, got %d", inv.ID, result.ID)
		}
		if result.Symbol != inv.Symbol {
			t.Errorf("expected symbol %s, got %s", inv.Symbol, result.Symbol)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.GetInvestmentByID(user.ID, 9999)
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})

	t.Run("wrong_user", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user1.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID)

		_, err := svc.GetInvestmentByID(user2.ID, inv.ID)
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

func TestGetAccountInvestments(t *testing.T) {
	t.Run("returns_investments", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		testutil.CreateTestInvestment(t, db, account.ID)
		testutil.CreateTestInvestment(t, db, account.ID)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := svc.GetAccountInvestments(user.ID, account.ID, page)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 2 {
			t.Errorf("expected 2 investments, got %d", result.TotalItems)
		}
		if len(result.Data) != 2 {
			t.Errorf("expected 2 items in data, got %d", len(result.Data))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		for i := 0; i < 5; i++ {
			testutil.CreateTestInvestment(t, db, account.ID)
		}

		page := pagination.PageRequest{Page: 1, PageSize: 2}
		result, err := svc.GetAccountInvestments(user.ID, account.ID, page)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 5 {
			t.Errorf("expected total 5, got %d", result.TotalItems)
		}
		if result.TotalPages != 3 {
			t.Errorf("expected 3 total pages, got %d", result.TotalPages)
		}
		if len(result.Data) != 2 {
			t.Errorf("expected 2 items on page, got %d", len(result.Data))
		}
	})

	t.Run("invalid_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		_, err := svc.GetAccountInvestments(user.ID, 9999, page)
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})
}

func TestUpdateInvestmentPrice(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID)

		beforeUpdate := time.Now()
		updated, err := svc.UpdateInvestmentPrice(user.ID, inv.ID, 12500)
		testutil.AssertNoError(t, err)

		if updated.CurrentPrice != 12500 {
			t.Errorf("expected price 12500, got %d", updated.CurrentPrice)
		}
		if updated.LastUpdated.Before(beforeUpdate) {
			t.Error("expected last_updated to be updated")
		}

		// Verify in DB
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		if dbInv.CurrentPrice != 12500 {
			t.Errorf("expected DB price 12500, got %d", dbInv.CurrentPrice)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.UpdateInvestmentPrice(user.ID, 9999, 12500)
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

func TestRecordBuy(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares @ $100, cost basis $1000

		buyTx, err := svc.RecordBuy(user.ID, inv.ID, time.Now(), 5.0, 10000, 500, "Buy more")
		testutil.AssertNoError(t, err)

		if buyTx.Type != models.InvestmentTransactionBuy {
			t.Errorf("expected type buy, got %s", buyTx.Type)
		}
		if buyTx.Quantity != 5.0 {
			t.Errorf("expected quantity 5.0, got %f", buyTx.Quantity)
		}
		// TotalAmount = 5 * 10000 + 500 = 50500
		if buyTx.TotalAmount != 50500 {
			t.Errorf("expected total 50500, got %d", buyTx.TotalAmount)
		}
		if buyTx.Fee != 500 {
			t.Errorf("expected fee 500, got %d", buyTx.Fee)
		}

		// Verify investment updated in DB
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		// 10 + 5 = 15 shares
		if dbInv.Quantity != 15.0 {
			t.Errorf("expected quantity 15.0, got %f", dbInv.Quantity)
		}
		// 100000 + 50500 = 150500 cents cost basis
		if dbInv.CostBasis != 150500 {
			t.Errorf("expected cost basis 150500, got %d", dbInv.CostBasis)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.RecordBuy(user.ID, 9999, time.Now(), 5.0, 10000, 0, "")
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

func TestRecordSell(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares, cost basis 100000

		sellTx, err := svc.RecordSell(user.ID, inv.ID, time.Now(), 4.0, 12000, 300, "Sell some")
		testutil.AssertNoError(t, err)

		if sellTx.Type != models.InvestmentTransactionSell {
			t.Errorf("expected type sell, got %s", sellTx.Type)
		}
		if sellTx.Quantity != 4.0 {
			t.Errorf("expected quantity 4.0, got %f", sellTx.Quantity)
		}
		// TotalAmount = 4 * 12000 - 300 = 47700
		if sellTx.TotalAmount != 47700 {
			t.Errorf("expected total 47700, got %d", sellTx.TotalAmount)
		}

		// Verify investment updated in DB
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		// 10 - 4 = 6 shares remaining
		if dbInv.Quantity != 6.0 {
			t.Errorf("expected quantity 6.0, got %f", dbInv.Quantity)
		}
		// Proportional reduction: 100000 * (4/10) = 40000 removed, leaving 60000
		if dbInv.CostBasis != 60000 {
			t.Errorf("expected cost basis 60000, got %d", dbInv.CostBasis)
		}
	})

	t.Run("insufficient_shares", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares

		_, err := svc.RecordSell(user.ID, inv.ID, time.Now(), 15.0, 12000, 0, "Too many")
		testutil.AssertAppError(t, err, "INSUFFICIENT_SHARES")

		// Verify quantity unchanged
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		if dbInv.Quantity != 10.0 {
			t.Errorf("expected quantity unchanged at 10.0, got %f", dbInv.Quantity)
		}
	})

	t.Run("sell_all_shares", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares, cost basis 100000

		_, err := svc.RecordSell(user.ID, inv.ID, time.Now(), 10.0, 12000, 0, "Sell all")
		testutil.AssertNoError(t, err)

		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		if dbInv.Quantity != 0.0 {
			t.Errorf("expected quantity 0.0, got %f", dbInv.Quantity)
		}
		if dbInv.CostBasis != 0 {
			t.Errorf("expected cost basis 0, got %d", dbInv.CostBasis)
		}
	})
}

func TestRecordDividend(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares, cost basis 100000

		divTx, err := svc.RecordDividend(user.ID, inv.ID, time.Now(), 5000, "Cash", "Q4 dividend")
		testutil.AssertNoError(t, err)

		if divTx.Type != models.InvestmentTransactionDividend {
			t.Errorf("expected type dividend, got %s", divTx.Type)
		}
		if divTx.TotalAmount != 5000 {
			t.Errorf("expected total 5000, got %d", divTx.TotalAmount)
		}
		if divTx.DividendType != "Cash" {
			t.Errorf("expected dividend type Cash, got %s", divTx.DividendType)
		}

		// Verify investment quantity and cost basis unchanged
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		if dbInv.Quantity != 10.0 {
			t.Errorf("expected quantity unchanged at 10.0, got %f", dbInv.Quantity)
		}
		if dbInv.CostBasis != 100000 {
			t.Errorf("expected cost basis unchanged at 100000, got %d", dbInv.CostBasis)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.RecordDividend(user.ID, 9999, time.Now(), 5000, "Cash", "")
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

func TestRecordSplit(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID) // 10 shares, cost basis 100000

		splitTx, err := svc.RecordSplit(user.ID, inv.ID, time.Now(), 2.0, "2-for-1 split")
		testutil.AssertNoError(t, err)

		if splitTx.Type != models.InvestmentTransactionSplit {
			t.Errorf("expected type split, got %s", splitTx.Type)
		}
		if splitTx.SplitRatio != 2.0 {
			t.Errorf("expected split ratio 2.0, got %f", splitTx.SplitRatio)
		}

		// Verify quantity doubled, cost basis unchanged
		var dbInv models.Investment
		db.First(&dbInv, inv.ID)
		if dbInv.Quantity != 20.0 {
			t.Errorf("expected quantity 20.0 after 2:1 split, got %f", dbInv.Quantity)
		}
		if dbInv.CostBasis != 100000 {
			t.Errorf("expected cost basis unchanged at 100000, got %d", dbInv.CostBasis)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.RecordSplit(user.ID, 9999, time.Now(), 2.0, "")
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

func TestGetPortfolio(t *testing.T) {
	t.Run("aggregation", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		// Create two investment accounts
		acct1 := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		acct2 := testutil.CreateTestInvestmentAccount(t, db, user.ID)

		// Account 1: stock - 10 shares @ $100 current, cost basis $1000
		testutil.CreateTestInvestment(t, db, acct1.ID) // stock, 10 qty, cost 100000, price 10000

		// Account 2: create an ETF manually
		etfInv := &models.Investment{
			AccountID:    acct2.ID,
			Symbol:       "VTI",
			AssetType:    models.AssetTypeETF,
			Name:         "Vanguard Total",
			Quantity:     20.0,
			CostBasis:    200000, // $2000
			CurrentPrice: 12000,  // $120 per share
			LastUpdated:  time.Now(),
			Currency:     "USD",
		}
		if err := db.Create(etfInv).Error; err != nil {
			t.Fatalf("failed to create ETF investment: %v", err)
		}

		portfolio, err := svc.GetPortfolio(user.ID)
		testutil.AssertNoError(t, err)

		// Stock value: 10 * 10000 = 100000
		// ETF value: 20 * 12000 = 240000
		// Total value: 340000
		expectedTotalValue := int64(340000)
		if portfolio.TotalValue != expectedTotalValue {
			t.Errorf("expected total value %d, got %d", expectedTotalValue, portfolio.TotalValue)
		}

		// Total cost basis: 100000 + 200000 = 300000
		expectedCostBasis := int64(300000)
		if portfolio.TotalCostBasis != expectedCostBasis {
			t.Errorf("expected total cost basis %d, got %d", expectedCostBasis, portfolio.TotalCostBasis)
		}

		// Gain/loss: 340000 - 300000 = 40000
		expectedGainLoss := int64(40000)
		if portfolio.TotalGainLoss != expectedGainLoss {
			t.Errorf("expected gain/loss %d, got %d", expectedGainLoss, portfolio.TotalGainLoss)
		}

		// Gain/loss %: 40000 / 300000 * 100 ~= 13.33%
		expectedPct := float64(40000) / float64(300000) * 100
		if portfolio.GainLossPct < expectedPct-0.01 || portfolio.GainLossPct > expectedPct+0.01 {
			t.Errorf("expected gain/loss pct ~%.2f, got %.2f", expectedPct, portfolio.GainLossPct)
		}

		// Holdings by type
		stockSummary, ok := portfolio.HoldingsByType[models.AssetTypeStock]
		if !ok {
			t.Fatal("expected stock type in holdings")
		}
		if stockSummary.Count != 1 {
			t.Errorf("expected 1 stock holding, got %d", stockSummary.Count)
		}
		if stockSummary.Value != 100000 {
			t.Errorf("expected stock value 100000, got %d", stockSummary.Value)
		}

		etfSummary, ok := portfolio.HoldingsByType[models.AssetTypeETF]
		if !ok {
			t.Fatal("expected ETF type in holdings")
		}
		if etfSummary.Count != 1 {
			t.Errorf("expected 1 ETF holding, got %d", etfSummary.Count)
		}
		if etfSummary.Value != 240000 {
			t.Errorf("expected ETF value 240000, got %d", etfSummary.Value)
		}
	})

	t.Run("no_investments", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		portfolio, err := svc.GetPortfolio(user.ID)
		testutil.AssertNoError(t, err)

		if portfolio.TotalValue != 0 {
			t.Errorf("expected total value 0, got %d", portfolio.TotalValue)
		}
		if portfolio.TotalCostBasis != 0 {
			t.Errorf("expected cost basis 0, got %d", portfolio.TotalCostBasis)
		}
		if len(portfolio.HoldingsByType) != 0 {
			t.Errorf("expected empty holdings map, got %d entries", len(portfolio.HoldingsByType))
		}
	})

	t.Run("user_isolation", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)

		acct1 := testutil.CreateTestInvestmentAccount(t, db, user1.ID)
		testutil.CreateTestInvestment(t, db, acct1.ID)

		acct2 := testutil.CreateTestInvestmentAccount(t, db, user2.ID)
		testutil.CreateTestInvestment(t, db, acct2.ID)

		portfolio1, err := svc.GetPortfolio(user1.ID)
		testutil.AssertNoError(t, err)

		// Each user has 1 investment with 10 shares @ $100 = $1000 value
		if portfolio1.TotalValue != 100000 {
			t.Errorf("expected user1 total value 100000, got %d", portfolio1.TotalValue)
		}

		portfolio2, err := svc.GetPortfolio(user2.ID)
		testutil.AssertNoError(t, err)
		if portfolio2.TotalValue != 100000 {
			t.Errorf("expected user2 total value 100000, got %d", portfolio2.TotalValue)
		}
	})
}

func TestGetInvestmentTransactions(t *testing.T) {
	t.Run("returns_transactions", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestInvestmentAccount(t, db, user.ID)
		inv := testutil.CreateTestInvestment(t, db, account.ID)

		// Record some transactions
		_, err := svc.RecordBuy(user.ID, inv.ID, time.Now(), 5.0, 10000, 0, "Buy 1")
		testutil.AssertNoError(t, err)
		_, err = svc.RecordDividend(user.ID, inv.ID, time.Now(), 2000, "Cash", "Div")
		testutil.AssertNoError(t, err)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := svc.GetInvestmentTransactions(user.ID, inv.ID, page)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 2 {
			t.Errorf("expected 2 transactions, got %d", result.TotalItems)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		svc := NewInvestmentService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		_, err := svc.GetInvestmentTransactions(user.ID, 9999, page)
		testutil.AssertAppError(t, err, "INVESTMENT_NOT_FOUND")
	})
}

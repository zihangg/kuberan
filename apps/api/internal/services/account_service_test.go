package services

import (
	"testing"

	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/testutil"
)

func TestCreateCashAccount(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		account, err := svc.CreateCashAccount(user.ID, "Savings", "My savings", "USD", 0)
		testutil.AssertNoError(t, err)

		if account.ID == 0 {
			t.Fatal("expected non-zero account ID")
		}
		if account.Name != "Savings" {
			t.Errorf("expected name Savings, got %s", account.Name)
		}
		if account.Type != models.AccountTypeCash {
			t.Errorf("expected type cash, got %s", account.Type)
		}
		if account.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", account.Currency)
		}
		if !account.IsActive {
			t.Error("expected account to be active")
		}
	})

	t.Run("with_initial_balance", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		account, err := svc.CreateCashAccount(user.ID, "Checking", "", "USD", 5000)
		testutil.AssertNoError(t, err)

		if account.Balance != 5000 {
			t.Errorf("expected balance 5000, got %d", account.Balance)
		}

		// Verify initial transaction was created atomically
		var txCount int64
		db.Model(&models.Transaction{}).Where("account_id = ?", account.ID).Count(&txCount)
		if txCount != 1 {
			t.Errorf("expected 1 initial transaction, got %d", txCount)
		}

		var tx models.Transaction
		db.Where("account_id = ?", account.ID).First(&tx)
		if tx.Type != models.TransactionTypeIncome {
			t.Errorf("expected initial transaction type income, got %s", tx.Type)
		}
		if tx.Amount != 5000 {
			t.Errorf("expected initial transaction amount 5000, got %d", tx.Amount)
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.CreateCashAccount(user.ID, "", "", "USD", 0)
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("default_currency", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		account, err := svc.CreateCashAccount(user.ID, "No Currency", "", "", 0)
		testutil.AssertNoError(t, err)

		if account.Currency != "USD" {
			t.Errorf("expected default currency USD, got %s", account.Currency)
		}
	})
}

func TestGetUserAccounts(t *testing.T) {
	t.Run("returns_user_accounts_only", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)

		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)

		testutil.CreateTestCashAccount(t, db, user1.ID)
		testutil.CreateTestCashAccount(t, db, user1.ID)
		testutil.CreateTestCashAccount(t, db, user2.ID)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := svc.GetUserAccounts(user1.ID, page)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 2 {
			t.Errorf("expected 2 accounts for user1, got %d", result.TotalItems)
		}
		if len(result.Data) != 2 {
			t.Errorf("expected 2 accounts in data, got %d", len(result.Data))
		}
	})

	t.Run("excludes_inactive", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		active := testutil.CreateTestCashAccount(t, db, user.ID)
		inactive := testutil.CreateTestCashAccount(t, db, user.ID)
		db.Model(inactive).Update("is_active", false)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := svc.GetUserAccounts(user.ID, page)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 1 {
			t.Errorf("expected 1 active account, got %d", result.TotalItems)
		}
		if result.Data[0].ID != active.ID {
			t.Errorf("expected active account ID %d, got %d", active.ID, result.Data[0].ID)
		}
	})
}

func TestGetAccountByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)
		created := testutil.CreateTestCashAccount(t, db, user.ID)

		account, err := svc.GetAccountByID(user.ID, created.ID)
		testutil.AssertNoError(t, err)

		if account.ID != created.ID {
			t.Errorf("expected account ID %d, got %d", created.ID, account.ID)
		}
		if account.Name != created.Name {
			t.Errorf("expected name %s, got %s", created.Name, account.Name)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.GetAccountByID(user.ID, 99999)
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})

	t.Run("wrong_user", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)

		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user1.ID)

		_, err := svc.GetAccountByID(user2.ID, account.ID)
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})
}

func TestUpdateCashAccount(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		updated, err := svc.UpdateCashAccount(user.ID, account.ID, "New Name", "New Description")
		testutil.AssertNoError(t, err)

		if updated.Name != "New Name" {
			t.Errorf("expected name 'New Name', got %s", updated.Name)
		}
		if updated.Description != "New Description" {
			t.Errorf("expected description 'New Description', got %s", updated.Description)
		}
	})

	t.Run("not_cash_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)
		investmentAccount := testutil.CreateTestInvestmentAccount(t, db, user.ID)

		_, err := svc.UpdateCashAccount(user.ID, investmentAccount.ID, "Name", "Desc")
		testutil.AssertAppError(t, err, "NOT_CASH_ACCOUNT")
	})
}

func TestUpdateAccountBalance(t *testing.T) {
	t.Run("income_adds", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 1000)

		err := svc.UpdateAccountBalance(db, account, models.TransactionTypeIncome, 500)
		testutil.AssertNoError(t, err)

		if account.Balance != 1500 {
			t.Errorf("expected balance 1500 after income, got %d", account.Balance)
		}

		// Verify persisted to DB
		var dbAccount models.Account
		db.First(&dbAccount, account.ID)
		if dbAccount.Balance != 1500 {
			t.Errorf("expected DB balance 1500, got %d", dbAccount.Balance)
		}
	})

	t.Run("expense_subtracts", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 1000)

		err := svc.UpdateAccountBalance(db, account, models.TransactionTypeExpense, 300)
		testutil.AssertNoError(t, err)

		if account.Balance != 700 {
			t.Errorf("expected balance 700 after expense, got %d", account.Balance)
		}

		// Verify persisted to DB
		var dbAccount models.Account
		db.First(&dbAccount, account.ID)
		if dbAccount.Balance != 700 {
			t.Errorf("expected DB balance 700, got %d", dbAccount.Balance)
		}
	})
}

func TestCreateInvestmentAccount(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		account, err := svc.CreateInvestmentAccount(user.ID, "Brokerage", "My investments", "USD", "Fidelity", "123456")
		testutil.AssertNoError(t, err)

		if account.Type != models.AccountTypeInvestment {
			t.Errorf("expected type investment, got %s", account.Type)
		}
		if account.Broker != "Fidelity" {
			t.Errorf("expected broker Fidelity, got %s", account.Broker)
		}
		if account.AccountNumber != "123456" {
			t.Errorf("expected account number 123456, got %s", account.AccountNumber)
		}
	})

	t.Run("empty_name", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		_, err := svc.CreateInvestmentAccount(user.ID, "", "", "USD", "", "")
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("default_currency", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		svc := NewAccountService(db)
		user := testutil.CreateTestUser(t, db)

		account, err := svc.CreateInvestmentAccount(user.ID, "Invest", "", "", "", "")
		testutil.AssertNoError(t, err)

		if account.Currency != "USD" {
			t.Errorf("expected default currency USD, got %s", account.Currency)
		}
	})
}

package services

import (
	"testing"
	"time"

	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/testutil"
)

func TestCreateTransaction(t *testing.T) {
	t.Run("income_increases_balance", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		tx, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeIncome, 5000, "Salary", time.Now())
		testutil.AssertNoError(t, err)

		if tx.ID == 0 {
			t.Fatal("expected non-zero transaction ID")
		}
		if tx.Amount != 5000 {
			t.Errorf("expected amount 5000, got %d", tx.Amount)
		}

		// Verify balance increased
		updated, err := acctSvc.GetAccountByID(user.ID, account.ID)
		testutil.AssertNoError(t, err)
		if updated.Balance != 5000 {
			t.Errorf("expected balance 5000, got %d", updated.Balance)
		}
	})

	t.Run("expense_decreases_balance", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)

		_, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeExpense, 3000, "Lunch", time.Now())
		testutil.AssertNoError(t, err)

		updated, err := acctSvc.GetAccountByID(user.ID, account.ID)
		testutil.AssertNoError(t, err)
		if updated.Balance != 7000 {
			t.Errorf("expected balance 7000, got %d", updated.Balance)
		}
	})

	t.Run("zero_amount", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeIncome, 0, "", time.Now())
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("negative_amount", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeIncome, -100, "", time.Now())
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("zero_account_id", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)

		_, err := txSvc.CreateTransaction(1, 0, nil, models.TransactionTypeIncome, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("invalid_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := txSvc.CreateTransaction(user.ID, 99999, nil, models.TransactionTypeIncome, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})

	t.Run("wrong_user_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user1.ID)

		_, err := txSvc.CreateTransaction(user2.ID, account.ID, nil, models.TransactionTypeIncome, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})

	t.Run("with_category", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)
		cat := testutil.CreateTestCategory(t, db, user.ID, models.CategoryTypeExpense)

		tx, err := txSvc.CreateTransaction(user.ID, account.ID, &cat.ID, models.TransactionTypeExpense, 500, "Coffee", time.Now())
		testutil.AssertNoError(t, err)

		if tx.CategoryID == nil || *tx.CategoryID != cat.ID {
			t.Error("expected category ID to be set")
		}
	})

	t.Run("default_date_when_zero", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		tx, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeIncome, 1000, "", time.Time{})
		testutil.AssertNoError(t, err)

		if tx.Date.IsZero() {
			t.Error("expected date to be defaulted to now, got zero")
		}
	})
}

func TestCreateTransfer(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		from := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)
		to := testutil.CreateTestCashAccount(t, db, user.ID)

		tx, err := txSvc.CreateTransfer(user.ID, from.ID, to.ID, 3000, "Transfer", time.Now())
		testutil.AssertNoError(t, err)

		if tx.Type != models.TransactionTypeTransfer {
			t.Errorf("expected type transfer, got %s", tx.Type)
		}
		if tx.ToAccountID == nil || *tx.ToAccountID != to.ID {
			t.Error("expected ToAccountID to be set")
		}

		// Verify from-account balance decreased
		fromUpdated, err := acctSvc.GetAccountByID(user.ID, from.ID)
		testutil.AssertNoError(t, err)
		if fromUpdated.Balance != 7000 {
			t.Errorf("expected from-balance 7000, got %d", fromUpdated.Balance)
		}

		// Verify to-account balance increased
		toUpdated, err := acctSvc.GetAccountByID(user.ID, to.ID)
		testutil.AssertNoError(t, err)
		if toUpdated.Balance != 3000 {
			t.Errorf("expected to-balance 3000, got %d", toUpdated.Balance)
		}
	})

	t.Run("same_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)

		_, err := txSvc.CreateTransfer(user.ID, account.ID, account.ID, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "SAME_ACCOUNT_TRANSFER")
	})

	t.Run("insufficient_balance", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		from := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 1000)
		to := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := txSvc.CreateTransfer(user.ID, from.ID, to.ID, 5000, "", time.Now())
		testutil.AssertAppError(t, err, "INSUFFICIENT_BALANCE")

		// Verify balances unchanged
		fromUpdated, err := acctSvc.GetAccountByID(user.ID, from.ID)
		testutil.AssertNoError(t, err)
		if fromUpdated.Balance != 1000 {
			t.Errorf("expected from-balance unchanged at 1000, got %d", fromUpdated.Balance)
		}
	})

	t.Run("zero_amount", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		from := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)
		to := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := txSvc.CreateTransfer(user.ID, from.ID, to.ID, 0, "", time.Now())
		testutil.AssertAppError(t, err, "INVALID_INPUT")
	})

	t.Run("invalid_from_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		to := testutil.CreateTestCashAccount(t, db, user.ID)

		_, err := txSvc.CreateTransfer(user.ID, 99999, to.ID, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})

	t.Run("invalid_to_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		from := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)

		_, err := txSvc.CreateTransfer(user.ID, from.ID, 99999, 1000, "", time.Now())
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})
}

func TestGetTransactionByID(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)
		created := testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 1000)

		tx, err := txSvc.GetTransactionByID(user.ID, created.ID)
		testutil.AssertNoError(t, err)

		if tx.ID != created.ID {
			t.Errorf("expected transaction ID %d, got %d", created.ID, tx.ID)
		}
		if tx.Amount != 1000 {
			t.Errorf("expected amount 1000, got %d", tx.Amount)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		_, err := txSvc.GetTransactionByID(user.ID, 99999)
		testutil.AssertAppError(t, err, "TRANSACTION_NOT_FOUND")
	})

	t.Run("wrong_user", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user1.ID)
		created := testutil.CreateTestTransaction(t, db, user1.ID, account.ID, models.TransactionTypeIncome, 1000)

		_, err := txSvc.GetTransactionByID(user2.ID, created.ID)
		testutil.AssertAppError(t, err, "TRANSACTION_NOT_FOUND")
	})
}

func TestGetAccountTransactions(t *testing.T) {
	t.Run("returns_account_transactions", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 1000)
		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeExpense, 500)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := txSvc.GetAccountTransactions(user.ID, account.ID, page, TransactionFilter{})
		testutil.AssertNoError(t, err)

		if result.TotalItems != 2 {
			t.Errorf("expected 2 transactions, got %d", result.TotalItems)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		for i := 0; i < 5; i++ {
			testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, int64((i+1)*1000))
		}

		page := pagination.PageRequest{Page: 1, PageSize: 2}
		result, err := txSvc.GetAccountTransactions(user.ID, account.ID, page, TransactionFilter{})
		testutil.AssertNoError(t, err)

		if result.TotalItems != 5 {
			t.Errorf("expected total 5, got %d", result.TotalItems)
		}
		if len(result.Data) != 2 {
			t.Errorf("expected 2 items on page, got %d", len(result.Data))
		}
		if result.TotalPages != 3 {
			t.Errorf("expected 3 total pages, got %d", result.TotalPages)
		}
	})

	t.Run("filter_by_type", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 1000)
		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeExpense, 500)
		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 2000)

		incomeType := models.TransactionTypeIncome
		filter := TransactionFilter{Type: &incomeType}
		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := txSvc.GetAccountTransactions(user.ID, account.ID, page, filter)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 2 {
			t.Errorf("expected 2 income transactions, got %d", result.TotalItems)
		}
	})

	t.Run("filter_by_amount_range", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 500)
		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 1500)
		testutil.CreateTestTransaction(t, db, user.ID, account.ID, models.TransactionTypeIncome, 3000)

		minAmt := int64(1000)
		maxAmt := int64(2000)
		filter := TransactionFilter{MinAmount: &minAmt, MaxAmount: &maxAmt}
		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := txSvc.GetAccountTransactions(user.ID, account.ID, page, filter)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 1 {
			t.Errorf("expected 1 transaction in range, got %d", result.TotalItems)
		}
	})

	t.Run("filter_by_date_range", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		now := time.Now()
		old := &models.Transaction{
			UserID: user.ID, AccountID: account.ID,
			Type: models.TransactionTypeIncome, Amount: 1000,
			Date: now.AddDate(0, -2, 0),
		}
		recent := &models.Transaction{
			UserID: user.ID, AccountID: account.ID,
			Type: models.TransactionTypeIncome, Amount: 2000,
			Date: now.AddDate(0, 0, -1),
		}
		db.Create(old)
		db.Create(recent)

		fromDate := now.AddDate(0, -1, 0)
		filter := TransactionFilter{FromDate: &fromDate}
		page := pagination.PageRequest{Page: 1, PageSize: 20}
		result, err := txSvc.GetAccountTransactions(user.ID, account.ID, page, filter)
		testutil.AssertNoError(t, err)

		if result.TotalItems != 1 {
			t.Errorf("expected 1 recent transaction, got %d", result.TotalItems)
		}
	})

	t.Run("invalid_account", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		page := pagination.PageRequest{Page: 1, PageSize: 20}
		_, err := txSvc.GetAccountTransactions(user.ID, 99999, page, TransactionFilter{})
		testutil.AssertAppError(t, err, "ACCOUNT_NOT_FOUND")
	})
}

func TestDeleteTransaction(t *testing.T) {
	t.Run("income_reversal", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user.ID)

		tx, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeIncome, 5000, "Income", time.Now())
		testutil.AssertNoError(t, err)

		// Verify balance increased
		updated, _ := acctSvc.GetAccountByID(user.ID, account.ID)
		if updated.Balance != 5000 {
			t.Fatalf("expected balance 5000 after income, got %d", updated.Balance)
		}

		// Delete the income transaction
		err = txSvc.DeleteTransaction(user.ID, tx.ID)
		testutil.AssertNoError(t, err)

		// Balance should be back to 0
		updated, _ = acctSvc.GetAccountByID(user.ID, account.ID)
		if updated.Balance != 0 {
			t.Errorf("expected balance 0 after deleting income, got %d", updated.Balance)
		}
	})

	t.Run("expense_reversal", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)

		tx, err := txSvc.CreateTransaction(user.ID, account.ID, nil, models.TransactionTypeExpense, 3000, "Expense", time.Now())
		testutil.AssertNoError(t, err)

		// Verify balance decreased
		updated, _ := acctSvc.GetAccountByID(user.ID, account.ID)
		if updated.Balance != 7000 {
			t.Fatalf("expected balance 7000 after expense, got %d", updated.Balance)
		}

		// Delete the expense
		err = txSvc.DeleteTransaction(user.ID, tx.ID)
		testutil.AssertNoError(t, err)

		// Balance should be restored
		updated, _ = acctSvc.GetAccountByID(user.ID, account.ID)
		if updated.Balance != 10000 {
			t.Errorf("expected balance 10000 after deleting expense, got %d", updated.Balance)
		}
	})

	t.Run("transfer_reversal", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)
		from := testutil.CreateTestCashAccountWithBalance(t, db, user.ID, 10000)
		to := testutil.CreateTestCashAccount(t, db, user.ID)

		tx, err := txSvc.CreateTransfer(user.ID, from.ID, to.ID, 4000, "Transfer", time.Now())
		testutil.AssertNoError(t, err)

		// Verify balances after transfer
		fromUpdated, _ := acctSvc.GetAccountByID(user.ID, from.ID)
		toUpdated, _ := acctSvc.GetAccountByID(user.ID, to.ID)
		if fromUpdated.Balance != 6000 {
			t.Fatalf("expected from-balance 6000, got %d", fromUpdated.Balance)
		}
		if toUpdated.Balance != 4000 {
			t.Fatalf("expected to-balance 4000, got %d", toUpdated.Balance)
		}

		// Delete the transfer
		err = txSvc.DeleteTransaction(user.ID, tx.ID)
		testutil.AssertNoError(t, err)

		// Balances should be restored
		fromUpdated, _ = acctSvc.GetAccountByID(user.ID, from.ID)
		toUpdated, _ = acctSvc.GetAccountByID(user.ID, to.ID)
		if fromUpdated.Balance != 10000 {
			t.Errorf("expected from-balance 10000 after delete, got %d", fromUpdated.Balance)
		}
		if toUpdated.Balance != 0 {
			t.Errorf("expected to-balance 0 after delete, got %d", toUpdated.Balance)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user := testutil.CreateTestUser(t, db)

		err := txSvc.DeleteTransaction(user.ID, 99999)
		testutil.AssertAppError(t, err, "TRANSACTION_NOT_FOUND")
	})

	t.Run("wrong_user", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		defer testutil.TeardownTestDB(t, db)
		acctSvc := NewAccountService(db)
		txSvc := NewTransactionService(db, acctSvc)
		user1 := testutil.CreateTestUser(t, db)
		user2 := testutil.CreateTestUser(t, db)
		account := testutil.CreateTestCashAccount(t, db, user1.ID)

		tx, err := txSvc.CreateTransaction(user1.ID, account.ID, nil, models.TransactionTypeIncome, 1000, "", time.Now())
		testutil.AssertNoError(t, err)

		err = txSvc.DeleteTransaction(user2.ID, tx.ID)
		testutil.AssertAppError(t, err, "TRANSACTION_NOT_FOUND")
	})
}

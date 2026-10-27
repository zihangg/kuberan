package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"gorm.io/gorm"

	"kuberan/internal/models"
	"kuberan/internal/services"
	"kuberan/internal/testutil"
)

const testClientID = "claude-client"

// writeEnv is a write-enabled MCP server backed by real services on SQLite,
// with one user who has granted testClientID the given scopes.
type writeEnv struct {
	s      *Server
	db     *gorm.DB
	user   *models.User
	grants services.OAuthGrantServicer
}

func setupWriteEnv(t *testing.T, approved ...string) *writeEnv {
	t.Helper()
	db := testutil.SetupTestDB(t)
	t.Cleanup(func() { testutil.TeardownTestDB(t, db) })

	accounts := services.NewAccountService(db)
	categories := services.NewCategoryService(db)
	rules := services.NewRuleService(db, categories)
	grants := services.NewOAuthGrantService(db)
	s := newServer(Services{
		Users:        services.NewUserService(db),
		Accounts:     accounts,
		Categories:   categories,
		Transactions: services.NewTransactionService(db, accounts, rules),
		Budgets:      services.NewBudgetService(db),
		Receivables:  services.NewReceivableService(db, accounts, rules),
		Audit:        services.NewAuditService(db),
		Grants:       grants,
	}, WriteConfig{Enabled: true, MaxAmount: 1_000_000, RatePerMinute: 30, RatePerDay: 500})

	user := testutil.CreateTestUser(t, db)
	if len(approved) > 0 {
		if _, err := grants.RecordConsent(services.RecordConsentInput{
			UserID: user.ID, ClientID: testClientID, ClientName: "Claude",
			Approved: approved, Reviewed: approved,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return &writeEnv{s: s, db: db, user: user, grants: grants}
}

// ctx returns a request context for the env's user and client with the given
// token scopes.
func (e *writeEnv) ctx(scopes ...string) context.Context {
	ctx := context.WithValue(context.Background(), userIDKey{}, e.user.ID)
	ctx = context.WithValue(ctx, clientIDKey{}, testClientID)
	ctx = context.WithValue(ctx, clientIPKey{}, "203.0.113.7")
	return context.WithValue(ctx, scopesKey{}, scopes)
}

// call runs a registered tool by name.
func (e *writeEnv) call(t *testing.T, ctx context.Context, tool string, args map[string]any) *mcpgo.CallToolResult {
	t.Helper()
	st := e.s.mcp.GetTool(tool)
	if st == nil {
		t.Fatalf("tool %s is not registered", tool)
	}
	res, err := st.Handler(ctx, callReq(args))
	if err != nil {
		t.Fatalf("%s returned a Go error: %v", tool, err)
	}
	return res
}

func (e *writeEnv) auditCount(t *testing.T, action string) int64 {
	t.Helper()
	var n int64
	e.db.Model(&models.AuditLog{}).Where("action = ?", action).Count(&n)
	return n
}

func (e *writeEnv) balance(t *testing.T, accountID string) int64 {
	t.Helper()
	var a models.Account
	if err := e.db.First(&a, "id = ?", accountID).Error; err != nil {
		t.Fatal(err)
	}
	return a.Balance
}

func (e *writeEnv) txnCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	e.db.Model(&models.Transaction{}).Count(&n)
	return n
}

func wantError(t *testing.T, res *mcpgo.CallToolResult, contains string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("expected an error result containing %q, got: %s", contains, resultText(t, res))
	}
	if text := resultText(t, res); !strings.Contains(text, contains) {
		t.Fatalf("expected error containing %q, got %q", contains, text)
	}
}

func wantOK(t *testing.T, res *mcpgo.CallToolResult) string {
	t.Helper()
	text := resultText(t, res)
	if res.IsError {
		t.Fatalf("expected success, got error: %s", text)
	}
	return text
}

func TestWriteToolsRegistration(t *testing.T) {
	writeTools := []string{
		"create_transaction", "create_transfer", "update_transaction", "categorize_transactions",
		"create_category", "update_category", "create_budget", "update_budget",
		"record_repayment", "write_off_receivable",
	}

	t.Run("disabled registers no write tools", func(t *testing.T) {
		s := newServer(Services{}, WriteConfig{})
		for _, name := range writeTools {
			if s.mcp.GetTool(name) != nil {
				t.Errorf("%s must not be registered when writes are disabled", name)
			}
		}
	})

	t.Run("enabled registers write tools annotated as non-read-only", func(t *testing.T) {
		s := newServer(Services{}, WriteConfig{Enabled: true})
		for _, name := range writeTools {
			st := s.mcp.GetTool(name)
			if st == nil {
				t.Fatalf("%s is not registered", name)
			}
			a := st.Tool.Annotations
			if a.ReadOnlyHint == nil || *a.ReadOnlyHint {
				t.Errorf("%s must have readOnlyHint false", name)
			}
			if !strings.Contains(st.Tool.Description, "confirm the details with the user") {
				t.Errorf("%s description is missing the confirmation notice", name)
			}
		}
		for _, name := range []string{"update_transaction", "write_off_receivable"} {
			if a := s.mcp.GetTool(name).Tool.Annotations; a.DestructiveHint == nil || !*a.DestructiveHint {
				t.Errorf("%s must be marked destructive", name)
			}
		}
		if a := s.mcp.GetTool("create_transaction").Tool.Annotations; a.DestructiveHint == nil || *a.DestructiveHint {
			t.Error("create_transaction must not be marked destructive")
		}
	})

	t.Run("read tools are annotated read-only", func(t *testing.T) {
		s := newServer(Services{}, WriteConfig{Enabled: true})
		for name, st := range s.mcp.ListTools() {
			if strings.HasPrefix(name, "list_") || strings.HasPrefix(name, "get_") {
				if a := st.Tool.Annotations; a.ReadOnlyHint == nil || !*a.ReadOnlyHint {
					t.Errorf("%s must have readOnlyHint true", name)
				}
			}
		}
	})
}

func TestAuthorizeWrite(t *testing.T) {
	const scope = "write:transactions"

	t.Run("allows a granted scope", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		c, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1)
		if denied != nil {
			t.Fatalf("unexpected denial: %s", resultText(t, denied))
		}
		if c.UserID != e.user.ID || c.ClientID != testClientID || c.IP != "203.0.113.7" {
			t.Errorf("unexpected caller: %+v", c)
		}
	})

	t.Run("rejects a missing user", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		_, denied := e.s.authorizeWrite(context.Background(), scope, 1)
		wantError(t, denied, "unauthorized")
	})

	t.Run("rejects a token without the scope", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		_, denied := e.s.authorizeWrite(e.ctx("read:transactions"), scope, 1)
		wantError(t, denied, "missing required scope")
	})

	t.Run("rejects a token without client_id", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		ctx := context.WithValue(e.ctx(scope), clientIDKey{}, "")
		_, denied := e.s.authorizeWrite(ctx, scope, 1)
		wantError(t, denied, "no client_id")
	})

	t.Run("rejects when the grant lacks the scope", func(t *testing.T) {
		e := setupWriteEnv(t, "read:transactions")
		_, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1)
		wantError(t, denied, "not allowed to make changes")
	})

	t.Run("rejects a revoked grant even with a valid token", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		if err := e.grants.Revoke(e.user.ID, testClientID); err != nil {
			t.Fatal(err)
		}
		_, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1)
		wantError(t, denied, "not allowed to make changes")
	})

	t.Run("rejects an inactive user", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		e.db.Model(e.user).Update("is_active", false)
		_, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1)
		wantError(t, denied, "not allowed to make changes")
	})

	t.Run("rate limits per user and client", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		e.s.limiter = newRateLimiter(2, 100, time.Now)
		for i := 0; i < 2; i++ {
			if _, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1); denied != nil {
				t.Fatalf("write %d unexpectedly denied", i+1)
			}
		}
		_, denied := e.s.authorizeWrite(e.ctx(scope), scope, 1)
		wantError(t, denied, "rate limited")
	})
}

func TestRateLimiter(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := newRateLimiter(3, 5, func() time.Time { return now })

	if ok, _ := l.allow("k", 3); !ok {
		t.Fatal("3 events must fit a limit of 3")
	}
	if ok, retry := l.allow("k", 1); ok || retry != time.Minute {
		t.Fatalf("4th event must be refused with a 1m retry, got ok=%v retry=%v", ok, retry)
	}
	if ok, _ := l.allow("other", 1); !ok {
		t.Fatal("keys must be limited independently")
	}

	now = now.Add(time.Minute)
	if ok, _ := l.allow("k", 2); !ok {
		t.Fatal("the minute window must reset")
	}
	if ok, retry := l.allow("k", 1); ok || retry != 23*time.Hour+59*time.Minute {
		t.Fatalf("day limit of 5 must refuse the 6th event, got ok=%v retry=%v", ok, retry)
	}

	now = now.Add(24 * time.Hour)
	if ok, _ := l.allow("k", 3); !ok {
		t.Fatal("the day window must reset")
	}
	if ok, _ := newRateLimiter(0, 0, time.Now).allow("k", 1000); !ok {
		t.Fatal("zero limits must disable limiting")
	}
}

func TestCreateTransactionTool(t *testing.T) {
	const scope = "write:transactions"

	t.Run("creates an expense, moves the balance and audits with the client", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccountWithBalance(t, e.db, e.user.ID, 10_000)
		cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)

		text := wantOK(t, e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": acct.ID, "type": "expense", "amount": 12.5,
			"category_id": cat.ID, "description": "Lunch", "date": "2026-01-15",
		}))

		if got := e.balance(t, acct.ID); got != 10_000-1_250 {
			t.Errorf("balance = %d, want %d", got, 10_000-1_250)
		}
		if !strings.Contains(text, "$12.50") || !strings.Contains(text, "balance is now $87.50") {
			t.Errorf("unexpected output: %s", text)
		}
		var log models.AuditLog
		if err := e.db.Where("action = ?", "MCP_CREATE_TRANSACTION").First(&log).Error; err != nil {
			t.Fatalf("expected an audit row: %v", err)
		}
		if log.UserID == nil || *log.UserID != e.user.ID || log.IPAddress != "203.0.113.7" ||
			!strings.Contains(log.Changes, testClientID) || !strings.Contains(log.Changes, `"via":"mcp"`) {
			t.Errorf("unexpected audit row: %+v", log)
		}
	})

	t.Run("an expense on a credit card increases the balance owed", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		card := testutil.CreateTestCreditCardAccount(t, e.db, e.user.ID, 5_000)

		wantOK(t, e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": card.ID, "type": "expense", "amount": 20.0,
		}))
		if got := e.balance(t, card.ID); got != 7_000 {
			t.Errorf("credit card balance = %d, want 7000", got)
		}
	})

	t.Run("rejects another user's account without creating anything", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		other := testutil.CreateTestUser(t, e.db)
		acct := testutil.CreateTestCashAccountWithBalance(t, e.db, other.ID, 10_000)

		res := e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": acct.ID, "type": "expense", "amount": 5.0,
		})
		wantError(t, res, "failed to create transaction")
		if e.txnCount(t) != 0 || e.balance(t, acct.ID) != 10_000 {
			t.Error("nothing may change for another user's account")
		}
	})

	t.Run("rejects another user's category", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		other := testutil.CreateTestUser(t, e.db)
		cat := testutil.CreateTestCategory(t, e.db, other.ID, models.CategoryTypeExpense)

		res := e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": acct.ID, "type": "expense", "amount": 5.0, "category_id": cat.ID,
		})
		wantError(t, res, "Category not found")
		if e.txnCount(t) != 0 {
			t.Error("no transaction may be created")
		}
	})

	t.Run("rejects a category of the wrong type", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeIncome)

		res := e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": acct.ID, "type": "expense", "amount": 5.0, "category_id": cat.ID,
		})
		wantError(t, res, "cannot be used for an expense transaction")
	})

	t.Run("validates inputs", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		future := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
		cases := []struct {
			name string
			args map[string]any
			want string
		}{
			{"zero amount", map[string]any{"type": "expense", "amount": 0.0}, "greater than zero"},
			{"negative amount", map[string]any{"type": "expense", "amount": -3.0}, "greater than zero"},
			{"over the ceiling", map[string]any{"type": "expense", "amount": 10_000.01}, "per-change limit of $10000.00"},
			{"missing amount", map[string]any{"type": "expense"}, "amount is required"},
			{"transfer type", map[string]any{"type": "transfer", "amount": 1.0}, "type must be 'income' or 'expense'"},
			{"bad date", map[string]any{"type": "expense", "amount": 1.0, "date": "15/01/2026"}, "YYYY-MM-DD"},
			{"far future date", map[string]any{"type": "expense", "amount": 1.0, "date": future}, "days in the future"},
			{"long description", map[string]any{"type": "expense", "amount": 1.0, "description": strings.Repeat("x", 501)}, "at most 500"},
		}
		for _, tc := range cases {
			tc.args["account_id"] = acct.ID
			wantError(t, e.call(t, e.ctx(scope), "create_transaction", tc.args), tc.want)
		}
		if e.txnCount(t) != 0 {
			t.Error("invalid input must not create transactions")
		}
		if e.auditCount(t, "MCP_CREATE_TRANSACTION") != 0 {
			t.Error("invalid input must not be audited as a write")
		}
	})

	t.Run("refuses when the grant does not allow writes", func(t *testing.T) {
		e := setupWriteEnv(t, "read:transactions")
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		res := e.call(t, e.ctx(scope), "create_transaction", map[string]any{
			"account_id": acct.ID, "type": "expense", "amount": 1.0,
		})
		wantError(t, res, "not allowed to make changes")
	})
}

func TestCreateTransferTool(t *testing.T) {
	const scope = "write:transactions"
	e := setupWriteEnv(t, scope)
	from := testutil.CreateTestCashAccountWithBalance(t, e.db, e.user.ID, 10_000)
	to := testutil.CreateTestCashAccountWithBalance(t, e.db, e.user.ID, 0)

	wantOK(t, e.call(t, e.ctx(scope), "create_transfer", map[string]any{
		"from_account_id": from.ID, "to_account_id": to.ID, "amount": 25.0,
	}))
	if e.balance(t, from.ID) != 7_500 || e.balance(t, to.ID) != 2_500 {
		t.Errorf("balances = %d / %d, want 7500 / 2500", e.balance(t, from.ID), e.balance(t, to.ID))
	}
	if e.auditCount(t, "MCP_CREATE_TRANSFER") != 1 {
		t.Error("expected one transfer audit row")
	}

	res := e.call(t, e.ctx(scope), "create_transfer", map[string]any{
		"from_account_id": from.ID, "to_account_id": from.ID, "amount": 1.0,
	})
	wantError(t, res, "Cannot transfer to the same account")
}

func TestUpdateTransactionTool(t *testing.T) {
	const scope = "write:transactions"

	t.Run("a partial update leaves other fields alone", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccountWithBalance(t, e.db, e.user.ID, 10_000)
		cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)
		txn, err := e.s.services.Transactions.CreateTransaction(e.user.ID, acct.ID, &cat.ID,
			models.TransactionTypeExpense, 1_000, "Groceries", time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}

		wantOK(t, e.call(t, e.ctx(scope), "update_transaction", map[string]any{
			"transaction_id": txn.ID, "amount": 15.0,
		}))

		var got models.Transaction
		e.db.First(&got, "id = ?", txn.ID)
		if got.Amount != 1_500 || got.Description != "Groceries" || got.CategoryID == nil || *got.CategoryID != cat.ID {
			t.Errorf("unexpected transaction after update: %+v", got)
		}
		if e.balance(t, acct.ID) != 8_500 {
			t.Errorf("balance = %d, want 8500", e.balance(t, acct.ID))
		}
		var log models.AuditLog
		e.db.Where("action = ?", "MCP_UPDATE_TRANSACTION").First(&log)
		if !strings.Contains(log.Changes, `"from":1000`) || !strings.Contains(log.Changes, `"to":1500`) {
			t.Errorf("audit must record before and after values, got %s", log.Changes)
		}
	})

	t.Run("'none' clears the category", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)
		txn, _ := e.s.services.Transactions.CreateTransaction(e.user.ID, acct.ID, &cat.ID,
			models.TransactionTypeExpense, 100, "x", time.Time{})

		wantOK(t, e.call(t, e.ctx(scope), "update_transaction", map[string]any{
			"transaction_id": txn.ID, "category_id": "none",
		}))
		var got models.Transaction
		e.db.First(&got, "id = ?", txn.ID)
		if got.CategoryID != nil {
			t.Errorf("expected category cleared, got %v", *got.CategoryID)
		}
	})

	t.Run("rejects changing the type when the kept category no longer fits", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)
		txn, _ := e.s.services.Transactions.CreateTransaction(e.user.ID, acct.ID, &cat.ID,
			models.TransactionTypeExpense, 100, "x", time.Time{})

		res := e.call(t, e.ctx(scope), "update_transaction", map[string]any{
			"transaction_id": txn.ID, "type": "income",
		})
		wantError(t, res, "cannot be used for an income transaction")
	})

	t.Run("rejects another user's transaction and empty updates", func(t *testing.T) {
		e := setupWriteEnv(t, scope)
		other := testutil.CreateTestUser(t, e.db)
		acct := testutil.CreateTestCashAccount(t, e.db, other.ID)
		txn := testutil.CreateTestTransaction(t, e.db, other.ID, acct.ID, models.TransactionTypeExpense, 100)

		wantError(t, e.call(t, e.ctx(scope), "update_transaction", map[string]any{
			"transaction_id": txn.ID, "amount": 1.0,
		}), "Transaction not found")

		own := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
		mine := testutil.CreateTestTransaction(t, e.db, e.user.ID, own.ID, models.TransactionTypeExpense, 100)
		wantError(t, e.call(t, e.ctx(scope), "update_transaction", map[string]any{
			"transaction_id": mine.ID,
		}), "nothing to update")
	})
}

func TestCategorizeTransactionsTool(t *testing.T) {
	const scope = "write:transactions"
	e := setupWriteEnv(t, scope)
	acct := testutil.CreateTestCashAccount(t, e.db, e.user.ID)
	cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)
	expense := testutil.CreateTestTransaction(t, e.db, e.user.ID, acct.ID, models.TransactionTypeExpense, 100)
	income := testutil.CreateTestTransaction(t, e.db, e.user.ID, acct.ID, models.TransactionTypeIncome, 100)
	other := testutil.CreateTestUser(t, e.db)
	otherAcct := testutil.CreateTestCashAccount(t, e.db, other.ID)
	foreign := testutil.CreateTestTransaction(t, e.db, other.ID, otherAcct.ID, models.TransactionTypeExpense, 100)

	text := wantOK(t, e.call(t, e.ctx(scope), "categorize_transactions", map[string]any{
		"transaction_ids": []any{expense.ID, income.ID, foreign.ID, expense.ID},
		"category_id":     cat.ID,
	}))

	if !strings.Contains(text, "on 1 of 3 transactions") {
		t.Errorf("unexpected summary: %s", text)
	}
	var got models.Transaction
	e.db.First(&got, "id = ?", expense.ID)
	if got.CategoryID == nil || *got.CategoryID != cat.ID {
		t.Error("the expense must be categorized")
	}
	var foreignAfter models.Transaction
	e.db.First(&foreignAfter, "id = ?", foreign.ID)
	if foreignAfter.CategoryID != nil {
		t.Error("another user's transaction must not change")
	}
	if e.auditCount(t, "MCP_UPDATE_TRANSACTION") != 1 {
		t.Errorf("expected one audit row, got %d", e.auditCount(t, "MCP_UPDATE_TRANSACTION"))
	}

	ids := make([]any, maxCategorizeBatch+1)
	for i := range ids {
		ids[i] = expense.ID
	}
	wantError(t, e.call(t, e.ctx(scope), "categorize_transactions", map[string]any{
		"transaction_ids": ids, "category_id": cat.ID,
	}), "at most 25 items")
}

func TestCategoryWriteTools(t *testing.T) {
	const scope = "write:categories"
	e := setupWriteEnv(t, scope)

	text := wantOK(t, e.call(t, e.ctx(scope), "create_category", map[string]any{
		"name": "Coffee", "type": "expense", "color": "#AA5500",
	}))
	var cat models.Category
	if err := e.db.Where("user_id = ? AND name = ?", e.user.ID, "Coffee").First(&cat).Error; err != nil {
		t.Fatalf("category not created: %v (%s)", err, text)
	}

	wantOK(t, e.call(t, e.ctx(scope), "update_category", map[string]any{
		"category_id": cat.ID, "name": "Cafes",
	}))
	e.db.First(&cat, "id = ?", cat.ID)
	if cat.Name != "Cafes" || cat.Color != "#AA5500" {
		t.Errorf("unexpected category after update: %+v", cat)
	}
	if e.auditCount(t, "MCP_CREATE_CATEGORY") != 1 || e.auditCount(t, "MCP_UPDATE_CATEGORY") != 1 {
		t.Error("expected create and update audit rows")
	}

	wantError(t, e.call(t, e.ctx(scope), "create_category", map[string]any{
		"name": "Bad", "type": "expense", "color": "orange",
	}), "#RGB or #RRGGBB")
	wantError(t, e.call(t, e.ctx(scope), "create_category", map[string]any{
		"name": "Bad", "type": "transfer",
	}), "type must be")

	other := testutil.CreateTestUser(t, e.db)
	foreign := testutil.CreateTestCategory(t, e.db, other.ID, models.CategoryTypeExpense)
	wantError(t, e.call(t, e.ctx(scope), "update_category", map[string]any{
		"category_id": foreign.ID, "name": "Mine now",
	}), "Category not found")

	// A transactions-only grant cannot create categories.
	e2 := setupWriteEnv(t, "write:transactions")
	wantError(t, e2.call(t, e2.ctx("write:transactions"), "create_category", map[string]any{
		"name": "Coffee", "type": "expense",
	}), "missing required scope")
}

func TestBudgetWriteTools(t *testing.T) {
	const scope = "write:budgets"
	e := setupWriteEnv(t, scope)
	cat := testutil.CreateTestCategory(t, e.db, e.user.ID, models.CategoryTypeExpense)

	wantOK(t, e.call(t, e.ctx(scope), "create_budget", map[string]any{
		"category_id": cat.ID, "name": "Food", "amount": 400.0, "period": "monthly",
	}))
	var budget models.Budget
	if err := e.db.Where("user_id = ?", e.user.ID).First(&budget).Error; err != nil {
		t.Fatal(err)
	}
	if budget.Amount != 40_000 || budget.Period != models.BudgetPeriodMonthly {
		t.Errorf("unexpected budget: %+v", budget)
	}

	text := wantOK(t, e.call(t, e.ctx(scope), "update_budget", map[string]any{
		"budget_id": budget.ID, "is_active": false,
	}))
	if !strings.Contains(text, "paused") {
		t.Errorf("expected paused budget, got: %s", text)
	}
	e.db.First(&budget, "id = ?", budget.ID)
	if budget.IsActive || budget.Amount != 40_000 {
		t.Errorf("unexpected budget after pause: %+v", budget)
	}

	wantError(t, e.call(t, e.ctx(scope), "create_budget", map[string]any{
		"category_id": cat.ID, "name": "Food", "amount": 1.0, "period": "weekly",
	}), "period must be")
	wantError(t, e.call(t, e.ctx(scope), "update_budget", map[string]any{
		"budget_id": budget.ID,
	}), "nothing to update")
}

func TestReceivableWriteTools(t *testing.T) {
	const scope = "write:transactions"
	e := setupWriteEnv(t, scope)
	acct := testutil.CreateTestCashAccountWithBalance(t, e.db, e.user.ID, 10_000)
	expense, err := e.s.services.Receivables.CreateSharedExpense(e.user.ID, services.CreateSharedExpenseInput{
		AccountID: acct.ID, Amount: 6_000, Description: "Dinner",
		Kind: models.ReceivableKindSplit, Mode: services.ShareModeEqual,
		Counterparties: []services.SharedCounterpartyInput{{Name: "Alex"}, {Name: "Sam"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var recs []models.Receivable
	e.db.Where("transaction_id = ?", expense.ID).Order("counterparty_name").Find(&recs)
	if len(recs) != 2 {
		t.Fatalf("expected 2 receivables, got %d", len(recs))
	}
	alex, sam := recs[0], recs[1]

	text := wantOK(t, e.call(t, e.ctx(scope), "record_repayment", map[string]any{
		"receivable_id": alex.ID, "account_id": acct.ID, "amount": 20.0,
	}))
	if !strings.Contains(text, "status: settled") {
		t.Errorf("expected Alex settled, got: %s", text)
	}
	if got := e.balance(t, acct.ID); got != 10_000-6_000+2_000 {
		t.Errorf("balance = %d, want %d", got, 10_000-6_000+2_000)
	}

	wantError(t, e.call(t, e.ctx(scope), "record_repayment", map[string]any{
		"receivable_id": sam.ID, "account_id": acct.ID, "amount": 50.0,
	}), "Repayment exceeds the outstanding amount")

	text = wantOK(t, e.call(t, e.ctx(scope), "write_off_receivable", map[string]any{
		"receivable_id": sam.ID,
	}))
	if !strings.Contains(text, "status: written_off") {
		t.Errorf("expected Sam written off, got: %s", text)
	}
	if e.auditCount(t, "MCP_RECORD_REPAYMENT") != 1 || e.auditCount(t, "MCP_WRITE_OFF_RECEIVABLE") != 1 {
		t.Error("expected repayment and write-off audit rows")
	}
}

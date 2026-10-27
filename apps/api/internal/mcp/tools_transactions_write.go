package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"kuberan/internal/models"
	"kuberan/internal/services"
)

// transactionsWriteScope gates every tool that creates or edits transactions,
// including repayments and write-offs, which move money through transactions.
const transactionsWriteScope = "write:transactions"

// maxCategorizeBatch caps categorize_transactions. It is kept below the default
// per-minute rate limit so a full batch can always run.
const maxCategorizeBatch = 25

// clearCategory is the category_id value that removes a transaction's category.
const clearCategory = "none"

func (s *Server) registerTransactionWriteTools() {
	s.mcp.AddTool(
		writeTool("create_transaction",
			"Record an income or expense transaction in one of the user's accounts and update its balance. "+
				"Get account and category IDs from list_accounts and list_categories. If no category is given, "+
				"the user's categorization rules may assign one.",
			false,
			mcp.WithString("account_id", mcp.Required(), mcp.Description("Account ID")),
			mcp.WithString("type", mcp.Required(), mcp.Enum("income", "expense"), mcp.Description("'income' or 'expense'")),
			mcp.WithNumber("amount", mcp.Required(), mcp.Description("Amount in dollars, positive (e.g. 12.50)")),
			mcp.WithString("category_id", mcp.Description("Category ID; must match the transaction type")),
			mcp.WithString("description", mcp.Description("Description, e.g. the merchant (max 500 characters)")),
			mcp.WithString("date", mcp.Description("Date (YYYY-MM-DD, default today)")),
		),
		s.handleCreateTransaction,
	)

	s.mcp.AddTool(
		writeTool("create_transfer",
			"Move money between two of the user's accounts (for example paying a credit card from a cash account). "+
				"Transfers are not income or spending.",
			false,
			mcp.WithString("from_account_id", mcp.Required(), mcp.Description("Account the money leaves")),
			mcp.WithString("to_account_id", mcp.Required(), mcp.Description("Account the money arrives in")),
			mcp.WithNumber("amount", mcp.Required(), mcp.Description("Amount in dollars, positive")),
			mcp.WithString("description", mcp.Description("Description (max 500 characters)")),
			mcp.WithString("date", mcp.Description("Date (YYYY-MM-DD, default today)")),
		),
		s.handleCreateTransfer,
	)

	s.mcp.AddTool(
		writeTool("update_transaction",
			"Edit an income or expense transaction. Only the fields given change; the account balance is adjusted. "+
				"Transfers, investment transactions and repayments cannot be edited here.",
			true,
			mcp.WithString("transaction_id", mcp.Required(), mcp.Description("Transaction ID")),
			mcp.WithString("type", mcp.Enum("income", "expense"), mcp.Description("New type: 'income' or 'expense'")),
			mcp.WithNumber("amount", mcp.Description("New amount in dollars, positive")),
			mcp.WithString("category_id", mcp.Description("New category ID, or 'none' to remove the category")),
			mcp.WithString("description", mcp.Description("New description (max 500 characters)")),
			mcp.WithString("date", mcp.Description("New date (YYYY-MM-DD)")),
		),
		s.handleUpdateTransaction,
	)

	s.mcp.AddTool(
		writeTool("categorize_transactions",
			fmt.Sprintf("Set the category of up to %d income or expense transactions at once. "+
				"Each transaction's type must match the category type; ones that do not are skipped and reported.", maxCategorizeBatch),
			true,
			mcp.WithArray("transaction_ids", mcp.Required(), mcp.WithStringItems(),
				mcp.MinItems(1), mcp.MaxItems(maxCategorizeBatch), mcp.Description("Transaction IDs")),
			mcp.WithString("category_id", mcp.Required(), mcp.Description("Category ID to assign")),
		),
		s.handleCategorizeTransactions,
	)
}

// parseTxnType validates a write tool's transaction type: only income and
// expense can be created or edited through MCP.
func parseTxnType(v string) (models.TransactionType, *mcp.CallToolResult) {
	switch t := models.TransactionType(v); t {
	case models.TransactionTypeIncome, models.TransactionTypeExpense:
		return t, nil
	default:
		return "", mcp.NewToolResultError("type must be 'income' or 'expense'")
	}
}

// checkCategory verifies that categoryID belongs to the user and that its type
// matches txType. The transaction service does not check category ownership,
// so the MCP layer does.
func (s *Server) checkCategory(userID, categoryID string, txType models.TransactionType) *mcp.CallToolResult {
	category, err := s.services.Categories.GetCategoryByID(userID, categoryID)
	if err != nil {
		return serviceError("find category", err)
	}
	if string(category.Type) != string(txType) {
		return mcp.NewToolResultError(fmt.Sprintf(
			"category %q is an %s category and cannot be used for an %s transaction", category.Name, category.Type, txType))
	}
	return nil
}

// balanceLine reports an account's balance after a write, or "" if it cannot
// be read (the write itself has already succeeded).
func (s *Server) balanceLine(userID, accountID string) string {
	account, err := s.services.Accounts.GetAccountByID(userID, accountID)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("  %s balance is now %s\n", account.Name, formatCents(account.Balance))
}

func (s *Server) handleCreateTransaction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	accountID, res := requiredString(args, "account_id")
	if res != nil {
		return res, nil
	}
	txType, res := parseTxnType(stringArg(args, "type"))
	if res != nil {
		return res, nil
	}
	amount, res := s.requiredAmount(args)
	if res != nil {
		return res, nil
	}
	description, res := descriptionArg(args)
	if res != nil {
		return res, nil
	}
	date, _, res := dateArg(args, time.Now())
	if res != nil {
		return res, nil
	}
	var categoryID *string
	if v := stringArg(args, "category_id"); v != "" {
		if res := s.checkCategory(c.UserID, v, txType); res != nil {
			return res, nil
		}
		categoryID = &v
	}

	txn, err := s.services.Transactions.CreateTransaction(c.UserID, accountID, categoryID, txType, amount, description, date)
	if err != nil {
		return serviceError("create transaction", err), nil
	}
	s.audit(c, "create_transaction", "MCP_CREATE_TRANSACTION", "transaction", txn.ID, map[string]interface{}{
		"type": txType, "amount": amount, "account_id": accountID,
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Created %s of %s on %s: %s\n",
		txn.Type, formatCents(txn.Amount), txn.Date.Format("2006-01-02"), txn.Description))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", txn.ID))
	if txn.CategoryID != nil {
		sb.WriteString(fmt.Sprintf("  Category ID: %s\n", *txn.CategoryID))
	}
	sb.WriteString(s.balanceLine(c.UserID, accountID))
	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) handleCreateTransfer(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	fromID, res := requiredString(args, "from_account_id")
	if res != nil {
		return res, nil
	}
	toID, res := requiredString(args, "to_account_id")
	if res != nil {
		return res, nil
	}
	amount, res := s.requiredAmount(args)
	if res != nil {
		return res, nil
	}
	description, res := descriptionArg(args)
	if res != nil {
		return res, nil
	}
	date, _, res := dateArg(args, time.Now())
	if res != nil {
		return res, nil
	}

	txn, err := s.services.Transactions.CreateTransfer(c.UserID, fromID, toID, amount, description, date)
	if err != nil {
		return serviceError("create transfer", err), nil
	}
	s.audit(c, "create_transfer", "MCP_CREATE_TRANSFER", "transaction", txn.ID, map[string]interface{}{
		"amount": amount, "from_account_id": fromID, "to_account_id": toID,
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Transferred %s on %s\n", formatCents(txn.Amount), txn.Date.Format("2006-01-02")))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", txn.ID))
	sb.WriteString(s.balanceLine(c.UserID, fromID))
	sb.WriteString(s.balanceLine(c.UserID, toID))
	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) handleUpdateTransaction(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	txnID, res := requiredString(args, "transaction_id")
	if res != nil {
		return res, nil
	}
	before, err := s.services.Transactions.GetTransactionByID(c.UserID, txnID)
	if err != nil {
		return serviceError("find transaction", err), nil
	}

	updates := services.TransactionUpdateFields{}
	changes := map[string]interface{}{}
	effectiveType := before.Type

	if v := stringArg(args, "type"); v != "" {
		t, res := parseTxnType(v)
		if res != nil {
			return res, nil
		}
		updates.Type = &t
		effectiveType = t
		changes["type"] = map[string]interface{}{"from": before.Type, "to": t}
	}
	if amount, present, res := s.amountArg(args); res != nil {
		return res, nil
	} else if present {
		updates.Amount = &amount
		changes["amount"] = map[string]interface{}{"from": before.Amount, "to": amount}
	}
	if v := optionalString(args, "description"); v != nil {
		if len(*v) > maxDescriptionLen {
			return mcp.NewToolResultError(fmt.Sprintf("description must be at most %d characters", maxDescriptionLen)), nil
		}
		updates.Description = v
		changes["description"] = map[string]interface{}{"from": before.Description, "to": *v}
	}
	if date, present, res := dateArg(args, time.Now()); res != nil {
		return res, nil
	} else if present {
		updates.Date = &date
		changes["date"] = map[string]interface{}{"from": before.Date, "to": date}
	}
	if v := stringArg(args, "category_id"); v != "" {
		var newCategory *string
		if !strings.EqualFold(v, clearCategory) {
			if res := s.checkCategory(c.UserID, v, effectiveType); res != nil {
				return res, nil
			}
			newCategory = &v
		}
		updates.CategoryID = &newCategory
		changes["category_id"] = map[string]interface{}{"from": before.CategoryID, "to": newCategory}
	} else if updates.Type != nil && before.CategoryID != nil {
		// Changing the type keeps the old category; make sure it still fits.
		if res := s.checkCategory(c.UserID, *before.CategoryID, effectiveType); res != nil {
			return res, nil
		}
	}
	if len(changes) == 0 {
		return mcp.NewToolResultError("nothing to update: give at least one of type, amount, category_id, description, date"), nil
	}

	txn, err := s.services.Transactions.UpdateTransaction(c.UserID, txnID, updates)
	if err != nil {
		return serviceError("update transaction", err), nil
	}
	s.audit(c, "update_transaction", "MCP_UPDATE_TRANSACTION", "transaction", txn.ID, changes)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Updated transaction: %s  %s  %s  %s\n",
		txn.Date.Format("2006-01-02"), txn.Type, formatCents(txn.Amount), txn.Description))
	sb.WriteString(fmt.Sprintf("  ID: %s\n", txn.ID))
	sb.WriteString(s.balanceLine(c.UserID, txn.AccountID))
	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) handleCategorizeTransactions(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	ids, res := stringListArg(args, "transaction_ids", maxCategorizeBatch)
	if res != nil {
		return res, nil
	}

	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, len(ids))
	if denied != nil {
		return denied, nil
	}
	categoryID, res := requiredString(args, "category_id")
	if res != nil {
		return res, nil
	}
	category, err := s.services.Categories.GetCategoryByID(c.UserID, categoryID)
	if err != nil {
		return serviceError("find category", err), nil
	}

	var updated, skipped []string
	for _, id := range ids {
		before, err := s.services.Transactions.GetTransactionByID(c.UserID, id)
		if err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %s", id, serviceErrorMessage(err)))
			continue
		}
		if string(before.Type) != string(category.Type) {
			skipped = append(skipped, fmt.Sprintf("%s: %s transaction cannot use %s category", id, before.Type, category.Type))
			continue
		}
		newCategory := &categoryID
		if _, err := s.services.Transactions.UpdateTransaction(c.UserID, id,
			services.TransactionUpdateFields{CategoryID: &newCategory}); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s: %s", id, serviceErrorMessage(err)))
			continue
		}
		s.audit(c, "categorize_transactions", "MCP_UPDATE_TRANSACTION", "transaction", id, map[string]interface{}{
			"category_id": map[string]interface{}{"from": before.CategoryID, "to": categoryID},
		})
		updated = append(updated, id)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Set category %q on %d of %d transactions.\n", category.Name, len(updated), len(ids)))
	if len(skipped) > 0 {
		sb.WriteString("Skipped:\n")
		for _, line := range skipped {
			sb.WriteString("  " + line + "\n")
		}
	}
	return mcp.NewToolResultText(sb.String()), nil
}

// stringListArg reads a non-empty array of distinct, non-empty strings of at
// most maxLen items.
func stringListArg(args map[string]interface{}, name string, maxLen int) ([]string, *mcp.CallToolResult) {
	raw, ok := args[name].([]interface{})
	if !ok || len(raw) == 0 {
		return nil, mcp.NewToolResultError(name + " must be a non-empty list of IDs")
	}
	if len(raw) > maxLen {
		return nil, mcp.NewToolResultError(fmt.Sprintf("%s can have at most %d items", name, maxLen))
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		v, ok := item.(string)
		v = strings.TrimSpace(v)
		if !ok || v == "" {
			return nil, mcp.NewToolResultError(name + " must contain only non-empty IDs")
		}
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out, nil
}

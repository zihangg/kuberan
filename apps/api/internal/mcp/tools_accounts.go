package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"kuberan/internal/logger"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/services"
)

func (s *Server) registerAccountTools() {
	s.mcp.AddTool(
		readTool("list_accounts",
			mcp.WithDescription("List the active financial accounts (types cash, fixed_deposit, investment, credit_card and debt, which is a loan) with current balances. Each balance is in the account's own currency, with its value in the user's base currency (≈) when that differs; an investment account's balance is the value of its holdings, a credit card or debt balance is the amount owed. Ends with the net worth in the base currency. Limited to the first 100 accounts."),
		),
		s.handleListAccounts,
	)
}

func (s *Server) handleListAccounts(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return errUnauthorized(), nil
	}
	if denied := requireScope(ctx, "read:accounts"); denied != nil {
		return denied, nil
	}

	page := pagination.PageRequest{Page: 1, PageSize: 100}
	result, err := s.services.Accounts.GetUserAccounts(userID, page)
	if err != nil {
		return readError("list accounts", err), nil
	}

	if len(result.Data) == 0 {
		return mcp.NewToolResultText("No accounts found."), nil
	}

	user, err := s.services.Users.GetUserByID(userID)
	if err != nil {
		return readError("list accounts", err), nil
	}
	base := user.BaseCurrency

	var sb strings.Builder
	fmt.Fprintf(&sb, "Accounts (%d total):\n\n", result.TotalItems)
	var netWorth int64
	var excluded []services.UnconvertedAmount
	stale := false
	for i := range result.Data {
		a := &result.Data[i]
		stale = stale || (a.FXStale && a.BalanceBase != nil)
		status := ""
		if !a.IsActive {
			status = " [inactive]"
		}
		fmt.Fprintf(&sb, "  %-32s %-12s %s%s\n",
			untrusted(a.Name), a.Type, moneyWithBase(a.Balance, a.Currency, a.BalanceBase, base), status)
		fmt.Fprintf(&sb, "    ID: %s\n", a.ID)
		if line := s.cardFeeLine(userID, a); line != "" {
			fmt.Fprintf(&sb, "    %s\n", line)
		}

		if !a.IsActive {
			continue
		}
		sign := int64(1)
		switch a.Type {
		case models.AccountTypeCash, models.AccountTypeFixedDeposit, models.AccountTypeInvestment:
		case models.AccountTypeDebt, models.AccountTypeCreditCard:
			sign = -1
		default:
			continue
		}
		switch {
		case a.BalanceBase != nil:
			netWorth += sign * *a.BalanceBase
		case strings.EqualFold(a.Currency, base):
			netWorth += sign * a.Balance
		default:
			excluded = append(excluded, services.UnconvertedAmount{Currency: a.Currency, Amount: a.Balance})
		}
	}
	fmt.Fprintf(&sb, "\nNet worth (%s): %s (active cash, fixed-deposit and investment accounts minus credit cards and debts; receivables are not counted)\n",
		strings.ToUpper(base), formatMoney(netWorth, base))
	sb.WriteString(fxNotes(false, excluded))
	if stale {
		sb.WriteString("Note: some exchange rates used are stale\n")
	}
	if result.TotalItems > int64(len(result.Data)) {
		fmt.Fprintf(&sb, "Note: only the first %d of %d accounts are listed and counted in the net worth\n", len(result.Data), result.TotalItems)
	}

	return mcp.NewToolResultText(sb.String()), nil
}

// cardFeeLine renders a credit card's annual fee and waiver progress as one
// line, or "" when the card has no fee. The progress comes from
// CardFeeServicer.Summary; a failure is logged and the line left out, so the
// list still renders.
func (s *Server) cardFeeLine(userID string, a *models.Account) string {
	if s.services.CardFees == nil || a.Type != models.AccountTypeCreditCard || a.AnnualFee == nil {
		return ""
	}
	sum, err := s.services.CardFees.Summary(userID, a.ID, time.Now().UTC())
	if err != nil {
		logger.Get().Warnw("MCP list_accounts: card fee summary failed", "error", err, "account_id", a.ID)
		return ""
	}
	if sum == nil || sum.Fee == nil {
		return ""
	}
	cur := sum.Currency
	if cur == "" {
		cur = a.Currency
	}
	next := "next " + sum.Fee.NextFeeDate
	if d, err := time.Parse("2006-01-02", sum.Fee.NextFeeDate); err == nil {
		if sum.Fee.DayKnown {
			next = "next " + d.Format("2 Jan 2006")
		} else {
			next = "next " + d.Format("January 2006")
		}
	}
	line := fmt.Sprintf("Annual fee %s \u00b7 %s", formatMoney(sum.Fee.Amount, cur), next)
	if w := sum.Waiver; w != nil {
		// spend_or_swipes shows both targets ("... or ... transactions").
		var targets []string
		if w.Spend != nil {
			targets = append(targets, fmt.Sprintf("%s of %s", formatMoney(w.Spend.Actual, cur), formatMoney(w.Spend.Target, cur)))
		}
		if w.Swipes != nil {
			targets = append(targets, fmt.Sprintf("%d of %d transactions", w.Swipes.Actual, w.Swipes.Target))
		}
		if len(targets) > 0 {
			line += fmt.Sprintf(" \u00b7 Waiver: %s (%s)", strings.Join(targets, " or "), waiverStatusText(w.Status))
		}
	}
	return line
}

func waiverStatusText(st services.FeeWaiverStatus) string {
	return strings.ReplaceAll(string(st), "_", " ")
}

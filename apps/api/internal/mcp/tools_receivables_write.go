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

func (s *Server) registerReceivableWriteTools() {
	s.mcp.AddTool(
		writeTool("record_repayment",
			"Record money received back for a receivable (for example a friend repaying their share of a bill). "+
				"It credits the chosen account with a repayment, which is not income. Get receivable IDs from list_receivables.",
			false,
			mcp.WithString("receivable_id", mcp.Required(), mcp.Description("Receivable ID")),
			mcp.WithString("account_id", mcp.Required(), mcp.Description("Account the repayment arrived in (cash, credit card or debt)")),
			mcp.WithNumber("amount", mcp.Required(), mcp.Description("Amount repaid in dollars, at most the outstanding amount")),
			mcp.WithString("description", mcp.Description("Description (default: 'Repayment from <name>: <expense>')")),
			mcp.WithString("date", mcp.Description("Date (YYYY-MM-DD, default today)")),
		),
		s.handleRecordRepayment,
	)

	s.mcp.AddTool(
		writeTool("write_off_receivable",
			"Forgive some or all of what is still owed on a receivable. The forgiven amount becomes the user's own spending "+
				"on the original expense. It can be undone in the Kuberan app.",
			true,
			mcp.WithString("receivable_id", mcp.Required(), mcp.Description("Receivable ID")),
			mcp.WithNumber("amount", mcp.Description("Amount to forgive in dollars (default: the whole outstanding amount)")),
		),
		s.handleWriteOffReceivable,
	)
}

// receivableLine summarizes a receivable after a write.
func receivableLine(r *models.Receivable) string {
	return fmt.Sprintf("  %s: paid %s, written off %s, outstanding %s (status: %s)\n",
		r.CounterpartyName, formatCents(r.PaidAmount), formatCents(r.WrittenOffAmount),
		formatCents(r.OutstandingAmount()), r.Status)
}

func (s *Server) handleRecordRepayment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	receivableID, res := requiredString(args, "receivable_id")
	if res != nil {
		return res, nil
	}
	accountID, res := requiredString(args, "account_id")
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

	rec, settlement, err := s.services.Receivables.Settle(c.UserID, receivableID, services.SettleInput{
		AccountID:   accountID,
		Amount:      amount,
		Date:        date,
		Description: description,
	})
	if err != nil {
		return serviceError("record repayment", err), nil
	}
	s.audit(c, "record_repayment", "MCP_RECORD_REPAYMENT", "receivable", rec.ID, map[string]interface{}{
		"amount": amount, "account_id": accountID, "settlement_id": settlement.ID, "transaction_id": settlement.TransactionID,
	})

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Recorded repayment of %s on %s\n", formatCents(settlement.Amount), settlement.Date.Format("2006-01-02")))
	sb.WriteString(receivableLine(rec))
	sb.WriteString(fmt.Sprintf("  Repayment transaction ID: %s\n", settlement.TransactionID))
	sb.WriteString(s.balanceLine(c.UserID, accountID))
	return mcp.NewToolResultText(sb.String()), nil
}

func (s *Server) handleWriteOffReceivable(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, transactionsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	receivableID, res := requiredString(args, "receivable_id")
	if res != nil {
		return res, nil
	}
	var amount *int64
	if cents, present, res := s.amountArg(args); res != nil {
		return res, nil
	} else if present {
		amount = &cents
	}

	rec, err := s.services.Receivables.WriteOff(c.UserID, receivableID, amount)
	if err != nil {
		return serviceError("write off receivable", err), nil
	}
	changes := map[string]interface{}{"written_off_amount": rec.WrittenOffAmount}
	if amount != nil {
		changes["amount"] = *amount
	}
	s.audit(c, "write_off_receivable", "MCP_WRITE_OFF_RECEIVABLE", "receivable", rec.ID, changes)

	return mcp.NewToolResultText("Wrote off receivable\n" + receivableLine(rec)), nil
}

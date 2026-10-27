package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"kuberan/internal/models"
)

// budgetsWriteScope gates the budget write tools.
const budgetsWriteScope = "write:budgets"

// maxBudgetNameLen matches the REST API's binding.
const maxBudgetNameLen = 100

func (s *Server) registerBudgetWriteTools() {
	s.mcp.AddTool(
		writeTool("create_budget",
			"Create a monthly or yearly spending budget for an expense category. "+
				"A category can have one active budget per period.",
			false,
			mcp.WithString("category_id", mcp.Required(), mcp.Description("Expense category ID")),
			mcp.WithString("name", mcp.Required(), mcp.Description("Budget name (max 100 characters)")),
			mcp.WithNumber("amount", mcp.Required(), mcp.Description("Budget limit in dollars, positive")),
			mcp.WithString("period", mcp.Required(), mcp.Enum("monthly", "yearly"), mcp.Description("'monthly' or 'yearly'")),
		),
		s.handleCreateBudget,
	)

	s.mcp.AddTool(
		writeTool("update_budget",
			"Change a budget's name, limit or period, or pause/resume it. Only the fields given change.",
			true,
			mcp.WithString("budget_id", mcp.Required(), mcp.Description("Budget ID")),
			mcp.WithString("name", mcp.Description("New name (max 100 characters)")),
			mcp.WithNumber("amount", mcp.Description("New limit in dollars, positive")),
			mcp.WithString("period", mcp.Enum("monthly", "yearly"), mcp.Description("New period: 'monthly' or 'yearly'")),
			mcp.WithBoolean("is_active", mcp.Description("false pauses the budget, true resumes it")),
		),
		s.handleUpdateBudget,
	)
}

// parseBudgetPeriod validates a budget period argument.
func parseBudgetPeriod(v string) (models.BudgetPeriod, *mcp.CallToolResult) {
	switch p := models.BudgetPeriod(v); p {
	case models.BudgetPeriodMonthly, models.BudgetPeriodYearly:
		return p, nil
	default:
		return "", mcp.NewToolResultError("period must be 'monthly' or 'yearly'")
	}
}

func (s *Server) handleCreateBudget(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, budgetsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	categoryID, res := requiredString(args, "category_id")
	if res != nil {
		return res, nil
	}
	name, res := requiredString(args, "name")
	if res != nil {
		return res, nil
	}
	if len(name) > maxBudgetNameLen {
		return mcp.NewToolResultError(fmt.Sprintf("name must be at most %d characters", maxBudgetNameLen)), nil
	}
	amount, res := s.requiredAmount(args)
	if res != nil {
		return res, nil
	}
	period, res := parseBudgetPeriod(stringArg(args, "period"))
	if res != nil {
		return res, nil
	}

	budget, err := s.services.Budgets.CreateBudget(c.UserID, categoryID, name, amount, period)
	if err != nil {
		return serviceError("create budget", err), nil
	}
	s.audit(c, "create_budget", "MCP_CREATE_BUDGET", "budget", budget.ID, map[string]interface{}{
		"name": name, "amount": amount, "period": period, "category_id": categoryID,
	})

	return mcp.NewToolResultText(fmt.Sprintf("Created %s budget %q of %s\n  ID: %s\n",
		budget.Period, budget.Name, formatCents(budget.Amount), budget.ID)), nil
}

func (s *Server) handleUpdateBudget(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, budgetsWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	budgetID, res := requiredString(args, "budget_id")
	if res != nil {
		return res, nil
	}
	before, err := s.services.Budgets.GetBudgetByID(c.UserID, budgetID)
	if err != nil {
		return serviceError("find budget", err), nil
	}

	changes := map[string]interface{}{}
	name := stringArg(args, "name")
	if len(name) > maxBudgetNameLen {
		return mcp.NewToolResultError(fmt.Sprintf("name must be at most %d characters", maxBudgetNameLen)), nil
	}
	if name != "" {
		changes["name"] = map[string]interface{}{"from": before.Name, "to": name}
	}
	var amount *int64
	if cents, present, res := s.amountArg(args); res != nil {
		return res, nil
	} else if present {
		amount = &cents
		changes["amount"] = map[string]interface{}{"from": before.Amount, "to": cents}
	}
	var period *models.BudgetPeriod
	if v := stringArg(args, "period"); v != "" {
		p, res := parseBudgetPeriod(v)
		if res != nil {
			return res, nil
		}
		period = &p
		changes["period"] = map[string]interface{}{"from": before.Period, "to": p}
	}
	var isActive *bool
	if v, ok := args["is_active"].(bool); ok {
		isActive = &v
		changes["is_active"] = map[string]interface{}{"from": before.IsActive, "to": v}
	}
	if len(changes) == 0 {
		return mcp.NewToolResultError("nothing to update: give at least one of name, amount, period, is_active"), nil
	}

	budget, err := s.services.Budgets.UpdateBudget(c.UserID, budgetID, name, amount, period, isActive)
	if err != nil {
		return serviceError("update budget", err), nil
	}
	s.audit(c, "update_budget", "MCP_UPDATE_BUDGET", "budget", budget.ID, changes)

	state := "active"
	if !budget.IsActive {
		state = "paused"
	}
	return mcp.NewToolResultText(fmt.Sprintf("Updated %s budget %q: %s (%s)\n  ID: %s\n",
		budget.Period, budget.Name, formatCents(budget.Amount), state, budget.ID)), nil
}

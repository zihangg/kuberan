package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"kuberan/internal/models"
	"kuberan/internal/validator"
)

// categoriesWriteScope gates the category write tools.
const categoriesWriteScope = "write:categories"

// maxCategoryNameLen and maxCategoryIconLen match the REST API's bindings.
const (
	maxCategoryNameLen = 100
	maxCategoryIconLen = 50
)

func (s *Server) registerCategoryWriteTools() {
	s.mcp.AddTool(
		writeTool("create_category",
			"Create an income or expense category. Check list_categories first to avoid duplicates.",
			false,
			mcp.WithString("name", mcp.Required(), mcp.Description("Category name (max 100 characters)")),
			mcp.WithString("type", mcp.Required(), mcp.Enum("income", "expense"), mcp.Description("'income' or 'expense'")),
			mcp.WithString("description", mcp.Description("Description (max 500 characters)")),
			mcp.WithString("parent_id", mcp.Description("Parent category ID, to create a subcategory")),
			mcp.WithString("icon", mcp.Description("Icon name (max 50 characters)")),
			mcp.WithString("color", mcp.Description("Color as #RGB or #RRGGBB")),
		),
		s.handleCreateCategory,
	)

	s.mcp.AddTool(
		writeTool("update_category",
			"Rename or edit a category. Only the fields given change.",
			true,
			mcp.WithString("category_id", mcp.Required(), mcp.Description("Category ID")),
			mcp.WithString("name", mcp.Description("New name (max 100 characters)")),
			mcp.WithString("description", mcp.Description("New description (max 500 characters)")),
			mcp.WithString("parent_id", mcp.Description("New parent category ID")),
			mcp.WithString("icon", mcp.Description("New icon name (max 50 characters)")),
			mcp.WithString("color", mcp.Description("New color as #RGB or #RRGGBB")),
		),
		s.handleUpdateCategory,
	)
}

// categoryFields holds the optional category arguments shared by create and
// update, validated like the REST API.
type categoryFields struct {
	Name, Description, Icon, Color string
	ParentID                       *string
}

func parseCategoryFields(args map[string]interface{}) (categoryFields, *mcp.CallToolResult) {
	f := categoryFields{
		Name:  stringArg(args, "name"),
		Icon:  stringArg(args, "icon"),
		Color: stringArg(args, "color"),
	}
	if len(f.Name) > maxCategoryNameLen {
		return f, mcp.NewToolResultError(fmt.Sprintf("name must be at most %d characters", maxCategoryNameLen))
	}
	if len(f.Icon) > maxCategoryIconLen {
		return f, mcp.NewToolResultError(fmt.Sprintf("icon must be at most %d characters", maxCategoryIconLen))
	}
	if f.Color != "" && !validator.IsHexColor(f.Color) {
		return f, mcp.NewToolResultError("color must be #RGB or #RRGGBB")
	}
	description, res := descriptionArg(args)
	if res != nil {
		return f, res
	}
	f.Description = description
	if v := stringArg(args, "parent_id"); v != "" {
		f.ParentID = &v
	}
	return f, nil
}

func (s *Server) handleCreateCategory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, categoriesWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	f, res := parseCategoryFields(args)
	if res != nil {
		return res, nil
	}
	if f.Name == "" {
		return mcp.NewToolResultError("name is required"), nil
	}
	catType := models.CategoryType(stringArg(args, "type"))
	if catType != models.CategoryTypeIncome && catType != models.CategoryTypeExpense {
		return mcp.NewToolResultError("type must be 'income' or 'expense'"), nil
	}

	category, err := s.services.Categories.CreateCategory(c.UserID, f.Name, catType, f.Description, f.Icon, f.Color, f.ParentID)
	if err != nil {
		return serviceError("create category", err), nil
	}
	s.audit(c, "create_category", "MCP_CREATE_CATEGORY", "category", category.ID, map[string]interface{}{
		"name": category.Name, "type": category.Type,
	})

	return mcp.NewToolResultText(fmt.Sprintf("Created %s category %q\n  ID: %s\n",
		category.Type, category.Name, category.ID)), nil
}

func (s *Server) handleUpdateCategory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	c, denied := s.authorizeWrite(ctx, categoriesWriteScope, 1)
	if denied != nil {
		return denied, nil
	}
	args := req.GetArguments()

	categoryID, res := requiredString(args, "category_id")
	if res != nil {
		return res, nil
	}
	f, res := parseCategoryFields(args)
	if res != nil {
		return res, nil
	}
	if f.Name == "" && f.Description == "" && f.Icon == "" && f.Color == "" && f.ParentID == nil {
		return mcp.NewToolResultError("nothing to update: give at least one of name, description, parent_id, icon, color"), nil
	}
	before, err := s.services.Categories.GetCategoryByID(c.UserID, categoryID)
	if err != nil {
		return serviceError("find category", err), nil
	}

	category, err := s.services.Categories.UpdateCategory(c.UserID, categoryID, f.Name, f.Description, f.Icon, f.Color, f.ParentID)
	if err != nil {
		return serviceError("update category", err), nil
	}
	changes := map[string]interface{}{}
	if f.Name != "" {
		changes["name"] = map[string]interface{}{"from": before.Name, "to": category.Name}
	}
	if f.Description != "" {
		changes["description"] = map[string]interface{}{"from": before.Description, "to": category.Description}
	}
	if f.Icon != "" {
		changes["icon"] = map[string]interface{}{"from": before.Icon, "to": category.Icon}
	}
	if f.Color != "" {
		changes["color"] = map[string]interface{}{"from": before.Color, "to": category.Color}
	}
	if f.ParentID != nil {
		changes["parent_id"] = map[string]interface{}{"from": before.ParentID, "to": *f.ParentID}
	}
	s.audit(c, "update_category", "MCP_UPDATE_CATEGORY", "category", category.ID, changes)

	return mcp.NewToolResultText(fmt.Sprintf("Updated %s category %q\n  ID: %s\n",
		category.Type, category.Name, category.ID)), nil
}

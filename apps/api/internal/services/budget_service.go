package services

import (
	"errors"
	"time"

	"gorm.io/gorm"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
)

// budgetService handles budget-related business logic.
type budgetService struct {
	db *gorm.DB
}

// NewBudgetService creates a new BudgetServicer.
func NewBudgetService(db *gorm.DB) BudgetServicer {
	return &budgetService{db: db}
}

// CreateBudget creates a new budget for a category.
func (s *budgetService) CreateBudget(
	userID, categoryID uint,
	name string,
	amount int64,
	period models.BudgetPeriod,
	startDate time.Time,
	endDate *time.Time,
) (*models.Budget, error) {
	// Verify category exists and belongs to user
	var category models.Category
	if err := s.db.Where("id = ? AND user_id = ?", categoryID, userID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrCategoryNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	budget := &models.Budget{
		UserID:     userID,
		CategoryID: categoryID,
		Name:       name,
		Amount:     amount,
		Period:     period,
		StartDate:  startDate,
		EndDate:    endDate,
		IsActive:   true,
	}

	if err := s.db.Create(budget).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	return budget, nil
}

// GetUserBudgets returns a paginated list of budgets for the user with optional filters.
func (s *budgetService) GetUserBudgets(
	userID uint,
	page pagination.PageRequest,
	isActive *bool,
	period *models.BudgetPeriod,
) (*pagination.PageResponse[models.Budget], error) {
	page.Defaults()

	base := s.db.Model(&models.Budget{}).Where("user_id = ?", userID)
	if isActive != nil {
		base = base.Where("is_active = ?", *isActive)
	}
	if period != nil {
		base = base.Where("period = ?", *period)
	}

	var totalItems int64
	if err := base.Count(&totalItems).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	var budgets []models.Budget
	if err := base.Preload("Category").Scopes(pagination.Paginate(page)).Find(&budgets).Error; err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	result := pagination.NewPageResponse(budgets, page.Page, page.PageSize, totalItems)
	return &result, nil
}

// GetBudgetByID returns a budget by ID if it belongs to the user.
func (s *budgetService) GetBudgetByID(userID, budgetID uint) (*models.Budget, error) {
	var budget models.Budget
	if err := s.db.Preload("Category").Where("id = ? AND user_id = ?", budgetID, userID).First(&budget).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrBudgetNotFound
		}
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return &budget, nil
}

// UpdateBudget updates an existing budget's fields.
func (s *budgetService) UpdateBudget(
	userID, budgetID uint,
	name string,
	amount *int64,
	period *models.BudgetPeriod,
	endDate *time.Time,
) (*models.Budget, error) {
	budget, err := s.GetBudgetByID(userID, budgetID)
	if err != nil {
		return nil, err
	}

	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}
	if amount != nil {
		updates["amount"] = *amount
	}
	if period != nil {
		updates["period"] = *period
	}
	if endDate != nil {
		updates["end_date"] = endDate
	}

	if len(updates) > 0 {
		if err := s.db.Model(budget).Updates(updates).Error; err != nil {
			return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
		}
	}

	return budget, nil
}

// DeleteBudget soft-deletes a budget.
func (s *budgetService) DeleteBudget(userID, budgetID uint) error {
	budget, err := s.GetBudgetByID(userID, budgetID)
	if err != nil {
		return err
	}

	if err := s.db.Delete(budget).Error; err != nil {
		return apperrors.Wrap(apperrors.ErrInternalServer, err)
	}
	return nil
}

// GetBudgetProgress calculates spending vs budget for the current period.
func (s *budgetService) GetBudgetProgress(userID, budgetID uint) (*BudgetProgress, error) {
	budget, err := s.GetBudgetByID(userID, budgetID)
	if err != nil {
		return nil, err
	}

	// Determine current period window
	now := time.Now()
	var periodStart, periodEnd time.Time

	switch budget.Period {
	case models.BudgetPeriodMonthly:
		periodStart = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		periodEnd = periodStart.AddDate(0, 1, -1)
		periodEnd = time.Date(periodEnd.Year(), periodEnd.Month(), periodEnd.Day(), 23, 59, 59, 999999999, now.Location())
	case models.BudgetPeriodYearly:
		periodStart = time.Date(now.Year(), 1, 1, 0, 0, 0, 0, now.Location())
		periodEnd = time.Date(now.Year(), 12, 31, 23, 59, 59, 999999999, now.Location())
	}

	// Sum expense transactions for this category within the period
	var spent int64
	err = s.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND category_id = ? AND type = ? AND date BETWEEN ? AND ?",
			userID, budget.CategoryID, models.TransactionTypeExpense, periodStart, periodEnd).
		Scan(&spent).Error
	if err != nil {
		return nil, apperrors.Wrap(apperrors.ErrInternalServer, err)
	}

	remaining := budget.Amount - spent
	var percentage float64
	if budget.Amount > 0 {
		percentage = float64(spent) / float64(budget.Amount) * 100
	}

	return &BudgetProgress{
		BudgetID:   budget.ID,
		Budgeted:   budget.Amount,
		Spent:      spent,
		Remaining:  remaining,
		Percentage: percentage,
	}, nil
}

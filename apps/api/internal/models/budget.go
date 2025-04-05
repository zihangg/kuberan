package models

import "time"

// BudgetPeriod represents the period type for a budget
type BudgetPeriod string

const (
	BudgetPeriodMonthly BudgetPeriod = "monthly"
	BudgetPeriodYearly  BudgetPeriod = "yearly"
)

// Budget represents a budget plan for a category
type Budget struct {
	Base
	UserID      uint         `gorm:"not null" json:"user_id"`
	CategoryID  uint         `gorm:"not null" json:"category_id"`
	Name        string       `gorm:"not null" json:"name"`
	Amount      float64      `gorm:"not null" json:"amount"`
	Period      BudgetPeriod `gorm:"not null" json:"period"`
	StartDate   time.Time    `gorm:"not null" json:"start_date"`
	EndDate     *time.Time   `json:"end_date,omitempty"`
	IsActive    bool         `gorm:"default:true" json:"is_active"`
	
	// Relationships
	Category    Category     `gorm:"foreignKey:CategoryID" json:"category"`
} 
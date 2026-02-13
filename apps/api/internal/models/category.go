package models

// CategoryType represents the type of category
type CategoryType string

const (
	CategoryTypeIncome  CategoryType = "income"
	CategoryTypeExpense CategoryType = "expense"
)

// Category represents a transaction category
type Category struct {
	Base
	UserID      string       `gorm:"type:uuid;not null" json:"user_id"`
	Name        string       `gorm:"not null" json:"name"`
	Type        CategoryType `gorm:"not null" json:"type"`
	Description string       `json:"description"`
	Icon        string       `json:"icon"`
	Color       string       `json:"color"`
	ParentID    *string      `gorm:"type:uuid" json:"parent_id,omitempty"`

	// Relationships
	Parent       *Category     `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children     []Category    `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Transactions []Transaction `gorm:"foreignKey:CategoryID" json:"transactions,omitempty"`
	Budgets      []Budget      `gorm:"foreignKey:CategoryID" json:"budgets,omitempty"`
}

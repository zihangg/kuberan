package services

import (
	"errors"

	"gorm.io/gorm"

	"kuberan/internal/models"
)

// CategoryService handles category-related business logic
type CategoryService struct {
	db *gorm.DB
}

// NewCategoryService creates a new CategoryService
func NewCategoryService(db *gorm.DB) *CategoryService {
	return &CategoryService{db: db}
}

// CreateCategory creates a new category
func (s *CategoryService) CreateCategory(
	userID uint,
	name string,
	categoryType models.CategoryType,
	description string,
	icon string,
	color string,
	parentID *uint,
) (*models.Category, error) {
	// Validate input
	if name == "" {
		return nil, errors.New("category name is required")
	}

	// Check if a category with the same name already exists for this user
	var count int64
	if err := s.db.Model(&models.Category{}).
		Where("user_id = ? AND name = ?", userID, name).
		Count(&count).Error; err != nil {
		return nil, err
	}

	if count > 0 {
		return nil, errors.New("category with this name already exists")
	}

	// If parentID is provided, check that it exists and belongs to the user
	if parentID != nil {
		var parent models.Category
		if err := s.db.Where("id = ? AND user_id = ?", *parentID, userID).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("parent category not found")
			}
			return nil, err
		}
	}

	// Create category
	category := &models.Category{
		UserID:      userID,
		Name:        name,
		Type:        categoryType,
		Description: description,
		Icon:        icon,
		Color:       color,
		ParentID:    parentID,
	}

	if err := s.db.Create(category).Error; err != nil {
		return nil, err
	}

	return category, nil
}

// GetUserCategories retrieves all categories for a user
func (s *CategoryService) GetUserCategories(userID uint) ([]models.Category, error) {
	var categories []models.Category
	if err := s.db.Where("user_id = ?", userID).Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// GetUserCategoriesByType retrieves all categories of a specific type for a user
func (s *CategoryService) GetUserCategoriesByType(userID uint, categoryType models.CategoryType) ([]models.Category, error) {
	var categories []models.Category
	if err := s.db.Where("user_id = ? AND type = ?", userID, categoryType).Find(&categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

// GetCategoryByID retrieves a category by ID for a specific user
func (s *CategoryService) GetCategoryByID(userID, categoryID uint) (*models.Category, error) {
	var category models.Category
	if err := s.db.Where("id = ? AND user_id = ?", categoryID, userID).First(&category).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("category not found")
		}
		return nil, err
	}
	return &category, nil
}

// UpdateCategory updates an existing category
func (s *CategoryService) UpdateCategory(
	userID uint,
	categoryID uint,
	name string,
	description string,
	icon string,
	color string,
	parentID *uint,
) (*models.Category, error) {
	// Get the category
	category, err := s.GetCategoryByID(userID, categoryID)
	if err != nil {
		return nil, err
	}

	// If parentID is provided, check that it exists, belongs to the user, and is not the category itself
	if parentID != nil && *parentID != 0 {
		if *parentID == categoryID {
			return nil, errors.New("category cannot be its own parent")
		}

		var parent models.Category
		if err := s.db.Where("id = ? AND user_id = ?", *parentID, userID).First(&parent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, errors.New("parent category not found")
			}
			return nil, err
		}
	}

	// Update fields if provided
	updates := make(map[string]interface{})
	if name != "" {
		updates["name"] = name
	}
	if description != "" {
		updates["description"] = description
	}
	if icon != "" {
		updates["icon"] = icon
	}
	if color != "" {
		updates["color"] = color
	}
	if parentID != nil {
		updates["parent_id"] = parentID
	}

	// Apply updates if any
	if len(updates) > 0 {
		if err := s.db.Model(category).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return category, nil
}

// DeleteCategory deletes a category
func (s *CategoryService) DeleteCategory(userID, categoryID uint) error {
	// Get the category to ensure it exists and belongs to the user
	category, err := s.GetCategoryByID(userID, categoryID)
	if err != nil {
		return err
	}

	// Check if there are any child categories
	var childCount int64
	if err := s.db.Model(&models.Category{}).Where("parent_id = ?", categoryID).Count(&childCount).Error; err != nil {
		return err
	}

	if childCount > 0 {
		return errors.New("cannot delete category with child categories")
	}

	// Check if there are any transactions using this category
	var transactionCount int64
	if err := s.db.Model(&models.Transaction{}).Where("category_id = ?", categoryID).Count(&transactionCount).Error; err != nil {
		return err
	}

	if transactionCount > 0 {
		return errors.New("cannot delete category that is used by transactions")
	}

	// Delete the category
	return s.db.Delete(category).Error
}

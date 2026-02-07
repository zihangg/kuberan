package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"kuberan/internal/models"
	"kuberan/internal/services"
)

// CategoryHandler handles category-related requests
type CategoryHandler struct {
	categoryService *services.CategoryService
}

// NewCategoryHandler creates a new CategoryHandler
func NewCategoryHandler(categoryService *services.CategoryService) *CategoryHandler {
	return &CategoryHandler{categoryService: categoryService}
}

// CreateCategoryRequest represents the request payload for creating a category
type CreateCategoryRequest struct {
	Name        string              `json:"name" binding:"required"`
	Type        models.CategoryType `json:"type" binding:"required"`
	Description string              `json:"description"`
	Icon        string              `json:"icon"`
	Color       string              `json:"color"`
	ParentID    *uint               `json:"parent_id"`
}

// UpdateCategoryRequest represents the request payload for updating a category
type UpdateCategoryRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	Color       string `json:"color"`
	ParentID    *uint  `json:"parent_id"`
}

// CategoryResponse represents a category in the response
type CategoryResponse struct {
	ID          uint                `json:"id"`
	UserID      uint                `json:"user_id"`
	Name        string              `json:"name"`
	Type        models.CategoryType `json:"type"`
	Description string              `json:"description"`
	Icon        string              `json:"icon"`
	Color       string              `json:"color"`
	ParentID    *uint               `json:"parent_id,omitempty"`
}

// CreateCategory handles the creation of a new category
// @Summary     Create a category
// @Description Create a new transaction category
// @Tags        categories
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body CreateCategoryRequest true "Category details"
// @Success     201 {object} CategoryResponse "Category created"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /categories [post]
func (h *CategoryHandler) CreateCategory(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category, err := h.categoryService.CreateCategory(
		userID.(uint),
		req.Name,
		req.Type,
		req.Description,
		req.Icon,
		req.Color,
		req.ParentID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"category": category})
}

// GetUserCategories handles the retrieval of all categories for a user
// @Summary     Get all categories
// @Description Get all transaction categories for the authenticated user
// @Tags        categories
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       type query string false "Filter by category type (income/expense)"
// @Success     200 {array} CategoryResponse "List of categories"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /categories [get]
func (h *CategoryHandler) GetUserCategories(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Check if type filter is provided
	categoryType := c.Query("type")
	var categories []models.Category
	var err error

	if categoryType != "" {
		categories, err = h.categoryService.GetUserCategoriesByType(userID.(uint), models.CategoryType(categoryType))
	} else {
		categories, err = h.categoryService.GetUserCategories(userID.(uint))
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve categories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

// GetCategoryByID handles the retrieval of a specific category
// @Summary     Get category by ID
// @Description Get a specific transaction category by ID
// @Tags        categories
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Category ID"
// @Success     200 {object} CategoryResponse "Category details"
// @Failure     400 {object} ErrorResponse "Invalid category ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Category not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /categories/{id} [get]
func (h *CategoryHandler) GetCategoryByID(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get category ID from URL
	categoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	category, err := h.categoryService.GetCategoryByID(userID.(uint), uint(categoryID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"category": category})
}

// UpdateCategory handles updating a category
// @Summary     Update category
// @Description Update an existing transaction category
// @Tags        categories
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Category ID"
// @Param       request body UpdateCategoryRequest true "Updated category details"
// @Success     200 {object} CategoryResponse "Updated category"
// @Failure     400 {object} ErrorResponse "Invalid input or category ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Category not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /categories/{id} [put]
func (h *CategoryHandler) UpdateCategory(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get category ID from URL
	categoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	var req UpdateCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	category, err := h.categoryService.UpdateCategory(
		userID.(uint),
		uint(categoryID),
		req.Name,
		req.Description,
		req.Icon,
		req.Color,
		req.ParentID,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"category": category})
}

// DeleteCategory handles deleting a category
// @Summary     Delete category
// @Description Delete a transaction category by ID
// @Tags        categories
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Category ID"
// @Success     200 {object} MessageResponse "Category deleted"
// @Failure     400 {object} ErrorResponse "Invalid category ID or cannot delete category in use"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Category not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /categories/{id} [delete]
func (h *CategoryHandler) DeleteCategory(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get category ID from URL
	categoryID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid category ID"})
		return
	}

	if err := h.categoryService.DeleteCategory(userID.(uint), uint(categoryID)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Category deleted successfully"})
}

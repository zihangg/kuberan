package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"kuberan/internal/models"
	"kuberan/internal/services"
)

// AccountHandler handles account-related requests
type AccountHandler struct {
	accountService *services.AccountService
}

// NewAccountHandler creates a new AccountHandler
func NewAccountHandler(accountService *services.AccountService) *AccountHandler {
	return &AccountHandler{accountService: accountService}
}

// CreateCashAccountRequest represents the request payload for creating a cash account
type CreateCashAccountRequest struct {
	Name           string  `json:"name" binding:"required"`
	Description    string  `json:"description"`
	Currency       string  `json:"currency"`
	InitialBalance float64 `json:"initial_balance"`
}

// UpdateCashAccountRequest represents the request payload for updating a cash account
type UpdateCashAccountRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// AccountResponse represents an account in the response
type AccountResponse struct {
	ID          uint              `json:"id"`
	UserID      uint              `json:"user_id"`
	Name        string            `json:"name"`
	Type        models.AccountType `json:"type"`
	Description string            `json:"description"`
	Balance     float64           `json:"balance"`
	Currency    string            `json:"currency"`
	IsActive    bool              `json:"is_active"`
}

// CreateCashAccount handles the creation of a new cash account
// @Summary     Create a cash account
// @Description Create a new cash account for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body CreateCashAccountRequest true "Cash account details"
// @Success     201 {object} AccountResponse "Account created"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts/cash [post]
func (h *AccountHandler) CreateCashAccount(c *gin.Context) {
	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req CreateCashAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := h.accountService.CreateCashAccount(
		userID.(uint),
		req.Name,
		req.Description,
		req.Currency,
		req.InitialBalance,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"account": account})
}

// GetUserAccounts handles the retrieval of all accounts for a user
// @Summary     Get all user accounts
// @Description Get all accounts for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {array} AccountResponse "List of accounts"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts [get]
func (h *AccountHandler) GetUserAccounts(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	accounts, err := h.accountService.GetUserAccounts(userID.(uint))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve accounts"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"accounts": accounts})
}

// GetAccountByID handles the retrieval of a specific account for a user
// @Summary     Get account by ID
// @Description Get a specific account by ID for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Account ID"
// @Success     200 {object} AccountResponse "Account details"
// @Failure     400 {object} ErrorResponse "Invalid account ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts/{id} [get]
func (h *AccountHandler) GetAccountByID(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get account ID from URL
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	account, err := h.accountService.GetAccountByID(userID.(uint), uint(accountID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"account": account})
}

// UpdateCashAccount handles updating a cash account
// @Summary     Update cash account
// @Description Update an existing cash account for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Account ID"
// @Param       request body UpdateCashAccountRequest true "Updated account details"
// @Success     200 {object} AccountResponse "Updated account"
// @Failure     400 {object} ErrorResponse "Invalid input or account ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts/{id} [put]
func (h *AccountHandler) UpdateCashAccount(c *gin.Context) {
	// Get user ID from context
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Get account ID from URL
	accountID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid account ID"})
		return
	}

	var req UpdateCashAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	account, err := h.accountService.UpdateCashAccount(
		userID.(uint),
		uint(accountID),
		req.Name,
		req.Description,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"account": account})
} 
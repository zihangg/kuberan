package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/pagination"
	"kuberan/internal/services"
)

// AccountHandler handles account-related requests.
type AccountHandler struct {
	accountService services.AccountServicer
	auditService   services.AuditServicer
}

// NewAccountHandler creates a new AccountHandler.
func NewAccountHandler(accountService services.AccountServicer, auditService services.AuditServicer) *AccountHandler {
	return &AccountHandler{accountService: accountService, auditService: auditService}
}

// CreateCashAccountRequest represents the request payload for creating a cash account
type CreateCashAccountRequest struct {
	Name           string `json:"name" binding:"required,min=1,max=100"`
	Description    string `json:"description" binding:"max=500"`
	Currency       string `json:"currency" binding:"omitempty,iso4217"`
	InitialBalance int64  `json:"initial_balance" binding:"gte=0"`
}

// CreateInvestmentAccountRequest represents the request payload for creating an investment account.
type CreateInvestmentAccountRequest struct {
	Name          string `json:"name" binding:"required,min=1,max=100"`
	Description   string `json:"description" binding:"max=500"`
	Currency      string `json:"currency" binding:"omitempty,iso4217"`
	Broker        string `json:"broker" binding:"max=100"`
	AccountNumber string `json:"account_number" binding:"max=50"`
}

// UpdateCashAccountRequest represents the request payload for updating a cash account
type UpdateCashAccountRequest struct {
	Name        string `json:"name" binding:"omitempty,min=1,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// AccountResponse represents an account in the response
type AccountResponse struct {
	ID          uint               `json:"id"`
	UserID      uint               `json:"user_id"`
	Name        string             `json:"name"`
	Type        models.AccountType `json:"type"`
	Description string             `json:"description"`
	Balance     int64              `json:"balance"`
	Currency    string             `json:"currency"`
	IsActive    bool               `json:"is_active"`
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
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	var req CreateCashAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	account, err := h.accountService.CreateCashAccount(
		userID,
		req.Name,
		req.Description,
		req.Currency,
		req.InitialBalance,
	)
	if err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(userID, "CREATE_ACCOUNT", "account", account.ID, c.ClientIP(),
		map[string]interface{}{"name": req.Name, "currency": req.Currency})

	c.JSON(http.StatusCreated, gin.H{"account": account})
}

// CreateInvestmentAccount handles the creation of a new investment account.
// @Summary     Create an investment account
// @Description Create a new investment account for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body CreateInvestmentAccountRequest true "Investment account details"
// @Success     201 {object} AccountResponse "Account created"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts/investment [post]
func (h *AccountHandler) CreateInvestmentAccount(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	var req CreateInvestmentAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	account, err := h.accountService.CreateInvestmentAccount(
		userID,
		req.Name,
		req.Description,
		req.Currency,
		req.Broker,
		req.AccountNumber,
	)
	if err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(userID, "CREATE_ACCOUNT", "account", account.ID, c.ClientIP(),
		map[string]interface{}{"name": req.Name, "type": "investment", "broker": req.Broker})

	c.JSON(http.StatusCreated, gin.H{"account": account})
}

// GetUserAccounts handles the retrieval of accounts for a user
// @Summary     Get user accounts
// @Description Get a paginated list of accounts for the authenticated user
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       page      query int false "Page number (default 1)"
// @Param       page_size query int false "Items per page (default 20, max 100)"
// @Success     200 {object} pagination.PageResponse[models.Account] "Paginated accounts"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts [get]
func (h *AccountHandler) GetUserAccounts(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	var page pagination.PageRequest
	if err := c.ShouldBindQuery(&page); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	result, err := h.accountService.GetUserAccounts(userID, page)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
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
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	accountID, err := parsePathID(c, "id")
	if err != nil {
		respondWithError(c, err)
		return
	}

	account, err := h.accountService.GetAccountByID(userID, accountID)
	if err != nil {
		respondWithError(c, err)
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
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	accountID, err := parsePathID(c, "id")
	if err != nil {
		respondWithError(c, err)
		return
	}

	var req UpdateCashAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	account, err := h.accountService.UpdateCashAccount(
		userID,
		accountID,
		req.Name,
		req.Description,
	)
	if err != nil {
		respondWithError(c, err)
		return
	}

	h.auditService.Log(userID, "UPDATE_ACCOUNT", "account", accountID, c.ClientIP(),
		map[string]interface{}{"name": req.Name, "description": req.Description})

	c.JSON(http.StatusOK, gin.H{"account": account})
}

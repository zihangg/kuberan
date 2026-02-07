package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/services"
)

// TransactionHandler handles transaction-related requests.
type TransactionHandler struct {
	transactionService services.TransactionServicer
}

// NewTransactionHandler creates a new TransactionHandler.
func NewTransactionHandler(transactionService services.TransactionServicer) *TransactionHandler {
	return &TransactionHandler{transactionService: transactionService}
}

// CreateTransactionRequest represents the request payload for creating a transaction
type CreateTransactionRequest struct {
	AccountID   uint                   `json:"account_id" binding:"required"`
	CategoryID  *uint                  `json:"category_id"`
	Type        models.TransactionType `json:"type" binding:"required,transaction_type"`
	Amount      int64                  `json:"amount" binding:"required,gt=0"`
	Description string                 `json:"description" binding:"max=500"`
	Date        *time.Time             `json:"date"`
}

// TransactionResponse represents a transaction in the response
type TransactionResponse struct {
	ID          uint                   `json:"id"`
	UserID      uint                   `json:"user_id"`
	AccountID   uint                   `json:"account_id"`
	CategoryID  *uint                  `json:"category_id,omitempty"`
	Type        models.TransactionType `json:"type"`
	Amount      int64                  `json:"amount"`
	Description string                 `json:"description"`
	Date        time.Time              `json:"date"`
}

// CreateTransaction handles the creation of a new transaction
// @Summary     Create a transaction
// @Description Create a new transaction (income or expense) for an account
// @Tags        transactions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       request body CreateTransactionRequest true "Transaction details"
// @Success     201 {object} TransactionResponse "Transaction created"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /transactions [post]
func (h *TransactionHandler) CreateTransaction(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	var req CreateTransactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	transactionDate := time.Now()
	if req.Date != nil {
		transactionDate = *req.Date
	}

	transaction, err := h.transactionService.CreateTransaction(
		userID,
		req.AccountID,
		req.CategoryID,
		req.Type,
		req.Amount,
		req.Description,
		transactionDate,
	)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{"transaction": transaction})
}

// GetAccountTransactions handles the retrieval of transactions for a specific account
// @Summary     Get account transactions
// @Description Get all transactions for a specific account
// @Tags        accounts,transactions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Account ID"
// @Success     200 {array} TransactionResponse "List of transactions"
// @Failure     400 {object} ErrorResponse "Invalid account ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /accounts/{id}/transactions [get]
func (h *TransactionHandler) GetAccountTransactions(c *gin.Context) {
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

	transactions, err := h.transactionService.GetAccountTransactions(userID, accountID)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"transactions": transactions})
}

// GetTransactionByID handles the retrieval of a specific transaction
// @Summary     Get transaction by ID
// @Description Get a specific transaction by ID
// @Tags        transactions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Transaction ID"
// @Success     200 {object} TransactionResponse "Transaction details"
// @Failure     400 {object} ErrorResponse "Invalid transaction ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Transaction not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /transactions/{id} [get]
func (h *TransactionHandler) GetTransactionByID(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	transactionID, err := parsePathID(c, "id")
	if err != nil {
		respondWithError(c, err)
		return
	}

	transaction, err := h.transactionService.GetTransactionByID(userID, transactionID)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"transaction": transaction})
}

// DeleteTransaction handles the deletion of a transaction
// @Summary     Delete transaction
// @Description Delete a transaction by ID
// @Tags        transactions
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id path int true "Transaction ID"
// @Success     200 {object} MessageResponse "Transaction deleted"
// @Failure     400 {object} ErrorResponse "Invalid transaction ID"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Failure     404 {object} ErrorResponse "Transaction not found"
// @Failure     500 {object} ErrorResponse "Server error"
// @Router      /transactions/{id} [delete]
func (h *TransactionHandler) DeleteTransaction(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	transactionID, err := parsePathID(c, "id")
	if err != nil {
		respondWithError(c, err)
		return
	}

	if err := h.transactionService.DeleteTransaction(userID, transactionID); err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Transaction deleted successfully"})
}

// MessageResponse represents a simple message response
type MessageResponse struct {
	Message string `json:"message"`
}

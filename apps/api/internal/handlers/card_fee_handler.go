package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/models"
	"kuberan/internal/services"
)

// CardFeeHandler serves a credit card's annual fee and its waiver progress
// (plan 025).
type CardFeeHandler struct {
	cardFeeService services.CardFeeServicer
	auditService   services.AuditServicer
	now            func() time.Time
}

// NewCardFeeHandler creates a new CardFeeHandler.
func NewCardFeeHandler(cardFeeService services.CardFeeServicer, auditService services.AuditServicer) *CardFeeHandler {
	return &CardFeeHandler{cardFeeService: cardFeeService, auditService: auditService, now: time.Now}
}

// SetCardAnnualFeeRequest sets or replaces a card's annual fee. Amounts are
// cents in the card's currency; the service validates the rest.
type SetCardAnnualFeeRequest struct {
	Amount       int64   `json:"amount" binding:"gte=0"`
	Month        int     `json:"month" binding:"required,gte=1,lte=12"`
	Day          *int    `json:"day" binding:"omitempty,gte=1,lte=31"`
	WaiverType   *string `json:"waiver_type" binding:"omitempty,oneof=spend swipes spend_or_swipes"`
	WaiverSpend  *int64  `json:"waiver_spend" binding:"omitempty,gte=1"`
	WaiverSwipes *int    `json:"waiver_swipes" binding:"omitempty,gte=1"`
	// WaiverWindow defaults to card_year when omitted.
	WaiverWindow string `json:"waiver_window" binding:"omitempty,oneof=card_year calendar_year"`
}

// Set sets or replaces the annual fee of a credit card.
// @Summary     Set a card's annual fee
// @Description Sets or replaces the annual fee and its optional waiver condition, and re-syncs the card's reminders
// @Tags        accounts
// @Accept      json
// @Produce     json
// @Param       id path string true "Account ID"
// @Param       request body SetCardAnnualFeeRequest true "Annual fee"
// @Success     200 {object} object "The annual fee (annual_fee)"
// @Failure     400 {object} ErrorResponse "Invalid input or not a credit card"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Router      /accounts/{id}/annual-fee [put]
// @Security    BearerAuth
func (h *CardFeeHandler) Set(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}
	var req SetCardAnnualFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}
	in := services.CardAnnualFeeInput{
		Amount:       req.Amount,
		Month:        req.Month,
		Day:          req.Day,
		WaiverSpend:  req.WaiverSpend,
		WaiverSwipes: req.WaiverSwipes,
		WaiverWindow: models.WaiverWindow(req.WaiverWindow),
	}
	if req.WaiverType != nil {
		wt := models.WaiverType(*req.WaiverType)
		in.WaiverType = &wt
	}
	if in.WaiverWindow == "" {
		in.WaiverWindow = models.WaiverWindowCardYear
	}
	accountID := c.Param("id")
	fee, err := h.cardFeeService.Set(userID, accountID, in, h.now())
	if err != nil {
		respondWithError(c, err)
		return
	}
	h.auditService.Log(userID, "SET_ANNUAL_FEE", "account", accountID, c.ClientIP(), nil)
	c.JSON(http.StatusOK, gin.H{"annual_fee": fee})
}

// Delete removes the annual fee of a credit card.
// @Summary     Remove a card's annual fee
// @Description Removes the annual fee and its waiver condition, and re-syncs the card's reminders
// @Tags        accounts
// @Param       id path string true "Account ID"
// @Success     204 "No content"
// @Failure     400 {object} ErrorResponse "Not a credit card"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Router      /accounts/{id}/annual-fee [delete]
// @Security    BearerAuth
func (h *CardFeeHandler) Delete(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}
	accountID := c.Param("id")
	if err := h.cardFeeService.Delete(userID, accountID, h.now()); err != nil {
		respondWithError(c, err)
		return
	}
	h.auditService.Log(userID, "DELETE_ANNUAL_FEE", "account", accountID, c.ClientIP(), nil)
	c.Status(http.StatusNoContent)
}

// Summary returns the card's fee and waiver progress.
// @Summary     Card fee and waiver progress
// @Description The annual fee, next fee date and progress toward the waiver target, computed from the card's own transactions; fee and waiver are null when not set
// @Tags        accounts
// @Produce     json
// @Param       id path string true "Account ID"
// @Success     200 {object} services.CardFeeSummary "Fee and waiver progress"
// @Failure     400 {object} ErrorResponse "Not a credit card"
// @Failure     404 {object} ErrorResponse "Account not found"
// @Router      /accounts/{id}/fee-waiver [get]
// @Security    BearerAuth
func (h *CardFeeHandler) Summary(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}
	summary, err := h.cardFeeService.Summary(userID, c.Param("id"), h.now())
	if err != nil {
		respondWithError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

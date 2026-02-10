package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/pagination"
	"kuberan/internal/services"
)

// PortfolioSnapshotHandler handles portfolio snapshot requests.
type PortfolioSnapshotHandler struct {
	snapshotService services.PortfolioSnapshotServicer
	auditService    services.AuditServicer
}

// NewPortfolioSnapshotHandler creates a new PortfolioSnapshotHandler.
func NewPortfolioSnapshotHandler(snapshotService services.PortfolioSnapshotServicer, auditService services.AuditServicer) *PortfolioSnapshotHandler {
	return &PortfolioSnapshotHandler{snapshotService: snapshotService, auditService: auditService}
}

// ComputeSnapshotsRequest represents the request payload for computing snapshots.
type ComputeSnapshotsRequest struct {
	RecordedAt time.Time `json:"recorded_at" binding:"required"`
}

// ComputeSnapshots handles computing and recording portfolio snapshots.
// @Summary     Compute portfolio snapshots
// @Description Compute and record portfolio snapshots for all users (pipeline endpoint)
// @Tags        pipeline
// @Accept      json
// @Produce     json
// @Param       X-API-Key  header   string                   true "Pipeline API key"
// @Param       request    body     ComputeSnapshotsRequest  true "Snapshot parameters"
// @Success     200        {object} map[string]int           "Snapshots recorded count"
// @Failure     400        {object} ErrorResponse            "Invalid input"
// @Failure     401        {object} ErrorResponse            "Invalid API key"
// @Failure     503        {object} ErrorResponse            "Pipeline not configured"
// @Router      /pipeline/snapshots [post]
func (h *PortfolioSnapshotHandler) ComputeSnapshots(c *gin.Context) {
	var req ComputeSnapshotsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	count, err := h.snapshotService.ComputeAndRecordSnapshots(req.RecordedAt)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"snapshots_recorded": count})
}

// GetSnapshots handles retrieving portfolio snapshots for the authenticated user.
// @Summary     Get portfolio snapshots
// @Description Get paginated portfolio snapshots for a date range
// @Tags        investments
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       from_date query string true  "Start date (RFC3339 or YYYY-MM-DD)"
// @Param       to_date   query string true  "End date (RFC3339 or YYYY-MM-DD)"
// @Param       page      query int    false "Page number (default 1)"
// @Param       page_size query int    false "Items per page (default 20, max 100)"
// @Success     200 {object} pagination.PageResponse[models.PortfolioSnapshot] "Paginated snapshots"
// @Failure     400 {object} ErrorResponse "Invalid input"
// @Failure     401 {object} ErrorResponse "Unauthorized"
// @Router      /investments/snapshots [get]
func (h *PortfolioSnapshotHandler) GetSnapshots(c *gin.Context) {
	userID, err := getUserID(c)
	if err != nil {
		respondWithError(c, err)
		return
	}

	fromStr := c.Query("from_date")
	if fromStr == "" {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, "from_date is required"))
		return
	}
	from, err := parseFlexibleTime(fromStr)
	if err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	toStr := c.Query("to_date")
	if toStr == "" {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, "to_date is required"))
		return
	}
	to, err := parseFlexibleTime(toStr)
	if err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	var page pagination.PageRequest
	if err := c.ShouldBindQuery(&page); err != nil {
		respondWithError(c, apperrors.WithMessage(apperrors.ErrInvalidInput, err.Error()))
		return
	}

	result, err := h.snapshotService.GetSnapshots(userID, from, to, page)
	if err != nil {
		respondWithError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

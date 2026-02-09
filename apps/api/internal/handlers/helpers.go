package handlers

import (
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
)

// parseFlexibleTime parses a date/time string accepting both RFC3339
// (e.g. "2006-01-02T15:04:05Z07:00") and date-only (e.g. "2006-01-02") formats.
// Date-only strings are interpreted as midnight UTC.
func parseFlexibleTime(value string) (time.Time, error) {
	// Try RFC3339 first (most specific)
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	// Fall back to date-only format
	if t, err := time.Parse("2006-01-02", value); err == nil {
		return t, nil
	}
	return time.Time{}, errors.New("invalid date format, use RFC3339 (e.g. 2024-01-01T00:00:00Z) or YYYY-MM-DD")
}

// getUserID extracts the authenticated user ID from the Gin context.
// Returns ErrUnauthorized if not present.
func getUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("userID")
	if !exists {
		return 0, apperrors.ErrUnauthorized
	}
	return userID.(uint), nil
}

// parsePathID parses a uint path parameter.
// Returns ErrInvalidInput if the parameter is not a valid positive integer.
//
//nolint:unparam // param is intentionally generic for reuse across handlers with different path params
func parsePathID(c *gin.Context, param string) (uint, error) {
	id, err := strconv.ParseUint(c.Param(param), 10, 32)
	if err != nil {
		return 0, apperrors.WithMessage(apperrors.ErrInvalidInput, "Invalid "+param)
	}
	return uint(id), nil
}

// respondWithError writes a consistent JSON error response. If the error is an
// *AppError it uses the error's status code, code, and message. Otherwise it
// logs the unexpected error and returns a generic internal server error.
func respondWithError(c *gin.Context, err error) {
	var appErr *apperrors.AppError
	if errors.As(err, &appErr) {
		if appErr.Internal != nil {
			logger.Get().Errorw("app error",
				"code", appErr.Code,
				"internal", appErr.Internal.Error(),
				"path", c.Request.URL.Path,
			)
		}
		c.JSON(appErr.StatusCode, gin.H{
			"error": gin.H{
				"code":    appErr.Code,
				"message": appErr.Message,
			},
		})
		return
	}

	logger.Get().Errorw("unexpected error",
		"error", err.Error(),
		"path", c.Request.URL.Path,
		"method", c.Request.Method,
	)
	c.JSON(apperrors.ErrInternalServer.StatusCode, gin.H{
		"error": gin.H{
			"code":    apperrors.ErrInternalServer.Code,
			"message": apperrors.ErrInternalServer.Message,
		},
	})
}

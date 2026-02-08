package handlers

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
)

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

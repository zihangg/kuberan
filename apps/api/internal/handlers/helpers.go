package handlers

import (
	"errors"

	"github.com/gin-gonic/gin"

	apperrors "kuberan/internal/errors"
	"kuberan/internal/logger"
)

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

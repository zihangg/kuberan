package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

// PipelineAuthMiddleware creates a Gin middleware that validates the X-API-Key
// header against the configured pipeline API key.
func PipelineAuthMiddleware(apiKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable,
				gin.H{"error": gin.H{"code": "PIPELINE_NOT_CONFIGURED", "message": "Pipeline endpoints are not configured"}})
			return
		}
		key := c.GetHeader("X-API-Key")
		if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized,
				gin.H{"error": gin.H{"code": "INVALID_API_KEY", "message": "Invalid or missing API key"}})
			return
		}
		c.Next()
	}
}

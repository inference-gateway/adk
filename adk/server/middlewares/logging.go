package middlewares

import (
	"github.com/gin-gonic/gin"
)

// LoggingMiddleware returns a gin middleware that logs requests,
// but can optionally skip logging for health check endpoints
func LoggingMiddleware(disableHealthcheckLog bool) gin.HandlerFunc {
	logger := gin.Logger()

	if !disableHealthcheckLog {
		return logger
	}

	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		logger(c)
	}
}

package http

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware returns a Gin middleware that validates Bearer token
// authentication. If token is empty, the middleware is a no-op.
func AuthMiddleware(token string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if token == "" {
			c.Next()
			return
		}
		auth := c.GetHeader("Authorization")
		if auth != "Bearer "+token {
			Fail(c, 401, "unauthorized")
			c.Abort()
			return
		}
		c.Next()
	}
}

// LoggingMiddleware logs each HTTP request with method, path, status, and latency.
// Health check requests are not logged to reduce noise.
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		c.Next()

		// Skip logging for health check endpoint
		if path == "/health" {
			return
		}

		slog.Info("http request",
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"client", c.ClientIP(),
		)
	}
}

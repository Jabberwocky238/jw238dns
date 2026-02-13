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
func LoggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency", time.Since(start).String(),
			"client", c.ClientIP(),
		)
	}
}

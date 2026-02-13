package http

import (
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

var startTime = time.Now()

// HealthHandler handles GET /health.
func HealthHandler(c *gin.Context) {
	OK(c, gin.H{"status": "ok"})
}

// StatusHandler handles GET /status and returns system runtime information.
func StatusHandler(c *gin.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	OK(c, gin.H{
		"uptime":      time.Since(startTime).String(),
		"goroutines":  runtime.NumGoroutine(),
		"go_version":  runtime.Version(),
		"alloc_bytes": mem.Alloc,
	})
}

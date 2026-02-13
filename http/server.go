package http

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"jabberwocky238/jw238dns/storage"

	"github.com/gin-gonic/gin"
)

// ServerConfig holds the configuration for the HTTP management server.
type ServerConfig struct {
	Listen    string
	AuthToken string // Bearer token; empty disables auth.
}

// Server is the HTTP management API server.
type Server struct {
	httpServer *http.Server
	engine     *gin.Engine
}

// NewServer creates a new HTTP management server wired to the given storage.
func NewServer(cfg ServerConfig, store storage.CoreStorage) *Server {
	gin.SetMode(gin.ReleaseMode)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(LoggingMiddleware())

	// Public endpoints (no auth).
	engine.GET("/health", HealthHandler)
	engine.GET("/status", StatusHandler)

	// Authenticated DNS management endpoints.
	dnsGroup := engine.Group("/dns")
	dnsGroup.Use(AuthMiddleware(cfg.AuthToken))
	{
		h := NewDNSHandler(store)
		dnsGroup.POST("/add", h.AddRecord)
		dnsGroup.POST("/delete", h.DeleteRecord)
		dnsGroup.POST("/update", h.UpdateRecord)
		dnsGroup.GET("/list", h.ListRecords)
		dnsGroup.GET("/get", h.GetRecord)
	}

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Listen,
			Handler: engine,
		},
		engine: engine,
	}
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start() error {
	slog.Info("HTTP management server starting", "address", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully stops the server with a 5-second deadline.
func (s *Server) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(ctx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}
}

// Engine returns the underlying Gin engine (useful for testing).
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

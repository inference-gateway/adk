package server

import (
	"context"
	"fmt"
	"mime"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
)

// ArtifactsServer provides HTTP endpoints for artifact download
type ArtifactsServer interface {
	// Start starts the artifacts server
	Start(ctx context.Context) error

	// Stop stops the artifacts server
	Stop(ctx context.Context) error

	// GetStorage returns the storage provider
	GetStorage() ArtifactStorageProvider

	// SetStorage sets the storage provider
	SetStorage(storage ArtifactStorageProvider)
}

// ArtifactsServerImpl implements the ArtifactsServer interface
type ArtifactsServerImpl struct {
	config  *config.ArtifactsConfig
	logger  *zap.Logger
	storage ArtifactStorageProvider
	server  *http.Server
	router  *gin.Engine
}

// NewArtifactsServer creates a new artifacts server instance
func NewArtifactsServer(cfg *config.ArtifactsConfig, logger *zap.Logger) ArtifactsServer {
	return &ArtifactsServerImpl{
		config: cfg,
		logger: logger,
	}
}

// Start starts the artifacts server
func (s *ArtifactsServerImpl) Start(ctx context.Context) error {
	if s.storage == nil {
		return fmt.Errorf("storage provider must be set before starting artifacts server")
	}

	s.setupRouter()

	// Create HTTP server
	addr := fmt.Sprintf("0.0.0.0:%s", s.config.ServerConfig.Port)
	s.server = &http.Server{
		Addr:           addr,
		Handler:        s.router,
		ReadTimeout:    s.config.ServerConfig.ReadTimeout,
		WriteTimeout:   s.config.ServerConfig.WriteTimeout,
		IdleTimeout:    s.config.ServerConfig.IdleTimeout,
		MaxHeaderBytes: 1 << 20, // 1 MB
	}

	s.logger.Info("starting artifacts server", zap.String("address", addr))

	// Start server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		if s.config.ServerConfig.TLSConfig.Enable {
			errChan <- s.server.ListenAndServeTLS(
				s.config.ServerConfig.TLSConfig.CertPath,
				s.config.ServerConfig.TLSConfig.KeyPath,
			)
		} else {
			errChan <- s.server.ListenAndServe()
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Info("artifacts server context cancelled, shutting down")
		return s.Stop(context.Background())
	case err := <-errChan:
		if err != http.ErrServerClosed {
			return fmt.Errorf("artifacts server failed to start: %w", err)
		}
		return nil
	}
}

// Stop stops the artifacts server
func (s *ArtifactsServerImpl) Stop(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("stopping artifacts server")

	// Create a context with timeout for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Error("failed to gracefully shutdown artifacts server", zap.Error(err))
		return err
	}

	// Close storage provider
	if s.storage != nil {
		if err := s.storage.Close(); err != nil {
			s.logger.Error("failed to close storage provider", zap.Error(err))
		}
	}

	s.logger.Info("artifacts server stopped")
	return nil
}

// GetStorage returns the storage provider
func (s *ArtifactsServerImpl) GetStorage() ArtifactStorageProvider {
	return s.storage
}

// SetStorage sets the storage provider
func (s *ArtifactsServerImpl) SetStorage(storage ArtifactStorageProvider) {
	s.storage = storage
}

// setupRouter configures the HTTP routes
func (s *ArtifactsServerImpl) setupRouter() {
	if s.config == nil {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.Use(s.loggingMiddleware())

	// Health check endpoint
	s.router.GET("/health", s.handleHealth)

	// Artifact download endpoint
	s.router.GET("/artifacts/:artifactId/:filename", s.handleArtifactDownload)
}

// loggingMiddleware provides request logging
func (s *ArtifactsServerImpl) loggingMiddleware() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		s.logger.Info("artifacts server request",
			zap.String("method", param.Method),
			zap.String("path", param.Path),
			zap.Int("status", param.StatusCode),
			zap.Duration("latency", param.Latency),
			zap.String("client_ip", param.ClientIP),
		)
		return ""
	})
}

// handleHealth handles health check requests
func (s *ArtifactsServerImpl) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// handleArtifactDownload handles artifact download requests
func (s *ArtifactsServerImpl) handleArtifactDownload(c *gin.Context) {
	artifactID := c.Param("artifactId")
	filename := c.Param("filename")

	if artifactID == "" || filename == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "artifact ID and filename are required",
		})
		return
	}

	// Check if artifact exists
	ctx := c.Request.Context()
	exists, err := s.storage.Exists(ctx, artifactID, filename)
	if err != nil {
		s.logger.Error("failed to check artifact existence",
			zap.String("artifact_id", artifactID),
			zap.String("filename", filename),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to check artifact existence",
		})
		return
	}

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "artifact not found",
		})
		return
	}

	// Retrieve the artifact
	reader, err := s.storage.Retrieve(ctx, artifactID, filename)
	if err != nil {
		s.logger.Error("failed to retrieve artifact",
			zap.String("artifact_id", artifactID),
			zap.String("filename", filename),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to retrieve artifact",
		})
		return
	}
	defer func() {
		if closeErr := reader.Close(); closeErr != nil {
			s.logger.Error("failed to close artifact reader", zap.Error(closeErr))
		}
	}()

	// Set appropriate content type
	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Set headers
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	// Stream the file content
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

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
}

// ArtifactsServerImpl implements the ArtifactsServer interface
type ArtifactsServerImpl struct {
	config          *config.ArtifactsConfig
	logger          *zap.Logger
	artifactService ArtifactService
	server          *http.Server
	router          *gin.Engine
	cleanupTicker   *time.Ticker
	stopCleanup     chan struct{}
}

// NewArtifactsServer creates a new artifacts server instance with the provided service
func NewArtifactsServer(cfg *config.ArtifactsConfig, logger *zap.Logger, artifactService ArtifactService) ArtifactsServer {
	return &ArtifactsServerImpl{
		config:          cfg,
		logger:          logger,
		artifactService: artifactService,
		stopCleanup:     make(chan struct{}),
	}
}

// Start starts the artifacts server
func (s *ArtifactsServerImpl) Start(ctx context.Context) error {
	if s.artifactService == nil {
		return fmt.Errorf("artifact service must be set before starting artifacts server")
	}

	s.setupRouter()

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

	s.startCleanupProcess(ctx)

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

	s.stopCleanupProcess()

	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := s.server.Shutdown(shutdownCtx)
	if err != nil {
		s.logger.Error("failed to gracefully shutdown artifacts server", zap.Error(err))
		return err
	}

	if s.artifactService != nil {
		if err := s.artifactService.Close(); err != nil {
			s.logger.Error("failed to close artifact service", zap.Error(err))
		}
	}

	s.logger.Info("artifacts server stopped")
	return nil
}

// setupRouter configures the HTTP routes
func (s *ArtifactsServerImpl) setupRouter() {
	if s.config == nil {
		gin.SetMode(gin.ReleaseMode)
	}

	s.router = gin.New()
	s.router.Use(gin.Recovery())
	s.router.Use(s.loggingMiddleware())

	s.router.GET("/health", s.handleHealth)

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

	ctx := c.Request.Context()
	exists, err := s.artifactService.Exists(ctx, artifactID, filename)
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

	reader, err := s.artifactService.Retrieve(ctx, artifactID, filename)
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

	contentType := mime.TypeByExtension(filepath.Ext(filename))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}

// startCleanupProcess starts the background artifact cleanup process
func (s *ArtifactsServerImpl) startCleanupProcess(ctx context.Context) {
	cleanupInterval := s.config.RetentionConfig.CleanupInterval
	if cleanupInterval <= 0 {
		s.logger.Info("artifact cleanup disabled", zap.Duration("cleanup_interval", cleanupInterval))
		return
	}

	s.logger.Info("starting artifact cleanup process", zap.Duration("cleanup_interval", cleanupInterval))

	s.cleanupTicker = time.NewTicker(cleanupInterval)
	go func() {
		for {
			select {
			case <-s.stopCleanup:
				s.logger.Info("artifact cleanup shutting down")
				return
			case <-s.cleanupTicker.C:
				s.performCleanup(ctx)
			}
		}
	}()
}

// stopCleanupProcess stops the background artifact cleanup process
func (s *ArtifactsServerImpl) stopCleanupProcess() {
	if s.cleanupTicker != nil {
		s.cleanupTicker.Stop()
	}

	select {
	case s.stopCleanup <- struct{}{}:
	default:
	}
}

// performCleanup performs the actual artifact cleanup
func (s *ArtifactsServerImpl) performCleanup(ctx context.Context) {
	s.logger.Debug("starting artifact cleanup run")

	retentionConfig := s.config.RetentionConfig
	totalRemoved := 0

	if retentionConfig.MaxAge > 0 {
		removed, err := s.artifactService.CleanupExpiredArtifacts(ctx, retentionConfig.MaxAge)
		if err != nil {
			s.logger.Error("failed to cleanup expired artifacts", zap.Error(err))
		} else {
			totalRemoved += removed
			s.logger.Debug("cleaned up expired artifacts",
				zap.Int("removed_count", removed),
				zap.Duration("max_age", retentionConfig.MaxAge))
		}
	}

	if retentionConfig.MaxArtifacts > 0 {
		removed, err := s.artifactService.CleanupOldestArtifacts(ctx, retentionConfig.MaxArtifacts)
		if err != nil {
			s.logger.Error("failed to cleanup oldest artifacts", zap.Error(err))
		} else {
			totalRemoved += removed
			s.logger.Debug("cleaned up oldest artifacts",
				zap.Int("removed_count", removed),
				zap.Int("max_artifacts", retentionConfig.MaxArtifacts))
		}
	}

	if totalRemoved > 0 {
		s.logger.Info("artifact cleanup completed", zap.Int("total_removed", totalRemoved))
	}
}

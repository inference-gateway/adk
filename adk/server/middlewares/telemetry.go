package middlewares

import (
	"bytes"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"
	config "github.com/inference-gateway/a2a/adk/server/config"
	otel "github.com/inference-gateway/a2a/adk/server/otel"
	zap "go.uber.org/zap"
)

type Telemetry interface {
	Middleware() gin.HandlerFunc
}

type TelemetryImpl struct {
	cfg       config.Config
	telemetry otel.OpenTelemetry
	logger    *zap.Logger
}

func NewTelemetryMiddleware(cfg config.Config, telemetry otel.OpenTelemetry, logger *zap.Logger) (Telemetry, error) {
	return &TelemetryImpl{
		cfg:       cfg,
		telemetry: telemetry,
		logger:    logger,
	}, nil
}

// responseBodyWriter is a wrapper for the response writer that captures the body
type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

// Write captures the response body
func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (t *TelemetryImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip telemetry collection if not enabled or if not an A2A request
		if t.cfg.TelemetryConfig == nil || !t.cfg.TelemetryConfig.Enable || !strings.Contains(c.Request.URL.Path, "/a2a") {
			c.Next()
			return
		}

		// Record start time for duration calculation
		startTime := time.Now()

		// Create telemetry attributes with safe null checks
		var provider, model string
		if t.cfg.LLMProviderClientConfig != nil {
			provider = t.cfg.LLMProviderClientConfig.Provider
			model = t.cfg.LLMProviderClientConfig.Model
		}

		attrs := otel.TelemetryAttributes{
			Provider: provider,
			Model:    model,
			TaskID:   "", // Task ID will be set later if available from context
		}

		// Wrap response writer to capture response body and status code
		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = responseWriter

		// Record request count
		t.telemetry.RecordRequestCount(c.Request.Context(), attrs, c.Request.Method)

		// Continue processing
		c.Next()

		// Calculate request duration
		duration := time.Since(startTime)
		durationMs := float64(duration.Nanoseconds()) / float64(time.Millisecond)

		// Record metrics after request completion
		statusCode := responseWriter.Status()

		// Record response status
		t.telemetry.RecordResponseStatus(
			c.Request.Context(),
			attrs,
			c.Request.Method,
			c.Request.URL.Path,
			statusCode,
		)

		// Record request duration
		t.telemetry.RecordRequestDuration(
			c.Request.Context(),
			attrs,
			c.Request.Method,
			c.Request.URL.Path,
			durationMs,
		)

		// Log telemetry information
		t.logger.Debug("request telemetry recorded",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status_code", statusCode),
			zap.Float64("duration_ms", durationMs),
			zap.String("provider", attrs.Provider),
			zap.String("model", attrs.Model),
		)
	}
}

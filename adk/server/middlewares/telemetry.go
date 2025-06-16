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
		if !t.cfg.TelemetryConfig.Enable || !strings.Contains(c.Request.URL.Path, "/a2a") {
			c.Next()
			return
		}

		startTime := time.Now()

		attrs := otel.TelemetryAttributes{
			Provider: t.cfg.AgentConfig.Provider,
			Model:    t.cfg.AgentConfig.Model,
			TaskID:   "",
		}

		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = responseWriter

		t.telemetry.RecordRequestCount(c.Request.Context(), attrs, c.Request.Method)

		c.Next()

		duration := time.Since(startTime)
		durationMs := float64(duration.Nanoseconds()) / float64(time.Millisecond)

		statusCode := responseWriter.Status()

		t.telemetry.RecordResponseStatus(
			c.Request.Context(),
			attrs,
			c.Request.Method,
			c.Request.URL.Path,
			statusCode,
		)

		t.telemetry.RecordRequestDuration(
			c.Request.Context(),
			attrs,
			c.Request.Method,
			c.Request.URL.Path,
			durationMs,
		)

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

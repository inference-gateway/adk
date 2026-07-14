package middlewares

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"

	gin "github.com/gin-gonic/gin"
	otel "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	baggage "go.opentelemetry.io/otel/baggage"
	codes "go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
	trace "go.opentelemetry.io/otel/trace"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/server/config"
	adkotel "github.com/inference-gateway/adk/server/otel"
)

type Telemetry interface {
	Middleware() gin.HandlerFunc
}

type TelemetryImpl struct {
	cfg       config.Config
	telemetry adkotel.OpenTelemetry
	logger    *zap.Logger
	tracer    trace.Tracer
}

func NewTelemetryMiddleware(cfg config.Config, telemetry adkotel.OpenTelemetry, logger *zap.Logger) (Telemetry, error) {
	tp := telemetry.TracerProvider()
	if tp == nil {
		tp = otel.GetTracerProvider()
	}
	tracer := tp.Tracer("github.com/inference-gateway/adk/server/middlewares")
	return &TelemetryImpl{
		cfg:       cfg,
		telemetry: telemetry,
		logger:    logger,
		tracer:    tracer,
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
		if !strings.Contains(c.Request.URL.Path, "/a2a") {
			c.Next()
			return
		}

		startTime := time.Now()

		propagator := otel.GetTextMapPropagator()
		ctx := propagator.Extract(c.Request.Context(), propagationHeaderCarrier(c.Request.Header))
		c.Request = c.Request.WithContext(ctx)

		bag := baggage.FromContext(ctx)
		sessionIDKey := t.cfg.TelemetryConfig.SessionIDKey()
		toolCallIDKey := t.cfg.TelemetryConfig.ToolCallIDKey()
		sessionID := bag.Member(sessionIDKey)
		toolCallID := bag.Member(toolCallIDKey)

		spanAttrs := []attribute.KeyValue{
			semconv.HTTPRequestMethodKey.String(c.Request.Method),
			semconv.URLFullKey.String(c.Request.URL.String()),
			semconv.HTTPRouteKey.String(c.Request.URL.Path),
		}
		if sessionID.Value() != "" {
			spanAttrs = append(spanAttrs, attribute.String(sessionIDKey, sessionID.Value()))
		}
		if toolCallID.Value() != "" {
			spanAttrs = append(spanAttrs, attribute.String(toolCallIDKey, toolCallID.Value()))
		}

		ctx, span := t.tracer.Start(ctx, "a2a.request",
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(spanAttrs...),
		)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)

		attrs := adkotel.TelemetryAttributes{
			Provider: t.cfg.AgentConfig.Provider,
			Model:    t.cfg.AgentConfig.Model,
			TaskID:   "",
		}

		responseWriter := &responseBodyWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
		}
		c.Writer = responseWriter

		t.telemetry.RecordRequestCount(ctx, attrs, c.Request.Method)

		c.Next()

		duration := time.Since(startTime)
		durationMs := float64(duration.Nanoseconds()) / float64(time.Millisecond)

		statusCode := responseWriter.Status()

		span.SetAttributes(semconv.HTTPResponseStatusCodeKey.Int(statusCode))
		if statusCode >= 500 {
			span.SetAttributes(semconv.ErrorTypeKey.String(fmt.Sprintf("%d", statusCode)))
			span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", statusCode))
		}

		t.telemetry.RecordResponseStatus(
			ctx,
			attrs,
			c.Request.Method,
			c.Request.URL.Path,
			statusCode,
		)

		t.telemetry.RecordRequestDuration(
			ctx,
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

// propagationHeaderCarrier adapts http.Header to the OTel TextMapCarrier interface
type propagationHeaderCarrier http.Header

func (c propagationHeaderCarrier) Get(key string) string {
	return http.Header(c).Get(key)
}

func (c propagationHeaderCarrier) Set(key string, value string) {
	http.Header(c).Set(key, value)
}

func (c propagationHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

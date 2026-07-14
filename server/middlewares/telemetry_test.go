package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	gin "github.com/gin-gonic/gin"
	otel "go.opentelemetry.io/otel"
	propagation "go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	tracetest "go.opentelemetry.io/otel/sdk/trace/tracetest"
	zap "go.uber.org/zap"

	config "github.com/inference-gateway/adk/server/config"
	middlewares "github.com/inference-gateway/adk/server/middlewares"
	mocks "github.com/inference-gateway/adk/server/mocks"
)

func TestTelemetryMiddleware_RecordsRequestMetrics(t *testing.T) {
	cfg := config.Config{
		TelemetryConfig: config.TelemetryConfig{
			Enable: true,
		},
		AgentConfig: config.AgentConfig{
			Provider: "test-provider",
			Model:    "test-model",
		},
	}
	logger := zap.NewNop()
	mockOtel := &mocks.FakeOpenTelemetry{}

	telemetryMw, err := middlewares.NewTelemetryMiddleware(cfg, mockOtel, logger)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(telemetryMw.Middleware())
	router.POST("/a2a", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("POST", "/a2a", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, 1, mockOtel.RecordRequestCountCallCount())
	assert.Equal(t, 1, mockOtel.RecordResponseStatusCallCount())
	assert.Equal(t, 1, mockOtel.RecordRequestDurationCallCount())
}

func TestTelemetryMiddleware_NonA2APath(t *testing.T) {
	cfg := config.Config{
		TelemetryConfig: config.TelemetryConfig{
			Enable: true,
		},
	}
	logger := zap.NewNop()
	mockOtel := &mocks.FakeOpenTelemetry{}

	telemetryMw, err := middlewares.NewTelemetryMiddleware(cfg, mockOtel, logger)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(telemetryMw.Middleware())
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, 0, mockOtel.RecordRequestCountCallCount())
	assert.Equal(t, 0, mockOtel.RecordResponseStatusCallCount())
	assert.Equal(t, 0, mockOtel.RecordRequestDurationCallCount())
}

// TestTelemetryMiddleware_ExtractsTraceContextAndBaggage verifies that incoming
// W3C trace context and baggage are surfaced on the request span.
func TestTelemetryMiddleware_ExtractsTraceContextAndBaggage(t *testing.T) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	cfg := config.Config{
		AgentConfig: config.AgentConfig{
			Provider: "test-provider",
			Model:    "test-model",
		},
	}
	logger := zap.NewNop()
	mockOtel := &mocks.FakeOpenTelemetry{}
	mockOtel.TracerProviderReturns(tracerProvider)

	telemetryMw, err := middlewares.NewTelemetryMiddleware(cfg, mockOtel, logger)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(telemetryMw.Middleware())
	router.POST("/a2a", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	req, _ := http.NewRequest("POST", "/a2a", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("baggage", "infer.session.id=session-123,infer.tool.call.id=tool-456")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	spans := exporter.GetSpans()
	require.Len(t, spans, 1)

	span := spans[0]
	assert.Equal(t, "a2a.request", span.Name)
	assert.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", span.SpanContext.TraceID().String())
	assert.Equal(t, "00f067aa0ba902b7", span.Parent.SpanID().String())

	attrs := map[string]string{}
	for _, kv := range span.Attributes {
		attrs[string(kv.Key)] = kv.Value.AsString()
	}
	assert.Equal(t, "session-123", attrs["infer.session.id"])
	assert.Equal(t, "tool-456", attrs["infer.tool.call.id"])
}

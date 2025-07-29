package middlewares_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/server/middlewares"
	"github.com/inference-gateway/adk/server/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestTelemetryMiddleware_Disabled(t *testing.T) {
	cfg := config.Config{
		TelemetryConfig: config.TelemetryConfig{
			Enable: false,
		},
	}
	logger := zap.NewNop()
	mockOtel := &mocks.FakeOpenTelemetry{}

	telemetryMw, err := middlewares.NewTelemetryMiddleware(cfg, mockOtel, logger)
	assert.NoError(t, err)

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

	assert.Equal(t, 0, mockOtel.RecordRequestCountCallCount())
	assert.Equal(t, 0, mockOtel.RecordResponseStatusCallCount())
	assert.Equal(t, 0, mockOtel.RecordRequestDurationCallCount())
}

func TestTelemetryMiddleware_Enabled(t *testing.T) {
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
	assert.NoError(t, err)

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
	assert.NoError(t, err)

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

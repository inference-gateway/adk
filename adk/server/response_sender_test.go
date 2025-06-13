package server_test

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/inference-gateway/a2a/adk/server"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDefaultResponseSender_SendSuccess(t *testing.T) {
	tests := []struct {
		name   string
		id     interface{}
		result interface{}
	}{
		{
			name:   "send success with string id",
			id:     "test-id-string",
			result: map[string]interface{}{"status": "success", "data": "test data"},
		},
		{
			name:   "send success with int id",
			id:     123,
			result: map[string]interface{}{"message": "operation completed"},
		},
		{
			name:   "send success with nil id",
			id:     nil,
			result: "simple string result",
		},
		{
			name: "send success with complex result",
			id:   "complex-id",
			result: map[string]interface{}{
				"user": map[string]interface{}{
					"name":  "John Doe",
					"email": "john@example.com",
				},
				"status": "active",
				"items":  []string{"item1", "item2", "item3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			responseSender := server.NewDefaultResponseSender(logger)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)

			assert.NotPanics(t, func() {
				responseSender.SendSuccess(ctx, tt.id, tt.result)
			})

			assert.Equal(t, 200, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		})
	}
}

func TestDefaultResponseSender_SendError(t *testing.T) {
	tests := []struct {
		name    string
		id      interface{}
		code    int
		message string
	}{
		{
			name:    "send error with string id",
			id:      "error-id-string",
			code:    400,
			message: "Bad request error",
		},
		{
			name:    "send error with int id",
			id:      999,
			code:    500,
			message: "Internal server error",
		},
		{
			name:    "send error with nil id",
			id:      nil,
			code:    404,
			message: "Resource not found",
		},
		{
			name:    "send error with custom code",
			id:      "custom-error",
			code:    -32601,
			message: "Method not found",
		},
		{
			name:    "send error with empty message",
			id:      "empty-msg",
			code:    422,
			message: "",
		},
		{
			name:    "send error with long message",
			id:      "long-msg",
			code:    503,
			message: "This is a very long error message that contains multiple sentences and describes the error in great detail, including possible causes and suggested solutions for the user.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			responseSender := server.NewDefaultResponseSender(logger)

			gin.SetMode(gin.TestMode)
			w := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(w)

			assert.NotPanics(t, func() {
				responseSender.SendError(ctx, tt.id, tt.code, tt.message)
			})

			assert.Equal(t, 200, w.Code)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
		})
	}
}

func TestDefaultResponseSender_SendErrorWithSpecialCharacters(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	specialMessage := "Error with special characters: 中文, émojis 🚀, and \"quotes\""

	assert.NotPanics(t, func() {
		responseSender.SendError(ctx, "special-id", 400, specialMessage)
	})

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestDefaultResponseSender_SendSuccessWithNilResult(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	assert.NotPanics(t, func() {
		responseSender.SendSuccess(ctx, "nil-result", nil)
	})

	assert.Equal(t, 200, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
}

func TestNewDefaultResponseSender(t *testing.T) {
	logger := zap.NewNop()
	responseSender := server.NewDefaultResponseSender(logger)

	assert.NotNil(t, responseSender)
}

func TestNewDefaultResponseSender_WithNilLogger(t *testing.T) {
	assert.NotPanics(t, func() {
		responseSender := server.NewDefaultResponseSender(nil)
		assert.NotNil(t, responseSender)
	})
}

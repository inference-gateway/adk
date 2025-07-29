package server

import (
	gin "github.com/gin-gonic/gin"
	adk "github.com/inference-gateway/adk/types"
	zap "go.uber.org/zap"
)

// ResponseSender defines how to send JSON-RPC responses
type ResponseSender interface {
	// SendSuccess sends a JSON-RPC success response
	SendSuccess(c *gin.Context, id interface{}, result interface{})

	// SendError sends a JSON-RPC error response
	SendError(c *gin.Context, id interface{}, code int, message string)
}

// DefaultResponseSender implements the ResponseSender interface
type DefaultResponseSender struct {
	logger *zap.Logger
}

// NewDefaultResponseSender creates a new default response sender
func NewDefaultResponseSender(logger *zap.Logger) *DefaultResponseSender {
	return &DefaultResponseSender{
		logger: logger,
	}
}

// SendSuccess sends a JSON-RPC success response
func (rs *DefaultResponseSender) SendSuccess(c *gin.Context, id interface{}, result interface{}) {
	resp := adk.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	c.JSON(200, resp)
	rs.logger.Info("sending success response", zap.Any("id", id))
}

// SendError sends a JSON-RPC error response
func (rs *DefaultResponseSender) SendError(c *gin.Context, id interface{}, code int, message string) {
	resp := adk.JSONRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &adk.JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	c.JSON(200, resp) // JSON-RPC always returns 200 OK, errors are in the response body
	rs.logger.Error("sending error response", zap.Int("code", code), zap.String("message", message))
}

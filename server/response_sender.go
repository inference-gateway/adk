package server

import (
	"errors"

	gin "github.com/gin-gonic/gin"
	zap "go.uber.org/zap"

	types "github.com/inference-gateway/adk/types"
)

// ResponseSender defines how to send JSON-RPC responses
type ResponseSender interface {
	// SendSuccess sends a JSON-RPC success response
	SendSuccess(c *gin.Context, id any, result any)

	// SendError sends a JSON-RPC error response
	SendError(c *gin.Context, id any, code int, message string)
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
func (rs *DefaultResponseSender) SendSuccess(c *gin.Context, id any, result any) {
	resp := types.JSONRPCSuccessResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	c.JSON(200, resp)
	rs.logger.Info("sending success response", zap.Any("id", id))
}

// SendError sends a JSON-RPC error response
func (rs *DefaultResponseSender) SendError(c *gin.Context, id any, code int, message string) {
	resp := types.JSONRPCErrorResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   newJSONRPCError(code, message),
	}
	c.JSON(200, resp) // JSON-RPC always returns 200 OK, errors are in the response body
	rs.logger.Error("sending error response", zap.Int("code", code), zap.String("message", message))
}

// a2aErrorReasons maps the A2A error codes to their ErrorInfo reasons (spec sections 5.4 and 9.5).
var a2aErrorReasons = map[JRPCErrorCode]string{
	ErrTaskNotFound:                   "TASK_NOT_FOUND",
	ErrTaskNotCancelable:              "TASK_NOT_CANCELABLE",
	ErrPushNotificationNotSupported:   "PUSH_NOTIFICATION_NOT_SUPPORTED",
	ErrUnsupportedOperation:           "UNSUPPORTED_OPERATION",
	ErrExtendedAgentCardNotConfigured: "EXTENDED_AGENT_CARD_NOT_CONFIGURED",
	ErrVersionNotSupported:            "VERSION_NOT_SUPPORTED",
}

// newJSONRPCError builds a JSON-RPC error object. A2A errors carry a google.rpc.ErrorInfo
// in their data array, as spec section 9.5 requires.
func newJSONRPCError(code int, message string) types.JSONRPCError {
	jsonrpcErr := types.JSONRPCError{Code: code, Message: message}
	reason, isA2AError := a2aErrorReasons[JRPCErrorCode(code)]
	if !isA2AError {
		return jsonrpcErr
	}
	var data types.Value = []any{map[string]any{
		"@type":  "type.googleapis.com/google.rpc.ErrorInfo",
		"reason": reason,
		"domain": "a2a-protocol.org",
	}}
	jsonrpcErr.Data = &data
	return jsonrpcErr
}

// a2aErrorCode returns the A2A error code for the task manager's typed errors, or fallback.
func a2aErrorCode(err error, fallback JRPCErrorCode) JRPCErrorCode {
	var notFound *TaskNotFoundError
	var notCancelable *TaskNotCancelableError
	switch {
	case errors.As(err, &notFound):
		return ErrTaskNotFound
	case errors.As(err, &notCancelable):
		return ErrTaskNotCancelable
	default:
		return fallback
	}
}

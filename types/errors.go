package types

import (
	"errors"
	"fmt"
	"strconv"
)

// Sentinel errors for the JSON-RPC and A2A error codes of the A2A spec, so callers
// can match a failure with errors.Is instead of parsing the error message.
var (
	ErrParseError     = errors.New("parse error")
	ErrInvalidRequest = errors.New("invalid request")
	ErrMethodNotFound = errors.New("method not found")
	ErrInvalidParams  = errors.New("invalid params")
	ErrInternalError  = errors.New("internal error")
	ErrServerError    = errors.New("server error")

	ErrTaskNotFound                   = errors.New("task not found")
	ErrTaskNotCancelable              = errors.New("task not cancelable")
	ErrPushNotificationNotSupported   = errors.New("push notification not supported")
	ErrUnsupportedOperation           = errors.New("unsupported operation")
	ErrExtendedAgentCardNotConfigured = errors.New("extended agent card not configured")
	ErrVersionNotSupported            = errors.New("version not supported")
)

var codeToError = map[int]error{
	-32700: ErrParseError,
	-32600: ErrInvalidRequest,
	-32601: ErrMethodNotFound,
	-32602: ErrInvalidParams,
	-32603: ErrInternalError,
	-32000: ErrServerError,
	-32001: ErrTaskNotFound,
	-32002: ErrTaskNotCancelable,
	-32003: ErrPushNotificationNotSupported,
	-32004: ErrUnsupportedOperation,
	-32007: ErrExtendedAgentCardNotConfigured,
	-32009: ErrVersionNotSupported,
}

func (e *JSONRPCError) Error() string {
	return fmt.Sprintf("A2A error: %s (code: %d)", e.Message, e.Code)
}

// Unwrap exposes the sentinel for the error code, or nil for an unknown code.
func (e *JSONRPCError) Unwrap() error {
	return codeToError[e.Code]
}

// HTTPStatusError reports an unexpected HTTP status from an A2A endpoint.
// Operation names the request that failed and is empty for the JSON-RPC endpoint.
type HTTPStatusError struct {
	Operation  string
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	msg := "unexpected status code"
	if e.Operation != "" {
		msg += " for " + e.Operation
	}
	msg += ": " + strconv.Itoa(e.StatusCode)
	if e.Body != "" {
		msg += ", body: " + e.Body
	}
	return msg
}

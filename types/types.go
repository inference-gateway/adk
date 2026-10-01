package types

// Health status constants
const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
)

// CloudEvent type constants for agent streaming operations
const (
	EventDelta              = "adk.agent.delta"
	EventIterationCompleted = "adk.agent.iteration.completed"
	EventToolStarted        = "adk.agent.tool.started"
	EventToolCompleted      = "adk.agent.tool.completed"
	EventToolFailed         = "adk.agent.tool.failed"
	EventToolResult         = "adk.agent.tool.result"
	EventInputRequired      = "adk.agent.input.required"
	EventTaskInterrupted    = "adk.agent.task.interrupted"
	EventTaskStatusChanged  = "adk.agent.task.status.changed"
	EventStreamFailed       = "adk.agent.stream.failed"
)

// Tool name constants
const (
	ToolInputRequired = "input_required"
)

// GetContextID returns the task's context ID, or "" when it is unset.
// The ADK always assigns one when it creates a task; the accessor exists because
// A2A v1.0 made the field optional on the wire.
func (t *Task) GetContextID() string {
	if t == nil || t.ContextID == nil {
		return ""
	}
	return *t.ContextID
}

// IsTerminal reports whether a task in this state will never change state again.
// A2A v1.0 dropped the `final` flag from status updates; terminal state is the signal.
func (s TaskState) IsTerminal() bool {
	switch s {
	case TaskStateCompleted, TaskStateFailed, TaskStateCanceled, TaskStateRejected:
		return true
	}
	return false
}

// A discriminated union representing all possible JSON-RPC 2.0 responses
// for the A2A specification methods.
type JSONRPCResponse any

// Represents a successful JSON-RPC 2.0 Response object.
type JSONRPCSuccessResponse struct {
	ID      any    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result"`
}

// An error indicating that the server received invalid JSON.
type JSONParseError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// Represents a JSON-RPC 2.0 Error object, included in an error response.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Data    *any   `json:"data,omitempty"`
	Message string `json:"message"`
}

// Represents a JSON-RPC 2.0 Error Response object.
type JSONRPCErrorResponse struct {
	Error   any    `json:"error"`
	ID      any    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
}

// Defines the base structure for any JSON-RPC 2.0 request, response, or notification.
type JSONRPCMessage struct {
	ID      *any   `json:"id,omitempty"`
	JSONRPC string `json:"jsonrpc"`
}

// Represents a JSON-RPC 2.0 Request object.
type JSONRPCRequest struct {
	ID      *any           `json:"id,omitempty"`
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

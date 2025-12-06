package types

// MessagePartKind represents the different types of message parts supported by A2A protocol.
// Based on the A2A specification: https://google-a2a.github.io/A2A/latest/
type MessagePartKind string

// MessagePartKind enum values for the three official message part types
const (
	// MessagePartKindText represents a text segment within message parts
	MessagePartKindText MessagePartKind = "text"

	// MessagePartKindFile represents a file segment within message parts
	MessagePartKindFile MessagePartKind = "file"

	// MessagePartKindData represents a structured data segment within message parts
	MessagePartKindData MessagePartKind = "data"
)

// String returns the string representation of the MessagePartKind
func (k MessagePartKind) String() string {
	return string(k)
}

// IsValid checks if the MessagePartKind is one of the supported values
func (k MessagePartKind) IsValid() bool {
	switch k {
	case MessagePartKindText, MessagePartKindFile, MessagePartKindData:
		return true
	default:
		return false
	}
}

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

// Compatibility types - These types were removed from the generated schema
// but are still used in the codebase for backward compatibility

// TextPart represents a text message part
type TextPart struct {
	Text string `json:"text"`
}

// FileWithBytes represents a file provided as bytes
type FileWithBytes struct {
	Name     string `json:"name"`
	MimeType string `json:"mime_type,omitempty"`
	Bytes    []byte `json:"bytes"`
}

// FileWithUri represents a file provided as a URI
type FileWithUri struct {
	Uri      string `json:"uri"`
	Name     string `json:"name,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// TaskState represents the state of a task
type TaskState string

const (
	TaskStatePending      TaskState = "pending"
	TaskStateRunning      TaskState = "running"
	TaskStateWorking      TaskState = "working"
	TaskStateInputNeeded  TaskState = "input_needed"
	TaskStateCompleted    TaskState = "completed"
	TaskStateFailed       TaskState = "failed"
	TaskStateCancelled    TaskState = "cancelled"
	TaskStateCanceled     TaskState = "canceled"  // Alias for TaskStateCancelled
	TaskStateUnspecified  TaskState = "unspecified"
)

// MessageKind represents the kind of message
type MessageKind string

const (
	MessageKindRequest  MessageKind = "request"
	MessageKindResponse MessageKind = "response"
)

// JSONRPC types for backward compatibility

// JSONRPCRequest represents a JSON-RPC 2.0 request
type JSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

// JSONRPCSuccessResponse represents a successful JSON-RPC 2.0 response
type JSONRPCSuccessResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result"`
	ID      any    `json:"id"`
}

// JSONRPCError represents a JSON-RPC 2.0 error object
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// JSONRPCErrorResponse represents a JSON-RPC 2.0 error response
type JSONRPCErrorResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	Error   *JSONRPCError `json:"error"`
	ID      any           `json:"id"`
}

// Request/Response parameter types

// MessageSendParams represents parameters for sending a message
type MessageSendParams struct {
	Configuration *SendMessageConfiguration `json:"configuration,omitempty"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
	Request       *Message                  `json:"request"`
}

// MessageSendConfiguration is an alias for SendMessageConfiguration
type MessageSendConfiguration = SendMessageConfiguration

// TaskIdParams represents parameters that include a task ID
type TaskIdParams struct {
	Name string `json:"name"`
}

// TaskQueryParams represents parameters for querying a task
type TaskQueryParams struct {
	Name          string `json:"name"`
	HistoryLength *int   `json:"history_length,omitempty"`
}

// TaskListParams represents parameters for listing tasks
type TaskListParams struct {
	ContextID        *string `json:"context_id,omitempty"`
	HistoryLength    *int    `json:"history_length,omitempty"`
	IncludeArtifacts *bool   `json:"include_artifacts,omitempty"`
	LastUpdatedAfter *string `json:"last_updated_after,omitempty"`
	PageSize         *int    `json:"page_size,omitempty"`
	PageToken        *string `json:"page_token,omitempty"`
	Status           *any    `json:"status,omitempty"`
}

// TaskList represents a list of tasks
type TaskList struct {
	Tasks         []Task  `json:"tasks"`
	NextPageToken *string `json:"next_page_token,omitempty"`
	PageSize      *int    `json:"page_size,omitempty"`
	TotalSize     *int    `json:"total_size,omitempty"`
}

// Push notification config params

// GetTaskPushNotificationConfigParams represents parameters for getting push notification config
type GetTaskPushNotificationConfigParams struct {
	Name string `json:"name"`
}

// ListTaskPushNotificationConfigParams represents parameters for listing push notification configs
type ListTaskPushNotificationConfigParams struct {
	PageSize  *int    `json:"page_size,omitempty"`
	PageToken *string `json:"page_token,omitempty"`
	Parent    *string `json:"parent,omitempty"`
}

// DeleteTaskPushNotificationConfigParams represents parameters for deleting push notification config
type DeleteTaskPushNotificationConfigParams struct {
	Name string `json:"name"`
}

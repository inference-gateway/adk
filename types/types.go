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

// Defines configuration options for a `message/send` or `message/stream` request.
type MessageSendConfiguration struct {
	AcceptedOutputModes    []string                `json:"acceptedOutputModes,omitempty"`
	Blocking               *bool                   `json:"blocking,omitempty"`
	HistoryLength          *int                    `json:"historyLength,omitempty"`
	PushNotificationConfig *PushNotificationConfig `json:"pushNotificationConfig,omitempty"`
}

// Defines the parameters for a request to send a message to an agent. This can be used
// to create a new task, continue an existing one, or restart a task.
type MessageSendParams struct {
	Configuration *MessageSendConfiguration `json:"configuration,omitempty"`
	Message       Message                   `json:"message"`
	Metadata      map[string]any            `json:"metadata,omitempty"`
}

// Defines parameters for querying a task, with an option to limit history length.
type TaskQueryParams struct {
	HistoryLength *int           `json:"historyLength,omitempty"`
	ID            string         `json:"id"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// Parameters for listing tasks with optional filtering and pagination.
type TaskListParams struct {
	ContextID *string        `json:"contextId,omitempty"`
	Limit     int            `json:"limit,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Offset    int            `json:"offset,omitempty"`
	State     *TaskState     `json:"state,omitempty"`
}

// Parameters for task operations that require only a task ID.
type TaskIdParams struct {
	ID       string         `json:"id"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TaskList represents a list of tasks with pagination info (alias for generated type)
type TaskList = ListTasksResponse

// GetTaskPushNotificationConfigParams is an alias for GetTaskPushNotificationConfigRequest
type GetTaskPushNotificationConfigParams = GetTaskPushNotificationConfigRequest

// ListTaskPushNotificationConfigParams is an alias for ListTaskPushNotificationConfigRequest
type ListTaskPushNotificationConfigParams = ListTaskPushNotificationConfigRequest

// DeleteTaskPushNotificationConfigParams is an alias for DeleteTaskPushNotificationConfigRequest
type DeleteTaskPushNotificationConfigParams = DeleteTaskPushNotificationConfigRequest

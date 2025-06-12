package server

import (
	"context"
	"net/http"

	gin "github.com/gin-gonic/gin"
	adk "github.com/inference-gateway/a2a/adk"
)

// A2AServer defines the interface for an A2A-compatible server
// This interface allows for easy testing and different implementations
type A2AServer interface {
	// SetupRouter configures the HTTP router with A2A endpoints
	SetupRouter(oidcAuthenticator OIDCAuthenticator) *gin.Engine

	// Start starts the A2A server on the configured port
	Start(ctx context.Context) error

	// Stop gracefully stops the A2A server
	Stop(ctx context.Context) error

	// GetAgentCard returns the agent's capabilities and metadata
	GetAgentCard() adk.AgentCard

	// ProcessTask processes a task with the given message
	ProcessTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error)

	// StartTaskProcessor starts the background task processor
	StartTaskProcessor(ctx context.Context)
}

// TaskHandler defines how to handle task processing
// This interface should be implemented by domain-specific task handlers
type TaskHandler interface {
	// HandleTask processes a task and returns the updated task
	// This is where the main business logic should be implemented
	HandleTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error)
}

// MessageHandler defines how to handle different types of A2A messages
type MessageHandler interface {
	// HandleMessageSend processes message/send requests
	HandleMessageSend(ctx context.Context, params adk.MessageSendParams) (*adk.Task, error)

	// HandleMessageStream processes message/stream requests (for streaming responses)
	HandleMessageStream(ctx context.Context, params adk.MessageSendParams) error
}

// TaskManager defines task lifecycle management
type TaskManager interface {
	// CreateTask creates a new task and stores it
	CreateTask(contextID string, state adk.TaskState, message *adk.Message) *adk.Task

	// UpdateTask updates an existing task
	UpdateTask(taskID string, state adk.TaskState, message *adk.Message) error

	// GetTask retrieves a task by ID
	GetTask(taskID string) (*adk.Task, bool)

	// CancelTask cancels a task
	CancelTask(taskID string) error

	// CleanupCompletedTasks removes old completed tasks from memory
	CleanupCompletedTasks()
}

// A2ARequestHandler defines how to handle different A2A protocol methods
type A2ARequestHandler interface {
	// HandleA2ARequest is the main entry point for A2A protocol requests
	HandleA2ARequest(c *gin.Context)

	// HandleTaskGet processes tasks/get requests
	HandleTaskGet(c *gin.Context, req adk.JSONRPCRequest)

	// HandleTaskCancel processes tasks/cancel requests
	HandleTaskCancel(c *gin.Context, req adk.JSONRPCRequest)
}

// ResponseSender defines how to send JSON-RPC responses
type ResponseSender interface {
	// SendSuccess sends a JSON-RPC success response
	SendSuccess(c *gin.Context, id interface{}, result interface{})

	// SendError sends a JSON-RPC error response
	SendError(c *gin.Context, id interface{}, code int, message string)
}

// HTTPServer defines the interface for HTTP server operations
type HTTPServer interface {
	// ListenAndServe starts the HTTP server
	ListenAndServe() error

	// Shutdown gracefully shuts down the HTTP server
	Shutdown(ctx context.Context) error

	// Handler returns the HTTP handler
	Handler() http.Handler
}

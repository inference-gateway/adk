package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	adk "github.com/inference-gateway/a2a/adk"
	zap "go.uber.org/zap"
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

	// SetLLMClient sets the LLM client for AI/ML processing
	SetLLMClient(client LLMClient)

	// GetLLMClient returns the configured LLM client
	GetLLMClient() LLMClient

	// SetTaskHandler sets the task handler for processing tasks
	SetTaskHandler(handler TaskHandler)

	// GetTaskHandler returns the configured task handler
	GetTaskHandler() TaskHandler
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

// TaskResultProcessor defines how to process tool call results for task completion
type TaskResultProcessor interface {
	// ProcessToolResult processes a tool call result and returns a completion message if the task should be completed
	// Returns nil if the task should continue processing
	ProcessToolResult(toolCallResult string) *adk.Message
}

// AgentInfoProvider defines how to provide agent-specific information
type AgentInfoProvider interface {
	// GetAgentCard returns the agent's capabilities and metadata
	GetAgentCard(baseConfig Config) adk.AgentCard
}

// JRPCErrorCode represents JSON-RPC error codes
type JRPCErrorCode int

const (
	ErrParseError     JRPCErrorCode = -32700
	ErrInvalidRequest JRPCErrorCode = -32600
	ErrMethodNotFound JRPCErrorCode = -32601
	ErrInvalidParams  JRPCErrorCode = -32602
	ErrInternalError  JRPCErrorCode = -32603
	ErrServerError    JRPCErrorCode = -32000
)

// QueuedTask represents a task in the processing queue
type QueuedTask struct {
	Task      *adk.Task
	RequestID interface{}
}

// DefaultA2AServer implements the A2AServer interface
// This is the main implementation inspired by the reference A2A sample
type DefaultA2AServer struct {
	cfg            Config
	logger         *zap.Logger
	taskHandler    TaskHandler
	taskManager    TaskManager
	messageHandler MessageHandler
	responseSender ResponseSender
	llmClient      LLMClient

	// Server state
	httpServer *http.Server
	taskQueue  chan *QueuedTask

	// Optional processors
	taskResultProcessor TaskResultProcessor
	agentInfoProvider   AgentInfoProvider
}

// NewDefaultA2AServer creates a new default A2A server implementation
func NewDefaultA2AServer(cfg Config, logger *zap.Logger) *DefaultA2AServer {
	server := &DefaultA2AServer{
		cfg:       cfg,
		logger:    logger,
		taskQueue: make(chan *QueuedTask, cfg.QueueConfig.MaxSize),
	}

	// Initialize components with dependency injection
	server.taskManager = NewDefaultTaskManager(logger)
	server.messageHandler = NewDefaultMessageHandler(logger)
	server.responseSender = NewDefaultResponseSender(logger)
	server.taskHandler = NewDefaultTaskHandler(logger)

	return server
}

// SetTaskHandler allows injecting a custom task handler
func (s *DefaultA2AServer) SetTaskHandler(handler TaskHandler) {
	s.taskHandler = handler
}

// SetTaskResultProcessor sets the task result processor for custom business logic
func (s *DefaultA2AServer) SetTaskResultProcessor(processor TaskResultProcessor) {
	s.taskResultProcessor = processor
}

// SetAgentInfoProvider sets the agent info provider for custom agent metadata
func (s *DefaultA2AServer) SetAgentInfoProvider(provider AgentInfoProvider) {
	s.agentInfoProvider = provider
}

// SetLLMClient sets the LLM client for AI/ML processing
func (s *DefaultA2AServer) SetLLMClient(client LLMClient) {
	s.llmClient = client
}

// GetLLMClient returns the configured LLM client
func (s *DefaultA2AServer) GetLLMClient() LLMClient {
	return s.llmClient
}

// GetTaskHandler returns the configured task handler
func (s *DefaultA2AServer) GetTaskHandler() TaskHandler {
	return s.taskHandler
}

// SetupRouter configures the HTTP router with A2A endpoints
func (s *DefaultA2AServer) SetupRouter(oidcAuthenticator OIDCAuthenticator) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	if s.cfg.Debug {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.GET("/.well-known/agent.json", s.handleAgentInfo)

	if s.cfg.AuthConfig.Enable {
		r.POST("/a2a", oidcAuthenticator.Middleware(), s.handleA2ARequest)
	} else {
		r.POST("/a2a", s.handleA2ARequest)
	}

	return r
}

// Start starts the A2A server
func (s *DefaultA2AServer) Start(ctx context.Context) error {
	router := s.SetupRouter(nil)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%s", s.cfg.Port),
		Handler:      router,
		ReadTimeout:  s.cfg.ServerConfig.ReadTimeout,
		WriteTimeout: s.cfg.ServerConfig.WriteTimeout,
		IdleTimeout:  s.cfg.ServerConfig.IdleTimeout,
	}

	s.logger.Info("starting A2A server", zap.String("port", s.cfg.Port))

	go s.StartTaskProcessor(ctx)

	if s.cfg.TLSConfig.Enable {
		return s.httpServer.ListenAndServeTLS(s.cfg.TLSConfig.CertPath, s.cfg.TLSConfig.KeyPath)
	}

	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the A2A server
func (s *DefaultA2AServer) Stop(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	s.logger.Info("stopping A2A server")
	return s.httpServer.Shutdown(ctx)
}

// GetAgentCard returns the agent's capabilities and metadata
func (s *DefaultA2AServer) GetAgentCard() adk.AgentCard {
	if s.agentInfoProvider != nil {
		return s.agentInfoProvider.GetAgentCard(s.cfg)
	}

	return adk.AgentCard{
		Name:        s.cfg.AgentName,
		Description: s.cfg.AgentDescription,
		URL:         s.cfg.AgentURL,
		Version:     s.cfg.AgentVersion,
		Capabilities: adk.AgentCapabilities{
			Streaming:              &s.cfg.CapabilitiesConfig.Streaming,
			PushNotifications:      &s.cfg.CapabilitiesConfig.PushNotifications,
			StateTransitionHistory: &s.cfg.CapabilitiesConfig.StateTransitionHistory,
		},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             []adk.AgentSkill{},
	}
}

// ProcessTask processes a task with the given message
func (s *DefaultA2AServer) ProcessTask(ctx context.Context, task *adk.Task, message *adk.Message) (*adk.Task, error) {
	return s.taskHandler.HandleTask(ctx, task, message)
}

// StartTaskProcessor starts the background task processing goroutine
func (s *DefaultA2AServer) StartTaskProcessor(ctx context.Context) {
	s.logger.Info("starting task processor")

	go s.startTaskCleanup(ctx)

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("task processor shutting down")
			return
		case queuedTask := <-s.taskQueue:
			s.processQueuedTask(ctx, queuedTask)
		}
	}
}

// processQueuedTask processes a single queued task
func (s *DefaultA2AServer) processQueuedTask(ctx context.Context, queuedTask *QueuedTask) {
	task := queuedTask.Task
	message := &adk.Message{
		Kind:      "message",
		MessageID: uuid.New().String(),
		Role:      "user",
		Parts:     []adk.Part{},
	}

	s.logger.Info("processing task", zap.String("task_id", task.ID))

	err := s.taskManager.UpdateTask(task.ID, adk.TaskStateWorking, nil)
	if err != nil {
		s.logger.Error("failed to update task state", zap.Error(err))
		return
	}

	updatedTask, err := s.taskHandler.HandleTask(ctx, task, message)
	if err != nil {
		s.logger.Error("failed to process task", zap.Error(err), zap.String("task_id", task.ID))
		updateErr := s.taskManager.UpdateTask(task.ID, adk.TaskStateFailed, &adk.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": err.Error(),
				},
			},
		})
		if updateErr != nil {
			s.logger.Error("failed to update task to failed state", zap.Error(updateErr), zap.String("task_id", task.ID))
		}
		return
	}

	if err := s.taskManager.UpdateTask(updatedTask.ID, updatedTask.Status.State, nil); err != nil {
		s.logger.Error("failed to update task status", zap.Error(err), zap.String("task_id", updatedTask.ID))
		return
	}
	s.logger.Info("task processed successfully", zap.String("task_id", task.ID))
}

// startTaskCleanup starts the background task cleanup process
func (s *DefaultA2AServer) startTaskCleanup(ctx context.Context) {
	ticker := time.NewTicker(s.cfg.QueueConfig.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("task cleanup shutting down")
			return
		case <-ticker.C:
			s.taskManager.CleanupCompletedTasks()
		}
	}
}

// handleAgentInfo returns agent capabilities and metadata
func (s *DefaultA2AServer) handleAgentInfo(c *gin.Context) {
	s.logger.Info("agent info requested")
	agentCard := s.GetAgentCard()
	c.JSON(http.StatusOK, agentCard)
}

// handleA2ARequest processes A2A protocol requests
func (s *DefaultA2AServer) handleA2ARequest(c *gin.Context) {
	var req adk.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to parse json request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrParseError), "parse error")
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.ID == nil {
		id := interface{}(uuid.New().String())
		req.ID = &id
	}

	s.logger.Info("received a2a request",
		zap.String("method", req.Method),
		zap.Any("id", req.ID))

	switch req.Method {
	case "message/send":
		s.handleMessageSend(c, req)
	case "message/stream":
		s.handleMessageStream(c, req)
	case "tasks/get":
		s.handleTaskGet(c, req)
	case "tasks/cancel":
		s.handleTaskCancel(c, req)
	default:
		s.logger.Warn("unknown method requested", zap.String("method", req.Method))
		s.responseSender.SendError(c, req.ID, int(ErrMethodNotFound), "method not found")
	}
}

// handleMessageSend processes message/send requests
func (s *DefaultA2AServer) handleMessageSend(c *gin.Context, req adk.JSONRPCRequest) {
	var params adk.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse message/send request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	task, err := s.messageHandler.HandleMessageSend(c.Request.Context(), params)
	if err != nil {
		s.logger.Error("failed to handle message send", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	queuedTask := &QueuedTask{
		Task:      task,
		RequestID: req.ID,
	}

	select {
	case s.taskQueue <- queuedTask:
		s.logger.Info("task queued for processing", zap.String("task_id", task.ID))
	default:
		s.logger.Error("task queue is full")
		err := s.taskManager.UpdateTask(task.ID, adk.TaskStateFailed, &adk.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Task queue is full. Please try again later.",
				},
			},
		})
		if err != nil {
			s.logger.Error("failed to update task to failed state due to full queue", zap.Error(err), zap.String("task_id", task.ID))
		}
	}

	s.responseSender.SendSuccess(c, req.ID, *task)
}

// handleMessageStream processes message/stream requests
func (s *DefaultA2AServer) handleMessageStream(c *gin.Context, req adk.JSONRPCRequest) {
	s.logger.Info("streaming not implemented yet")
	s.responseSender.SendError(c, req.ID, int(ErrServerError), "streaming not implemented")
}

// handleTaskGet processes tasks/get requests
func (s *DefaultA2AServer) handleTaskGet(c *gin.Context, req adk.JSONRPCRequest) {
	var params adk.TaskQueryParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/get request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("retrieving task", zap.String("task_id", params.ID))

	task, exists := s.taskManager.GetTask(params.ID)
	if !exists {
		s.logger.Error("task not found", zap.String("task_id", params.ID))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "task not found")
		return
	}

	s.logger.Info("task retrieved successfully", zap.String("task_id", params.ID), zap.String("status", string(task.Status.State)))
	s.responseSender.SendSuccess(c, req.ID, *task)
}

// handleTaskCancel processes tasks/cancel requests
func (s *DefaultA2AServer) handleTaskCancel(c *gin.Context, req adk.JSONRPCRequest) {
	var params adk.TaskIdParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/cancel request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("canceling task", zap.String("task_id", params.ID))

	err = s.taskManager.CancelTask(params.ID)
	if err != nil {
		s.logger.Error("failed to cancel task", zap.Error(err), zap.String("task_id", params.ID))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), err.Error())
		return
	}

	task, _ := s.taskManager.GetTask(params.ID)
	s.responseSender.SendSuccess(c, req.ID, *task)
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	config "github.com/inference-gateway/adk/server/config"
	middlewares "github.com/inference-gateway/adk/server/middlewares"
	otel "github.com/inference-gateway/adk/server/otel"
	types "github.com/inference-gateway/adk/types"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"
)

// A2AServer defines the interface for an A2A-compatible server
type A2AServer interface {
	// Start starts the A2A server on the configured port
	Start(ctx context.Context) error

	// Stop gracefully stops the A2A server
	Stop(ctx context.Context) error

	// GetAgentCard returns the agent's capabilities and metadata
	// Returns nil if no agent card has been explicitly set
	GetAgentCard() *types.AgentCard

	// ProcessTask processes a task with the given message
	ProcessTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error)

	// StartTaskProcessor starts the background task processor
	StartTaskProcessor(ctx context.Context)

	// SetTaskHandler sets the task handler for processing tasks
	SetTaskHandler(handler TaskHandler)

	// GetTaskHandler returns the configured task handler
	GetTaskHandler() TaskHandler

	// SetAgent sets the OpenAI-compatible agent for processing tasks
	SetAgent(agent OpenAICompatibleAgent)

	// GetAgent returns the configured OpenAI-compatible agent
	GetAgent() OpenAICompatibleAgent

	// SetAgentName sets the agent's name dynamically
	SetAgentName(name string)

	// SetAgentDescription sets the agent's description dynamically
	SetAgentDescription(description string)

	// SetAgentURL sets the agent's URL dynamically
	SetAgentURL(url string)

	// SetAgentVersion sets the agent's version dynamically
	SetAgentVersion(version string)

	// SetAgentCard sets a custom agent card that overrides the default card generation
	SetAgentCard(agentCard types.AgentCard)

	// LoadAgentCardFromFile loads and sets an agent card from a JSON file
	// The optional overrides map allows dynamic replacement of JSON attribute values
	LoadAgentCardFromFile(filePath string, overrides map[string]interface{}) error
}

// TaskResultProcessor defines how to process tool call results for task completion
type TaskResultProcessor interface {
	// ProcessToolResult processes a tool call result and returns a completion message if the task should be completed
	// Returns nil if the task should continue processing
	ProcessToolResult(toolCallResult string) *types.Message
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
	Task      *types.Task
	RequestID interface{}
}

type A2AServerImpl struct {
	cfg            *config.Config
	logger         *zap.Logger
	taskHandler    TaskHandler
	taskManager    TaskManager
	messageHandler MessageHandler
	responseSender ResponseSender
	otel           otel.OpenTelemetry

	// Server state
	httpServer    *http.Server
	metricsServer *http.Server
	taskQueue     chan *QueuedTask

	// Optional processors
	taskResultProcessor TaskResultProcessor
	agent               OpenAICompatibleAgent

	// Custom agent card
	customAgentCard *types.AgentCard
}

var _ A2AServer = (*A2AServerImpl)(nil)

// NewA2AServer creates a new A2A server with the provided configuration and logger
func NewA2AServer(cfg *config.Config, logger *zap.Logger, otel otel.OpenTelemetry) *A2AServerImpl {
	if cfg.AgentName == "" {
		cfg.AgentName = BuildAgentName
	}
	if cfg.AgentDescription == "" {
		cfg.AgentDescription = BuildAgentDescription
	}
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = BuildAgentVersion
	}

	server := &A2AServerImpl{
		cfg:       cfg,
		logger:    logger,
		otel:      otel,
		taskQueue: make(chan *QueuedTask, cfg.QueueConfig.MaxSize),
	}

	maxConversationHistory := cfg.AgentConfig.MaxConversationHistory
	server.taskManager = NewDefaultTaskManager(logger, maxConversationHistory)
	server.messageHandler = NewDefaultMessageHandler(logger, server.taskManager, cfg)
	server.responseSender = NewDefaultResponseSender(logger)
	server.taskHandler = NewDefaultTaskHandler(logger)

	return server
}

// NewA2AServerWithAgent creates a new A2A server with an optional OpenAI-compatible agent
func NewA2AServerWithAgent(cfg *config.Config, logger *zap.Logger, otel otel.OpenTelemetry, agent OpenAICompatibleAgent) *A2AServerImpl {
	server := NewA2AServer(cfg, logger, otel)

	if agent != nil {
		server.agent = agent
		server.taskHandler = NewAgentTaskHandler(logger, agent)
		server.messageHandler = NewDefaultMessageHandlerWithAgent(logger, server.taskManager, agent, cfg)
	}

	return server
}

// NewDefaultA2AServer creates a new default A2A server implementation
func NewDefaultA2AServer(cfg *config.Config) *A2AServerImpl {
	var finalCfg *config.Config
	var err error

	finalCfg, err = config.LoadWithLookuper(context.Background(), cfg, envconfig.OsLookuper())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	var logger *zap.Logger
	if finalCfg.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to initialize logger: %v", err)
	}

	var telemetryInstance otel.OpenTelemetry
	if finalCfg.TelemetryConfig.Enable {
		telemetryInstance, err = otel.NewOpenTelemetry(finalCfg, logger)
		if err != nil {
			logger.Fatal("failed to initialize telemetry", zap.Error(err))
		}
		metricsAddr := finalCfg.TelemetryConfig.MetricsConfig.Host + ":" + finalCfg.TelemetryConfig.MetricsConfig.Port
		logger.Info("telemetry enabled - metrics will be available", zap.String("metrics_url", metricsAddr+"/metrics"))
	}

	server := NewA2AServer(finalCfg, logger, telemetryInstance)

	return server
}

// NewA2AServerEnvironmentAware creates a new A2A server with environment-aware configuration.
func NewA2AServerEnvironmentAware(cfg *config.Config, logger *zap.Logger, otel otel.OpenTelemetry) *A2AServerImpl {
	var err error
	cfg, err = config.LoadWithLookuper(context.Background(), cfg, envconfig.OsLookuper())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	server := &A2AServerImpl{
		cfg:       cfg,
		logger:    logger,
		otel:      otel,
		taskQueue: make(chan *QueuedTask, cfg.QueueConfig.MaxSize),
	}

	server.taskManager = NewDefaultTaskManager(logger, cfg.AgentConfig.MaxConversationHistory)
	server.messageHandler = NewDefaultMessageHandler(logger, server.taskManager, cfg)
	server.responseSender = NewDefaultResponseSender(logger)
	server.taskHandler = NewDefaultTaskHandler(logger)

	return server
}

// SetTaskHandler allows injecting a custom task handler
func (s *A2AServerImpl) SetTaskHandler(handler TaskHandler) {
	s.taskHandler = handler
}

// GetTaskHandler returns the configured task handler
func (s *A2AServerImpl) GetTaskHandler() TaskHandler {
	return s.taskHandler
}

// SetAgent sets the OpenAI-compatible agent for processing tasks
func (s *A2AServerImpl) SetAgent(agent OpenAICompatibleAgent) {
	s.agent = agent
	s.messageHandler = NewDefaultMessageHandlerWithAgent(s.logger, s.taskManager, agent, s.cfg)
}

// GetAgent returns the configured OpenAI-compatible agent
func (s *A2AServerImpl) GetAgent() OpenAICompatibleAgent {
	return s.agent
}

// SetTaskResultProcessor sets the task result processor for custom business logic
func (s *A2AServerImpl) SetTaskResultProcessor(processor TaskResultProcessor) {
	s.taskResultProcessor = processor
}

// SetAgentName sets the agent's name dynamically
func (s *A2AServerImpl) SetAgentName(name string) {
	s.cfg.AgentName = name
}

// SetAgentDescription sets the agent's description dynamically
func (s *A2AServerImpl) SetAgentDescription(description string) {
	s.cfg.AgentDescription = description
}

// SetAgentURL sets the agent's URL dynamically
func (s *A2AServerImpl) SetAgentURL(url string) {
	s.cfg.AgentURL = url
}

// SetAgentVersion sets the agent's version dynamically
func (s *A2AServerImpl) SetAgentVersion(version string) {
	s.cfg.AgentVersion = version
}

// SetAgentCard sets a custom agent card that overrides the default card generation
func (s *A2AServerImpl) SetAgentCard(agentCard types.AgentCard) {
	s.customAgentCard = &agentCard
}

// LoadAgentCardFromFile loads and sets an agent card from a JSON file
// The optional overrides map allows dynamic replacement of JSON attribute values
func (s *A2AServerImpl) LoadAgentCardFromFile(filePath string, overrides map[string]interface{}) error {
	if filePath == "" {
		return nil
	}

	s.logger.Info("loading agent card from file", zap.String("file_path", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read agent card file: %w", err)
	}

	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		return fmt.Errorf("failed to parse agent card JSON: %w", err)
	}

	for key, value := range overrides {
		s.logger.Debug("overriding agent card attribute",
			zap.String("key", key),
			zap.Any("value", value))
		rawData[key] = value
	}

	modifiedData, err := json.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to marshal modified agent card data: %w", err)
	}

	var agentCard types.AgentCard
	if err := json.Unmarshal(modifiedData, &agentCard); err != nil {
		return fmt.Errorf("failed to parse modified agent card JSON: %w", err)
	}

	s.logger.Info("successfully loaded agent card from file",
		zap.String("name", agentCard.Name),
		zap.String("version", agentCard.Version),
		zap.Int("overrides_count", len(overrides)))
	s.customAgentCard = &agentCard
	return nil
}

// SetupRouter configures the HTTP router with A2A endpoints
func (s *A2AServerImpl) setupRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	}

	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(middlewares.LoggingMiddleware(cfg.ServerConfig.DisableHealthcheckLog))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": types.HealthStatusHealthy})
	})

	r.GET("/.well-known/agent.json", s.handleAgentInfo)

	var telemetryMiddleware gin.HandlerFunc
	if s.cfg.TelemetryConfig.Enable && s.otel != nil {
		telemetryMw, err := middlewares.NewTelemetryMiddleware(*s.cfg, s.otel, s.logger)
		if err != nil {
			s.logger.Error("failed to create telemetry middleware", zap.Error(err))
		} else {
			telemetryMiddleware = telemetryMw.Middleware()
		}
	}

	if !cfg.AuthConfig.Enable {
		if telemetryMiddleware != nil {
			r.POST("/a2a", telemetryMiddleware, s.handleA2ARequest)
		} else {
			r.POST("/a2a", s.handleA2ARequest)
		}
		s.logger.Warn("authentication is disabled, oidcAuthenticator will be nil")
		return r
	}
	oidcAuthenticator, err := middlewares.NewOIDCAuthenticatorMiddleware(s.logger, *s.cfg)
	if err != nil {
		s.logger.Error("failed to create OIDC authenticator", zap.Error(err))
		return r
	}

	s.logger.Info("oidcAuthenticator is valid, setting up authentication")
	if telemetryMiddleware != nil {
		r.POST("/a2a", telemetryMiddleware, oidcAuthenticator.Middleware(), s.handleA2ARequest)
	} else {
		r.POST("/a2a", oidcAuthenticator.Middleware(), s.handleA2ARequest)
	}

	return r
}

// Start starts the A2A server
func (s *A2AServerImpl) Start(ctx context.Context) error {
	if s.customAgentCard == nil {
		return fmt.Errorf("agent card must be configured before starting the server - use SetAgentCard() or LoadAgentCardFromFile()")
	}

	router := s.setupRouter(s.cfg)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%s", s.cfg.ServerConfig.Port),
		Handler:      router,
		ReadTimeout:  s.cfg.ServerConfig.ReadTimeout,
		WriteTimeout: s.cfg.ServerConfig.WriteTimeout,
		IdleTimeout:  s.cfg.ServerConfig.IdleTimeout,
	}

	s.logger.Info("starting A2A server", zap.String("port", s.cfg.ServerConfig.Port))

	if s.cfg.TelemetryConfig.Enable && s.otel != nil {
		go func() {
			metricsRouter := gin.Default()
			metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))

			metricsAddr := s.cfg.TelemetryConfig.MetricsConfig.Host + ":" + s.cfg.TelemetryConfig.MetricsConfig.Port
			s.metricsServer = &http.Server{
				Addr:         metricsAddr,
				Handler:      metricsRouter,
				ReadTimeout:  s.cfg.TelemetryConfig.MetricsConfig.ReadTimeout,
				WriteTimeout: s.cfg.TelemetryConfig.MetricsConfig.WriteTimeout,
				IdleTimeout:  s.cfg.TelemetryConfig.MetricsConfig.IdleTimeout,
			}

			s.logger.Info("starting metrics server", zap.String("port", s.cfg.TelemetryConfig.MetricsConfig.Port))
			if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Error("metrics server failed", zap.Error(err))
			}
		}()
	}

	go s.StartTaskProcessor(ctx)

	if s.cfg.ServerConfig.TLSConfig.Enable {
		return s.httpServer.ListenAndServeTLS(s.cfg.ServerConfig.TLSConfig.CertPath, s.cfg.ServerConfig.TLSConfig.KeyPath)
	}

	return s.httpServer.ListenAndServe()
}

// Stop gracefully stops the A2A server
func (s *A2AServerImpl) Stop(ctx context.Context) error {
	s.logger.Info("stopping A2A server")

	var err error

	if s.httpServer != nil {
		if shutdownErr := s.httpServer.Shutdown(ctx); shutdownErr != nil {
			s.logger.Error("error stopping HTTP server", zap.Error(shutdownErr))
			err = shutdownErr
		}
	}

	if s.metricsServer != nil {
		if shutdownErr := s.metricsServer.Shutdown(ctx); shutdownErr != nil {
			s.logger.Error("error stopping metrics server", zap.Error(shutdownErr))
			if err == nil {
				err = shutdownErr
			}
		}
	}

	if s.otel != nil {
		if shutdownErr := s.otel.ShutDown(ctx); shutdownErr != nil {
			s.logger.Error("error shutting down telemetry", zap.Error(shutdownErr))
			if err == nil {
				err = shutdownErr
			}
		}
	}

	defer func() {
		if syncErr := s.logger.Sync(); syncErr != nil {
			s.logger.Error("failed to sync logger on shutdown", zap.Error(syncErr))
		}
	}()

	return err
}

// GetAgentCard returns the agent's capabilities and metadata
// Returns nil if no agent card has been explicitly set
func (s *A2AServerImpl) GetAgentCard() *types.AgentCard {
	return s.customAgentCard
}

// ProcessTask processes a task with the given message
func (s *A2AServerImpl) ProcessTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	if s.agent != nil {
		s.logger.Info("processing task with openai-compatible agent",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))
		return s.agent.ProcessTask(ctx, task, message)
	}

	return s.taskHandler.HandleTask(ctx, task, message)
}

// StartTaskProcessor starts the background task processing goroutine
func (s *A2AServerImpl) StartTaskProcessor(ctx context.Context) {
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
func (s *A2AServerImpl) processQueuedTask(ctx context.Context, queuedTask *QueuedTask) {
	task := queuedTask.Task

	var message *types.Message
	if task.Status.Message != nil {
		message = task.Status.Message
	} else {
		message = &types.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "user",
			Parts:     []types.Part{},
		}
	}

	s.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))

	err := s.taskManager.UpdateTask(task.ID, types.TaskStateWorking, nil)
	if err != nil {
		s.logger.Error("failed to update task state", zap.Error(err))
		return
	}

	updatedTask, err := s.taskHandler.HandleTask(ctx, task, message)
	if err != nil {
		s.logger.Error("failed to process task",
			zap.Error(err),
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))
		updateErr := s.taskManager.UpdateTask(task.ID, types.TaskStateFailed, &types.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": err.Error(),
				},
			},
		})
		if updateErr != nil {
			s.logger.Error("failed to update task to failed state",
				zap.Error(updateErr),
				zap.String("task_id", task.ID),
				zap.String("context_id", task.ContextID))
		}
		return
	}

	if err := s.taskManager.UpdateTask(updatedTask.ID, updatedTask.Status.State, nil); err != nil {
		s.logger.Error("failed to update task status",
			zap.Error(err),
			zap.String("task_id", updatedTask.ID),
			zap.String("context_id", updatedTask.ContextID))
		return
	}
	s.logger.Info("task processed successfully",
		zap.String("task_id", task.ID),
		zap.String("context_id", task.ContextID))
}

// startTaskCleanup starts the background task cleanup process
func (s *A2AServerImpl) startTaskCleanup(ctx context.Context) {
	// Get cleanup interval from config (defaults applied in NewWithDefaults)
	cleanupInterval := s.cfg.QueueConfig.CleanupInterval

	ticker := time.NewTicker(cleanupInterval)
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
func (s *A2AServerImpl) handleAgentInfo(c *gin.Context) {
	s.logger.Info("agent info requested")
	agentCard := s.GetAgentCard()
	if agentCard == nil {
		s.logger.Error("no agent card configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "Agent card not configured",
			"message": "This server requires an agent card to be explicitly set via JSON file or programmatically",
		})
		return
	}
	c.JSON(http.StatusOK, *agentCard)
}

// handleA2ARequest processes A2A protocol requests
func (s *A2AServerImpl) handleA2ARequest(c *gin.Context) {
	var req types.JSONRPCRequest
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
	case "tasks/list":
		s.handleTaskList(c, req)
	case "tasks/cancel":
		s.handleTaskCancel(c, req)
	case "tasks/pushNotificationConfig/set":
		s.handleTaskPushNotificationConfigSet(c, req)
	case "tasks/pushNotificationConfig/get":
		s.handleTaskPushNotificationConfigGet(c, req)
	case "tasks/pushNotificationConfig/list":
		s.handleTaskPushNotificationConfigList(c, req)
	case "tasks/pushNotificationConfig/delete":
		s.handleTaskPushNotificationConfigDelete(c, req)
	default:
		s.logger.Warn("unknown method requested", zap.String("method", req.Method))
		s.responseSender.SendError(c, req.ID, int(ErrMethodNotFound), "method not found")
	}
}

// handleMessageSend processes message/send requests
func (s *A2AServerImpl) handleMessageSend(c *gin.Context, req types.JSONRPCRequest) {
	var params types.MessageSendParams
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
		s.logger.Info("task queued for processing",
			zap.String("task_id", task.ID),
			zap.String("context_id", task.ContextID))
	default:
		s.logger.Error("task queue is full")
		err := s.taskManager.UpdateTask(task.ID, types.TaskStateFailed, &types.Message{
			Kind:      "message",
			MessageID: uuid.New().String(),
			Role:      "assistant",
			Parts: []types.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Task queue is full. Please try again later.",
				},
			},
		})
		if err != nil {
			s.logger.Error("failed to update task to failed state due to full queue",
				zap.Error(err),
				zap.String("task_id", task.ID),
				zap.String("context_id", task.ContextID))
		}
	}

	s.responseSender.SendSuccess(c, req.ID, *task)
}

// handleMessageStream processes message/stream requests
func (s *A2AServerImpl) handleMessageStream(c *gin.Context, req types.JSONRPCRequest) {
	var params types.MessageSendParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse message/stream request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	ctx := c.Request.Context()

	responseChan := make(chan types.SendStreamingMessageResponse, 10)

	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				s.logger.Error("panic in streaming response handler", zap.Any("panic", r))
				done <- fmt.Errorf("panic in streaming response handler: %v", r)
			}
		}()

		for {
			select {
			case <-ctx.Done():
				done <- ctx.Err()
				return
			case response, ok := <-responseChan:
				if !ok {
					done <- nil
					return
				}

				jsonRPCResponse := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      req.ID,
					Result:  response,
				}

				if err := s.writeStreamingResponse(c, &jsonRPCResponse); err != nil {
					s.logger.Error("failed to write streaming response", zap.Error(err))
					done <- err
					return
				}
			}
		}
	}()

	err = s.messageHandler.HandleMessageStream(ctx, params, responseChan)

	close(responseChan)
	if err != nil {
		s.logger.Error("failed to handle message stream", zap.Error(err))

		errorResponse := types.JSONRPCErrorResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &types.JSONRPCError{
				Code:    int(ErrInternalError),
				Message: err.Error(),
			},
		}
		if writeErr := s.writeStreamingErrorResponse(c, &errorResponse); writeErr != nil {
			s.logger.Error("failed to write streaming error response", zap.Error(writeErr))
		}
		return
	}

	select {
	case <-ctx.Done():
		s.logger.Warn("streaming context cancelled")
	case streamErr := <-done:
		if streamErr != nil {
			s.logger.Error("streaming completed with error", zap.Error(streamErr))
		} else {
			s.logger.Info("streaming completed successfully")
		}
	}

	if _, err := c.Writer.Write([]byte("data: [DONE]\n\n")); err != nil {
		s.logger.Error("failed to write stream termination signal", zap.Error(err))
	} else {
		c.Writer.Flush()
		s.logger.Debug("sent stream termination signal [DONE]")
	}
}

// writeStreamingResponse writes a JSON-RPC response to the streaming connection in SSE format
func (s *A2AServerImpl) writeStreamingResponse(c *gin.Context, response *types.JSONRPCSuccessResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return fmt.Errorf("failed to write data prefix: %w", err)
	}

	if _, err := c.Writer.Write(responseBytes); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return fmt.Errorf("failed to write SSE terminator: %w", err)
	}

	c.Writer.Flush()
	return nil
}

// writeStreamingErrorResponse writes a JSON-RPC error response to the streaming connection in SSE format
func (s *A2AServerImpl) writeStreamingErrorResponse(c *gin.Context, response *types.JSONRPCErrorResponse) error {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal error response: %w", err)
	}

	// Write in SSE format: "data: <json>\n\n"
	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return fmt.Errorf("failed to write data prefix: %w", err)
	}

	if _, err := c.Writer.Write(responseBytes); err != nil {
		return fmt.Errorf("failed to write error response: %w", err)
	}

	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return fmt.Errorf("failed to write SSE terminator: %w", err)
	}

	c.Writer.Flush()
	return nil
}

// handleTaskGet processes tasks/get requests
func (s *A2AServerImpl) handleTaskGet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskQueryParams
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

	s.logger.Info("task retrieved successfully",
		zap.String("task_id", params.ID),
		zap.String("context_id", task.ContextID),
		zap.String("status", string(task.Status.State)))
	s.responseSender.SendSuccess(c, req.ID, *task)
}

// handleTaskCancel processes tasks/cancel requests
func (s *A2AServerImpl) handleTaskCancel(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskIdParams
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
		s.logger.Error("failed to cancel task",
			zap.Error(err),
			zap.String("task_id", params.ID))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), err.Error())
		return
	}

	task, _ := s.taskManager.GetTask(params.ID)
	s.responseSender.SendSuccess(c, req.ID, *task)
}

// handleTaskList processes tasks/list requests
func (s *A2AServerImpl) handleTaskList(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskListParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/list request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("listing tasks")

	taskList, err := s.taskManager.ListTasks(params)
	if err != nil {
		s.logger.Error("failed to list tasks", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	s.logger.Info("tasks listed successfully", zap.Int("count", len(taskList.Tasks)), zap.Int("total", taskList.Total))
	s.responseSender.SendSuccess(c, req.ID, taskList)
}

// handleTaskPushNotificationConfigSet processes tasks/pushNotificationConfig/set requests
func (s *A2AServerImpl) handleTaskPushNotificationConfigSet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.TaskPushNotificationConfig
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/pushNotificationConfig/set request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("setting push notification config for task",
		zap.String("task_id", params.TaskID),
		zap.String("url", params.PushNotificationConfig.URL))

	config, err := s.taskManager.SetTaskPushNotificationConfig(params)
	if err != nil {
		s.logger.Error("failed to set push notification config", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	s.logger.Info("push notification config set successfully", zap.String("task_id", params.TaskID))
	s.responseSender.SendSuccess(c, req.ID, config)
}

// handleTaskPushNotificationConfigGet processes tasks/pushNotificationConfig/get requests
func (s *A2AServerImpl) handleTaskPushNotificationConfigGet(c *gin.Context, req types.JSONRPCRequest) {
	var params types.GetTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/pushNotificationConfig/get request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("getting push notification config for task", zap.String("task_id", params.ID))

	config, err := s.taskManager.GetTaskPushNotificationConfig(params)
	if err != nil {
		s.logger.Error("failed to get push notification config", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	s.logger.Info("push notification config retrieved successfully", zap.String("task_id", params.ID))
	s.responseSender.SendSuccess(c, req.ID, config)
}

// handleTaskPushNotificationConfigList processes tasks/pushNotificationConfig/list requests
func (s *A2AServerImpl) handleTaskPushNotificationConfigList(c *gin.Context, req types.JSONRPCRequest) {
	var params types.ListTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/pushNotificationConfig/list request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("listing push notification configs for task", zap.String("task_id", params.ID))

	configs, err := s.taskManager.ListTaskPushNotificationConfigs(params)
	if err != nil {
		s.logger.Error("failed to list push notification configs", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	s.logger.Info("push notification configs listed successfully",
		zap.String("task_id", params.ID),
		zap.Int("count", len(configs)))
	s.responseSender.SendSuccess(c, req.ID, configs)
}

// handleTaskPushNotificationConfigDelete processes tasks/pushNotificationConfig/delete requests
func (s *A2AServerImpl) handleTaskPushNotificationConfigDelete(c *gin.Context, req types.JSONRPCRequest) {
	var params types.DeleteTaskPushNotificationConfigParams
	paramsBytes, err := json.Marshal(req.Params)
	if err != nil {
		s.logger.Error("failed to marshal params", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid params")
		return
	}

	if err := json.Unmarshal(paramsBytes, &params); err != nil {
		s.logger.Error("failed to parse tasks/pushNotificationConfig/delete request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInvalidParams), "invalid request")
		return
	}

	s.logger.Info("deleting push notification config",
		zap.String("task_id", params.ID),
		zap.String("config_id", params.PushNotificationConfigID))

	err = s.taskManager.DeleteTaskPushNotificationConfig(params)
	if err != nil {
		s.logger.Error("failed to delete push notification config", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrInternalError), err.Error())
		return
	}

	s.logger.Info("push notification config deleted successfully",
		zap.String("task_id", params.ID),
		zap.String("config_id", params.PushNotificationConfigID))
	s.responseSender.SendSuccess(c, req.ID, nil)
}

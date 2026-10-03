package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	gin "github.com/gin-gonic/gin"
	uuid "github.com/google/uuid"
	promhttp "github.com/prometheus/client_golang/prometheus/promhttp"
	envconfig "github.com/sethvargo/go-envconfig"
	otelapi "go.opentelemetry.io/otel"
	attribute "go.opentelemetry.io/otel/attribute"
	trace "go.opentelemetry.io/otel/trace"
	zap "go.uber.org/zap"

	serverConfig "github.com/inference-gateway/adk/server/config"
	middlewares "github.com/inference-gateway/adk/server/middlewares"
	otel "github.com/inference-gateway/adk/server/otel"
	types "github.com/inference-gateway/adk/types"
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

	// StartTaskProcessor starts the background task processor
	StartTaskProcessor(ctx context.Context)

	// SetPollingTaskHandler sets the task handler for polling/queue-based scenarios
	SetBackgroundTaskHandler(handler TaskHandler)

	// GetPollingTaskHandler returns the configured polling task handler
	GetBackgroundTaskHandler() TaskHandler

	// SetStreamingTaskHandler sets the task handler for streaming scenarios
	SetStreamingTaskHandler(handler StreamableTaskHandler)

	// GetStreamingTaskHandler returns the configured streaming task handler
	GetStreamingTaskHandler() StreamableTaskHandler

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
	LoadAgentCardFromFile(filePath string, overrides map[string]any) error
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

	// A2A-specific error codes (spec section 5.4).
	ErrTaskNotFound                   JRPCErrorCode = -32001
	ErrTaskNotCancelable              JRPCErrorCode = -32002
	ErrUnsupportedOperation           JRPCErrorCode = -32004
	ErrExtendedAgentCardNotConfigured JRPCErrorCode = -32007
	ErrVersionNotSupported            JRPCErrorCode = -32009
)

type A2AServerImpl struct {
	cfg            *serverConfig.Config
	logger         *zap.Logger
	storage        Storage
	taskManager    TaskManager
	responseSender ResponseSender
	otel           otel.OpenTelemetry

	// Server state
	httpServer    *http.Server
	metricsServer *http.Server

	// Optional processors
	taskResultProcessor TaskResultProcessor
	agent               OpenAICompatibleAgent

	// Custom agent card
	customAgentCard *types.AgentCard

	// Optional extended agent card returned to authenticated callers via
	// GetExtendedAgentCard. When nil but the public card declares
	// supportsExtendedAgentCard, the RPC returns ErrExtendedAgentCardNotConfigured.
	extendedAgentCard *types.AgentCard

	// Separate task handlers for different scenarios
	backgroundTaskHandler TaskHandler
	streamingTaskHandler  StreamableTaskHandler

	// Protocol handler
	protocolHandler A2AProtocolHandler
}

var _ A2AServer = (*A2AServerImpl)(nil)

// NewA2AServer creates a new A2A server with the provided configuration and logger
func NewA2AServer(cfg *serverConfig.Config, logger *zap.Logger, otel otel.OpenTelemetry) *A2AServerImpl {
	if cfg.AgentName == "" {
		cfg.AgentName = BuildAgentName
	}
	if cfg.AgentDescription == "" {
		cfg.AgentDescription = BuildAgentDescription
	}
	if cfg.AgentVersion == "" {
		cfg.AgentVersion = BuildAgentVersion
	}

	ctx := context.Background()
	storage, err := CreateStorage(ctx, cfg.QueueConfig, logger)
	if err != nil {
		if cfg.QueueConfig.Provider == "" {
			logger.Info("no storage provider configured, using in-memory storage")
		} else {
			logger.Warn("failed to create configured storage, falling back to in-memory",
				zap.String("provider", cfg.QueueConfig.Provider),
				zap.Error(err))
		}

		maxConversationHistory := cfg.AgentConfig.MaxConversationHistory
		storage = NewInMemoryStorage(logger, maxConversationHistory)
	}

	server := &A2AServerImpl{
		cfg:     cfg,
		logger:  logger,
		storage: storage,
		otel:    otel,
	}

	server.taskManager = NewDefaultTaskManagerWithStorage(logger, storage)
	server.responseSender = NewDefaultResponseSender(logger)
	bgHandler := NewDefaultBackgroundTaskHandler(logger, server.agent)
	bgHandler.SetEnableUsageMetadata(cfg.AgentConfig.EnableUsageMetadata)
	server.backgroundTaskHandler = bgHandler
	streamHandler := NewDefaultStreamingTaskHandler(logger, server.agent)
	streamHandler.SetEnableUsageMetadata(cfg.AgentConfig.EnableUsageMetadata)
	server.streamingTaskHandler = streamHandler
	server.protocolHandler = NewDefaultA2AProtocolHandler(
		logger,
		server.storage,
		server.taskManager,
		server.responseSender,
	)

	return server
}

// NewA2AServerWithAgent creates a new A2A server with an optional OpenAI-compatible agent
func NewA2AServerWithAgent(cfg *serverConfig.Config, logger *zap.Logger, otel otel.OpenTelemetry, agent OpenAICompatibleAgent) *A2AServerImpl {
	server := NewA2AServer(cfg, logger, otel)

	if agent != nil {
		server.SetAgent(agent)
	}

	return server
}

// NewDefaultA2AServer creates a new default A2A server implementation
func NewDefaultA2AServer(cfg *serverConfig.Config) *A2AServerImpl {
	var finalCfg *serverConfig.Config
	var err error

	finalCfg, err = serverConfig.LoadWithLookuper(context.Background(), cfg, envconfig.OsLookuper())
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
		resolvedTelemetry := finalCfg.ResolveTelemetry()
		if resolvedTelemetry.MetricsExporter == serverConfig.MetricsExporterPrometheus {
			metricsAddr := resolvedTelemetry.PrometheusHost + ":" + resolvedTelemetry.PrometheusPort
			logger.Info("telemetry enabled - metrics will be available", zap.String("metrics_url", metricsAddr+"/metrics"))
		} else {
			logger.Info("telemetry enabled",
				zap.String("metrics_exporter", resolvedTelemetry.MetricsExporter),
				zap.String("traces_exporter", resolvedTelemetry.TracesExporter))
		}
	}

	server := NewA2AServer(finalCfg, logger, telemetryInstance)

	return server
}

// NewA2AServerEnvironmentAware creates a new A2A server with environment-aware configuration.
func NewA2AServerEnvironmentAware(cfg *serverConfig.Config, logger *zap.Logger, otel otel.OpenTelemetry) *A2AServerImpl {
	var err error
	cfg, err = serverConfig.LoadWithLookuper(context.Background(), cfg, envconfig.OsLookuper())
	if err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	server := &A2AServerImpl{
		cfg:    cfg,
		logger: logger,
		otel:   otel,
	}

	maxConversationHistory := cfg.AgentConfig.MaxConversationHistory
	storage := NewInMemoryStorage(logger, maxConversationHistory)
	server.storage = storage

	server.taskManager = NewDefaultTaskManagerWithStorage(logger, storage)
	server.responseSender = NewDefaultResponseSender(logger)
	bgHandler := NewDefaultBackgroundTaskHandler(logger, server.agent)
	bgHandler.SetEnableUsageMetadata(cfg.AgentConfig.EnableUsageMetadata)
	server.backgroundTaskHandler = bgHandler
	streamHandler := NewDefaultStreamingTaskHandler(logger, server.agent)
	streamHandler.SetEnableUsageMetadata(cfg.AgentConfig.EnableUsageMetadata)
	server.streamingTaskHandler = streamHandler
	server.protocolHandler = NewDefaultA2AProtocolHandler(
		logger,
		server.storage,
		server.taskManager,
		server.responseSender,
	)

	return server
}

// SetBackgroundTaskHandler sets the task handler for polling/queue-based scenarios
func (s *A2AServerImpl) SetBackgroundTaskHandler(handler TaskHandler) {
	s.backgroundTaskHandler = handler
}

// GetBackgroundTaskHandler returns the configured polling task handler
func (s *A2AServerImpl) GetBackgroundTaskHandler() TaskHandler {
	return s.backgroundTaskHandler
}

// SetStreamingTaskHandler sets the task handler for streaming scenarios
func (s *A2AServerImpl) SetStreamingTaskHandler(handler StreamableTaskHandler) {
	s.streamingTaskHandler = handler
}

// GetStreamingTaskHandler returns the configured streaming task handler
func (s *A2AServerImpl) GetStreamingTaskHandler() StreamableTaskHandler {
	return s.streamingTaskHandler
}

// SetAgent sets the OpenAI-compatible agent for processing tasks
func (s *A2AServerImpl) SetAgent(agent OpenAICompatibleAgent) {
	s.agent = agent
	if s.backgroundTaskHandler != nil {
		s.backgroundTaskHandler.SetAgent(agent)
	}
	if s.streamingTaskHandler != nil {
		s.streamingTaskHandler.SetAgent(agent)
	}
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

// SetExtendedAgentCard sets the extended agent card returned to authenticated
// callers via GetExtendedAgentCard.
func (s *A2AServerImpl) SetExtendedAgentCard(agentCard types.AgentCard) {
	s.extendedAgentCard = &agentCard
}

// validateStreamingConfiguration checks if streaming is enabled but no streaming handler is configured
func (s *A2AServerImpl) validateStreamingConfiguration() {
	if s.customAgentCard == nil {
		return
	}

	streamingEnabled := false
	if s.customAgentCard.Capabilities.Streaming != nil {
		streamingEnabled = *s.customAgentCard.Capabilities.Streaming
	}

	if streamingEnabled {
		if s.streamingTaskHandler == nil {
			s.logger.Warn("streaming is enabled in agent capabilities but no streaming task handler is configured",
				zap.String("warning", "streaming requests will fail"),
				zap.String("suggestion", "use WithStreamingTaskHandler() for custom streaming logic or WithDefaultStreamingTaskHandler() for optimized streaming support"))
		}
	} else {
		s.logger.Info("streaming is disabled in agent capabilities",
			zap.String("note", "only background/polling requests will be supported"),
			zap.String("tip", "to enable streaming, set streaming capability to true in your agent card and configure a streaming task handler"))
	}
}

// validateAuthConfiguration warns when the auth config and the served card's
// security declaration are inconsistent - authentication enabled but the card
// declares no securitySchemes (clients cannot discover how to authenticate), or
// the inverse. Spec section 7 requires the card to declare how it is secured.
func (s *A2AServerImpl) validateAuthConfiguration() {
	if s.customAgentCard == nil {
		return
	}

	declaresSchemes := len(s.customAgentCard.SecuritySchemes) > 0
	authEnabled := s.cfg.AuthConfig.Enabled || s.cfg.AuthConfig.Token != ""

	switch {
	case authEnabled && !declaresSchemes:
		s.logger.Warn("authentication is enabled but the agent card declares no securitySchemes",
			zap.String("impact", "clients cannot discover how to authenticate"),
			zap.String("suggestion", "declare schemes on the card, e.g. via server.OIDCSecuritySchemes(cfg.AuthConfig) or server.BearerTokenSecuritySchemes()"))
	case !authEnabled && declaresSchemes:
		s.logger.Warn("the agent card declares securitySchemes but authentication is disabled",
			zap.String("impact", "advertised schemes are not enforced"),
			zap.String("suggestion", "set AUTH_ENABLED=true or remove the securitySchemes from the card"))
	}
}

// LoadAgentCardFromFile loads and sets an agent card from a JSON file
// The optional overrides map allows dynamic replacement of JSON attribute values
func (s *A2AServerImpl) LoadAgentCardFromFile(filePath string, overrides map[string]any) error {
	if filePath == "" {
		return nil
	}

	s.logger.Info("loading agent card from file", zap.String("file_path", filePath))

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read agent card file: %w", err)
	}

	var rawData map[string]any
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
func (s *A2AServerImpl) setupRouter(cfg *serverConfig.Config) (*gin.Engine, error) {
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

	r.GET("/.well-known/agent-card.json", s.handleAgentInfo)

	var telemetryMiddleware gin.HandlerFunc
	if s.otel != nil {
		telemetryMw, err := middlewares.NewTelemetryMiddleware(*s.cfg, s.otel, s.logger)
		if err != nil {
			s.logger.Error("failed to create telemetry middleware", zap.Error(err))
		} else {
			telemetryMiddleware = telemetryMw.Middleware()
		}
	}

	authMiddleware, err := s.authMiddleware(cfg)
	if err != nil {
		return nil, err
	}

	handlers := make([]gin.HandlerFunc, 0, 3)
	if telemetryMiddleware != nil {
		handlers = append(handlers, telemetryMiddleware)
	}
	if authMiddleware != nil {
		handlers = append(handlers, authMiddleware)
	}
	r.POST("/a2a", append(handlers, s.handleA2ARequest)...)

	return r, nil
}

// authMiddleware picks the /a2a guard from the auth config: a static bearer token
// (AUTH_TOKEN), OIDC (AUTH_ENABLED), or none. Setting both is a configuration error.
func (s *A2AServerImpl) authMiddleware(cfg *serverConfig.Config) (gin.HandlerFunc, error) {
	switch {
	case cfg.AuthConfig.Token != "" && cfg.AuthConfig.Enabled:
		return nil, fmt.Errorf("AUTH_TOKEN and AUTH_ENABLED are mutually exclusive: use static bearer-token auth or OIDC, not both")
	case cfg.AuthConfig.Token != "":
		s.logger.Info("static bearer-token authentication enabled for /a2a")
		return middlewares.NewBearerTokenMiddleware(cfg.AuthConfig.Token), nil
	case cfg.AuthConfig.Enabled:
		oidcAuthenticator, err := middlewares.NewOIDCAuthenticatorMiddleware(s.logger, *s.cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create OIDC authenticator: %w", err)
		}
		s.logger.Info("oidcAuthenticator is valid, setting up authentication")
		return oidcAuthenticator.Middleware(), nil
	default:
		s.logger.Warn("authentication is disabled, oidcAuthenticator will be nil")
		return nil, nil
	}
}

// Start starts the A2A server
func (s *A2AServerImpl) Start(ctx context.Context) error {
	if s.customAgentCard == nil {
		return fmt.Errorf("agent card must be configured before starting the server - use SetAgentCard() or LoadAgentCardFromFile()")
	}

	router, err := s.setupRouter(s.cfg)
	if err != nil {
		return err
	}

	s.httpServer = &http.Server{
		Addr:         net.JoinHostPort(s.cfg.ServerConfig.Host, s.cfg.ServerConfig.Port),
		Handler:      router,
		ReadTimeout:  s.cfg.ServerConfig.ReadTimeout,
		WriteTimeout: s.cfg.ServerConfig.WriteTimeout,
		IdleTimeout:  s.cfg.ServerConfig.IdleTimeout,
	}

	s.logger.Info("starting A2A server",
		zap.String("host", s.cfg.ServerConfig.Host),
		zap.String("port", s.cfg.ServerConfig.Port),
		zap.String("agent_name", s.cfg.AgentName),
		zap.String("agent_description", s.cfg.AgentDescription),
		zap.String("agent_version", s.cfg.AgentVersion))

	s.validateStreamingConfiguration()
	s.validateAuthConfiguration()

	resolvedTelemetry := s.cfg.ResolveTelemetry()
	if s.otel != nil && resolvedTelemetry.MetricsExporter == serverConfig.MetricsExporterPrometheus {
		go func() {
			metricsRouter := gin.Default()
			metricsRouter.GET("/metrics", gin.WrapH(promhttp.Handler()))

			metricsAddr := resolvedTelemetry.PrometheusHost + ":" + resolvedTelemetry.PrometheusPort
			s.metricsServer = &http.Server{
				Addr:         metricsAddr,
				Handler:      metricsRouter,
				ReadTimeout:  s.cfg.TelemetryConfig.MetricsConfig.ReadTimeout,
				WriteTimeout: s.cfg.TelemetryConfig.MetricsConfig.WriteTimeout,
				IdleTimeout:  s.cfg.TelemetryConfig.MetricsConfig.IdleTimeout,
			}

			s.logger.Info("starting metrics server", zap.String("port", resolvedTelemetry.PrometheusPort))
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

// StartTaskProcessor starts the background task processing goroutine
func (s *A2AServerImpl) StartTaskProcessor(ctx context.Context) {
	s.logger.Info("starting task processor")

	s.taskManager.SetRetentionConfig(s.cfg.TaskRetentionConfig)
	defer s.taskManager.StopCleanup()

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("task processor shutting down")
			return
		default:
			queuedTask, err := s.storage.DequeueTask(ctx)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					s.logger.Info("task processor shutting down due to context cancellation")
					return
				}
				s.logger.Error("failed to dequeue task", zap.Error(err))
				continue
			}

			if queuedTask != nil {
				s.processQueuedTask(ctx, queuedTask)
			}
		}
	}
}

// processQueuedTask processes a single queued task
func (s *A2AServerImpl) processQueuedTask(ctx context.Context, queuedTask *QueuedTask) {
	task := queuedTask.Task

	ctx = extractTraceContext(ctx, queuedTask.TraceContext)
	ctx = injectAuthContext(ctx, queuedTask)
	ctx, span := otelapi.Tracer("github.com/inference-gateway/adk/server").Start(ctx, "task.process",
		trace.WithAttributes(attribute.String("a2a.task.id", task.ID)))
	defer span.End()

	var message *types.Message
	if task.Status.Message != nil {
		message = task.Status.Message
	} else {
		message = &types.Message{
			MessageID: uuid.New().String(),
			Role:      "user",
			Parts:     []types.Part{},
		}
	}

	s.logger.Info("processing task",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID))

	err := s.taskManager.UpdateState(task.ID, types.TaskStateWorking)
	if err != nil {
		s.logger.Error("failed to update task state", zap.Error(err))
		return
	}

	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	if defaultTM, ok := s.taskManager.(*DefaultTaskManager); ok {
		defaultTM.RegisterTaskCancelFunc(task.ID, cancel)
		defer defaultTM.UnregisterTaskCancelFunc(task.ID)
	}

	updatedTask, err := s.backgroundTaskHandler.HandleTask(taskCtx, task, message)
	if err != nil {
		s.logger.Error("failed to process task",
			zap.Error(err),
			zap.String("task_id", task.ID),
			zap.Stringp("context_id", task.ContextID))
		updateErr := s.taskManager.UpdateError(task.ID, &types.Message{
			MessageID: uuid.New().String(),
			Role:      types.RoleAgent,
			Parts: []types.Part{
				types.CreateTextPart(err.Error()),
			},
		})
		if updateErr != nil {
			s.logger.Error("failed to update task to failed state",
				zap.Error(updateErr),
				zap.String("task_id", task.ID),
				zap.Stringp("context_id", task.ContextID))
		}
		return
	}

	if err := s.taskManager.UpdateTask(updatedTask); err != nil {
		s.logger.Error("failed to update task",
			zap.Error(err),
			zap.String("task_id", updatedTask.ID),
			zap.Stringp("context_id", updatedTask.ContextID))
		return
	}
	s.logger.Info("task processed successfully",
		zap.String("task_id", task.ID),
		zap.Stringp("context_id", task.ContextID))
}

// injectAuthContext restores the authenticated caller claims that were
// captured when the task was enqueued (see QueuedTask.Claims) back onto
// the processing context so that callbacks can inspect the caller via
// ctx.Value(middlewares.ClaimsContextKey).
func injectAuthContext(ctx context.Context, qt *QueuedTask) context.Context {
	return context.WithValue(ctx, middlewares.ClaimsContextKey, qt.Claims)
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

// isSupportedA2AVersion reports whether the server speaks the requested A2A-Version.
// ponytail: an empty header is accepted so clients that omit it keep working, although spec
// section 3.6 reads it as 0.3; reject it once 0.3 clients are gone.
func isSupportedA2AVersion(version string) bool {
	return version == "" || version == types.A2AProtocolVersion
}

// handleA2ARequest processes A2A protocol requests
func (s *A2AServerImpl) handleA2ARequest(c *gin.Context) {
	var req types.JSONRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("failed to parse json request", zap.Error(err))
		s.responseSender.SendError(c, req.ID, int(ErrParseError), "parse error")
		return
	}

	if version := c.GetHeader("A2A-Version"); !isSupportedA2AVersion(version) {
		s.responseSender.SendError(c, req.ID, int(ErrVersionNotSupported), "a2a version "+version+" is not supported")
		return
	}

	if req.JSONRPC == "" {
		req.JSONRPC = "2.0"
	}
	if req.ID == nil {
		id := any(uuid.New().String())
		req.ID = &id
	}

	s.logger.Info("received a2a request",
		zap.String("method", string(req.Method)),
		zap.Any("id", req.ID))

	switch req.Method {
	case types.A2AMethodSendMessage:
		s.protocolHandler.HandleMessageSend(c, req)
	case types.A2AMethodSendStreamingMessage:
		s.protocolHandler.HandleMessageStream(c, req, s.streamingTaskHandler)
	case types.A2AMethodGetTask:
		s.protocolHandler.HandleTaskGet(c, req)
	case types.A2AMethodListTasks:
		s.protocolHandler.HandleTaskList(c, req)
	case types.A2AMethodCancelTask:
		s.protocolHandler.HandleTaskCancel(c, req)
	case types.A2AMethodCreateTaskPushNotificationConfig:
		s.protocolHandler.HandleTaskPushNotificationConfigSet(c, req)
	case types.A2AMethodGetTaskPushNotificationConfig:
		s.protocolHandler.HandleTaskPushNotificationConfigGet(c, req)
	case types.A2AMethodListTaskPushNotificationConfigs:
		s.protocolHandler.HandleTaskPushNotificationConfigList(c, req)
	case types.A2AMethodDeleteTaskPushNotificationConfig:
		s.protocolHandler.HandleTaskPushNotificationConfigDelete(c, req)
	case types.A2AMethodSubscribeToTask:
		s.protocolHandler.HandleTaskResubscribe(c, req, s.streamingTaskHandler)
	case types.A2AMethodGetExtendedAgentCard:
		s.protocolHandler.HandleGetAuthenticatedExtendedCard(c, req, s.customAgentCard, s.extendedAgentCard)
	default:
		s.logger.Warn("unknown method requested", zap.String("method", string(req.Method)))
		s.responseSender.SendError(c, req.ID, int(ErrMethodNotFound), "method not found")
	}
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	uuid "github.com/google/uuid"
	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/authentication/server/config"
)

// EchoTaskHandler is a minimal handler; the focus of this example is the
// card-driven authentication flow, not the task logic.
type EchoTaskHandler struct {
	logger *zap.Logger
	agent  server.OpenAICompatibleAgent
}

func (h *EchoTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
	userInput := ""
	if message != nil {
		for _, part := range message.Parts {
			if part.Text != nil {
				userInput = *part.Text
			}
		}
	}

	responseMessage := types.Message{
		MessageID: uuid.New().String(),
		ContextID: &task.ContextID,
		TaskID:    &task.ID,
		Role:      types.RoleAgent,
		Parts:     []types.Part{types.CreateTextPart(fmt.Sprintf("Echo: %s", userInput))},
	}

	task.History = append(task.History, responseMessage)
	task.Status.State = types.TaskStateCompleted
	task.Status.Message = &responseMessage

	return task, nil
}

func (h *EchoTaskHandler) SetAgent(agent server.OpenAICompatibleAgent) { h.agent = agent }
func (h *EchoTaskHandler) GetAgent() server.OpenAICompatibleAgent      { return h.agent }

// Authentication Flow A2A Server Example
//
// This example demonstrates the A2A card-driven authentication flow (spec
// section 7 and 3.3.4):
//
//   - The public agent card declares its securitySchemes via
//     server.OIDCSecuritySchemes(), so clients can discover how to authenticate.
//   - An extended agent card is configured via WithExtendedAgentCard(), which
//     also advertises supportsExtendedAgentCard on the public card.
//   - Authenticated callers fetch the richer card via
//     agent/getAuthenticatedExtendedCard.
//
// Authentication is left disabled by default (AUTH_ENABLED=false) so the example
// runs without a live OIDC provider - the discovery half of the flow and the
// extended-card contract are still exercised end to end. Set AUTH_ENABLED=true
// and point AUTH_ISSUER_URL at a real provider to enforce the declared scheme.
//
// To run: go run .
func main() {
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        "authentication-agent",
			AgentDescription: "demonstrates the A2A card-driven authentication flow",
			AgentVersion:     "0.1.0",
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming: false,
			},
			QueueConfig:  serverConfig.QueueConfig{CleanupInterval: 5 * time.Minute},
			ServerConfig: serverConfig.ServerConfig{Port: "8080"},
			AuthConfig: serverConfig.AuthConfig{
				IssuerURL: "https://issuer.example.com/realms/demo",
			},
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	baseURL := fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)

	// Public card: declare how to authenticate (spec section 7) so clients can
	// discover the OIDC scheme before sending any credentials.
	schemes, security := server.OIDCSecuritySchemes(cfg.A2A.AuthConfig)
	publicCard := types.AgentCard{
		Name:               cfg.A2A.AgentName,
		Description:        cfg.A2A.AgentDescription,
		Version:            cfg.A2A.AgentVersion,
		URL:                new(baseURL),
		ProtocolVersion:    "0.3.0",
		Capabilities:       types.AgentCapabilities{Streaming: new(false)},
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Skills:             []types.AgentSkill{},
		SecuritySchemes:    schemes,
		Security:           security,
	}

	// Extended card: richer detail returned only to authenticated callers via
	// agent/getAuthenticatedExtendedCard. Configuring it also flips
	// supportsExtendedAgentCard=true on the public card.
	extendedCard := publicCard
	extendedCard.Description = "extended card with richer detail for authenticated callers"
	extendedCard.Skills = []types.AgentSkill{
		{
			ID:          "echo",
			Name:        "echo",
			Description: "echoes the message back",
			Tags:        []string{"demo"},
		},
	}

	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithBackgroundTaskHandler(&EchoTaskHandler{logger: logger}).
		WithAgentCard(publicCard).
		WithExtendedAgentCard(extendedCard).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(runCtx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("server running", zap.String("url", baseURL))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutting down server")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}

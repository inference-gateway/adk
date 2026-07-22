package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"

	config "github.com/inference-gateway/adk/examples/mcp/server/config"
)

// MCP-Powered A2A Server Example
//
// This example starts an A2A server whose agent can discover and invoke tools
// from one or more MCP servers. Tools are not loaded into the LLM context
// directly - instead the agent gets two selector tools (mcp_list_tools,
// mcp_call_tool) and pulls tool metadata on demand, so a large MCP catalog does
// not overwhelm the context window.
//
// The MCP client is feature-flagged and only wired when an LLM is configured.
//
// Configuration (environment variables):
//
//   - A2A_AGENT_CLIENT_PROVIDER: LLM provider (required to enable the agent/MCP)
//
//   - A2A_AGENT_CLIENT_MODEL:    LLM model (required)
//
//   - A2A_AGENT_CLIENT_API_KEY:  provider API key
//
//   - A2A_MCP_ENABLE:            enable the MCP client (default: false)
//
//   - A2A_MCP_SERVERS:           comma-separated MCP server base URLs
//
//   - A2A_MCP_ENDPOINT:          MCP HTTP endpoint path (default: /mcp)
//
//     To run: A2A_MCP_ENABLE=true A2A_MCP_SERVERS=http://localhost:8083 \
//     A2A_AGENT_CLIENT_PROVIDER=openai A2A_AGENT_CLIENT_MODEL=gpt-4o-mini \
//     A2A_AGENT_CLIENT_API_KEY=... go run .
func main() {
	cfg := &config.Config{
		Environment: "development",
		A2A: serverConfig.Config{
			AgentName:        server.BuildAgentName,
			AgentDescription: server.BuildAgentDescription,
			AgentVersion:     server.BuildAgentVersion,
			CapabilitiesConfig: serverConfig.CapabilitiesConfig{
				Streaming: true,
			},
			ServerConfig: serverConfig.ServerConfig{Port: "8080"},
		},
	}

	ctx := context.Background()
	if err := envconfig.Process(ctx, cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.A2A.Debug {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// An MCP connection only makes sense when an LLM is configured to use it.
	if cfg.A2A.AgentConfig.Provider == "" {
		logger.Fatal("this example requires an LLM - set A2A_AGENT_CLIENT_PROVIDER and A2A_AGENT_CLIENT_MODEL")
	}

	toolBox := server.NewDefaultToolBox(&cfg.A2A.AgentConfig.ToolBoxConfig)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if cfg.A2A.MCPConfig.Enable {
		mcpManager, err := server.NewMCPClientManager(cfg.A2A.MCPConfig, logger)
		if err != nil {
			logger.Fatal("failed to create MCP client manager", zap.Error(err))
		}
		mcpManager.Start(ctx) // background connect + refresh with retry/polling backoff
		defer func() { _ = mcpManager.Close() }()
		mcpManager.RegisterTools(toolBox)
		logger.Info("mcp client enabled", zap.Strings("servers", cfg.A2A.MCPConfig.Servers))
	} else {
		logger.Warn("mcp client disabled - set A2A_MCP_ENABLE=true and A2A_MCP_SERVERS to use MCP tools")
	}

	llmClient, err := server.NewOpenAICompatibleLLMClient(&cfg.A2A.AgentConfig, logger)
	if err != nil {
		logger.Fatal("failed to create LLM client", zap.Error(err))
	}

	agent, err := server.NewAgentBuilder(logger).
		WithConfig(&cfg.A2A.AgentConfig).
		WithLLMClient(llmClient).
		WithSystemPrompt("You are a helpful assistant. Use mcp_list_tools to discover available tools and mcp_call_tool to invoke them when they help answer the user.").
		WithToolBox(toolBox).
		Build()
	if err != nil {
		logger.Fatal("failed to create agent", zap.Error(err))
	}

	agentURL := fmt.Sprintf("http://localhost:%s", cfg.A2A.ServerConfig.Port)
	a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
		WithAgent(agent).
		WithDefaultTaskHandlers().
		WithAgentCard(types.AgentCard{
			Name:            cfg.A2A.AgentName,
			Description:     cfg.A2A.AgentDescription,
			Version:         cfg.A2A.AgentVersion,
			URL:             &agentURL,
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming: &cfg.A2A.CapabilitiesConfig.Streaming,
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()
	logger.Info("server running", zap.String("port", cfg.A2A.ServerConfig.Port))

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

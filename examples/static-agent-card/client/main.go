package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
)

// Config holds client configuration
type Config struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
}

// Static Agent Card A2A Client Example
//
// This client demonstrates fetching the agent card from a server that
// loads its configuration from a static JSON file using WithAgentCardFromFile().
//
// To run: go run main.go
func main() {
	// Load configuration
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	// Initialize logger based on environment
	var logger *zap.Logger
	var err error
	if cfg.Environment == "development" || cfg.Environment == "dev" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	logger.Info("client starting", zap.String("server_url", cfg.ServerURL))

	// Create A2A client
	a2aClient := client.NewClientWithLogger(cfg.ServerURL, logger)

	logger.Info("fetching agent card from server")
	logger.Info("this demonstrates how WithAgentCardFromFile() loads configuration from agent-card.json")

	// Get the agent card to show the static configuration
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		logger.Fatal("failed to get agent card", zap.Error(err))
	}

	// Pretty print the agent card
	cardJSON, err := json.MarshalIndent(agentCard, "", "  ")
	if err != nil {
		logger.Fatal("failed to marshal agent card", zap.Error(err))
	}

	fmt.Println("\nAgent Card Configuration (loaded from agent-card.json):")
	fmt.Println(string(cardJSON))

	logger.Info("key agent details",
		zap.String("name", agentCard.Name),
		zap.String("description", agentCard.Description),
		zap.String("version", agentCard.Version),
		zap.String("protocol_version", agentCard.ProtocolVersion))

	if agentCard.Capabilities.Streaming != nil {
		logger.Info("streaming capability", zap.Bool("enabled", *agentCard.Capabilities.Streaming))
	}

	if len(agentCard.Skills) > 0 {
		logger.Info("available skills", zap.Int("count", len(agentCard.Skills)))
		for i, skill := range agentCard.Skills {
			fmt.Printf("  %d. %s: %s\n", i+1, skill.Name, skill.Description)
		}
	}

	logger.Info("configuration loaded from agent-card.json using WithAgentCardFromFile()")
	logger.Info("agent card demonstration completed")
}

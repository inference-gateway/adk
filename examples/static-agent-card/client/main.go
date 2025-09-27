package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
)

// Static Agent Card A2A Client Example
//
// This client demonstrates fetching the agent card from a server that
// loads its configuration from a static JSON file using WithAgentCardFromFile().
//
// To run: go run main.go
func main() {
	fmt.Println("📞 Starting Static Agent Card A2A Client...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Create A2A client
	a2aClient := client.NewClientWithLogger("http://localhost:8080", logger)

	ctx := context.Background()

	fmt.Println("\n🃏 Fetching agent card from server...")
	fmt.Println("This demonstrates how WithAgentCardFromFile() loads configuration from agent-card.json")

	// Get the agent card to show the static configuration
	agentCard, err := a2aClient.GetAgentCard(ctx)
	if err != nil {
		log.Fatalf("failed to get agent card: %v", err)
	}

	// Pretty print the agent card
	cardJSON, err := json.MarshalIndent(agentCard, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal agent card: %v", err)
	}

	fmt.Println("\n📋 Agent Card Configuration (loaded from agent-card.json):")
	fmt.Println(string(cardJSON))

	fmt.Println("\n🔍 Key Points:")
	fmt.Printf("✓ Agent Name: %s\n", agentCard.Name)
	fmt.Printf("✓ Description: %s\n", agentCard.Description)
	fmt.Printf("✓ Version: %s\n", agentCard.Version)
	fmt.Printf("✓ Protocol Version: %s\n", agentCard.ProtocolVersion)

	if agentCard.Capabilities.Streaming != nil {
		fmt.Printf("✓ Streaming Enabled: %t\n", *agentCard.Capabilities.Streaming)
	}

	if len(agentCard.Skills) > 0 {
		fmt.Printf("✓ Skills Available: %d\n", len(agentCard.Skills))
		for i, skill := range agentCard.Skills {
			fmt.Printf("  %d. %s: %s\n", i+1, skill.Name, skill.Description)
		}
	}

	fmt.Println("\n💡 This configuration was loaded from agent-card.json using:")
	fmt.Println("   WithAgentCardFromFile(cfg.A2A.AgentCardFile, map[string]any{")
	fmt.Printf("       \"url\": fmt.Sprintf(\"http://localhost:%%s\", port),\n")
	fmt.Println("   })")

	fmt.Println("\n✅ Agent card demonstration completed!")
}

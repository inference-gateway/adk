package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	envconfig "github.com/sethvargo/go-envconfig"
	zap "go.uber.org/zap"

	client "github.com/inference-gateway/adk/client"
	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
)

// Config holds client configuration.
type Config struct {
	Environment string `env:"ENVIRONMENT,default=development"`
	ServerURL   string `env:"SERVER_URL,default=http://localhost:8080"`
	// Token is the out-of-band credential obtained separately (e.g. from the
	// OIDC provider discovered on the card). For this example any value works
	// since the server leaves auth disabled by default.
	Token string `env:"TOKEN,default=out-of-band-token"`
}

// Authentication Flow A2A Client Example
//
// Walks the card-driven authentication flow end to end:
//  1. Fetch the public agent card (unauthenticated) and read its securitySchemes
//     to discover how to authenticate.
//  2. Attach the out-of-band credential as an Authorization header.
//  3. Call agent/getAuthenticatedExtendedCard to receive the richer card.
//
// To run: go run .
func main() {
	ctx := context.Background()
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		log.Fatalf("failed to load configuration: %v", err)
	}

	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer func() { _ = logger.Sync() }()

	// 1. Discovery: fetch the public card unauthenticated and inspect how to auth.
	discovery := client.NewClientWithLogger(cfg.ServerURL, logger)
	publicCard, err := discovery.GetAgentCard(ctx)
	if err != nil {
		logger.Fatal("failed to fetch public agent card", zap.Error(err))
	}

	scheme, ok := publicCard.SecuritySchemes[server.OIDCSchemeName]
	if !ok {
		logger.Fatal("public card declares no OIDC security scheme")
	}
	if scheme.OpenIDConnectSecurityScheme != nil {
		logger.Info("discovered how to authenticate",
			zap.String("scheme", server.OIDCSchemeName),
			zap.String("openIdConnectUrl", scheme.OpenIDConnectSecurityScheme.OpenIDConnectURL))
	}

	if publicCard.SupportsExtendedAgentCard == nil || !*publicCard.SupportsExtendedAgentCard {
		logger.Fatal("server does not advertise an extended agent card")
	}

	// 2. Attach the out-of-band credential on every request via a header.
	authClient := client.NewClientWithConfig(&client.Config{
		BaseURL: cfg.ServerURL,
		Headers: map[string]string{"Authorization": "Bearer " + cfg.Token},
	})

	// 3. Fetch the extended card as an authenticated caller.
	resp, err := authClient.GetAuthenticatedExtendedCard(ctx, types.GetAuthenticatedExtendedCardParams{})
	if err != nil {
		logger.Fatal("failed to fetch extended agent card", zap.Error(err))
	}

	cardBytes, err := json.Marshal(resp.Result)
	if err != nil {
		logger.Fatal("failed to marshal extended card", zap.Error(err))
	}
	var extended types.AgentCard
	if err := json.Unmarshal(cardBytes, &extended); err != nil {
		logger.Fatal("failed to decode extended card", zap.Error(err))
	}

	pretty, _ := json.MarshalIndent(extended, "", "  ")
	fmt.Println("\nExtended agent card (authenticated):")
	fmt.Println(string(pretty))

	logger.Info("authentication flow completed",
		zap.String("name", extended.Name),
		zap.Int("skills", len(extended.Skills)))
}

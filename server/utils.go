package server

import (
	"github.com/google/uuid"
	"github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
)

// StringPtr returns a pointer to the given string
func StringPtr(s string) *string {
	return &s
}

// BoolPtr returns a pointer to the given boolean
func BoolPtr(b bool) *bool {
	return &b
}

// GenerateTaskID generates a unique task ID using UUID v4
func GenerateTaskID() string {
	return uuid.New().String()
}

// CreateOIDCSecurityScheme creates an OpenID Connect security scheme
func CreateOIDCSecurityScheme(openIDConnectURL string, description string) types.SecurityScheme {
	return types.OpenIdConnectSecurityScheme{
		Type:             "openIdConnect",
		OpenIDConnectURL: openIDConnectURL,
		Description:      StringPtr(description),
	}
}

// CreateAPIKeySecurityScheme creates an API key security scheme
func CreateAPIKeySecurityScheme(name string, in string, description string) types.SecurityScheme {
	return types.APIKeySecurityScheme{
		Type:        "apiKey",
		Name:        name,
		In:          in,
		Description: StringPtr(description),
	}
}

// CreateHTTPAuthSecurityScheme creates an HTTP authentication security scheme
func CreateHTTPAuthSecurityScheme(scheme string, bearerFormat *string, description string) types.SecurityScheme {
	return types.HTTPAuthSecurityScheme{
		Type:         "http",
		Scheme:       scheme,
		BearerFormat: bearerFormat,
		Description:  StringPtr(description),
	}
}

// CreateOAuth2SecurityScheme creates an OAuth 2.0 security scheme
func CreateOAuth2SecurityScheme(flows types.OAuthFlows, oauth2MetadataURL *string, description string) types.SecurityScheme {
	return types.OAuth2SecurityScheme{
		Type:              "oauth2",
		Flows:             flows,
		Oauth2metadataURL: oauth2MetadataURL,
		Description:       StringPtr(description),
	}
}

// CreateMutualTLSSecurityScheme creates a mutual TLS security scheme
func CreateMutualTLSSecurityScheme(description string) types.SecurityScheme {
	return types.MutualTLSSecurityScheme{
		Type:        "mutualTLS",
		Description: StringPtr(description),
	}
}

// AgentCardSecurityConfig holds security configuration options for an agent card
type AgentCardSecurityConfig struct {
	EnableOIDC                        bool
	OIDCIssuerURL                     string
	SupportsAuthenticatedExtendedCard bool
	EnableAPIKey                      bool
	APIKeyName                        string
	APIKeyLocation                    string // "header", "query", "cookie"
	EnableMutualTLS                   bool
}

// ConfigureAgentCardSecurity adds security configuration to an agent card
func ConfigureAgentCardSecurity(card *types.AgentCard, securityConfig AgentCardSecurityConfig) {
	if card.SecuritySchemes == nil {
		card.SecuritySchemes = make(map[string]types.SecurityScheme)
	}

	// Reset security array to ensure clean state
	card.Security = nil

	var securityRequirement map[string][]string

	// Configure OIDC security
	if securityConfig.EnableOIDC && securityConfig.OIDCIssuerURL != "" {
		card.SecuritySchemes["oidc"] = CreateOIDCSecurityScheme(
			securityConfig.OIDCIssuerURL,
			"OpenID Connect authentication",
		)
		if securityRequirement == nil {
			securityRequirement = make(map[string][]string)
		}
		securityRequirement["oidc"] = []string{}
	}

	// Configure API Key security
	if securityConfig.EnableAPIKey && securityConfig.APIKeyName != "" {
		location := securityConfig.APIKeyLocation
		if location == "" {
			location = "header" // default location
		}
		card.SecuritySchemes["api_key"] = CreateAPIKeySecurityScheme(
			securityConfig.APIKeyName,
			location,
			"API key authentication",
		)
		if securityRequirement == nil {
			securityRequirement = make(map[string][]string)
		}
		securityRequirement["api_key"] = []string{}
	}

	// Configure mutual TLS security
	if securityConfig.EnableMutualTLS {
		card.SecuritySchemes["mtls"] = CreateMutualTLSSecurityScheme(
			"Mutual TLS authentication",
		)
		if securityRequirement == nil {
			securityRequirement = make(map[string][]string)
		}
		securityRequirement["mtls"] = []string{}
	}

	// Add security requirements to the agent card
	if securityRequirement != nil {
		card.Security = []map[string][]string{securityRequirement}
	}

	// Set authenticated extended card support
	card.SupportsAuthenticatedExtendedCard = BoolPtr(securityConfig.SupportsAuthenticatedExtendedCard)
}

// CreateSecurityConfigFromAuthConfig creates security configuration from auth config
func CreateSecurityConfigFromAuthConfig(authConfig config.AuthConfig) AgentCardSecurityConfig {
	return AgentCardSecurityConfig{
		EnableOIDC:                        authConfig.Enable && authConfig.IssuerURL != "",
		OIDCIssuerURL:                     authConfig.IssuerURL,
		SupportsAuthenticatedExtendedCard: authConfig.SupportsAuthenticatedExtendedCard,
		EnableAPIKey:                      authConfig.EnableAPIKey,
		APIKeyName:                        authConfig.APIKeyHeader,
		APIKeyLocation:                    "header",
		EnableMutualTLS:                   authConfig.EnableMutualTLS,
	}
}

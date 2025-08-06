package middlewares

import (
	"context"
	"net/http"
	"strings"

	oidcV3 "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/adk/server/config"
	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type contextKey string

const (
	AuthTokenContextKey contextKey = "authToken"
	IDTokenContextKey   contextKey = "idToken"
)

// OIDCAuthenticator interface for authentication middleware
type OIDCAuthenticator interface {
	Middleware() gin.HandlerFunc
}

// OIDCAuthenticatorImpl implements OIDC authentication
type OIDCAuthenticatorImpl struct {
	logger   *zap.Logger
	verifier *oidcV3.IDTokenVerifier
	config   oauth2.Config
}

// OIDCAuthenticatorNoop is a no-op authenticator for when auth is disabled
type OIDCAuthenticatorNoop struct{}

// NewOIDCAuthenticatorMiddleware creates a new OIDC authenticator middleware
func NewOIDCAuthenticatorMiddleware(logger *zap.Logger, cfg config.Config) (OIDCAuthenticator, error) {
	if !cfg.AuthConfig.Enable {
		return &OIDCAuthenticatorNoop{}, nil
	}

	if cfg.AuthConfig.IssuerURL == "" || cfg.AuthConfig.ClientID == "" || cfg.AuthConfig.ClientSecret == "" {
		logger.Warn("AuthConfig is enabled but required fields are missing, disabling authentication")
		return &OIDCAuthenticatorNoop{}, nil
	}

	provider, err := oidcV3.NewProvider(context.Background(), cfg.AuthConfig.IssuerURL)
	if err != nil {
		return nil, err
	}

	oidcConfig := &oidcV3.Config{
		ClientID: cfg.AuthConfig.ClientID,
	}

	return &OIDCAuthenticatorImpl{
		logger:   logger,
		verifier: provider.Verifier(oidcConfig),
		config: oauth2.Config{
			ClientID:     cfg.AuthConfig.ClientID,
			ClientSecret: cfg.AuthConfig.ClientSecret,
			Endpoint:     provider.Endpoint(),
			Scopes:       []string{oidcV3.ScopeOpenID, "profile", "email"},
		},
	}, nil
}

// Middleware returns the OIDC authentication middleware for OIDCAuthenticatorImpl
func (auth *OIDCAuthenticatorImpl) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			auth.logger.Error("missing authorization header")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			auth.logger.Error("invalid authorization header format")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		idToken, err := auth.verifier.Verify(c.Request.Context(), token)
		if err != nil {
			auth.logger.Error("failed to verify id token", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set(string(AuthTokenContextKey), token)
		c.Set(string(IDTokenContextKey), idToken)
		c.Next()
	}
}

// Middleware returns a no-op middleware for OIDCAuthenticatorNoop
func (auth *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// SecurityValidator interface for validating security requirements
type SecurityValidator interface {
	ValidateSecurityRequirements(agentCard *types.AgentCard) gin.HandlerFunc
}

// SecurityValidatorImpl implements security requirement validation
type SecurityValidatorImpl struct {
	logger *zap.Logger
	config config.Config
}

// SecurityValidatorNoop is a no-op security validator
type SecurityValidatorNoop struct{}

// NewSecurityValidator creates a new security validator
func NewSecurityValidator(logger *zap.Logger, cfg config.Config) SecurityValidator {
	if !cfg.AuthConfig.Enable {
		return &SecurityValidatorNoop{}
	}

	return &SecurityValidatorImpl{
		logger: logger,
		config: cfg,
	}
}

// ValidateSecurityRequirements validates that the request meets security requirements
func (sv *SecurityValidatorImpl) ValidateSecurityRequirements(agentCard *types.AgentCard) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip validation for non-authenticated endpoints
		if agentCard == nil || len(agentCard.Security) == 0 {
			c.Next()
			return
		}

		// Check if any security requirement is satisfied
		var lastError string
		satisfied := false

		for _, securityGroup := range agentCard.Security {
			groupSatisfied := true

			for schemeName := range securityGroup {
				scheme, exists := agentCard.SecuritySchemes[schemeName]
				if !exists {
					sv.logger.Warn("security scheme not found in agent card",
						zap.String("scheme", schemeName))
					groupSatisfied = false
					lastError = "security scheme configuration error"
					break
				}

				if !sv.validateSecurityScheme(c, schemeName, scheme) {
					groupSatisfied = false
					lastError = "authentication credentials not provided or invalid"
					break
				}
			}

			if groupSatisfied {
				satisfied = true
				break
			}
		}

		if !satisfied {
			sv.logger.Error("security validation failed", zap.String("error", lastError))
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "Authentication required",
				"message": lastError,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// validateSecurityScheme validates a specific security scheme
func (sv *SecurityValidatorImpl) validateSecurityScheme(c *gin.Context, schemeName string, scheme types.SecurityScheme) bool {
	switch s := scheme.(type) {
	case types.OpenIdConnectSecurityScheme:
		return sv.validateOIDC(c)
	case types.HTTPAuthSecurityScheme:
		return sv.validateHTTPAuth(c, s)
	case types.APIKeySecurityScheme:
		return sv.validateAPIKey(c, s)
	case types.MutualTLSSecurityScheme:
		return sv.validateMutualTLS(c)
	default:
		sv.logger.Warn("unsupported security scheme type", zap.String("scheme", schemeName))
		return false
	}
}

// validateOIDC validates OIDC authentication
func (sv *SecurityValidatorImpl) validateOIDC(c *gin.Context) bool {
	// Check if OIDC token is present in context (set by OIDC middleware)
	token, exists := c.Get(string(IDTokenContextKey))
	return exists && token != nil
}

// validateHTTPAuth validates HTTP authentication
func (sv *SecurityValidatorImpl) validateHTTPAuth(c *gin.Context, scheme types.HTTPAuthSecurityScheme) bool {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return false
	}

	if scheme.Scheme == "bearer" {
		return strings.HasPrefix(strings.ToLower(authHeader), "bearer ")
	}

	if scheme.Scheme == "basic" {
		return strings.HasPrefix(strings.ToLower(authHeader), "basic ")
	}

	return false
}

// validateAPIKey validates API key authentication
func (sv *SecurityValidatorImpl) validateAPIKey(c *gin.Context, scheme types.APIKeySecurityScheme) bool {
	var value string

	switch scheme.In {
	case "header":
		value = c.GetHeader(scheme.Name)
	case "query":
		value = c.Query(scheme.Name)
	case "cookie":
		cookie, err := c.Cookie(scheme.Name)
		if err != nil {
			return false
		}
		value = cookie
	default:
		return false
	}

	return value != ""
}

// validateMutualTLS validates mutual TLS authentication
func (sv *SecurityValidatorImpl) validateMutualTLS(c *gin.Context) bool {
	// Check if client certificate is present
	if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
		return false
	}

	// Additional validation could be performed here
	return true
}

// ValidateSecurityRequirements returns a no-op middleware for SecurityValidatorNoop
func (sv *SecurityValidatorNoop) ValidateSecurityRequirements(agentCard *types.AgentCard) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

package middlewares

import (
	"context"
	"errors"
	"net/http"
	"strings"

	oidcV3 "github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	config "github.com/inference-gateway/adk/server/config"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

type contextKey string

const (
	ClaimsContextKey contextKey = "claims"
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
	if !cfg.AuthConfig.Enabled {
		return &OIDCAuthenticatorNoop{}, nil
	}

	if cfg.AuthConfig.IssuerURL == "" || cfg.AuthConfig.ClientID == "" || cfg.AuthConfig.ClientSecret == "" {
		return nil, errors.New("authentication is enabled but required OIDC fields (issuer URL, client ID, client secret) are missing")
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
			c.Header("WWW-Authenticate", "Bearer")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			auth.logger.Error("invalid authorization header format")
			c.Header("WWW-Authenticate", `Bearer error="invalid_request"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header format"})
			c.Abort()
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		idToken, err := auth.verifier.Verify(c.Request.Context(), token)
		if err != nil {
			auth.logger.Error("failed to verify id token", zap.Error(err))
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		// Extract verified claims and propagate to context so they survive
		// goroutine boundaries (the background task processor runs outside
		// the request scope).
		claims := make(map[string]any)
		if err := idToken.Claims(&claims); err != nil {
			auth.logger.Error("failed to extract id token claims", zap.Error(err))
			c.Header("WWW-Authenticate", `Bearer error="invalid_token"`)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}
		reqCtx := context.WithValue(c.Request.Context(), ClaimsContextKey, claims)
		c.Request = c.Request.WithContext(reqCtx)

		c.Next()
	}
}

// Middleware returns a no-op middleware for OIDCAuthenticatorNoop
func (auth *OIDCAuthenticatorNoop) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

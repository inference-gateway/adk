package server

import (
	"strings"

	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// OIDCSchemeName is the key used for the OpenID Connect security scheme when
// declaring it on an agent card via OIDCSecuritySchemes.
const OIDCSchemeName = "openId"

// BearerTokenSchemeName is the key used for the static bearer-token security scheme
// when declaring it on an agent card via BearerTokenSecuritySchemes.
const BearerTokenSchemeName = "bearer"

// BearerTokenSecuritySchemes builds the agent card security declaration for the
// static bearer-token mode (AUTH_TOKEN): one HTTP `bearer` scheme plus a matching
// requirement. Attach it the same way as OIDCSecuritySchemes.
func BearerTokenSecuritySchemes() (map[string]types.SecurityScheme, []types.SecurityRequirement) {
	schemes := map[string]types.SecurityScheme{
		BearerTokenSchemeName: {
			HTTPAuthSecurityScheme: &types.HTTPAuthSecurityScheme{Scheme: "bearer"},
		},
	}
	security := []types.SecurityRequirement{
		{Schemes: map[string]types.StringList{BearerTokenSchemeName: {List: []string{}}}},
	}
	return schemes, security
}

// OIDCSecuritySchemes builds the agent card security declaration (spec section 7)
// from the OIDC auth configuration. It returns a single openIdConnect scheme,
// keyed by OIDCSchemeName, whose discovery URL is derived from the issuer, plus a
// matching security requirement referencing that scheme.
//
// Attach the result to an agent card before serving it so clients can discover
// how to authenticate:
//
//	schemes, security := server.OIDCSecuritySchemes(cfg.AuthConfig)
//	card.SecuritySchemes = schemes
//	card.SecurityRequirements = security
func OIDCSecuritySchemes(cfg serverConfig.AuthConfig) (map[string]types.SecurityScheme, []types.SecurityRequirement) {
	discoveryURL := strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/openid-configuration"

	schemes := map[string]types.SecurityScheme{
		OIDCSchemeName: {
			OpenIDConnectSecurityScheme: &types.OpenIDConnectSecurityScheme{
				OpenIDConnectURL: discoveryURL,
			},
		},
	}

	security := []types.SecurityRequirement{
		{Schemes: map[string]types.StringList{OIDCSchemeName: {List: []string{}}}},
	}

	return schemes, security
}

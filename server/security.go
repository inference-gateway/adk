package server

import (
	"strings"

	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

// OIDCSchemeName is the key used for the OpenID Connect security scheme when
// declaring it on an agent card via OIDCSecuritySchemes.
const OIDCSchemeName = "openId"

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
//	card.Security = security
func OIDCSecuritySchemes(cfg config.AuthConfig) (map[string]types.SecurityScheme, []types.Security) {
	discoveryURL := strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/openid-configuration"

	schemes := map[string]types.SecurityScheme{
		OIDCSchemeName: {
			OpenIDConnectSecurityScheme: &types.OpenIDConnectSecurityScheme{
				OpenIDConnectURL: discoveryURL,
			},
		},
	}

	security := []types.Security{
		{Schemes: map[string]types.StringList{OIDCSchemeName: {List: []string{}}}},
	}

	return schemes, security
}

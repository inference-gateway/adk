# Authentication and Authorization

The ADK follows the [A2A specification, section 7](https://a2a-protocol.org/latest/specification/#7-authentication-and-authorization). Authentication is **card-driven**: the agent card advertises how to authenticate, and clients transmit credentials obtained out-of-band on every request. A2A does not run OAuth flows in-protocol.

## The flow

1. **Discovery** - the client fetches the public card from `/.well-known/agent-card.json` (always unauthenticated). The card declares:
   - `securitySchemes` - named schemes the agent accepts (`apiKey`, `http`, `oauth2`, `openIdConnect`, `mutualTLS`).
   - `security` - a requirement list with OR-of-ANDs semantics; satisfying any one entry is sufficient.
2. **Credential acquisition is out-of-band** - the client obtains a token/key however the chosen scheme dictates.
3. **Transmission** - the client sends the credential (e.g. `Authorization: Bearer <token>`) on every request.
4. **Server enforcement** - with `AUTH_ENABLED=true` the `/a2a` endpoint is protected; unauthenticated requests get `401` with a `WWW-Authenticate` challenge.
5. **Extended card** - if the card sets `supportsExtendedAgentCard: true`, an authenticated client MAY call `agent/getAuthenticatedExtendedCard` to receive a richer card and SHOULD replace its cached public card with the response.

## Declaring security schemes on the card

With `AUTH_ENABLED=true` the served card must declare `securitySchemes` so clients can discover how to authenticate. A helper builds the OIDC declaration from the auth config:

```go
schemes, security := server.OIDCSecuritySchemes(cfg.AuthConfig)
card.SecuritySchemes = schemes
card.Security = security
```

Or declare it directly in a card JSON:

```json
{
  "securitySchemes": {
    "openId": {
      "openIdConnectSecurityScheme": {
        "openIdConnectUrl": "https://issuer.example.com/realms/app/.well-known/openid-configuration"
      }
    }
  },
  "security": [{ "schemes": { "openId": { "list": [] } } }]
}
```

If `AUTH_ENABLED=true` but the card declares no `securitySchemes` (or the inverse), the server logs a startup warning - the discovery step would otherwise be broken.

## Configuring the extended card

The extended card is served only to authenticated callers via `agent/getAuthenticatedExtendedCard`. Configure it with the builder; this also forces `supportsExtendedAgentCard: true` on the public card:

```go
srv, _ := server.NewA2AServerBuilder(cfg, logger).
    WithAgentCard(publicCard).
    WithExtendedAgentCard(extendedCard). // extra skills, capability detail, ...
    WithDefaultTaskHandlers().
    Build()
```

### Error contract (spec 3.3.4)

- Card does not declare `supportsExtendedAgentCard` -> `-32004` (UnsupportedOperationError)
- Flag is `true` but no extended card is configured -> `-32007` (ExtendedAgentCardNotConfigured)
- Flag is `true` and an extended card is configured -> the extended card is returned

## Client-side flow

```go
c := client.NewClient("https://agent.example.com")

// 1. Discover how to authenticate.
card, _ := c.GetAgentCard(ctx)
_ = card.SecuritySchemes // inspect which schemes the agent accepts

// 2. Attach the out-of-band credential to every request.
authed := client.NewClientWithConfig(&client.Config{
    BaseURL: "https://agent.example.com",
    Headers: map[string]string{"Authorization": "Bearer " + token},
})
// (or c.(*client.Client).SetHeader("Authorization", "Bearer "+token) on a mutable client)

// 3. Fetch the richer authenticated card.
resp, _ := authed.GetAuthenticatedExtendedCard(ctx, types.GetAuthenticatedExtendedCardParams{})
```

## Authorization via callbacks

Authentication answers "who is calling"; authorization ("may they do this") is implementation-specific per spec 7.5. Use the agent callbacks as the policy seam rather than framework code:

```go
agent, _ := server.NewAgentBuilder(logger).
    WithLLMClient(llm).
    WithCallbacks(&server.CallbackConfig{
        BeforeAgent: []server.BeforeAgentCallback{
            func(ctx context.Context, cbCtx *server.CallbackContext) *types.Message {
                // inspect identity from ctx; return a non-nil message to short-circuit (deny).
                return nil
            },
        },
    }).
    Build()
```

`BeforeTool` / `BeforeModel` callbacks work the same way for finer-grained gating.

## Out of scope

- **mTLS** - transport-layer; terminate at the proxy/load balancer. A card may declare the `mutualTLS` scheme, but there is nothing for ADK app code to implement.
- **OAuth2 flow engines** - credential acquisition is out-of-band (spec 7.3); the client only transmits credentials.
- **API key middleware** - the flow is spec-compliant with OIDC alone. Open an issue if a consumer needs API key enforcement.

# Authentication Flow Example

Demonstrates the A2A **card-driven authentication flow** ([spec section 7](https://a2a-protocol.org/latest/specification/#7-authentication-and-authorization) and the extended-card contract in 3.3.4).

The flow has three steps:

1. **Discovery** - the client fetches the public agent card (unauthenticated) and reads its `securitySchemes` to learn how to authenticate. The server declares an OpenID Connect scheme via `server.OIDCSecuritySchemes(cfg.AuthConfig)`.
2. **Credentials** - the client obtains a credential out of band (from the discovered OIDC provider) and attaches it as an `Authorization` header on every request.
3. **Extended card** - the authenticated client calls `agent/getAuthenticatedExtendedCard` to receive a richer card. The server exposes it via `WithExtendedAgentCard()`, which also advertises `supportsExtendedAgentCard: true` on the public card.

## Running

```bash
# terminal 1
cd server && go run .

# terminal 2
cd client && go run .
```

Authentication is **disabled by default** (`AUTH_ENABLE=false`) so the example runs without a live OIDC provider - the discovery half of the flow and the extended-card contract are still exercised end to end. To enforce the declared scheme, run the server with:

```bash
A2A_AUTH_ENABLE=true A2A_AUTH_ISSUER_URL=https://your-issuer/realms/demo go run .
```

and point the client's `TOKEN` at a valid token from that provider.

## Error contract (spec 3.3.4)

`agent/getAuthenticatedExtendedCard` returns:

- `-32004` (unsupported operation) when the card does not advertise `supportsExtendedAgentCard`.
- `-32007` (extended card not configured) when the flag is set but no extended card is configured.
- the extended card otherwise.

See [`docs/authentication.md`](../../docs/authentication.md) for the full write-up.

# Authentication Flow Example

Demonstrates the A2A **card-driven authentication flow**
([spec section 7](https://a2a-protocol.org/latest/specification/#7-authentication-and-authorization)
and the extended-card contract in 3.3.4).

The flow has three steps:

1. **Discovery** - the client fetches the public agent card (unauthenticated) and reads its
   `securitySchemes` to learn how to authenticate. The server declares an OpenID Connect scheme
   via `server.OIDCSecuritySchemes(cfg.AuthConfig)`.
2. **Credentials** - the client obtains a credential out of band (from the discovered OIDC
   provider) and attaches it as an `Authorization` header on every request.
3. **Extended card** - the authenticated client calls `agent/getAuthenticatedExtendedCard` to
   receive a richer card. The server exposes it via `WithExtendedAgentCard()`, which also
   advertises `supportsExtendedAgentCard: true` on the public card.

## Running (no auth)

```bash
# terminal 1
cd server && go run .

# terminal 2
cd client && go run .
```

Authentication is **disabled by default** (`AUTH_ENABLED=false`) so the example runs without a
live OIDC provider - the discovery half of the flow and the extended-card contract are still
exercised end to end.

## Running with Keycloak (on-prem OIDC)

`docker-compose.yaml` brings up Keycloak with a pre-imported realm
(`keycloak/realm-import.json`): realm `inference-gateway-realm` and a confidential client
`inference-gateway-client` (secret `very-secret`) with a service account. An audience mapper puts
`inference-gateway-client` into the access token `aud`, which is what the ADK OIDC middleware
verifies against. The server only checks signatures against the issuer's public keys, so it never
needs the client secret; only the token request does.

```bash
# terminal 1 - start the issuer and wait for the realm to be importable
docker compose up -d --wait

# terminal 2 - server with auth enforced against Keycloak (A2A on :8090)
cd server && A2A_AUTH_ENABLED=true go run .

# terminal 3 - client with a token issued by Keycloak (client credentials grant)
TOKEN=$(curl -s http://localhost:8080/realms/inference-gateway-realm/protocol/openid-connect/token \
  -d grant_type=client_credentials -d client_id=inference-gateway-client -d client_secret=very-secret \
  | jq -r .access_token)
cd client && TOKEN=$TOKEN go run .
```

Keycloak owns `:8080` (the issuer), so the A2A server runs on `:8090`. `KC_HOSTNAME` pins the
issuer to `http://localhost:8080`, so tokens always carry the issuer the server discovered at
startup. Nothing here is Keycloak-specific: any OpenID Connect issuer works with
`A2A_AUTH_ISSUER_URL` plus `A2A_AUTH_AUDIENCE` (or `A2A_AUTH_CLIENT_ID`), see
[`docs/authentication.md`](../../docs/authentication.md#other-identity-providers).

## End-to-end test

`server/e2e_test.go` (build tag `e2e`) drives the full contract against the running Keycloak:
requests with no / bad token are rejected with **HTTP 401 + a `WWW-Authenticate` challenge and no
task submitted**, while requests carrying a valid Keycloak JWT submit the task and return the
authenticated extended card. Both `message/send` and `agent/getAuthenticatedExtendedCard` are
covered.

```bash
docker compose up -d
cd server && go test -tags e2e -v -run TestE2E
```

The test skips itself if Keycloak is not reachable.

## Error contract (spec 3.3.4)

`agent/getAuthenticatedExtendedCard` returns:

- `-32004` (unsupported operation) when the card does not advertise `supportsExtendedAgentCard`.
- `-32007` (extended card not configured) when the flag is set but no extended card is configured.
- the extended card otherwise.

See [`docs/authentication.md`](../../docs/authentication.md) for the full write-up.

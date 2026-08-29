//go:build e2e

// End-to-end test for the card-driven authentication flow against a real
// Keycloak issuer. Requires the Keycloak from docker-compose.yaml to be running:
//
//	docker compose up -d
//	cd server && go test -tags e2e -v -run TestE2E
//
// It asserts the two halves of the contract the issue asks for:
//   - unauthenticated / bad-token requests are rejected with HTTP 401 and a
//     WWW-Authenticate challenge, and no task is submitted;
//   - requests carrying a valid Keycloak JWT succeed - the task is submitted and
//     the authenticated extended card is returned.
package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	zap "go.uber.org/zap"

	types "github.com/inference-gateway/adk/types"
)

const (
	keycloakBase = "http://localhost:8080/realms/inference-gateway-realm"
	serverPort   = "8090"
	serverBase   = "http://localhost:" + serverPort
)

// fetchToken performs a Keycloak direct-access-grant (password) login and
// returns the access token. Skips the whole test if Keycloak is unreachable.
func fetchToken(t *testing.T) string {
	t.Helper()

	form := url.Values{
		"grant_type":    {"password"},
		"client_id":     {"inference-gateway-client"},
		"client_secret": {"inference-gateway-secret"},
		"username":      {"demo"},
		"password":      {"demo"},
		"scope":         {"openid"},
	}

	resp, err := http.PostForm(keycloakBase+"/protocol/openid-connect/token", form)
	if err != nil {
		t.Skipf("keycloak not reachable (%v); run `docker compose up -d` first", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("token request failed: %d %s", resp.StatusCode, body)
	}

	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode token response: %v", err)
	}
	if out.AccessToken == "" {
		t.Fatalf("empty access token in %s", body)
	}
	return out.AccessToken
}

// rpc posts a JSON-RPC request to /a2a with the given bearer token ("" for none)
// and returns the HTTP status, the WWW-Authenticate header, and the raw body.
func rpc(t *testing.T, token, method string, params any) (int, string, []byte) {
	t.Helper()

	reqBody, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, serverBase+"/a2a", strings.NewReader(string(reqBody)))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("WWW-Authenticate"), body
}

func sendMessageParams(text string) types.MessageSendParams {
	return types.MessageSendParams{
		Message: types.Message{
			MessageID: "e2e-" + text,
			Role:      types.RoleUser,
			Parts:     []types.Part{types.CreateTextPart(text)},
		},
	}
}

// startServer builds the example server with auth enforced against Keycloak and
// starts it, returning a stop func. Waits until the card endpoint is live.
func startServer(t *testing.T) func() {
	t.Helper()

	logger := zap.NewNop()
	cfg := defaultConfig()
	cfg.A2A.AuthConfig.Enabled = true // enforce Keycloak
	cfg.A2A.AuthConfig.IssuerURL = keycloakBase
	cfg.A2A.ServerConfig.Port = serverPort

	srv, err := buildServer(cfg, logger)
	if err != nil {
		t.Fatalf("build server: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := srv.Start(ctx); err != nil && ctx.Err() == nil {
			t.Errorf("server start: %v", err)
		}
	}()

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(serverBase + "/.well-known/agent-card.json")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return func() {
					cancel()
					stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer stopCancel()
					_ = srv.Stop(stopCtx)
				}
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	cancel()
	t.Fatal("server did not become ready")
	return func() {}
}

func TestE2E_AuthenticationFlow(t *testing.T) {
	token := fetchToken(t) // skips if Keycloak is down
	stop := startServer(t)
	defer stop()

	// --- rejection: no task is submitted without a valid credential ---
	rejections := []struct {
		name   string
		token  string
		method string
		params any
	}{
		{"send/no-token", "", "message/send", sendMessageParams("hello")},
		{"send/bad-token", "not-a-jwt", "message/send", sendMessageParams("hello")},
		{"extendedCard/no-token", "", "agent/getAuthenticatedExtendedCard", map[string]any{}},
		{"extendedCard/bad-token", "not-a-jwt", "agent/getAuthenticatedExtendedCard", map[string]any{}},
	}
	for _, tc := range rejections {
		t.Run("reject/"+tc.name, func(t *testing.T) {
			status, challenge, body := rpc(t, tc.token, tc.method, tc.params)
			if status != http.StatusUnauthorized {
				t.Fatalf("want 401, got %d: %s", status, body)
			}
			if challenge == "" {
				t.Errorf("missing WWW-Authenticate challenge header (spec 7)")
			}
			if strings.Contains(string(body), `"result"`) {
				t.Errorf("rejected request must not carry a result: %s", body)
			}
		})
	}

	// --- happy path: task is submitted with a valid Keycloak JWT ---
	t.Run("accept/send", func(t *testing.T) {
		status, _, body := rpc(t, token, "message/send", sendMessageParams("ping"))
		if status != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", status, body)
		}
		var out struct {
			Result *types.Task `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, body)
		}
		if out.Error != nil {
			t.Fatalf("unexpected error: %s", out.Error.Message)
		}
		if out.Result == nil || out.Result.ID == "" {
			t.Fatalf("task was not submitted: %s", body)
		}
		// message/send enqueues the task and returns it in the submitted state;
		// processing then continues on the background queue.
		if out.Result.Status.State != types.TaskStateSubmitted {
			t.Fatalf("task not submitted, state=%s", out.Result.Status.State)
		}
		t.Logf("task submitted: id=%s state=%s", out.Result.ID, out.Result.Status.State)
	})

	// --- happy path: extended card returned to an authenticated caller ---
	t.Run("accept/extendedCard", func(t *testing.T) {
		status, _, body := rpc(t, token, "agent/getAuthenticatedExtendedCard", map[string]any{})
		if status != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", status, body)
		}
		var out struct {
			Result *types.AgentCard `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(body, &out); err != nil {
			t.Fatalf("decode: %v (%s)", err, body)
		}
		if out.Error != nil {
			t.Fatalf("unexpected error %d: %s", out.Error.Code, out.Error.Message)
		}
		if out.Result == nil || len(out.Result.Skills) == 0 {
			t.Fatalf("extended card not returned: %s", body)
		}
		t.Logf("extended card returned: %q skills=%d", out.Result.Description, len(out.Result.Skills))
	})
}

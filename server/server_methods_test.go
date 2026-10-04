package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	mocks "github.com/inference-gateway/adk/server/mocks"

	zaptest "go.uber.org/zap/zaptest"

	sdk "github.com/inference-gateway/sdk"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

const methodNotFoundCode = -32601

func TestServer_DispatchesOnlyA2AV1MethodNames(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	baseURL := startServer(t, srv, cfg.ServerConfig.Port)

	v1Methods := []string{
		"SendMessage", "SendStreamingMessage", "GetTask", "ListTasks", "CancelTask", "SubscribeToTask",
		"CreateTaskPushNotificationConfig", "GetTaskPushNotificationConfig", "ListTaskPushNotificationConfigs",
		"DeleteTaskPushNotificationConfig", "GetExtendedAgentCard",
	}
	for _, method := range v1Methods {
		t.Run(method+" is dispatched", func(t *testing.T) {
			body, err := io.ReadAll(postA2A(t, baseURL, "", method).Body)
			require.NoError(t, err)
			assert.NotContains(t, string(body), `"code":-32601`)
		})
	}

	v0Methods := []string{
		"message/send", "message/stream", "tasks/get", "tasks/list", "tasks/cancel", "tasks/resubscribe",
		"tasks/pushNotificationConfig/set", "tasks/pushNotificationConfig/get", "tasks/pushNotificationConfig/list",
		"tasks/pushNotificationConfig/delete", "agent/getAuthenticatedExtendedCard",
	}
	for _, method := range v0Methods {
		t.Run(method+" is method not found", func(t *testing.T) {
			var resp struct {
				Error struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.NewDecoder(postA2A(t, baseURL, "", method).Body).Decode(&resp))
			assert.Equal(t, methodNotFoundCode, resp.Error.Code)
		})
	}
}

func TestServer_RejectsPushNotificationConfigWhenCapabilityDisabled(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	card.Capabilities.PushNotifications = new(false)
	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	baseURL := startServer(t, srv, cfg.ServerConfig.Port)

	methods := []string{
		"CreateTaskPushNotificationConfig", "GetTaskPushNotificationConfig",
		"ListTaskPushNotificationConfigs", "DeleteTaskPushNotificationConfig",
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var payload struct {
				Error struct {
					Code int              `json:"code"`
					Data []map[string]any `json:"data"`
				} `json:"error"`
			}
			require.NoError(t, json.NewDecoder(postA2A(t, baseURL, "", method).Body).Decode(&payload))
			assert.Equal(t, -32003, payload.Error.Code)
			require.Len(t, payload.Error.Data, 1)
			assert.Equal(t, "PUSH_NOTIFICATION_NOT_SUPPORTED", payload.Error.Data[0]["reason"])
			assert.Equal(t, "a2a-protocol.org", payload.Error.Data[0]["domain"])
		})
	}
}

func TestServer_RejectsUnsupportedA2AVersion(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	baseURL := startServer(t, srv, cfg.ServerConfig.Port)

	tests := []struct {
		version  string
		wantCode int
	}{
		{version: "99.0", wantCode: -32009},
		{version: "1.0", wantCode: -32001},
		{version: "", wantCode: -32001},
	}
	for _, tt := range tests {
		t.Run("version "+tt.version, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":"1","method":"GetTask","params":{"id":"missing"}}`
			req, err := http.NewRequest(http.MethodPost, baseURL+"/a2a", strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			if tt.version != "" {
				req.Header.Set("A2A-Version", tt.version)
			}
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			var payload struct {
				Error struct {
					Code int `json:"code"`
				} `json:"error"`
			}
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
			assert.Equal(t, tt.wantCode, payload.Error.Code)
		})
	}
}

func TestServer_UsageExtension(t *testing.T) {
	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	baseURL := startServer(t, srv, cfg.ServerConfig.Port)

	t.Run("the agent card declares it as optional", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/.well-known/agent-card.json")
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		var served types.AgentCard
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&served))
		require.Len(t, served.Capabilities.Extensions, 1)
		assert.Equal(t, types.UsageExtensionURI, *served.Capabilities.Extensions[0].URI)
		assert.False(t, *served.Capabilities.Extensions[0].Required)
	})

	tests := []struct {
		name       string
		extensions []string
		want       string
	}{
		{name: "not requested", want: ""},
		{name: "requested alone", extensions: []string{types.UsageExtensionURI}, want: types.UsageExtensionURI},
		{name: "requested in a list", extensions: []string{"https://example.com/ext/other/v1, " + types.UsageExtensionURI}, want: types.UsageExtensionURI},
		{name: "requested in a second header", extensions: []string{"https://example.com/ext/other/v1", types.UsageExtensionURI}, want: types.UsageExtensionURI},
		{name: "only another extension", extensions: []string{"https://example.com/ext/other/v1"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":"1","method":"GetTask","params":{"id":"missing"}}`
			req, err := http.NewRequest(http.MethodPost, baseURL+"/a2a", strings.NewReader(body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/json")
			for _, value := range tt.extensions {
				req.Header.Add("A2A-Extensions", value)
			}
			resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			assert.Equal(t, tt.want, resp.Header.Get("A2A-Extensions"))
		})
	}
}

func TestServer_UsageExtensionGatesTaskMetadata(t *testing.T) {
	llm := &mocks.FakeLLMClient{}
	llm.CreateStreamingChatCompletionStub = func(ctx context.Context, _ []sdk.Message, _ ...sdk.ChatCompletionTool) (<-chan *sdk.CreateChatCompletionStreamResponse, <-chan error) {
		responses := make(chan *sdk.CreateChatCompletionStreamResponse)
		errs := make(chan error, 1)
		go func() {
			defer close(responses)
			defer close(errs)
			for _, chunk := range []*sdk.CreateChatCompletionStreamResponse{
				{Choices: []sdk.ChatCompletionStreamChoice{{Delta: sdk.ChatCompletionStreamResponseDelta{Content: "hi"}, FinishReason: "stop"}}},
				{Choices: []sdk.ChatCompletionStreamChoice{}, Usage: &sdk.CompletionUsage{PromptTokens: 7, CompletionTokens: 3, TotalTokens: 10}},
			} {
				select {
				case responses <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}()
		return responses, errs
	}
	agent, err := server.NewAgentBuilder(zaptest.NewLogger(t)).WithLLMClient(llm).Build()
	require.NoError(t, err)

	cfg := serverConfig.Config{}
	cfg.ServerConfig.Host = "127.0.0.1"
	cfg.ServerConfig.Port = freePort(t)
	card := createTestAgentCard()
	card.Capabilities.Streaming = new(false)
	srv, err := server.NewA2AServerBuilder(cfg, zaptest.NewLogger(t)).
		WithAgent(agent).
		WithAgentCard(card).
		WithDefaultBackgroundTaskHandler().
		Build()
	require.NoError(t, err)
	baseURL := startServer(t, srv, cfg.ServerConfig.Port)

	call := func(body, extensions string) types.Task {
		req, err := http.NewRequest(http.MethodPost, baseURL+"/a2a", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if extensions != "" {
			req.Header.Set("A2A-Extensions", extensions)
		}
		resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		var payload struct {
			Result json.RawMessage `json:"result"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&payload))
		var task types.Task
		var wrapped types.SendMessageResponse
		if json.Unmarshal(payload.Result, &wrapped) == nil && wrapped.Task != nil {
			return *wrapped.Task
		}
		require.NoError(t, json.Unmarshal(payload.Result, &task))
		return task
	}

	sent := call(`{"jsonrpc":"2.0","id":"1","method":"SendMessage","params":{"message":{"role":"ROLE_USER","parts":[{"text":"hi"}],"messageId":"m1"}}}`, "")
	require.Equal(t, types.TaskStateCompleted, sent.Status.State)
	if sent.Metadata != nil {
		assert.NotContains(t, *sent.Metadata, types.UsageMetadataKey, "an inactive extension must not reach the client")
	}

	got := call(fmt.Sprintf(`{"jsonrpc":"2.0","id":"2","method":"GetTask","params":{"id":%q}}`, sent.ID), types.UsageExtensionURI)
	require.NotNil(t, got.Metadata)
	usage, ok := (*got.Metadata)[types.UsageMetadataKey].(map[string]any)
	require.True(t, ok, "an activated extension returns the usage the stored task kept")
	assert.InDelta(t, 7, usage["prompt_tokens"], 0)
	assert.InDelta(t, 3, usage["completion_tokens"], 0)
}

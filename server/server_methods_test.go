package server_test

import (
	"encoding/json"
	"io"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	zaptest "go.uber.org/zap/zaptest"

	server "github.com/inference-gateway/adk/server"
	serverConfig "github.com/inference-gateway/adk/server/config"
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

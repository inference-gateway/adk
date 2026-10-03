package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
)

func TestHTTPPushNotificationSender_PostsStreamResponseAsA2AJSON(t *testing.T) {
	var contentType string
	var payload types.StreamResponse
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	task := &types.Task{ID: "task-1", Status: types.TaskStatus{State: types.TaskStateCompleted}}
	sender := server.NewHTTPPushNotificationSender(zap.NewNop())
	require.NoError(t, sender.SendTaskUpdate(context.Background(), types.TaskPushNotificationConfig{URL: webhook.URL}, task))

	assert.Equal(t, "application/a2a+json", contentType)
	require.NotNil(t, payload.Task)
	assert.Equal(t, "task-1", payload.Task.ID)
	assert.Equal(t, types.TaskStateCompleted, payload.Task.Status.State)
}

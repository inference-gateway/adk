package server_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"

	mocks "github.com/inference-gateway/adk/server/mocks"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	gin "github.com/gin-gonic/gin"
	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	types "github.com/inference-gateway/adk/types"
)

// makeProtocolHandlerWithMocks wires a DefaultA2AProtocolHandler against fresh mocks and a
// real DefaultResponseSender. Returning the real response sender lets the tests inspect the
// actual HTTP responses written to the gin recorder (instead of asserting on mock calls),
// which more closely mirrors what the JSON-RPC dispatcher emits in production.
func makeProtocolHandlerWithMocks(t *testing.T) (server.A2AProtocolHandler, *mocks.FakeStorage, *mocks.FakeTaskManager, server.ResponseSender) {
	t.Helper()
	logger := zap.NewNop()
	storage := &mocks.FakeStorage{}
	taskManager := &mocks.FakeTaskManager{}
	responseSender := server.NewDefaultResponseSender(logger)
	h := server.NewDefaultA2AProtocolHandler(logger, storage, taskManager, responseSender)
	return h, storage, taskManager, responseSender
}

func newRequestContext(t *testing.T, body string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/a2a", bytes.NewBufferString(body))
	return c, w
}

func TestProtocolHandler_HandleTaskResubscribe_TaskNotFound(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)
	taskManager.GetTaskReturns(nil, false)

	c, w := newRequestContext(t, "{}")

	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "SubscribeToTask",
		Params:  &types.Struct{"id": "missing-task"},
	}

	h.HandleTaskResubscribe(c, req, &mocks.FakeStreamableTaskHandler{})

	require.Equal(t, 1, taskManager.GetTaskCallCount())
	assert.Equal(t, "missing-task", taskManager.GetTaskArgsForCall(0))

	var resp types.JSONRPCErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, -32001, resp.Error.Code)
	require.NotNil(t, resp.Error.Data)
	errorDetails, err := json.Marshal(*resp.Error.Data)
	require.NoError(t, err)
	assert.Contains(t, string(errorDetails), `"reason":"TASK_NOT_FOUND"`)
}

func TestProtocolHandler_HandleTaskResubscribe_MissingName(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)

	c, w := newRequestContext(t, "{}")
	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "SubscribeToTask",
		Params:  &types.Struct{},
	}

	h.HandleTaskResubscribe(c, req, &mocks.FakeStreamableTaskHandler{})

	assert.Equal(t, 0, taskManager.GetTaskCallCount(), "no task lookup expected when name is missing")
	body := w.Body.String()
	assert.Contains(t, body, "task id is required")
}

func TestProtocolHandler_HandleTaskResubscribe_TerminalTaskIsUnsupported(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)
	taskManager.GetTaskReturns(&types.Task{
		ID:        "task-done",
		ContextID: new("ctx-1"),
		Status:    types.TaskStatus{State: types.TaskStateCompleted},
	}, true)

	c, w := newRequestContext(t, "{}")
	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "SubscribeToTask",
		Params:  &types.Struct{"id": "task-done"},
	}

	streamingHandler := &mocks.FakeStreamableTaskHandler{}
	h.HandleTaskResubscribe(c, req, streamingHandler)

	assert.Equal(t, 0, streamingHandler.HandleStreamingTaskCallCount())
	var resp types.JSONRPCErrorResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, -32004, resp.Error.Code)
}

func TestProtocolHandler_HandleTaskResubscribe_WorkingTaskInvokesStreamingHandler(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)
	workingTask := &types.Task{
		ID:        "task-working",
		ContextID: new("ctx-1"),
		Status: types.TaskStatus{
			State: types.TaskStateWorking,
		},
	}
	taskManager.GetTaskReturns(workingTask, true)

	streamingHandler := &mocks.FakeStreamableTaskHandler{}
	events := make(chan cloudevents.Event, 1)
	statusEvent := cloudevents.NewEvent()
	statusEvent.SetType(types.EventTaskStatusChanged)
	statusEvent.SetSource("test")
	require.NoError(t, statusEvent.SetData(cloudevents.ApplicationJSON, types.TaskStatus{
		State: types.TaskStateCompleted,
	}))
	events <- statusEvent
	close(events)
	streamingHandler.HandleStreamingTaskReturns(events, nil)

	c, w := newRequestContext(t, "{}")
	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "SubscribeToTask",
		Params:  &types.Struct{"id": "task-working"},
	}

	h.HandleTaskResubscribe(c, req, streamingHandler)

	assert.Equal(t, 1, streamingHandler.HandleStreamingTaskCallCount(),
		"streaming handler should be invoked for working tasks")

	body := w.Body.String()
	assert.NotContains(t, body, "[DONE]")
	assert.Equal(t, 2, strings.Count(body, "data: "), "the Task, then the status update")
	assert.Contains(t, body, `"statusUpdate"`)
}

func TestProtocolHandler_HandleGetAuthenticatedExtendedCard(t *testing.T) {
	makeCard := func(name string, supports *bool) *types.AgentCard {
		return &types.AgentCard{
			Name:               name,
			Description:        "test card",
			Version:            "1.2.3",
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
			Capabilities:       types.AgentCapabilities{ExtendedAgentCard: supports},
		}
	}
	truePtr := true
	falsePtr := false

	tests := []struct {
		name         string
		publicCard   *types.AgentCard
		extendedCard *types.AgentCard
		wantCode     int
		wantErrCode  float64
		wantName     string
	}{
		{
			name:        "nil public card is internal error",
			publicCard:  nil,
			wantErrCode: float64(server.ErrInternalError),
		},
		{
			name:        "flag absent returns unsupported operation",
			publicCard:  makeCard("agent", nil),
			wantErrCode: float64(server.ErrUnsupportedOperation),
		},
		{
			name:        "flag false returns unsupported operation",
			publicCard:  makeCard("agent", &falsePtr),
			wantErrCode: float64(server.ErrUnsupportedOperation),
		},
		{
			name:        "flag true but no extended card returns not configured",
			publicCard:  makeCard("agent", &truePtr),
			wantErrCode: float64(server.ErrExtendedAgentCardNotConfigured),
		},
		{
			name:         "flag true with extended card returns it",
			publicCard:   makeCard("public-agent", &truePtr),
			extendedCard: makeCard("extended-agent", &truePtr),
			wantCode:     200,
			wantName:     "extended-agent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _, _ := makeProtocolHandlerWithMocks(t)
			c, w := newRequestContext(t, "{}")
			reqID := any("req-1")
			req := types.JSONRPCRequest{
				JSONRPC: "2.0",
				ID:      &reqID,
				Method:  "GetExtendedAgentCard",
				Params:  &types.Struct{"tenant": "tenant-1"},
			}

			h.HandleGetAuthenticatedExtendedCard(c, req, tt.publicCard, tt.extendedCard)

			var payload map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &payload))
			assert.Equal(t, "2.0", payload["jsonrpc"])

			if tt.wantName != "" {
				assert.Equal(t, tt.wantCode, w.Code)
				result, ok := payload["result"].(map[string]any)
				require.True(t, ok, "result should decode as object")
				assert.Equal(t, tt.wantName, result["name"])
				return
			}

			errObj, ok := payload["error"].(map[string]any)
			require.True(t, ok, "error should be present")
			assert.Equal(t, tt.wantErrCode, errObj["code"])
		})
	}
}

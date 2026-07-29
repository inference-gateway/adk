package server_test

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/gin-gonic/gin"
	server "github.com/inference-gateway/adk/server"
	mocks "github.com/inference-gateway/adk/server/mocks"
	types "github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
		Method:  "tasks/resubscribe",
		Params:  map[string]any{"name": "missing-task"},
	}

	h.HandleTaskResubscribe(c, req, &mocks.FakeStreamableTaskHandler{})

	require.Equal(t, 1, taskManager.GetTaskCallCount())
	assert.Equal(t, "missing-task", taskManager.GetTaskArgsForCall(0))

	body := w.Body.String()
	assert.Contains(t, body, "task not found", "error message should be surfaced via SSE")
	assert.Contains(t, body, "data: ")
}

func TestProtocolHandler_HandleTaskResubscribe_MissingName(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)

	c, w := newRequestContext(t, "{}")
	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "tasks/resubscribe",
		Params:  map[string]any{},
	}

	h.HandleTaskResubscribe(c, req, &mocks.FakeStreamableTaskHandler{})

	assert.Equal(t, 0, taskManager.GetTaskCallCount(), "no task lookup expected when name is missing")
	body := w.Body.String()
	assert.Contains(t, body, "task name is required")
}

func TestProtocolHandler_HandleTaskResubscribe_CompletedTaskEmitsFinalState(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)
	completedTask := &types.Task{
		ID:        "task-done",
		ContextID: "ctx-1",
		Status: types.TaskStatus{
			State: types.TaskStateCompleted,
		},
	}
	taskManager.GetTaskReturns(completedTask, true)

	c, w := newRequestContext(t, "{}")
	reqID := any("req-1")
	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  "tasks/resubscribe",
		Params:  map[string]any{"name": "task-done"},
	}

	streamingHandler := &mocks.FakeStreamableTaskHandler{}
	h.HandleTaskResubscribe(c, req, streamingHandler)

	assert.Equal(t, 0, streamingHandler.HandleStreamingTaskCallCount(),
		"streaming handler should not be invoked for terminal tasks")

	body := w.Body.String()
	require.Contains(t, body, "data: ")
	require.Contains(t, body, "[DONE]")

	chunks := strings.Split(body, "data: ")
	require.GreaterOrEqual(t, len(chunks), 2, "expected at least one data event before [DONE]")
	firstEvent := strings.TrimSpace(chunks[1])

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(firstEvent), &payload))
	assert.Equal(t, "2.0", payload["jsonrpc"])

	result, ok := payload["result"].(map[string]any)
	require.True(t, ok, "result should be present")
	assert.Equal(t, "task-done", result["taskId"])
	assert.Equal(t, true, result["final"], "completed task should be marked final")
}

func TestProtocolHandler_HandleTaskResubscribe_WorkingTaskInvokesStreamingHandler(t *testing.T) {
	h, _, taskManager, _ := makeProtocolHandlerWithMocks(t)
	workingTask := &types.Task{
		ID:        "task-working",
		ContextID: "ctx-1",
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
		Method:  "tasks/resubscribe",
		Params:  map[string]any{"name": "task-working"},
	}

	h.HandleTaskResubscribe(c, req, streamingHandler)

	assert.Equal(t, 1, streamingHandler.HandleStreamingTaskCallCount(),
		"streaming handler should be invoked for working tasks")

	body := w.Body.String()
	assert.Contains(t, body, "[DONE]", "stream should terminate with [DONE]")
	assert.GreaterOrEqual(t, strings.Count(body, "data: "), 2)
}

func TestProtocolHandler_HandleGetAuthenticatedExtendedCard(t *testing.T) {
	makeCard := func(name string, supports *bool) *types.AgentCard {
		return &types.AgentCard{
			Name:                      name,
			Description:               "test card",
			Version:                   "1.2.3",
			ProtocolVersion:           "1.0",
			DefaultInputModes:         []string{"text/plain"},
			DefaultOutputModes:        []string{"text/plain"},
			Skills:                    []types.AgentSkill{},
			SupportsExtendedAgentCard: supports,
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
				Method:  "agent/getAuthenticatedExtendedCard",
				Params:  map[string]any{"tenant": "tenant-1"},
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

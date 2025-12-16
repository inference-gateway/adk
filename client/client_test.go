package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inference-gateway/adk/client"
	types "github.com/inference-gateway/adk/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		expected string
	}{
		{
			name:     "creates client with default config",
			baseURL:  "http://localhost:8080",
			expected: "http://localhost:8080",
		},
		{
			name:     "creates client with https url",
			baseURL:  "https://example.com",
			expected: "https://example.com",
		},
		{
			name:     "creates client with custom port",
			baseURL:  "http://localhost:9090",
			expected: "http://localhost:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client.NewClient(tt.baseURL)

			assert.NotNil(t, c)
			assert.Equal(t, tt.expected, c.GetBaseURL())
		})
	}
}

func TestNewClientWithConfig(t *testing.T) {
	tests := []struct {
		name         string
		setupConfig  func() *client.Config
		expectedURL  string
		expectedUA   string
		expectClient bool
	}{
		{
			name: "creates client with custom config",
			setupConfig: func() *client.Config {
				return &client.Config{
					BaseURL:    "http://custom.example.com",
					Timeout:    45 * time.Second,
					UserAgent:  "Custom-Agent/2.0",
					Headers:    map[string]string{"X-Custom": "value"},
					MaxRetries: 5,
					RetryDelay: 2 * time.Second,
				}
			},
			expectedURL:  "http://custom.example.com",
			expectedUA:   "Custom-Agent/2.0",
			expectClient: true,
		},
		{
			name: "creates client with minimal config",
			setupConfig: func() *client.Config {
				return &client.Config{
					BaseURL: "http://minimal.example.com",
				}
			},
			expectedURL:  "http://minimal.example.com",
			expectedUA:   "",
			expectClient: true,
		},
		{
			name: "creates client with custom http client",
			setupConfig: func() *client.Config {
				httpClient := &http.Client{Timeout: 10 * time.Second}
				return &client.Config{
					BaseURL:    "http://httpclient.example.com",
					HTTPClient: httpClient,
				}
			},
			expectedURL:  "http://httpclient.example.com",
			expectedUA:   "",
			expectClient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.setupConfig()
			c := client.NewClientWithConfig(config)

			assert.NotNil(t, c)
			assert.Equal(t, tt.expectedURL, c.GetBaseURL())
		})
	}
}

func TestDefaultConfig(t *testing.T) {
	tests := []struct {
		name               string
		baseURL            string
		expectedBaseURL    string
		expectedTimeout    time.Duration
		expectedUserAgent  string
		expectedMaxRetries int
		expectedRetryDelay time.Duration
	}{
		{
			name:               "creates default config with provided base url",
			baseURL:            "http://test.example.com",
			expectedBaseURL:    "http://test.example.com",
			expectedTimeout:    30 * time.Second,
			expectedUserAgent:  "A2A-Go-Client/1.0",
			expectedMaxRetries: 3,
			expectedRetryDelay: 1 * time.Second,
		},
		{
			name:               "creates default config with different url",
			baseURL:            "https://secure.example.com:8443",
			expectedBaseURL:    "https://secure.example.com:8443",
			expectedTimeout:    30 * time.Second,
			expectedUserAgent:  "A2A-Go-Client/1.0",
			expectedMaxRetries: 3,
			expectedRetryDelay: 1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := client.DefaultConfig(tt.baseURL)

			assert.NotNil(t, config)
			assert.Equal(t, tt.expectedBaseURL, config.BaseURL)
			assert.Equal(t, tt.expectedTimeout, config.Timeout)
			assert.Equal(t, tt.expectedUserAgent, config.UserAgent)
			assert.Equal(t, tt.expectedMaxRetries, config.MaxRetries)
			assert.Equal(t, tt.expectedRetryDelay, config.RetryDelay)
			assert.NotNil(t, config.Headers)
			assert.NotNil(t, config.Logger)
		})
	}
}

func TestNewClientWithLogger(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		logger   *zap.Logger
		expected string
	}{
		{
			name:     "creates client with development logger",
			baseURL:  "http://localhost:8080",
			logger:   zap.NewExample(),
			expected: "http://localhost:8080",
		},
		{
			name:     "creates client with no-op logger",
			baseURL:  "https://example.com",
			logger:   zap.NewNop(),
			expected: "https://example.com",
		},
		{
			name:     "creates client with nil logger (defaults to no-op)",
			baseURL:  "http://localhost:9090",
			logger:   nil,
			expected: "http://localhost:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := client.NewClientWithLogger(tt.baseURL, tt.logger)

			assert.NotNil(t, c)
			assert.Equal(t, tt.expected, c.GetBaseURL())

			logger := c.GetLogger()
			assert.NotNil(t, logger)
		})
	}
}

func TestClient_LoggerConfiguration(t *testing.T) {
	tests := []struct {
		name            string
		setupClient     func() client.A2AClient
		setupLogger     func() *zap.Logger
		expectLoggerSet bool
	}{
		{
			name: "set and get development logger",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:8080")
			},
			setupLogger: func() *zap.Logger {
				return zap.NewExample()
			},
			expectLoggerSet: true,
		},
		{
			name: "set nil logger defaults to no-op",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:8080")
			},
			setupLogger: func() *zap.Logger {
				return nil
			},
			expectLoggerSet: true,
		},
		{
			name: "get logger from config",
			setupClient: func() client.A2AClient {
				config := client.DefaultConfig("http://localhost:8080")
				config.Logger = zap.NewExample()
				return client.NewClientWithConfig(config)
			},
			setupLogger: func() *zap.Logger {
				return nil // Not setting via SetLogger
			},
			expectLoggerSet: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()

			if logger := tt.setupLogger(); logger != nil {
				c.SetLogger(logger)
			}

			retrievedLogger := c.GetLogger()
			if tt.expectLoggerSet {
				assert.NotNil(t, retrievedLogger)
			}
		})
	}
}

func TestClient_SendTask(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         types.MessageSendParams
		expectError    bool
		expectedResult bool
		errorContains  string
	}{
		{
			name: "successful message send",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					assert.Equal(t, "A2A-Go-Client/1.0", r.Header.Get("User-Agent"))

					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "message/send", req.Method)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]any{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]any{
								"state": "submitted",
							},
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Hello, world!"),
					},
				},
			},
			expectError:    false,
			expectedResult: true,
		},
		{
			name: "server returns error response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := map[string]any{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]any{
							"code":    -32602,
							"message": "invalid params",
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "test-msg-error",
					Role:      "user",
					Parts:     []types.Part{},
				},
			},
			expectError:   true,
			errorContains: "A2A error: invalid params",
		},
		{
			name: "server returns non-200 status",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					if _, err := w.Write([]byte("internal server error")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "test-msg-500",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("This should fail"),
					},
				},
			},
			expectError:   true,
			errorContains: "unexpected status code: 500",
		},
		{
			name: "server returns invalid json",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte("invalid json")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "test-msg-invalid",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Invalid response test"),
					},
				},
			},
			expectError:   true,
			errorContains: "failed to decode response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			resp, err := c.SendTask(ctx, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "2.0", resp.JSONRPC)
			}
		})
	}
}

func TestClient_GetTask(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         types.TaskQueryParams
		expectError    bool
		expectedResult bool
		errorContains  string
	}{
		{
			name: "successful task retrieval",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "tasks/get", req.Method)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]any{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]any{
								"state": "completed",
								"message": map[string]any{
									"kind":      "message",
									"messageId": "response-msg",
									"role":      "assistant",
									"parts": []any{
										map[string]any{
											"kind": "text",
											"text": "Task completed successfully",
										},
									},
								},
							},
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskQueryParams{
				ID:            "task-123",
				HistoryLength: &[]int{10}[0],
			},
			expectError:    false,
			expectedResult: true,
		},
		{
			name: "task not found error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := map[string]any{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]any{
							"code":    -32001,
							"message": "task not found",
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskQueryParams{
				ID: "nonexistent-task",
			},
			expectError:   true,
			errorContains: "A2A error: task not found",
		},
		{
			name: "minimal task query params",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]any{
							"id":        "task-minimal",
							"contextId": "ctx-minimal",
							"status": map[string]any{
								"state": "working",
							},
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskQueryParams{
				ID: "task-minimal",
			},
			expectError:    false,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			resp, err := c.GetTask(ctx, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "2.0", resp.JSONRPC)
			}
		})
	}
}

func TestClient_CancelTask(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         types.TaskIdParams
		expectError    bool
		expectedResult bool
		errorContains  string
	}{
		{
			name: "successful task cancellation",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "tasks/cancel", req.Method)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]any{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]any{
								"state": "canceled",
							},
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskIdParams{
				ID: "task-123",
			},
			expectError:    false,
			expectedResult: true,
		},
		{
			name: "task not cancelable error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := map[string]any{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]any{
							"code":    -32002,
							"message": "task not cancelable",
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskIdParams{
				ID: "completed-task",
			},
			expectError:   true,
			errorContains: "A2A error: task not cancelable",
		},
		{
			name: "task with metadata",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]any{
							"id":        "task-with-metadata",
							"contextId": "ctx-metadata",
							"status": map[string]any{
								"state": "canceled",
							},
						},
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			params: types.TaskIdParams{
				ID: "task-with-metadata",
				Metadata: map[string]any{
					"reason": "user_requested",
				},
			},
			expectError:    false,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			resp, err := c.CancelTask(ctx, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, "2.0", resp.JSONRPC)
			}
		})
	}
}

func TestClient_SendTaskStreaming(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         types.MessageSendParams
		expectError    bool
		errorContains  string
		expectedEvents int
	}{
		{
			name: "successful streaming response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
					assert.Equal(t, "text/event-stream", r.Header.Get("Accept"))

					var req types.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "message/stream", req.Method)

					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)

					events := []types.JSONRPCSuccessResponse{
						{
							JSONRPC: "2.0",
							ID:      req.ID,
							Result:  "Starting task processing...",
						},
						{
							JSONRPC: "2.0",
							ID:      req.ID,
							Result:  "Task in progress...",
						}, {
							JSONRPC: "2.0",
							ID:      req.ID,
							Result:  "Task completed!",
						},
					}

					for _, event := range events {
						eventBytes, err := json.Marshal(event)
						if err != nil {
							t.Errorf("Failed to marshal event: %v", err)
							return
						}
						if _, err := w.Write([]byte("data: ")); err != nil {
							t.Errorf("Failed to write data prefix: %v", err)
							return
						}
						if _, err := w.Write(eventBytes); err != nil {
							t.Errorf("Failed to write event: %v", err)
							return
						}
						if _, err := w.Write([]byte("\n\n")); err != nil {
							t.Errorf("Failed to write SSE terminator: %v", err)
							return
						}
						if flusher, ok := w.(http.Flusher); ok {
							flusher.Flush()
						}
					}

					if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
						t.Errorf("Failed to write termination signal: %v", err)
						return
					}
					if flusher, ok := w.(http.Flusher); ok {
						flusher.Flush()
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "stream-msg-1",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Stream this message"),
					},
				},
			},
			expectError:    false,
			expectedEvents: 3,
		},
		{name: "server returns error status for streaming",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusBadRequest)
					if _, err := w.Write([]byte("bad request")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "stream-error",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("This should fail"),
					},
				},
			},
			expectError:   true,
			errorContains: "unexpected status code: 400",
		},
		{name: "invalid json in stream",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte("data: invalid json stream\n\n")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "stream-invalid",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Invalid stream test"),
					},
				},
			},
			expectError:    false,
			expectedEvents: 0,
		},
		{
			name: "empty stream response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/event-stream")
					w.WriteHeader(http.StatusOK)

					if _, err := w.Write([]byte("data: [DONE]\n\n")); err != nil {
						t.Errorf("Failed to write termination signal: %v", err)
					}
				}))
			},
			params: types.MessageSendParams{
				Message: types.Message{
					MessageID: "stream-empty",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Empty stream test"),
					},
				},
			},
			expectError:    false,
			expectedEvents: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			eventChan, err := c.SendTaskStreaming(ctx, tt.params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, eventChan)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, eventChan)

				eventCount := 0
				timeout := time.After(200 * time.Millisecond)

			eventLoop:
				for {
					select {
					case _, ok := <-eventChan:
						if !ok {
							break eventLoop
						}
						eventCount++
					case <-timeout:
						break eventLoop
					}
				}

				assert.Equal(t, tt.expectedEvents, eventCount)
			}
		})
	}
}

func TestClient_RetryMechanism(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		maxRetries    int
		retryDelay    time.Duration
		expectError   bool
		errorContains string
		expectedTries int
	}{
		{
			name: "successful request on first try",
			setupServer: func() *httptest.Server {
				tries := 0
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					tries++

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      1,
						Result:  "success",
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			maxRetries:    3,
			retryDelay:    100 * time.Millisecond,
			expectError:   false,
			expectedTries: 1,
		}, {
			name: "successful request on second try",
			setupServer: func() *httptest.Server {
				tries := 0
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					tries++

					if tries == 1 {
						w.WriteHeader(http.StatusInternalServerError)
						return
					}

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      1,
						Result:  "success after retry",
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			maxRetries:    3,
			retryDelay:    50 * time.Millisecond,
			expectError:   true,
			expectedTries: 1,
		},
		{
			name: "exhausts all retries and fails",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// Always force connection close
					conn, _, _ := w.(http.Hijacker).Hijack()
					if err := conn.Close(); err != nil {
						t.Errorf("Failed to close connection: %v", err)
					}
				}))
			},
			maxRetries:    2,
			retryDelay:    50 * time.Millisecond,
			expectError:   true,
			errorContains: "failed to send request after 3 attempts",
			expectedTries: 3,
		},
		{
			name: "non-200 status returns immediate error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
					if _, err := w.Write([]byte("internal server error")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			maxRetries:    3,
			retryDelay:    50 * time.Millisecond,
			expectError:   true,
			errorContains: "unexpected status code: 500",
			expectedTries: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			config := &client.Config{
				BaseURL:    server.URL,
				MaxRetries: tt.maxRetries,
				RetryDelay: tt.retryDelay,
			}
			c := client.NewClientWithConfig(config)
			ctx := context.Background()

			params := types.MessageSendParams{
				Message: types.Message{
					MessageID: "retry-test",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Testing retry mechanism"),
					},
				},
			}

			_, err := c.SendTask(ctx, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		setupContext  func() (context.Context, context.CancelFunc)
		expectError   bool
		errorContains string
	}{
		{
			name: "context cancelled during request",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(50 * time.Millisecond)

					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      1,
						Result:  "should not reach here",
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
				return ctx, cancel
			},
			expectError:   true,
			errorContains: "context deadline exceeded",
		},
		{
			name: "context cancelled during retry",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					conn, _, _ := w.(http.Hijacker).Hijack()
					if err := conn.Close(); err != nil {
						t.Errorf("Failed to close connection: %v", err)
					}
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
				return ctx, cancel
			},
			expectError:   true,
			errorContains: "context deadline exceeded",
		},
		{
			name: "context with sufficient timeout succeeds",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := types.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      1,
						Result:  "success with timeout",
					}

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if err := json.NewEncoder(w).Encode(response); err != nil {
						t.Errorf("Failed to encode response: %v", err)
					}
				}))
			},
			setupContext: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				return ctx, cancel
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			config := &client.Config{
				BaseURL:    server.URL,
				MaxRetries: 2,
				RetryDelay: 50 * time.Millisecond,
			}
			c := client.NewClientWithConfig(config)

			ctx, cancel := tt.setupContext()
			defer cancel()

			params := types.MessageSendParams{
				Message: types.Message{
					MessageID: "context-test",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Testing context cancellation"),
					},
				},
			}

			_, err := c.SendTask(ctx, params)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestClient_Configuration(t *testing.T) {
	tests := []struct {
		name              string
		setupClient       func() client.A2AClient
		setupExpectations func(c client.A2AClient)
		validateConfig    func(t *testing.T, c client.A2AClient)
	}{
		{
			name: "set and get timeout",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:8080")
			},
			setupExpectations: func(c client.A2AClient) {
				c.SetTimeout(45 * time.Second)
			},
			validateConfig: func(t *testing.T, c client.A2AClient) {
				assert.NotNil(t, c)
			},
		},
		{
			name: "set custom http client",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:8080")
			},
			setupExpectations: func(c client.A2AClient) {
				customClient := &http.Client{
					Timeout: 10 * time.Second,
				}
				c.SetHTTPClient(customClient)
			},
			validateConfig: func(t *testing.T, c client.A2AClient) {
				assert.NotNil(t, c)
			},
		},
		{
			name: "get base url",
			setupClient: func() client.A2AClient {
				return client.NewClient("https://test.example.com:9443")
			},
			setupExpectations: func(c client.A2AClient) {

			},
			validateConfig: func(t *testing.T, c client.A2AClient) {
				assert.Equal(t, "https://test.example.com:9443", c.GetBaseURL())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			tt.setupExpectations(c)
			tt.validateConfig(t, c)
		})
	}
}

func TestClient_HeadersAndAuthentication(t *testing.T) {
	tests := []struct {
		name            string
		setupConfig     func() *client.Config
		expectedHeaders map[string]string
	}{
		{
			name: "custom headers in config",
			setupConfig: func() *client.Config {
				config := client.DefaultConfig("http://localhost:8080")
				config.Headers["Authorization"] = "Bearer token123"
				config.Headers["X-API-Key"] = "api-key-456"
				config.Headers["X-Custom"] = "custom-value"
				return config
			},
			expectedHeaders: map[string]string{
				"Content-Type":  "application/json",
				"User-Agent":    "A2A-Go-Client/1.0",
				"Authorization": "Bearer token123",
				"X-Api-Key":     "api-key-456",
				"X-Custom":      "custom-value",
			},
		},
		{
			name: "default headers only",
			setupConfig: func() *client.Config {
				return client.DefaultConfig("http://localhost:8080")
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "A2A-Go-Client/1.0",
			},
		},
		{
			name: "custom user agent",
			setupConfig: func() *client.Config {
				config := client.DefaultConfig("http://localhost:8080")
				config.UserAgent = "CustomAgent/2.0"
				return config
			},
			expectedHeaders: map[string]string{
				"Content-Type": "application/json",
				"User-Agent":   "CustomAgent/2.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			receivedHeaders := make(map[string]string)

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for name, values := range r.Header {
					if len(values) > 0 {
						receivedHeaders[name] = values[0]
					}
				}

				response := types.JSONRPCSuccessResponse{
					JSONRPC: "2.0",
					ID:      1,
					Result:  "success",
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					t.Errorf("Failed to encode response: %v", err)
				}
			}))
			defer server.Close()

			config := tt.setupConfig()
			config.BaseURL = server.URL
			c := client.NewClientWithConfig(config)

			ctx := context.Background()
			params := types.MessageSendParams{
				Message: types.Message{
					MessageID: "header-test",
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart("Testing headers"),
					},
				},
			}

			_, err := c.SendTask(ctx, params)
			require.NoError(t, err)

			for expectedName, expectedValue := range tt.expectedHeaders {
				actualValue, exists := receivedHeaders[expectedName]
				assert.True(t, exists, "Expected header %s not found", expectedName)
				assert.Equal(t, expectedValue, actualValue, "Header %s has wrong value", expectedName)
			}
		})
	}
}

func TestClient_InvalidParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Server should not be called for invalid params")
	}))
	defer server.Close()

	c := client.NewClient(server.URL)

	assert.NotNil(t, c)
}

func TestClient_LargeResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		largeText := strings.Repeat("This is a large response text. ", 10000)

		response := types.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: map[string]any{
				"id":        "large-task",
				"contextId": "large-ctx",
				"status": map[string]any{
					"state": "completed",
					"message": map[string]any{
						"kind":      "message",
						"messageId": "large-response",
						"role":      "assistant",
						"parts": []any{
							map[string]any{
								"kind": "text",
								"text": largeText,
							},
						},
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := client.NewClient(server.URL)
	ctx := context.Background()

	params := types.MessageSendParams{
		Message: types.Message{
			MessageID: "large-test",
			Role:      "user",
			Parts: []types.Part{
				types.CreateTextPart("Request large response"),
			},
		},
	}

	resp, err := c.SendTask(ctx, params)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
}

func TestClient_ConcurrentRequests(t *testing.T) {
	var requestCount int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt64(&requestCount, 1)

		response := types.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      count,
			Result:  fmt.Sprintf("response-%d", count),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("Failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	c := client.NewClient(server.URL)
	ctx := context.Background()

	const numGoroutines = 10
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			params := types.MessageSendParams{
				Message: types.Message{
					MessageID: fmt.Sprintf("concurrent-msg-%d", index),
					Role:      "user",
					Parts: []types.Part{
						types.CreateTextPart(fmt.Sprintf("Concurrent request %d", index)),
					},
				},
			}

			_, err := c.SendTask(ctx, params)
			results <- err
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent request %d failed", i)
	}

	finalCount := atomic.LoadInt64(&requestCount)
	assert.Equal(t, int64(numGoroutines), finalCount, "Expected %d requests, got %d", numGoroutines, finalCount)
}

func TestClient_GetAgentCard(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		expectedError string
		expectedCard  *types.AgentCard
	}{
		{
			name: "successful agent card retrieval",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "A2A-Go-Client/1.0", r.Header.Get("User-Agent"))

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					agentCard := types.AgentCard{
						Name:        "test-agent",
						Description: "A test agent for demonstration",
						Version:     "0.1.0",
						URL:         &[]string{"https://example.com"}[0],
						Capabilities: types.AgentCapabilities{
							Streaming:              &[]bool{true}[0],
							PushNotifications:      &[]bool{false}[0],
							StateTransitionHistory: &[]bool{true}[0],
						},
						DefaultInputModes:  []string{"text/plain"},
						DefaultOutputModes: []string{"text/plain"},
						Skills: []types.AgentSkill{
							{
								Name:        "text_processing",
								Description: "Process and analyze text",
							},
						},
					}

					_ = json.NewEncoder(w).Encode(agentCard)
				}))
			},
			expectedCard: &types.AgentCard{
				Name:        "test-agent",
				Description: "A test agent for demonstration",
				Version:     "0.1.0",
				URL:         &[]string{"https://example.com"}[0],
				Capabilities: types.AgentCapabilities{
					Streaming:              &[]bool{true}[0],
					PushNotifications:      &[]bool{false}[0],
					StateTransitionHistory: &[]bool{true}[0],
				},
				DefaultInputModes:  []string{"text/plain"},
				DefaultOutputModes: []string{"text/plain"},
				Skills: []types.AgentSkill{
					{
						Name:        "text_processing",
						Description: "Process and analyze text",
					},
				},
			},
		},
		{
			name: "server returns 404 not found",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)

					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte("Agent card not found"))
				}))
			},
			expectedError: "unexpected status code for agent card: 404, body: Agent card not found",
		},
		{
			name: "server returns 500 internal server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)

					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal server error"))
				}))
			},
			expectedError: "unexpected status code for agent card: 500, body: Internal server error",
		},
		{
			name: "server returns invalid JSON",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("invalid json response"))
				}))
			},
			expectedError: "failed to decode agent card response:",
		},
		{
			name: "minimal agent card response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					minimalCard := types.AgentCard{
						Name:    "minimal-agent",
						Version: "0.1.0",
					}

					_ = json.NewEncoder(w).Encode(minimalCard)
				}))
			},
			expectedCard: &types.AgentCard{
				Name:    "minimal-agent",
				Version: "0.1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			card, err := c.GetAgentCard(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, card)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, card)
				assert.Equal(t, tt.expectedCard, card)
			}
		})
	}
}

func TestClient_GetAgentCard_NetworkErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() client.A2AClient
		expectedError string
	}{
		{
			name: "connection refused",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:99999")
			},
			expectedError: "agent card request failed:",
		},
		{
			name: "invalid URL in base config",
			setupClient: func() client.A2AClient {
				config := client.DefaultConfig("://invalid-url")
				return client.NewClientWithConfig(config)
			},
			expectedError: "failed to create agent card request:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			ctx := context.Background()

			card, err := c.GetAgentCard(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Nil(t, card)
		})
	}
}

func TestClient_GetAgentCard_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		agentCard := types.AgentCard{
			Name:    "slow-agent",
			Version: "0.1.0",
		}
		_ = json.NewEncoder(w).Encode(agentCard)
	}))
	defer server.Close()

	c := client.NewClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	card, err := c.GetAgentCard(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.Nil(t, card)
}

func TestClient_GetAgentCard_WithCustomHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "A2A-Go-Client/1.0", r.Header.Get("User-Agent"))

		assert.Empty(t, r.Header.Get("Authorization"))
		assert.Empty(t, r.Header.Get("X-Custom-Header"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		agentCard := types.AgentCard{
			Name:    "test-agent",
			Version: "0.1.0",
		}
		_ = json.NewEncoder(w).Encode(agentCard)
	}))
	defer server.Close()

	config := client.DefaultConfig(server.URL)
	config.Headers = map[string]string{
		"Authorization":   "Bearer test-token",
		"X-Custom-Header": "custom-value",
	}
	c := client.NewClientWithConfig(config)

	ctx := context.Background()
	card, err := c.GetAgentCard(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, card)
	assert.Equal(t, "test-agent", card.Name)
	assert.Equal(t, "0.1.0", card.Version)
}

func TestClient_GetAgentCard_WithCustomUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/.well-known/agent-card.json", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "CustomAgent/2.0", r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		agentCard := types.AgentCard{
			Name:    "test-agent",
			Version: "0.1.0",
		}
		_ = json.NewEncoder(w).Encode(agentCard)
	}))
	defer server.Close()

	config := client.DefaultConfig(server.URL)
	config.UserAgent = "CustomAgent/2.0"
	c := client.NewClientWithConfig(config)

	ctx := context.Background()
	card, err := c.GetAgentCard(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, card)
	assert.Equal(t, "test-agent", card.Name)
	assert.Equal(t, "0.1.0", card.Version)
}

func TestClient_ListTasks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/a2a", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req types.JSONRPCRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)
		assert.Equal(t, "tasks/list", req.Method)

		mockTaskList := types.TaskList{
			Tasks: []types.Task{
				{
					ID:        "task-1",
					ContextID: "context-1",
					Status: types.TaskStatus{
						State: string(types.TaskStateCompleted),
					},
				},
				{
					ID:        "task-2",
					ContextID: "context-1",
					Status: types.TaskStatus{
						State: string(types.TaskStateWorking),
					},
				},
			},
			TotalSize: 2,
			PageSize:  50,
			
		}

		response := types.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  mockTaskList,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	a2aClient := client.NewClientWithLogger(server.URL, logger)

	t.Run("successful_tasks_list", func(t *testing.T) {
		params := types.TaskListParams{
			Limit: 50,
		}

		resp, err := a2aClient.ListTasks(context.Background(), params)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, "2.0", resp.JSONRPC)

		resultBytes, err := json.Marshal(resp.Result)
		assert.NoError(t, err)

		var taskList types.TaskList
		err = json.Unmarshal(resultBytes, &taskList)
		assert.NoError(t, err)

		assert.Equal(t, 2, len(taskList.Tasks))
		assert.Equal(t, 2, taskList.TotalSize)
		assert.Equal(t, 50, taskList.PageSize)
		assert.Equal(t, "task-1", taskList.Tasks[0].ID)
		assert.Equal(t, types.TaskStateCompleted, taskList.Tasks[0].Status.State)
	})

	t.Run("list_tasks_with_filtering", func(t *testing.T) {
		completedState := types.TaskStateCompleted
		params := types.TaskListParams{
			State:  &completedState,
			Limit:  10,
			
		}

		resp, err := a2aClient.ListTasks(context.Background(), params)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("list_tasks_with_context_filter", func(t *testing.T) {
		contextID := "some-context"
		params := types.TaskListParams{
			ContextID: &contextID,
			Limit:     25,
		}

		resp, err := a2aClient.ListTasks(context.Background(), params)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestClient_ListTasks_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req types.JSONRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Failed to decode request", http.StatusBadRequest)
			return
		}

		response := map[string]any{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error": map[string]any{
				"code":    -32603,
				"message": "Internal server error",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	logger := zap.NewNop()
	a2aClient := client.NewClientWithLogger(server.URL, logger)

	params := types.TaskListParams{}
	resp, err := a2aClient.ListTasks(context.Background(), params)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "Internal server error")
}

func TestClient_GetHealth(t *testing.T) {
	tests := []struct {
		name          string
		setupServer   func() *httptest.Server
		expectedError string
		expectedResp  *client.HealthResponse
	}{
		{
			name: "successful health check",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)
					assert.Equal(t, "application/json", r.Header.Get("Accept"))
					assert.Equal(t, "A2A-Go-Client/1.0", r.Header.Get("User-Agent"))

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					healthResp := client.HealthResponse{
						Status: "healthy",
					}

					_ = json.NewEncoder(w).Encode(healthResp)
				}))
			},
			expectedResp: &client.HealthResponse{
				Status: "healthy",
			},
		},
		{
			name: "server returns 404 not found",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte("Health endpoint not found"))
				}))
			},
			expectedError: "unexpected status code for health check: 404, body: Health endpoint not found",
		},
		{
			name: "server returns 500 internal server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte("Internal server error"))
				}))
			},
			expectedError: "unexpected status code for health check: 500, body: Internal server error",
		},
		{
			name: "server returns invalid JSON",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte("invalid json response"))
				}))
			},
			expectedError: "failed to decode health response:",
		},
		{
			name: "different health status",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					healthResp := client.HealthResponse{
						Status: "degraded",
					}

					_ = json.NewEncoder(w).Encode(healthResp)
				}))
			},
			expectedResp: &client.HealthResponse{
				Status: "degraded",
			},
		},
		{
			name: "empty status field",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					healthResp := client.HealthResponse{
						Status: "",
					}

					_ = json.NewEncoder(w).Encode(healthResp)
				}))
			},
			expectedError: "health response missing status field",
		},
		{
			name: "unknown status value",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "GET", r.Method)
					assert.Equal(t, "/health", r.URL.Path)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					healthResp := client.HealthResponse{
						Status: "unknown",
					}

					_ = json.NewEncoder(w).Encode(healthResp)
				}))
			},
			expectedResp: &client.HealthResponse{
				Status: "unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			resp, err := c.GetHealth(ctx)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, tt.expectedResp, resp)
			}
		})
	}
}

func TestClient_GetHealth_NetworkErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupClient   func() client.A2AClient
		expectedError string
	}{
		{
			name: "connection refused",
			setupClient: func() client.A2AClient {
				return client.NewClient("http://localhost:99999")
			},
			expectedError: "health request failed:",
		},
		{
			name: "invalid URL in base config",
			setupClient: func() client.A2AClient {
				config := client.DefaultConfig("://invalid-url")
				return client.NewClientWithConfig(config)
			},
			expectedError: "failed to create health request:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient()
			ctx := context.Background()

			resp, err := c.GetHealth(ctx)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
			assert.Nil(t, resp)
		})
	}
}

func TestClient_GetHealth_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		healthResp := client.HealthResponse{
			Status: "healthy",
		}
		_ = json.NewEncoder(w).Encode(healthResp)
	}))
	defer server.Close()

	c := client.NewClient(server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	resp, err := c.GetHealth(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
	assert.Nil(t, resp)
}

func TestClient_GetHealth_WithCustomUserAgent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "CustomAgent/2.0", r.Header.Get("User-Agent"))

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		healthResp := client.HealthResponse{
			Status: "healthy",
		}
		_ = json.NewEncoder(w).Encode(healthResp)
	}))
	defer server.Close()

	config := client.DefaultConfig(server.URL)
	config.UserAgent = "CustomAgent/2.0"
	c := client.NewClientWithConfig(config)

	ctx := context.Background()
	resp, err := c.GetHealth(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "healthy", resp.Status)
}

func TestClient_GetHealth_WithConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected string
	}{
		{
			name:     "healthy status constant",
			status:   types.HealthStatusHealthy,
			expected: types.HealthStatusHealthy,
		},
		{
			name:     "degraded status constant",
			status:   types.HealthStatusDegraded,
			expected: types.HealthStatusDegraded,
		},
		{
			name:     "unhealthy status constant",
			status:   types.HealthStatusUnhealthy,
			expected: types.HealthStatusUnhealthy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "GET", r.Method)
				assert.Equal(t, "/health", r.URL.Path)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)

				healthResp := client.HealthResponse{
					Status: tt.status,
				}

				_ = json.NewEncoder(w).Encode(healthResp)
			}))
			defer server.Close()

			c := client.NewClient(server.URL)
			ctx := context.Background()

			resp, err := c.GetHealth(ctx)

			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.Equal(t, tt.expected, resp.Status)
		})
	}
}

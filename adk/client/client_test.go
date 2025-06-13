package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/inference-gateway/a2a/adk"
	"github.com/inference-gateway/a2a/adk/client"
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

func TestClient_SendMessage(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         adk.MessageSendParams
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

					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "message/send", req.Method)

					response := adk.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]interface{}{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]interface{}{
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-1",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Hello, world!",
						},
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
					response := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]interface{}{
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-error",
					Role:      "user",
					Parts:     []adk.Part{},
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-500",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "This should fail",
						},
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "test-msg-invalid",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Invalid response test",
						},
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

			resp, err := c.SendMessage(ctx, tt.params)

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
		params         adk.TaskQueryParams
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

					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "tasks/get", req.Method)

					response := adk.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]interface{}{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]interface{}{
								"state": "completed",
								"message": map[string]interface{}{
									"kind":      "message",
									"messageId": "response-msg",
									"role":      "assistant",
									"parts": []interface{}{
										map[string]interface{}{
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
			params: adk.TaskQueryParams{
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
					response := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]interface{}{
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
			params: adk.TaskQueryParams{
				ID: "nonexistent-task",
			},
			expectError:   true,
			errorContains: "A2A error: task not found",
		},
		{
			name: "minimal task query params",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)

					response := adk.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]interface{}{
							"id":        "task-minimal",
							"contextId": "ctx-minimal",
							"status": map[string]interface{}{
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
			params: adk.TaskQueryParams{
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
		params         adk.TaskIdParams
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

					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "tasks/cancel", req.Method)

					response := adk.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]interface{}{
							"id":        "task-123",
							"contextId": "ctx-456",
							"status": map[string]interface{}{
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
			params: adk.TaskIdParams{
				ID: "task-123",
			},
			expectError:    false,
			expectedResult: true,
		},
		{
			name: "task not cancelable error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					response := map[string]interface{}{
						"jsonrpc": "2.0",
						"id":      1,
						"error": map[string]interface{}{
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
			params: adk.TaskIdParams{
				ID: "completed-task",
			},
			expectError:   true,
			errorContains: "A2A error: task not cancelable",
		},
		{
			name: "task with metadata",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)

					response := adk.JSONRPCSuccessResponse{
						JSONRPC: "2.0",
						ID:      req.ID,
						Result: map[string]interface{}{
							"id":        "task-with-metadata",
							"contextId": "ctx-metadata",
							"status": map[string]interface{}{
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
			params: adk.TaskIdParams{
				ID: "task-with-metadata",
				Metadata: map[string]interface{}{
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

func TestClient_SendMessageStreaming(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		params         adk.MessageSendParams
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

					var req adk.JSONRPCRequest
					err := json.NewDecoder(r.Body).Decode(&req)
					assert.NoError(t, err)
					assert.Equal(t, "2.0", req.JSONRPC)
					assert.Equal(t, "message/stream", req.Method)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)

					// Send multiple streaming events
					events := []adk.JSONRPCSuccessResponse{
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

					encoder := json.NewEncoder(w)
					for _, event := range events {
						if err := encoder.Encode(event); err != nil {
							t.Errorf("Failed to encode event: %v", err)
							return
						}
						if flusher, ok := w.(http.Flusher); ok {
							flusher.Flush()
						}
					}
				}))
			},
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "stream-msg-1",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Stream this message",
						},
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
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "stream-error",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "This should fail",
						},
					},
				},
			},
			expectError:   true,
			errorContains: "unexpected status code: 400",
		},
		{name: "invalid json in stream",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					if _, err := w.Write([]byte("invalid json stream")); err != nil {
						t.Errorf("Failed to write response: %v", err)
					}
				}))
			},
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "stream-invalid",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Invalid stream test",
						},
					},
				},
			},
			expectError:   true,
			errorContains: "failed to decode event",
		},
		{
			name: "empty stream response",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
				}))
			},
			params: adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "stream-empty",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Empty stream test",
						},
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

			eventChan := make(chan interface{}, 10)

			err := c.SendMessageStreaming(ctx, tt.params, eventChan)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)

				eventCount := 0
				timeout := time.After(1 * time.Second)

			eventLoop:
				for {
					select {
					case <-eventChan:
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

					response := adk.JSONRPCSuccessResponse{
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

					response := adk.JSONRPCSuccessResponse{
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

			params := adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "retry-test",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Testing retry mechanism",
						},
					},
				},
			}

			_, err := c.SendMessage(ctx, params)

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
					time.Sleep(200 * time.Millisecond)

					response := adk.JSONRPCSuccessResponse{
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
				ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
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
					response := adk.JSONRPCSuccessResponse{
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

			params := adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "context-test",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Testing context cancellation",
						},
					},
				},
			}

			_, err := c.SendMessage(ctx, params)

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

				response := adk.JSONRPCSuccessResponse{
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
			params := adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: "header-test",
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": "Testing headers",
						},
					},
				},
			}

			_, err := c.SendMessage(ctx, params)
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

		response := adk.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      1,
			Result: map[string]interface{}{
				"id":        "large-task",
				"contextId": "large-ctx",
				"status": map[string]interface{}{
					"state": "completed",
					"message": map[string]interface{}{
						"kind":      "message",
						"messageId": "large-response",
						"role":      "assistant",
						"parts": []interface{}{
							map[string]interface{}{
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

	params := adk.MessageSendParams{
		Message: adk.Message{
			Kind:      "message",
			MessageID: "large-test",
			Role:      "user",
			Parts: []adk.Part{
				map[string]interface{}{
					"kind": "text",
					"text": "Request large response",
				},
			},
		},
	}

	resp, err := c.SendMessage(ctx, params)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "2.0", resp.JSONRPC)
}

func TestClient_ConcurrentRequests(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		response := adk.JSONRPCSuccessResponse{
			JSONRPC: "2.0",
			ID:      requestCount,
			Result:  fmt.Sprintf("response-%d", requestCount),
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
			params := adk.MessageSendParams{
				Message: adk.Message{
					Kind:      "message",
					MessageID: fmt.Sprintf("concurrent-msg-%d", index),
					Role:      "user",
					Parts: []adk.Part{
						map[string]interface{}{
							"kind": "text",
							"text": fmt.Sprintf("Concurrent request %d", index),
						},
					},
				},
			}

			_, err := c.SendMessage(ctx, params)
			results <- err
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent request %d failed", i)
	}

	assert.Equal(t, numGoroutines, requestCount, "Expected %d requests, got %d", numGoroutines, requestCount)
}

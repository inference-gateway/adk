package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/inference-gateway/adk/types"
	"go.uber.org/zap"
)

// A2AClient defines the interface for an A2A protocol client
type A2AClient interface {
	// Agent discovery
	GetAgentCard(ctx context.Context) (*types.AgentCard, error)
	GetHealth(ctx context.Context) (*HealthResponse, error)

	// Task operations
	SendTask(ctx context.Context, params types.MessageSendParams) (*types.JSONRPCSuccessResponse, error)
	SendTaskStreaming(ctx context.Context, params types.MessageSendParams, eventChan chan<- any) error
	GetTask(ctx context.Context, params types.TaskQueryParams) (*types.JSONRPCSuccessResponse, error)
	ListTasks(ctx context.Context, params types.TaskListParams) (*types.JSONRPCSuccessResponse, error)
	CancelTask(ctx context.Context, params types.TaskIdParams) (*types.JSONRPCSuccessResponse, error)

	// Configuration
	SetTimeout(timeout time.Duration)
	SetHTTPClient(client *http.Client)
	GetBaseURL() string

	// Logger configuration
	SetLogger(logger *zap.Logger)
	GetLogger() *zap.Logger
}

var _ A2AClient = (*Client)(nil)

// HealthResponse represents the response from the health endpoint
type HealthResponse struct {
	Status string `json:"status"`
}

// Config holds configuration options for the A2A client
type Config struct {
	BaseURL    string
	Timeout    time.Duration
	HTTPClient *http.Client
	UserAgent  string
	Headers    map[string]string
	MaxRetries int
	RetryDelay time.Duration
	Logger     *zap.Logger
}

// DefaultConfig returns a default configuration
func DefaultConfig(baseURL string) *Config {
	return &Config{
		BaseURL:    baseURL,
		Timeout:    30 * time.Second,
		UserAgent:  "A2A-Go-Client/1.0",
		Headers:    make(map[string]string),
		MaxRetries: 3,
		RetryDelay: 1 * time.Second,
		Logger:     zap.NewNop(),
	}
}

// Client represents an A2A protocol client
type Client struct {
	config     *Config
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new A2A client with default configuration
func NewClient(baseURL string) A2AClient {
	config := DefaultConfig(baseURL)
	return NewClientWithConfig(config)
}

// NewClientWithConfig creates a new A2A client with custom configuration
func NewClientWithConfig(config *Config) A2AClient {
	httpClient := config.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: config.Timeout,
		}
	}

	logger := config.Logger
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Client{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
	}
}

// getA2AEndpointURL constructs the A2A endpoint URL by appending /a2a to the base URL
func (c *Client) getA2AEndpointURL() string {
	baseURL := c.config.BaseURL

	if strings.HasSuffix(baseURL, "/a2a") {
		return baseURL
	}

	if strings.HasSuffix(baseURL, "/") {
		return baseURL + "a2a"
	}
	return baseURL + "/a2a"
}

// SendTask sends a task to the agent (primary interface following official A2A pattern)
func (c *Client) SendTask(ctx context.Context, params types.MessageSendParams) (*types.JSONRPCSuccessResponse, error) {
	c.logger.Debug("sending task",
		zap.String("method", "message/send"),
		zap.String("message_id", params.Message.MessageID),
		zap.String("role", params.Message.Role))

	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params:  make(map[string]any),
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.Error("failed to marshal params", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	var paramsMap map[string]any
	if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
		c.logger.Error("failed to unmarshal params to map", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal params to map: %w", err)
	}
	req.Params = paramsMap

	var resp types.JSONRPCSuccessResponse
	if err := c.doRequestWithContext(ctx, req, &resp); err != nil {
		c.logger.Error("failed to send task", zap.Error(err), zap.String("message_id", params.Message.MessageID))
		return nil, err
	}

	c.logger.Debug("task sent successfully", zap.String("message_id", params.Message.MessageID))
	return &resp, nil
}

// SendTaskStreaming sends a task and streams the response (primary interface following official A2A pattern)
func (c *Client) SendTaskStreaming(ctx context.Context, params types.MessageSendParams, eventChan chan<- any) error {
	c.logger.Debug("starting task streaming",
		zap.String("method", "message/stream"),
		zap.String("message_id", params.Message.MessageID),
		zap.String("role", params.Message.Role))

	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "message/stream",
		Params:  make(map[string]any),
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.Error("failed to marshal params", zap.Error(err))
		return fmt.Errorf("failed to marshal params: %w", err)
	}

	var paramsMap map[string]any
	if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
		c.logger.Error("failed to unmarshal params to map", zap.Error(err))
		return fmt.Errorf("failed to unmarshal params to map: %w", err)
	}
	req.Params = paramsMap

	body, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("failed to marshal request", zap.Error(err))
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.getA2AEndpointURL(), bytes.NewBuffer(body))
	if err != nil {
		c.logger.Error("failed to create request", zap.Error(err))
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)
	httpReq.Header.Set("Accept", "text/event-stream")

	c.logger.Debug("sending streaming request", zap.String("url", c.getA2AEndpointURL()))

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("failed to send request", zap.Error(err))
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	if httpResp.StatusCode != http.StatusOK {
		c.logger.Error("unexpected status code", zap.Int("status_code", httpResp.StatusCode))
		return fmt.Errorf("unexpected status code: %d", httpResp.StatusCode)
	}

	c.logger.Debug("streaming response started successfully")

	scanner := bufio.NewScanner(httpResp.Body)
	eventCount := 0
	for {
		select {
		case <-ctx.Done():
			c.logger.Debug("streaming context cancelled", zap.Int("events_received", eventCount))
			return ctx.Err()
		default:
			if !scanner.Scan() {
				if err := scanner.Err(); err != nil {
					c.logger.Error("failed to scan response", zap.Error(err), zap.Int("events_received", eventCount))
					return fmt.Errorf("failed to scan response: %w", err)
				}
				c.logger.Debug("streaming completed", zap.Int("events_received", eventCount))
				return nil
			}

			line := scanner.Text()
			c.logger.Debug("received line", zap.String("line", line))

			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}

			if strings.TrimSpace(line) == "data: [DONE]" {
				c.logger.Debug("received stream termination signal", zap.Int("events_received", eventCount))
				return nil
			}

			jsonData := strings.TrimPrefix(line, "data: ")

			var event types.JSONRPCSuccessResponse
			if err := json.Unmarshal([]byte(jsonData), &event); err != nil {
				c.logger.Error("failed to decode event", zap.Error(err), zap.Int("events_received", eventCount), zap.String("json_data", jsonData))
				return fmt.Errorf("failed to decode event: %w", err)
			}

			eventCount++
			c.logger.Debug("received streaming event", zap.Int("event_number", eventCount))

			select {
			case eventChan <- event.Result:
			case <-ctx.Done():
				c.logger.Debug("streaming context cancelled while sending event", zap.Int("events_received", eventCount))
				return ctx.Err()
			}
		}
	}
}

// GetTaskWithContext retrieves the status of a task with context support
func (c *Client) GetTask(ctx context.Context, params types.TaskQueryParams) (*types.JSONRPCSuccessResponse, error) {
	c.logger.Debug("retrieving task", zap.String("method", "tasks/get"), zap.String("task_id", params.ID))

	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tasks/get",
		Params:  make(map[string]any),
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.Error("failed to marshal params", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	var paramsMap map[string]any
	if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
		c.logger.Error("failed to unmarshal params to map", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal params to map: %w", err)
	}
	req.Params = paramsMap

	var resp types.JSONRPCSuccessResponse
	if err := c.doRequestWithContext(ctx, req, &resp); err != nil {
		c.logger.Error("failed to retrieve task", zap.Error(err), zap.String("task_id", params.ID))
		return nil, err
	}

	c.logger.Debug("task retrieved successfully", zap.String("task_id", params.ID))
	return &resp, nil
}

// CancelTaskWithContext cancels a task with context support
func (c *Client) CancelTask(ctx context.Context, params types.TaskIdParams) (*types.JSONRPCSuccessResponse, error) {
	c.logger.Debug("cancelling task", zap.String("method", "tasks/cancel"), zap.String("task_id", params.ID))

	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tasks/cancel",
		Params:  make(map[string]any),
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.Error("failed to marshal params", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	var paramsMap map[string]any
	if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
		c.logger.Error("failed to unmarshal params to map", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal params to map: %w", err)
	}
	req.Params = paramsMap

	var resp types.JSONRPCSuccessResponse
	if err := c.doRequestWithContext(ctx, req, &resp); err != nil {
		c.logger.Error("failed to cancel task", zap.Error(err), zap.String("task_id", params.ID))
		return nil, err
	}

	c.logger.Debug("task cancelled successfully", zap.String("task_id", params.ID))
	return &resp, nil
}

// ListTasks retrieves a list of tasks from the agent
func (c *Client) ListTasks(ctx context.Context, params types.TaskListParams) (*types.JSONRPCSuccessResponse, error) {
	c.logger.Debug("listing tasks", zap.String("method", "tasks/list"))

	req := types.JSONRPCRequest{
		JSONRPC: "2.0",
		Method:  "tasks/list",
		Params:  make(map[string]any),
	}

	paramsBytes, err := json.Marshal(params)
	if err != nil {
		c.logger.Error("failed to marshal params", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	var paramsMap map[string]any
	if err := json.Unmarshal(paramsBytes, &paramsMap); err != nil {
		c.logger.Error("failed to unmarshal params to map", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal params to map: %w", err)
	}
	req.Params = paramsMap

	var resp types.JSONRPCSuccessResponse
	if err := c.doRequestWithContext(ctx, req, &resp); err != nil {
		c.logger.Error("failed to list tasks", zap.Error(err))
		return nil, err
	}

	c.logger.Debug("tasks listed successfully")
	return &resp, nil
}

// GetAgentCard retrieves the agent card information via HTTP GET to .well-known/agent.json
func (c *Client) GetAgentCard(ctx context.Context) (*types.AgentCard, error) {
	c.logger.Debug("retrieving agent card", zap.String("endpoint", "/.well-known/agent.json"))

	agentCardURL := c.config.BaseURL + "/.well-known/agent.json"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", agentCardURL, nil)
	if err != nil {
		c.logger.Error("failed to create agent card request", zap.Error(err))
		return nil, fmt.Errorf("failed to create agent card request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.config.UserAgent)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("agent card request failed", zap.Error(err))
		return nil, fmt.Errorf("agent card request failed: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close agent card response body", zap.Error(closeErr))
		}
	}()

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		c.logger.Error("unexpected status code for agent card",
			zap.Int("status_code", httpResp.StatusCode),
			zap.String("response_body", string(bodyBytes)))
		return nil, fmt.Errorf("unexpected status code for agent card: %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var agentCard types.AgentCard
	if err := json.NewDecoder(httpResp.Body).Decode(&agentCard); err != nil {
		c.logger.Error("failed to decode agent card response", zap.Error(err))
		return nil, fmt.Errorf("failed to decode agent card response: %w", err)
	}

	c.logger.Debug("agent card retrieved successfully",
		zap.String("name", agentCard.Name),
		zap.String("version", agentCard.Version))
	return &agentCard, nil
}

// GetHealth retrieves the health status of the agent via HTTP GET to /health
func (c *Client) GetHealth(ctx context.Context) (*HealthResponse, error) {
	c.logger.Debug("retrieving agent health", zap.String("endpoint", "/health"))

	healthURL := c.config.BaseURL + "/health"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		c.logger.Error("failed to create health request", zap.Error(err))
		return nil, fmt.Errorf("failed to create health request: %w", err)
	}

	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", c.config.UserAgent)

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		c.logger.Error("health request failed", zap.Error(err))
		return nil, fmt.Errorf("health request failed: %w", err)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close health response body", zap.Error(closeErr))
		}
	}()

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		c.logger.Error("unexpected status code for health check",
			zap.Int("status_code", httpResp.StatusCode),
			zap.String("response_body", string(bodyBytes)))
		return nil, fmt.Errorf("unexpected status code for health check: %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var healthResp HealthResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&healthResp); err != nil {
		c.logger.Error("failed to decode health response", zap.Error(err))
		return nil, fmt.Errorf("failed to decode health response: %w", err)
	}

	if healthResp.Status == "" {
		c.logger.Error("health response missing status field")
		return nil, fmt.Errorf("health response missing status field")
	}

	validStatuses := []string{types.HealthStatusHealthy, types.HealthStatusDegraded, types.HealthStatusUnhealthy}
	isValidStatus := false
	for _, validStatus := range validStatuses {
		if healthResp.Status == validStatus {
			isValidStatus = true
			break
		}
	}
	if !isValidStatus {
		c.logger.Warn("health response contains unknown status", zap.String("status", healthResp.Status))
	}

	c.logger.Debug("health check completed successfully", zap.String("status", healthResp.Status))
	return &healthResp, nil
}

// doRequestWithContext performs the HTTP request with context support and handles the response
func (c *Client) doRequestWithContext(ctx context.Context, req types.JSONRPCRequest, resp *types.JSONRPCSuccessResponse) error {
	c.logger.Debug("preparing request", zap.String("method", req.Method), zap.String("base_url", c.config.BaseURL))

	body, err := json.Marshal(req)
	if err != nil {
		c.logger.Error("failed to marshal request", zap.Error(err))
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.getA2AEndpointURL(), bytes.NewBuffer(body))
	if err != nil {
		c.logger.Error("failed to create request", zap.Error(err))
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	var httpResp *http.Response
	var lastErr error

	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			c.logger.Debug("retrying request",
				zap.String("method", req.Method),
				zap.Int("attempt", attempt+1),
				zap.Int("max_retries", c.config.MaxRetries+1))
		}

		httpResp, err = c.httpClient.Do(httpReq)
		if err == nil {
			c.logger.Debug("request successful",
				zap.String("method", req.Method),
				zap.Int("attempt", attempt+1),
				zap.Int("status_code", httpResp.StatusCode))
			break
		}
		lastErr = err
		c.logger.Warn("request failed",
			zap.String("method", req.Method),
			zap.Int("attempt", attempt+1),
			zap.Error(err))

		if attempt < c.config.MaxRetries {
			delay := c.config.RetryDelay * time.Duration(attempt+1)
			c.logger.Debug("waiting before retry",
				zap.Duration("delay", delay),
				zap.Int("attempt", attempt+1))
			select {
			case <-ctx.Done():
				c.logger.Debug("request context cancelled during retry delay")
				return ctx.Err()
			case <-time.After(delay):
				// Continue to next attempt
			}
		}
	}

	if httpResp == nil {
		c.logger.Error("all retry attempts exhausted",
			zap.String("method", req.Method),
			zap.Int("attempts", c.config.MaxRetries+1),
			zap.Error(lastErr))
		return fmt.Errorf("failed to send request after %d attempts: %w", c.config.MaxRetries+1, lastErr)
	}
	defer func() {
		if closeErr := httpResp.Body.Close(); closeErr != nil {
			c.logger.Warn("failed to close response body", zap.Error(closeErr))
		}
	}()

	if httpResp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(httpResp.Body)
		c.logger.Error("unexpected status code",
			zap.String("method", req.Method),
			zap.Int("status_code", httpResp.StatusCode),
			zap.String("response_body", string(bodyBytes)))
		return fmt.Errorf("unexpected status code: %d, body: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var rawResp struct {
		JSONRPC string              `json:"jsonrpc"`
		ID      any                 `json:"id,omitempty"`
		Result  json.RawMessage     `json:"result,omitempty"`
		Error   *types.JSONRPCError `json:"error,omitempty"`
	}

	if err := json.NewDecoder(httpResp.Body).Decode(&rawResp); err != nil {
		c.logger.Error("failed to decode response", zap.Error(err))
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if rawResp.Error != nil {
		c.logger.Error("received A2A error response",
			zap.String("error_message", rawResp.Error.Message),
			zap.Int("error_code", rawResp.Error.Code))
		return fmt.Errorf("A2A error: %s (code: %d)", rawResp.Error.Message, rawResp.Error.Code)
	}

	resp.JSONRPC = rawResp.JSONRPC
	resp.ID = rawResp.ID

	if len(rawResp.Result) > 0 {
		resp.Result = rawResp.Result
	}

	c.logger.Debug("request completed successfully", zap.String("method", req.Method))
	return nil
}

// setHeaders sets the common headers for HTTP requests
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.config.UserAgent)

	for key, value := range c.config.Headers {
		req.Header.Set(key, value)
	}
}

// SetHTTPClient allows customizing the HTTP client
func (c *Client) SetHTTPClient(client *http.Client) {
	c.httpClient = client
	c.config.HTTPClient = client
}

// SetTimeout sets the timeout for HTTP requests
func (c *Client) SetTimeout(timeout time.Duration) {
	c.config.Timeout = timeout
	if c.httpClient != nil {
		c.httpClient.Timeout = timeout
	}
}

// GetBaseURL returns the base URL of the client
func (c *Client) GetBaseURL() string {
	return c.config.BaseURL
}

// SetHeader sets a custom header for all requests
func (c *Client) SetHeader(key, value string) {
	if c.config.Headers == nil {
		c.config.Headers = make(map[string]string)
	}
	c.config.Headers[key] = value
}

// RemoveHeader removes a custom header
func (c *Client) RemoveHeader(key string) {
	if c.config.Headers != nil {
		delete(c.config.Headers, key)
	}
}

// GetConfig returns a copy of the client configuration
func (c *Client) GetConfig() Config {
	config := *c.config
	if c.config.Headers != nil {
		config.Headers = make(map[string]string)
		for k, v := range c.config.Headers {
			config.Headers[k] = v
		}
	}
	return config
}

// SetMaxRetries sets the maximum number of retry attempts
func (c *Client) SetMaxRetries(maxRetries int) {
	c.config.MaxRetries = maxRetries
}

// SetRetryDelay sets the delay between retry attempts
func (c *Client) SetRetryDelay(delay time.Duration) {
	c.config.RetryDelay = delay
}

// SetLogger sets the logger for the client
func (c *Client) SetLogger(logger *zap.Logger) {
	if logger == nil {
		logger = zap.NewNop()
	}
	c.logger = logger
	c.config.Logger = logger
}

// GetLogger returns the current logger
func (c *Client) GetLogger() *zap.Logger {
	return c.logger
}

// NewClientWithLogger creates a new A2A client with a custom logger
func NewClientWithLogger(baseURL string, logger *zap.Logger) A2AClient {
	config := DefaultConfig(baseURL)
	config.Logger = logger
	return NewClientWithConfig(config)
}

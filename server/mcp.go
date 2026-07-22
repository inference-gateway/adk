package server

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	config "github.com/inference-gateway/adk/server/config"
	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
	zap "go.uber.org/zap"
)

// mcpToolEntry is the metadata the manager keeps for a single discovered MCP tool.
type mcpToolEntry struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema any    `json:"input_schema,omitempty"`
}

// mcpToolCaller is the minimal MCP client surface the manager needs. The real
// implementation wraps metoro-io/mcp-golang over streamable HTTP; tests provide
// an in-process fake so no network or MCP server is required.
type mcpToolCaller interface {
	// Initialize performs the MCP handshake with the server.
	Initialize(ctx context.Context) error
	// ListTools returns the server's tool metadata.
	ListTools(ctx context.Context) ([]mcpToolEntry, error)
	// CallTool invokes a tool by name and returns its text content.
	CallTool(ctx context.Context, name string, arguments map[string]any) (string, error)
}

// mcpConn tracks a single MCP server connection and its last-known tool catalog.
type mcpConn struct {
	url    string
	client mcpToolCaller
	tools  []mcpToolEntry
}

// MCPClientManager connects to one or more MCP servers, keeps their tool catalog
// fresh with retry + polling backoff, and exposes the catalog to an agent through
// two selector tools (mcp_list_tools, mcp_call_tool). This keeps only tool
// metadata - never the full per-tool schemas - in the LLM context window.
//
// It only makes sense alongside a configured LLM/agent: the selector tools are
// registered into the agent's toolbox via RegisterTools.
type MCPClientManager struct {
	cfg    config.MCPConfig
	logger *zap.Logger

	// newClient builds a client for a server URL. Overridable in tests.
	newClient func(url string) mcpToolCaller

	mu     sync.RWMutex
	conns  map[string]*mcpConn
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewMCPClientManager creates a manager for the configured MCP servers. It does
// not connect until Start is called.
func NewMCPClientManager(cfg config.MCPConfig, logger *zap.Logger) (*MCPClientManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}
	if len(cfg.Servers) == 0 {
		return nil, fmt.Errorf("mcp: no servers configured")
	}

	m := &MCPClientManager{
		cfg:    cfg,
		logger: logger,
		conns:  make(map[string]*mcpConn, len(cfg.Servers)),
	}
	m.newClient = func(url string) mcpToolCaller {
		return newHTTPMCPClient(url, cfg.Endpoint)
	}
	for _, url := range cfg.Servers {
		url = strings.TrimSpace(url)
		if url == "" {
			continue
		}
		m.conns[url] = &mcpConn{url: url}
	}
	return m, nil
}

// Start connects to every configured server in the background and keeps their
// tool catalogs refreshed. It returns immediately so server startup is never
// blocked by an MCP server that is not up yet (common in cloud-native rollouts).
func (m *MCPClientManager) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	m.mu.RLock()
	conns := make([]*mcpConn, 0, len(m.conns))
	for _, c := range m.conns {
		conns = append(conns, c)
	}
	m.mu.RUnlock()

	for _, c := range conns {
		c.client = m.newClient(c.url)
		m.wg.Add(1)
		go func(conn *mcpConn) {
			defer m.wg.Done()
			m.run(runCtx, conn)
		}(c)
	}
}

// Close stops all background loops and waits for them to exit.
func (m *MCPClientManager) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	return nil
}

// run connects a single server with retry backoff, then refreshes its catalog on
// the configured interval. A failed refresh backs off (doubling up to
// RetryMaxInterval) and resets once the server responds again.
func (m *MCPClientManager) run(ctx context.Context, conn *mcpConn) {
	backoff := m.cfg.RetryInterval
	connected := false

	for attempt := 1; ; attempt++ {
		if !connected {
			if err := m.initialize(ctx, conn); err != nil {
				m.logger.Warn("mcp: connect failed, backing off",
					zap.String("server", conn.url), zap.Int("attempt", attempt), zap.Error(err))
				if m.cfg.MaxRetries > 0 && attempt >= m.cfg.MaxRetries {
					m.logger.Error("mcp: giving up on server", zap.String("server", conn.url))
					return
				}
				if !sleepCtx(ctx, backoff) {
					return
				}
				backoff = nextBackoff(backoff, m.cfg.RetryMaxInterval)
				continue
			}
			connected = true
			backoff = m.cfg.RetryInterval
			m.logger.Info("mcp: connected", zap.String("server", conn.url))
		}

		if err := m.refresh(ctx, conn); err != nil {
			m.logger.Warn("mcp: refresh failed, backing off",
				zap.String("server", conn.url), zap.Error(err))
			if !sleepCtx(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff, m.cfg.RetryMaxInterval)
			continue
		}

		backoff = m.cfg.RetryInterval
		attempt = 0
		if !sleepCtx(ctx, m.cfg.RefreshInterval) {
			return
		}
	}
}

// initialize performs the MCP handshake bounded by DialTimeout.
func (m *MCPClientManager) initialize(ctx context.Context, conn *mcpConn) error {
	cctx, cancel := context.WithTimeout(ctx, m.cfg.DialTimeout)
	defer cancel()
	return conn.client.Initialize(cctx)
}

// refresh pulls the current tool list from a server and updates its catalog.
func (m *MCPClientManager) refresh(ctx context.Context, conn *mcpConn) error {
	cctx, cancel := context.WithTimeout(ctx, m.cfg.DialTimeout)
	defer cancel()

	tools, err := conn.client.ListTools(cctx)
	if err != nil {
		return err
	}

	m.mu.Lock()
	conn.tools = tools
	m.mu.Unlock()

	m.logger.Debug("mcp: refreshed tool catalog",
		zap.String("server", conn.url), zap.Int("tools", len(tools)))
	return nil
}

// snapshot returns a flattened copy of every server's current tool catalog.
func (m *MCPClientManager) snapshot() []mcpToolEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]mcpToolEntry, 0)
	for _, conn := range m.conns {
		out = append(out, conn.tools...)
	}
	return out
}

// resolveServer finds the server URL that offers the named tool. An explicit
// preferred URL wins; otherwise the first server advertising the tool is used.
func (m *MCPClientManager) resolveServer(name, preferred string) (*mcpConn, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if preferred != "" {
		conn, ok := m.conns[preferred]
		return conn, ok
	}
	// ponytail: first-match on tool name; pass "server" to disambiguate when the
	// same tool name exists on multiple servers.
	for _, conn := range m.conns {
		for _, t := range conn.tools {
			if t.Name == name {
				return conn, true
			}
		}
	}
	return nil, false
}

// RegisterTools adds the two MCP selector tools to the toolbox. These are the
// only MCP-related tools ever exposed to the LLM, regardless of how many tools
// the connected MCP servers offer.
func (m *MCPClientManager) RegisterTools(tb *DefaultToolBox) {
	tb.AddTool(NewBasicTool(
		"mcp_list_tools",
		"List tools available on connected MCP servers. Call this first to discover what MCP tools exist, then invoke one with mcp_call_tool. Returns each tool's server, name, description and input schema.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"search": map[string]any{
					"type":        "string",
					"description": "Optional case-insensitive substring to filter tools by name or description.",
				},
			},
		},
		m.executeListTools,
	))

	tb.AddTool(NewBasicTool(
		"mcp_call_tool",
		"Invoke a tool discovered via mcp_list_tools on its MCP server. Provide the tool name and an arguments object matching that tool's input schema.",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Name of the MCP tool to invoke (from mcp_list_tools).",
				},
				"server": map[string]any{
					"type":        "string",
					"description": "Optional MCP server URL to disambiguate when the same tool name exists on multiple servers.",
				},
				"arguments": map[string]any{
					"type":        "object",
					"description": "Arguments object matching the tool's input schema.",
				},
			},
			"required": []string{"name"},
		},
		m.executeCallTool,
	))
}

// executeListTools implements the mcp_list_tools selector tool.
func (m *MCPClientManager) executeListTools(_ context.Context, args map[string]any) (string, error) {
	search, _ := args["search"].(string)
	search = strings.ToLower(strings.TrimSpace(search))

	tools := m.snapshot()
	if search != "" {
		filtered := make([]mcpToolEntry, 0, len(tools))
		for _, t := range tools {
			if strings.Contains(strings.ToLower(t.Name), search) ||
				strings.Contains(strings.ToLower(t.Description), search) {
				filtered = append(filtered, t)
			}
		}
		tools = filtered
	}

	return JSONTool(map[string]any{
		"count": len(tools),
		"tools": tools,
	})
}

// executeCallTool implements the mcp_call_tool selector tool.
func (m *MCPClientManager) executeCallTool(ctx context.Context, args map[string]any) (string, error) {
	name, _ := args["name"].(string)
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("mcp_call_tool: 'name' is required")
	}
	preferred, _ := args["server"].(string)

	arguments, _ := args["arguments"].(map[string]any)
	if arguments == nil {
		arguments = map[string]any{}
	}

	conn, ok := m.resolveServer(name, preferred)
	if !ok {
		return "", fmt.Errorf("mcp_call_tool: no MCP server offers tool %q (try mcp_list_tools)", name)
	}

	cctx, cancel := context.WithTimeout(ctx, m.cfg.CallTimeout)
	defer cancel()

	result, err := conn.client.CallTool(cctx, name, arguments)
	if err != nil {
		return "", fmt.Errorf("mcp_call_tool: %q on %s failed: %w", name, conn.url, err)
	}
	return result, nil
}

// nextBackoff doubles the current interval, capping it at maxInterval.
func nextBackoff(current, maxInterval time.Duration) time.Duration {
	next := current * 2
	if maxInterval > 0 && next > maxInterval {
		return maxInterval
	}
	return next
}

// sleepCtx waits for d or ctx cancellation, returning false if the context was
// cancelled (i.e. the caller should stop).
func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

// httpMCPClient adapts metoro-io/mcp-golang's streamable HTTP client to
// mcpToolCaller.
type httpMCPClient struct {
	url   string
	inner *mcp.Client
}

// newHTTPMCPClient builds an MCP client for a server base URL and endpoint path.
func newHTTPMCPClient(url, endpoint string) *httpMCPClient {
	transport := mcphttp.NewHTTPClientTransport(endpoint)
	transport.WithBaseURL(url)
	return &httpMCPClient{
		url:   url,
		inner: mcp.NewClient(transport),
	}
}

func (c *httpMCPClient) Initialize(ctx context.Context) error {
	_, err := c.inner.Initialize(ctx)
	return err
}

func (c *httpMCPClient) ListTools(ctx context.Context) ([]mcpToolEntry, error) {
	// ponytail: single page; add cursor paging if a server ever returns NextCursor.
	resp, err := c.inner.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}
	entries := make([]mcpToolEntry, 0, len(resp.Tools))
	for _, t := range resp.Tools {
		description := ""
		if t.Description != nil {
			description = *t.Description
		}
		entries = append(entries, mcpToolEntry{
			Server:      c.url,
			Name:        t.Name,
			Description: description,
			InputSchema: t.InputSchema,
		})
	}
	return entries, nil
}

func (c *httpMCPClient) CallTool(ctx context.Context, name string, arguments map[string]any) (string, error) {
	resp, err := c.inner.CallTool(ctx, name, arguments)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(resp.Content))
	for _, content := range resp.Content {
		if content != nil && content.TextContent != nil {
			parts = append(parts, content.TextContent.Text)
		}
	}
	return strings.Join(parts, ""), nil
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	config "github.com/inference-gateway/adk/server/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	zap "go.uber.org/zap"
)

// fakeMCPClient is an in-process mcpToolCaller for tests - no network, no server.
type fakeMCPClient struct {
	url          string
	tools        []mcpToolEntry
	failInits    int32
	initCalls    int32
	listErr      error
	callResult   string
	callErr      error
	lastCallName string
	lastCallArgs map[string]any
	mu           sync.Mutex
}

func (f *fakeMCPClient) Initialize(context.Context) error {
	n := atomic.AddInt32(&f.initCalls, 1)
	if n <= atomic.LoadInt32(&f.failInits) {
		return fmt.Errorf("connection refused")
	}
	return nil
}

func (f *fakeMCPClient) ListTools(context.Context) ([]mcpToolEntry, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]mcpToolEntry, len(f.tools))
	for i, t := range f.tools {
		t.Server = f.url
		out[i] = t
	}
	return out, nil
}

func (f *fakeMCPClient) CallTool(_ context.Context, name string, arguments map[string]any) (string, error) {
	f.mu.Lock()
	f.lastCallName = name
	f.lastCallArgs = arguments
	f.mu.Unlock()
	if f.callErr != nil {
		return "", f.callErr
	}
	return f.callResult, nil
}

func testMCPConfig(servers ...string) config.MCPConfig {
	return config.MCPConfig{
		Enable:           true,
		Servers:          servers,
		Endpoint:         "/mcp",
		RefreshInterval:  50 * time.Millisecond,
		DialTimeout:      time.Second,
		CallTimeout:      time.Second,
		RetryInterval:    5 * time.Millisecond,
		RetryMaxInterval: 20 * time.Millisecond,
	}
}

// waitFor polls until cond is true or the deadline passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("condition not met before deadline")
}

func TestMCPManager_DiscoversToolsAndRegisters(t *testing.T) {
	fake := &fakeMCPClient{
		url: "http://mcp-a",
		tools: []mcpToolEntry{
			{Name: "get_weather", Description: "Get weather for a city"},
			{Name: "get_time", Description: "Get current time"},
		},
	}

	m, err := NewMCPClientManager(testMCPConfig("http://mcp-a"), zap.NewNop())
	require.NoError(t, err)
	m.newClient = func(string) mcpToolCaller { return fake }

	m.Start(context.Background())
	defer func() { _ = m.Close() }()

	waitFor(t, func() bool { return len(m.snapshot()) == 2 })

	tb := NewToolBox()
	m.RegisterTools(tb)
	assert.True(t, tb.HasTool("mcp_list_tools"))
	assert.True(t, tb.HasTool("mcp_call_tool"))
}

func TestMCPManager_ListToolsToolFiltersBySearch(t *testing.T) {
	fake := &fakeMCPClient{
		url: "http://mcp-a",
		tools: []mcpToolEntry{
			{Name: "get_weather", Description: "Get weather for a city"},
			{Name: "get_time", Description: "Get current time"},
		},
	}
	m, err := NewMCPClientManager(testMCPConfig("http://mcp-a"), zap.NewNop())
	require.NoError(t, err)
	m.newClient = func(string) mcpToolCaller { return fake }
	m.Start(context.Background())
	defer func() { _ = m.Close() }()
	waitFor(t, func() bool { return len(m.snapshot()) == 2 })

	out, err := m.executeListTools(context.Background(), map[string]any{"search": "weather"})
	require.NoError(t, err)

	var payload struct {
		Count int            `json:"count"`
		Tools []mcpToolEntry `json:"tools"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &payload))
	assert.Equal(t, 1, payload.Count)
	require.Len(t, payload.Tools, 1)
	assert.Equal(t, "get_weather", payload.Tools[0].Name)
	assert.Equal(t, "http://mcp-a", payload.Tools[0].Server)
}

func TestMCPManager_CallToolDispatches(t *testing.T) {
	fake := &fakeMCPClient{
		url:        "http://mcp-a",
		tools:      []mcpToolEntry{{Name: "get_weather", Description: "Get weather"}},
		callResult: `{"temp":"22C"}`,
	}
	m, err := NewMCPClientManager(testMCPConfig("http://mcp-a"), zap.NewNop())
	require.NoError(t, err)
	m.newClient = func(string) mcpToolCaller { return fake }
	m.Start(context.Background())
	defer func() { _ = m.Close() }()
	waitFor(t, func() bool { return len(m.snapshot()) == 1 })

	out, err := m.executeCallTool(context.Background(), map[string]any{
		"name":      "get_weather",
		"arguments": map[string]any{"city": "berlin"},
	})
	require.NoError(t, err)
	assert.Equal(t, `{"temp":"22C"}`, out)
	assert.Equal(t, "get_weather", fake.lastCallName)
	assert.Equal(t, "berlin", fake.lastCallArgs["city"])
}

func TestMCPManager_CallToolUnknownTool(t *testing.T) {
	m, err := NewMCPClientManager(testMCPConfig("http://mcp-a"), zap.NewNop())
	require.NoError(t, err)
	m.newClient = func(string) mcpToolCaller { return &fakeMCPClient{url: "http://mcp-a"} }
	m.Start(context.Background())
	defer func() { _ = m.Close() }()

	_, err = m.executeCallTool(context.Background(), map[string]any{"name": "nope"})
	assert.Error(t, err)
}

// TestMCPManager_ConnectRetriesWithBackoff proves the manager keeps retrying a
// server that is not up yet - the cloud-native resilience the feature targets.
func TestMCPManager_ConnectRetriesWithBackoff(t *testing.T) {
	fake := &fakeMCPClient{
		url:       "http://mcp-a",
		failInits: 3, // fail the first 3 connect attempts, then succeed
		tools:     []mcpToolEntry{{Name: "get_weather"}},
	}
	m, err := NewMCPClientManager(testMCPConfig("http://mcp-a"), zap.NewNop())
	require.NoError(t, err)
	m.newClient = func(string) mcpToolCaller { return fake }
	m.Start(context.Background())
	defer func() { _ = m.Close() }()

	waitFor(t, func() bool { return len(m.snapshot()) == 1 })
	assert.GreaterOrEqual(t, atomic.LoadInt32(&fake.initCalls), int32(4))
}

func TestMCPManager_NoServers(t *testing.T) {
	_, err := NewMCPClientManager(config.MCPConfig{Enable: true}, zap.NewNop())
	assert.Error(t, err)
}

func TestNextBackoff(t *testing.T) {
	assert.Equal(t, 4*time.Second, nextBackoff(2*time.Second, 30*time.Second))
	assert.Equal(t, 30*time.Second, nextBackoff(20*time.Second, 30*time.Second))
}

func TestSleepCtx_Cancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	assert.False(t, sleepCtx(ctx, time.Second))
}

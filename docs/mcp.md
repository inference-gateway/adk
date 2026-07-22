# Connecting an A2A Server to MCP Servers

The ADK can connect an A2A server's agent to one or more [Model Context
Protocol](https://modelcontextprotocol.io) (MCP) servers, discover the tools they
expose, and let the LLM invoke them. This is useful when you have a common MCP
server (or a fleet of them) that all your A2A agents should reuse.

The integration is built on [`metoro-io/mcp-golang`](https://github.com/metoro-io/mcp-golang)
and is **disabled by default** - it only makes sense when an LLM/agent is
configured.

## Table of Contents

- [How it works: the selector pattern](#how-it-works-the-selector-pattern)
- [Configuration](#configuration)
- [Wiring it up](#wiring-it-up)
- [Resilience: retries and polling backoff](#resilience-retries-and-polling-backoff)
- [Limitations](#limitations)

## How it works: the selector pattern

A single MCP server can expose dozens or hundreds of tools. Loading every tool
schema into the LLM context would overwhelm the context window, so the ADK does
**not** register MCP tools individually. Instead the `MCPClientManager` registers
just **two selector tools** into the agent's toolbox, regardless of how many
tools the MCP servers offer:

- **`mcp_list_tools`** - Launchpad: lists discovered tools (server, name, description, input schema); accepts an optional `search` filter.
- **`mcp_call_tool`** - Invokes a tool by `name` (optionally disambiguated by `server`) with an `arguments` object.

A typical agent turn: the LLM calls `mcp_list_tools` to find a relevant tool,
reads its input schema, then calls `mcp_call_tool` to run it. Only the metadata
the model actually asks for reaches the context window.

The tool catalog is discovered in the background and refreshed on an interval, so
`mcp_list_tools` returns fast from an in-memory snapshot.

## Configuration

All settings are read from environment variables under the `MCP_` prefix (in the
examples, which nest the ADK config under `A2A_`, the prefix is `A2A_MCP_`).

- **`MCP_ENABLE`** (default `false`) - Enable the MCP client.
- **`MCP_SERVERS`** - Comma-separated MCP server base URLs, e.g. `http://mcp:8080`.
- **`MCP_ENDPOINT`** (default `/mcp`) - HTTP path appended to each server URL for the streamable MCP endpoint.
- **`MCP_REFRESH_INTERVAL`** (default `5m`) - How often to refresh the tool catalog from each server.
- **`MCP_DIAL_TIMEOUT`** (default `30s`) - Timeout for initializing / listing tools.
- **`MCP_CALL_TIMEOUT`** (default `30s`) - Timeout for a single tool invocation.
- **`MCP_MAX_RETRIES`** (default `0`) - Max initial connection attempts per server (`0` = retry forever).
- **`MCP_RETRY_INTERVAL`** (default `2s`) - Initial backoff between connection/refresh retries (doubles).
- **`MCP_RETRY_MAX_INTERVAL`** (default `30s`) - Maximum backoff between retries.

## Wiring it up

The manager is a standalone piece you construct, start, and register into the
agent's toolbox:

```go
toolBox := server.NewDefaultToolBox(&cfg.AgentConfig.ToolBoxConfig)

if cfg.MCPConfig.Enable {
	mcpManager, err := server.NewMCPClientManager(cfg.MCPConfig, logger)
	if err != nil {
		logger.Fatal("failed to create MCP client manager", zap.Error(err))
	}
	mcpManager.Start(ctx)               // background connect + refresh
	defer mcpManager.Close()
	mcpManager.RegisterTools(toolBox)   // adds mcp_list_tools + mcp_call_tool
}

agent, _ := server.NewAgentBuilder(logger).
	WithConfig(&cfg.AgentConfig).
	WithLLMClient(llmClient).
	WithToolBox(toolBox).
	Build()
```

`Start` returns immediately; it never blocks server startup if an MCP server is
not up yet. See [`examples/mcp`](../examples/mcp) for a full runnable example.

## Resilience: retries and polling backoff

Built for cloud-native rollouts where an MCP server may start after, or restart
independently of, the A2A server:

- **Connection retries** - each server is connected in a background goroutine
  with exponential backoff (`MCP_RETRY_INTERVAL` doubling up to
  `MCP_RETRY_MAX_INTERVAL`), retrying forever by default (`MCP_MAX_RETRIES=0`).
- **Polling backoff** - once connected, the tool catalog is refreshed every
  `MCP_REFRESH_INTERVAL`. If a refresh fails (server restarted or briefly
  unreachable) the manager backs off - doubling up to `MCP_RETRY_MAX_INTERVAL` -
  and resumes the normal interval as soon as the server responds again. A failing
  server never drops the catalog of the healthy ones.

## Limitations

- Transport is streamable **HTTP** only (the cloud-native case). stdio/subprocess
  MCP servers are not wired.
- Tool listing fetches a single page; cursor-based pagination is not yet handled.
- MCP resources and prompts are not exposed - tools only.

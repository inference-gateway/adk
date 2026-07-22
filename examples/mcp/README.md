# MCP-Powered A2A Example

This example runs an A2A server whose agent discovers and invokes tools from one
or more [MCP](https://modelcontextprotocol.io) servers, using the ADK's MCP
client.

Instead of loading every MCP tool into the LLM context, the agent gets two
selector tools - `mcp_list_tools` and `mcp_call_tool` - and pulls tool metadata
on demand, so a large MCP catalog never overwhelms the context window. See
[`docs/mcp.md`](../../docs/mcp.md) for the full design.

## Directory Structure

```text
mcp/
├── server/
│   ├── main.go         # A2A server that connects to MCP servers
│   └── config/
│       └── config.go   # Configuration
└── README.md
```

## Running the Example

An MCP connection only makes sense with an LLM configured, so set the provider,
model, and API key, plus the MCP server(s) to connect to:

```bash
cd server
A2A_AGENT_CLIENT_PROVIDER=openai \
A2A_AGENT_CLIENT_MODEL=gpt-4o-mini \
A2A_AGENT_CLIENT_API_KEY=sk-... \
A2A_MCP_ENABLE=true \
A2A_MCP_SERVERS=http://localhost:8083 \
go run .
```

Point `A2A_MCP_SERVERS` at any streamable-HTTP MCP server (comma-separate several).
The MCP endpoint path defaults to `/mcp`; override with `A2A_MCP_ENDPOINT`.

The server starts even if the MCP server is not up yet - it keeps retrying with
backoff in the background.

## Try It

Send a task and watch the agent discover and call MCP tools:

```bash
curl -s http://localhost:8080/a2a -H 'Content-Type: application/json' -d '{
  "jsonrpc": "2.0",
  "id": "1",
  "method": "message/send",
  "params": {
    "message": {
      "role": "user",
      "messageId": "m1",
      "parts": [{ "kind": "text", "text": "What MCP tools do you have? Use one of them." }]
    }
  }
}'
```

## Configuration

See the MCP settings in [`docs/mcp.md`](../../docs/mcp.md#configuration). In this
example the ADK config is nested under `A2A_`, so every `MCP_*` variable becomes
`A2A_MCP_*`.

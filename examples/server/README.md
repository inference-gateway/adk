# A2A Server Examples

This directory contains examples demonstrating how to create A2A (Agent-to-Agent) compatible servers using the A2A ADK (Agent Development Kit).

## Overview

The A2A protocol enables agents to communicate with each other using JSON-RPC over HTTP. These examples show different approaches to creating A2A servers:

1. **Basic Non-AI Server** - A minimal server without AI capabilities that handles A2A protocol messages using simple task handlers
2. **AI-Powered Server** - A full-featured server with LLM integration and tool capabilities

## Quick Start

### 1. Minimal Non-AI Server

The simplest way to create an A2A server is using `NewDefaultA2AServer()`. This creates a basic server that handles A2A protocol messages without any AI capabilities:

```bash
go run cmd/minimal/main.go
```

This minimal example:

- ✅ Handles A2A protocol messages (`message/send`, `message/stream`, `tasks/get`, `tasks/cancel`)
- ✅ Provides agent metadata via `/.well-known/agent.json`
- ✅ Health check endpoint at `/health`
- ✅ Automatic configuration from environment variables
- ❌ No AI/LLM integration
- ❌ No custom tools
- ❌ No complex setup required

This is perfect for understanding the basic A2A protocol or creating simple non-AI agents.

### 2. AI-Powered Server with Tools

For AI capabilities, use the aipowered example which supports both modes:

```bash
# Without AI (mock mode)
go run cmd/aipowered/main.go

# With AI (set your API key)
LLM_API_KEY=your-api-key go run cmd/aipowered/main.go
```

## Example Usage

Once your server is running, you can test it with the A2A protocol:

```bash
# Test agent info
curl http://localhost:8080/.well-known/agent.json

# Send a message
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "role": "user",
        "content": "Hello!"
      }
    },
    "id": 1
  }'
```

## Configuration

All servers support configuration via environment variables:

- `AGENT_NAME` - Name of your agent (default: "helloworld-agent")
- `AGENT_DESCRIPTION` - Description of your agent
- `PORT` - Server port (default: "8080")
- `DEBUG` - Enable debug logging (default: false)
- `LLM_API_KEY` - API key for LLM provider (enables AI features)

## Files

- `cmd/minimal/main.go` - Minimal non-AI server example using `NewDefaultA2AServer()`
- `cmd/aipowered/main.go` - Full-featured example with AI support and tools
- `main.go` - Placeholder with general guidance

## Next Steps

1. Start with the minimal example to understand the basic A2A protocol
2. Add custom task handlers for your specific use case
3. Integrate LLM providers for AI capabilities
4. Add custom tools for enhanced functionality

For more advanced usage, see the ADK documentation and builder patterns in the main codebase.

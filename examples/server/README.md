# A2A Server Examples

This directory contains examples demonstrating how to create A2A (Agent-to-Agent) compatible servers using the A2A ADK (Agent Development Kit).

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
   1. [Minimal Server (No AI Required)](#1-minimal-server-no-ai-required)
   2. [AI-Powered Server (API Key Required)](#2-ai-powered-server-api-key-required)
   3. [Pausable Task Server (API Key Required)](#3-pausable-task-server-api-key-required)
   4. [Travel Planning Server (Domain Expert)](#4-travel-planning-server-domain-expert)
3. [Example Usage](#example-usage)

## Overview

The A2A protocol enables agents to communicate with each other using JSON-RPC over HTTP. These examples show different approaches to creating A2A servers:

1. **Minimal Server** - A working server with custom task handler that provides simple responses without AI
2. **AI-Powered Server** - A full-featured server with LLM integration and tool calling capabilities
3. **Pausable Task Server** - An AI-powered server that demonstrates intelligent task pausing with the `input-required` state
4. **Travel Planning Server** - A specialized domain expert agent for vacation planning with streaming and paused tasks

## Quick Start

### 1. Minimal Server (No AI Required)

A working A2A server with simple conversational responses using a custom task handler:

```bash
go run cmd/minimal/main.go
```

This minimal example:

- ✅ Handles A2A protocol messages correctly (`message/send`, `tasks/get`, `tasks/cancel`)
- ✅ Provides conversational responses (greetings, status, help, time)
- ✅ Agent metadata via `/.well-known/agent.json`
- ✅ Health check endpoint at `/health`
- ✅ Echo functionality for any text input
- ✅ **Works immediately** - no configuration required
- ❌ No AI/LLM integration (by design)
- ❌ No advanced tools or function calling

Perfect for learning the A2A protocol, creating deterministic business logic agents, or simple automation tasks.

### 2. AI-Powered Server (API Key Required)

For AI capabilities with LLM integration and tool calling:

```bash
# Configure the Inference Gateway
cp .env.gateway.example .env

# Edit .env to configure the Inference Gateway URL and other settings
docker run -d --name inference-gateway -p 8081:8080 ghcr.io/inference-gateway/inference-gateway:latest
export AGENT_CLIENT_BASE_URL=http://localhost:8081/v1
export AGENT_CLIENT_PROVIDER=deepseek # Choose your LLM provider (openai, anthropic, ollama, deepseek, google, claudflare, etc.)
export AGENT_CLIENT_MODEL=deepseek-chat

go run cmd/aipowered/main.go
```

This AI-powered example:

- ✅ **Requires valid API key** - will not start without proper configuration
- ✅ Supports multiple LLM providers (OpenAI, Anthropic, DeepSeek, Ollama)
- ✅ Tool calling capabilities (weather, time tools included)
- ✅ Full conversation context and history
- ✅ Works with Inference Gateway for unified LLM access
- ✅ Production-ready AI agent architecture

### 3. Pausable Task Server (API Key Required)

Demonstrates intelligent task pausing where the LLM decides when to request user input:

```bash
# Configure the Inference Gateway
cp .env.gateway.example .env

# Edit .env to configure the Inference Gateway URL and other settings
docker run -d --name inference-gateway --env-file .env -p 8081:8080 ghcr.io/inference-gateway/inference-gateway:latest
export AGENT_CLIENT_BASE_URL=http://localhost:8081/v1
export AGENT_CLIENT_PROVIDER='deepseek' # Choose your LLM provider (openai, anthropic, ollama, deepseek, google, claudflare, etc.)
export AGENT_CLIENT_MODEL='deepseek-chat'

go run cmd/pausedtask/main.go
```

This pausable task example:

- ✅ **LLM-driven pausing** - Agent intelligently determines when more user input is needed
- ✅ Built-in `request_user_input` tool that LLM can call to pause execution
- ✅ Demonstrates complete input-required workflow (submit → pause → resume → complete)
- ✅ Works with existing pausedtask client example
- ✅ Production-ready pattern for human-in-the-loop AI workflows

### 4. Travel Planning Server (Domain Expert)

A specialized travel planning agent that demonstrates domain expertise with streaming and paused task capabilities:

```bash
# Configure the Inference Gateway (same as above)
export AGENT_CLIENT_BASE_URL=http://localhost:8081/v1
export AGENT_CLIENT_PROVIDER='deepseek'
export AGENT_CLIENT_MODEL='deepseek-chat'

go run cmd/travelplanner/main.go
```

This travel planning example:

- ✅ **Domain Expertise** - Specialized in vacation planning with travel-specific tools
- ✅ **Smart Travel Tools** - Weather data, budget estimation, activity recommendations
- ✅ **Interactive Planning** - Intelligently gathers preferences through conversation
- ✅ **Perfect Match** - Designed specifically for the `pausedtask-streaming` client example
- ✅ **Comprehensive Output** - Creates detailed day-by-day itineraries with budget breakdowns
- ✅ **Real-world Pattern** - Shows how to build domain-specific A2A agents

**Perfect for testing with**: `examples/client/cmd/pausedtask-streaming` - Send "I need help planning a vacation" to see the full interactive planning process!

## Example Usage

### Test the A2A Protocol

Both servers support the standard A2A protocol. Here's how to test them:

```bash
# Get agent information
curl http://localhost:8080/.well-known/agent.json | jq .

# Send a message (proper A2A format)
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-001",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Hello! Can you help me?"
          }
        ]
      }
    },
    "id": 1
  }' | jq .

# Get task results
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tasks/get",
    "params": {
      "id": "TASK_ID_FROM_PREVIOUS_RESPONSE"
    },
    "id": 2
  }' | jq .
```

## Building with Custom Agent Metadata

Both examples use build-time metadata injection via LD flags. You can customize the agent metadata when building:

```bash
# Build with custom agent metadata
go build -ldflags="-X github.com/inference-gateway/adk/server.BuildAgentName=my-custom-agent -X 'github.com/inference-gateway/adk/server.BuildAgentDescription=My custom A2A agent description' -X github.com/inference-gateway/adk/server.BuildAgentVersion=2.0.0" -o my-agent cmd/minimal/main.go
```

The agent metadata appears in:

- `/.well-known/agent.json` endpoint
- Server startup logs
- A2A protocol responses

## Configuration

### Minimal Server

- `PORT` - Server port (default: "8080")
- No other configuration required!

### AI-Powered Server

**Required:**

- `AGENT_CLIENT_BASE_URL` - Your inference gateway URL (automatically configures AGENT_CLIENT_BASE_URL)
- `AGENT_CLIENT_PROVIDER` - Your LLM provider (openai, anthropic, ollama, deepseek, google, claudflare, etc.)
- `AGENT_CLIENT_MODEL` - Model name (e.g., "gpt-4", "claude-2", "deepseek-chat")

**Optional:**

- `AGENT_CLIENT_MODEL` - Model name (uses provider defaults if not specified)
- `PORT` - Server port (default: "8080")

### Configuration Examples

```bash
# Standard Inference Gateway
export AGENT_CLIENT_BASE_URL="http://localhost:3000/v1"

# Custom Inference Gateway with specific model
export AGENT_CLIENT_BASE_URL="http://localhost:3000/v1"
export AGENT_CLIENT_MODEL="gpt-4"

# Production Gateway
export AGENT_CLIENT_BASE_URL="https://gateway.example.com/v1"
```

## Architecture

### Minimal Server

- **CustomTaskHandler**: Processes messages with simple business logic
- **No LLM dependency**: Fast, deterministic responses
- **A2A compliant**: Full protocol support without AI complexity

### AI-Powered Server

- **OpenAICompatibleAgent**: Handles LLM communication
- **ToolBox**: Function calling capabilities
- **Multiple LLM Providers**: Flexible provider support
- **Conversation Management**: Context-aware interactions

## Example Output

When you run the examples with custom build-time metadata, you'll see the agent information displayed in the startup logs:

**Minimal Server Example:**

```bash
# Build with custom metadata
go build -ldflags="-X 'github.com/inference-gateway/adk/server.BuildAgentName=Weather Assistant' \
  -X 'github.com/inference-gateway/adk/server.BuildAgentDescription=AI-powered weather and time assistant' \
  -X 'github.com/inference-gateway/adk/server.BuildAgentVersion=2.1.0'" \
  -o weather-agent cmd/minimal/main.go

# Run the agent
./weather-agent
```

**Output:**

```
🤖 Starting Minimal A2A Server (Non-AI)...
2025-07-20T09:14:26.290Z  INFO  ✅ minimal A2A server created with simple task handler
2025-07-20T09:14:26.290Z  INFO  🤖 agent metadata {"name": "Weather Assistant", "description": "AI-powered weather and time assistant", "version": "2.1.0"}
2025-07-20T09:14:26.290Z  INFO  🌐 server running {"port": "8080"}

🎯 Test the server:
📋 Agent info: http://localhost:8080/.well-known/agent.json
💚 Health check: http://localhost:8080/health
📡 A2A endpoint: http://localhost:8080/a2a
```

The agent metadata is also available via the agent info endpoint:

```bash
curl http://localhost:8080/.well-known/agent.json | jq
{
  "name": "Weather Assistant",
  "description": "AI-powered weather and time assistant",
  "version": "2.1.0",
  "capabilities": { ... },
  ...
}
```

## Files

- `cmd/minimal/main.go` - Simple working server with custom task handler
- `cmd/aipowered/main.go` - AI-powered server with LLM integration and tools
- `cmd/pausedtask/main.go` - AI-powered server with intelligent task pausing capabilities
- `README.md` - This documentation

## Next Steps

1. **Start Simple**: Run the minimal example to understand A2A protocol basics
2. **Add Business Logic**: Customize the task handler for your specific use case
3. **Add AI**: Use the AI-powered example with your API key for intelligent responses
4. **Extend Tools**: Add custom tools and functions for your domain
5. **Production**: See the main ADK documentation for advanced patterns and deployment

For more information, see the [A2A ADK documentation](../../README.md) and [A2A Protocol Specification](https://github.com/inference-gateway/schemas/tree/main/a2a).

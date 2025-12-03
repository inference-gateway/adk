# Callbacks Example

This example demonstrates how to use the **callback feature** in the ADK (Agent Development Kit) to hook into various points of the agent's execution lifecycle.

Callbacks allow you to:

- Log and monitor agent execution at each step
- Implement guardrails and validation
- Cache LLM responses
- Authorize tool usage
- Modify inputs and outputs

## Callback Types

The ADK supports six types of callbacks:

| Callback      | When Triggered                  | Purpose                                       |
| ------------- | ------------------------------- | --------------------------------------------- |
| `BeforeAgent` | Before agent execution starts   | Guardrails, validation, early returns         |
| `AfterAgent`  | After agent execution completes | Post-processing, logging, output modification |
| `BeforeModel` | Before each LLM call            | Caching, request modification, guardrails     |
| `AfterModel`  | After each LLM response         | Response modification, logging, sanitization  |
| `BeforeTool`  | Before each tool execution      | Authorization, caching, argument modification |
| `AfterTool`   | After each tool execution       | Result modification, logging, sanitization    |

## Flow Control

- **Before callbacks**: Return `nil` to continue normal execution, or return a value to skip the default behavior and use the returned value instead.
- **After callbacks**: Return `nil` to use the original output, or return a modified value to replace it.

## Usage Example

```go
callbackConfig := &server.CallbackConfig{
    BeforeAgent: []server.BeforeAgentCallback{
        func(ctx context.Context, callbackCtx *server.CallbackContext) *types.Message {
            // Return nil to proceed, or return a message to skip agent execution
            return nil
        },
    },
    AfterAgent: []server.AfterAgentCallback{
        func(ctx context.Context, callbackCtx *server.CallbackContext, output *types.Message) *types.Message {
            // Return nil to use original output, or return modified output
            return nil
        },
    },
    BeforeModel: []server.BeforeModelCallback{
        func(ctx context.Context, callbackCtx *server.CallbackContext, request *server.LLMRequest) *server.LLMResponse {
            // Return nil to proceed with LLM call, or return a response to skip it
            return nil
        },
    },
    BeforeTool: []server.BeforeToolCallback{
        func(ctx context.Context, tool server.Tool, args map[string]any, toolCtx *server.ToolContext) map[string]any {
            // Return nil to execute tool, or return a result to skip execution
            return nil
        },
    },
}

agent, err := server.NewAgentBuilder(logger).
    WithConfig(&cfg.AgentConfig).
    WithLLMClient(llmClient).
    WithCallbacks(callbackConfig).
    Build()
```

## Running the Example

### Option 1: Docker Compose (Recommended)

This setup includes the Inference Gateway for LLM routing.

1. Copy the environment file and add your DeepSeek API key:

```bash
cp .env.example .env
# Edit .env and add your DEEPSEEK_API_KEY
```

2. Start the services:

```bash
docker-compose up --build
```

The server will be available at `http://localhost:8080` (port can be configured in `.env`)

### Option 2: Local Development

1. Set required environment variables:

```bash
export A2A_SERVER_PORT=8081
export A2A_AGENT_CLIENT_PROVIDER=deepseek
export A2A_AGENT_CLIENT_MODEL=deepseek-chat
export A2A_AGENT_CLIENT_API_KEY=your-deepseek-api-key
export A2A_AGENT_CLIENT_BASE_URL=http://localhost:8080/v1 # Inference Gateway URL - you need to run it separately
```

2. Run the server:

```bash
cd examples/callbacks/server
go run main.go
```

The server will be available at `http://localhost:8081`

## Testing the Callbacks

Send a test request (adjust port based on how you're running):

```bash
curl -X POST http://localhost:8081/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "params": {
      "message": {
        "kind": "message",
        "role": "user",
        "parts": [{"kind": "text", "text": "Please use the echo tool to say hello"}]
      }
    },
    "id": "1"
  }'

# Or use the streaming endpoint
curl -X POST http://localhost:8081/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/stream",
    "params": {
      "message": {
        "kind": "message",
        "role": "user",
        "parts": [{"kind": "text", "text": "Please use the echo tool to say hello"}]
      }
    },
    "id": "1"
  }'
```

## What to Expect

Watch the server logs to see the callbacks being triggered in sequence with colored emojis:

```
🔵 BeforeAgent: Starting agent execution
🟢 BeforeModel: About to call LLM
🟡 AfterModel: Received LLM response
🟣 BeforeTool: About to execute tool (echo)
🟠 AfterTool: Tool execution completed
🔴 AfterAgent: Agent execution completed
```

Each callback shows the execution flow and allows you to inspect or modify the data at each step.

## Context Objects

### CallbackContext

Available in agent and model callbacks:

- `AgentName`: Name of the agent
- `TaskID`: Current task ID
- `ContextID`: Conversation context ID
- `State`: Mutable state map for passing data between callbacks
- `Logger`: Logger instance

### ToolContext

Available in tool callbacks (extends CallbackContext functionality):

- `AgentName`: Name of the agent
- `TaskID`: Current task ID
- `ContextID`: Conversation context ID
- `State`: Mutable state map
- `Logger`: Logger instance

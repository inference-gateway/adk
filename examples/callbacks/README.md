# Callbacks Example

This example demonstrates how to use the callback feature in the ADK (Agent Development Kit) to hook into various points of the agent's execution lifecycle.

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

1. Set required environment variables:

```bash
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
export A2A_AGENT_CLIENT_API_KEY=your-api-key
```

2. Run the server:

```bash
cd examples/callbacks/server
go run main.go
```

3. Send a test request:

```bash
curl -X POST http://localhost:8080/tasks/send \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "tasks/send",
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

Watch the server logs to see the callbacks being triggered in sequence.

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

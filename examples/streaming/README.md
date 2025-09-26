# Streaming A2A Example

This example demonstrates real-time streaming responses from an A2A server, perfect for chat applications and interactive AI experiences.

## What This Example Shows

- Real-time streaming of AI responses
- Character-by-character output for better UX
- Mock streaming when no AI is configured
- Proper event stream handling

## Directory Structure

```
streaming/
├── client/
│   ├── main.go       # Streaming client
│   └── Dockerfile
├── server/
│   ├── main.go       # Streaming-enabled server
│   └── Dockerfile
├── docker-compose.yaml
└── README.md
```

## Running the Example

### Using Docker Compose (Recommended)

With AI (requires API key):

```bash
export AGENT_CLIENT_API_KEY="your-api-key"
docker-compose up --build
```

Without AI (mock streaming):

```bash
docker-compose up --build
```

### Running Locally

#### Start the Server

```bash
cd server
# For AI streaming:
export AGENT_CLIENT_API_KEY="your-api-key"
export AGENT_CLIENT_PROVIDER="openai"
export AGENT_CLIENT_MODEL="gpt-4o-mini"
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## Understanding Streaming

### Server Stream Events

The server sends different event types during streaming:

1. **Status Event** - Task state updates
2. **Delta Event** - Content chunks (characters/words)
3. **Error Event** - Error notifications
4. **Task Complete** - Final task with full response

### Client Stream Handling

```go
stream, err := a2aClient.StreamMessage(ctx, message)

for event := range stream {
    switch event.Type {
    case "delta":
        // Print character without newline
        fmt.Print(event.Data)
    case "task_complete":
        // Task finished successfully
    case "error":
        // Handle error
    }
}
```

## Example Output

```
Sending streaming request...
Received stream:
T-h-i-s- -i-s- -a- -s-t-r-e-a-m-i-n-g- -r-e-s-p-o-n-s-e-.- -E-a-c-h- -c-h-a-r-a-c-t-e-r-
-a-p-p-e-a-r-s- -i-n- -r-e-a-l---t-i-m-e-.

✅ Stream completed
```

## Configuration

### Server

- `CAPABILITIES_STREAMING`: Must be `true`
- `AGENT_CLIENT_API_KEY`: For AI streaming
- `AGENT_CLIENT_PROVIDER`: LLM provider
- `AGENT_CLIENT_MODEL`: Model to use

### Client

- Automatically detects server streaming capability
- Falls back to non-streaming if not supported

## Use Cases

- **Chat Applications**: Real-time conversation UI
- **Code Generation**: Show code as it's generated
- **Content Creation**: Display writing in progress
- **Progress Updates**: Stream processing steps

## Next Steps

- Try `paused-task` for interactive workflows
- Check `artifacts` for streaming with file generation
- See `travel-planner` for complex streaming scenarios

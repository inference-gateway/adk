# Streaming A2A Example

This example demonstrates real-time streaming responses from an A2A server, perfect for chat applications and interactive experiences.

## What This Example Shows

- Real-time streaming of responses
- Character-by-character output for better UX
- Mock streaming when no AI is configured
- Proper event stream handling

## Directory Structure

```
streaming/
├── client/
│   └── main.go          # Streaming client
├── server/
│   └── main.go          # Streaming-enabled server
├── docker-compose.yaml  # Uses ../Dockerfile.server and ../Dockerfile.client
└── README.md
```

## Running the Example

### Using Docker Compose (Recommended)

```bash
docker-compose up --build
```

### Running Locally

#### Start the Server

```bash
cd server
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
stream, err := a2aClient.SendTaskStreaming(ctx, params)
if err != nil {
    log.Fatalf("Failed to start streaming: %v", err)
}

for event := range stream {
    // Process each streaming event
    fmt.Printf("Received event: %+v\n", event)
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

### Client

- Automatically detects server streaming capability
- Falls back to non-streaming if not supported

## Use Cases

- **Chat Applications**: Real-time conversation UI
- **Code Generation**: Show code as it's generated
- **Content Creation**: Display writing in progress
- **Progress Updates**: Stream processing steps

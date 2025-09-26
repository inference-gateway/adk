# Minimal A2A Example

This example demonstrates the most basic A2A server and client setup without any AI integration.

## What This Example Shows

- Basic A2A server setup with mock responses
- Simple client sending messages to the server
- Docker Compose configuration for easy deployment
- No external dependencies or AI services required

## Directory Structure

```
minimal/
├── client/
│   ├── main.go       # Simple A2A client
│   └── Dockerfile    # Client container
├── server/
│   ├── main.go       # Basic A2A server with mock handler
│   └── Dockerfile    # Server container
├── docker-compose.yaml
└── README.md
```

## Running the Example

### Using Docker Compose (Recommended)

```bash
docker-compose up --build
```

This will:
1. Start the A2A server on port 8080
2. Wait 5 seconds for the server to be ready
3. Run the client which sends a test message
4. Display the server's mock response

### Running Locally

#### Start the Server

```bash
cd server
go run main.go
```

The server will start on `http://localhost:8080`

#### Run the Client

In another terminal:

```bash
cd client
go run main.go
```

## Server Configuration

The server uses environment variables for configuration:

- `PORT`: Server port (default: 8080)
- `AGENT_NAME`: Agent identifier (default: minimal-agent)
- `LOG_LEVEL`: Logging verbosity (default: debug)

## Client Configuration

- `SERVER_URL`: A2A server URL (default: http://localhost:8080)

## Understanding the Code

### Server (`server/main.go`)

The server implements a basic task handler that returns mock responses without any AI processing:

```go
func (h *MockTaskHandler) HandleTask(ctx context.Context, task *types.Task, message types.Message) (*types.Task, error) {
    // Returns a simple mock response
    task.Status = types.TaskStatusCompleted
    task.Output = fmt.Sprintf("Mock response: Received message: %s", message.Content)
    return task, nil
}
```

### Client (`client/main.go`)

The client demonstrates basic message sending:

```go
// Create client
a2aClient := client.NewA2AClient(serverURL)

// Send message
response, err := a2aClient.SendMessage(ctx, message)
```

## Next Steps

- Try the `ai-powered` example to see AI integration
- Check the `streaming` example for real-time responses
- Explore `artifacts` example for file handling
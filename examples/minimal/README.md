# Minimal A2A Example

This example demonstrates the most basic A2A server and client setup without any AI integration.

## What This Example Shows

- Basic A2A server setup with mock responses
- Simple client sending messages to the server
- No external dependencies or AI services required

## Directory Structure

```
minimal/
├── client/
│   └── main.go          # Simple A2A client
├── server/
│   ├── main.go          # Basic A2A server with echo handler
│   └── config/
│       └── config.go    # Configuration
├── docker-compose.yaml  # Uses ../Dockerfile.server and ../Dockerfile.client
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

The server will start on port `8080`

#### Run the Client

In another terminal:

```bash
cd client
go run main.go
```

## Server Configuration

The server uses environment variables for configuration, following the production agent pattern with the `A2A_` prefix:

| Environment Variable | Description | Default |
|---------------------|-------------|---------||
| `ENVIRONMENT` | Runtime environment (development/production) | `development` |
| `A2A_AGENT_NAME` | Name of the agent | `minimal-agent` |
| `A2A_AGENT_DESCRIPTION` | Agent description | `A minimal A2A server that echoes messages` |
| `A2A_AGENT_VERSION` | Agent version | `0.3.0` |
| `A2A_SERVER_PORT` | Server port | `8080` |
| `A2A_DEBUG` | Enable debug logging | `false` |
| `A2A_CAPABILITIES_STREAMING` | Enable streaming support | `false` |
| `A2A_CAPABILITIES_PUSH_NOTIFICATIONS` | Enable push notifications | `false` |

### Configuration Pattern

This example follows the same configuration pattern as production agents like the n8n-agent:

```go
type Config struct {
    Environment string              `env:"ENVIRONMENT,default=development"`
    A2A         serverConfig.Config `env:",prefix=A2A_"`
}
```

All A2A-specific settings are grouped under the `A2A_` prefix, making it easy to distinguish agent configuration from other application settings.

## Client Configuration

- `SERVER_URL`: A2A server URL (default: http://localhost:8080)

## Understanding the Code

### Server (`server/main.go`)

The server implements a simple echo task handler without any AI processing:

```go
func (h *SimpleTaskHandler) HandleTask(ctx context.Context, task *types.Task, message *types.Message) (*types.Task, error) {
    // Echoes back the user's message
    responseText := fmt.Sprintf("Echo: %s", userInput)
    // Updates task with response and marks as completed
    return task, nil
}
```

### Configuration (`server/config/config.go`)

The configuration module follows production patterns for consistency:

```go
type Config struct {
    Environment string              `env:"ENVIRONMENT,default=development"`
    A2A         serverConfig.Config `env:",prefix=A2A_"`
}
```

### Client (`client/main.go`)

The client demonstrates basic message sending:

```go
// Create client
a2aClient := client.NewClient(serverURL)

// Send task
response, err := a2aClient.SendTask(ctx, message)
```

## Next Steps

- Try the `ai-powered` example to see AI integration
- Check the `streaming` example for real-time responses
- Explore `artifacts` example for file handling

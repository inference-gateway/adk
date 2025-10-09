# Default Handlers A2A Example

This example demonstrates an A2A server using the **default handlers** provided by the ADK. The server uses `WithDefaultTaskHandlers()` which provides built-in task processing capabilities without requiring custom handler implementations.

## Table of Contents

- [Key Features](#key-features)
- [Architecture](#architecture)
- [Running the Example](#running-the-example)
- [AI Integration](#ai-integration)
- [How Default Handlers Work](#how-default-handlers-work)
- [Files Structure](#files-structure)
- [Troubleshooting](#troubleshooting)

## Key Features

- **Default Task Handlers**: Uses built-in handlers for both background and streaming tasks - no need to implement agent logic yourself
- **Simplified Setup**: No need to implement custom task handlers for common scenarios
- **Mock Responses**: Provides mock responses when no LLM is configured
- **AI Integration**: When an LLM is configured, the default handlers use it for intelligent responses
- **Toolbox Support**: Includes sample tools (weather, time) for AI-powered mode

## Architecture

The server is built using:

```go
serverBuilder := server.NewA2AServerBuilder(cfg.A2A, logger).
    WithDefaultTaskHandlers()
```

This approach provides:

- Default background task handler for polling scenarios (no need to implement agent logic yourself)
- Default streaming task handler for real-time responses (no need to implement streaming agent logic yourself)
- Automatic AI integration when an agent is provided
- Built-in error handling and response formatting

## Running the Example

### Prerequisites

- Go 1.25 or later
- Docker and Docker Compose (optional)

### Option 1: Using Docker Compose (Recommended)

1. **Start the services:**

   ```bash
   docker-compose up --build
   ```

2. **The client will automatically run and send test messages to the server**

### Option 2: Running Locally

1. **Start the server:**

   ```bash
   cd server
   go mod tidy
   go run main.go
   ```

2. **In another terminal, run the client:**
   ```bash
   cd client
   go mod tidy
   go run main.go
   ```

## Configuration

The server can be configured via environment variables:

| Variable                     | Description              | Default       |
| ---------------------------- | ------------------------ | ------------- |
| `ENVIRONMENT`                | Runtime environment      | `development` |
| `A2A_SERVER_PORT`            | Server port              | `8080`        |
| `A2A_DEBUG`                  | Enable debug logging     | `false`       |
| `A2A_CAPABILITIES_STREAMING` | Enable streaming support | `true`        |
| `A2A_AGENT_CLIENT_PROVIDER`  | LLM provider (optional)  | -             |
| `A2A_AGENT_CLIENT_MODEL`     | LLM model (optional)     | -             |

### Adding AI Capabilities

To enable AI-powered responses through the default handlers:

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Configure your LLM provider:

   ```bash
   # For OpenAI
   A2A_AGENT_CLIENT_PROVIDER=openai
   A2A_AGENT_CLIENT_MODEL=gpt-3.5-turbo

   # For Anthropic
   A2A_AGENT_CLIENT_PROVIDER=anthropic
   A2A_AGENT_CLIENT_MODEL=claude-3-haiku-20240307
   ```

3. Restart the server

## Expected Output

When you run the example, you should see:

**Server Output:**

```
🔧 Starting Default Handlers A2A Server...
2024/01/15 10:30:00 INFO configuration loaded
2024/01/15 10:30:00 INFO no LLM provider configured - using default handlers with mock responses
2024/01/15 10:30:00 INFO ✅ server created
2024/01/15 10:30:00 INFO 🌐 server running on port 8080
```

**Client Output:**

```
--- Request 1 ---
Sending: Hello, how are you?
Response:
{
  "id": "task-123",
  "status": {
    "state": "completed",
    "message": {
      "role": "assistant",
      "parts": [
        {
          "kind": "text",
          "text": "Hello! I'm doing well, thank you for asking. This is a response from the default task handler..."
        }
      ]
    }
  }
}
```

## Comparison with Other Examples

| Example              | Handler Type                | Use Case                                       |
| -------------------- | --------------------------- | ---------------------------------------------- |
| **default-handlers** | `WithDefaultTaskHandlers()` | Quick setup, mock responses, optional AI       |
| **ai-powered**       | Custom `AITaskHandler`      | Full AI integration with custom logic          |
| **streaming**        | Custom streaming handlers   | Real-time streaming with custom implementation |

## Files Structure

```
default-handlers/
├── README.md
├── docker-compose.yaml
├── .env.example
├── server/
│   ├── main.go
│   ├── config/config.go
│   ├── go.mod
│   └── go.sum
└── client/
    ├── main.go
    ├── go.mod
    └── go.sum

Note: Uses ../Dockerfile.server and ../Dockerfile.client for containers
```

## Troubleshooting

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server
docker compose run --rm a2a-debugger tasks list --include-history
```

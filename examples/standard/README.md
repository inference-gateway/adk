# Standard A2A Server Example

This example demonstrates a basic A2A (Agent-to-Agent) server configuration using the inference-gateway A2A ADK.

## Features

- ✅ Basic A2A server setup
- ✅ Default task and message handlers
- ✅ Health check endpoint
- ✅ Agent capabilities endpoint
- ✅ Graceful shutdown
- ✅ Basic logging

## Running the Example

```bash
cd examples/standard
go run main.go
```

The server will start on `http://localhost:8080`

## Available Endpoints

- `GET /health` - Health check endpoint
- `GET /.well-known/agent.json` - Agent capabilities and metadata
- `POST /a2a` - A2A protocol endpoint for sending messages and tasks

## Testing the Server

### Health Check

```bash
curl http://localhost:8080/health
```

### Get Agent Info

```bash
curl http://localhost:8080/.well-known/agent.json
```

### Send a Message

```bash
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "message/send",
    "id": "test-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-1",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Hello, A2A server!"
          }
        ]
      }
    }
  }'
```

## Configuration

The example uses basic configuration suitable for development:

- **Port**: 8080
- **TLS**: Disabled
- **Authentication**: Disabled
- **Task Queue Size**: 100
- **Streaming**: Enabled
- **Debug Mode**: Enabled

## Next Steps

- Check out the [Advanced Example](../advanced/) for custom handlers and enhanced features
- Review the [A2A Protocol Documentation](../../README.md) for more details
- Explore custom task processing and business logic integration

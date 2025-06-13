# A2A Server Example

This example demonstrates a basic A2A (Agent-to-Agent) server using the inference-gateway A2A framework.

## Features

- Basic A2A server setup
- Message and task handlers
- Health check endpoint
- Agent capabilities endpoint
- OpenTelemetry telemetry support
- Graceful shutdown

## Running the Example

```bash
cd examples/server
go run main.go
```

The server will start on `http://localhost:8080` with telemetry metrics on `http://localhost:9090/metrics`

## API Endpoints

- `GET /health` - Health check
- `GET /.well-known/agent.json` - Agent capabilities
- `POST /a2a` - A2A protocol endpoint
- `GET /metrics` - Prometheus metrics (port 9090)

## Testing

Send a test message:

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

Check health:

```bash
curl http://localhost:8080/health
```

View metrics:

```bash
curl http://localhost:9090/metrics
```

## Configuration

Basic development configuration:

- Port: 8080 (main server)
- Metrics Port: 9090 (telemetry)
- TLS: Disabled
- Authentication: Disabled
- Telemetry: Disabled

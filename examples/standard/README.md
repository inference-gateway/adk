# Standard A2A Server Example

This example demonstrates a basic A2A (Agent-to-Agent) server configuration using the inference-gateway A2A ADK with telemetry support.

## Features

- ✅ Basic A2A server setup
- ✅ Default task and message handlers
- ✅ Health check endpoint
- ✅ Agent capabilities endpoint
- ✅ OpenTelemetry integration with Prometheus metrics
- ✅ Request tracking and performance monitoring
- ✅ Graceful shutdown
- ✅ Basic logging

## Running the Example

```bash
cd examples/standard
go run main.go
```

The server will start on `http://localhost:8080` with telemetry metrics available on `http://localhost:9090/metrics`

## Available Endpoints

### Main Server (Port 8080)

- `GET /health` - Health check endpoint
- `GET /.well-known/agent.json` - Agent capabilities and metadata
- `POST /a2a` - A2A protocol endpoint for sending messages and tasks

### Telemetry (Port 9090)

- `GET /metrics` - Prometheus metrics endpoint

## Telemetry Features

This example includes comprehensive telemetry powered by OpenTelemetry:

- **Request Metrics**: Count and duration of all A2A requests
- **Response Monitoring**: HTTP status code tracking
- **Provider Metrics**: LLM provider and model usage statistics
- **Task Processing**: Task queue and processing metrics
- **Error Tracking**: Failed requests and task processing errors

### Available Metrics

The telemetry middleware automatically tracks:

- `a2a_requests_total` - Total number of A2A requests processed
- `a2a_response_status_total` - Total number of responses by status code
- `a2a_request_duration` - Duration of A2A request processing (histogram)
- `a2a_prompt_tokens_total` - Total prompt tokens consumed
- `a2a_completion_tokens_total` - Total completion tokens generated
- `a2a_tokens_total` - Total tokens used in A2A requests

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

### View Telemetry Metrics

```bash
curl http://localhost:9090/metrics
```

This will return Prometheus-formatted metrics including:

- Request counts and durations
- Response status codes
- Task processing metrics
- Provider/model usage statistics

### Send Multiple Requests to Generate Metrics

To see telemetry in action, send several requests and then check the metrics:

```bash
# Send a few test requests
for i in {1..5}; do
  curl -X POST http://localhost:8080/a2a \
    -H "Content-Type: application/json" \
    -d "{
      \"jsonrpc\": \"2.0\",
      \"method\": \"message/send\",
      \"id\": \"test-$i\",
      \"params\": {
        \"message\": {
          \"kind\": \"message\",
          \"messageId\": \"msg-$i\",
          \"role\": \"user\",
          \"parts\": [
            {
              \"kind\": \"text\",
              \"text\": \"Hello, A2A server! Request #$i\"
            }
          ]
        }
      }
    }"
done

# Check the metrics
curl http://localhost:9090/metrics | grep a2a_
```

## Configuration

The example uses basic configuration suitable for development with telemetry enabled:

- **Port**: 8080 (main server)
- **Metrics Port**: 9090 (telemetry)
- **TLS**: Disabled
- **Authentication**: Disabled
- **Telemetry**: **Enabled** with Prometheus metrics
- **Task Queue Size**: 100
- **Streaming**: Enabled
- **Debug Mode**: Enabled

## Next Steps

- Check out the [Advanced Example](../advanced/) for custom handlers and enhanced features
- Review the [A2A Protocol Documentation](../../README.md) for more details
- Explore custom task processing and business logic integration

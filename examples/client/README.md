# A2A Client Examples

Examples showing how to use the A2A ADK client to interact with agents using different communication patterns.

## Examples

### Async Client (`cmd/async`)

Non-blocking task submission with polling for completion.

```bash
cd cmd/async
go run main.go
```

**Best for:** Long-running tasks, batch processing, background operations

### Streaming Client (`cmd/streaming`)

Real-time communication with immediate event processing.

```bash
cd cmd/streaming
go run main.go
```

**Best for:** Interactive applications, real-time UIs, live progress updates

## Configuration

Environment variables:

| Variable            | Default                     | Description                 |
| ------------------- | --------------------------- | --------------------------- |
| `A2A_SERVER_URL`    | `http://localhost:8080/a2a` | A2A agent server URL        |
| `POLL_INTERVAL`     | `2s`                        | Polling interval (async)    |
| `MAX_POLL_TIMEOUT`  | `30s`                       | Max polling timeout (async) |
| `STREAMING_TIMEOUT` | `60s`                       | Max streaming timeout       |

## Quick Comparison

| Aspect            | Async Pattern         | Streaming Pattern          |
| ----------------- | --------------------- | -------------------------- |
| **Compatibility** | Any A2A agent         | Requires streaming support |
| **Use Case**      | Background processing | Interactive applications   |
| **Network**       | Multiple requests     | Single connection          |
| **Latency**       | Higher                | Lower                      |

## Troubleshooting

**Connection issues:**

- Check network connectivity and server URL
- Ensure agent is running at the specified URL

**Streaming not working:**

- Verify agent supports streaming capabilities
- Use async pattern as fallback

**Timeouts:**

- Increase timeout values in environment variables
- Check agent processing performance

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

### Paused Task Client (`cmd/pausedtask`)

Demonstrates handling tasks that require user input (input-required state). Shows how to:
- Monitor tasks that may pause for additional input
- Handle the input-required state
- Resume paused tasks with user-provided input
- Manage the complete pause/resume workflow

```bash
cd cmd/pausedtask
go run main.go
```

**Best for:** Interactive workflows, multi-step processes, tasks requiring user clarification

### Paused Task Streaming (`cmd/pausedtask-streaming`)

Combines paused task handling with real-time streaming. Features:
- Start streaming conversations that may pause for input
- Handle real-time streaming chunks during task execution
- Resume paused tasks with continued streaming
- Show live conversation flow throughout the process

```bash
cd cmd/pausedtask-streaming
go run main.go
```

**Best for:** Interactive streaming applications, real-time multi-step workflows, conversational agents that need user input

## Configuration

Environment variables:

| Variable            | Default                     | Description                 |
| ------------------- | --------------------------- | --------------------------- |
| `A2A_SERVER_URL`    | `http://localhost:8080`     | A2A agent server URL        |
| `POLL_INTERVAL`     | `2s`                        | Polling interval (async)    |
| `MAX_POLL_TIMEOUT`  | `30s`                       | Max polling timeout (async) |
| `STREAMING_TIMEOUT` | `60s`                       | Max streaming timeout       |

## Quick Comparison

| Aspect            | Async Pattern         | Streaming Pattern          | Paused Task Pattern          | Paused Task Streaming       |
| ----------------- | --------------------- | -------------------------- | ---------------------------- | --------------------------- |
| **Compatibility** | Any A2A agent         | Requires streaming support | Any A2A agent                | Requires streaming support  |
| **Use Case**      | Background processing | Interactive applications   | Multi-step interactions      | Real-time multi-step flows  |
| **Network**       | Multiple requests     | Single connection          | Multiple requests with pauses| Streaming with pause/resume |
| **Latency**       | Higher                | Lower                      | Variable (user-dependent)    | Low + user interaction      |
| **User Input**    | No                    | No                         | Yes (on demand)              | Yes (during streaming)      |
| **Real-time**     | No                    | Yes                        | No (polling for updates)     | Yes                         |

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

**Further debugging tips:**

There is a utility developed to allow you to troubleshot the A2A server using a client, it is called `a2a-debugger`. It can be used to list tasks, submit new tasks, get task details, and view task history:

```bash
docker run --rm --net host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks list
docker run --rm --net host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks submit "Hello, can you help me?"
docker run --rm --net host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks get <task_id>
docker run --rm --net host ghcr.io/inference-gateway/a2a-debugger:latest --server-url http://localhost:8080 tasks history <context-id>
```

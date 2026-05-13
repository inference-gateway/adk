# A2A ADK Examples

This directory contains scenario-based examples demonstrating different
capabilities of the A2A Agent Development Kit (ADK).

## Table of Contents

- [📁 Structure](#-structure)
- [🚀 Quick Start](#-quick-start)
- [📚 Available Examples](#-available-examples)
- [🔧 Configuration](#-configuration)
- [📖 Learning Path](#-learning-path)

## 📁 Structure

Each example is a self-contained scenario with:

- **Server**: A2A server implementation with task handlers
- **Client**: A2A client that demonstrates sending tasks and receiving responses
- **Configuration**: Environment-based config
- **README**: Detailed documentation and usage instructions

```text
examples/
├── minimal/                   # Basic server/client without AI (echo responses)
├── default-handlers/          # Using built-in default task handlers
├── static-agent-card/         # Loading agent config from JSON file
├── ai-powered/                # Server with LLM integration
├── ai-powered-streaming/      # AI with real-time streaming
├── streaming/                 # Real-time streaming responses
├── input-required/            # Input-required flow (non-streaming and streaming)
├── callbacks/                 # Callback hooks for agent/model/tool lifecycle
├── artifacts-filesystem/      # Artifact storage using local filesystem
├── artifacts-minio/           # Artifact storage using MinIO (S3-compatible)
├── artifacts-autonomous-tool/ # LLM autonomously creates artifacts via create_artifact tool
├── queue-storage/             # Queue storage backends (in-memory and Redis)
├── tls-example/               # TLS-enabled server with HTTPS communication
├── usage-metadata/            # Token usage and execution metrics tracking
└── protocol-methods/          # Full A2A JSON-RPC surface (cancel, list, pushNotificationConfig, resubscribe, authenticated card)
```

## 🚀 Quick Start

### Running Any Example

1. Navigate to the example directory:

```bash
cd examples/minimal
```

2. Run with Docker Compose:

```bash
docker-compose up --build
```

3. Or run locally:

```bash
# Terminal 1 - Server
cd server && go run main.go

# Terminal 2 - Client
cd client && go run main.go
```

## 📚 Available Examples

### Core Examples

#### `minimal/`

The simplest A2A server and client setup with custom echo task handler.

- Custom `TaskHandler` implementation
- Basic request/response pattern
- No external dependencies

#### `default-handlers/`

Server using built-in default task handlers - no need to implement custom handlers.

- `WithDefaultTaskHandlers()` for quick setup
- Automatic mock responses (no LLM required)
- Optional AI integration when LLM is configured

#### `static-agent-card/`

Demonstrates loading agent configuration from JSON files using `WithAgentCardFromFile()`.

- Agent metadata defined in `agent-card.json`
- Runtime field overrides (URLs, ports)
- Environment-specific configurations

#### `ai-powered/`

Custom AI task handler with LLM integration (OpenAI, Anthropic, etc.).

- Custom `AITaskHandler` implementation
- Multiple provider support
- Environment-based LLM configuration

#### `streaming/`

Real-time streaming responses for chat-like experiences.

- Custom `StreamableTaskHandler` implementation
- Character-by-character streaming
- Event-based communication

#### `ai-powered-streaming/`

AI-powered streaming with LLM integration.

- Real-time AI responses
- Streaming LLM integration
- Event-driven architecture

#### `input-required/`

Demonstrates input-required flow where agents pause to request additional information.

- **Non-streaming**: Traditional request-response with input pausing
- **Streaming**: Real-time streaming that can pause for user input
- Task state management and conversation continuity
- Built-in `input_required` tool usage
- Interactive conversation examples

#### `callbacks/`

Demonstrates callback hooks for intercepting and modifying agent execution lifecycle.

- `BeforeAgent`/`AfterAgent`: Hook into overall agent execution
- `BeforeModel`/`AfterModel`: Hook into LLM calls for caching, guardrails
- `BeforeTool`/`AfterTool`: Hook into tool execution for authorization, logging
- Flow control: skip default behavior or modify outputs
- Use cases: guardrails, caching, logging, authorization, sanitization

#### `artifacts-filesystem/`

Demonstrates artifact creation and download using local filesystem storage.

- Filesystem storage provider
- HTTP download endpoints
- Client artifact download integration

#### `artifacts-minio/`

Demonstrates artifact creation and download using MinIO (S3-compatible) storage.

- MinIO storage provider
- S3-compatible API
- Enterprise-ready cloud storage

#### `artifacts-autonomous-tool/`

Demonstrates autonomous artifact creation where the LLM decides when and what artifacts to create using the built-in `create_artifact` tool.

- LLM autonomously creates artifacts
- Built-in `create_artifact` tool enabled via configuration
- No custom task handler required
- AI-driven decision making for artifact creation
- Supports multiple file types (JSON, CSV, code files, etc.)

#### `queue-storage/`

Demonstrates different queue storage backends for task management and horizontal
scaling.

- **In-Memory**: Simple development setup with in-memory storage
- **Redis**: Enterprise-ready Redis-based queue storage
- Docker Compose setups for both storage backends
- Complete server and client implementations

#### `tls-example/`

TLS-enabled A2A server demonstrating secure HTTPS communication.

- Self-signed certificate generation
- TLS/SSL encryption for client-server communication
- Docker Compose orchestration with TLS setup
- Secure task submission and response handling

#### `usage-metadata/`

Demonstrates automatic token usage and execution metrics tracking in task responses.

- Token usage tracking (prompt_tokens, completion_tokens, total_tokens)
- Execution statistics (iterations, messages, tool_calls, failed_tools)
- Automatic metadata population in Task.Metadata field
- Configuration options for enabling/disabling usage tracking
- Cost monitoring and performance analysis use cases

#### `protocol-methods/`

End-to-end walk-through of every A2A JSON-RPC method beyond `message/send`,
`message/stream`, and `tasks/get`. Useful as a reference when you need to
call one of these methods directly from your own client.

- `tasks/cancel` against an in-flight task
- `tasks/list` with paginated iteration
- `tasks/pushNotificationConfig/{set,get,list,delete}` round-trip
- `tasks/resubscribe` after a dropped streaming connection
- `agent/getAuthenticatedExtendedCard` via JSON-RPC

## 🔧 Configuration

All examples follow a consistent environment variable pattern with the `A2A_` prefix:

### Common A2A Variables

- `A2A_SERVER_PORT`: Server port (default: `8080`)
- `A2A_DEBUG`: Enable debug logging (default: `false`)
- `A2A_AGENT_NAME`: Agent identifier
- `A2A_AGENT_DESCRIPTION`: Agent description
- `A2A_AGENT_VERSION`: Agent version

### AI/LLM Configuration

For examples with AI integration:

- `A2A_AGENT_CLIENT_PROVIDER`: LLM provider (`openai`, `anthropic`)
- `A2A_AGENT_CLIENT_MODEL`: Model to use (`gpt-4`, `claude-3-haiku-20240307`)
- `A2A_AGENT_CLIENT_BASE_URL`: Custom gateway URL (optional)

See each example's README for specific configuration details.

## 📖 Learning Path

**Recommended progression:**

1. **`minimal/`** - Understand basic A2A protocol and custom task handlers
2. **`default-handlers/`** - Learn built-in handlers for rapid development
3. **`static-agent-card/`** - Externalize agent configuration to JSON files
4. **`input-required/`** - Learn input-required flow for interactive conversations
5. **`callbacks/`** - Hook into agent lifecycle for guardrails, caching, and logging
6. **`artifacts-filesystem/`** - Add file generation and download capabilities
7. **`ai-powered/`** - Add LLM integration for intelligent responses
8. **`streaming/`** - Implement real-time streaming capabilities
9. **`ai-powered-streaming/`** - Combine AI integration with real-time streaming
10. **`artifacts-autonomous-tool/`** - Enable LLM to autonomously create artifacts
11. **`artifacts-minio/`** - Enterprise-ready artifact storage with MinIO
12. **`queue-storage/`** - Learn different queue storage backends for scaling
13. **`tls-example/`** - Learn TLS/SSL encryption and secure communication
14. **`usage-metadata/`** - Track token usage and execution metrics for cost monitoring
15. **`protocol-methods/`** - Exercise the rest of the A2A JSON-RPC surface (cancel, list, pushNotificationConfig, resubscribe, authenticated card)

---

For detailed setup instructions, configuration options, and troubleshooting, see
each example's individual README file.

For more information about the A2A protocol and framework, see the main
[README](../README.md) or refer to the
[official documentation](https://google.github.io/adk-docs/).

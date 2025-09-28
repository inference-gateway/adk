# A2A ADK Examples

This directory contains scenario-based examples demonstrating different capabilities of the A2A Agent Development Kit (ADK).

## 📁 Structure

Each example is a self-contained scenario with:

- **Server**: A2A server implementation with task handlers
- **Client**: Go client that demonstrates sending tasks and receiving responses
- **Configuration**: Environment-based config following production patterns
- **README**: Detailed documentation and usage instructions

```
examples/
├── minimal/              # Basic server/client without AI (echo responses)
├── default-handlers/     # Using built-in default task handlers
├── static-agent-card/    # Loading agent config from JSON file
├── ai-powered/           # Server with LLM integration
├── ai-powered-streaming/ # AI with real-time streaming
├── streaming/            # Real-time streaming responses
└── tls-example/          # TLS-enabled server with HTTPS communication
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

### Learning Path

**Start Here:**

#### `minimal/`

The simplest A2A server and client setup with custom echo task handler.

- Custom `TaskHandler` implementation
- Basic request/response pattern
- No external dependencies
- Production-style configuration with `A2A_` environment variables

#### `default-handlers/`

Server using built-in default task handlers - no need to implement custom handlers.

- `WithDefaultTaskHandlers()` for quick setup
- Automatic mock responses (no LLM required)
- Optional AI integration when LLM is configured
- Built-in error handling and response formatting

#### `static-agent-card/`

Demonstrates loading agent configuration from JSON files using `WithAgentCardFromFile()`.

- Agent metadata defined in `agent-card.json`
- Runtime field overrides (URLs, ports)
- Environment-specific configurations
- Version-controlled agent definitions

**Advanced Examples:**

#### `ai-powered/`

Custom AI task handler with LLM integration (OpenAI, Anthropic, etc.).

- Custom `AITaskHandler` implementation
- Multiple provider support
- Environment-based LLM configuration
- Background task processing

#### `streaming/`

Real-time streaming responses for chat-like experiences.

- Custom `StreamableTaskHandler` implementation
- Character-by-character streaming
- Event-based communication (`DeltaStreamEvent`, `StatusStreamEvent`)
- Mock and AI modes

#### `ai-powered-streaming/`

AI-powered streaming with LLM integration.

- Real-time AI responses
- Streaming LLM integration
- Event-driven architecture

#### `tls-example/`

TLS-enabled A2A server demonstrating secure HTTPS communication.

- Self-signed certificate generation
- TLS/SSL encryption for client-server communication
- Docker Compose orchestration with TLS setup
- Secure task submission and response handling

## 🔧 Configuration

All examples follow a consistent environment variable pattern with the `A2A_` prefix:

### Common A2A Variables

- `ENVIRONMENT`: Runtime environment (default: `development`)
- `A2A_SERVER_PORT`: Server port (default: `8080`)
- `A2A_DEBUG`: Enable debug logging (default: `false`)
- `A2A_AGENT_NAME`: Agent identifier
- `A2A_AGENT_DESCRIPTION`: Agent description
- `A2A_AGENT_VERSION`: Agent version
- `A2A_CAPABILITIES_STREAMING`: Enable streaming support
- `A2A_CAPABILITIES_PUSH_NOTIFICATIONS`: Enable push notifications

### AI/LLM Configuration

For examples with AI integration:

- `A2A_AGENT_CLIENT_PROVIDER`: LLM provider (`openai`, `anthropic`)
- `A2A_AGENT_CLIENT_MODEL`: Model to use (`gpt-4`, `claude-3-haiku-20240307`)
- `A2A_AGENT_CLIENT_BASE_URL`: Custom gateway URL (optional)

### Example-Specific Variables

- `A2A_AGENT_CARD_FILE`: Path to agent card JSON file (`static-agent-card` example)

## 🐳 Docker Support

Most examples include:

- Multi-stage Docker files for optimized images
- Docker Compose for easy orchestration
- Network isolation between services
- Go 1.25+ base images

## 📖 Learning Path

1. **`minimal/`** - Understand basic A2A protocol and custom task handlers
2. **`default-handlers/`** - Learn built-in handlers for rapid development
3. **`static-agent-card/`** - Externalize agent configuration to JSON files
4. **`tls-example/`** - Learn TLS/SSL encryption and secure communication
5. **`ai-powered/`** - Add LLM integration for intelligent responses
6. **`ai-powered-streaming/`** - Combine AI integration with real-time streaming
7. **`streaming/`** - Implement real-time streaming capabilities

## Documentation

For more detailed information about the A2A protocol and framework, see the main [README](../README.md).

# A2A ADK Examples

This directory contains scenario-based examples demonstrating different capabilities of the A2A Agent Development Kit (ADK).

## 📁 New Structure

Each example is a self-contained scenario with:

- **Server**: A2A server implementation
- **Client**: Matching client that interacts with the server
- **Docker Compose**: Ready-to-run containerized setup
- **README**: Detailed documentation for the scenario

```
examples/
├── minimal/           # Basic server/client without AI
├── ai-powered/        # Server with LLM integration
├── streaming/         # Real-time streaming responses
└── ...
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

### Basic Examples

#### `minimal/`

Basic A2A server and client without AI integration. Perfect for understanding the core protocol.

- Mock task handler
- Simple request/response
- No external dependencies

#### `ai-powered/`

A2A server integrated with AI language models (OpenAI, Anthropic, etc.).

- LLM integration
- Multiple provider support
- Environment-based configuration

#### `streaming/`

Real-time streaming responses for chat-like experiences.

- Character-by-character streaming
- Event-based communication
- Mock and AI modes

## 🔧 Configuration

Most examples use environment variables for configuration:

### Common Variables

- `PORT`: Server port (default: 8080)
- `AGENT_NAME`: Agent identifier
- `LOG_LEVEL`: Logging verbosity

### AI Configuration

- `AGENT_CLIENT_API_KEY`: Your LLM API key
- `AGENT_CLIENT_PROVIDER`: Provider (openai, anthropic)
- `AGENT_CLIENT_MODEL`: Model to use
- `INFERENCE_GATEWAY_URL`: Custom gateway URL

## 🐳 Docker Support

All examples include:

- Multi-stage Dockerfiles for small images
- Docker Compose for easy orchestration
- Network isolation between services
- Golang 1.25 base images

## 📖 Learning Path

1. Start with `minimal/` to understand basics
2. Move to `ai-powered/` for LLM integration
3. Try `streaming/` for real-time features

## Documentation

For more detailed information about the A2A protocol and framework, see the main [README](../README.md).

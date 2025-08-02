# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

This is a Go project that uses Task for build automation. Common commands:

- `task test` - Run all tests with coverage (`go test -v -cover ./...`)
- `task lint` - Run Go static analysis and linting (`golangci-lint run`)
- `task tidy` - Tidy all Go modules
- `task clean` - Remove build artifacts

### A2A Schema Management

- `task a2a:download-schema` - Download latest A2A schema from GitHub
- `task a2a:generate-types` - Generate Go types from A2A schema (creates `types/generated_types.go`)

### Mock Generation

- `task generate:mocks` - Generate all mocks using counterfeiter
- `task clean:mocks` - Clean up generated mocks

## Architecture Overview

This is the **A2A ADK (Agent Development Kit)** - a Go library for building Agent-to-Agent (A2A) protocol compatible agents. The A2A protocol enables AI agents to communicate, delegate tasks, and share capabilities.

### Core Components

**Server Architecture (`adk/server/`):**
- `server.go` - Main A2AServer interface and implementation with HTTP endpoints
- `server_builder.go` - Builder pattern for creating configured servers
- `agent_builder.go` - Builder pattern for creating OpenAI-compatible agents
- `task_manager.go` - Task lifecycle management, queuing, and persistence
- `message_handler.go` - A2A protocol message processing
- `config/config.go` - Environment-based configuration management

**Client Architecture (`adk/client/`):**
- `client.go` - A2A protocol client with retry logic and streaming support

**Key Interfaces:**
- `A2AServer` - Main server interface with Start/Stop, task processing
- `A2AClient` - Client interface for communicating with A2A servers
- `TaskHandler` - Custom task processing logic
- `TaskManager` - Task lifecycle and state management
- `OpenAICompatibleAgent` - LLM integration interface

### A2A Protocol Implementation

The server implements these A2A JSON-RPC methods:
- `message/send` - Send tasks to agents
- `message/stream` - Stream responses in real-time
- `tasks/get` - Retrieve task status
- `tasks/list` - List tasks with filtering
- `tasks/cancel` - Cancel running tasks
- `tasks/pushNotificationConfig/*` - Webhook notifications

HTTP endpoints:
- `POST /a2a` - Main A2A protocol endpoint
- `GET /.well-known/agent.json` - Agent capabilities discovery
- `GET /health` - Health check

### Configuration System

Uses `github.com/sethvargo/go-envconfig` for environment-based configuration:

**Key Environment Variables:**
- `AGENT_NAME` - Agent identifier
- `INFERENCE_GATEWAY_URL` - Inference Gateway URL (configures AGENT_CLIENT_BASE_URL)
- `AGENT_CLIENT_PROVIDER` - LLM provider (openai, anthropic, etc.)
- `AGENT_CLIENT_MODEL` - Model name
- `CAPABILITIES_STREAMING` - Enable streaming support
- `AUTH_ENABLE` - Enable OIDC authentication
- `TELEMETRY_ENABLE` - Enable OpenTelemetry metrics

### Testing Approach

Uses table-driven tests with generated mocks:
- Test files: `*_test.go`
- Mocks: `adk/server/mocks/` (generated via counterfeiter)
- Comprehensive coverage for HTTP endpoints, task processing, and client operations

### Dependencies

**Key Dependencies:**
- `github.com/gin-gonic/gin` - HTTP server framework
- `github.com/inference-gateway/sdk` - LLM provider integration
- `go.uber.org/zap` - Structured logging
- `github.com/sethvargo/go-envconfig` - Configuration management
- `go.opentelemetry.io/otel` - Observability and metrics
- `github.com/stretchr/testify` - Testing framework

### Development Patterns

**Builder Pattern Usage:**
- `NewA2AServerBuilder()` - Fluent server configuration
- `NewAgentBuilder()` - Fluent agent configuration with LLM clients

**Error Handling:**
- Structured error responses following JSON-RPC specification
- Comprehensive logging with context

**Concurrency:**
- Background task processing with goroutines
- Context-aware request handling
- Graceful shutdown support

## Important Notes

- Always run `task a2a:generate-types` after schema updates
- The project follows Go module structure with `go.mod` at root
- Generated types are in `types/generated_types.go` (do not edit manually)
- Configuration supports both defaults and environment overrides
- Server supports both mock mode (no LLM) and AI-powered mode
- Always ensure on each given task that you push the changes to a branch and open a PR for review

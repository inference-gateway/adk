<h1 align="center">Agent Development Kit (ADK)</h1>

<p align="center">
  <strong>Build powerful, interoperable AI agents with the Agent-to-Agent (A2A) protocol</strong>
</p>

> ⚠️ **Early Stage Warning**: This project is in its early stages of development. Breaking changes are expected as the API evolves and improves. Please use pinned versions in production environments and be prepared to update your code when upgrading versions.

<p align="center">
  <!-- CI Status Badge -->
  <a href="https://github.com/inference-gateway/adk/actions/workflows/ci.yml?query=branch%3Amain">
    <img src="https://github.com/inference-gateway/adk/actions/workflows/ci.yml/badge.svg?branch=main" alt="CI Status"/>
  </a>
  <!-- Release Workflow Badge -->
  <a href="https://github.com/inference-gateway/adk/actions/workflows/release.yml">
    <img src="https://github.com/inference-gateway/adk/actions/workflows/release.yml/badge.svg" alt="Release"/>
  </a>
  <!-- Version Badge -->
  <a href="https://github.com/inference-gateway/adk/releases">
    <img src="https://img.shields.io/github/v/release/inference-gateway/adk?color=blue&style=flat-square" alt="Version"/>
  </a>
  <!-- License Badge -->
  <a href="https://github.com/inference-gateway/adk/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/inference-gateway/adk?color=blue&style=flat-square" alt="License"/>
  </a>
  <!-- Go Version -->
  <img src="https://img.shields.io/github/go-mod/go-version/inference-gateway/adk?style=flat-square" alt="Go Version"/>
  <!-- Go Report Card -->
  <a href="https://goreportcard.com/report/github.com/inference-gateway/adk">
    <img src="https://goreportcard.com/badge/github.com/inference-gateway/adk?style=flat-square" alt="Go Report Card"/>
  </a>
</p>

---

## Table of Contents

- [Overview](#overview)
  - [What is A2A?](#what-is-a2a)
- [🚀 Quick Start](#-quick-start)
  - [Installation](#installation)
  - [Examples](#examples)
- [✨ Key Features](#-key-features)
- [🛠️ Development](#️-development)
- [📖 API Reference](#-api-reference)
  - [Core Components](#core-components)
  - [Configuration](#configuration)
- [🔧 Advanced Usage](#-advanced-usage)
- [🌐 A2A Ecosystem](#-a2a-ecosystem)
- [📋 Requirements](#-requirements)
- [🐳 Docker Support](#-docker-support)
- [📄 License](#-license)
- [🤝 Contributing](#-contributing)
- [📞 Support](#-support)
- [🔗 Resources](#-resources)

---

## Overview

The **A2A ADK (Agent Development Kit)** is a Go library that simplifies building [Agent-to-Agent (A2A) protocol](https://github.com/inference-gateway/schemas/tree/main/a2a) compatible agents. A2A enables seamless communication between AI agents, allowing them to collaborate, delegate tasks, and share capabilities across different systems and providers.

### What is A2A?

Agent-to-Agent (A2A) is a standardized protocol that enables AI agents to:

- **Communicate** with each other using a unified JSON-RPC interface
- **Delegate tasks** to specialized agents with specific capabilities
- **Stream responses** in real-time for better user experience
- **Authenticate** securely using OIDC/OAuth2
- **Discover capabilities** through standardized agent cards

## 🚀 Quick Start

### Installation

```bash
go get github.com/inference-gateway/adk
```

### Examples

For complete working examples, see the [examples](./examples/) directory:

- **[Minimal](./examples/minimal/)** - Basic A2A server without AI capabilities
- **[AI-Powered](./examples/ai-powered/)** - Full A2A server with LLM integration
- **[AI-Powered Streaming](./examples/ai-powered-streaming/)** - AI agent with streaming capabilities
- **[Callbacks](./examples/callbacks/)** - Lifecycle hooks for guardrails, caching, and logging
- **[Default Handlers](./examples/default-handlers/)** - Built-in task processing
- **[Static Agent Card](./examples/static-agent-card/)** - JSON-based agent metadata management
- **[Streaming](./examples/streaming/)** - Real-time streaming responses
- **[Input Required](./examples/input-required/)** - Interactive conversation with input pausing
- **[Artifacts - MinIO](./examples/artifacts-minio/)** - MinIO cloud storage with direct/proxy download modes
- **[Artifacts - Filesystem](./examples/artifacts-filesystem/)** - Filesystem-based artifact storage
- **[Artifacts - Autonomous Tool](./examples/artifacts-autonomous-tool/)** - Autonomous tool for artifact creation
- **[Artifacts - Default Handlers](./examples/artifacts-with-default-handlers/)** - Artifacts with default handlers
- **[Queue Storage](./examples/queue-storage/)** - Memory and Redis storage backends
- **[TLS Server](./examples/tls-server/)** - Secure HTTPS with TLS configuration
- **[Usage Metadata](./examples/usage-metadata/)** - Token usage and execution metrics tracking

#### Getting Started

To run any example:

```bash
cd examples/minimal/server
go run main.go
```

Each example includes its own README with setup instructions and usage details.

## ✨ Key Features

### Core Capabilities

- 🤖 **A2A Protocol Compliance**: Full implementation of the Agent-to-Agent communication standard
- 🔌 **Multi-Provider Support**: Works with OpenAI, Ollama, Groq, Cohere, and other LLM providers
- 🌊 **Real-time Streaming**: Stream responses as they're generated from language models
- 🔧 **Custom Tools**: Easy integration of custom tools and capabilities
- 🪝 **Callback Hooks**: Lifecycle hooks for agent, model, and tool execution with flow control
- 📎 **File Artifacts**: Support for downloadable file artifacts with filesystem and MinIO storage backends
- 🔐 **Secure Authentication**: Built-in OIDC/OAuth2 authentication support
- 📨 **Push Notifications**: Webhook notifications for real-time task state updates
- ⏸️ **Task Pausing**: Built-in support for input-required state pausing and resumption
- 🗄️ **Multiple Storage Backends**: Support for in-memory and Redis storage with horizontal scaling
- 📊 **Usage Metadata**: Automatic tracking of LLM token consumption and execution metrics

### Developer Experience

- ⚙️ **Environment Configuration**: Simple setup through environment variables
- 📊 **Task Management**: Built-in task queuing, polling, and lifecycle management
- 🏗️ **Extensible Architecture**: Pluggable components for custom business logic
- 📚 **Type-Safe**: Generated types from A2A schema for compile-time safety
- 🧪 **Well Tested**: Comprehensive test coverage with table-driven tests

### Enterprise Ready

- 🌿 **Lightweight**: Optimized binary size for efficient deployment
- 🛡️ **Production Hardened**: Configurable timeouts, TLS support, and error handling
- ☸️ **Cloud Native**: Ready for cloud-native deployments and orchestration
- 📊 **Observability**: OpenTelemetry integration for monitoring and tracing

## 🛠️ Development

### Quick Setup

```bash
# Clone the repository
git clone https://github.com/inference-gateway/adk.git
cd adk

# Install dependencies
go mod download

# Install pre-commit hook
task precommit:install
```

### Essential Tasks

| Task                       | Description                               |
| -------------------------- | ----------------------------------------- |
| `task a2a:download-schema` | Download the latest A2A schema            |
| `task a2a:generate-types`  | Generate Go types from A2A schema         |
| `task lint`                | Run linting and code quality checks       |
| `task test`                | Run all tests                             |
| `task precommit:install`   | Install Git pre-commit hook (recommended) |

### Build-Time Agent Metadata

The ADK supports injecting agent metadata at build time using Go linker flags (LD flags). This makes agent information immutable and embedded in the binary, which is useful for production deployments.

#### Available LD Flags

The following build-time metadata variables can be set via LD flags:

- **`BuildAgentName`** - The agent's display name
- **`BuildAgentDescription`** - A description of the agent's capabilities
- **`BuildAgentVersion`** - The agent's version number

#### Usage Examples

**Simple A2A Server Example:**

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	zap "go.uber.org/zap"

	server "github.com/inference-gateway/adk/server"
	config "github.com/inference-gateway/adk/server/config"
	types "github.com/inference-gateway/adk/types"
)

func main() {
	fmt.Println("🤖 Starting Simple A2A Server...")

	// Initialize logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatalf("failed to create logger: %v", err)
	}
	defer logger.Sync()

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Configuration
	cfg := config.Config{
		AgentName:        "simple-agent",
		AgentDescription: "A simple A2A server with default handlers",
		AgentVersion:     "0.1.0",
		Debug:            true,
		QueueConfig: config.QueueConfig{
			CleanupInterval: 5 * time.Minute,
		},
		ServerConfig: config.ServerConfig{
			Port: port,
		},
	}

	// Build and start server with default handlers
	a2aServer, err := server.NewA2AServerBuilder(cfg, logger).
		WithDefaultTaskHandlers().
		WithAgentCard(types.AgentCard{
			Name:            cfg.AgentName,
			Description:     cfg.AgentDescription,
			Version:         cfg.AgentVersion,
			URL:             fmt.Sprintf("http://localhost:%s", port),
			ProtocolVersion: "0.3.0",
			Capabilities: types.AgentCapabilities{
				Streaming:              &[]bool{true}[0],
				PushNotifications:      &[]bool{false}[0],
				StateTransitionHistory: &[]bool{false}[0],
			},
			DefaultInputModes:  []string{"text/plain"},
			DefaultOutputModes: []string{"text/plain"},
			Skills:             []types.AgentSkill{},
		}).
		Build()
	if err != nil {
		logger.Fatal("failed to create A2A server", zap.Error(err))
	}

	logger.Info("✅ server created")

	// Start server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := a2aServer.Start(ctx); err != nil {
			logger.Fatal("server failed to start", zap.Error(err))
		}
	}()

	logger.Info("🌐 server running on port " + port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("🛑 shutting down...")

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := a2aServer.Stop(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	} else {
		logger.Info("✅ goodbye!")
	}
}
```

See the [Docker Support](#-docker-support) section for containerized builds.

---

**For detailed development workflows, testing guidelines, and contribution processes, see the [Contributing Guide](./CONTRIBUTING.md).**

## 📖 API Reference

### Core Components

#### A2AServer

The main server interface that handles A2A protocol communication. See [server examples](./examples/) for complete implementation details.

#### A2AServerBuilder

Build A2A servers with custom configurations using a fluent interface. The builder provides methods for:

- `WithAgent()` - Configure AI agent integration
- `WithDefaultTaskHandlers()` - Use built-in task processing
- `WithBackgroundTaskHandler()` - Custom background task handling
- `WithStreamingTaskHandler()` - Custom streaming task handling
- `WithAgentCardFromFile()` - Load agent metadata from JSON

See [examples](./examples/) for complete usage patterns.

#### Task Handler Interfaces

The ADK provides two distinct interfaces for handling tasks:

- **`TaskHandler`** - For background/polling scenarios (message/send)
- **`StreamableTaskHandler`** - For real-time streaming scenarios (message/stream)

Streaming handlers require an agent to be configured. See [task handler examples](./examples/) for implementation details.

#### AgentBuilder

Build OpenAI-compatible agents using a fluent interface. Supports:

- Custom LLM clients
- System prompts and conversation limits
- Tool integration
- Callback hooks (BeforeAgent, AfterAgent, BeforeModel, AfterModel, BeforeTool, AfterTool)
- Configuration management

See [AI-powered examples](./examples/ai-powered/) and [callback examples](./examples/callbacks/) for complete agent setup.

#### A2AClient

Client interface for communicating with A2A servers. Supports:

- Task sending and streaming
- Health monitoring
- Agent card retrieval
- Custom configuration

See [client examples](./examples/client/) for usage patterns.

#### Agent Health Monitoring

Monitor agent operational status with three health states:

- `healthy`: Fully operational
- `degraded`: Partially operational
- `unhealthy`: Not operational

See [client examples](./examples/) for implementation.

### LLM Client

Create OpenAI-compatible LLM clients for agent integration. See [AI examples](./examples/ai-powered/) for setup details.

### Configuration

Configure your A2A agent using environment variables. All configuration is optional and includes sensible defaults.

#### Core Server Configuration

| Variable                           | Default                        | Description                                |
| ---------------------------------- | ------------------------------ | ------------------------------------------ |
| `PORT`                             | `8080`                         | Server port                                |
| `DEBUG`                            | `false`                        | Enable debug logging                       |
| `AGENT_URL`                        | `http://helloworld-agent:8080` | Agent URL for internal references          |
| `STREAMING_STATUS_UPDATE_INTERVAL` | `1s`                           | How often to send streaming status updates |

#### Agent & LLM Configuration

| Variable                                      | Default | Description                                  |
| --------------------------------------------- | ------- | -------------------------------------------- |
| `AGENT_CLIENT_PROVIDER`                       | -       | LLM provider (openai, anthropic, groq, etc.) |
| `AGENT_CLIENT_MODEL`                          | -       | Model name (e.g., `openai/gpt-4`)            |
| `AGENT_CLIENT_BASE_URL`                       | -       | Custom LLM endpoint URL                      |
| `AGENT_CLIENT_API_KEY`                        | -       | API key for LLM provider                     |
| `AGENT_CLIENT_TIMEOUT`                        | `30s`   | Request timeout                              |
| `AGENT_CLIENT_MAX_RETRIES`                    | `3`     | Maximum retry attempts                       |
| `AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS` | `50`    | Max chat completion rounds                   |
| `AGENT_CLIENT_MAX_TOKENS`                     | `4096`  | Maximum tokens per response                  |
| `AGENT_CLIENT_TEMPERATURE`                    | `0.7`   | LLM temperature (0.0-2.0)                    |
| `AGENT_CLIENT_SYSTEM_PROMPT`                  | -       | System prompt for the agent                  |
| `AGENT_CLIENT_ENABLE_USAGE_METADATA`          | `true`  | Track token usage and execution metrics      |

#### Agent Capabilities

| Variable                                | Default | Description                  |
| --------------------------------------- | ------- | ---------------------------- |
| `CAPABILITIES_STREAMING`                | `true`  | Enable streaming responses   |
| `CAPABILITIES_PUSH_NOTIFICATIONS`       | `false` | Enable webhook notifications |
| `CAPABILITIES_STATE_TRANSITION_HISTORY` | `false` | Track state changes          |

#### Authentication (Optional)

| Variable             | Default | Description                |
| -------------------- | ------- | -------------------------- |
| `AUTH_ENABLE`        | `false` | Enable OIDC authentication |
| `AUTH_ISSUER_URL`    | -       | OIDC issuer URL            |
| `AUTH_CLIENT_ID`     | -       | OIDC client ID             |
| `AUTH_CLIENT_SECRET` | -       | OIDC client secret         |

#### Task Management

| Variable                             | Default | Description                                 |
| ------------------------------------ | ------- | ------------------------------------------- |
| `TASK_RETENTION_MAX_COMPLETED_TASKS` | `100`   | Max completed tasks to keep (0 = unlimited) |
| `TASK_RETENTION_MAX_FAILED_TASKS`    | `50`    | Max failed tasks to keep (0 = unlimited)    |
| `TASK_RETENTION_CLEANUP_INTERVAL`    | `5m`    | Cleanup frequency (0 = manual only)         |

#### Storage Configuration (Optional)

| Variable                 | Default  | Description                                      |
| ------------------------ | -------- | ------------------------------------------------ |
| `QUEUE_PROVIDER`         | `memory` | Storage backend: `memory` or `redis`             |
| `QUEUE_URL`              | -        | Redis connection URL (required when using Redis) |
| `QUEUE_MAX_SIZE`         | `100`    | Maximum queue size                               |
| `QUEUE_CLEANUP_INTERVAL` | `120s`   | How often to clean up completed tasks            |

**Storage Backends:**

- **Memory Storage (Default)**: Fast in-memory storage for development and single-instance deployments
- **Redis Storage**: Persistent storage with horizontal scaling support for production deployments

**Redis Configuration Examples:**

```bash
# Basic Redis setup
export QUEUE_PROVIDER=redis
export QUEUE_URL=redis://localhost:6379

# Redis with authentication
export QUEUE_URL=redis://:password@localhost:6379
export QUEUE_URL=redis://username:password@localhost:6379

# Redis with specific database
export QUEUE_URL=redis://localhost:6379/1

# Redis with TLS (Redis 6.0+)
export QUEUE_URL=rediss://username:password@redis.example.com:6380/0
```

#### Artifacts Configuration (Optional)

Enable file artifacts support for downloadable files generated by your agent:

| Variable                               | Default            | Description                              |
| -------------------------------------- | ------------------ | ---------------------------------------- |
| `ARTIFACTS_ENABLE`                     | `false`            | Enable artifacts support                 |
| `ARTIFACTS_SERVER_HOST`                | `localhost`        | Artifacts server host                    |
| `ARTIFACTS_SERVER_PORT`                | `8081`             | Artifacts server port                    |
| `ARTIFACTS_STORAGE_PROVIDER`           | `filesystem`       | Storage backend: `filesystem` or `minio` |
| `ARTIFACTS_STORAGE_BASE_PATH`          | `./artifacts`      | Base path for filesystem storage         |
| `ARTIFACTS_STORAGE_BASE_URL`           | _(auto-generated)_ | Override base URL for direct downloads   |
| `ARTIFACTS_STORAGE_ENDPOINT`           | -                  | MinIO/S3 endpoint URL                    |
| `ARTIFACTS_STORAGE_ACCESS_KEY`         | -                  | MinIO/S3 access key                      |
| `ARTIFACTS_STORAGE_SECRET_KEY`         | -                  | MinIO/S3 secret key                      |
| `ARTIFACTS_STORAGE_BUCKET_NAME`        | `artifacts`        | MinIO/S3 bucket name                     |
| `ARTIFACTS_STORAGE_USE_SSL`            | `true`             | Use SSL for MinIO/S3 connections         |
| `ARTIFACTS_RETENTION_MAX_ARTIFACTS`    | `5`                | Max artifacts per task (0 = unlimited)   |
| `ARTIFACTS_RETENTION_MAX_AGE`          | `7d`               | Max artifact age (0 = no age limit)      |
| `ARTIFACTS_RETENTION_CLEANUP_INTERVAL` | `24h`              | Cleanup frequency (0 = manual only)      |

**Storage Backends:**

- **Filesystem Storage (Default)**: Store artifacts locally on disk, suitable for single-instance deployments
- **MinIO Storage**: Cloud-native object storage with S3 compatibility, ideal for distributed deployments

**Download Modes:**

- **Proxy Mode (Default)**: Downloads go through the artifacts server (port 8081) with authentication and logging
- **Direct Mode**: Configure `ARTIFACTS_STORAGE_BASE_URL` to enable direct downloads from storage backend

**MinIO Configuration Example:**

```bash
# Enable artifacts with MinIO storage
export ARTIFACTS_ENABLE=true
export ARTIFACTS_STORAGE_PROVIDER=minio
export ARTIFACTS_STORAGE_ENDPOINT=localhost:9000
export ARTIFACTS_STORAGE_ACCESS_KEY=minioadmin
export ARTIFACTS_STORAGE_SECRET_KEY=minioadmin
export ARTIFACTS_STORAGE_USE_SSL=false

# Optional: Enable direct downloads (bypasses artifacts server)
export ARTIFACTS_STORAGE_BASE_URL=http://localhost:9000
```

**Benefits of Redis Storage:**

- ✅ **Persistent Tasks** - Tasks survive server restarts
- ✅ **Distributed Processing** - Multiple server instances can share the same queue
- ✅ **High Performance** - Redis provides fast task queuing and retrieval
- ✅ **Task History** - Completed and failed tasks are retained based on configuration
- ✅ **Horizontal Scaling** - Scale to N number of A2A servers processing the same queue

#### TLS Configuration (Optional)

| Variable               | Default | Description             |
| ---------------------- | ------- | ----------------------- |
| `SERVER_TLS_ENABLE`    | `false` | Enable TLS/HTTPS        |
| `SERVER_TLS_CERT_PATH` | -       | Path to TLS certificate |
| `SERVER_TLS_KEY_PATH`  | -       | Path to TLS private key |

#### Telemetry (Optional)

| Variable                 | Default     | Description              |
| ------------------------ | ----------- | ------------------------ |
| `TELEMETRY_ENABLE`       | `false`     | Enable OpenTelemetry     |
| `TELEMETRY_ENDPOINT`     | -           | OTLP endpoint URL        |
| `TELEMETRY_SERVICE_NAME` | `a2a-agent` | Service name for tracing |

#### Example Configuration

See [configuration examples](./examples/) for complete setup patterns, including environment variables, custom config structs, and programmatic overrides.

## 🔧 Advanced Usage

For detailed implementation examples and patterns, see the [examples](./examples/) directory:

- **[Custom Tools](./examples/ai-powered/)** - Creating and integrating custom tools
- **[Callback Hooks](./examples/callbacks/)** - Lifecycle hooks for guardrails, caching, and flow control
- **[Agent Configuration](./examples/static-agent-card/)** - JSON-based agent metadata management
- **[Task Handling](./examples/default-handlers/)** - Built-in and custom task processing
- **[Streaming](./examples/streaming/)** - Real-time response handling
- **[Usage Metadata](./examples/usage-metadata/)** - Tracking token consumption and execution metrics for cost monitoring and performance analysis

## 🌐 A2A Ecosystem

This ADK is part of the broader Inference Gateway ecosystem:

### Related Projects

- **[Inference Gateway](https://github.com/inference-gateway/inference-gateway)** - Unified API gateway for AI providers
- **[Go SDK](https://github.com/inference-gateway/go-sdk)** - Go client library for Inference Gateway
- **[TypeScript SDK](https://github.com/inference-gateway/typescript-sdk)** - TypeScript/JavaScript client library
- **[Python SDK](https://github.com/inference-gateway/python-sdk)** - Python client library
- **[Rust SDK](https://github.com/inference-gateway/rust-sdk)** - Rust client library
- **[Rust ADK](https://github.com/inference-gateway/rust-adk)** - Rust A2A Development Kit

### A2A Agents

- **[Awesome A2A](https://github.com/inference-gateway/awesome-a2a)** - Curated list of A2A-compatible agents
- **[Browser Agent](https://github.com/inference-gateway/browser-agent)** - Web browser automation and interaction agent
- **[Documentation Agent](https://github.com/inference-gateway/documentation-agent)** - Documentation generation and management agent
- **[Google Calendar Agent](https://github.com/inference-gateway/google-calendar-agent)** - Google Calendar integration agent
- **[n8n Agent](https://github.com/inference-gateway/n8n-agent)** - n8n workflow automation integration agent

## 📋 Requirements

- **Go**: 1.25 or later
- **Dependencies**: See [go.mod](./go.mod) for full dependency list

## 🐳 Docker Support

Build and run your A2A agent application in a container. Here's an example Dockerfile for an application using the ADK:

```dockerfile
FROM golang:1.25-alpine AS builder

# Build arguments for agent metadata
ARG AGENT_NAME="My A2A Agent"
ARG AGENT_DESCRIPTION="A custom A2A agent built with the ADK"
ARG AGENT_VERSION="0.1.0"

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && \
    go build -ldflags "-X 'github.com/inference-gateway/adk/server.BuildAgentName=${AGENT_NAME}' -X 'github.com/inference-gateway/adk/server.BuildAgentDescription=${AGENT_DESCRIPTION}' -X 'github.com/inference-gateway/adk/server.BuildAgentVersion=${AGENT_VERSION}'" -o bin/agent .

FROM alpine:latest
RUN apk --no-cache add ca-certificates && \
    addgroup -g 1001 -S a2a && \
    adduser -u 1001 -S agent -G a2a
WORKDIR /home/agent
COPY --from=builder /app/bin/agent .
RUN chown agent:a2a ./agent
USER agent
CMD ["./agent"]
```

**Build with custom metadata:**

```bash
docker build \
  --build-arg AGENT_NAME="Weather Assistant" \
  --build-arg AGENT_DESCRIPTION="AI-powered weather forecasting agent" \
  --build-arg AGENT_VERSION="0.1.1" \
  -t my-a2a-agent .
```

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](./LICENSE) file for details.

## 🤝 Contributing

Contributions to the A2A ADK are welcome! Whether you're fixing bugs, adding features, improving documentation, or helping with testing, your contributions make the project better for everyone.

**Please see the [Contributing Guide](./CONTRIBUTING.md) for:**

- 🚀 **Getting Started** - Development environment setup and prerequisites
- 📋 **Development Workflow** - Step-by-step development process and tools
- 🎯 **Coding Guidelines** - Code style, testing patterns, and best practices
- 🛠️ **Making Changes** - Branch naming, commit format, and submission process
- 🧪 **Testing Guidelines** - Test structure, mocking, and coverage requirements
- 🔄 **Pull Request Process** - Review process and submission checklist

**Quick Start for Contributors:**

```bash
# Fork the repo and clone it
git clone https://github.com/your-username/adk.git
cd adk

# Install pre-commit hook
task precommit:install
```

For questions or help getting started, please [open a discussion](https://github.com/inference-gateway/adk/discussions) or check out the [contributing guide](./CONTRIBUTING.md).

## 📞 Support

### Issues & Questions

- **Bug Reports**: [GitHub Issues](https://github.com/inference-gateway/adk/issues)
- **Documentation**: [Official Docs](https://docs.inference-gateway.com)

## 🔗 Resources

### Documentation

- [A2A Protocol Specification](https://github.com/inference-gateway/schemas/tree/main/a2a)
- [API Documentation](https://docs.inference-gateway.com/a2a)

---

<p align="center">
  <a href="https://github.com/inference-gateway">GitHub</a> •
  <a href="https://docs.inference-gateway.com">Documentation</a>
</p>

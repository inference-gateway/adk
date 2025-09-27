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
└── streaming/            # Real-time streaming responses
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
4. **`ai-powered/`** - Add LLM integration for intelligent responses
5. **`ai-powered-streaming/`** - Combine AI integration with real-time streaming
6. **`streaming/`** - Implement real-time streaming capabilities

## Artifacts Server Storage Options

The ADK provides two storage providers for artifacts:

### 1. Filesystem Storage

**Use Case**: Local development, testing, simple deployments
**Benefits**:

- Simple setup and configuration
- No external dependencies
- Direct file system access
- Suitable for single-server deployments

**Example**: See `server/cmd/artifacts-server/` and `server/cmd/integrated-artifacts/`

```go
artifactsServer := server.NewArtifactsServerBuilder(cfg, logger).
    WithFilesystemStorage("./artifacts", "http://localhost:8081").
    Build()
```

### 2. MinIO Storage (S3-Compatible)

**Use Case**: Production deployments, scalable applications, cloud-native environments
**Benefits**:

- S3-compatible API
- Horizontal scalability
- Built-in data protection (erasure coding)
- Versioning and lifecycle management
- Multi-cloud compatibility
- Enterprise features (encryption, access control)

**Example**: See `server/cmd/minio-artifacts/`

```go
artifactsServer := server.NewArtifactsServerBuilder(cfg, logger).
    WithMinIOStorage(
        "minio:9000",        // endpoint
        "admin",             // access key
        "password123",       // secret key
        "artifacts",         // bucket
        false,               // use SSL
        "http://localhost:8081", // base URL
    ).
    Build()
```

## Server Examples

### Client Examples (`client/`)

Examples demonstrating how to use the A2A client library:

- **`cmd/artifacts/`** - Download artifacts from A2A servers
- **`cmd/async/`** - Asynchronous task processing
- **`cmd/healthcheck/`** - Health check monitoring
- **`cmd/listtasks/`** - Task listing and filtering
- **`cmd/pausedtask/`** - Handling paused tasks
- **`cmd/pausedtask-streaming/`** - Streaming paused tasks
- **`cmd/streaming/`** - Real-time streaming responses

### Server Examples (`server/`)

Examples showing different A2A server configurations:

- **`cmd/minimal/`** - Minimal A2A server setup
- **`cmd/aipowered/`** - AI-powered agent with LLM integration
- **`cmd/pausedtask/`** - Server handling paused tasks
- **`cmd/travelplanner/`** - Complex travel planning agent
- **`cmd/artifacts-server/`** - Standalone artifacts server (filesystem)
- **`cmd/artifacts/`** - Basic artifact creation
- **`cmd/integrated-artifacts/`** - A2A + artifacts integration (filesystem)
- **`cmd/minio-artifacts/`** - MinIO-based artifact storage (NEW)

## Running Examples Locally

### Prerequisites

```bash
# Install Go dependencies
task tidy

# Install development tools (optional)
task precommit:install
```

### Filesystem Storage Example

```bash
# Terminal 1: Start standalone artifacts server
cd examples/server
go run cmd/artifacts-server/main.go

# Terminal 2: Start integrated A2A + artifacts server
go run cmd/integrated-artifacts/main.go

# Terminal 3: Test with client
cd ../client
go run cmd/artifacts/main.go
```

### MinIO Storage Example

```bash
# Start MinIO (or use docker-compose)
docker run -d \
  -p 9000:9000 -p 9001:9001 \
  -e MINIO_ROOT_USER=admin \
  -e MINIO_ROOT_PASSWORD=password123 \
  minio/minio server /data --console-address ":9001"

# Create bucket
docker run --rm -it \
  --add-host=host.docker.internal:host-gateway \
  minio/mc alias set local http://host.docker.internal:9000 admin password123
docker run --rm -it \
  --add-host=host.docker.internal:host-gateway \
  minio/mc mb local/artifacts

# Start A2A server with MinIO
cd examples/server
go run cmd/minio-artifacts/main.go
```

## Storage Provider Comparison

| Feature              | Filesystem            | MinIO               |
| -------------------- | --------------------- | ------------------- |
| **Setup Complexity** | Simple                | Moderate            |
| **Scalability**      | Single server         | Horizontal          |
| **Data Protection**  | File system dependent | Erasure coding      |
| **Cloud Native**     | No                    | Yes                 |
| **S3 Compatible**    | No                    | Yes                 |
| **Multi-Server**     | Shared storage needed | Built-in clustering |
| **Versioning**       | Manual                | Built-in            |
| **Access Control**   | File permissions      | IAM policies        |
| **Best For**         | Development, Testing  | Production, Scale   |

## Configuration

### Environment Variables

Both storage providers support environment-based configuration:

#### Filesystem Storage

```bash
ARTIFACTS_ENABLE=true
ARTIFACTS_STORAGE_PROVIDER=filesystem
ARTIFACTS_STORAGE_BASE_PATH=./artifacts
ARTIFACTS_SERVER_PORT=8081
```

#### MinIO Storage

```bash
ARTIFACTS_ENABLE=true
ARTIFACTS_STORAGE_PROVIDER=minio
ARTIFACTS_STORAGE_MINIO_ENDPOINT=localhost:9000
ARTIFACTS_STORAGE_MINIO_ACCESS_KEY=admin
ARTIFACTS_STORAGE_MINIO_SECRET_KEY=password123
ARTIFACTS_STORAGE_MINIO_BUCKET=artifacts
ARTIFACTS_STORAGE_MINIO_USE_SSL=false
ARTIFACTS_SERVER_PORT=8081
```

## API Usage

All examples provide the same A2A protocol endpoints:

```bash
# Send a task that creates artifacts
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
            "text": "Create a downloadable analysis report"
          }
        ]
      }
    }
  }'

# Download artifacts directly
curl -O http://localhost:8081/artifacts/TASK_ID/filename.ext
```

## Development

### Adding New Storage Providers

1. Implement the `ArtifactStorageProvider` interface
2. Add configuration options to `config.ArtifactsStorageConfig`
3. Update the artifacts server builder with `WithXXXStorage()` method
4. Add example in `cmd/xxx-artifacts/`
5. Update this README

### Testing

```bash
# Run all tests
task test

# Run specific example tests
cd examples/server
go test ./...
```

### Code Quality

```bash
# Format and lint
task format
task lint

# Pre-commit checks
task precommit:install
git commit  # Runs automatic checks
```

## Troubleshooting

### Common Issues

1. **MinIO Connection Failed**: Ensure MinIO is running and accessible
2. **Bucket Not Found**: Create the bucket using MinIO console or mc client
3. **Permission Denied**: Check MinIO access keys and bucket policies
4. **Port Conflicts**: Ensure ports 8080, 8081, 9000, 9001 are available

### Debug Mode

Enable debug logging:

```bash
export LOG_LEVEL=debug
go run cmd/minio-artifacts/main.go
```

### Health Checks

Check service health:

```bash
# A2A Server
curl http://localhost:8080/health

# Artifacts Server
curl http://localhost:8081/health

# MinIO
curl http://localhost:9000/minio/health/live
```

## Next Steps

1. Choose your storage provider based on requirements
2. Start with Docker Compose for quick evaluation
3. Customize examples for your specific use case
4. Deploy to your target environment
5. Monitor and scale as needed

For more information, see the [main ADK documentation](../README.md).

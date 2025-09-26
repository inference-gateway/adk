# ADK Examples

This directory contains comprehensive examples demonstrating how to use the A2A Agent Development Kit (ADK) with different configurations and storage providers.

## Quick Start with Docker Compose

The easiest way to try the examples is using Docker Compose, which provides pre-configured setups for different storage scenarios.

### Filesystem Storage (Simple Local Development)

```bash
# Start A2A server with local filesystem storage
docker-compose --profile filesystem up -d

# Access the services
# - A2A Server: http://localhost:8080
# - Artifacts Server: http://localhost:8081
# - Artifacts stored in: ./artifacts directory
```

### MinIO Storage (Production-Ready Cloud Storage)

```bash
# Start A2A server with MinIO S3-compatible storage
docker-compose --profile minio up -d

# Access the services
# - A2A Server: http://localhost:8080
# - Artifacts Server: http://localhost:8081  
# - MinIO API: http://localhost:9000
# - MinIO Console: http://localhost:9001 (admin/password123)
```

### MinIO Only (External Storage)

```bash
# Start only MinIO for use with external A2A servers
docker-compose up minio minio-init -d

# MinIO will be available for external servers to connect to
```

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

| Feature | Filesystem | MinIO |
|---------|------------|-------|
| **Setup Complexity** | Simple | Moderate |
| **Scalability** | Single server | Horizontal |
| **Data Protection** | File system dependent | Erasure coding |
| **Cloud Native** | No | Yes |
| **S3 Compatible** | No | Yes |
| **Multi-Server** | Shared storage needed | Built-in clustering |
| **Versioning** | Manual | Built-in |
| **Access Control** | File permissions | IAM policies |
| **Best For** | Development, Testing | Production, Scale |

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

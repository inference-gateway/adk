# Artifacts S3 Example (RustFS)

This example demonstrates an A2A server that creates downloadable artifacts using S3-compatible object
storage. The server generates analysis reports as markdown files and makes them available for
download via HTTP endpoints. The docker-compose setup uses [RustFS](https://github.com/rustfs/rustfs)
as the S3-compatible object store.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Running the Example](#running-the-example)
- [Configuration](#configuration)
- [Download Modes](#download-modes)
- [What Happens](#what-happens)
- [Generated Artifacts](#generated-artifacts)
- [API Endpoints](#api-endpoints)
- [Example Usage](#example-usage)
- [Troubleshooting](#troubleshooting)
- [RustFS Benefits](#rustfs-benefits)
- [Next Steps](#next-steps)

## Features

- **S3-Compatible Object Storage**: Stores artifacts in RustFS object storage for scalability and distributed access
- **Automatic Artifact Creation**: Generates markdown reports for user requests
- **HTTP Download API**: Serves artifacts via REST endpoints (`GET /artifacts/{artifactId}/{filename}`)
- **Client Integration**: Demonstrates how to download artifacts from A2A responses stored in RustFS
- **Docker Support**: Full containerized setup with docker-compose including RustFS
- **Bucket Management**: The server creates the `artifacts` bucket automatically on startup

## Architecture

This example supports two download modes for artifacts stored in object storage:

### Proxy Mode (Default)

```text
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                  │    │                 │    │                 │
│  A2A Client     │◄──►│  A2A Server      │◄──►│ Artifacts Server│◄──►│ RustFS Storage  │
│  (Downloads)    │    │  (Port 8080)     │    │  (Port 8081)    │    │  (Port 9000)    │
│                 │    │                  │    │                 │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘    └─────────────────┘
```

### Direct Mode (Configurable)

```text
┌─────────────────┐                              ┌─────────────────┐
│                 │                              │                 │
│  A2A Client     │◄────────────────────────────►│ RustFS Storage  │
│  (Downloads)    │                              │  (Port 9000)    │
│                 │                              │                 │
└─────────────────┘                              └─────────────────┘
```

## Running the Example

### Option 1: Docker Compose (Recommended)

```bash
# Start the services (includes RustFS, A2A server, and client)
docker-compose up --build

# The setup will automatically:
# 1. Start RustFS object storage service
# 2. Start the A2A server with the S3 storage provider (it creates the 'artifacts' bucket)
# 3. Run the client which will:
#    - Send a request for an analysis report
#    - Wait for the server to process it and store in RustFS
#    - Download the generated artifact from RustFS
#    - Save it to the client/downloads/ directory
```

### Option 2: Local Development

```bash
# Terminal 1: Start RustFS (requires Docker)
docker run -d -p 9000:9000 -p 9001:9001 rustfs/rustfs:1.0.4

# Terminal 2: Start the server
cd server
A2A_ARTIFACTS_STORAGE_PROVIDER=s3 \
A2A_ARTIFACTS_STORAGE_ENDPOINT=localhost:9000 \
A2A_ARTIFACTS_STORAGE_ACCESS_KEY=rustfsadmin \
A2A_ARTIFACTS_STORAGE_SECRET_KEY=rustfsadmin \
A2A_ARTIFACTS_STORAGE_USE_SSL=false \
go run main.go

# Terminal 3: Run the client
cd client
go run main.go
```

The server creates the `artifacts` bucket on startup when it does not exist.

## Configuration

The server can be configured via environment variables:

| Variable                            | Default                           | Description                            |
| ----------------------------------- | --------------------------------- | -------------------------------------- |
| `A2A_AGENT_NAME`                    | `artifacts-s3-agent`              | Agent name                             |
| `A2A_AGENT_DESCRIPTION`             | `An agent that creates...`        | Agent description                      |
| `A2A_AGENT_VERSION`                 | `0.1.0`                           | Agent version                          |
| `A2A_SERVER_PORT`                   | `8080`                            | A2A server port                        |
| `A2A_ARTIFACTS_ENABLED`             | `true`                            | Enable artifacts support               |
| `A2A_ARTIFACTS_SERVER_HOST`         | `localhost`                       | Artifacts server host                  |
| `A2A_ARTIFACTS_SERVER_PORT`         | `8081`                            | Artifacts server port                  |
| `A2A_ARTIFACTS_STORAGE_PROVIDER`    | `s3`                              | Storage provider                       |
| `A2A_ARTIFACTS_STORAGE_ENDPOINT`    | `localhost:9000`                  | Object storage endpoint                |
| `A2A_ARTIFACTS_STORAGE_ACCESS_KEY`  | `rustfsadmin`                     | Storage access key                     |
| `A2A_ARTIFACTS_STORAGE_SECRET_KEY`  | `rustfsadmin`                     | Storage secret key                     |
| `A2A_ARTIFACTS_STORAGE_BUCKET_NAME` | `artifacts`                       | Storage bucket name                    |
| `A2A_ARTIFACTS_STORAGE_USE_SSL`     | `false`                           | Use SSL for storage                    |
| `A2A_ARTIFACTS_STORAGE_BASE_URL`    | `http://server:8081` (proxy mode) | Override base URL for direct downloads |

Client configuration:

| Variable        | Default                 | Description                            |
| --------------- | ----------------------- | -------------------------------------- |
| `SERVER_URL`    | `http://localhost:8080` | A2A server URL                         |
| `ARTIFACTS_URL` | `http://localhost:8081` | Artifacts server URL                   |
| `DOWNLOADS_DIR` | `downloads`             | Directory to save downloaded artifacts |

## Download Modes

This example demonstrates two different approaches for downloading artifacts from RustFS:

### Proxy Mode

**Flow**: Client → Artifacts Server → RustFS Storage

In proxy mode, the artifacts server acts as an intermediary:

- ✅ **Authentication**: Server can enforce access control and authentication
- ✅ **Rate Limiting**: Server can implement rate limiting and throttling
- ✅ **Audit Logging**: All downloads are logged through the artifacts server
- ✅ **Error Handling**: Unified error responses and retry logic
- ❌ **Performance**: Additional network hop adds latency
- ❌ **Bandwidth**: All data flows through the artifacts server

**Configuration**: Uses auto-generated URLs pointing to the artifacts server (port 8081).

### Direct Mode

**Flow**: Client → RustFS Storage (direct)

In direct mode, clients download directly from the object store:

- ✅ **Performance**: Direct connection to storage eliminates proxy overhead
- ✅ **Bandwidth**: Optimal network utilization
- ✅ **Scalability**: RustFS handles download traffic directly
- ❌ **Security**: RBAC must be configured on the RustFS bucket itself
- ❌ **Monitoring**: Download activity not tracked by artifacts server

**Configuration**: Override the base URL to point directly to RustFS:

```yaml
# Enable direct downloads from RustFS
A2A_ARTIFACTS_STORAGE_BASE_URL: http://rustfs:9000

# Ensure the bucket allows anonymous downloads
# (configure a public read policy on the `artifacts` bucket via the RustFS
# console or any S3 client)
```

**Important**: When using direct mode, access control must be handled at the storage level since the artifacts server is bypassed.

## What Happens

1. **RustFS Setup**: RustFS object storage starts
2. **Client Request**: The client sends a message requesting an analysis report
3. **Server Processing**: The server creates a markdown analysis report
4. **RustFS Storage**: The report is stored in the RustFS bucket using the S3 storage provider
5. **Response**: The server responds with the task containing artifact metadata and URLs
6. **Artifact Download**: The client downloads the artifact from RustFS via the artifacts server
7. **Local Storage**: The downloaded file is saved to the `client/downloads/` directory

## Generated Artifacts

The server generates markdown reports that include:

- User request summary
- Timestamp and task ID
- Sample analysis content with storage-specific information
- Storage backend details (RustFS bucket, storage class, etc.)
- Conclusions about cloud storage capabilities

Example output structure:

```text
client/downloads/
          └── analysis_report.md  # Downloaded artifact from RustFS
```

## API Endpoints

### A2A Server (Port 8080)

- `POST /a2a` - Main A2A protocol endpoint
- `GET /.well-known/agent-card.json` - Agent capabilities discovery
- `GET /health` - Health check

### Artifacts Server (Port 8081)

- `GET /artifacts/{artifactId}/{filename}` - Download artifact from RustFS
- `GET /health` - Health check

### RustFS Web Console (Port 9001)

- `http://localhost:9001` - RustFS web console (admin: rustfsadmin/rustfsadmin)
- Browse buckets, objects, and manage storage

## Example Usage

### Direct API Testing

```bash
# Create a task with artifact
curl -X POST http://localhost:8080/a2a \
  -H "Content-Type: application/json" \
  -d '{
    "jsonrpc": "2.0",
    "method": "SendMessage",
    "id": "test-1",
    "params": {
      "message": {
        "kind": "message",
        "messageId": "msg-1",
        "role": "user",
        "parts": [
          {
            "kind": "text",
            "text": "Create a detailed analysis report about renewable energy with RustFS object storage"
          }
        ]
      }
    }
  }'

# Download the artifact (replace ARTIFACT_ID with actual artifact ID)
curl -O http://localhost:8081/artifacts/ARTIFACT_ID/analysis_report.md
```

### RustFS Console Access

1. Open <http://localhost:9001> in your browser
2. Login with: **rustfsadmin** / **rustfsadmin**
3. Browse the 'artifacts' bucket to see stored files
4. View object metadata, download files, and manage permissions

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 8080, 8081, 9000, and 9001 are available
2. **Storage Connection**: Check that RustFS is running and accessible on port 9000
3. **Bucket Creation**: Verify the 'artifacts' bucket exists in the RustFS console
4. **Build Errors**: Run `go mod tidy` in both server and client directories

### Debug Mode

Enable debug logging:

```bash
export A2A_DEBUG=true
go run main.go
```

### Health Checks

```bash
# Check A2A server
curl http://localhost:8080/health

# Check artifacts server
curl http://localhost:8081/health

# Check RustFS health
curl http://localhost:9000/health
```

### RustFS Troubleshooting

```bash
# Check RustFS status via console
open http://localhost:9001

# Check the S3 API with any S3 client, for example the AWS CLI
aws --endpoint-url http://localhost:9000 s3api list-objects-v2 \
  --bucket artifacts --no-verify-ssl
```

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server
docker compose run --rm a2a-debugger tasks list --include-artifacts
```

## RustFS Benefits

This example demonstrates several advantages of using RustFS for artifact storage:

- **Open Source**: Apache 2.0 licensed, actively maintained object storage
- **S3 Compatibility**: Works with existing S3-compatible tools and SDKs
- **Web Console**: Browser-based management interface
- **Access Control**: Fine-grained bucket and object permissions
- **Performance**: High-performance object storage built in Rust
- **Cloud Native**: Runs on Docker, Kubernetes (Helm charts), and distributed topologies

## Next Steps

- Explore the [filesystem artifacts example](../artifacts-filesystem/) for local storage
- See [ADK documentation](../../README.md) for more features
- Try [streaming examples](../streaming/) for real-time responses
- Configure RustFS with custom access policies and TLS
- Run RustFS in distributed mode for production deployments

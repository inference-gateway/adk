# Artifacts MinIO Example

This example demonstrates an A2A server that creates downloadable artifacts using MinIO cloud storage. The server generates analysis reports as markdown files and makes them available for download via HTTP endpoints using MinIO as the storage backend.

## Features

- **MinIO Cloud Storage**: Stores artifacts in MinIO object storage for scalability and distributed access
- **Automatic Artifact Creation**: Generates markdown reports for user requests
- **HTTP Download API**: Serves artifacts via REST endpoints (`GET /artifacts/{artifactId}/{filename}`)
- **Client Integration**: Demonstrates how to download artifacts from A2A responses stored in MinIO
- **Docker Support**: Full containerized setup with docker-compose including MinIO service
- **Bucket Management**: Automatic bucket creation and configuration
- **Object Versioning**: Supports MinIO's object versioning capabilities

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│                 │    │                  │    │                 │    │                 │
│  A2A Client     │◄──►│  A2A Server      │◄──►│ Artifacts Server│◄──►│ MinIO Storage   │
│  (Port 8080)    │    │  (Port 8080)     │    │  (Port 8081)    │    │  (Port 9000)    │
│                 │    │                  │    │                 │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘    └─────────────────┘
                                │                        │                        │
                                ▼                        ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐    ┌─────────────────┐
                       │                  │    │                 │    │                 │
                       │  Task Processing │    │ Artifact API    │    │ MinIO Buckets   │
                       │  & Artifact      │    │ Management      │    │ - artifacts/    │
                       │  Creation        │    │                 │    │ - Versioning    │
                       │                  │    │                 │    │ - Access Control│
                       └──────────────────┘    └─────────────────┘    └─────────────────┘
```

## Running the Example

### Option 1: Docker Compose (Recommended)

```bash
# Start the services (includes MinIO, A2A server, and client)
docker-compose up --build

# The setup will automatically:
# 1. Start MinIO object storage service
# 2. Create the 'artifacts' bucket with public read access
# 3. Start the A2A server with MinIO storage backend
# 4. Run the client which will:
#    - Send a request for an analysis report
#    - Wait for the server to process it and store in MinIO
#    - Download the generated artifact from MinIO
#    - Save it to the client/downloads/ directory
```

### Option 2: Local Development

```bash
# Terminal 1: Start MinIO (requires Docker)
docker run -p 9000:9000 -p 9001:9001 \
  -e "MINIO_ROOT_USER=minioadmin" \
  -e "MINIO_ROOT_PASSWORD=minioadmin" \
  minio/minio server /data --console-address ":9001"

# Terminal 2: Create MinIO bucket
docker run --rm --network host \
  -e MC_HOST_minio=http://minioadmin:minioadmin@localhost:9000 \
  minio/mc /bin/sh -c \
  "mc alias set minio http://localhost:9000 minioadmin minioadmin; \
   mc mb minio/artifacts; \
   mc policy set public minio/artifacts"

# Terminal 3: Start the server
cd server
go run main.go

# Terminal 4: Run the client
cd client
go run main.go
```

## Configuration

The server can be configured via environment variables:

| Variable                            | Default                    | Description              |
| ----------------------------------- | -------------------------- | ------------------------ |
| `A2A_AGENT_NAME`                    | `artifacts-minio-agent`    | Agent name               |
| `A2A_AGENT_DESCRIPTION`             | `An agent that creates...` | Agent description        |
| `A2A_AGENT_VERSION`                 | `0.1.0`                    | Agent version            |
| `A2A_SERVER_PORT`                   | `8080`                     | A2A server port          |
| `A2A_ARTIFACTS_ENABLE`              | `true`                     | Enable artifacts support |
| `A2A_ARTIFACTS_SERVER_HOST`         | `localhost`                | Artifacts server host    |
| `A2A_ARTIFACTS_SERVER_PORT`         | `8081`                     | Artifacts server port    |
| `A2A_ARTIFACTS_STORAGE_PROVIDER`    | `minio`                    | Storage provider         |
| `A2A_ARTIFACTS_STORAGE_ENDPOINT`    | `localhost:9000`           | MinIO endpoint           |
| `A2A_ARTIFACTS_STORAGE_ACCESS_KEY`  | `minioadmin`               | MinIO access key         |
| `A2A_ARTIFACTS_STORAGE_SECRET_KEY`  | `minioadmin`               | MinIO secret key         |
| `A2A_ARTIFACTS_STORAGE_BUCKET_NAME` | `artifacts`                | MinIO bucket name        |
| `A2A_ARTIFACTS_STORAGE_USE_SSL`     | `false`                    | Use SSL for MinIO        |

Client configuration:

| Variable        | Default                 | Description                            |
| --------------- | ----------------------- | -------------------------------------- |
| `SERVER_URL`    | `http://localhost:8080` | A2A server URL                         |
| `ARTIFACTS_URL` | `http://localhost:8081` | Artifacts server URL                   |
| `DOWNLOADS_DIR` | `downloads`             | Directory to save downloaded artifacts |

## What Happens

1. **MinIO Setup**: MinIO object storage starts and creates the 'artifacts' bucket
2. **Client Request**: The client sends a message requesting an analysis report
3. **Server Processing**: The server creates a markdown analysis report
4. **MinIO Storage**: The report is stored in MinIO bucket using the cloud storage provider
5. **Response**: The server responds with the task containing artifact metadata and MinIO URLs
6. **Artifact Download**: The client downloads the artifact from MinIO via the artifacts server
7. **Local Storage**: The downloaded file is saved to the `client/downloads/` directory

## Generated Artifacts

The server generates markdown reports that include:

- User request summary
- Timestamp and task ID
- Sample analysis content with MinIO-specific information
- Storage backend details (MinIO bucket, versioning, etc.)
- Conclusions about cloud storage capabilities

Example output structure:

```
client/downloads/
          └── analysis_report.md  # Downloaded artifact from MinIO
```

## API Endpoints

### A2A Server (Port 8080)

- `POST /a2a` - Main A2A protocol endpoint
- `GET /.well-known/agent-card.json` - Agent capabilities discovery
- `GET /health` - Health check

### Artifacts Server (Port 8081)

- `GET /artifacts/{artifactId}/{filename}` - Download artifact from MinIO
- `GET /health` - Health check

### MinIO Console (Port 9001)

- `http://localhost:9001` - MinIO web console (admin: minioadmin/minioadmin)
- Browse buckets, objects, and manage storage

## Example Usage

### Direct API Testing

```bash
# Create a task with artifact
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
            "text": "Create a detailed analysis report about renewable energy with MinIO cloud storage"
          }
        ]
      }
    }
  }'

# Download the artifact (replace ARTIFACT_ID with actual artifact ID)
curl -O http://localhost:8081/artifacts/ARTIFACT_ID/analysis_report.md
```

### MinIO Console Access

1. Open http://localhost:9001 in your browser
2. Login with: **minioadmin** / **minioadmin**
3. Browse the 'artifacts' bucket to see stored files
4. View object metadata, download files, and manage permissions

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 8080, 8081, 9000, and 9001 are available
2. **MinIO Connection**: Check that MinIO is running and accessible on port 9000
3. **Bucket Creation**: Verify the 'artifacts' bucket exists in MinIO console
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

# Check MinIO health
curl http://localhost:9000/minio/health/live
```

### MinIO Troubleshooting

```bash
# Check MinIO status via console
open http://localhost:9001

# Check bucket existence using mc (MinIO client)
docker run --rm --network host minio/mc \
  mc ls minio/artifacts --insecure

# Check bucket policy
docker run --rm --network host minio/mc \
  mc policy get minio/artifacts --insecure
```

## MinIO Benefits

This example demonstrates several advantages of using MinIO for artifact storage:

- **Scalability**: Distributed object storage that scales horizontally
- **S3 Compatibility**: Works with existing S3-compatible tools and SDKs
- **Versioning**: Built-in object versioning for artifact history
- **Access Control**: Fine-grained bucket and object permissions
- **Web Console**: Browser-based management interface
- **Performance**: High-performance object storage optimized for cloud workloads
- **Cost Effective**: Open-source alternative to cloud storage services

## Next Steps

- Explore the [filesystem artifacts example](../artifacts-filesystem/) for local storage
- See [ADK documentation](../../README.md) for more features
- Try [streaming examples](../streaming/) for real-time responses
- Configure MinIO with custom access policies and encryption
- Set up MinIO in distributed mode for production deployments

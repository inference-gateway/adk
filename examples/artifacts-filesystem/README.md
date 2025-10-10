# Artifacts Filesystem Example

This example demonstrates an A2A server that creates downloadable artifacts using filesystem storage. The server generates analysis reports as markdown files and makes them available for download via HTTP endpoints.

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Running the Example](#running-the-example)
- [Configuration](#configuration)
- [What Happens](#what-happens)
- [Generated Artifacts](#generated-artifacts)
- [API Endpoints](#api-endpoints)
- [Example Usage](#example-usage)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## Features

- **Filesystem Storage**: Stores artifacts locally on the file system
- **Automatic Artifact Creation**: Generates markdown reports for user requests
- **HTTP Download API**: Serves artifacts via REST endpoints (`GET /artifacts/{artifactId}/{filename}`)
- **Client Integration**: Demonstrates how to download artifacts from A2A responses
- **Docker Support**: Full containerized setup with docker-compose

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│                 │    │                  │    │                 │
│  A2A Client     │◄──►│  A2A Server      │◄──►│ Artifacts Server│
│  (Port 8080)    │    │  (Port 8080)     │    │  (Port 8081)    │
│                 │    │                  │    │                 │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │                        │
                                ▼                        ▼
                       ┌──────────────────┐    ┌─────────────────┐
                       │                  │    │                 │
                       │  Task Processing │    │ Filesystem      │
                       │  & Artifact      │    │ Storage         │
                       │  Creation        │    │ ./artifacts/    │
                       │                  │    │                 │
                       └──────────────────┘    └─────────────────┘
```

## Running the Example

### Option 1: Docker Compose (Recommended)

```bash
# Start the services
docker-compose up --build

# The client will automatically:
# 1. Send a request for an analysis report
# 2. Wait for the server to process it
# 3. Download the generated artifact
# 4. Save it to the client/downloads/ directory
```

### Option 2: Local Development

```bash
# Terminal 1: Start the server
cd server
go run main.go

# Terminal 2: Run the client
cd client
go run main.go
```

## Configuration

The server can be configured via environment variables:

| Variable                          | Default                      | Description              |
| --------------------------------- | ---------------------------- | ------------------------ |
| `A2A_AGENT_NAME`                  | `artifacts-filesystem-agent` | Agent name               |
| `A2A_AGENT_DESCRIPTION`           | `An agent that creates...`   | Agent description        |
| `A2A_AGENT_VERSION`               | `0.1.0`                      | Agent version            |
| `A2A_SERVER_PORT`                 | `8080`                       | A2A server port          |
| `A2A_ARTIFACTS_ENABLE`            | `true`                       | Enable artifacts support |
| `A2A_ARTIFACTS_SERVER_HOST`       | `localhost`                  | Artifacts server host    |
| `A2A_ARTIFACTS_SERVER_PORT`       | `8081`                       | Artifacts server port    |
| `A2A_ARTIFACTS_STORAGE_PROVIDER`  | `filesystem`                 | Storage provider         |
| `A2A_ARTIFACTS_STORAGE_BASE_PATH` | `./artifacts`                | Base path for artifacts  |

Client configuration:

| Variable        | Default                 | Description                            |
| --------------- | ----------------------- | -------------------------------------- |
| `SERVER_URL`    | `http://localhost:8080` | A2A server URL                         |
| `ARTIFACTS_URL` | `http://localhost:8081` | Artifacts server URL                   |
| `DOWNLOADS_DIR` | `downloads`             | Directory to save downloaded artifacts |

## What Happens

1. **Client Request**: The client sends a message requesting an analysis report
2. **Server Processing**: The server creates a markdown analysis report
3. **Artifact Storage**: The report is stored using the filesystem storage provider
4. **Response**: The server responds with the task containing artifact metadata
5. **Artifact Download**: The client downloads the artifact using the provided URI
6. **Local Storage**: The downloaded file is saved to the `client/downloads/` directory

## Generated Artifacts

The server generates markdown reports that include:

- User request summary
- Timestamp and task ID
- Sample analysis content
- Conclusions about artifact capabilities

Example output structure:

```
client/downloads/
          └── analysis_report.md  # Downloaded artifact
```

## API Endpoints

### A2A Server (Port 8080)

- `POST /a2a` - Main A2A protocol endpoint
- `GET /.well-known/agent-card.json` - Agent capabilities discovery
- `GET /health` - Health check

### Artifacts Server (Port 8081)

- `GET /artifacts/{artifactId}/{filename}` - Download artifact
- `GET /health` - Health check

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
            "text": "Create a detailed analysis report about renewable energy"
          }
        ]
      }
    }
  }'

# Download the artifact (replace ARTIFACT_ID with actual artifact ID)
curl -O http://localhost:8081/artifacts/ARTIFACT_ID/analysis_report.md
```

## Troubleshooting

### Common Issues

1. **Port Conflicts**: Ensure ports 8080 and 8081 are available
2. **Permission Denied**: Check write permissions for `./artifacts` directory
3. **Build Errors**: Run `go mod tidy` in both server and client directories

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
```

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server
docker compose run --rm a2a-debugger tasks list --include-artifacts
```

## Next Steps

- Explore the [MinIO artifacts example](../artifacts-minio/) for cloud storage
- See [ADK documentation](../../README.md) for more features
- Try [streaming examples](../streaming/) for real-time responses

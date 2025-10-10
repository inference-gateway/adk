# In-Memory Queue Storage Example

This example demonstrates the simplest queue storage configuration using in-memory storage. This is the default storage backend and is perfect for development, testing, and small deployments where persistence is not required.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Features](#features)
- [Directory Structure](#directory-structure)
- [Running the Example](#running-the-example)
- [Configuration](#configuration)
- [How It Works](#how-it-works)
- [Next Steps](#next-steps)
- [Troubleshooting](#troubleshooting)

## What This Example Shows

- A2A server with in-memory queue storage (default configuration)
- Task queuing and processing without external dependencies
- Simple client demonstrating task submission
- Docker Compose setup for easy local development

## Features

- **No External Dependencies**: Works out-of-the-box without Redis or other services
- **Fast Setup**: Perfect for development and testing scenarios
- **Task Queuing**: Demonstrates background task processing
- **Memory-Only**: Tasks are lost when server restarts (development-friendly)

## Directory Structure

```
in-memory/
├── client/
│   ├── main.go         # A2A client submitting tasks
│   └── go.mod          # Client dependencies
├── server/
│   ├── main.go         # A2A server with in-memory storage
│   ├── config/
│   │   └── config.go   # Configuration structure
│   └── go.mod          # Server dependencies
├── docker-compose.yaml # Docker setup, uses ../../Dockerfile.server and ../../Dockerfile.client
├── .env.example        # Environment variables
└── README.md           # This file
```

## Running the Example

### Using Docker Compose (Recommended)

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Run the example:

```bash
docker-compose up --build
```

This will:

1. Start the A2A server with in-memory queue storage
2. Run the client to submit tasks for background processing
3. Show logs demonstrating task queuing and processing

### Running Locally

#### Start the Server

```bash
cd server
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## Configuration

The server uses in-memory storage by default. Key environment variables:

| Environment Variable         | Description           | Default  |
| ---------------------------- | --------------------- | -------- |
| `A2A_QUEUE_PROVIDER`         | Storage provider      | `memory` |
| `A2A_QUEUE_MAX_SIZE`         | Maximum queue size    | `100`    |
| `A2A_QUEUE_CLEANUP_INTERVAL` | Task cleanup interval | `30s`    |
| `A2A_SERVER_PORT`            | Server port           | `8080`   |
| `A2A_DEBUG`                  | Enable debug logging  | `false`  |

## Understanding In-Memory Storage

### Advantages

- **Zero Dependencies**: No external services required
- **Fast Performance**: Direct memory access
- **Simple Setup**: Works immediately
- **Development Friendly**: Easy debugging and testing

### Limitations

- **No Persistence**: Tasks lost on server restart
- **Single Instance**: Cannot scale horizontally
- **Memory Limited**: Queue size limited by available RAM
- **No Clustering**: No support for multiple server instances

### Use Cases

- Development and testing environments
- Small applications with low task volumes
- Prototyping and experimentation
- Scenarios where task persistence is not required

## Task Processing Flow

1. **Client Submission**: Client submits tasks via A2A protocol
2. **Memory Queue**: Tasks stored in in-memory queue data structure
3. **Background Processing**: Server processes tasks from queue
4. **Status Updates**: Task status updated in memory
5. **Cleanup**: Completed tasks cleaned up based on retention policy

## Example Output

When running the example, you'll see logs like:

```
Server:
INFO: A2A server starting with in-memory storage
INFO: Task enqueued for processing, task_id=task-123, queue_length=1
INFO: Task dequeued for processing, task_id=task-123, remaining_queue_length=0
INFO: Task completed successfully, task_id=task-123

Client:
INFO: Submitting task to server
INFO: Task submitted successfully, task_id=task-123
INFO: Task status: completed
```

## Next Steps

- Try the `redis` example for production-ready queue storage
- Explore other ADK examples for different patterns
- Check the main ADK documentation for advanced configuration options

## Troubleshooting

### Troubleshooting with A2A Debugger

```bash
# List tasks and debug the A2A server
docker compose run --rm a2a-debugger tasks list
```

### Common Issues

1. **Port already in use**: Change `A2A_SERVER_PORT` in environment
2. **Memory limits**: Adjust `A2A_QUEUE_MAX_SIZE` for large workloads
3. **Task cleanup**: Tune `A2A_QUEUE_CLEANUP_INTERVAL` for your needs

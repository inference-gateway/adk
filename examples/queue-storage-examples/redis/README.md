# Redis Queue Storage Example

This example demonstrates Redis-based queue storage for production environments. Redis provides persistent, scalable queue storage with support for horizontal scaling and high availability.

## What This Example Shows

- A2A server configured with Redis queue storage
- Redis instance running in Docker container
- Task persistence across server restarts
- Production-ready queue configuration
- Client demonstrating task submission and monitoring

## Features

- **Persistent Storage**: Tasks survive server restarts
- **Horizontal Scaling**: Multiple server instances can share the same queue
- **High Performance**: Redis provides fast queue operations
- **Production Ready**: Suitable for production deployments
- **Monitoring**: Redis provides built-in monitoring capabilities
- **Clustering Support**: Can be configured for Redis clusters

## Directory Structure

```
redis/
├── client/
│   ├── main.go       # A2A client submitting tasks
│   ├── go.mod        # Client dependencies
│   └── Dockerfile    # Client container
├── server/
│   ├── main.go       # A2A server with Redis storage
│   ├── config/
│   │   └── config.go # Configuration structure
│   ├── go.mod        # Server dependencies
│   └── Dockerfile    # Server container
├── docker-compose.yaml # Docker setup with Redis
├── .env.example      # Environment variables
└── README.md         # This file
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

1. Start a Redis container for queue storage
2. Start the A2A server configured to use Redis
3. Run the client to submit tasks
4. Demonstrate persistent task storage

### Running Locally

You'll need a Redis instance running locally. You can start one with Docker:

```bash
docker run -d -p 6379:6379 redis:7-alpine
```

#### Start the Server

```bash
cd server
export A2A_QUEUE_PROVIDER=redis
export A2A_QUEUE_URL=redis://localhost:6379
go run main.go
```

#### Run the Client

```bash
cd client
go run main.go
```

## Configuration

The server uses Redis storage with the following key environment variables:

| Environment Variable           | Description                        | Default                |
| ------------------------------ | ---------------------------------- | ---------------------- |
| `A2A_QUEUE_PROVIDER`          | Storage provider                   | `redis`                |
| `A2A_QUEUE_URL`               | Redis connection URL               | `redis://redis:6379`   |
| `A2A_QUEUE_OPTIONS_DB`        | Redis database number              | `0`                    |
| `A2A_QUEUE_OPTIONS_MAX_RETRIES` | Maximum connection retries       | `3`                    |
| `A2A_QUEUE_OPTIONS_TIMEOUT`   | Connection timeout                 | `5s`                   |
| `A2A_QUEUE_CREDENTIALS_PASSWORD` | Redis password (if required)    | (empty)                |
| `A2A_SERVER_PORT`             | Server port                        | `8080`                 |
| `A2A_DEBUG`                   | Enable debug logging               | `false`                |

### Redis URL Format

The Redis URL supports various formats:

```bash
# Basic connection
redis://localhost:6379

# With password
redis://:password@localhost:6379

# With username and password
redis://username:password@localhost:6379

# Specific database
redis://localhost:6379/2

# TLS connection
rediss://localhost:6380
```

## Understanding Redis Storage

### Advantages

- **Persistence**: Tasks survive server restarts and crashes
- **Scalability**: Multiple server instances can share the same queue
- **Performance**: Optimized for high-throughput queue operations
- **Reliability**: Redis provides durability and consistency guarantees
- **Monitoring**: Rich set of monitoring and debugging tools
- **Clustering**: Supports Redis Cluster for high availability

### Architecture

- **Task Queue**: Uses Redis lists for FIFO task processing
- **Active Tasks**: Stores currently processing tasks in Redis hashes
- **Dead Letter Queue**: Completed/failed tasks stored for history
- **Context Mapping**: Redis sets track tasks by context ID
- **Atomic Operations**: Redis transactions ensure data consistency

### Use Cases

- Production environments requiring task persistence
- Applications with high task volumes
- Multi-instance deployments requiring shared queues
- Systems requiring task history and auditing
- Applications needing reliable task processing

## Task Processing Flow

1. **Client Submission**: Client submits tasks via A2A protocol
2. **Redis Queue**: Tasks stored in Redis list with LPUSH
3. **Background Processing**: Server uses BRPOP for blocking dequeue
4. **Active Tracking**: Processing tasks stored in Redis hashes
5. **Status Updates**: Task status atomically updated in Redis
6. **Dead Letter Storage**: Completed tasks moved to permanent storage
7. **Cleanup**: Periodic cleanup based on retention policies

## Redis Data Structure

The example uses the following Redis keys:

```
a2a:queue              # Main task queue (Redis list)
a2a:active:{task_id}   # Active task data (Redis hash)
a2a:deadletter:{task_id} # Completed task data (Redis hash)
a2a:context:{context_id} # Tasks per context (Redis set)
a2a:queue:notify       # Queue notification channel (Redis pub/sub)
```

## Example Output

When running the example, you'll see logs like:

```
Redis:
Ready to accept connections

Server:
INFO: A2A server starting with Redis storage
INFO: Connected to Redis, addr=redis:6379, db=0
INFO: Task enqueued for processing, task_id=task-123, queue_length=1
INFO: Task dequeued for processing, task_id=task-123, remaining_queue_length=0
INFO: Task completed successfully, task_id=task-123

Client:
INFO: Submitting task to Redis queue
INFO: Task submitted successfully, task_id=task-123
INFO: Task status: completed
```

## Monitoring Redis

You can monitor the Redis instance using:

```bash
# Connect to Redis CLI
docker exec -it redis-queue-example redis-cli

# Check queue length
LLEN a2a:queue

# List all keys
KEYS a2a:*

# Get queue statistics
INFO keyspace
```

## Production Considerations

### Redis Configuration

For production deployments, consider:

```bash
# Redis persistence
A2A_QUEUE_OPTIONS_MAXMEMORY_POLICY=allkeys-lru

# Connection pooling
A2A_QUEUE_OPTIONS_MAX_RETRIES=5
A2A_QUEUE_OPTIONS_TIMEOUT=10s

# Security
A2A_QUEUE_CREDENTIALS_PASSWORD=your_secure_password
```

### High Availability

- Use Redis Sentinel for automatic failover
- Configure Redis Cluster for horizontal scaling
- Set up Redis replication for data redundancy
- Monitor Redis health and performance metrics

### Security

- Enable Redis AUTH with strong passwords
- Use TLS encryption (rediss://) for connections
- Configure network security and firewall rules
- Regularly update Redis to latest stable version

## Troubleshooting

### Common Issues

1. **Connection failed**: Check Redis URL and network connectivity
2. **Authentication failed**: Verify Redis password in credentials
3. **Database not found**: Ensure Redis database number exists
4. **Memory issues**: Monitor Redis memory usage and configure limits
5. **Performance issues**: Check Redis slow log and optimize queries

### Debug Commands

```bash
# Check Redis connection
redis-cli ping

# Monitor Redis commands
redis-cli monitor

# Check Redis logs
docker logs redis-queue-example

# View queue contents
redis-cli LRANGE a2a:queue 0 -1
```

## Next Steps

- Try the `in-memory` example for development scenarios
- Explore Redis Cluster configuration for high availability
- Check the main ADK documentation for advanced Redis options
- Consider Redis monitoring solutions like RedisInsight
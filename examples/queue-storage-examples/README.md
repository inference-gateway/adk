# Queue Storage Examples

This directory contains examples demonstrating different queue storage backends for the ADK (Agent Development Kit).

## Examples

### In-Memory Queue Storage (`in-memory/`)

Demonstrates the simplest queue storage using in-memory storage. Perfect for development and testing.

### Redis Queue Storage (`redis/`)

Demonstrates Redis-based queue storage for production environments with horizontal scaling capabilities.

## Queue Storage Overview

The ADK uses a pluggable storage system that supports different queue backends:

- **Memory**: Simple in-memory queue (default)
- **Redis**: Redis-based queue for production with clustering support

Queue storage is configured via environment variables:
- `A2A_QUEUE_PROVIDER`: Storage provider (`memory` or `redis`)
- `A2A_QUEUE_URL`: Connection URL (for Redis)
- `A2A_QUEUE_*`: Additional provider-specific options

## Running Examples

Each example includes:
- Complete server and client implementations
- Docker Compose setup for easy deployment
- Comprehensive README with setup instructions
- Environment configuration examples

Navigate to individual example directories for detailed instructions.
# A2A Examples

This directory contains examples demonstrating how to use the A2A (Agent-to-Agent) framework.

## Available Examples

### Server Example

The server example shows how to create a basic A2A server that can receive and process messages and tasks.

**Location**: `examples/server/`

**Features**:

- Basic A2A server setup
- Message and task handlers
- Health check endpoint
- Agent capabilities endpoint
- OpenTelemetry telemetry support

**Quick Start**:

```bash
cd examples/server
go run main.go
```

The server will start on `http://localhost:8080`

If AI using LLMs completions are required, create a simple Inference Gateway instance and run the server:

```bash
docker run -d --name inference-gateway -p 8081:8080 ghcr.io/inference-gateway/inference-gateway:latest
export INFERENCE_GATEWAY_URL=http://localhost:8081
```

### Client Example

The client example demonstrates how to create an A2A client to communicate with A2A servers.

**Location**: `examples/client/` (Coming Soon)

## Getting Started

1. Choose the example that fits your use case
2. Navigate to the example directory
3. Follow the README instructions in each example
4. Run the example code

## Documentation

For more detailed information about the A2A protocol and framework, see the main [README](../README.md).

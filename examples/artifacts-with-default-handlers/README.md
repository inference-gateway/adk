# Artifacts with Default Handlers Example

This example demonstrates how to use artifacts with an **AI-powered agent and default task handlers**. It showcases automatic artifact extraction functionality where AI agents use tools to create artifacts via `ArtifactHelper.CreateFileArtifactFromBytes()` and the default handlers automatically extract and attach them to tasks. This example requires an LLM provider configuration.

## Table of Contents

- [Quick Start](#quick-start)
- [Key Features](#key-features)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Running the Example](#running-the-example)
  - [Prerequisites](#prerequisites)
  - [Option 1: Using Docker Compose](#option-1-using-docker-compose-recommended)
  - [Option 2: Running Locally](#option-2-running-locally)
- [Available Tools](#available-tools)
- [Expected Output](#expected-output)
- [Key Differences from Other Examples](#key-differences-from-other-examples)
- [Project Structure](#project-structure)
- [Testing the Artifact Extraction](#testing-the-artifact-extraction)

## Quick Start

```bash
# Using Docker Compose (recommended)
docker-compose up --build

# Or run locally
cd server && go run main.go  # Terminal 1
cd client && go run main.go  # Terminal 2
```

The client will automatically submit tasks and display artifacts created by the AI agent.

## Key Features

- **AI-Powered Processing**: Requires LLM provider (OpenAI, Anthropic, DeepSeek, etc.) for intelligent task processing
- **Default Task Handlers**: Uses `WithDefaultTaskHandlers()` with automatic artifact extraction
- **Artifact-Creating Tools**: AI agent uses tools that create reports and diagrams as downloadable artifacts
- **Automatic Extraction**: No custom task handler logic needed - default handlers extract artifacts from tool results
- **OpenAI-Compatible Agent**: AI agent that intelligently invokes tools to generate artifacts based on user requests

## Configuration

### Environment Variables

The server supports configuration through environment variables. Copy `.env.example` to `.env` and configure:

```bash
# Environment
ENVIRONMENT=development

# A2A Server Configuration
A2A_SERVER_PORT=8080                          # Server port
A2A_DEBUG=true                                # Enable debug logging
A2A_CAPABILITIES_STREAMING=true               # Enable streaming support

# LLM Configuration (REQUIRED)
A2A_AGENT_CLIENT_PROVIDER=openai                  # LLM provider (e.g., openai, anthropic, deepseek)
A2A_AGENT_CLIENT_MODEL=gpt-4                      # Model to use (e.g., gpt-4, claude-3)
A2A_AGENT_CLIENT_BASE_URL=http://gateway:8080/v1  # Base URL for LLM API

# API Keys (Set the one matching your provider)
OPENAI_API_KEY=                              # OpenAI API key
ANTHROPIC_API_KEY=                           # Anthropic API key
DEEPSEEK_API_KEY=                            # DeepSeek API key
GOOGLE_API_KEY=                              # Google API key
CLOUDFLARE_API_KEY=                          # Cloudflare API key
COHERE_API_KEY=                              # Cohere API key
MISTRAL_API_KEY=                             # Mistral API key

# Client Configuration
SERVER_URL=http://localhost:8080             # Server URL for client
```

### AI Configuration

This example requires an LLM provider to be configured. To set up AI-powered responses:

1. Copy the environment file:

   ```bash
   cp .env.example .env
   ```

2. Configure your LLM provider in `.env`:

   ```bash
   # For OpenAI
   A2A_AGENT_CLIENT_PROVIDER=openai
   A2A_AGENT_CLIENT_MODEL=gpt-4
   OPENAI_API_KEY=sk-your-key

   # For Anthropic
   A2A_AGENT_CLIENT_PROVIDER=anthropic
   A2A_AGENT_CLIENT_MODEL=claude-3-opus
   ANTHROPIC_API_KEY=your-anthropic-key

   # For DeepSeek
   A2A_AGENT_CLIENT_PROVIDER=deepseek
   A2A_AGENT_CLIENT_MODEL=deepseek-chat
   DEEPSEEK_API_KEY=your-deepseek-key
   ```

3. Restart the server to apply changes

## Architecture

The key difference from other artifact examples is that this uses **default task handlers** instead of custom ones:

```go
serverBuilder := server.NewA2AServerBuilder(cfg.A2A, logger).
    WithAgent(agent).
    WithDefaultTaskHandlers()  // <-- Automatic artifact extraction
```

Tools create artifacts:

```go
artifactHelper := server.NewArtifactHelper()
artifact := artifactHelper.CreateFileArtifactFromBytes(
    "Report", "Analysis report", "report.md", data, &mimeType)
```

The default handlers automatically:

1. Extract artifacts from tool execution results
2. Validate artifact structure
3. Attach artifacts to task responses

## Running the Example

### Prerequisites

- Go 1.25 or later
- Docker and Docker Compose (optional)

### Option 1: Using Docker Compose (Recommended)

1. **Start the services:**

   ```bash
   docker-compose up --build
   ```

2. **The client will automatically run and demonstrate artifact creation**

### Option 2: Running Locally

1. **Start the server:**

   ```bash
   cd server
   go mod tidy
   go run main.go
   ```

2. **In another terminal, run the client:**
   ```bash
   cd client
   go mod tidy
   go run main.go
   ```

## Available Tools

The server includes several artifact-creating tools:

### 1. Report Generator (`generate_report`)

Creates markdown analysis reports with downloadable artifacts.

**Parameters:**

- `topic` (string): The topic for the report
- `format` (string): Report format ("markdown", "json", "xml")

### 2. Diagram Creator (`create_diagram`)

Generates PlantUML diagrams as text artifacts.

**Parameters:**

- `diagram_type` (string): Type of diagram ("sequence", "class", "activity")
- `title` (string): Diagram title
- `description` (string): Diagram description

### 3. Data Exporter (`export_data`)

Creates CSV data exports with sample data.

**Parameters:**

- `dataset` (string): Dataset name to export
- `format` (string): Export format (currently supports "csv")

## Expected Output

When you run the example, you should see:

**Server Output:**

```
🔧 Starting Artifacts with Default Handlers A2A Server...
2024/01/15 10:30:00 INFO configuration loaded
2024/01/15 10:30:00 INFO ✅ server created with default handlers
2024/01/15 10:30:00 INFO 🌐 server running on port 8080
```

**Client Output:**

```
--- Request 1: Generate Report ---
Sending: Generate a comprehensive report about renewable energy technologies

Response includes:
- Task completed successfully
- Artifacts: 1 artifact available
  - Name: "Renewable Energy Analysis Report"
  - Type: markdown file
  - Automatically extracted by default handler

--- Request 2: Create Diagram ---
Sending: Create a sequence diagram showing user authentication flow

Response includes:
- Task completed successfully
- Artifacts: 1 artifact available
  - Name: "Authentication Flow Diagram"
  - Type: PlantUML diagram
  - Automatically extracted by default handler
```

## Key Differences from Other Examples

| Example                             | Handler Type                  | Artifact Creation        | AI Required |
| ----------------------------------- | ----------------------------- | ------------------------ | ----------- |
| **artifacts-with-default-handlers** | `WithDefaultTaskHandlers()`   | **Automatic extraction** | **Yes**     |
| **artifacts-filesystem**            | Custom `ArtifactsTaskHandler` | Manual in handler        | No          |
| **artifacts-minio**                 | Custom `ArtifactsTaskHandler` | Manual in handler        | No          |
| **default-handlers**                | `WithDefaultTaskHandlers()`   | None                     | Optional    |

## Project Structure

```
artifacts-with-default-handlers/
├── README.md                 # This file
├── docker-compose.yaml       # Docker orchestration
├── .env.example             # Environment variables template
├── server/
│   ├── main.go              # Server entry point
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── Dockerfile           # Server container definition
│   ├── go.mod               # Go dependencies
│   └── go.sum               # Dependency checksums
└── client/
    ├── main.go              # Client entry point
    ├── Dockerfile           # Client container definition
    ├── go.mod               # Go dependencies
    └── go.sum               # Dependency checksums
```

## Testing the Artifact Extraction

The example demonstrates how artifacts created by tools are automatically:

1. **Extracted** from tool execution results in `agentResponse.AdditionalMessages`
2. **Validated** using `ArtifactHelper.ValidateArtifact()`
3. **Attached** to the task using `ArtifactHelper.AddArtifactToTask()`
4. **Returned** in the response without any custom handler logic

This shows the power of the default handlers - they handle all the artifact extraction automatically, so you can focus on creating useful tools that generate artifacts.

# Artifacts with Default Handlers Example

This example demonstrates how to use artifacts with openai-compatible agent and **default task handlers**. It showcases the new automatic artifact extraction functionality where tools can create artifacts using `ArtifactHelper.CreateFileArtifactFromBytes()` and the default handlers will automatically extract and attach them to tasks.

## Key Features

- **Default Task Handlers**: Uses `WithDefaultTaskHandlers()` with automatic artifact extraction
- **Artifact-Creating Tools**: Tools that create reports and diagrams as downloadable artifacts
- **Automatic Extraction**: No custom task handler logic needed - default handlers extract artifacts from tool results
- **OpenAI-Compatible Agent**: Full AI integration with artifact-creating tools
- **Mock LLM Support**: Works without LLM configuration for testing

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

## Configuration

The server can be configured via environment variables:

| Variable                     | Description              | Default       |
| ---------------------------- | ------------------------ | ------------- |
| `ENVIRONMENT`                | Runtime environment      | `development` |
| `A2A_SERVER_PORT`            | Server port              | `8080`        |
| `A2A_DEBUG`                  | Enable debug logging     | `false`       |
| `A2A_CAPABILITIES_STREAMING` | Enable streaming support | `true`        |
| `A2A_AGENT_CLIENT_PROVIDER`  | LLM provider (optional)  | -             |
| `A2A_AGENT_CLIENT_MODEL`     | LLM model (optional)     | -             |

### Adding AI Capabilities

To enable AI-powered responses:

1. Copy the example environment file:

   ```bash
   cp .env.example .env
   ```

2. Configure your LLM provider:

   ```bash
   # For OpenAI
   A2A_AGENT_CLIENT_PROVIDER=openai
   A2A_AGENT_CLIENT_MODEL=gpt-3.5-turbo

   # For DeepSeek
   A2A_AGENT_CLIENT_PROVIDER=deepseek
   A2A_AGENT_CLIENT_MODEL=deepseek-chat
   ```

3. Restart the server

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
| **artifacts-with-default-handlers** | `WithDefaultTaskHandlers()`   | **Automatic extraction** | Optional    |
| **artifacts-filesystem**            | Custom `ArtifactsTaskHandler` | Manual in handler        | No          |
| **artifacts-minio**                 | Custom `ArtifactsTaskHandler` | Manual in handler        | No          |
| **default-handlers**                | `WithDefaultTaskHandlers()`   | None                     | Optional    |

## Files Structure

```
artifacts-with-default-handlers/
├── README.md
├── docker-compose.yaml
├── .env.example
├── server/
│   ├── main.go
│   ├── config/config.go
│   ├── Dockerfile
│   ├── go.mod
│   └── go.sum
└── client/
    ├── main.go
    ├── Dockerfile
    ├── go.mod
    └── go.sum
```

## Testing the Artifact Extraction

The example demonstrates how artifacts created by tools are automatically:

1. **Extracted** from tool execution results in `agentResponse.AdditionalMessages`
2. **Validated** using `ArtifactHelper.ValidateArtifact()`
3. **Attached** to the task using `ArtifactHelper.AddArtifactToTask()`
4. **Returned** in the response without any custom handler logic

This shows the power of the default handlers - they handle all the artifact extraction automatically, so you can focus on creating useful tools that generate artifacts.

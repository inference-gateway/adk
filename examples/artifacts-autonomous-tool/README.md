# Artifacts Autonomous Tool Example

This example demonstrates an A2A server where an LLM can **autonomously** create artifacts using the built-in `create_artifact` tool. Unlike examples with custom task handlers that explicitly create artifacts, this approach lets the AI decide when and what artifacts to create based on user requests.

## Table of Contents

- [What This Example Shows](#what-this-example-shows)
- [Key Features](#key-features)
- [How It Works](#how-it-works)
- [Directory Structure](#directory-structure)
- [Running the Example](#running-the-example)
- [Configuration](#configuration)
- [Example Interactions](#example-interactions)
- [Understanding the Code](#understanding-the-code)
- [Comparing to Other Artifact Examples](#comparing-to-other-artifact-examples)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## What This Example Shows

- **Autonomous Artifact Creation**: LLM decides when to create artifacts using the `create_artifact` tool
- **No Custom Task Handler**: Uses default streaming task handler instead of custom artifact logic
- **AI-Powered Decision Making**: LLM interprets user requests and creates appropriate artifacts
- **Multiple File Types**: Demonstrates JSON, CSV, code files, and more
- **Full Integration**: Combines AI agent, toolbox, and artifact storage seamlessly

## Key Features

### The CreateArtifact Tool

The `create_artifact` tool is a built-in tool that can be enabled in the default toolbox:

```go
AgentConfig: serverConfig.AgentConfig{
    ToolBoxConfig: serverConfig.ToolBoxConfig{
        EnableCreateArtifact: true,  // Enable the tool
    },
},
```

When enabled, the LLM can call this tool with:

- `content`: The file content to save
- `type`: Must be "url" (indicates downloadable artifact)
- `filename`: Filename with extension (e.g., "report.json", "data.csv")
- `name`: Optional artifact name

### Autonomous Behavior

The LLM autonomously decides:

- **When** to create an artifact (e.g., user asks for a report, code, data file)
- **What** content to generate
- **What** filename and type to use
- **Whether** an artifact is needed at all

## How It Works

```
┌─────────────────┐
│  User Request   │  "Create a JSON report with user data"
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   LLM Agent     │  Analyzes request, decides to create artifact
│  (with toolbox) │
└────────┬────────┘
         │
         │ Calls create_artifact tool
         ▼
┌─────────────────┐
│ CreateArtifact  │  Generates content, saves to filesystem
│      Tool       │  Returns URL to artifact
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Task Response   │  Includes artifact metadata + download URL
│  with Artifact  │
└─────────────────┘
```

## Directory Structure

```
artifacts-autonomous-tool/
├── server/
│   ├── main.go           # AI-powered A2A server with create_artifact tool
│   └── config/
│       └── config.go     # Configuration structure
├── client/
│   └── main.go           # Client that tests autonomous artifact creation
├── docker-compose.yaml   # Docker setup with Inference Gateway
├── .env.example          # Environment variables template
└── README.md             # This file
```

## Running the Example

### Option 1: Docker Compose (Recommended)

1. Copy environment variables:

```bash
cp .env.example .env
```

2. Edit `.env` and add your API key for at least one provider:

```bash
# Choose one or more providers
OPENAI_API_KEY=your_openai_api_key_here
ANTHROPIC_API_KEY=your_anthropic_api_key_here
DEEPSEEK_API_KEY=your_deepseek_api_key_here

# Configure agent
A2A_AGENT_CLIENT_PROVIDER=openai
A2A_AGENT_CLIENT_MODEL=gpt-4o-mini

# Enable create_artifact tool (required for this example)
A2A_AGENT_CLIENT_TOOLS_CREATE_ARTIFACT=true
```

3. Run the example:

```bash
docker-compose up --build
```

This will:

1. Start the Inference Gateway with your configured providers
2. Start the A2A server with create_artifact tool enabled
3. Run the client with test prompts that trigger artifact creation
4. Download generated artifacts to `client/downloads/`

### Option 2: Local Development

#### Prerequisites

- Go 1.25+
- An LLM API key (OpenAI, Anthropic, etc.)
- Access to an Inference Gateway or direct LLM endpoint

#### Start the Server

```bash
cd server
export A2A_AGENT_CLIENT_PROVIDER=openai
export A2A_AGENT_CLIENT_MODEL=gpt-4o-mini
export A2A_AGENT_CLIENT_BASE_URL=http://localhost:8080/v1
export A2A_AGENT_CLIENT_TOOLS_CREATE_ARTIFACT=true
go run main.go
```

#### Run the Client

```bash
cd client
export SERVER_URL=http://localhost:8080
export ARTIFACTS_URL=http://localhost:8081
go run main.go
```

## Configuration

### Server Configuration

| Environment Variable                     | Default                         | Description                     |
| ---------------------------------------- | ------------------------------- | ------------------------------- |
| `ENVIRONMENT`                            | `development`                   | Runtime environment             |
| `A2A_AGENT_NAME`                         | `artifacts-autonomous-agent`    | Agent name                      |
| `A2A_AGENT_DESCRIPTION`                  | `An agent that autonomously...` | Agent description               |
| `A2A_AGENT_VERSION`                      | `0.1.0`                         | Agent version                   |
| `A2A_SERVER_PORT`                        | `8080`                          | A2A server port                 |
| `A2A_DEBUG`                              | `false`                         | Enable debug logging            |
| `A2A_CAPABILITIES_STREAMING`             | `true`                          | Enable streaming (required)     |
| `A2A_AGENT_CLIENT_BASE_URL`              | Via Inference Gateway           | LLM API endpoint                |
| `A2A_AGENT_CLIENT_PROVIDER`              | Required                        | LLM provider                    |
| `A2A_AGENT_CLIENT_MODEL`                 | Required                        | Model name                      |
| `A2A_AGENT_CLIENT_TOOLS_CREATE_ARTIFACT` | `true`                          | **Enable create_artifact tool** |
| `A2A_ARTIFACTS_ENABLE`                   | `true`                          | Enable artifacts support        |
| `A2A_ARTIFACTS_SERVER_PORT`              | `8081`                          | Artifacts server port           |
| `A2A_ARTIFACTS_SERVER_HOST`              | `localhost`                     | Artifacts server hostname       |
| `A2A_ARTIFACTS_STORAGE_PROVIDER`         | `filesystem`                    | Storage provider                |
| `A2A_ARTIFACTS_STORAGE_BASE_PATH`        | `./artifacts`                   | Base path for artifacts         |

**Docker Networking Note**: When running in Docker, set `A2A_ARTIFACTS_SERVER_HOST` to the service name (e.g., `server`) so artifact URLs are accessible from other containers in the network. The docker-compose.yaml already configures this correctly.

### Client Configuration

| Variable        | Default                 | Description                            |
| --------------- | ----------------------- | -------------------------------------- |
| `SERVER_URL`    | `http://localhost:8080` | A2A server URL                         |
| `ARTIFACTS_URL` | `http://localhost:8081` | Artifacts server URL                   |
| `DOWNLOADS_DIR` | `downloads`             | Directory to save downloaded artifacts |

## Example Interactions

### Request 1: JSON Report

**User**: "Create a JSON report with sample user data including names, emails, and ages for 3 users"

**LLM Actions**:

1. Analyzes the request
2. Generates JSON content with sample data
3. Calls `create_artifact` tool with:
   - `content`: The JSON data
   - `filename`: "users_report.json"
   - `type`: "url"
4. Returns response with artifact URL

**Result**: `users_report.json` available for download

### Request 2: CSV File

**User**: "Generate a CSV file with product inventory data for 5 products"

**LLM Actions**:

1. Interprets CSV format requirement
2. Generates properly formatted CSV content
3. Creates artifact with `.csv` extension
4. Provides download URL

**Result**: `inventory.csv` available for download

### Request 3: Python Script

**User**: "Write a Python script that calculates fibonacci numbers recursively"

**LLM Actions**:

1. Generates working Python code
2. Saves as `.py` file
3. Returns artifact with proper MIME type

**Result**: `fibonacci.py` available for download

## Understanding the Code

### Server (`server/main.go`)

#### Enable CreateArtifact Tool

```go
AgentConfig: serverConfig.AgentConfig{
    ToolBoxConfig: serverConfig.ToolBoxConfig{
        EnableCreateArtifact: true,  // Key setting!
    },
},
```

#### Create Agent with Default Toolbox

```go
agent, err := server.NewAgentBuilder(logger).
    WithConfig(&cfg.A2A.AgentConfig).
    WithLLMClient(llmClient).
    WithSystemPrompt(`You are a helpful AI assistant that can create artifacts...`).
    WithMaxChatCompletion(10).
    WithDefaultToolBox().  // Includes create_artifact when enabled
    Build()
```

#### Use Default Task Handlers

```go
a2aServer, err := server.NewA2AServerBuilder(cfg.A2A, logger).
    WithArtifactService(artifactService).  // Inject artifact service
    WithDefaultTaskHandlers().  // Configures both background and streaming handlers
    WithAgent(agent).
    Build()
```

The default task handlers automatically:

- Provides task context to tools via `TaskContextKey`
- Provides artifact service via `ArtifactServiceContextKey`
- Manages tool execution and artifact attachment
- Works with both polling (SendTask) and streaming modes

### Client (`client/main.go`)

The client demonstrates:

1. Sending requests that trigger artifact creation
2. Polling for task completion
3. Detecting artifacts in the response
4. Downloading artifacts from the provided URLs

```go
// Check for artifacts
if len(task.Artifacts) > 0 {
    for _, artifact := range task.Artifacts {
        // Extract download URL from artifact
        // Download and save locally
    }
}
```

## Comparing to Other Artifact Examples

### artifacts-filesystem

- **Custom task handler** explicitly creates artifacts
- Handler controls what artifacts are created
- Good for: Deterministic artifact generation

### artifacts-autonomous-tool (This Example)

- **LLM autonomously** decides when to create artifacts
- Uses built-in `create_artifact` tool
- Good for: AI-driven artifact creation based on user intent

### When to Use Each Approach

**Use Custom Task Handler** when:

- You need guaranteed artifact creation
- Artifact format/structure is fixed
- Business logic determines artifact content

**Use Autonomous Tool** when:

- LLM should decide when to create artifacts
- Artifact type varies based on user request
- You want AI-driven user experience

## Troubleshooting

### Common Issues

#### 1. No Artifacts Created

**Problem**: LLM doesn't use the `create_artifact` tool

**Solutions**:

- Verify `A2A_AGENT_CLIENT_TOOLS_CREATE_ARTIFACT=true` is set
- Check system prompt guides LLM to create artifacts
- Use more explicit prompts (e.g., "create a file with...")
- Try a different LLM model (some are better at tool usage)

#### 2. Artifacts Server Not Available

**Problem**: Can't download artifacts

**Solutions**:

```bash
# Check artifacts server health
curl http://localhost:8081/health

# Verify server is running on correct port
docker-compose ps
```

#### 3. Tool Not Available

**Problem**: LLM says it can't create artifacts

**Solutions**:

- Verify `WithDefaultToolBox()` is called in agent builder
- Check `EnableCreateArtifact: true` in configuration
- Restart server after config changes

### Debug Mode

Enable debug logging to see tool calls:

```bash
export A2A_DEBUG=true
go run main.go
```

Look for log entries like:

```
tool_call: create_artifact
tool_args: {"content":"...", "filename":"report.json", "type":"url"}
```

### Health Checks

```bash
# Check A2A server
curl http://localhost:8080/health

# Check artifacts server
curl http://localhost:8081/health

# Check agent card (verify capabilities)
curl http://localhost:8080/.well-known/agent-card.json
```

### Troubleshooting with A2A Debugger

```bash
# List tasks with artifacts
docker compose run --rm a2a-debugger tasks list --include-artifacts

# View specific task details
docker compose run --rm a2a-debugger tasks get <task-id>
```

## Next Steps

- Review the [artifacts documentation](../../docs/artifacts.md) for more details
- Try the [artifacts-filesystem example](../artifacts-filesystem/) for custom handlers
- Explore [streaming example](../streaming/) for real-time AI responses
- Check the [ai-powered example](../ai-powered/) for custom tools

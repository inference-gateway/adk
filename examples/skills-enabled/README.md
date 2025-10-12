# Skills-Enabled A2A Agent Example

This example demonstrates an A2A agent with built-in skills enabled for file operations and web access.

## Features

- **Built-in Skills**: File operations (Read, Write, Edit, MultiEdit) and web operations (WebSearch, WebFetch)
- **Security**: Sandboxing, rate limiting, and domain whitelisting
- **Environment Configuration**: Granular control over skill activation and settings
- **Agent Card Integration**: Skills are automatically declared in the agent card for discoverability

## Skills Included

### File Operations
- **Read**: Read file contents with line range support
- **Write**: Write content to files with backup creation
- **Edit**: String replacement editing with diff preview
- **MultiEdit**: Atomic multiple edit operations

### Web Operations
- **WebSearch**: Search the web using DuckDuckGo, Google, or Bing
- **WebFetch**: Fetch content from URLs with domain whitelisting

## Configuration

All skills are controlled via environment variables. Set `SKILLS_ENABLED=true` to enable the skills system globally.

### Global Skills Configuration

```bash
# Enable built-in skills globally
SKILLS_ENABLED=true

# Require terms of service acceptance (default: true)
SKILLS_REQUIRE_TOS=true

# Security settings
SKILLS_SAFETY_ENABLE_SANDBOX=true
SKILLS_SAFETY_SANDBOX_PATHS="/home/user/documents,/tmp/workspace"
SKILLS_SAFETY_PROTECTED_PATHS=".git,.env,*.key,*.pem"
SKILLS_SAFETY_MAX_FILE_SIZE=10485760  # 10MB
SKILLS_SAFETY_MAX_OPERATION_TIME=30s
```

### Individual Skill Configuration

#### Read Skill
```bash
SKILLS_READ_ENABLED=true
SKILLS_READ_REQUIRE_APPROVAL=false
SKILLS_READ_RATE_LIMIT_PER_MIN=60
SKILLS_READ_TIMEOUT=10s
```

#### Write Skill
```bash
SKILLS_WRITE_ENABLED=true
SKILLS_WRITE_REQUIRE_APPROVAL=true
SKILLS_WRITE_RATE_LIMIT_PER_MIN=30
SKILLS_WRITE_TIMEOUT=15s
SKILLS_WRITE_CREATE_BACKUPS=true
SKILLS_WRITE_ALLOWED_EXTENSIONS=".txt,.md,.json,.yaml,.yml"
```

#### Edit Skill
```bash
SKILLS_EDIT_ENABLED=true
SKILLS_EDIT_REQUIRE_APPROVAL=true
SKILLS_EDIT_RATE_LIMIT_PER_MIN=30
SKILLS_EDIT_TIMEOUT=15s
SKILLS_EDIT_CREATE_BACKUPS=true
SKILLS_EDIT_MAX_EDITS_PER_CALL=10
```

#### MultiEdit Skill
```bash
SKILLS_MULTI_EDIT_ENABLED=true
SKILLS_MULTI_EDIT_REQUIRE_APPROVAL=true
SKILLS_MULTI_EDIT_RATE_LIMIT_PER_MIN=20
SKILLS_MULTI_EDIT_TIMEOUT=30s
SKILLS_MULTI_EDIT_CREATE_BACKUPS=true
SKILLS_MULTI_EDIT_MAX_EDITS_PER_CALL=50
SKILLS_MULTI_EDIT_ATOMIC_OPERATIONS=true
```

#### WebSearch Skill
```bash
SKILLS_WEB_SEARCH_ENABLED=true
SKILLS_WEB_SEARCH_REQUIRE_APPROVAL=false
SKILLS_WEB_SEARCH_RATE_LIMIT_PER_MIN=30
SKILLS_WEB_SEARCH_TIMEOUT=10s
SKILLS_WEB_SEARCH_MAX_RESULTS=10
SKILLS_WEB_SEARCH_ALLOWED_ENGINES="duckduckgo,google"
SKILLS_WEB_SEARCH_API_KEY=""  # Required for Google/Bing
```

#### WebFetch Skill
```bash
SKILLS_WEB_FETCH_ENABLED=true
SKILLS_WEB_FETCH_REQUIRE_APPROVAL=false
SKILLS_WEB_FETCH_RATE_LIMIT_PER_MIN=60
SKILLS_WEB_FETCH_TIMEOUT=15s
SKILLS_WEB_FETCH_WHITELISTED_DOMAINS="github.com,golang.org,docs.anthropic.com"
SKILLS_WEB_FETCH_MAX_CONTENT_SIZE=1048576  # 1MB
SKILLS_WEB_FETCH_FOLLOW_REDIRECTS=true
SKILLS_WEB_FETCH_USER_AGENT="A2A-Agent/1.0"
SKILLS_WEB_FETCH_CACHE_ENABLED=true
SKILLS_WEB_FETCH_CACHE_TTL=3600s
```

## Security Features

### Sandboxing
- File operations are restricted to configured sandbox directories
- Path traversal protection prevents access outside allowed areas
- Protected path patterns block access to sensitive files

### Rate Limiting
- Each skill has configurable rate limits per minute
- Prevents abuse and resource exhaustion
- Per-skill granular control

### Domain Whitelisting
- WebFetch operations limited to whitelisted domains
- Prevents access to internal/sensitive endpoints
- Support for subdomain matching

### Content Size Limits
- File size limits prevent memory exhaustion
- Web content size limits for fetched resources
- Configurable per operation type

## Usage Examples

Once the server is running with skills enabled, you can send requests like:

### File Operations
```json
{
  "message": "Please read the contents of /home/user/documents/config.yaml"
}
```

```json
{
  "message": "Write this configuration to /home/user/documents/new-config.yaml: \napi_key: secret\nport: 8080"
}
```

```json
{
  "message": "Replace 'old_value' with 'new_value' in /home/user/documents/config.yaml"
}
```

### Web Operations
```json
{
  "message": "Search for 'golang best practices' and show me the top 5 results"
}
```

```json
{
  "message": "Fetch the content from https://docs.golang.org/pkg/fmt/"
}
```

## Running the Example

1. Set up your environment variables (see configuration above)
2. Set your LLM provider credentials:
   ```bash
   export AGENT_CLIENT_PROVIDER=openai
   export AGENT_CLIENT_MODEL=gpt-4
   export AGENT_CLIENT_API_KEY=your-api-key
   ```
3. Run the server:
   ```bash
   go run main.go
   ```
4. The server will start with skills enabled and ready to process requests

## Security Considerations

- **Always use sandboxing** in production environments
- **Carefully configure whitelisted domains** for web access
- **Review rate limits** based on your use case
- **Enable approval workflows** for destructive operations
- **Monitor skill usage** through logs and metrics
- **Regularly audit skill configurations** and permissions

## Skill Development

To add custom skills, implement the `Skill` interface and register them with the `SkillsRegistry`. See the built-in skills for examples of implementation patterns.
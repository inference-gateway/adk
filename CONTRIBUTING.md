# Contributing to A2A ADK

We welcome contributions to the Application Development Kit for A2A-compatible Agents! This document provides guidelines for contributing to the project.

## 🚀 Getting Started

### Prerequisites

Before contributing, ensure you have the following installed:

- **Go 1.25 or later**
- **[Task](https://taskfile.dev/)** for build automation
- **[golangci-lint](https://golangci-lint.run/)** for linting
- **Git** for version control

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub
2. **Clone your fork**:
   ```bash
   git clone https://github.com/your-username/adk.git
   cd adk
   ```
3. **Add the upstream remote**:
   ```bash
   git remote add upstream https://github.com/inference-gateway/adk.git
   ```
4. **Install dependencies**:
   ```bash
   go mod download
   ```
5. **Verify your setup**:
   ```bash
   task lint
   task test
   ```

## 📋 Development Workflow

### 1. Download Latest A2A Schema

When working on A2A features, always start by downloading the latest schema:

```bash
task a2a:download-schema
```

### 2. Generate Types

After downloading the schema, generate the types:

```bash
task a2a:generate-types
```

This will update the generated types in `types/generated_types.go` based on the latest schema.

### 3. Generate Mocks

If you've modified interfaces, regenerate the testing mocks:

```bash
task generate:mocks
```

### 4. Development Cycle

Follow this cycle for development:

```bash
# Make your changes
# ...

# Run linting
task lint

# Run tests
task test

# Tidy modules if needed
task tidy
```

### 5. Pre-commit Hooks (Recommended)

**Install the pre-commit hook to automatically run quality checks:**

```bash
task precommit:install
```

The pre-commit hook will automatically run smart checks based on file types:

- **Go files**: Full workflow (formatting, tidying, linting, tests)
- **Markdown files only**: Just formatting
- **Mixed files**: Full workflow

**Important**: The hook will **fail** if any checks fail or files need formatting. You'll need to review the changes, stage them, and commit again.

- **Mixed files**: Full workflow

This ensures consistent code quality and reduces CI failures.

**To bypass the hook if needed (not recommended):**

```bash
git commit --no-verify
```

**To uninstall the hook:**

```bash
task precommit:uninstall
```

### 6. Before Committing

**Always run these commands before committing:**

1. `task precommit:install` (recommended for first-time setup)
2. `task a2a:download-schema` (if working on A2A features)
3. `task a2a:generate-types` (if schema changes were made)
4. `task generate:mocks` (if interfaces were modified)
5. `task lint` (ensure code quality)
6. `task test` (ensure all tests pass)

Note: If you have the pre-commit hook installed, steps 4-6 will run automatically on commit.

## 🎯 Coding Guidelines

### Code Style

- **Follow established conventions**: Use Go standard formatting and naming conventions
- **Early returns**: Favor early returns to simplify logic and avoid deep nesting
- **Switch over if-else**: Prefer switch statements over if-else chains for cleaner, more readable code
- **Type safety**: Use strong typing and interfaces to ensure type safety and reduce runtime errors
- **Interface-driven design**: When possible, code to an interface to make mocking easier in tests

### Logging

- **Lowercase messages**: Always use lowercase log messages for consistency and readability
- **Structured logging**: Use structured logging with appropriate log levels

### Testing

- **Table-driven tests**: Always prefer table-driven testing patterns
- **Isolated mocks**: Each test case should have its own isolated mock server and mock dependencies
- **Test coverage**: Ensure comprehensive test coverage for new functionality

#### Example Table-Driven Test

```go
import (
    "net/http"
    "net/http/httptest"
    "strings"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/inference-gateway/adk/server"
)

func TestAgentServer(t *testing.T) {
    tests := []struct {
        name           string
        endpoint       string
        method         string
        body           string
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "health check returns ok",
            endpoint:       "/health",
            method:         "GET",
            body:           "",
            expectedStatus: http.StatusOK,
            expectedBody:   `{"status":"healthy"}`,
        },
        {
            name:           "agent info returns metadata",
            endpoint:       "/.well-known/agent-card.json",
            method:         "GET",
            body:           "",
            expectedStatus: http.StatusOK,
            expectedBody:   "", // validate structure separately
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Create isolated test environment
            server := setupTestAgent(t)

            // Make request
            req := httptest.NewRequest(tt.method, tt.endpoint, strings.NewReader(tt.body))
            rec := httptest.NewRecorder()

            // Use the server's HTTP handler
            server.ServeHTTP(rec, req)

            // Assert results
            assert.Equal(t, tt.expectedStatus, rec.Code)
            if tt.expectedBody != "" {
                assert.JSONEq(t, tt.expectedBody, rec.Body.String())
            }
        })
    }
}
```

## 🛠️ Making Changes

### Creating a Feature Branch

```bash
git checkout main
git pull upstream main
git checkout -b feature/your-feature-name
```

### Branch Naming Convention

- **Features**: `feature/feature-name`
- **Bug fixes**: `fix/bug-description`
- **Documentation**: `docs/doc-topic`
- **Refactoring**: `refactor/component-name`

### Commit Message Format

Use conventional commit format:

```
type(scope): Description

[optional body]

[optional footer]
```

**Types:**

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**

```
feat(agent): Add custom task result processor interface

fix(auth): Handle expired OIDC tokens properly

docs(readme): Update installation instructions

test(agent): Add table-driven tests for endpoint handlers
```

## 🧪 Testing Guidelines

### Test Structure

- **Unit tests**: Test individual functions and methods
- **Integration tests**: Test component interactions
- **End-to-end tests**: Test complete workflows

### Test Organization

The project follows Go conventions for test organization:

```
adk/
├── client/
│   ├── client.go
│   └── client_test.go
├── server/
│   ├── agent.go
│   ├── agent_test.go
│   ├── server.go
│   └── server_test.go
├── types/
│   └── generated_types.go
│   └── types.go
└── examples/
    ├── client/
    └── server/
```

### Mock Guidelines

- Use interfaces for dependencies to enable easy mocking
- Create isolated mocks for each test case
- Prefer dependency injection for testability
- Generate mocks using counterfeiter: `task generate:mocks`
- Mock files are generated in `adk/server/mocks/` and `adk/client/mocks/`

### Test Utilities

Create helper functions for common test setup:

```go
import (
    "testing"
    "github.com/inference-gateway/adk/server"
    "github.com/inference-gateway/adk/server/config"
    "github.com/inference-gateway/adk/server/mocks"
    "github.com/stretchr/testify/require"
    "go.uber.org/zap"
)

func setupTestAgent(t *testing.T) server.A2AServer {
    logger := zap.NewNop()
    cfg := &config.Config{
        Port: "0", // Use random port for tests
        AgentName: "test-agent",
        AgentDescription: "Test agent",
        AgentVersion: "0.1.0",
    }

    // Use mocks for dependencies
    mockTaskHandler := &mocks.FakeTaskHandler{}
    mockTaskResultProcessor := &mocks.FakeTaskResultProcessor{}

    builder := &mocks.FakeA2AServerBuilder{}
    builder.WithConfigReturns(builder)
    builder.WithLoggerReturns(builder)
    builder.WithTaskHandlerReturns(builder)
    builder.WithTaskResultProcessorReturns(builder)

    mockServer := &mocks.FakeA2AServer{}
    builder.BuildReturns(mockServer)

    server := builder.
        WithConfig(cfg).
        WithLogger(logger).
        WithTaskHandler(mockTaskHandler).
        WithTaskResultProcessor(mockTaskResultProcessor).
        Build()

    return server
}
```

## 📚 Documentation

### Code Documentation

- **Godoc comments**: Write clear godoc comments for all public APIs
- **Examples**: Include code examples in documentation
- **Inline comments**: Use inline comments for complex logic

### Documentation Updates

When adding new features:

1. Update relevant godoc comments
2. Add examples to documentation
3. Update README.md if needed
4. Consider adding to the project wiki

## 🔄 Pull Request Process

### Before Submitting

1. **Sync with upstream**:

   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run the complete workflow**:

   ```bash
   task a2a:download-schema  # if needed
   task a2a:generate-types   # if needed
   task generate:mocks       # if interfaces changed
   task lint
   task test
   ```

3. **Check for conflicts** and resolve them

### Pull Request Template

When creating a pull request, include:

- **Clear description** of the changes
- **Motivation** for the changes
- **Testing performed**
- **Screenshots** (if UI changes)
- **Breaking changes** (if any)
- **Related issues** (if applicable)

### Review Process

1. **Automated checks**: Ensure all CI checks pass
2. **Code review**: Address reviewer feedback promptly
3. **Testing**: Verify that tests cover the new functionality
4. **Documentation**: Ensure documentation is updated

## 🐛 Reporting Issues

### Bug Reports

When reporting bugs, include:

- **Go version** and operating system
- **Steps to reproduce** the issue
- **Expected behavior**
- **Actual behavior**
- **Error messages** and stack traces
- **Minimal reproducible example**

### Feature Requests

For feature requests:

- **Use case description**
- **Proposed solution**
- **Alternative solutions considered**
- **Additional context**

## 🛡️ Security

### Reporting Security Vulnerabilities

- **Do not** create public issues for security vulnerabilities
- Email security concerns to: security@inference-gateway.com
- Include detailed information about the vulnerability
- Allow time for the team to address the issue before disclosure

## 📞 Getting Help

### Community Resources

- **GitHub Discussions**: [Project Discussions](https://github.com/inference-gateway/adk/discussions)
- **Discord**: [Inference Gateway Discord](https://discord.gg/inference-gateway)
- **Documentation**: [Official Docs](https://docs.inference-gateway.com)

### Development Questions

- Check existing issues and discussions first
- Provide context and examples when asking questions
- Be respectful and patient with community members

## 🎉 Recognition

Contributors are recognized in:

- **CONTRIBUTORS.md** file
- **Release notes** for significant contributions
- **Project documentation** for documentation improvements

## 📋 Checklist

Before submitting your contribution:

- [ ] Code follows project style guidelines
- [ ] Tests are written and passing
- [ ] Documentation is updated
- [ ] Commit messages follow conventional format
- [ ] All CI checks pass
- [ ] Changes are rebased on latest main
- [ ] Pull request description is clear and complete

## 🔗 Additional Resources

- [A2A Protocol Specification](https://github.com/inference-gateway/schemas/tree/main/a2a)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Task Documentation](https://taskfile.dev/)

---

Thank you for contributing to the A2A ADK! Your contributions help make agent-to-agent communication more accessible and powerful for developers worldwide.

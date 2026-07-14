# Repository Guidelines

## Project Structure & Module Organization

This repository is a Go module for the Inference Gateway A2A ADK. Core packages live in `client/`, `server/`, and `types/`. Generated protocol types are in `types/generated_types.go`, based on `schema.yaml`. Examples are organized under `examples/<scenario>/`, usually with separate `client/`, `server/`, and `docker-compose.yaml` files. Additional documentation belongs in `docs/`; contributor workflow details are in `CONTRIBUTING.md`.

## Build, Test, and Development Commands

Use Task for common workflows:

- `task` lists available tasks.
- `task format` runs `gofmt` on Go files and Prettier on Markdown.
- `task lint` runs `golangci-lint run`.
- `task test` runs `go test -v -cover ./...`.
- `task tidy` runs `go mod tidy` in every module, including examples.
- `task a2a:generate-types` regenerates `types/generated_types.go` from `schema.yaml`.
- `task generate:mocks` regenerates Counterfeiter mocks after interface changes.
- `go build .` verifies the root package builds.

Install the optional hook with `task precommit:install`; it runs formatting, tidying, linting, and tests based on changed file types.

## Coding Style & Naming Conventions

Follow standard Go formatting and idioms. `.editorconfig` specifies tabs for Go, LF endings, final newlines, and a 120-column guideline. Prefer early returns, table-driven tests, interface-driven dependencies, and structured logging with lowercase messages. Keep generated files generated: update the source schema or interface, then rerun the relevant Task command.

No inline comments inside function bodies - only docblocks and comments above struct members are allowed.

### Import Conventions

All non-standard-library imports MUST use explicit named imports (aliases). Never rely on the default package name matching the import path.

Imports MUST be grouped in this order, with each group separated by a blank line:

1. **Standard library** (stdlib packages, no named imports needed)
2. **External testing libraries** (e.g. `require "github.com/stretchr/testify/require"`)
3. **External libraries** (third-party dependencies)
4. **Internal testing libraries** (e.g. `testutils "github.com/inference-gateway/adk/server/testutils"`)
5. **Internal libraries** (packages within the `github.com/inference-gateway/adk/` module)

Within each group, imports are sorted alphabetically by import path (handled by `gofmt`).

Example:

```go
import (
    "context"
    "net/http"
    "strings"
    "time"

    require "github.com/stretchr/testify/require"

    gin "github.com/gin-gonic/gin"
    otel "go.opentelemetry.io/otel"
    attribute "go.opentelemetry.io/otel/attribute"
    semconv "go.opentelemetry.io/otel/semconv/v1.32.0"
    trace "go.opentelemetry.io/otel/trace"
    zap "go.uber.org/zap"

    config "github.com/inference-gateway/adk/server/config"
    adkotel "github.com/inference-gateway/adk/server/otel"
)
```

## Testing Guidelines

Place tests beside the code under test using Go’s `*_test.go` convention, for example `server/task_manager_test.go`. Prefer table-driven tests with isolated mocks per case. Use helpers from `server/test_helpers.go`, `server/testutils/`, or package-local helpers where appropriate. Run `task test` before submitting; add focused coverage for new behavior and regressions.

## Commit & Pull Request Guidelines

The project uses conventional commits, such as `feat(agent): add task processor`, `fix(auth): handle expired token`, `docs: update examples`, and `chore(deps): bump tooling`. Branch names commonly use `feature/...`, `fix/...`, `docs/...`, or `refactor/...`.

Before opening a pull request, run `task format`, `task tidy`, `task lint`, and `task test`. Include a concise description, linked issues when applicable, and call out schema, generated type, mock, or example changes. For behavior visible in examples, describe how you verified it locally.

## Security & Configuration Tips

Do not commit secrets from `.env` or local example configuration. Prefer documented environment variables and keep example credentials clearly non-production.

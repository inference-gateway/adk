# Inference Gateway A2A ADK

README for AI coding agents working in this repository. See README.md for user-facing docs and CONTRIBUTING.md for the
full contributor workflow.

## Project Overview

Go library for building Agent-to-Agent (A2A) protocol agents. Module path: `github.com/inference-gateway/adk`. Core
packages: `client/` (JSON-RPC client), `server/` (server builder, task handlers, agent, storage, artifacts,
middlewares, telemetry), `types/` (protocol types). It is not a binary - `main.go` is a placeholder. The root module
and every `examples/<scenario>/client` and `examples/<scenario>/server` directory are separate Go modules, each example
with its own `docker-compose.yaml` (run with `cd examples/<name>/server && go run .`). Examples are how users learn the
API and broken examples do not surface in unit tests; scan them when changing public surfaces.

## Commands

Run from the repo root; `task --list` lists all targets.

- `task format` - gofmt on Go files, Prettier on Markdown
- `task lint` - `golangci-lint run`; `task lint:examples` runs `markdownlint --fix` on example Markdown
- `task test` - `go test -v -cover ./...`; single test: `go test -run TestNameRegex -v ./server/...`
- `task tidy` - `go mod tidy` in every module (root + examples)
- `task a2a:download-schema` + `task a2a:generate-types` - refresh the vendored `schema.yaml` and regenerate
  `types/generated_types.go`
- `task providers:download-schema` + `task generate:providers` - refresh `providers-schema.yaml` and regenerate
  provider artifacts (`.env.example`, `docker-compose.yaml`, READMEs)
- `task generate:mocks` (`generate:mocks:clean` to wipe first) - regenerate Counterfeiter mocks in `client/mocks/` and
  `server/mocks/`
- `task generate` - umbrella for types, providers, and mocks
- `task precommit:install` - install the pre-commit hook (format, tidy, mocks, lint, test; fails if files stay dirty)

## Architecture

- **Server** is assembled with `A2AServerBuilder`: gin HTTP/JSON-RPC layer (`/.well-known/agent-card.json` discovery,
  `/a2a` JSON-RPC entrypoint), task storage, task handlers, an optional LLM agent, and optional artifact storage.
- **Two task-handler interfaces, deliberately distinct:** `TaskHandler.HandleTask` (sync/queued path for
  `message/send` and the background processor) and `StreamableTaskHandler.HandleStreamingTask` (returns a channel of
  CloudEvents for `message/stream` and `tasks/resubscribe`; event types are the `EventXxx` constants in
  `types/types.go`). `WithDefaultTaskHandlers()` installs implementations that handle input-required pausing; use
  `WithBackgroundTaskHandler` / `WithStreamingTaskHandler` for custom orchestration.
- **The agent is stateless.** It does not own conversation history; the task's `History` is passed to every
  `RunWithStream(ctx, []Message)` call. Build one with `AgentBuilder`.
- **Callbacks** (`server/callbacks.go`): `BeforeAgent`/`AfterAgent`, `BeforeModel`/`AfterModel`,
  `BeforeTool`/`AfterTool`. Before callbacks short-circuit by returning non-nil; after callbacks can replace outputs.
  Guardrails, caching, and authorization are built on these - there is no separate middleware concept for them.
- **Storage** is an interface with in-memory (default) and Redis implementations, selected via `QUEUE_PROVIDER`.
- **Artifacts** run on a separate HTTP server (default port 8081, `ArtifactsServerBuilder`) with filesystem or MinIO
  backends. Proxy mode is default; `ARTIFACTS_STORAGE_BASE_URL` enables direct downloads. See `docs/artifacts.md`.
- **Config** (`server/config/`) is one `Config` struct loaded by `sethvargo/go-envconfig` with prefixed groups
  (`AGENT_CLIENT_*`, `CAPABILITIES_*`, `AUTH_*`, `QUEUE_*`, `TASK_RETENTION_*`, `SERVER_*`, `TELEMETRY_*`,
  `ARTIFACTS_*`). `AgentName`, `AgentDescription`, `AgentVersion` have no env tags; inject them at build time with
  `-ldflags "-X 'github.com/inference-gateway/adk/server.BuildAgentName=...'"` (likewise `BuildAgentDescription`,
  `BuildAgentVersion`).

## Generated Files

`types/generated_types.go`, `client/mocks/*.go`, and `server/mocks/*.go` are generated - never hand-edit them.
Change the source (`schema.yaml`, `providers-schema.yaml`, or the interface) and rerun the matching task.
`CHANGELOG.md` is maintained by semantic-release. CI re-runs formatting, tidy, and generation and then fails on
`git diff --exit-code`, so uncommitted generated or formatting changes break the build.

Mocks use Counterfeiter (`go run github.com/maxbrunsfeld/counterfeiter/v6`), not mockgen, even though CI installs
mockgen. `Taskfile.yml` has one `generate:mock:*` target per interface with `sources:` for incremental builds; add or
rename the target when you add or rename an interface.

## Schema Is Upstream

`schema.yaml` is vendored from `a2a/a2a-schema.yaml` and `providers-schema.yaml` from `openapi.yaml` in
`inference-gateway/schemas`. Propose protocol changes upstream, not here. To add an LLM provider: add it to the
`Provider` enum upstream, run `task providers:download-schema` and `task generate`, and commit the schema bump with the
generated changes; no hand-editing of example or doc files is needed.

## Code Style

Go 1.26 (matches `go.mod`; CI uses `go-version-file`). Tabs, LF endings, final newlines, 120-column limit for Go
(`.editorconfig`). Prefer early returns, `switch` over `if/else` chains, interface-driven dependencies, table-driven
tests with `t.Run`, and structured logging with lowercase messages.

Import order is enforced by `gci` (`.golangci.yml`): stdlib, `testify`, `server/mocks`, third-party, other
`inference-gateway/*` modules, then this module. Every non-stdlib import must be aliased to its last path element
(`zap "go.uber.org/zap"`, enforced by `importas`); pin an alias in `.golangci.yml` only when two packages collide.
Auto-fix with `golangci-lint fmt` and `golangci-lint run --fix`.

### Code Readability

- Write self-explanatory code: clear names and small, single-purpose functions carry the intent.
  If a block needs a comment to be understood, extract it into a well-named function or variable.
- No inline comments inside function bodies.
- Doc comments on functions and types are at most 5 lines: what it does and why, not how.
- No comments above modules, packages, or files.
- Tool directives are not comments and stay where the tool needs them (lint suppressions, build
  tags, compiler pragmas, code generation markers).

## Testing

Tests live beside the code (`server/task_manager_test.go`). Prefer table-driven tests with isolated mocks per case;
reuse helpers from `server/test_helpers.go` or `server/testutils/`. Run `task test` before submitting.

## Commits & Pull Requests

Conventional commits (`feat(agent): ...`, `fix(auth): ...`, `docs: ...`, `chore:`, `refactor:`, `test:`, `style:`);
semantic-release reads them (`.releaserc.yaml`). Branch names: `feature/...`, `fix/...`, `docs/...`, `refactor/...`.
Before opening a PR run `task format`, `task tidy`, `task lint`, `task test`. Call out schema, generated type, mock, or
example changes; for example behavior, describe how you verified it locally.

## Ecosystem Coordination

Part of the `inference-gateway` polyrepo; surface follow-up work when changing public surfaces:

- `feat:` or public-API `refactor:` needs a docs ticket in `inference-gateway/docs` (`[DOCS] ` title prefix,
  `documentation` label). Internal-only refactors can skip it, but say so in the PR body.
- Schema regeneration may mean `inference-gateway/rust-adk` and other ADKs need the same upstream change.
- Siblings: `inference-gateway/schemas` (protocol source of truth), `inference-gateway/docs`,
  `inference-gateway/a2a-debugger` and the `*-agent` repos (A2A consumers).

## Security

Never commit secrets from `.env` or example configs; keep example credentials clearly non-production. Auth is
OIDC/OAuth2 (`server/middlewares/auth.go`).

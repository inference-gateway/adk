# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

The **A2A Agent Development Kit (ADK)** — a Go library for building servers and clients that speak the [Agent-to-Agent (A2A) protocol](https://github.com/inference-gateway/schemas/tree/main/a2a). Module path: `github.com/inference-gateway/adk`. It is **not** a binary — `main.go` is a placeholder; consumers import the `server`, `client`, and `types` packages. Runnable end-to-end demos live under `examples/`.

This repo is one of many in the `inference-gateway` GitHub org polyrepo. The A2A schema is owned by `inference-gateway/schemas` (see "Schema is upstream" below).

## Commands

All commands run from the repo root. Discover the full list with `task --list`.

| Command | Purpose |
| --- | --- |
| `task test` | Run all tests (`go test -v -cover ./...`) |
| `task lint` | `golangci-lint run` |
| `task format` | `gofmt -w` on Go + `prettier -w` on Markdown |
| `task tidy` | Walks every `go.mod` in the tree (root + each `examples/*/{server,client}`) and runs `go mod tidy` |
| `task a2a:download-schema` | Pull the latest A2A schema YAML from `inference-gateway/schemas` into `schema.yaml` |
| `task a2a:generate-types` | Regenerate `types/generated_types.go` from `schema.yaml` |
| `task generate:mocks` | Regenerate every counterfeiter mock under `client/mocks/` and `server/mocks/` |
| `task generate:mocks:clean` | Wipe and regenerate all mocks |
| `task lint:examples` | `markdownlint --fix 'examples/**/*.md'` |
| `task precommit:install` | Copy `scripts/pre-commit` into `.git/hooks/` (recommended) |

**Single test / single package:**

```bash
go test -run TestNameRegex -v ./server/...
go test -v ./types/...
```

CI lives in `.github/workflows/ci.yml`. It mirrors the local flow and ends with `git diff --exit-code` after running `gofmt`, `prettier`, `go mod tidy`, and the type generator — so any formatting / tidy / generation change must be committed, otherwise CI fails. The pre-commit hook (`scripts/pre-commit`) runs format → tidy → `generate:mocks` → lint → test on staged Go files and likewise exits non-zero if files are still dirty after.

## Architecture

The two surfaces are the **server** (build an A2A agent) and the **client** (call one). Everything else is plumbing.

### Server (`server/`)

The server is assembled with a **builder** (`A2AServerBuilder`) that wires together: the HTTP / JSON-RPC layer (`server.go`, gin-based, with the `/.well-known/agent-card.json` discovery endpoint and `/a2a` JSON-RPC entrypoint), task storage, two task-handler interfaces, an optional LLM-backed agent, and optional artifact storage.

**Two task-handler interfaces, deliberately distinct:**

- `TaskHandler.HandleTask(...) (*Task, error)` — synchronous / queued path, used by `message/send` and the background processor.
- `StreamableTaskHandler.HandleStreamingTask(...) (<-chan cloudevents.Event, error)` — used by `message/stream` and `tasks/resubscribe`. Emits CloudEvents with types like `adk.agent.delta`, `adk.agent.tool.started`, `adk.agent.input.required` (see `types/types.go` `EventXxx` constants).

`WithDefaultTaskHandlers()` installs production-ready implementations of both that automatically handle input-required pausing; provide your own with `WithBackgroundTaskHandler` / `WithStreamingTaskHandler` when you need custom orchestration.

**The agent (`agent.go`, `agent_streamable.go`) is stateless.** It does not own conversation history — the task's `History` is passed in on every `RunWithStream` call. Its only method is `RunWithStream(ctx, []Message) (<-chan cloudevents.Event, error)`. Build one with `AgentBuilder` (LLM client + toolbox + callbacks + config).

**Callbacks (`callbacks.go`)** are lifecycle hooks: `BeforeAgent` / `AfterAgent`, `BeforeModel` / `AfterModel`, `BeforeTool` / `AfterTool`. *Before* callbacks can short-circuit by returning a non-nil value (skips the LLM call, the tool, or the whole agent). *After* callbacks can replace outputs. This is how guardrails, caching, and authorization are implemented — there is no separate middleware concept for those.

**Storage (`storage.go`, `storage_factory.go`)** is an interface with two implementations: in-memory (default) and Redis (`storage_redis.go`). Selected at runtime via `QUEUE_PROVIDER`; Redis enables horizontal scaling.

**Artifacts (`artifacts_*.go`)** run on a **separate HTTP server** (default port 8081) built with `ArtifactsServerBuilder`. Storage backends: filesystem and MinIO (S3). "Proxy mode" (default) downloads through the artifacts server; setting `ARTIFACTS_STORAGE_BASE_URL` enables direct downloads from the storage backend. See `docs/artifacts.md`.

**Middlewares (`server/middlewares/`):** auth (`auth.go`, OIDC), logging, telemetry (Prometheus + OpenTelemetry, also `server/otel/`).

### Client (`client/`)

`A2AClient` (`client.go`) is the full JSON-RPC client: `SendTask` / `StreamTask` / `GetTask` plus `CancelTask`, `ListTasks`, the four `*TaskPushNotificationConfig` methods, `ResubscribeTask`, and `GetAuthenticatedExtendedCard`. `artifact_helper.go` wraps the artifact download flow (proxy and direct).

### Types (`types/`)

- `generated_types.go` — **generated** from `schema.yaml` by `task a2a:generate-types`. **Do not hand-edit.** Header: `// Code generated from JSON schema. DO NOT EDIT.`
- `types.go` — hand-written additions (CloudEvent type constants, health status constants, JSON-RPC error helpers).
- `message_utils.go`, `part_marshaling.go` — helpers for the polymorphic `Part` union in A2A messages.

### Config (`server/config/`)

Single `Config` struct populated by `sethvargo/go-envconfig`. Nested groups use prefixes: `AGENT_CLIENT_*`, `CAPABILITIES_*`, `AUTH_*`, `QUEUE_*`, `TASK_RETENTION_*`, `SERVER_*`, `TELEMETRY_*`, `ARTIFACTS_*`. See `README.md` for the full table.

**Build-time-only fields:** `AgentName`, `AgentDescription`, `AgentVersion` have no env tags. They are intended to be injected via Go linker flags so the binary's identity is immutable:

```
-ldflags "-X 'github.com/inference-gateway/adk/server.BuildAgentName=...' \
          -X 'github.com/inference-gateway/adk/server.BuildAgentDescription=...' \
          -X 'github.com/inference-gateway/adk/server.BuildAgentVersion=...'"
```

### Examples (`examples/`)

Each example is a self-contained module pair: `examples/<name>/server/` and `examples/<name>/client/`, each with its own `go.mod`, plus a `docker-compose.yaml`. Run one with `cd examples/<name>/server && go run .`. When changing a public API, scan the examples — they're how users learn the library, and broken examples don't surface in unit tests.

## Conventions and gotchas

- **Generated files.** `types/generated_types.go` and everything in `*/mocks/` are regenerated. To change them, change the input (`schema.yaml` or the interface), then re-run the generator. CI's `git diff --exit-code` check enforces this.
- **Counterfeiter, not mockgen.** The `Taskfile.yml` has one `generate:mock:*` target per interface; if you add or rename an interface, add/rename its task target too (each one points at a specific source file via `sources:` for incremental builds). CI installs mockgen, but generation here uses `go run github.com/maxbrunsfeld/counterfeiter/v6`.
- **Code style** (see `CONTRIBUTING.md`): early returns over nesting; `switch` over `if/else` chains; lowercase log messages; code to interfaces for mockability; table-driven tests with `t.Run`.
- **Conventional Commits.** `feat:` / `fix:` / `docs:` / `chore:` / `refactor:` / `test:` / `style:`. semantic-release reads these — `CHANGELOG.md` is generated, do not hand-edit it. Release config: `.releaserc.yaml`.
- **Go version: 1.26.** Matches `go.mod`; CI uses `go-version-file: 'go.mod'`.

## Schema is upstream

`schema.yaml` in this repo is a vendored copy of `a2a/a2a-schema.yaml` from `inference-gateway/schemas`. The flow is: change the upstream schema → `task a2a:download-schema` → `task a2a:generate-types` → commit both `schema.yaml` and `types/generated_types.go`. Don't propose protocol changes here — open them against `inference-gateway/schemas` first.

## Ecosystem coordination

Part of the `inference-gateway` polyrepo. Public-surface changes here often need follow-up elsewhere — surface this explicitly when proposing such a change:

- `feat:` / public-API `refactor:` → docs ticket against `inference-gateway/docs` (`[DOCS] ` title prefix, `documentation` label). Internal-only refactors can skip, but say so explicitly in the PR body.
- Schema regeneration here may signal that `inference-gateway/rust-adk` and other ADKs need the same upstream change pulled in.

Sibling repos worth knowing: `inference-gateway/rust-adk` (Rust ADK), `inference-gateway/schemas` (protocol source of truth), `inference-gateway/docs` (user-facing docs), `inference-gateway/a2a-debugger` and the `*-agent` repos (A2A consumers).

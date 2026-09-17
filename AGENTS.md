# Inference Gateway A2A ADK

README for AI coding agents working in this repository. See README.md for user-facing docs, CONTRIBUTING.md for the
full contributor workflow, and CLAUDE.md for architecture notes.

## Project Overview

Go library for building Agent-to-Agent (A2A) protocol agents. Core packages: `client/` (JSON-RPC client), `server/`
(server builder, task handlers, agent, storage, artifacts, middlewares, telemetry), `types/` (protocol types). It is
not a binary — `main.go` is a placeholder. The root module and every `examples/<scenario>/client` and
`examples/<scenario>/server` directory are separate Go modules. Examples are how users learn the API; scan them when
changing public surfaces.

## Commands

Run from the repo root; `task` lists all targets.

- `task format` — gofmt on Go files, Prettier on Markdown
- `task lint` — `golangci-lint run`; `task lint:examples` also lint example modules
- `task test` — `go test -v -cover ./...`; single test: `go test -run TestNameRegex -v ./server/...`
- `task tidy` — `go mod tidy` in every module (root + examples)
- `task a2a:download-schema` + `task a2a:generate-types` — refresh the vendored `schema.yaml` and regenerate
  `types/generated_types.go`
- `task generate:providers` — regenerate provider artifacts from `providers-schema.yaml`
- `task generate:mocks` — regenerate Counterfeiter mocks in `client/mocks/` and `server/mocks/`
- `task precommit:install` — install the optional pre-commit hook

## Generated Files

`types/generated_types.go`, `client/mocks/*.go`, and `server/mocks/*.go` are generated — never hand-edit them.
Change the source (`schema.yaml`, `providers-schema.yaml`, or the interface) and rerun the matching task.
`CHANGELOG.md` is maintained by semantic-release. CI re-runs formatting, tidy, and generation and then fails on
`git diff --exit-code`, so uncommitted generated or formatting changes break the build. The schema is vendored from
`inference-gateway/schemas`; propose protocol changes upstream, not here.

## Code Style

Tabs, LF endings, final newlines, 120-column limit for Go (`.editorconfig`). Prefer early returns, `switch` over
`if/else` chains, interface-driven dependencies, table-driven tests with `t.Run`, and structured logging with
lowercase messages.

Import order is enforced by `gci` (`.golangci.yml`): stdlib, `testify`, `server/mocks`, third-party, other
`inference-gateway/*` modules, then this module. Every non-stdlib import must be aliased to its last path element
(`zap "go.uber.org/zap"`, enforced by `importas`); pin an alias in `.golangci.yml` only when two packages collide.
Auto-fix with `golangci-lint fmt` and `golangci-lint run --fix`.

## Testing

Tests live beside the code (`server/task_manager_test.go`). Prefer table-driven tests with isolated mocks per case;
reuse helpers from `server/test_helpers.go` or `server/testutils/`. Run `task test` before submitting.

## Commits & Pull Requests

Conventional commits (`feat(agent): ...`, `fix(auth): ...`, `docs: ...`). Branch names: `feature/...`, `fix/...`,
`docs/...`, `refactor/...`. Before opening a PR run `task format`, `task tidy`, `task lint`, `task test`. Call out
schema, generated type, mock, or example changes; for example behavior, describe how you verified it locally.

## Security

Never commit secrets from `.env` or example configs; keep example credentials clearly non-production. Auth is
OIDC/OAuth2 (`server/middlewares/auth.go`).
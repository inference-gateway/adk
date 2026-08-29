# Inference Gateway A2A ADK

This file is a README for AI coding agents working in this repository. See README.md for user-facing docs; CONTRIBUTING.md has the full contributor workflow.

## Project Overview

Go library for building Agent-to-Agent (A2A) protocol agents. Core packages: `client/` (A2A client), `server/` (server builders, storage, middlewares, telemetry), `types/` (protocol types). The root module and every example are separate Go modules: each `examples/<scenario>/server` (and `client/` where present) has its own `go.mod`.

## Build, Test, and Development Commands

Use Task for common workflows (`task` lists all tasks):

- `task format` — gofmt on Go files, Prettier on Markdown
- `task lint` — `golangci-lint run`
- `task test` — `go test -v -cover ./...`
- `task tidy` — `go mod tidy` in every module (root + examples)
- `task a2a:generate-types` — regenerate `types/generated_types.go` from `schema.yaml`
- `task generate:providers` — regenerate provider artifacts from `providers-schema.yaml`
- `task generate:mocks` — regenerate Counterfeiter mocks in `client/mocks/` and `server/mocks/`
- `task generate` — regenerate types, providers, and mocks (then `go build .` to verify the root package)

Install the optional pre-commit hook with `task precommit:install`.

## Generated Files

`types/generated_types.go`, `client/mocks/*.go`, and `server/mocks/*.go` are generated. Never edit them by hand: change the source (`schema.yaml`, `providers-schema.yaml`, or the interface) and rerun the matching `task` target. CI runs `git diff --exit-code` after format/tidy/generate, so commit regenerated output. Both schemas are vendored from `inference-gateway/schemas` — propose protocol changes upstream.

## Code Style

Tabs for Go, LF endings, final newlines, 120-column guideline (`.editorconfig`). Docblocks sit above exported symbols; inline comments are fine for complex logic (CONTRIBUTING.md). Prefer early returns, interface-driven dependencies, table-driven tests, and structured logging with lowercase messages.

All non-standard-library imports use explicit named imports (aliases): stdlib group first, blank line, then everything else (external and internal together — see `server/server.go`).

## Testing

Tests live beside the code (`server/task_manager_test.go`). Prefer table-driven tests with isolated mocks per case; reuse helpers from `server/test_helpers.go` or `server/testutils/`. Add focused coverage for new behavior and regressions, and run `task test` before submitting.

## Commits & Pull Requests

Conventional commits, e.g. `feat(agent): ...`, `fix(auth): ...`, `docs: ...`, `chore(deps): ...`. Branch names: `feature/...`, `fix/...`, `docs/...`, `refactor/...`. Before opening a PR run `task format`, `task tidy`, `task lint`, `task test`. Call out schema, generated type, mock, or example changes; for example behavior, describe how you verified it locally.

## Security

Never commit secrets from `.env` or example configs; prefer documented environment variables and keep example credentials clearly non-production. Auth is OIDC/OAuth2 (`server/middlewares/auth.go`).

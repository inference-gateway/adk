# Repository Guidelines

## Project Structure & Module Organization

This repository is the Go module `github.com/inference-gateway/adk`. Core packages live at the top level: `client/` contains A2A client helpers, `server/` contains server builders, handlers, storage, middleware, and test utilities, and `types/` contains shared and generated protocol types. `types/generated_types.go` is generated from `schema.yaml`; do not hand-edit it. Documentation lives in `docs/` and `README.md`. Runnable scenarios live under `examples/`, usually split into `client/` and `server/` submodules with their own `go.mod` files.

## Build, Test, and Development Commands

Use Task for common workflows:

- `task` lists available tasks.
- `task test` runs `go test -v -cover ./...` for the root module.
- `task lint` runs `golangci-lint run`.
- `task format` applies `gofmt` to Go files and Prettier to Markdown.
- `task tidy` runs `go mod tidy` in every module, including examples.
- `task a2a:generate-types` regenerates `types/generated_types.go` from `schema.yaml`.
- `task generate:mocks` regenerates Counterfeiter mocks after interface changes.
- `task precommit:install` installs the repository pre-commit hook.

Run example apps from their specific example directories; many include a `docker-compose.yaml`.

## Coding Style & Naming Conventions

Follow standard Go conventions and keep public names documented when they are exported. `.editorconfig` requires tabs for Go files, spaces with two-space indentation for Markdown and YAML, LF endings, UTF-8, and trimmed trailing whitespace. Prefer table-driven tests, early returns, interface-driven dependencies, and structured lowercase log messages. Keep generated mocks in `client/mocks/` or `server/mocks/` and name them `fake_<interface>.go`.

## Testing Guidelines

Place tests beside implementation files using Go’s `*_test.go` convention. Use `testing` with `stretchr/testify` where helpful, and isolate mocks or test servers per test case. Add or update tests for behavioral changes in `client/`, `server/`, and `types/`. Before submitting, run `task test`; run `task lint` as well for shared or CI-sensitive changes.

## Commit & Pull Request Guidelines

History follows Conventional Commits, often with scopes, for example `feat(server): ...`, `fix(auth): ...`, `docs(examples): ...`, `chore(deps): ...`, and `chore(release): ...`. Keep commits focused and mention generated updates when applicable. Pull requests should include a clear description, linked issues when relevant, test results, and documentation or example updates for user-facing behavior changes.

## Security & Configuration Tips

Do not commit secrets, local credentials, downloaded artifacts, or example runtime data. Keep `.gitignore` placeholders in upload, download, and artifact directories intact. When changing protocol behavior, regenerate types from the schema and verify compatibility with representative examples.

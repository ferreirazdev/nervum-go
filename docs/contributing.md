# Contributing to nervum-go

Thanks for your interest in contributing. This guide covers local setup, running tests, code style, and how to add a new feature.

## Prerequisites

- Go 1.24+ (see [go.mod](../go.mod))
- Postgres (for local run and optional integration tests)

## Setup

1. Clone the repo and enter the project directory.
2. Create a Postgres database (e.g. `CREATE DATABASE nervum;`).
3. Copy env config and edit as needed:
   ```bash
   cp .env.example .env
   # Set DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, PORT (default 8080)
   ```
4. Install dependencies and run the API:
   ```bash
   go mod download
   go run ./cmd/api
   ```
   Or use the Makefile: `make run`.

The API will listen on `http://localhost:8080`. Point the [nervum-ui](https://github.com/nervum/nervum-ui) app at `http://localhost:8080/api/v1` for full-stack development.

## Tests

- **Unit tests** (SQLite in-memory, no Postgres required):
  ```bash
  go test ./internal/...
  # or
  make test
  ```
- **Integration tests** (real Postgres; optional):
  ```bash
  # Set DB_* to a test database (e.g. nervum_test)
  go test -tags=integration ./internal/database/...
  # or
  make test-integration
  ```

Please run the relevant tests before submitting a PR.

## Code style

- Format code with `gofmt` or your editor’s “format on save”. The project does not currently enforce a specific linter in CI; keeping style consistent with the existing codebase is appreciated.
- Prefer clear names and short functions. Add a brief godoc comment for exported symbols (see existing packages under `internal/`).

## Adding a new feature

1. **Create a feature package** under `internal/features/<feature_name>/`:
   - `model.go` – domain types and table name (GORM).
   - `repository.go` – interface and implementation for DB access.
   - `handler.go` – Gin handlers and route registration (e.g. `Register(gin.RouterGroup)`).

2. **Register in the app**:
   - In `cmd/api/main.go`, create the repository (and any dependencies), create the handler, and call `handler.Register(protected)` or the appropriate group (e.g. public auth group for public routes).

3. **Migrations**:
   - In `internal/database/migrate.go`, add the new model(s) to `AutoMigrate` so GORM creates/updates tables.

4. **API spec**:
   - In `openapi/openapi.yaml`, add the new paths, request/response schemas, and security (cookie auth for protected routes). Keep the spec in sync so the API reference stays accurate.

5. **Package docs**:
   - Add a package comment in the feature’s `model.go` (or another file) and brief comments for exported types and functions so `go doc` is useful.

## Pull requests and issues

- Open an issue for bugs or feature ideas if you want to discuss first.
- For small fixes (typos, docs, tests), a direct PR is fine.
- Keep PRs focused; link to related issues when applicable.

This repo is maintained as a small, focused backend for the Nervum SaaS; we welcome improvements to layout, testing, and documentation.

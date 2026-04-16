# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make run                    # Start the API server (loads .env automatically)
make test                   # Run unit tests (SQLite in-memory, no Postgres needed)
make test-integration       # Run integration tests (requires real Postgres)
go test ./internal/features/entities/...  # Run tests for a single feature
make migrate-up             # Apply pending migrations
make migrate-down N=1       # Roll back N migrations
make migrate-create         # Interactively create a new migration pair
make migrate-version        # Show current schema version
make migrate-force V=1      # Force version (use to recover from dirty state)
make openapi-serve          # Serve Swagger UI at http://localhost:8081 (requires Docker)
```

Build binaries:
```bash
go build ./cmd/api          # API server
go build ./cmd/healthcheck  # Health check CLI
go build ./cmd/migrate      # Migration CLI
```

## Architecture

**Three binaries** under `cmd/`:
- `api` — main HTTP server (Gin), runs migrations on startup
- `healthcheck` — CLI that polls entity health check URLs and updates statuses; run on a schedule
- `migrate` — standalone migration CLI (`up`, `down [N]`, `version`, `force <V>`)

**Request flow**: Gin router → trusted proxy config → `secureheaders` + CORS middleware → `RequireAuth` (session cookie or service token) on protected groups → feature handler → repository → GORM/Postgres.

Auth login/register are rate-limited (5 req/min/IP). Public invitation and OAuth callback routes have their own per-IP limits (see `cmd/api/main.go`). All other `/api/v1/...` routes require an authenticated session. The `RequireAuth` middleware sets the current `*user.User` on the Gin context under `authPkg.ContextUser`.

**Feature structure** — every domain lives in `internal/features/<name>/`:
- `model.go` — GORM struct
- `repository.go` — data-access interface + GORM implementation
- `handler.go` — Gin HTTP handlers; call `Register(*gin.RouterGroup)`
- `*_test.go` — unit tests using `database.NewTestDB()` (SQLite)

New features follow the same pattern and are wired up in `cmd/api/main.go`.

**Multi-tenancy**: `OrganizationID` is always set from the authenticated user in handlers — never trusted from the client request body. All queries must scope by organization.

**Database**: Postgres in production, SQLite in-memory for unit tests. Migrations live in `migrations/` as numbered SQL pairs (`*.up.sql` / `*.down.sql`) and are embedded into the binary via `internal/database/embed.go`. Migrations run automatically on API startup.

**Config** (`internal/config/`): loaded from environment variables (`.env` via godotenv). Key groups: `Database`, `Server` (port, CORS, session cookie, service token), `Integrations` (GitHub/Google OAuth, encryption key).

**JSONB fields**: use `internal/pkg/types.JSONB` for Postgres `jsonb` columns (e.g., `Metadata`, health check headers).

## Key design decisions

- `OrganizationID` is enforced server-side — never trust it from clients.
- The `healthcheck` binary is designed to run as a cron job; exit code 0 = all healthy, 1 = any failure.
- Integration credentials (OAuth tokens) are AES-GCM encrypted at rest using `INTEGRATION_ENCRYPTION_KEY`.
- Service-to-service auth uses a static `SERVICE_TOKEN` header checked in `RequireAuth`.

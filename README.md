# Nervum – Environment Maps API (SaaS backend)

**Nervum** is a SaaS for visualizing and managing your software environments as a living map – services, databases, queues, and the relationships between them. This repository contains the **Go API** (`nervum-go`) that powers the product.

The API is designed to be:

- **Production-ready**: Postgres + GORM, env-based config, health checks
- **Feature-oriented**: clear domain boundaries (organizations, environments, entities, relationships, access control)
- **SaaS-friendly**: multi-organization support, user accounts, environment-level access

It pairs with the React frontend in [`nervum-ui`](https://github.com/nervum/nervum-ui) to deliver the full SaaS experience.

---

## What Nervum does (SaaS overview)

- **Environment maps**: visualize prod/staging/dev with nodes and relationships between services, databases, queues, third‑party systems, etc.
- **Ownership & access**: attach owners/teams and roles to environments so the right people see the right maps.
- **Change-friendly**: quickly add, edit, or retire entities and relationships as your architecture evolves.
- **API-first**: everything in the UI is backed by this API, so you can integrate Nervum into your own workflows and automations.

If you’re running Nervum as a SaaS:

- This repo is the **backend service** you deploy (e.g. to Fly.io, Render, Kubernetes, or a simple VM) behind your public SaaS domain.
- `nervum-ui` is the **public web app** your users interact with.

---

## Architecture (backend only)

- **Language**: Go
- **Database**: Postgres (SQLite in unit tests)
- **ORM**: GORM
- **HTTP framework**: Gin
- **Layout**: feature-oriented `internal/features/*`

High-level layout:

```text
cmd/api/                    # Entrypoint (main service for the SaaS)
internal/
  config/                   # Env-based config for SaaS deployments
  database/                 # GORM connection, migrations, test DB
  pkg/                      # Shared internal packages (ratelimit, secureheaders, types)
  features/
    auth/                   # Sessions, login/register/logout, RequireAuth
    organizations/          # Tenants / orgs
    users/                  # Accounts + permissions
    teams/                  # Teams and team–environment links
    user_teams/             # User–team membership
    invitations/            # Invite-by-email, by-token, accept
    environments/           # Environments per org (prod/staging/dev...)
    entities/               # Nodes in the environment map
    relationships/          # Edges between entities
    user_environment_access/ # RBAC-style environment access
    integrations/           # OAuth (GitHub, Google), dashboard
    repositories/           # Org-linked repositories
    orgservices/            # Organization-level services
```

---

## Running locally (SaaS-style dev setup)

1. **Create a Postgres database**

   ```sql
   CREATE DATABASE nervum;
   ```

2. **Configure environment variables**

   ```bash
   cp .env.example .env
   # Edit .env:
   # DB_HOST, DB_USER, DB_PASSWORD, DB_NAME
   # PORT (default 8080)
   # APP_ENV=local|staging|production
   ```

3. **Install dependencies and run the API**

   ```bash
   go mod download
   go run ./cmd/api
   ```

   The API will be available at `http://localhost:8080/api/v1` (or the `PORT` set in `.env`).

4. **Connect the SaaS UI**

   - Clone and run [`nervum-ui`](https://github.com/nervum/nervum-ui).
   - Point the UI’s base API URL (see its `README.md` or `.env`) at `http://localhost:8080/api/v1`.

---

## Core API surface

For full request/response schemas, status codes, and auth, see the [OpenAPI spec](openapi/openapi.yaml) and [openapi/README.md](openapi/README.md). Summary:

These endpoints back the main SaaS features (exact routes may evolve over time; check the code or OpenAPI spec for the latest definitions):

- **Health**
  - `GET /health` — Returns 200 with `{ "status": "ok", "database": "ok" }` when the API and database are healthy. Returns 503 with `{ "status": "unhealthy", "database": "unreachable" }` when the DB ping fails (suitable for load balancer or Kubernetes readiness probes).

- **Auth** (public: login, register; protected: logout, me)
  - `POST /api/v1/auth/register`, `POST /api/v1/auth/login` (rate-limited)
  - `POST /api/v1/auth/logout`, `GET /api/v1/auth/me` (protected)

- **Organizations**
  - `POST /api/v1/organizations`
  - `GET /api/v1/organizations`
  - `PUT /api/v1/organizations/:id`
  - `DELETE /api/v1/organizations/:id`

- **Users**
  - `POST /api/v1/users`
  - `GET /api/v1/users`
  - `PUT /api/v1/users/:id`
  - `DELETE /api/v1/users/:id`

- **Teams**
  - `POST/GET/PUT/DELETE /api/v1/teams` (query: `organization_id`)

- **User teams** (user–team membership)
  - `POST/GET/PUT/DELETE /api/v1/user-teams` (query: `user_id` or `team_id`)

- **Invitations** (public: by-token, accept; protected: create, list, etc.)
  - `GET /api/v1/invitations/by-token/:token`, `POST /api/v1/invitations/accept` (public)
  - `POST/GET/DELETE /api/v1/invitations` (protected)

- **Environments**
  - `POST/GET/PUT/DELETE /api/v1/environments` (query: `organization_id`)

- **Entities**
  - `POST/GET/PUT/DELETE /api/v1/entities` (query: `organization_id`, optional `environment_id`)
  - `GET /api/v1/entities/with-health-check` (query: optional `environment_id`) — list entities that have a health check URL configured (for CLI/automation)

- **Relationships**
  - `POST/GET/PUT/DELETE /api/v1/relationships` (query: `organization_id`)

- **User environment access**
  - `POST/GET/PUT/DELETE /api/v1/user-environment-access` (query: `user_id` or `environment_id`)

- **Integrations** (OAuth connect, disconnect, state; some routes public)
  - `GET/POST/DELETE /api/v1/integrations/...` (e.g. connect callback, list)

- **Dashboard** (under `/api/v1/organizations/:orgId/...`) — GitHub/GCloud dashboard data
  - Dashboard handlers mounted under `protected.Group("/organizations")`

- **Repositories** (under `/api/v1/organizations/:orgId/...`) — org-linked repositories

- **Org services** (under `/api/v1/organizations/:orgId/...`) — organization-level services

---

## Data model (high level)

- **organizations** — id (UUID), name, timestamps
- **users** — id, email (unique), name, role (admin/member), timestamps
- **environments** — id, organization_id, name (prod/staging/dev), created_at
- **entities** — id, organization_id, environment_id, type, name, status, owner_team_id, metadata (JSONB), health_check_url, health_check_method, health_check_headers (JSONB), health_check_expected_status, timestamps
- **relationships** — id, organization_id, from_entity_id, to_entity_id, type, metadata (JSONB), created_at
- **user_environment_access** — id, user_id, environment_id, role, created_at; UNIQUE(user_id, environment_id)

This structure lets you run Nervum as:

- A **single-tenant internal tool** (one organization) or
- A **multi-tenant SaaS** (multiple organizations with isolated data).

---

## Health check automation

Entities can have an optional **health check** configuration (URL, method, headers, expected HTTP status). A CLI binary runs on a schedule, probes each configured endpoint, and updates the entity’s status to `healthy` or `critical` based on the result.

- **Configure in the UI**: When adding or editing a component (entity) on the map, open the “Health check (automation)” section and set the URL (and optionally method, headers, expected status).
- **Run the checker**: Use the `healthcheck` CLI with API URL and service token. See [docs/HEALTHCHECK.md](docs/HEALTHCHECK.md) for env vars, cron example, and exit code behavior.

**Server (API)** — optional env for Bearer-token (CLI) auth:

- `NERVUM_SERVICE_TOKEN` — shared secret; when `Authorization: Bearer <token>` matches, the request is authenticated.
- `NERVUM_SERVICE_USER_ID` — UUID of an existing user (in the desired org) to act as when using the service token. That user must have `organization_id` set.

**CLI** — build and run. Two modes:

- **API mode** (CLI calls the API; use from any host): set `NERVUM_API_URL` and `NERVUM_SERVICE_TOKEN`.
- **DB mode** (CLI connects to Postgres directly; no token): set `DB_*` env vars (same as the API). `NERVUM_ORGANIZATION_ID` is optional — when unset, all entities with a health check (all orgs) are checked; when set, scope to that org.

```bash
go build -o healthcheck ./cmd/healthcheck
# API mode:
NERVUM_API_URL=http://localhost:8080 NERVUM_SERVICE_TOKEN=your-secret ./healthcheck
# DB mode, global (all orgs):
DB_HOST=localhost DB_NAME=nervum DB_USER=postgres DB_PASSWORD=postgres ./healthcheck
# DB mode, scoped to one org (optional):
DB_HOST=localhost DB_NAME=nervum DB_USER=postgres DB_PASSWORD=postgres NERVUM_ORGANIZATION_ID=<org-uuid> ./healthcheck
```

Optional: `NERVUM_ENVIRONMENT_ID` or `-env <uuid>` to scope checks to one environment. Exit code: 0 if all checks passed, 1 if any failed.

---

## Testing

- **Unit tests** (SQLite in-memory, no Postgres required):

  ```bash
  go test ./internal/...
  ```

- **Integration tests** (real Postgres, optional):

  ```bash
  # Set DB_HOST, DB_USER, DB_PASSWORD, DB_NAME (e.g. nervum_test)
  go test -tags=integration ./internal/database/...
  ```

---

## Deploying as a SaaS backend

You can deploy `nervum-go` like any Go HTTP API:

- **Containerized**: build a Docker image and run on Fly.io, Render, Railway, ECS, or Kubernetes.
- **Bare metal / VM**: compile a static binary and run behind Nginx or another reverse proxy.

At minimum you’ll need to configure:

- **DATABASE URL** (via `DB_*` env vars)
- **APP_ENV** (e.g. `production`)
- **PORT** and any **HTTP proxy / TLS** in front of the service

Then point your hosted `nervum-ui` at the public API base URL.

---

## Documentation

- **API reference**: [openapi/openapi.yaml](openapi/openapi.yaml) – OpenAPI 3 spec for all routes, schemas, and auth. See [openapi/README.md](openapi/README.md) for how to view it (e.g. Swagger UI) or generate clients. Run `make openapi-serve` to serve Swagger UI locally (requires Docker).
- **Architecture and contributing**: [docs/README.md](docs/README.md) – Index of architecture, contributing guide, and design notes.
- **Package docs**: run `go doc ./internal/...` or `make docs` to browse godoc for the codebase.

---

## Roadmap & design notes

- **Roadmap**: see [ROADMAP.md](ROADMAP.md) for planned phases (RBAC, multi-tenant hardening, graph APIs, caching, observability, etc.).
- **Design notes**: see [posts.md](posts.md) for discussions on entities, relationships, and access control.

---

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [docs/contributing.md](docs/contributing.md). You can:

- Adapt it as a **template** for your own SaaS backend.
- Propose improvements to layout, testing, or data modeling.

Feel free to open an issue or PR in your fork and iterate from there.

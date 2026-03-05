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
  pkg/types/                # Shared types (e.g. JSONB)
  features/
    organizations/          # Tenants / orgs
    users/                  # Accounts
    environments/           # Environments per org (prod/staging/dev...)
    entities/               # Nodes in the environment map
    relationships/          # Edges between entities
    user_environment_access/# RBAC-style environment access
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

These endpoints back the main SaaS features (exact routes may evolve over time; check the code for the latest definitions):

- **Health**
  - `GET /health`

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

- **Environments**
  - `POST/GET/PUT/DELETE /api/v1/environments` (query: `organization_id`)

- **Entities**
  - `POST/GET/PUT/DELETE /api/v1/entities` (query: `organization_id`, optional `environment_id`)

- **Relationships**
  - `POST/GET/PUT/DELETE /api/v1/relationships` (query: `organization_id`)

- **User environment access**
  - `POST/GET/PUT/DELETE /api/v1/user-environment-access` (query: `user_id` or `environment_id`)

---

## Data model (high level)

- **organizations** — id (UUID), name, timestamps
- **users** — id, email (unique), name, role (admin/member), timestamps
- **environments** — id, organization_id, name (prod/staging/dev), created_at
- **entities** — id, organization_id, environment_id, type, name, status, owner_team_id, metadata (JSONB), timestamps
- **relationships** — id, organization_id, from_entity_id, to_entity_id, type, metadata (JSONB), created_at
- **user_environment_access** — id, user_id, environment_id, role, created_at; UNIQUE(user_id, environment_id)

This structure lets you run Nervum as:

- A **single-tenant internal tool** (one organization) or
- A **multi-tenant SaaS** (multiple organizations with isolated data).

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

## Roadmap & design notes

- **Roadmap**: see [ROADMAP.md](ROADMAP.md) for planned phases (RBAC, multi-tenant hardening, graph APIs, caching, observability, etc.).
- **Design notes**: see [posts.md](posts.md) for discussions on entities, relationships, and access control.

---

## Contributing

This repo is intentionally small and focused. You can:

- Adapt it as a **template** for your own SaaS backend.
- Propose improvements to layout, testing, or data modeling.

Feel free to open an issue or PR in your fork and iterate from there.

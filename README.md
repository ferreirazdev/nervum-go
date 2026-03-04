# Nervum API (nervum-go)

Feature-oriented Go API for **Nervum** — software environment maps. Built with **GORM** and **Postgres**, focused on clear domain boundaries, testability, and a production-friendly layout. Pairs with [nervum-ui](https://github.com/nervum/nervum-ui) for the frontend.

Use this project as:

- **Starter kit** for new Go APIs
- **Reference implementation** of feature-first folder structure
- **Playground** for experimenting with entities/relationships modeling

## Tech stack

- **Language**: Go
- **Database**: Postgres (SQLite in unit tests)
- **ORM**: GORM
- **HTTP**: Gin
- **Architecture**: Feature-oriented, clean separation between config, database, and features

## Project structure

```text
cmd/api/                    # Entrypoint
internal/
  config/                   # Env-based config
  database/                 # GORM connection, migrations, test DB
  pkg/types/                # Shared types (e.g. JSONB)
  features/
    organizations/          # model, repository, handler
    users/
    environments/
    entities/
    relationships/
    user_environment_access/
```

## Getting started

1. **Create a Postgres database**

   ```sql
   CREATE DATABASE nervum;
   ```

2. **Configure environment variables**

   ```bash
   cp .env.example .env
   # Edit .env: DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, PORT
   ```

3. **Install dependencies and run the API**

   ```bash
   go mod download
   go run ./cmd/api
   ```

   The API is available at `http://localhost:8080/api/v1` (or the `PORT` set in `.env`).

## HTTP endpoints

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

## Testing

- **Unit tests** (in-memory SQLite, no Postgres required):

  ```bash
  go test ./internal/...
  ```

- **Integration tests** (real Postgres, optional):

  ```bash
  # Set DB_HOST, DB_USER, DB_PASSWORD, DB_NAME (e.g. nervum_test)
  go test -tags=integration ./internal/database/...
  ```

## Data model (high level)

- **organizations** — id (UUID), name, timestamps
- **users** — id, email (unique), name, role (admin/member), timestamps
- **environments** — id, organization_id, name (prod/staging/dev), created_at
- **entities** — id, organization_id, environment_id, type, name, status, owner_team_id, metadata (JSONB), timestamps
- **relationships** — id, organization_id, from_entity_id, to_entity_id, type, metadata (JSONB), created_at
- **user_environment_access** — id, user_id, environment_id, role, created_at; UNIQUE(user_id, environment_id)

## Roadmap & design notes

- **Roadmap**: see [ROADMAP.md](ROADMAP.md) for planned phases (RBAC, multi-tenant, graph APIs, caching, observability, etc.).
- **Design notes**: see [posts.md](posts.md) for discussions on entities, relationships, and access control.

## Related

- **Frontend**: [nervum-ui](https://github.com/nervum/nervum-ui) — React/Vite app for environment maps and entities.

## Contributing

This repo is small, focused, and easy to understand. To adapt it as a starter, propose layout/testing/data-model improvements, or contribute back, open issues or PRs in your fork and iterate from there.

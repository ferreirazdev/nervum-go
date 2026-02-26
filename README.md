# Nervum API

Go API boilerplate with feature-oriented layout, GORM, and Postgres.

## Layout

```
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

## Setup

1. Create a Postgres database (e.g. `nervum`).
2. Copy env and set values:

```bash
cp .env.example .env
# Edit .env: DB_HOST, DB_USER, DB_PASSWORD, DB_NAME, PORT
```

3. Run:

```bash
go mod download
go run ./cmd/api
```

API base: `http://localhost:8080/api/v1` (or `PORT` from env).

## Endpoints

- `GET /health` – health check
- `POST/GET/PUT/DELETE /api/v1/organizations`
- `POST/GET/PUT/DELETE /api/v1/users`
- `POST/GET/PUT/DELETE /api/v1/environments` (query: `organization_id`)
- `POST/GET/PUT/DELETE /api/v1/entities` (query: `organization_id`, optional `environment_id`)
- `POST/GET/PUT/DELETE /api/v1/relationships` (query: `organization_id`)
- `POST/GET/PUT/DELETE /api/v1/user-environment-access` (query: `user_id` or `environment_id`)

## Tests

- **Unit tests** (in-memory SQLite, no Postgres):

```bash
go test ./internal/...
```

- **Integration tests** (real Postgres, optional):

```bash
# Set DB_HOST, DB_USER, DB_PASSWORD, DB_NAME (e.g. nervum_test)
go test -tags=integration ./internal/database/...
```

## Schema

- **organizations** – id (UUID), name, timestamps
- **users** – id, email (unique), name, role (admin/member), timestamps
- **environments** – id, organization_id, name (prod/staging/dev), created_at
- **entities** – id, organization_id, environment_id, type, name, status, owner_team_id, metadata (JSONB), timestamps
- **relationships** – id, organization_id, from_entity_id, to_entity_id, type, metadata (JSONB), created_at
- **user_environment_access** – id, user_id, environment_id, role, created_at; UNIQUE(user_id, environment_id)

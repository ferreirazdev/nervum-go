# Backend architecture

This document summarizes the Nervum API backend layout, request flow, authentication, and multi-tenant model. For setup and deployment, see the root [README.md](../README.md).

## High-level layout

- **Language**: Go  
- **HTTP**: Gin  
- **Database**: Postgres (GORM); SQLite in-memory for unit tests  
- **Layout**: Feature-oriented under `internal/features/*`

```text
cmd/api/                    Entrypoint (main service)
internal/
  config/                   Env-based config (PORT, DB_*)
  database/                 GORM connection, migrations, test DB
  pkg/types/                Shared types (e.g. JSONB)
  features/
    auth/                   Sessions, login/register/logout, RequireAuth middleware
    organizations/          Tenants (orgs)
    users/                   Accounts + role helpers (CanInvite, CanManageTeams, …)
    teams/                   Teams and team–environment links
    user_teams/              User–team membership
    invitations/             Invite-by-email, get-by-token, accept (public + protected)
    environments/            Environments per org (prod/staging/dev)
    entities/                Map nodes (services, DBs, etc.)
    relationships/           Edges between entities
    user_environment_access/  RBAC-style access per user–environment
```

## Request flow

1. **Gin** receives the request and runs global middleware (e.g. CORS).
2. **Protected routes** use `auth.RequireAuth`: read session cookie → load session from DB → load user → set user in context.
3. **Handler** reads the authenticated user (if any) from context, parses path/query/body, and calls the **repository** for the feature.
4. **Repository** uses GORM to read/write the database; returns domain types or errors.
5. Handler returns JSON and appropriate status codes (400, 401, 403, 404, 500).

There is no separate “service” layer; handlers orchestrate repositories and permission helpers (e.g. `user.CanManageEnvironments(role)`).

## Authentication and sessions

- **Session-based**: Login and register create a `Session` row and set a cookie `nervum_session` (UUID).  
- **RequireAuth** middleware: validates cookie, loads session (and checks expiry), loads user, sets `auth_user` in Gin context.  
- **Protected routes** are mounted under a group that uses `RequireAuth`; public routes (e.g. `/api/v1/auth/register`, `/api/v1/auth/login`, `/api/v1/invitations/by-token/:token`, `/api/v1/invitations/accept`) do not require auth.

## Multi-tenant and access

- **Organizations** are the tenant boundary. Users have an `organization_id`; data (environments, entities, teams, etc.) is scoped by `organization_id`.
- **Environments** belong to an organization. Access can be restricted so that only users with the right role or with explicit **user_environment_access** (or team membership in a team that has the environment) can view/edit.
- **Roles** (admin, manager, member) are used for permission helpers: who can manage teams, invite users, manage environments, list all org members, etc. See `internal/features/users/permissions.go` and the OpenAPI spec for which routes enforce which checks.

## Data flow (summary)

- **Organizations** → **Environments** (one org has many environments).  
- **Entities** belong to an org and an environment (nodes on the map).  
- **Relationships** link two entities (same org).  
- **Teams** belong to an org and can be linked to environments via `team_environments`.  
- **User–team** and **user_environment_access** determine visibility and edit rights for non-admin users.

For the full data model and table names, see the root README “Data model” section and the OpenAPI schemas in [openapi/openapi.yaml](../openapi/openapi.yaml).

## Where to find things

- **Add a new endpoint**: Add the handler method and route in the feature’s `handler.go`, register the route group in `cmd/api/main.go`, and add the path and schemas to `openapi/openapi.yaml`.
- **Change permissions**: Update `internal/features/users/permissions.go` and the handlers that call those helpers.
- **New feature**: Create a new package under `internal/features/<name>` with model, repository, and handler; register in main and run migrations for new tables. See [contributing.md](contributing.md).

# Health check automation runbook

This document describes how to run the **entity health check** automation: the CLI that probes endpoints configured on entities and updates their status (`healthy` / `critical`).

## Overview

1. **Configure entities** — In the Nervum UI, add or edit a component (entity) and set a **Health check** URL (and optionally method, headers, expected status). Only entities with a non-empty health check URL are checked.
2. **Run the CLI** — The `healthcheck` binary either calls the API (API mode) or connects to the database directly (DB mode). It performs an HTTP request to each entity’s URL and updates status accordingly.
3. **Schedule** — Run the CLI from cron (or a job scheduler) so status stays up to date.

The CLI supports two modes:

- **API mode** — Uses `NERVUM_API_URL` and `NERVUM_SERVICE_TOKEN`. The CLI talks to the Nervum API over HTTP. Use when the CLI runs on a different host than the API or in CI.
- **DB mode** — Uses the same `DB_*` env vars as the API. The CLI connects to Postgres directly; no API and no token. Use when the CLI runs where the database is reachable (e.g. same host as the API). `NERVUM_ORGANIZATION_ID` is **optional**: when unset, all entities with a health check URL (across all organizations) are checked; when set, only that org is checked.

## Prerequisites

- At least one **entity** with a health check URL set (done in the UI).
- **API mode**: API must be running and reachable; service token auth configured (see below).
- **DB mode**: Postgres reachable; `DB_*` set. `NERVUM_ORGANIZATION_ID` is optional (global when unset).

## Server configuration (API)

To allow the CLI to authenticate without a browser session, set:

| Variable | Description |
|----------|-------------|
| `NERVUM_SERVICE_TOKEN` | Shared secret. When the CLI sends `Authorization: Bearer <value>`, the API treats the request as authenticated if the value matches. |
| `NERVUM_SERVICE_USER_ID` | UUID of an **existing user** in your database. That user must belong to an organization (`organization_id` set). The CLI will act as this user; it will only see and update entities in that user’s organization. |

Create a dedicated user (e.g. “Health check automation”) in the desired organization and use its UUID for `NERVUM_SERVICE_USER_ID`. Keep `NERVUM_SERVICE_TOKEN` secret and rotate it if compromised.

## Building the CLI

From the `nervum-go` repo:

```bash
go build -o healthcheck ./cmd/healthcheck
```

This produces a `healthcheck` binary you can run locally or deploy next to your cron.

## Environment variables (CLI)

**API mode** (use when you set `NERVUM_API_URL` and `NERVUM_SERVICE_TOKEN`):

| Variable | Required | Description |
|----------|----------|-------------|
| `NERVUM_API_URL` | Yes (API mode) | Base URL of the API (e.g. `http://localhost:8080` or `https://api.example.com`). No trailing slash. |
| `NERVUM_SERVICE_TOKEN` | Yes (API mode) | Must match the value of `NERVUM_SERVICE_TOKEN` on the server. |
| `NERVUM_ENVIRONMENT_ID` | No | If set, only entities in this environment are checked. |

**DB mode** (use when the CLI runs where Postgres is reachable; no API or token):

| Variable | Required | Description |
|----------|----------|-------------|
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | Yes (DB mode) | Same as the API (see main README). |
| `NERVUM_ORGANIZATION_ID` | No | When set, scope health checks to this organization only. When **unset**, all entities with a health check URL (across all orgs) are checked (global). |
| `NERVUM_ENVIRONMENT_ID` or `-env` | No | If set, only entities in this environment are checked. |

You can pass the environment scope with a flag in both modes:

```bash
./healthcheck -env <environment-uuid>
```

## Running the CLI

**API mode:**

```bash
export NERVUM_API_URL=http://localhost:8080
export NERVUM_SERVICE_TOKEN=your-shared-secret
./healthcheck
```

**DB mode** (no API, no token; same host as API or anywhere with DB access). Global (all orgs):

```bash
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres
export DB_NAME=nervum
./healthcheck
```

Optional: scope to one organization:

```bash
export NERVUM_ORGANIZATION_ID=<your-org-uuid>
./healthcheck
```

Optional: scope to one environment (both modes):

```bash
./healthcheck -env a1b2c3d4-e5f6-7890-abcd-ef1234567890
```

## Exit codes

- **0** — All entities with a health check URL were probed and reported OK; their status was set to `healthy` (or left as-is if already healthy).
- **1** — At least one check failed (non-OK response or network error); those entities’ status was set to `critical`. Useful for alerting (e.g. cron job fails and your scheduler sends a notification).

## Cron example

Run every 5 minutes and log output:

```bash
*/5 * * * * cd /opt/nervum && NERVUM_API_URL=https://api.example.com NERVUM_SERVICE_TOKEN=secret ./healthcheck >> /var/log/nervum-healthcheck.log 2>&1
```

Or run only during business hours:

```bash
0 * * * * cd /opt/nervum && NERVUM_API_URL=https://api.example.com NERVUM_SERVICE_TOKEN=secret ./healthcheck
```

## Behavior summary

- **API mode**: The CLI calls `GET /api/v1/entities/with-health-check` (with optional `environment_id`). The API returns only entities that have `health_check_url` set (in the service user’s org).
- **DB mode**: The CLI queries the database for entities with `health_check_url` set. If `NERVUM_ORGANIZATION_ID` is set, only that org is queried; otherwise all organizations (global).
- For each entity it performs an HTTP request: URL from the entity, method (default `GET`), headers from the entity, timeout 10 seconds.
- If the response status code equals the entity’s expected status (default 200), the entity is marked **OK** and its status is set to `healthy`.
- If the response status differs or the request fails (timeout, connection error), the entity is marked **FAIL** and its status is set to `critical`.
- The CLI then calls `PUT /api/v1/entities/:id` with `{ "status": "healthy" }` or `{ "status": "critical" }` to persist the result.

No retries are performed in the initial implementation; you can run the CLI more frequently if you want quicker recovery.

# Nervum backend documentation

This folder is the index for architecture, runbooks, and contributing guides. For the API surface and request/response shapes, see the [OpenAPI spec](../openapi/openapi.yaml) and [openapi/README.md](../openapi/README.md).

## Contents

- **[architecture.md](architecture.md)** – Backend layout, request flow, auth/sessions, multi-tenant and environment access.
- **[contributing.md](contributing.md)** – How to set up, run tests, and add a new feature.
- **[HEALTHCHECK.md](HEALTHCHECK.md)** – Health check automation runbook: CLI env vars, server config, cron example, exit codes.

## Other references

- **API reference**: [openapi/openapi.yaml](../openapi/openapi.yaml) – Full OpenAPI 3 spec. Use Swagger UI or your preferred tool to view or generate clients.
- **Package docs**: Run `go doc ./internal/...` or `make docs` to browse godoc for the codebase.
- **Roadmap**: [ROADMAP.md](../ROADMAP.md) – Planned phases (RBAC, multi-tenant, graph APIs, observability, etc.).
- **Design notes**: [posts.md](../posts.md) – Decisions and tradeoffs (e.g. feature-oriented structure).

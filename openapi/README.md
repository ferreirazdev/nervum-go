# OpenAPI spec for Nervum API

This folder contains the [OpenAPI 3](https://spec.openapis.org/oas/v3.0.3) specification for the Nervum backend API.

## File

- **openapi.yaml** – Full API definition: paths, request/response schemas, auth (cookie/session), and error responses.

## How to use this spec

- **Swagger UI**: Use [Swagger Editor](https://editor.swagger.io/) (paste or load `openapi.yaml`) or run Swagger UI locally (e.g. `docker run -p 8081:8080 -e SWAGGER_JSON=/spec/openapi.yaml -v $(pwd):/spec swaggerapi/swagger-ui`).
- **Codegen**: Generate clients or server stubs with [OpenAPI Generator](https://openapi-generator.tech/) or [oapi-codegen](https://github.com/deepmap/oapi-codegen) (Go).
- **Postman**: Import `openapi.yaml` via File → Import.

## Serving API docs locally

From the repo root:

```bash
make openapi-serve   # Serves Swagger UI for openapi/openapi.yaml (if configured)
# or
make docs            # Serves godoc; see README "Documentation" section
```

The canonical API reference is the OpenAPI spec; keep it in sync when adding or changing routes.

run:
	go run cmd/api/main.go

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down $(N)

migrate-version:
	go run ./cmd/migrate version

migrate-force:
	go run ./cmd/migrate force $(V)

migrate-create:
	@read -p "Migration name (snake_case): " name; \
	count=$$(ls migrations/*.sql 2>/dev/null | wc -l | tr -d ' '); \
	seq=$$(printf "%06d" $$(( $$count / 2 + 1 ))); \
	touch migrations/$${seq}_$${name}.up.sql migrations/$${seq}_$${name}.down.sql; \
	echo "Created migrations/$${seq}_$${name}.up.sql and migrations/$${seq}_$${name}.down.sql"

cli:
	go run cmd/healthcheck/main.go
test:
	go test ./...

test-integration:
	go test -tags=integration ./...

# Serve godoc at http://localhost:6060. Browse packages under .../internal/...
# OpenAPI spec is in openapi/openapi.yaml.
docs:
	@echo "Serving godoc at http://localhost:6060 (packages: .../internal/...)"
	@echo "OpenAPI spec: openapi/openapi.yaml"
	go run golang.org/x/tools/cmd/godoc@latest -http=:6060

# Serve Swagger UI for openapi/openapi.yaml at http://localhost:8081 (requires Docker).
openapi-serve:
	@echo "Serving Swagger UI at http://localhost:8081"
	docker run --rm -p 8081:8080 -e SWAGGER_JSON=/spec/openapi.yaml -v $(PWD)/openapi:/spec swaggerapi/swagger-ui
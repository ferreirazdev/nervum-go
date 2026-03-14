run:
	go run cmd/api/main.go

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
run:
	go run cmd/api/main.go

test:
	go test ./...

test-integration:
	go test -tags=integration ./...
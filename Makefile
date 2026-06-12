.PHONY: run build test docker-up docker-down tidy

# Run app locally
run:
	go run ./cmd/main.go

# Build binary
build:
	go build -o bin/chatapp ./cmd/main.go

# Run tests
test:
	go test ./... -v

# Download dependencies
tidy:
	go mod tidy

# Start PostgreSQL via Docker
docker-up:
	docker-compose up -d db

# Stop Docker
docker-down:
	docker-compose down

# Full stack with Docker
docker-all:
	docker-compose up --build

# Format code
fmt:
	gofmt -w .

# Lint
lint:
	golangci-lint run ./...

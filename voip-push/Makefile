.PHONY: build test clean run docker-build docker-up docker-down lint fmt

# Variables
BINARY_NAME=voip-push
DOCKER_IMAGE=voip-push:latest
PORT=8084

# Build the application
build:
	go build -o $(BINARY_NAME) ./cmd

# Run tests
test:
	go test ./... -v

# Run tests with coverage
test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Run the application locally
run:
	go run ./cmd

# Format code
fmt:
	go fmt ./...

# Run linter
lint:
	go vet ./...

# Download dependencies
deps:
	go mod download
	go mod tidy

# Build Docker image
docker-build:
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .

# Start services with Docker Compose
docker-up:
	docker-compose up -d

# Stop services with Docker Compose
docker-down:
	docker-compose down

# View logs
docker-logs:
	docker-compose logs -f

# Run all checks before committing
check: fmt lint test

# Development mode with auto-reload (requires air)
dev:
	air

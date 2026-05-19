.PHONY: help build run test clean docker-up docker-down docker-logs install-deps fmt lint

help:
	@echo "Available targets:"
	@echo "  make build          - Build the Go application"
	@echo "  make run            - Run the application"
	@echo "  make test           - Run tests"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make docker-up      - Start Docker containers (FreeSWITCH + PostgreSQL)"
	@echo "  make docker-down    - Stop Docker containers"
	@echo "  make docker-logs    - View Docker container logs"
	@echo "  make install-deps   - Install Go dependencies"
	@echo "  make fmt            - Format Go code"
	@echo "  make lint           - Run linter"

build:
	@echo "Building application..."
	@go build -o bin/freeswitch-cdr ./main.go

run:
	@echo "Running application..."
	@go run ./main.go

test:
	@echo "Running tests..."
	@go test -v ./...

clean:
	@echo "Cleaning..."
	@rm -rf bin/

install-deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

fmt:
	@echo "Formatting code..."
	@go fmt ./...

lint:
	@echo "Running linter..."
	@golangci-lint run ./...

docker-up:
	@echo "Starting Docker containers..."
	@docker-compose up -d

docker-down:
	@echo "Stopping Docker containers..."
	@docker-compose down

docker-logs:
	@docker-compose logs -f

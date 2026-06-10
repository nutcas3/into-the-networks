.PHONY: build test clean run docker-build docker-up docker-down lint fmt

BINARY_NAME=wireguard-voip
DOCKER_IMAGE=wireguard-voip:latest
PORT=8085

build:
	go build -o $(BINARY_NAME) ./cmd

test:
	go test ./... -v

test-coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

run:
	go run ./cmd

fmt:
	go fmt ./...

lint:
	go vet ./...

deps:
	go mod download
	go mod tidy

docker-build:
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

docker-logs:
	docker-compose logs -f

check: fmt lint test

dev:
	air

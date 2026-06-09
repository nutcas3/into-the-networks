.PHONY: build test clean run docker-build docker-up docker-down lint fmt

BINARY_NAME=amd-system
DOCKER_IMAGE=amd-system:latest
PORT=8086

build:
	go build -o $(BINARY_NAME) ./cmd

test:
	go test ./... -v

clean:
	rm -f $(BINARY_NAME)

run:
	go run ./cmd

run-ml:
	cd ml && python -m uvicorn src.main:app --host 0.0.0.0 --port 5000

fmt:
	go fmt ./...

lint:
	go vet ./...

deps:
	go mod download
	go mod tidy

docker-build:
	docker build -f docker/Dockerfile -t $(DOCKER_IMAGE) .
	docker build -f ml/Dockerfile -t amd-ml:latest ./ml

docker-up:
	docker-compose up -d

docker-down:
	docker-compose down

check: fmt lint test

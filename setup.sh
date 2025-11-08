#!/bin/bash

set -e

echo "Setting up Products Service..."

# Check if go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21 or higher."
    exit 1
fi

echo "Installing dependencies (buf, protoc-gen-go, protoc-gen-go-grpc)..."
make deps

echo "Downloading Go modules..."
go mod download

echo "Generating protocol buffers using buf..."
make proto

echo "Building services..."
make build-products build-loadtest

echo "Setup complete!"
echo ""
echo "Next steps:"
echo "1. Start infrastructure: docker-compose up -d"
echo "2. Run products service: ./bin/products-service"
echo "3. Run load test: ./bin/load-test -vusers 10 -rpm 60 -duration 5m"
echo "4. Open Grafana: http://localhost:3000"


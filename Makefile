.PHONY: proto build-products build-loadtest run-products run-loadtest clean deps

proto: deps
	@if ! command -v buf >/dev/null 2>&1; then \
		echo "Error: buf is not installed."; \
		echo "Installing buf..."; \
		go install github.com/bufbuild/buf/cmd/buf@latest; \
	fi
	@export PATH=$$PATH:$$(go env GOPATH)/bin && buf generate

build-products: proto
	go build -o bin/products-service ./cmd/products-service

build-loadtest: proto
	go build -o bin/load-test ./cmd/load-test

run-products: build-products
	./bin/products-service

run-loadtest: build-loadtest
	./bin/load-test -addr localhost:50051 -vusers 10 -rpm 60 -duration 5m

clean:
	rm -rf bin/
	rm -f proto/*.pb.go

deps:
	go mod download
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest


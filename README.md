# Products Service with Load Testing

This project contains two Go services:
1. **Products Service**: A gRPC service that manages products for an online store, with Redis/RedisSearch backend and full observability (metrics, traces, logs) integrated with Grafana.
2. **Load Testing Service**: A service that generates configurable load against the products service with configurable virtual users and requests per minute (RPM).

## Architecture

- **Products Service**: gRPC server with Redis/RedisSearch for data storage
- **Observability Stack**: Prometheus (metrics), Tempo (traces), Loki (logs), Grafana (visualization)
- **Load Testing**: Configurable virtual users and RPM for generating realistic load

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Redis with RedisSearch (included in docker-compose)

## Setup

### 1. Install Dependencies

```bash
make deps
```

This will install:
- Go dependencies
- `buf` (modern Protobuf tool, installed via Go)
- `protoc-gen-go` and `protoc-gen-go-grpc` plugins

### 2. Generate gRPC Code

```bash
make proto
```

This uses `buf` to generate the Go code from your `.proto` files. `buf` is installed automatically via `go install` - no need for system-wide `protoc` installation!

### 3. Start Infrastructure

Start Redis and the observability stack:

```bash
docker-compose up -d
```

This will start:
- Redis with RedisSearch on port 6379
- Prometheus on port 9090
- Loki on port 3100
- Tempo on port 3200
- Grafana on port 3000

### 4. Build Services

```bash
make build-products
make build-loadtest
```

Or build both:

```bash
make build-products build-loadtest
```

### 5. Run Products Service

```bash
make run-products
```

Or directly:

```bash
./bin/products-service
```

The service will:
- Listen on port 50051 (gRPC)
- Expose metrics on port 2112
- Send traces to Tempo (Jaeger endpoint)
- Log to stdout (structured JSON)

### 6. Run Load Testing Service

```bash
./bin/load-test -addr localhost:50051 -vusers 10 -rpm 60 -duration 5m
```

Options:
- `-addr`: gRPC server address (default: localhost:50051)
- `-vusers`: Number of virtual users (default: 10)
- `-rpm`: Requests per minute (default: 60)
- `-duration`: Test duration (default: 5m)

Example with higher load:

```bash
./bin/load-test -vusers 50 -rpm 300 -duration 10m
```

## Grafana Dashboard

1. Open Grafana at http://localhost:3000
2. Navigate to Dashboards
3. The "Products Service Dashboard" should be available with:
   - Request rate (requests per second)
   - Request duration (p95, p99)
   - Error rate
   - Logs

## API Endpoints

The products service exposes the following gRPC methods:

- `ListProducts`: List products with pagination, category filter, and search
- `GetProduct`: Get a single product by ID
- `CreateProduct`: Create a new product

## Configuration

Environment variables:

- `GRPC_PORT`: gRPC server port (default: 50051)
- `REDIS_ADDR`: Redis address (default: localhost:6379)
- `JAEGER_ENDPOINT`: Jaeger/Tempo endpoint for traces (default: http://localhost:14268/api/traces)
- `METRICS_PORT`: Prometheus metrics port (default: 2112)
- `ENVIRONMENT`: Environment name (default: development)

## Project Structure

```
.
├── cmd/
│   ├── products-service/  # Products gRPC service
│   └── load-test/         # Load testing service
├── internal/
│   ├── config/           # Configuration management
│   ├── observability/    # Metrics, traces, logs
│   ├── repository/       # Redis/RedisSearch repository
│   └── server/           # gRPC server implementation
├── proto/                # Protocol buffer definitions
├── monitoring/           # Grafana, Prometheus, Loki configs
├── docker-compose.yml    # Infrastructure setup
└── Makefile             # Build and run commands
```

## Load Testing Details

The load testing service:
- Simulates realistic user behavior with different operation types:
  - 70% ListProducts requests
  - 20% GetProduct requests
  - 10% CreateProduct requests
- Supports configurable virtual users and RPM
- Reports metrics every 10 seconds including:
  - Total requests
  - Success/failure counts
  - Requests per second
  - Success rate

## Development

### Running Tests

```bash
go test ./...
```

### Cleaning Build Artifacts

```bash
make clean
```

### Regenerating Protos

```bash
make proto
```

## Troubleshooting

1. **Redis connection issues**: Ensure Redis is running: `docker-compose ps`
2. **Metrics not showing**: Check that Prometheus can reach the service on port 2112
3. **Traces not appearing**: Verify Tempo is running and the endpoint is correct
4. **Logs not in Loki**: Ensure Promtail is configured to scrape log files

## License

MIT


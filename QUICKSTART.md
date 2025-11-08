# Quick Start Guide

## Prerequisites

- Go 1.21+
- Docker and Docker Compose

**Note**: No need to install `protoc` separately! We use `buf` which is installed via Go.

## Setup (One-time)

```bash
# Run the setup script
./setup.sh

# Or manually:
make deps
go mod download
make proto
make build-products build-loadtest
```

## Running the Services

### 1. Start Infrastructure

```bash
docker-compose up -d
```

This starts:
- Redis with RedisSearch (port 6379)
- Prometheus (port 9090)
- Loki (port 3100)
- Tempo (port 3200)
- Grafana (port 3000)

### 2. Start Products Service

```bash
./bin/products-service
```

The service will:
- Listen on gRPC port 50051
- Expose metrics on port 2112
- Send traces to Tempo
- Log to stdout

### 3. Run Load Test

In another terminal:

```bash
# Basic test: 10 virtual users, 60 RPM, 5 minutes
./bin/load-test

# Custom test: 50 virtual users, 300 RPM, 10 minutes
./bin/load-test -vusers 50 -rpm 300 -duration 10m

# High load test: 100 virtual users, 1000 RPM, 5 minutes
./bin/load-test -vusers 100 -rpm 1000 -duration 5m
```

### 4. View Dashboards

Open Grafana: http://localhost:3000

Navigate to Dashboards to see:
- Request rate
- Request duration (p95, p99)
- Error rate
- Logs

## Troubleshooting

**Products service won't start:**
- Check Redis is running: `docker-compose ps`
- Check port 50051 is available

**Metrics not showing in Grafana:**
- Verify Prometheus can reach the service: `curl http://localhost:2112/metrics`
- Check Prometheus targets: http://localhost:9090/targets

**Traces not appearing:**
- Verify Tempo is running: `docker-compose ps tempo`
- Check service is sending traces to correct endpoint

**Load test connection errors:**
- Ensure products service is running
- Check the server address: `./bin/load-test -addr localhost:50051`


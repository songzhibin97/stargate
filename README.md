# Stargate - High-Performance API Gateway

<div align="center">

![Stargate Logo](https://img.shields.io/badge/Stargate-API%20Gateway-blue?style=for-the-badge)

[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen?style=flat-square)](https://github.com/songzhibin97/stargate)
[![Coverage](https://img.shields.io/badge/Coverage-85%25-yellow?style=flat-square)](https://github.com/songzhibin97/stargate)

[English](README.md) | [中文](README_zh.md)

</div>

---

##

## Project Overview

Stargate is a modern, high-performance API Gateway designed for cloud-native environments. It provides a complete API management solution including routing, load balancing, traffic governance, authentication, authorization, and observability.

## Key Features

- **High Performance**: Built with Go, supports high-concurrency request processing
- **Smart Routing**: Multi-dimensional routing (host, path, method, headers)
- **Load Balancing**: Multiple algorithms (round-robin, weighted, IP hash, least connections)
- **Security**: API Key, JWT, OAuth2 authentication methods
- **Traffic Governance**: Rate limiting, circuit breaking, traffic mirroring
- **Health Checks**: Active and passive health checking with automatic failover
- **Observability**: Complete monitoring, logging, and distributed tracing
- **Developer Portal**: User-friendly web interface with API docs and testing tools
- **Easy Configuration**: YAML/JSON config with dynamic updates
- **Cloud Native**: Docker and Kubernetes ready

## System Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client Apps   │    │  Developer      │    │   Admin Panel   │
│                 │    │  Portal         │    │                 │
└─────────┬───────┘    └─────────┬───────┘    └─────────┬───────┘
          │                      │                      │
          │              ┌───────▼───────┐              │
          │              │               │              │
          └──────────────►   Stargate    ◄──────────────┘
                         │   Gateway     │
                         └───────┬───────┘
                                 │
                    ┌────────────┼────────────┐
                    │            │            │
            ┌───────▼───────┐ ┌──▼──┐ ┌──────▼──────┐
            │   Service A   │ │ ... │ │  Service N  │
            │               │ │     │ │             │
            └───────────────┘ └─────┘ └─────────────┘
```

## Quick Start

### Prerequisites

- Go 1.21 or higher
- Node.js 18+ (for frontend development)
- Redis (optional, for distributed rate limiting)
- PostgreSQL/MySQL (optional, for configuration storage)

### Installation

**1. Clone the repository**
```bash
git clone https://github.com/songzhibin97/stargate.git
cd stargate
```

**2. Build backend**
```bash
# Install dependencies
go mod tidy

# Build binary
make build

# Or run directly
go run cmd/stargate/main.go
```

**3. Build frontend**
```bash
cd web/developer-portal
npm install
npm run build
```

**4. Start the service**
```bash
# Start with default config
./bin/stargate

# Or specify config file
./bin/stargate -config config/stargate.yaml
```

**5. Verify installation**
```bash
# Check service status
curl http://localhost:8080/health

# Access developer portal
open http://localhost:8080
```

### Docker Deployment

```bash
# Build image
docker build -t stargate:latest .

# Run container
docker run -d \
  --name stargate \
  -p 8080:8080 \
  -p 9090:9090 \
  -v $(pwd)/config:/app/config \
  stargate:latest
```

### Kubernetes Deployment

```bash
# Apply Kubernetes manifests
kubectl apply -f deployments/kubernetes/

# Check deployment status
kubectl get pods -l app=stargate
```

## Basic Usage

### 1. Configure Upstream Services

```yaml
# config/stargate.yaml
upstreams:
  - id: "user-service"
    name: "User Service"
    algorithm: "round_robin"
    targets:
      - url: "http://user-service-1:8080"
        weight: 100
      - url: "http://user-service-2:8080"
        weight: 100
    health_check:
      enabled: true
      path: "/health"
      interval: 30
```

### 2. Configure Routes

```yaml
routes:
  - id: "user-api"
    name: "User API"
    rules:
      hosts:
        - "api.example.com"
      paths:
        - type: "prefix"
          value: "/api/v1/users"
      methods:
        - "GET"
        - "POST"
    upstream_id: "user-service"
    priority: 100
```

### 3. Enable Authentication

```yaml
auth:
  enabled: true
  methods:
    jwt:
      enabled: true
      secret: "your-jwt-secret"
      algorithm: "HS256"
```

### 4. Configure Rate Limiting

```yaml
rate_limit:
  enabled: true
  per_ip:
    requests_per_minute: 60
    burst: 10
```

## Monitoring & Observability

Stargate provides comprehensive observability:

- **Metrics**: Prometheus format metrics with Grafana visualization
- **Logging**: Structured logging with multiple output formats
- **Tracing**: Support for Jaeger, Zipkin, and other tracing systems
- **Health Checks**: Built-in health check endpoints

```bash
# View metrics
curl http://localhost:9090/metrics

# Check health status
curl http://localhost:8080/health
```

## Testing

```bash
# Run unit tests
make test

# Run integration tests
make test-integration

# Run performance tests
make test-performance

# View test coverage
make test-coverage
```

## Documentation

- [Quick Start](docs/api-quickstart.md) - Get started in 5 minutes
- [Architecture](docs/architecture.md) - System architecture overview
- [API Documentation](docs/api-documentation.md) - REST API reference
- [Troubleshooting](docs/api-troubleshooting.md) - Common issues and solutions
- [SDK Documentation](docs/sdk-documentation.md) - Multi-language SDKs

## Contributing

We welcome community contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details on how to get involved.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/songzhibin97/stargate.git
cd stargate

# Install development dependencies
make dev-setup

# Run development server
make dev

# Run tests
make test
```

## License

This project is licensed under the [MIT License](LICENSE).

## Acknowledgments

Thanks to all contributors and the open source community for their support!

---

<div align="center">

**If this project helps you, please give us a Star!**

</div>

# roachprod-centralized

A centralized REST API service for managing CockroachDB roachprod clusters, tasks, and cloud provider operations.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [CLI Usage](#cli-usage)
- [Configuration](#configuration)
- [Architecture](#architecture)
- [Deployment](#deployment)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Documentation](#documentation)

## Overview

The `roachprod-centralized` service provides a unified HTTP API for:

- **Cluster Management**: Create, monitor, and manage roachprod clusters across multiple cloud providers
- **Task Processing**: Distributed background task system for cluster operations
- **DNS Management**: Public DNS record management for clusters
- **Multi-Cloud Support**: AWS, GCE, Azure, and IBM cloud provider integration
- **Health Monitoring**: Comprehensive health checks and metrics

## Quick Start

### Prerequisites

- CockroachDB development environment (run `./dev doctor` to verify)
- Access to cloud provider credentials (AWS, GCE, Azure, or IBM)

### 1. Build the Application

```bash
# From the CockroachDB repository root
./dev build roachprod-centralized
```

### 2. Basic Configuration

```bash
# Set minimum required configuration
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
export ROACHPROD_DATABASE_TYPE=memory
```

### 3. Start the API Server

```bash
./dev run roachprod-centralized api
```

The API will be available at `http://localhost:8080` with metrics at `http://localhost:8081/metrics`.

## CLI Usage

The `roachprod-centralized` command provides the following subcommands:

### API Server

```bash
# Start the API server with default configuration
roachprod-centralized api

# Start with custom configuration file
roachprod-centralized api --config /path/to/config.yaml

# Start with specific port
roachprod-centralized api --api-port 9090

# View all available options
roachprod-centralized api --help
```

### Available Flags

Key configuration flags (all can be set via environment variables):

```bash
--api-port                     HTTP API port (default: 8080)
--api-base-path               Base URL path for API endpoints
--api-metrics-enabled         Enable metrics collection (default: true)
--api-metrics-port            Metrics HTTP port (default: 8081)
--api-authentication-disabled Disable API authentication (default: false)
--database-type               Database type: memory|cockroachdb (default: memory)
--database-url                Database connection URL
--log-level                   Logging level: debug|info|warn|error (default: info)
--tasks-workers               Number of background task workers (default: 1)
```

## Configuration

### Environment Variables

All configuration can be set via environment variables with the `ROACHPROD_` prefix:

```bash
# Core API settings
export ROACHPROD_API_PORT=8080
export ROACHPROD_API_METRICS_ENABLED=true
export ROACHPROD_LOG_LEVEL=info

# Authentication (disable for development)
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
export ROACHPROD_API_AUTHENTICATION_JWT_HEADER="X-Goog-IAP-JWT-Assertion"
export ROACHPROD_API_AUTHENTICATION_JWT_AUDIENCE="your-audience"

# Database configuration
export ROACHPROD_DATABASE_TYPE=cockroachdb
export ROACHPROD_DATABASE_URL="postgresql://user:password@localhost:26257/roachprod?sslmode=require"
export ROACHPROD_DATABASE_MAX_CONNS=10

# Task processing
export ROACHPROD_TASKS_WORKERS=3
```

### Configuration File

Create a YAML configuration file for more complex setups:

```yaml
log:
  level: info
api:
  port: 8080
  base_path: "/api"
  metrics:
    enabled: true
    port: 8081
  authentication:
    disabled: false
    jwt:
      header: "X-Goog-IAP-JWT-Assertion"
      audience: "your-audience"
database:
  type: cockroachdb
  url: "postgresql://user:password@localhost:26257/roachprod?sslmode=require"
  max_conns: 10
  max_idle_time: 300
tasks:
  workers: 3
```

### Cloud Provider Configuration

See [docs/CLOUD_PROVIDER_CONFIG.md](docs/CLOUD_PROVIDER_CONFIG.md) for detailed cloud provider setup.

**Quick Examples:**
- [Development config](examples/development-config.yaml) - Minimal setup for local development
- [Production config](examples/production-config.yaml) - Full production configuration
- [Cloud config example](examples/cloud_config.yaml.example) - Multi-cloud provider setup

## Architecture

The service follows a clean architecture pattern:

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Controllers   │────│    Services     │────│  Repositories   │
│  (HTTP Layer)   │    │ (Business Logic)│    │  (Data Layer)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐             │
         └──────────────│   Background    │─────────────┘
                        │   Task System   │
                        └─────────────────┘
```

**Key Components:**
- **Controllers**: HTTP request handlers (`controllers/`)
- **Services**: Business logic and orchestration (`services/`)
- **Repositories**: Data persistence abstraction (`repositories/`)
- **Models**: Data structures and entities (`models/`)
- **Utils**: Shared utilities and helpers (`utils/`)

For detailed architecture information, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Deployment

### Production Configuration

For production deployments:

1. **Enable Authentication**:
   ```bash
   export ROACHPROD_API_AUTHENTICATION_DISABLED=false
   export ROACHPROD_API_AUTHENTICATION_JWT_AUDIENCE="your-production-audience"
   ```

2. **Use CockroachDB Backend**:
   ```bash
   export ROACHPROD_DATABASE_TYPE=cockroachdb
   export ROACHPROD_DATABASE_URL="postgresql://user:password@prod-cluster:26257/roachprod?sslmode=require"
   ```

3. **Configure Cloud Providers**: Set up cloud provider credentials as detailed in [docs/CLOUD_PROVIDER_CONFIG.md](docs/CLOUD_PROVIDER_CONFIG.md)

4. **Resource Limits**:
   ```bash
   export ROACHPROD_DATABASE_MAX_CONNS=20
   export ROACHPROD_TASKS_WORKERS=5
   ```

### Docker Deployment

See [docker/README.md](docker/README.md) for containerized deployment options.

**Quick Examples:**
- [Docker Compose setup](examples/docker-compose.yml) - Complete local environment
- [Kubernetes deployment](examples/kubernetes-deployment.yaml) - Production K8s deployment

### Health Checks

The service provides comprehensive health checks:

- **API Health**: `GET /health` - Basic API availability
- **Detailed Health**: `GET /health/detailed` - Component-level status
- **Metrics**: `GET :8081/metrics` - Prometheus metrics

## Development

For local development setup and contribution guidelines, see [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).

### Running Tests

```bash
# Run all tests
./dev test pkg/cmd/roachprod-centralized/...

# Run specific package tests
./dev test pkg/cmd/roachprod-centralized/services/clusters

# Run with race detection
./dev test pkg/cmd/roachprod-centralized/... --race
```

### Code Generation

After modifying protocol buffers or other generated code:

```bash
./dev generate
```

## Troubleshooting

### Common Issues

**1. Authentication Errors**
```
Error: authentication failed
```
**Solution**: For development, disable authentication:
```bash
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
```

**2. Database Connection Issues**
```
Error: failed to connect to database
```
**Solution**: Check database configuration and connectivity:
```bash
# For development, use in-memory storage
export ROACHPROD_DATABASE_TYPE=memory

# Or verify CockroachDB connection
psql "postgresql://user:password@localhost:26257/roachprod?sslmode=require"
```

**3. Cloud Provider Configuration**
```
Error: failed to initialize cloud provider
```
**Solution**: Verify cloud provider credentials are properly configured. See [docs/CLOUD_PROVIDER_CONFIG.md](docs/CLOUD_PROVIDER_CONFIG.md).

**4. Port Already in Use**
```
Error: bind: address already in use
```
**Solution**: Change the API port:
```bash
export ROACHPROD_API_PORT=9090
```

### Debugging

Enable debug logging for detailed troubleshooting:

```bash
export ROACHPROD_LOG_LEVEL=debug
```

## Documentation

- **[API Reference](docs/API.md)** - Complete REST API documentation
- **[Architecture Guide](docs/ARCHITECTURE.md)** - System design and components
- **[Development Guide](docs/DEVELOPMENT.md)** - Local setup and contribution guidelines
- **[Cloud Provider Configuration](docs/CLOUD_PROVIDER_CONFIG.md)** - Multi-cloud setup
- **[Examples](docs/EXAMPLES.md)** - Common workflows and use cases
- **[Docker Deployment](docker/README.md)** - Container deployment guide

### Related CockroachDB Documentation

- **[Main Documentation](https://cockroachlabs.com/docs/stable/)**
- **[Roachprod Documentation](https://cockroachlabs.com/docs/stable/roachprod.html)**
- **[CockroachDB Contributing Guide](../../CONTRIBUTING.md)**
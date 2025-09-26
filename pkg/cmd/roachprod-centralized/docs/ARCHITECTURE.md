# roachprod-centralized Architecture

This document describes the system architecture, design patterns, and key components of the roachprod-centralized service.

**📚 Related Documentation:**
- [← Back to Main README](../README.md)
- [🔌 API Reference](API.md) - Complete REST API documentation
- [💻 Development Guide](DEVELOPMENT.md) - Local development setup
- [📋 Examples & Workflows](EXAMPLES.md) - Practical usage examples
- [⚙️ Configuration Examples](../examples/) - Ready-to-use configurations

## Table of Contents

- [Overview](#overview)
- [System Architecture](#system-architecture)
- [Core Components](#core-components)
- [Data Flow](#data-flow)
- [Background Processing](#background-processing)
- [Cloud Provider Integration](#cloud-provider-integration)
- [Database Layer](#database-layer)
- [Security](#security)
- [Configuration Management](#configuration-management)
- [Design Patterns](#design-patterns)
- [Scalability Considerations](#scalability-considerations)

## Overview

The roachprod-centralized service is designed as a modern, cloud-native REST API that centralizes management of CockroachDB roachprod clusters across multiple cloud providers. The architecture follows clean architecture principles with clear separation of concerns and dependency inversion.

### Design Goals

- **Scalability**: Support horizontal scaling across multiple instances
- **Reliability**: Graceful handling of failures and recovery
- **Maintainability**: Clean code structure with clear responsibilities
- **Extensibility**: Easy addition of new cloud providers and features
- **Security**: Secure authentication and authorization
- **Observability**: Comprehensive logging, metrics, and monitoring

## System Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                          Load Balancer                          │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    API Gateway / Router                         │
│                   (Gin HTTP Framework)                          │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                     Controllers Layer                           │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Health    │ │  Clusters   │ │    Tasks    │ │ Public DNS  ││
│  │ Controller  │ │ Controller  │ │ Controller  │ │ Controller  ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                     Services Layer                              │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Health    │ │  Clusters   │ │    Tasks    │ │ Public DNS  ││
│  │   Service   │ │   Service   │ │   Service   │ │   Service   ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                  Repositories Layer                             │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Health    │ │  Clusters   │ │    Tasks    │ │   Config    ││
│  │ Repository  │ │ Repository  │ │ Repository  │ │ Repository  ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    Storage Layer                                │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │   Memory    │ │ CockroachDB │ │    Files    │ │ Cloud APIs  ││
│  │   Storage   │ │  Database   │ │   System    │ │Integration  ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘

         ┌─────────────────────────────────────────────┐
         │           Background Systems                │
         │  ┌─────────────┐ ┌─────────────┐ ┌─────────┐│
         │  │Task Workers │ │  Cluster    │ │   DNS   ││
         │  │   Pool      │ │   Sync      │ │  Sync   ││
         │  └─────────────┘ └─────────────┘ └─────────┘│
         └─────────────────────────────────────────────┘
```

## Core Components

### 1. Controllers (`controllers/`)

**Responsibility**: HTTP request/response handling and routing

- **Health Controller**: Basic health checks and status reporting
- **Clusters Controller**: CRUD operations for cluster management
- **Tasks Controller**: Task querying and monitoring
- **Public DNS Controller**: DNS synchronization triggers

**Key Features**:
- Request validation and binding
- Response formatting with `request_id` and `result_type`
- Error handling and HTTP status code mapping
- Authentication middleware integration

### 2. Services (`services/`)

**Responsibility**: Business logic and orchestration

- **Clusters Service**: Core cluster management logic
- **Tasks Service**: Background task coordination
- **Public DNS Service**: DNS record management
- **Health Service**: System health monitoring

**Key Features**:
- Business rule enforcement
- Cross-service coordination
- Background job scheduling
- Cloud provider orchestration

### 3. Repositories (`repositories/`)

**Responsibility**: Data access abstraction

- **Abstract Interfaces**: Define data access contracts
- **Memory Implementation**: In-memory storage for development
- **CockroachDB Implementation**: Production database storage
- **Mock Implementation**: Testing support

**Key Features**:
- Storage backend abstraction
- Transaction management
- Data consistency guarantees
- Migration support

### 4. Background Task System

**Responsibility**: Asynchronous task processing

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Task Queue    │────│ Task Processor  │────│  Task Workers   │
│   (Database)    │    │   Coordinator   │    │     Pool        │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐             │
         └──────────────│   Task Types    │─────────────┘
                        │                 │
                        │ • Cluster Sync  │
                        │ • DNS Sync      │
                        │ • Health Check  │
                        └─────────────────┘
```

## Data Flow

### 1. HTTP Request Flow

```
HTTP Request
    │
    ▼
┌─────────────────┐
│   Middleware    │ ── Authentication, Logging, CORS
│    Pipeline     │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│   Controller    │ ── Request validation, parameter binding
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│    Service      │ ── Business logic, orchestration
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│   Repository    │ ── Data access, persistence
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│   Response      │ ── Formatted JSON with request_id/result_type
│                 │
└─────────────────┘
```

### 2. Background Task Flow

```
API Request (POST /clusters/sync)
    │
    ▼
┌─────────────────┐
│   Controller    │ ── Validate request
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│    Service      │ ── Create task record
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│  Task Queue     │ ── Store task in database
│                 │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Background      │ ── Process task asynchronously
│   Worker        │
└─────────┬───────┘
          │
          ▼
┌─────────────────┐
│ Cloud Provider  │ ── Execute actual operations
│    APIs         │
└─────────────────┘
```

## Background Processing

### Task Processing Architecture

The background task system uses a distributed, fault-tolerant design:

#### Components

1. **Task Repository**: Persistent task storage with atomic operations
2. **Task Processor**: Coordinates task execution across workers
3. **Worker Pool**: Configurable number of concurrent workers
4. **Cleanup System**: Handles stale tasks and failure recovery

#### Task Lifecycle

```
┌─────────┐    ┌─────────┐    ┌─────────┐    ┌─────────┐
│ pending │───▶│ running │───▶│  done   │    │ failed  │
└─────────┘    └─────────┘    └─────────┘    └─────────┘
     │              │              │              │
     │              │              │              │
     │              ▼              │              │
     │         ┌─────────┐         │              │
     │         │ timeout │         │              │
     │         └─────────┘         │              │
     │              │              │              │
     │              ▼              │              │
     └──────────► ┌─────────┐ ◄────┘              │
                  │ pending │                     │
                  └─────────┘ ◄───────────────────┘
```

#### Features

- **Adaptive Polling**: Fast polling when busy (100ms), slow when idle (5s)
- **Distributed Cleanup**: Each worker cleans stale tasks before polling
- **Concurrency Safety**: Uses CockroachDB's strong consistency
- **Failure Recovery**: Automatic retry of failed tasks

### Task Types

| Task Type | Description | Trigger |
|-----------|-------------|---------|
| `cluster_sync` | Synchronize cluster data from cloud providers | POST /clusters/sync |
| `dns_sync` | Update DNS records for clusters | POST /public-dns/sync |
| `health_check` | Periodic system health validation | Scheduled |

## Cloud Provider Integration

### Provider Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                    Cloud Provider Abstraction                   │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                Provider Implementations                          │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │     GCE     │ │     AWS     │ │    Azure    │ │     IBM     ││
│  │  Provider   │ │  Provider   │ │  Provider   │ │  Provider   ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                   Cloud APIs                                    │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐│
│  │GCP Compute  │ │   AWS EC2   │ │ Azure VMs   │ │  IBM Cloud  ││
│  │   Engine    │ │             │ │             │ │   VMs       ││
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘│
└─────────────────────────────────────────────────────────────────┘
```

### Provider Configuration

Each provider supports:
- **Credentials Management**: Secure credential storage and rotation
- **Multi-Region Support**: Operations across different regions
- **DNS Integration**: Public DNS record management
- **Resource Tagging**: Consistent tagging for resource organization

## Database Layer

### Schema Design

```sql
-- Tasks table (primary workload)
CREATE TABLE tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type STRING NOT NULL,
    status STRING NOT NULL DEFAULT 'pending',
    payload JSONB,
    result JSONB,
    consumer_id STRING,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    INDEX idx_tasks_status_type (status, type),
    INDEX idx_tasks_consumer (consumer_id),
    INDEX idx_tasks_created (created_at)
);

-- Clusters table (cached cloud data)
CREATE TABLE clusters (
    name STRING PRIMARY KEY,
    provider STRING NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    INDEX idx_clusters_provider (provider)
);
```

### Storage Backends

#### Memory Backend
- **Use Case**: Development and testing
- **Features**: Fast, ephemeral, no persistence
- **Limitations**: Single instance, data loss on restart

#### CockroachDB Backend
- **Use Case**: Production deployments
- **Features**: Distributed, consistent, scalable
- **Benefits**:
  - Strong consistency for task coordination
  - Horizontal scalability
  - Built-in replication and fault tolerance
  - ACID transactions

## Security

### Authentication

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client        │────│  Load Balancer  │────│   API Server    │
│                 │    │   (IAP/JWT)     │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │              ┌─────────────────┐             │
         │              │ JWT Validation  │             │
         │              │   Middleware    │             │
         │              └─────────────────┘             │
         │                       │                       │
         ▼              ┌─────────▼─────────┐             ▼
┌─────────────────┐     │  Authentication  │    ┌─────────────────┐
│ Request Headers │────▶│    Service       │───▶│   Controller    │
│                 │     │                  │    │                 │
└─────────────────┘     └──────────────────┘    └─────────────────┘
```

### Security Features

- **JWT Authentication**: Google IAP integration for production
- **Input Validation**: Comprehensive request validation
- **SQL Injection Prevention**: Parameterized queries only
- **Credential Management**: Secure cloud provider credential handling
- **Audit Logging**: Request tracking with unique IDs

## Configuration Management

### Configuration Hierarchy

```
┌─────────────────────────────────────────────────────────────────┐
│                     Configuration Sources                       │
│                    (Highest to Lowest Priority)                 │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│               Environment Variables                              │
│                (ROACHPROD_* prefix)                             │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                Command Line Flags                               │
│              (--api-port, --log-level, etc.)                   │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                 YAML Configuration File                         │
│                (config.yaml, --config flag)                    │
└─────────────────────┬───────────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────────┐
│                    Default Values                               │
│                  (Built into code)                              │
└─────────────────────────────────────────────────────────────────┘
```

### Configuration Features

- **Hot Reloading**: Not currently supported (restart required)
- **Validation**: Comprehensive configuration validation on startup
- **Environment Support**: Development, staging, production profiles
- **Secrets Management**: Secure handling of sensitive configuration

## Design Patterns

### 1. Dependency Injection

```go
// Service interfaces define contracts
type IClusterService interface {
    GetAllClusters(ctx context.Context, ...) ([]Cluster, error)
}

// Implementation injected at runtime
type ClusterService struct {
    repo clusters.IRepository
    logger *logger.Logger
}

// Factory pattern for service creation
func NewServicesFromConfig(cfg *config.Config) (*Services, error) {
    // Create repositories based on configuration
    // Inject dependencies into services
    // Return configured service collection
}
```

### 2. Repository Pattern

```go
// Abstract interface for data access
type IRepository interface {
    Create(ctx context.Context, cluster *Cluster) error
    GetByName(ctx context.Context, name string) (*Cluster, error)
    Update(ctx context.Context, cluster *Cluster) error
    Delete(ctx context.Context, name string) error
}

// Multiple implementations
type MemoryRepository struct { /* ... */ }
type CockroachDBRepository struct { /* ... */ }
```

### 3. Factory Pattern

- **Service Factory**: Creates service instances based on configuration
- **Repository Factory**: Selects storage backend based on settings
- **Provider Factory**: Instantiates cloud providers dynamically

### 4. Observer Pattern

- **Task Events**: Background workers observe task state changes
- **Health Events**: Health service monitors component status
- **Metrics Events**: Prometheus metrics collection

## Scalability Considerations

### Horizontal Scaling

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Instance 1    │    │   Instance 2    │    │   Instance 3    │
│                 │    │                 │    │                 │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │ API Server  │ │    │ │ API Server  │ │    │ │ API Server  │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
│ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │
│ │Task Workers │ │    │ │Task Workers │ │    │ │Task Workers │ │
│ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────▼─────────────┐
                    │     Shared Database       │
                    │     (CockroachDB)         │
                    └───────────────────────────┘
```

### Performance Characteristics

- **API Throughput**: ~1000 requests/second per instance
- **Task Processing**: Configurable worker count (default: 1-5)
- **Database Connections**: Pooled connections with configurable limits
- **Memory Usage**: ~100MB baseline + working set
- **Startup Time**: < 5 seconds

### Bottlenecks and Mitigation

1. **Database Connections**: Connection pooling and limits
2. **Cloud API Rate Limits**: Exponential backoff and retry logic
3. **Task Queue Contention**: Optimistic concurrency control
4. **Memory Usage**: Streaming responses for large datasets

### Monitoring and Observability

- **Metrics**: Prometheus integration with custom metrics
- **Logging**: Structured logging with correlation IDs
- **Tracing**: Request tracing for debugging
- **Health Checks**: Multi-level health status reporting
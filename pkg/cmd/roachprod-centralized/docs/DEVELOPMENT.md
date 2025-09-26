# roachprod-centralized Development Guide

This guide covers local development setup, testing practices, and contribution guidelines for the roachprod-centralized service.

**📚 Related Documentation:**
- [← Back to Main README](../README.md)
- [🔌 API Reference](API.md) - Complete REST API documentation
- [🏗️ Architecture Guide](ARCHITECTURE.md) - System design and components
- [📋 Examples & Workflows](EXAMPLES.md) - Practical usage examples
- [⚙️ Configuration Examples](../examples/) - Ready-to-use configurations

## Table of Contents

- [Prerequisites](#prerequisites)
- [Development Environment Setup](#development-environment-setup)
- [Project Structure](#project-structure)
- [Building and Running](#building-and-running)
- [Testing](#testing)
- [Code Style and Standards](#code-style-and-standards)
- [Debugging](#debugging)
- [Contributing](#contributing)
- [Common Development Tasks](#common-development-tasks)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Tools

Ensure you have the CockroachDB development environment set up:

```bash
# Verify your development environment
./dev doctor
```

This should validate:
- **Go**: Version 1.21+ (managed by CockroachDB build system)
- **Bazel**: CockroachDB uses Bazel for builds
- **Git**: For version control
- **Make**: For various development tasks

### Optional Tools

For enhanced development experience:

```bash
# Install useful development tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install github.com/swaggo/swag/cmd/swag@latest  # For API docs generation
```

### Cloud Provider Access (Optional)

For testing cloud provider integration:
- **GCP**: Service account key or `gcloud` CLI
- **AWS**: AWS credentials configured
- **Azure**: Azure CLI configured
- **IBM**: IBM Cloud CLI configured

## Development Environment Setup

### 1. Repository Setup

From the CockroachDB repository root:

```bash
# Navigate to the roachprod-centralized directory
cd pkg/cmd/roachprod-centralized

# Check the current structure
ls -la
```

### 2. IDE Configuration

#### VS Code

Create `.vscode/settings.json` in the roachprod-centralized directory:

```json
{
  "go.toolsManagement.checkForUpdates": "local",
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": [
    "--fast"
  ],
  "go.formatTool": "goimports",
  "files.exclude": {
    "**/.git": true,
    "**/bazel-*": true
  }
}
```

#### GoLand/IntelliJ

- Set Go SDK to the version used by CockroachDB build system
- Configure Bazel plugin
- Set project root to the CockroachDB repository root

### 3. Local Configuration

Create a development configuration file:

```bash
# Create local config directory
mkdir -p ~/.roachprod

# Create development configuration
cat > ~/.roachprod/dev-config.yaml << EOF
log:
  level: debug
api:
  port: 8080
  authentication:
    disabled: true
database:
  type: memory
tasks:
  workers: 1
EOF
```

## Project Structure

```
pkg/cmd/roachprod-centralized/
├── README.md                    # Main documentation
├── main.go                      # Application entry point
├── config.yml                   # Default configuration
├── BUILD.bazel                  # Bazel build configuration
│
├── app/                         # Application initialization
│   ├── app.go                   # Main app structure
│   ├── api.go                   # API server setup
│   ├── factory.go               # Service factory
│   └── options.go               # App configuration options
│
├── cmd/                         # CLI commands (Cobra)
│   ├── root.go                  # Root command
│   └── api.go                   # API server command
│
├── config/                      # Configuration management
│   └── config.go                # Config struct and loading
│
├── controllers/                 # HTTP request handlers
│   ├── clusters/                # Cluster endpoints
│   ├── health/                  # Health check endpoints
│   ├── tasks/                   # Task endpoints
│   └── public-dns/              # DNS endpoints
│
├── services/                    # Business logic layer
│   ├── clusters/                # Cluster management
│   ├── tasks/                   # Task processing
│   ├── health/                  # Health monitoring
│   └── public-dns/              # DNS management
│
├── repositories/                # Data access layer
│   ├── clusters/                # Cluster storage
│   ├── tasks/                   # Task storage
│   └── health/                  # Health storage
│
├── models/                      # Data structures
│   └── tasks/                   # Task models
│
├── utils/                       # Shared utilities
│   ├── api/                     # API utilities
│   ├── database/                # Database utilities
│   ├── filters/                 # Query filtering
│   └── logger/                  # Logging utilities
│
├── docker/                      # Docker configuration
│   ├── Dockerfile
│   ├── build.sh
│   └── README.md
│
├── docs/                        # Documentation
│   ├── API.md
│   ├── ARCHITECTURE.md
│   ├── DEVELOPMENT.md           # This file
│   └── EXAMPLES.md
│
└── examples/                    # Example configurations
    └── cloud_config.yaml.example
```

## Building and Running

### Build Commands

```bash
# Build the binary (from CockroachDB root)
./dev build roachprod-centralized

# Build with race detection (for development)
./dev build roachprod-centralized --race

# Build without UI (faster)
./dev build short
```

### Running Locally

```bash
# Run with development configuration
./dev run roachprod-centralized api --config ~/.roachprod/dev-config.yaml

# Run with in-memory storage
export ROACHPROD_DATABASE_TYPE=memory
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
./dev run roachprod-centralized api

# Run with debug logging
export ROACHPROD_LOG_LEVEL=debug
./dev run roachprod-centralized api
```

### Development with Live Reload

For rapid development iteration:

```bash
# Option 1: Use air for live reloading (install first)
go install github.com/cosmtrek/air@latest
air

# Option 2: Simple rebuild script
cat > dev-reload.sh << 'EOF'
#!/bin/bash
while true; do
  ./dev build roachprod-centralized
  ./dev run roachprod-centralized api &
  PID=$!
  inotifywait -r -e modify pkg/cmd/roachprod-centralized/
  kill $PID
done
EOF
chmod +x dev-reload.sh
./dev-reload.sh
```

## Testing

### Unit Tests

```bash
# Run all tests
./dev test pkg/cmd/roachprod-centralized/...

# Run tests for specific package
./dev test pkg/cmd/roachprod-centralized/services/clusters

# Run tests with race detection
./dev test pkg/cmd/roachprod-centralized/... --race

# Run tests with coverage
./dev test pkg/cmd/roachprod-centralized/... --coverage

# Run specific test
./dev test pkg/cmd/roachprod-centralized/services/clusters -f TestClustersService

# Verbose test output
./dev test pkg/cmd/roachprod-centralized/services/clusters -v
```

### Integration Tests

```bash
# Run integration tests (if available)
./dev test pkg/cmd/roachprod-centralized/... --tags=integration

# Test with real database
export ROACHPROD_DATABASE_TYPE=cockroachdb
export ROACHPROD_DATABASE_URL="postgresql://root@localhost:26257/roachprod_test?sslmode=disable"
./dev test pkg/cmd/roachprod-centralized/repositories/...
```

### Testing Best Practices

1. **Use Mocks**: Mock external dependencies (cloud APIs, databases)
2. **Table Tests**: Use table-driven tests for multiple scenarios
3. **Test Isolation**: Each test should be independent
4. **Error Cases**: Test both success and failure paths

#### Example Test Structure

```go
func TestClustersService_GetAllClusters(t *testing.T) {
    tests := []struct {
        name      string
        filters   filters.FilterSet
        mockData  []cloud.Cluster
        expected  []cloud.Cluster
        wantError bool
    }{
        {
            name:     "success - no filters",
            filters:  filters.FilterSet{},
            mockData: []cloud.Cluster{testCluster1, testCluster2},
            expected: []cloud.Cluster{testCluster1, testCluster2},
        },
        // More test cases...
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

## Code Style and Standards

### Go Code Standards

Follow standard Go conventions and CockroachDB coding standards:

```bash
# Format code
./dev generate go

# Run linting
./dev lint

# Run specific linters
./dev lint --short
```

### Code Organization Principles

1. **Package Naming**: Use clear, descriptive package names
2. **Interface Segregation**: Small, focused interfaces
3. **Dependency Injection**: Inject dependencies via constructors
4. **Error Handling**: Comprehensive error handling with context

### Documentation Standards

1. **Godoc Comments**: All public functions and types
2. **Package Documentation**: Clear package purpose
3. **Example Code**: Include examples for complex functions

```go
// ClusterService handles cluster management operations.
// It provides CRUD operations and synchronization with cloud providers.
type ClusterService struct {
    repo   clusters.IRepository
    logger *logger.Logger
}

// GetAllClusters retrieves all clusters matching the provided filters.
// Returns an empty slice if no clusters match the criteria.
func (s *ClusterService) GetAllClusters(ctx context.Context, logger *logger.Logger, input InputGetAllClustersDTO) ([]cloud.Cluster, error) {
    // Implementation...
}
```

## Debugging

### Local Debugging

#### Using Delve Debugger

```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug the application
dlv debug github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized -- api

# Or attach to running process
dlv attach $(pgrep roachprod-centralized)
```

#### VS Code Debugging

Add to `.vscode/launch.json`:

```json
{
    "version": "0.2.0",
    "configurations": [
        {
            "name": "Debug API",
            "type": "go",
            "request": "launch",
            "mode": "debug",
            "program": "${workspaceFolder}/main.go",
            "args": ["api", "--config", "~/.roachprod/dev-config.yaml"],
            "env": {
                "ROACHPROD_LOG_LEVEL": "debug",
                "ROACHPROD_DATABASE_TYPE": "memory"
            }
        }
    ]
}
```

### Logging and Monitoring

#### Debug Logging

```bash
# Enable debug logging
export ROACHPROD_LOG_LEVEL=debug

# Log specific operations
curl -X POST http://localhost:8080/clusters/sync
# Check logs for detailed operation traces
```

#### Health Monitoring

```bash
# Check API health
curl http://localhost:8080/health

# Monitor metrics
curl http://localhost:8081/metrics | grep roachprod
```

## Contributing

### Development Workflow

1. **Create Feature Branch**:
   ```bash
   git checkout -b feature/new-functionality
   ```

2. **Make Changes**:
   - Write code following style guidelines
   - Add comprehensive tests
   - Update documentation

3. **Test Changes**:
   ```bash
   ./dev test pkg/cmd/roachprod-centralized/...
   ./dev lint
   ```

4. **Commit Changes**:
   ```bash
   git add .
   git commit -m "roachprod-centralized: add new functionality

   This commit adds X functionality to support Y use case.
   - Implement Z feature
   - Add tests for Z
   - Update documentation

   Release notes: None
   Epic: CRDB-12345"
   ```

### Code Review Process

1. **Pre-Review Checklist**:
   - [ ] All tests pass
   - [ ] Code follows style guidelines
   - [ ] Documentation updated
   - [ ] No security vulnerabilities

2. **Review Criteria**:
   - Code correctness and clarity
   - Test coverage and quality
   - Performance implications
   - Security considerations

### Release Process

1. **Version Tagging**: Follow CockroachDB versioning
2. **Release Notes**: Document user-facing changes
3. **Documentation Updates**: Keep docs current
4. **Deployment**: Follow CockroachDB deployment process

## Common Development Tasks

### Adding a New Endpoint

1. **Create Controller Handler**:
   ```go
   // In controllers/clusters/clusters.go
   func (ctrl *Controller) NewOperation(c *gin.Context) {
       // Implementation
   }
   ```

2. **Add Route**:
   ```go
   // In NewController()
   &controllers.ControllerHandler{
       Method: "POST",
       Path:   ControllerPath + "/new-operation",
       Func:   ctrl.NewOperation,
   }
   ```

3. **Add Service Method**:
   ```go
   // In services/clusters/clusters.go
   func (s *Service) NewOperation(ctx context.Context, ...) error {
       // Business logic
   }
   ```

4. **Add Tests**:
   ```go
   func TestController_NewOperation(t *testing.T) {
       // Test implementation
   }
   ```

### Adding a New Cloud Provider

1. **Implement Provider Interface**:
   ```go
   // In services/clusters/providers/
   type NewProvider struct {
       // Provider-specific fields
   }

   func (p *NewProvider) GetClusters(ctx context.Context) ([]cloud.Cluster, error) {
       // Implementation
   }
   ```

2. **Register Provider**:
   ```go
   // In service factory
   switch providerType {
   case "new-provider":
       return &NewProvider{}, nil
   }
   ```

3. **Add Configuration**:
   ```go
   // In config/config.go
   type CloudProvider struct {
       NewProvider NewProviderOptions `env:"NEWPROVIDER"`
   }
   ```

### Database Schema Changes

1. **Create Migration**:
   ```go
   // In repositories/*/cockroachdb/migrations_definition.go
   func Migration_001_AddNewTable() string {
       return `CREATE TABLE IF NOT EXISTS new_table (...);`
   }
   ```

2. **Update Repository**:
   ```go
   // Add new methods to repository interface and implementation
   ```

3. **Test Migration**:
   ```bash
   # Test with CockroachDB
   ./dev test pkg/cmd/roachprod-centralized/repositories/*/cockroachdb/...
   ```

## Troubleshooting

### Common Issues

#### Build Failures

```bash
# Error: module not found
# Solution: Ensure you're in the CockroachDB repository root
cd /path/to/cockroach
./dev build roachprod-centralized

# Error: Bazel build failed
# Solution: Clean and rebuild
bazel clean
./dev build roachprod-centralized
```

#### Runtime Issues

```bash
# Error: Port already in use
# Solution: Use different port or kill existing process
export ROACHPROD_API_PORT=9090
# Or find and kill the process
lsof -ti:8080 | xargs kill

# Error: Database connection failed
# Solution: Use memory database for development
export ROACHPROD_DATABASE_TYPE=memory
```

#### Test Failures

```bash
# Error: Tests fail with timeout
# Solution: Increase test timeout
./dev test pkg/cmd/roachprod-centralized/... --timeout=60s

# Error: Race conditions detected
# Solution: Fix race conditions or use build constraints
./dev test pkg/cmd/roachprod-centralized/... --race
```

### Performance Issues

#### Memory Usage

```bash
# Monitor memory usage
top -p $(pgrep roachprod-centralized)

# Profile memory usage
go tool pprof http://localhost:8080/debug/pprof/heap
```

#### CPU Usage

```bash
# Profile CPU usage
go tool pprof http://localhost:8080/debug/pprof/profile
```

### Getting Help

1. **CockroachDB Documentation**: https://cockroachlabs.com/docs/
2. **Internal Documentation**: Check `/docs/` in the CockroachDB repository
3. **Team Channels**: Reach out to the roachprod team
4. **Code Review**: Ask for help during code review process

### Development Tips

1. **Use Memory Database**: Faster iteration during development
2. **Enable Debug Logging**: Better insight into operations
3. **Mock External Services**: Avoid rate limits and dependencies
4. **Test Edge Cases**: Comprehensive error handling
5. **Profile Performance**: Regular performance monitoring
6. **Keep Dependencies Updated**: Follow CockroachDB update cycles
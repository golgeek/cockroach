# roachprod-centralized Examples

This document provides practical examples and common workflows for using the roachprod-centralized service.

**📚 Related Documentation:**
- [← Back to Main README](../README.md)
- [🔌 API Reference](API.md) - Complete REST API documentation
- [🏗️ Architecture Guide](ARCHITECTURE.md) - System design and components
- [💻 Development Guide](DEVELOPMENT.md) - Local development setup
- [⚙️ Configuration Examples](../examples/) - Ready-to-use configurations

## Table of Contents

- [Quick Start Examples](#quick-start-examples)
- [Cluster Management Workflows](#cluster-management-workflows)
- [Task Monitoring and Management](#task-monitoring-and-management)
- [DNS Management](#dns-management)
- [Configuration Examples](#configuration-examples)
- [Integration Examples](#integration-examples)
- [Troubleshooting Scenarios](#troubleshooting-scenarios)
- [Automation Scripts](#automation-scripts)

## Quick Start Examples

### Basic Setup and Health Check

```bash
# 1. Start the service in development mode
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
export ROACHPROD_DATABASE_TYPE=memory
export ROACHPROD_LOG_LEVEL=info

./dev run roachprod-centralized api

# 2. Verify the service is running
curl http://localhost:8080/health

# Expected response:
{
  "request_id": "req_1a2b3c4d5e6f7g8h",
  "result_type": "health.HealthDTO",
  "data": {
    "status": "ok",
    "timestamp": "2025-01-15T10:30:00Z"
  }
}
```

### First API Call

```bash
# List all clusters (should be empty initially)
curl http://localhost:8080/clusters

# Expected response:
{
  "request_id": "req_2b3c4d5e6f7g8h9i",
  "result_type": "cloud.Clusters",
  "data": []
}
```

## Cluster Management Workflows

### Complete Cluster Lifecycle

```bash
#!/bin/bash
# cluster-lifecycle-example.sh

API_BASE="http://localhost:8080"

# 1. Create a new test cluster
echo "Creating test cluster..."
CLUSTER_RESPONSE=$(curl -s -X POST $API_BASE/clusters \
  -H "Content-Type: application/json" \
  -d '{
    "name": "example-test-cluster",
    "provider": "gce",
    "region": "us-central1",
    "nodes": ["example-test-cluster-1", "example-test-cluster-2", "example-test-cluster-3"],
    "vm_type": "n1-standard-4",
    "disk_size": 100
  }')

echo "Cluster creation response:"
echo $CLUSTER_RESPONSE | jq .

# 2. Verify cluster was created
echo -e "\nFetching cluster details..."
curl -s $API_BASE/clusters/example-test-cluster | jq .

# 3. List all clusters to see our new cluster
echo -e "\nListing all clusters:"
curl -s $API_BASE/clusters | jq .

# 4. Update cluster (add a node)
echo -e "\nUpdating cluster to add a node..."
curl -s -X PUT $API_BASE/clusters/example-test-cluster \
  -H "Content-Type: application/json" \
  -d '{
    "name": "example-test-cluster",
    "provider": "gce",
    "region": "us-central1",
    "nodes": ["example-test-cluster-1", "example-test-cluster-2", "example-test-cluster-3", "example-test-cluster-4"]
  }' | jq .

# 5. Trigger cluster sync
echo -e "\nTriggering cluster sync..."
SYNC_RESPONSE=$(curl -s -X POST $API_BASE/clusters/sync)
TASK_ID=$(echo $SYNC_RESPONSE | jq -r '.data.id')
echo "Sync task ID: $TASK_ID"

# 6. Monitor sync task
echo -e "\nMonitoring sync task..."
for i in {1..10}; do
  echo "Check $i:"
  TASK_STATUS=$(curl -s $API_BASE/tasks/$TASK_ID | jq -r '.data.status')
  echo "Task status: $TASK_STATUS"

  if [[ "$TASK_STATUS" == "done" || "$TASK_STATUS" == "failed" ]]; then
    break
  fi

  sleep 2
done

# 7. Get final task details
echo -e "\nFinal task details:"
curl -s $API_BASE/tasks/$TASK_ID | jq .

# 8. Clean up - delete the cluster
echo -e "\nDeleting test cluster..."
curl -s -X DELETE $API_BASE/clusters/example-test-cluster

echo -e "\nVerifying cluster deletion:"
curl -s $API_BASE/clusters/example-test-cluster || echo "Cluster successfully deleted"
```

### Bulk Cluster Operations

```bash
#!/bin/bash
# bulk-cluster-operations.sh

API_BASE="http://localhost:8080"

# Create multiple clusters for testing
CLUSTER_NAMES=("test-cluster-1" "test-cluster-2" "test-cluster-3")

echo "Creating multiple test clusters..."
for cluster_name in "${CLUSTER_NAMES[@]}"; do
  echo "Creating $cluster_name..."
  curl -s -X POST $API_BASE/clusters \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"$cluster_name\",
      \"provider\": \"gce\",
      \"region\": \"us-central1\",
      \"nodes\": [\"${cluster_name}-1\", \"${cluster_name}-2\"],
      \"vm_type\": \"n1-standard-2\"
    }" | jq '.data.name'
done

echo -e "\nListing all clusters:"
curl -s $API_BASE/clusters | jq '.data[].name'

echo -e "\nFiltering clusters by name pattern:"
curl -s "$API_BASE/clusters?name=test-cluster" | jq '.data[].name'

echo -e "\nCleaning up all test clusters..."
for cluster_name in "${CLUSTER_NAMES[@]}"; do
  echo "Deleting $cluster_name..."
  curl -s -X DELETE $API_BASE/clusters/$cluster_name
done

echo "Bulk operations completed."
```

## Task Monitoring and Management

### Task Monitoring Workflow

```bash
#!/bin/bash
# task-monitoring-example.sh

API_BASE="http://localhost:8080"

echo "Starting multiple background tasks..."

# Start cluster sync
SYNC_RESPONSE=$(curl -s -X POST $API_BASE/clusters/sync)
CLUSTER_TASK_ID=$(echo $SYNC_RESPONSE | jq -r '.data.id')
echo "Cluster sync task ID: $CLUSTER_TASK_ID"

# Start DNS sync
DNS_RESPONSE=$(curl -s -X POST $API_BASE/public-dns/sync)
DNS_TASK_ID=$(echo $DNS_RESPONSE | jq -r '.data.id')
echo "DNS sync task ID: $DNS_TASK_ID"

# Monitor all tasks
echo -e "\nMonitoring all tasks..."
while true; do
  echo "--- Current task status ---"

  # Get all tasks
  ALL_TASKS=$(curl -s $API_BASE/tasks | jq '.data[]')

  # Show running tasks
  RUNNING_TASKS=$(echo $ALL_TASKS | jq -s 'map(select(.status == "running"))')
  RUNNING_COUNT=$(echo $RUNNING_TASKS | jq 'length')

  echo "Running tasks: $RUNNING_COUNT"
  if [[ $RUNNING_COUNT -gt 0 ]]; then
    echo $RUNNING_TASKS | jq '.[] | {id: .id, type: .type, status: .status}'
  fi

  # Check if our specific tasks are complete
  CLUSTER_STATUS=$(curl -s $API_BASE/tasks/$CLUSTER_TASK_ID | jq -r '.data.status')
  DNS_STATUS=$(curl -s $API_BASE/tasks/$DNS_TASK_ID | jq -r '.data.status')

  echo "Cluster sync: $CLUSTER_STATUS"
  echo "DNS sync: $DNS_STATUS"

  if [[ "$CLUSTER_STATUS" != "running" && "$CLUSTER_STATUS" != "pending" ]] && \
     [[ "$DNS_STATUS" != "running" && "$DNS_STATUS" != "pending" ]]; then
    break
  fi

  sleep 3
done

echo -e "\nFinal task results:"
echo "Cluster sync task:"
curl -s $API_BASE/tasks/$CLUSTER_TASK_ID | jq '.data'

echo -e "\nDNS sync task:"
curl -s $API_BASE/tasks/$DNS_TASK_ID | jq '.data'
```

### Task Filtering and Analysis

```bash
#!/bin/bash
# task-analysis-example.sh

API_BASE="http://localhost:8080"

echo "Task Analysis Examples"
echo "======================"

# Get all tasks
echo -e "\n1. All tasks:"
curl -s $API_BASE/tasks | jq '.data[] | {id: .id, type: .type, status: .status, created_at: .created_at}'

# Filter by status
echo -e "\n2. Failed tasks only:"
curl -s "$API_BASE/tasks?status=failed" | jq '.data[]'

# Filter by type
echo -e "\n3. Cluster sync tasks only:"
curl -s "$API_BASE/tasks?type=cluster_sync" | jq '.data[]'

# Combine filters (if supported)
echo -e "\n4. Running cluster sync tasks:"
curl -s "$API_BASE/tasks?status=running&type=cluster_sync" | jq '.data[]'

# Task statistics
echo -e "\n5. Task statistics:"
ALL_TASKS=$(curl -s $API_BASE/tasks | jq '.data[]')

TOTAL_COUNT=$(echo $ALL_TASKS | jq -s 'length')
PENDING_COUNT=$(echo $ALL_TASKS | jq -s 'map(select(.status == "pending")) | length')
RUNNING_COUNT=$(echo $ALL_TASKS | jq -s 'map(select(.status == "running")) | length')
DONE_COUNT=$(echo $ALL_TASKS | jq -s 'map(select(.status == "done")) | length')
FAILED_COUNT=$(echo $ALL_TASKS | jq -s 'map(select(.status == "failed")) | length')

echo "Total tasks: $TOTAL_COUNT"
echo "Pending: $PENDING_COUNT"
echo "Running: $RUNNING_COUNT"
echo "Done: $DONE_COUNT"
echo "Failed: $FAILED_COUNT"
```

## DNS Management

### DNS Synchronization Example

```bash
#!/bin/bash
# dns-sync-example.sh

API_BASE="http://localhost:8080"

echo "DNS Management Example"
echo "====================="

# Trigger DNS sync
echo "1. Triggering DNS synchronization..."
DNS_RESPONSE=$(curl -s -X POST $API_BASE/public-dns/sync)
echo $DNS_RESPONSE | jq .

TASK_ID=$(echo $DNS_RESPONSE | jq -r '.data.id')
echo "DNS sync task ID: $TASK_ID"

# Monitor DNS sync progress
echo -e "\n2. Monitoring DNS sync progress..."
while true; do
  TASK_INFO=$(curl -s $API_BASE/tasks/$TASK_ID)
  STATUS=$(echo $TASK_INFO | jq -r '.data.status')

  echo "DNS sync status: $STATUS"

  if [[ "$STATUS" == "done" || "$STATUS" == "failed" ]]; then
    echo -e "\nFinal DNS sync result:"
    echo $TASK_INFO | jq '.data'
    break
  fi

  sleep 2
done

# List all DNS-related tasks
echo -e "\n3. All DNS sync tasks:"
curl -s "$API_BASE/tasks?type=dns_sync" | jq '.data[] | {id: .id, status: .status, created_at: .created_at}'
```

## Configuration Examples

### Development Configuration

```yaml
# ~/.roachprod/dev-config.yaml
log:
  level: debug

api:
  port: 8080
  base_path: "/api"
  metrics:
    enabled: true
    port: 8081
  authentication:
    disabled: true

database:
  type: memory

tasks:
  workers: 2
```

### Production Configuration

```yaml
# /etc/roachprod/prod-config.yaml
log:
  level: info

api:
  port: 8080
  metrics:
    enabled: true
    port: 8081
  authentication:
    disabled: false
    jwt:
      header: "X-Goog-IAP-JWT-Assertion"
      audience: "your-production-audience"

database:
  type: cockroachdb
  url: "postgresql://roachprod:password@crdb-cluster:26257/roachprod?sslmode=require"
  max_conns: 20
  max_idle_time: 300

tasks:
  workers: 5

cloudproviders:
  - type: gce
    gce:
      project: "production-project"
      dns_project: "dns-project"
  - type: aws
    aws:
      account_id: "123456789012"
      assume_sts_role: true
```

### Docker Compose Configuration

```yaml
# docker-compose.yml
version: '3.8'

services:
  roachprod-centralized:
    build: .
    ports:
      - "8080:8080"
      - "8081:8081"
    environment:
      - ROACHPROD_LOG_LEVEL=info
      - ROACHPROD_API_AUTHENTICATION_DISABLED=true
      - ROACHPROD_DATABASE_TYPE=cockroachdb
      - ROACHPROD_DATABASE_URL=postgresql://root@cockroachdb:26257/roachprod?sslmode=disable
      - ROACHPROD_TASKS_WORKERS=3
    depends_on:
      - cockroachdb
    volumes:
      - ./config.yaml:/etc/roachprod/config.yaml:ro
      - ./secrets:/secrets:ro

  cockroachdb:
    image: cockroachdb/cockroach:latest
    command: start-single-node --insecure --http-addr=0.0.0.0:8080
    ports:
      - "26257:26257"
      - "8090:8080"
    volumes:
      - cockroach-data:/cockroach/cockroach-data

volumes:
  cockroach-data:
```

## Integration Examples

### Python Client Integration

```python
#!/usr/bin/env python3
# roachprod_client.py

import requests
import json
import time
from typing import Dict, List, Optional

class RoachprodClient:
    def __init__(self, base_url: str = "http://localhost:8080"):
        self.base_url = base_url.rstrip('/')
        self.session = requests.Session()

    def health_check(self) -> Dict:
        """Check API health"""
        response = self.session.get(f"{self.base_url}/health")
        response.raise_for_status()
        return response.json()

    def list_clusters(self, name_filter: Optional[str] = None) -> List[Dict]:
        """List all clusters with optional name filter"""
        url = f"{self.base_url}/clusters"
        params = {}
        if name_filter:
            params['name'] = name_filter

        response = self.session.get(url, params=params)
        response.raise_for_status()
        return response.json()['data']

    def get_cluster(self, name: str) -> Dict:
        """Get specific cluster by name"""
        response = self.session.get(f"{self.base_url}/clusters/{name}")
        response.raise_for_status()
        return response.json()['data']

    def create_cluster(self, cluster_data: Dict) -> Dict:
        """Create a new cluster"""
        response = self.session.post(
            f"{self.base_url}/clusters",
            json=cluster_data
        )
        response.raise_for_status()
        return response.json()['data']

    def delete_cluster(self, name: str) -> None:
        """Delete a cluster"""
        response = self.session.delete(f"{self.base_url}/clusters/{name}")
        response.raise_for_status()

    def sync_clusters(self) -> str:
        """Trigger cluster sync, returns task ID"""
        response = self.session.post(f"{self.base_url}/clusters/sync")
        response.raise_for_status()
        return response.json()['data']['id']

    def get_task(self, task_id: str) -> Dict:
        """Get task status and details"""
        response = self.session.get(f"{self.base_url}/tasks/{task_id}")
        response.raise_for_status()
        return response.json()['data']

    def wait_for_task(self, task_id: str, timeout: int = 300) -> Dict:
        """Wait for task to complete"""
        start_time = time.time()

        while time.time() - start_time < timeout:
            task = self.get_task(task_id)

            if task['status'] in ['done', 'failed']:
                return task

            time.sleep(2)

        raise TimeoutError(f"Task {task_id} did not complete within {timeout} seconds")

# Example usage
if __name__ == "__main__":
    client = RoachprodClient()

    # Health check
    health = client.health_check()
    print(f"API Status: {health['data']['status']}")

    # Create a test cluster
    cluster_data = {
        "name": "python-test-cluster",
        "provider": "gce",
        "region": "us-central1",
        "nodes": ["python-test-cluster-1", "python-test-cluster-2"]
    }

    print("Creating cluster...")
    cluster = client.create_cluster(cluster_data)
    print(f"Created cluster: {cluster['name']}")

    # Sync clusters
    print("Triggering sync...")
    task_id = client.sync_clusters()

    # Wait for sync to complete
    print(f"Waiting for task {task_id} to complete...")
    task_result = client.wait_for_task(task_id)
    print(f"Sync completed with status: {task_result['status']}")

    # List clusters
    clusters = client.list_clusters()
    print(f"Total clusters: {len(clusters)}")

    # Clean up
    print("Cleaning up...")
    client.delete_cluster("python-test-cluster")
    print("Cluster deleted")
```

### Go Client Integration

```go
// roachprod_client.go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type RoachprodClient struct {
    BaseURL    string
    HTTPClient *http.Client
}

type APIResponse struct {
    RequestID  string      `json:"request_id"`
    ResultType string      `json:"result_type"`
    Data       interface{} `json:"data,omitempty"`
    Error      *APIError   `json:"error,omitempty"`
}

type APIError struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

type Cluster struct {
    Name      string    `json:"name"`
    Provider  string    `json:"provider"`
    Region    string    `json:"region"`
    Status    string    `json:"status"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type Task struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Status    string                 `json:"status"`
    Payload   map[string]interface{} `json:"payload"`
    Result    map[string]interface{} `json:"result"`
    CreatedAt time.Time              `json:"created_at"`
    UpdatedAt time.Time              `json:"updated_at"`
}

func NewRoachprodClient(baseURL string) *RoachprodClient {
    return &RoachprodClient{
        BaseURL:    baseURL,
        HTTPClient: &http.Client{Timeout: 30 * time.Second},
    }
}

func (c *RoachprodClient) makeRequest(method, endpoint string, body interface{}) (*APIResponse, error) {
    var reqBody io.Reader
    if body != nil {
        jsonBody, err := json.Marshal(body)
        if err != nil {
            return nil, err
        }
        reqBody = bytes.NewBuffer(jsonBody)
    }

    req, err := http.NewRequest(method, c.BaseURL+endpoint, reqBody)
    if err != nil {
        return nil, err
    }

    if body != nil {
        req.Header.Set("Content-Type", "application/json")
    }

    resp, err := c.HTTPClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var apiResp APIResponse
    if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
        return nil, err
    }

    if resp.StatusCode >= 400 {
        return nil, fmt.Errorf("API error: %s", apiResp.Error.Message)
    }

    return &apiResp, nil
}

func (c *RoachprodClient) ListClusters() ([]Cluster, error) {
    resp, err := c.makeRequest("GET", "/clusters", nil)
    if err != nil {
        return nil, err
    }

    var clusters []Cluster
    data, _ := json.Marshal(resp.Data)
    json.Unmarshal(data, &clusters)

    return clusters, nil
}

func (c *RoachprodClient) CreateCluster(cluster Cluster) (*Cluster, error) {
    resp, err := c.makeRequest("POST", "/clusters", cluster)
    if err != nil {
        return nil, err
    }

    var createdCluster Cluster
    data, _ := json.Marshal(resp.Data)
    json.Unmarshal(data, &createdCluster)

    return &createdCluster, nil
}

func (c *RoachprodClient) SyncClusters() (string, error) {
    resp, err := c.makeRequest("POST", "/clusters/sync", nil)
    if err != nil {
        return "", err
    }

    task := resp.Data.(map[string]interface{})
    return task["id"].(string), nil
}

func (c *RoachprodClient) GetTask(taskID string) (*Task, error) {
    resp, err := c.makeRequest("GET", "/tasks/"+taskID, nil)
    if err != nil {
        return nil, err
    }

    var task Task
    data, _ := json.Marshal(resp.Data)
    json.Unmarshal(data, &task)

    return &task, nil
}

func main() {
    client := NewRoachprodClient("http://localhost:8080")

    // Create test cluster
    cluster := Cluster{
        Name:     "go-test-cluster",
        Provider: "gce",
        Region:   "us-central1",
    }

    fmt.Println("Creating cluster...")
    created, err := client.CreateCluster(cluster)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Created: %s\n", created.Name)

    // Sync clusters
    fmt.Println("Syncing clusters...")
    taskID, err := client.SyncClusters()
    if err != nil {
        panic(err)
    }

    // Monitor task
    for {
        task, err := client.GetTask(taskID)
        if err != nil {
            panic(err)
        }

        fmt.Printf("Task status: %s\n", task.Status)

        if task.Status == "done" || task.Status == "failed" {
            break
        }

        time.Sleep(2 * time.Second)
    }

    // List clusters
    clusters, err := client.ListClusters()
    if err != nil {
        panic(err)
    }
    fmt.Printf("Total clusters: %d\n", len(clusters))
}
```

## Troubleshooting Scenarios

### Service Won't Start

```bash
#!/bin/bash
# troubleshoot-startup.sh

echo "Troubleshooting roachprod-centralized startup issues"
echo "=================================================="

# Check if port is already in use
echo "1. Checking if port 8080 is in use:"
lsof -i :8080 || echo "Port 8080 is available"

# Check if another instance is running
echo -e "\n2. Checking for existing processes:"
pgrep -f roachprod-centralized || echo "No existing processes found"

# Try minimal configuration
echo -e "\n3. Testing minimal configuration:"
export ROACHPROD_API_AUTHENTICATION_DISABLED=true
export ROACHPROD_DATABASE_TYPE=memory
export ROACHPROD_LOG_LEVEL=debug
export ROACHPROD_API_PORT=8080

echo "Starting with minimal config..."
timeout 10s ./dev run roachprod-centralized api &
sleep 5

# Test health endpoint
echo -e "\n4. Testing health endpoint:"
curl -f http://localhost:8080/health || echo "Health check failed"

# Kill any test processes
pkill -f roachprod-centralized
```

### Database Connection Issues

```bash
#!/bin/bash
# troubleshoot-database.sh

echo "Troubleshooting database connection issues"
echo "========================================="

# Test with memory database
echo "1. Testing with memory database:"
export ROACHPROD_DATABASE_TYPE=memory
./dev run roachprod-centralized api &
sleep 3
curl -f http://localhost:8080/health && echo "Memory database works"
pkill -f roachprod-centralized

# Test CockroachDB connection
echo -e "\n2. Testing CockroachDB connection:"
DB_URL="postgresql://root@localhost:26257/roachprod?sslmode=disable"

# Check if CockroachDB is running
if command -v cockroach &> /dev/null; then
    cockroach sql --url="$DB_URL" --execute="SELECT 1;" 2>/dev/null && echo "CockroachDB connection successful" || echo "CockroachDB connection failed"
else
    echo "CockroachDB client not found"
fi

# Test with CockroachDB
export ROACHPROD_DATABASE_TYPE=cockroachdb
export ROACHPROD_DATABASE_URL="$DB_URL"
./dev run roachprod-centralized api &
sleep 5
curl -f http://localhost:8080/health && echo "CockroachDB integration works"
pkill -f roachprod-centralized
```

### Performance Troubleshooting

```bash
#!/bin/bash
# troubleshoot-performance.sh

API_BASE="http://localhost:8080"

echo "Performance troubleshooting for roachprod-centralized"
echo "===================================================="

# Start service with performance monitoring
export ROACHPROD_LOG_LEVEL=debug
./dev run roachprod-centralized api &
SERVICE_PID=$!
sleep 5

echo "1. Basic performance test:"
time curl -s $API_BASE/health > /dev/null

echo -e "\n2. Concurrent requests test:"
for i in {1..10}; do
  curl -s $API_BASE/health > /dev/null &
done
wait

echo -e "\n3. Memory usage:"
ps -o pid,ppid,cmd,%mem,%cpu -p $SERVICE_PID

echo -e "\n4. Metrics endpoint:"
curl -s http://localhost:8081/metrics | grep roachprod | head -10

# Clean up
kill $SERVICE_PID
```

## Automation Scripts

### Continuous Monitoring Script

```bash
#!/bin/bash
# monitor-roachprod.sh

API_BASE="http://localhost:8080"
METRICS_BASE="http://localhost:8081"
LOG_FILE="/var/log/roachprod-monitor.log"

log() {
    echo "$(date): $1" | tee -a "$LOG_FILE"
}

check_health() {
    if curl -sf "$API_BASE/health" > /dev/null; then
        return 0
    else
        return 1
    fi
}

check_metrics() {
    if curl -sf "$METRICS_BASE/metrics" > /dev/null; then
        return 0
    else
        return 1
    fi
}

get_task_stats() {
    curl -s "$API_BASE/tasks" | jq -r '.data | group_by(.status) | map({status: .[0].status, count: length}) | .[] | "\(.status): \(.count)"'
}

monitor_loop() {
    while true; do
        if check_health; then
            log "✓ API health check passed"
        else
            log "✗ API health check failed"
        fi

        if check_metrics; then
            log "✓ Metrics endpoint accessible"
        else
            log "✗ Metrics endpoint failed"
        fi

        log "Task statistics:"
        get_task_stats | while read line; do
            log "  $line"
        done

        sleep 60
    done
}

log "Starting roachprod-centralized monitoring"
monitor_loop
```

### Deployment Health Check

```bash
#!/bin/bash
# deployment-health-check.sh

API_BASE="${1:-http://localhost:8080}"
TIMEOUT=300
INTERVAL=5

echo "Performing deployment health check for $API_BASE"
echo "Timeout: ${TIMEOUT}s, Check interval: ${INTERVAL}s"

start_time=$(date +%s)

while true; do
    current_time=$(date +%s)
    elapsed=$((current_time - start_time))

    if [ $elapsed -ge $TIMEOUT ]; then
        echo "❌ Health check timed out after ${TIMEOUT}s"
        exit 1
    fi

    echo "⏱️  Checking health... (${elapsed}s elapsed)"

    # Basic health check
    if ! curl -sf "$API_BASE/health" > /dev/null; then
        echo "   Health endpoint not responding, retrying..."
        sleep $INTERVAL
        continue
    fi

    # Check API functionality
    if ! curl -sf "$API_BASE/clusters" > /dev/null; then
        echo "   Clusters endpoint not responding, retrying..."
        sleep $INTERVAL
        continue
    fi

    # Check task endpoint
    if ! curl -sf "$API_BASE/tasks" > /dev/null; then
        echo "   Tasks endpoint not responding, retrying..."
        sleep $INTERVAL
        continue
    fi

    echo "✅ All health checks passed!"
    echo "🚀 Deployment is healthy and ready to serve traffic"
    exit 0
done
```

These examples provide comprehensive coverage of common roachprod-centralized usage patterns, from basic operations to advanced automation scenarios. They can be adapted for specific environments and use cases.
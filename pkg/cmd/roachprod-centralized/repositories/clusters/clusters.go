// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
)

var (
	// ErrClusterNotFound is returned when a cluster is not found.
	ErrClusterNotFound = fmt.Errorf("cluster not found")
	// ErrSyncLockHeld is returned when another instance holds the sync lock.
	ErrSyncLockHeld = fmt.Errorf("sync lock held by another instance")
)

// IClustersRepository defines the interface for cluster data persistence and distributed coordination.
// This repository manages roachprod cluster metadata, handles distributed synchronization across
// multiple instances, and provides operations queuing for eventual consistency in cluster state management.
type IClustersRepository interface {
	// GetClusters retrieves multiple clusters based on the provided filter criteria.
	GetClusters(ctx context.Context, l *logger.Logger, filters filtertypes.FilterSet) (cloudcluster.Clusters, error)
	// GetCluster retrieves a single cluster by its name.
	GetCluster(ctx context.Context, l *logger.Logger, name string) (cloudcluster.Cluster, error)
	// StoreClusters persists multiple clusters in a batch operation for efficiency.
	StoreClusters(ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters) error
	// StoreCluster persists a single cluster to the repository.
	StoreCluster(ctx context.Context, l *logger.Logger, cluster cloudcluster.Cluster) error
	// DeleteCluster removes a cluster from the repository.
	DeleteCluster(ctx context.Context, l *logger.Logger, cluster cloudcluster.Cluster) error

	// AcquireSyncLock attempts to acquire a distributed lock for cluster synchronization.
	// Returns true if the lock was acquired, false if another instance holds it.
	AcquireSyncLock(ctx context.Context, l *logger.Logger, instanceID string) (bool, error)
	// ReleaseSyncLock releases the distributed sync lock held by the given instance.
	ReleaseSyncLock(ctx context.Context, l *logger.Logger, instanceID string) error
	// GetSyncStatus returns the current synchronization status across all instances.
	GetSyncStatus(ctx context.Context, l *logger.Logger) (*SyncStatus, error)

	// EnqueueOperation adds a cluster operation to the pending operations queue.
	EnqueueOperation(ctx context.Context, l *logger.Logger, operation OperationData) error
	// ConditionalEnqueueOperation adds an operation only if a similar one isn't already queued.
	// Returns true if the operation was enqueued, false if a duplicate was found.
	ConditionalEnqueueOperation(ctx context.Context, l *logger.Logger, operation OperationData) (bool, error)
	// GetPendingOperations retrieves all operations waiting to be processed.
	GetPendingOperations(ctx context.Context, l *logger.Logger) ([]OperationData, error)
	// ClearPendingOperations removes all pending operations from the queue.
	ClearPendingOperations(ctx context.Context, l *logger.Logger) error
}

// SyncStatus represents the current distributed synchronization state across all instances.
// This helps coordinate cluster discovery activities to prevent conflicts and duplicate work.
type SyncStatus struct {
	InProgress bool      `json:"in_progress"`
	InstanceID string    `json:"instance_id,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
}

// OperationData represents a serializable cluster operation in the pending operations queue.
// These operations ensure eventual consistency when cluster changes cannot be applied immediately.
type OperationData struct {
	ID          string          `json:"id"`
	Type        OperationType   `json:"type"`
	ClusterName string          `json:"cluster_name"`
	ClusterData json.RawMessage `json:"cluster_data"`
	Timestamp   time.Time       `json:"timestamp"`
}

// OperationType defines the types of cluster operations that can be queued for processing.
type OperationType string

const (
	OperationTypeCreate OperationType = "create"
	OperationTypeDelete OperationType = "delete"
	OperationTypeUpdate OperationType = "update"
)

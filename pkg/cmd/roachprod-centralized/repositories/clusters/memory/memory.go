// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package memory

import (
	"context"
	"log/slog"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	filters "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// MemClustersRepo is a memory implementation of clusters repository.
type MemClustersRepo struct {
	clusters cloudcluster.Clusters
	lock     syncutil.Mutex

	// Distributed state (in-memory - no coordination across instances)
	syncState  clusters.SyncStatus
	operations []clusters.OperationData
}

// NewClustersRepository creates a new memory clusters repository.
func NewClustersRepository() *MemClustersRepo {
	return &MemClustersRepo{
		clusters: make(cloudcluster.Clusters),
	}
}

// GetClusters returns all clusters.
func (s *MemClustersRepo) GetClusters(
	ctx context.Context, l *logger.Logger, filterSet filtertypes.FilterSet,
) (cloudcluster.Clusters, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// If no filters are specified, return all tasks
	if filterSet.IsEmpty() {
		return s.clusters, nil
	}

	// Use the memory filter evaluator to filter tasks
	evaluator := filters.NewMemoryFilterEvaluator()
	filteredClusters := make(cloudcluster.Clusters)

	for clusterName, cluster := range s.clusters {
		matches, err := evaluator.Evaluate(cluster, &filterSet)
		if err != nil {
			l.Error(
				"Error filtering cluster, continuing with other clusters",
				slog.String("cluster_name", clusterName),
				slog.Any("error", err),
			)
			continue
		}

		if matches {
			filteredClusters[clusterName] = cluster
		}
	}

	return filteredClusters, nil
}

// GetCluster returns a cluster by name.
func (s *MemClustersRepo) GetCluster(
	ctx context.Context, l *logger.Logger, name string,
) (cloudcluster.Cluster, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if c, ok := s.clusters[name]; !ok {
		return cloudcluster.Cluster{}, clusters.ErrClusterNotFound
	} else {
		return *c, nil
	}
}

// StoreClusters stores all clusters.
func (s *MemClustersRepo) StoreClusters(
	ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clusters = clusters
	return nil
}

// StoreCluster stores a cluster.
func (s *MemClustersRepo) StoreCluster(
	ctx context.Context, l *logger.Logger, cluster cloudcluster.Cluster,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clusters[cluster.Name] = &cluster
	return nil
}

// DeleteCluster deletes a cluster.
func (s *MemClustersRepo) DeleteCluster(
	ctx context.Context, l *logger.Logger, cluster cloudcluster.Cluster,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.clusters, cluster.Name)
	return nil
}

// Distributed sync state management (stub implementations for memory)

// AcquireSyncLock attempts to acquire the sync lock.
// In memory implementation: always succeeds (no coordination across instances).
func (s *MemClustersRepo) AcquireSyncLock(
	ctx context.Context, l *logger.Logger, instanceID string,
) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// In memory: always allow lock acquisition
	s.syncState = clusters.SyncStatus{
		InProgress: true,
		InstanceID: instanceID,
		StartedAt:  timeutil.Now(),
	}

	return true, nil
}

// ReleaseSyncLock releases the sync lock.
func (s *MemClustersRepo) ReleaseSyncLock(
	ctx context.Context, l *logger.Logger, instanceID string,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.syncState.InstanceID == instanceID {
		s.syncState = clusters.SyncStatus{
			InProgress: false,
		}
	}

	return nil
}

// GetSyncStatus returns the current sync status.
func (s *MemClustersRepo) GetSyncStatus(
	ctx context.Context, l *logger.Logger,
) (*clusters.SyncStatus, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Return a copy
	status := s.syncState
	return &status, nil
}

// Operations queue management (stub implementations for memory)

// EnqueueOperation adds an operation to the queue.
func (s *MemClustersRepo) EnqueueOperation(
	ctx context.Context, l *logger.Logger, operation clusters.OperationData,
) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Set timestamp if not provided
	if operation.Timestamp.IsZero() {
		operation.Timestamp = timeutil.Now()
	}

	// Set ID if not provided
	if operation.ID == "" {
		operation.ID = uuid.MakeV4().String()
	}

	s.operations = append(s.operations, operation)
	return nil
}

// ConditionalEnqueueOperation adds an operation to the queue only if sync is in progress.
// Returns true if the operation was enqueued, false if not needed (no sync in progress).
func (s *MemClustersRepo) ConditionalEnqueueOperation(
	ctx context.Context, l *logger.Logger, operation clusters.OperationData,
) (bool, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Only enqueue if sync is in progress
	if !s.syncState.InProgress {
		return false, nil
	}

	// Set timestamp if not provided
	if operation.Timestamp.IsZero() {
		operation.Timestamp = timeutil.Now()
	}

	// Set ID if not provided
	if operation.ID == "" {
		operation.ID = uuid.MakeV4().String()
	}

	s.operations = append(s.operations, operation)
	return true, nil
}

// GetPendingOperations returns all pending operations.
func (s *MemClustersRepo) GetPendingOperations(
	ctx context.Context, l *logger.Logger,
) ([]clusters.OperationData, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Return a copy
	result := make([]clusters.OperationData, len(s.operations))
	copy(result, s.operations)
	return result, nil
}

// ClearPendingOperations removes all pending operations.
func (s *MemClustersRepo) ClearPendingOperations(ctx context.Context, l *logger.Logger) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.operations = nil
	return nil
}

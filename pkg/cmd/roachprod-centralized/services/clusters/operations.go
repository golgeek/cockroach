// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
)

// IOperation is an interface for operations on clusters.
type IOperation interface {
	applyOnRepository(ctx context.Context, l *logger.Logger, store clusters.IClustersRepository) error
	applyOnStagingClusters(ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters) error
}

// OperationCreate represents a create operation.
type OperationCreate struct {
	Cluster cloudcluster.Cluster
}

// applyOnRepository applies the operation on the repository.
func (o OperationCreate) applyOnRepository(
	ctx context.Context, l *logger.Logger, store clusters.IClustersRepository,
) error {
	return store.StoreCluster(ctx, l, o.Cluster)
}

// applyOnStagingClusters applies the operation on the staging clusters.
func (o OperationCreate) applyOnStagingClusters(
	ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters,
) error {
	clusters[o.Cluster.Name] = &o.Cluster
	return nil
}

// OperationDelete represents a delete operation.
type OperationDelete struct {
	Cluster cloudcluster.Cluster
}

// applyOnRepository applies the operation on the repository.
func (o OperationDelete) applyOnRepository(
	ctx context.Context, l *logger.Logger, store clusters.IClustersRepository,
) error {
	return store.DeleteCluster(ctx, l, o.Cluster)
}

// applyOnStagingClusters applies the operation on the staging clusters.
func (o OperationDelete) applyOnStagingClusters(
	ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters,
) error {
	delete(clusters, o.Cluster.Name)
	return nil
}

// OperationUpdate is an operation to update a cluster.
type OperationUpdate struct {
	Cluster cloudcluster.Cluster
}

// applyOnRepository applies the operation on the repository.
func (o OperationUpdate) applyOnRepository(
	ctx context.Context, l *logger.Logger, store clusters.IClustersRepository,
) error {
	return store.StoreCluster(ctx, l, o.Cluster)
}

// applyOnStagingClusters applies the operation on the staging clusters.
func (o OperationUpdate) applyOnStagingClusters(
	ctx context.Context, l *logger.Logger, clusters cloudcluster.Clusters,
) error {
	clusters[o.Cluster.Name] = &o.Cluster
	return nil
}

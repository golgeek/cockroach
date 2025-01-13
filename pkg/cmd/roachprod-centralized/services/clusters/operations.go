// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
)

type IOperation interface {
	applyOnRepository(ctx context.Context, store clusters.IClustersRepository) error
	applyOnStagingClusters(ctx context.Context, clusters cloud.Clusters) error
}

type OperationCreate struct {
	Cluster cloud.Cluster
}

func (o OperationCreate) applyOnRepository(
	ctx context.Context, store clusters.IClustersRepository,
) error {
	return store.StoreCluster(ctx, o.Cluster)
}

func (o OperationCreate) applyOnStagingClusters(
	ctx context.Context, clusters cloud.Clusters,
) error {
	clusters[o.Cluster.Name] = &o.Cluster
	return nil
}

type OperationDelete struct {
	Cluster cloud.Cluster
}

func (o OperationDelete) applyOnRepository(
	ctx context.Context, store clusters.IClustersRepository,
) error {
	return store.DeleteCluster(nil, o.Cluster)
}

func (o OperationDelete) applyOnStagingClusters(
	ctx context.Context, clusters cloud.Clusters,
) error {
	delete(clusters, o.Cluster.Name)
	return nil
}

type OperationUpdate struct {
	Cluster cloud.Cluster
}

func (o OperationUpdate) applyOnRepository(
	ctx context.Context, store clusters.IClustersRepository,
) error {
	return store.StoreCluster(ctx, o.Cluster)
}

func (o OperationUpdate) applyOnStagingClusters(
	ctx context.Context, clusters cloud.Clusters,
) error {
	clusters[o.Cluster.Name] = &o.Cluster
	return nil
}

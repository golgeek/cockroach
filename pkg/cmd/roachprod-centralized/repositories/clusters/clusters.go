// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
)

var (
	ErrClusterNotFound = fmt.Errorf("cluster not found")
)

type IClustersRepository interface {
	GetClusters(ctx context.Context) (cloud.Clusters, error)
	GetCluster(ctx context.Context, name string) (cloud.Cluster, error)
	StoreClusters(ctx context.Context, clusters cloud.Clusters) error
	StoreCluster(ctx context.Context, cluster cloud.Cluster) error
	DeleteCluster(ctx context.Context, cluster cloud.Cluster) error
}

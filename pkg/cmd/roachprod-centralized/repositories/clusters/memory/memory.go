// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package memory

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

type MemClustersRepo struct {
	clusters cloud.Clusters
	lock     syncutil.Mutex
}

func NewClustersRepository() *MemClustersRepo {
	return &MemClustersRepo{
		clusters: make(cloud.Clusters),
	}
}

func (s *MemClustersRepo) GetClusters(ctx context.Context) (cloud.Clusters, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.clusters, nil
}

func (s *MemClustersRepo) GetCluster(ctx context.Context, name string) (cloud.Cluster, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if c, ok := s.clusters[name]; !ok {
		return cloud.Cluster{}, clusters.ErrClusterNotFound
	} else {
		return *c, nil
	}
}

func (s *MemClustersRepo) StoreClusters(ctx context.Context, clusters cloud.Clusters) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clusters = clusters
	return nil
}

func (s *MemClustersRepo) StoreCluster(ctx context.Context, cluster cloud.Cluster) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.clusters[cluster.Name] = &cluster
	return nil
}

func (s *MemClustersRepo) DeleteCluster(ctx context.Context, cluster cloud.Cluster) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.clusters, cluster.Name)
	return nil
}

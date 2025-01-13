// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
)

// ClustersDTO is the data transfer object for a Cluster list.
type ClustersDTO struct {
	Data         *cloud.Clusters
	PublicError  error
	PrivateError error
}

// ClusterDTO is the data transfer object for a cluster.
type ClusterDTO struct {
	Data         *cloud.Cluster
	PublicError  error
	PrivateError error
}

// TaskDTO is the data transfer object for a task created by this service.
type TaskDTO struct {
	Data         tasks.ITask
	PublicError  error
	PrivateError error
}

// InputGetAllDTO is the data transfer object to get all clusters.
type InputGetAllClustersDTO struct {
	Username string `json:"username" binding:"omitempty,alphanum"`
}

// InputGetClusterDTO is the data transfer object to get a cluster.
type InputGetClusterDTO struct {
	Name string `json:"name" binding:"required"`
}

// InputCreateClusterDTO is the data transfer object to create a new cluster.
type InputCreateClusterDTO struct {
	cloud.Cluster
}

// InputUpdateClusterDTO is the data transfer object to update a cluster.
type InputUpdateClusterDTO struct {
	cloud.Cluster
}

// InputDeleteClusterDTO is the data transfer object to delete a cluster.
type InputDeleteClusterDTO struct {
	Name string `json:"name" binding:"required"`
}

package models

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
)

var (
	// ErrClusterNotFound is the error returned when a cluster is not found.
	ErrClusterNotFound = utils.NewPublicError(fmt.Errorf("cluster not found"))
	// ErrClusterAlreadyExists is the error returned when a cluster already
	// exists.
	ErrClusterAlreadyExists = utils.NewPublicError(fmt.Errorf("cluster already exists"))
	// ErrShutdownTimeout is the error returned when the service shutdown times out.
	ErrShutdownTimeout = fmt.Errorf("service shutdown timeout")
)

// IService is the interface for the clusters service.
type IService interface {
	SyncClouds(context.Context, *logger.Logger) (tasks.ITask, error)
	GetAllClusters(context.Context, *logger.Logger, InputGetAllClustersDTO) (cloudcluster.Clusters, error)
	GetAllDNSZoneVMs(context.Context, *logger.Logger, string) (vm.List, error)
	GetCluster(context.Context, *logger.Logger, InputGetClusterDTO) (*cloudcluster.Cluster, error)
	CreateCluster(context.Context, *logger.Logger, InputCreateClusterDTO) (*cloudcluster.Cluster, error)
	UpdateCluster(context.Context, *logger.Logger, InputUpdateClusterDTO) (*cloudcluster.Cluster, error)
	DeleteCluster(context.Context, *logger.Logger, InputDeleteClusterDTO) error
	Sync(ctx context.Context, l *logger.Logger) (cloudcluster.Clusters, error)
}

// InputGetAllClustersDTO is the data transfer object to get all clusters.
type InputGetAllClustersDTO struct {
	Filters filtertypes.FilterSet `json:"filters,omitempty"`
}

// InputGetClusterDTO is the data transfer object to get a cluster.
type InputGetClusterDTO struct {
	Name string `json:"name" binding:"required"`
}

// InputCreateClusterDTO is the data transfer object to create a new cluster.
type InputCreateClusterDTO struct {
	cloudcluster.Cluster
}

// InputUpdateClusterDTO is the data transfer object to update a cluster.
type InputUpdateClusterDTO struct {
	cloudcluster.Cluster
}

// InputDeleteClusterDTO is the data transfer object to delete a cluster.
type InputDeleteClusterDTO struct {
	Name string `json:"name" binding:"required"`
}

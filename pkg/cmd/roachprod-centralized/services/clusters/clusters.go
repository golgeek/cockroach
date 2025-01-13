// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"container/list"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks"
	"github.com/cockroachdb/cockroach/pkg/roachprod"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
	"github.com/cockroachdb/cockroach/pkg/roachprod/logger"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

const (
	// DefaultPeriodicRefreshInterval is the default interval at which the clusters are refreshed.
	DefaultPeriodicRefreshInterval = 10 * time.Minute
)

var (
	once sync.Once

	ErrClusterNotFound      = fmt.Errorf("cluster not found")
	ErrClusterAlreadyExists = fmt.Errorf("cluster already exists")
)

// IService is the interface for the clusters service.
type IService interface {
	SyncClouds(ctx context.Context, l *slog.Logger) *TaskDTO
	GetAllClusters(ctx context.Context, l *slog.Logger, input InputGetAllClustersDTO) *ClustersDTO
	GetCluster(ctx context.Context, l *slog.Logger, input InputGetClusterDTO) *ClusterDTO
	CreateCluster(ctx context.Context, l *slog.Logger, input InputCreateClusterDTO) *ClusterDTO
	UpdateCluster(ctx context.Context, l *slog.Logger, input InputUpdateClusterDTO) *ClusterDTO
	DeleteCluster(ctx context.Context, l *slog.Logger, input InputDeleteClusterDTO) *ClusterDTO
}

// Service is the implementation of the clusters service.
type Service struct {
	options Options

	_taskService        tasks.IService
	_store              clusters.IClustersRepository
	_operationsStackMux syncutil.Mutex
	_operationsStack    *list.List
	_syncing            bool
}

// Options contains the options for the clusters service.
type Options struct {
	PeriodicRefreshEnabled  bool
	PeriodicRefreshInterval time.Duration
}

// NewService creates a new clusters service.
func NewService(
	ctx context.Context,
	l *slog.Logger,
	store clusters.IClustersRepository,
	tasksService tasks.IService,
	options Options,
) (*Service, error) {

	// Initialize the roachprod providers
	// only during the first instantiation of the service.
	once.Do(func() {
		_ = roachprod.InitProviders()
	})

	service := &Service{
		_taskService:     tasksService,
		_store:           store,
		_operationsStack: list.New(),
		options:          options,
	}

	// Initial sync
	_, err := service.sync(ctx, l)
	if err != nil {
		return nil, err
	}

	// Register the tasks service.
	err = tasksService.RegisterTasksService(service)
	if err != nil {
		return nil, err
	}

	// If the periodic refresh is enabled, start the periodic refresh.
	if options.PeriodicRefreshEnabled {
		l.Info("periodic refresh is enabled")
		if options.PeriodicRefreshInterval == 0 {
			l.Info(
				"using default periodic refresh interval",
				slog.Duration("interval", DefaultPeriodicRefreshInterval),
			)
			service.options.PeriodicRefreshInterval = DefaultPeriodicRefreshInterval
		}

		// Periodically create a sync task to sync the clouds.
		go service.periodicRefresh(ctx, l)
	}

	return service, nil
}

func (s *Service) periodicRefresh(ctx context.Context, l *slog.Logger) {

	logger := l.With("service", "clusters", "method", "periodicRefresh")

	logger.Info(
		"starting periodic refresh routine",
		slog.Duration("interval", s.options.PeriodicRefreshInterval),
	)

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(s.options.PeriodicRefreshInterval):
			logger.Info("periodic refresh routine triggered")
			taskDTO := s.SyncClouds(ctx, logger)
			// TODO: what to do when error?
			switch {
			case taskDTO.PublicError != nil:
				logger.Error(
					"error in periodic refresh routine",
					slog.Any("error", taskDTO.PublicError),
				)
			case taskDTO.PrivateError != nil:
				logger.Error(
					"error in periodic refresh routine",
					slog.Any("error", taskDTO.PrivateError),
				)
			default:
				logger.Info(
					"periodic refresh routine done, task created",
					slog.String("task_id", taskDTO.Data.GetID().String()),
				)
			}
		}
	}
}

// SyncClouds syncs all clouds.
func (s *Service) SyncClouds(ctx context.Context, l *slog.Logger) *TaskDTO {

	// Create a task to sync the clouds.
	task := &TaskSync{}
	task.Type = string(ClustersTaskSync)

	// Save the task.
	taskServiceDTO := s._taskService.CreateTaskIfNotAlreadyPlanned(ctx, l, task)

	return &TaskDTO{
		Data:         taskServiceDTO.Data,
		PublicError:  taskServiceDTO.PublicError,
		PrivateError: taskServiceDTO.PrivateError,
	}
}

// GetAll returns all clusters.
func (s *Service) GetAllClusters(
	ctx context.Context, l *slog.Logger, input InputGetAllClustersDTO,
) *ClustersDTO {

	c, err := s._store.GetClusters(ctx)
	if err != nil {
		return &ClustersDTO{
			PrivateError: err,
		}
	}

	if input.Username != "" {
		regexp, err := regexp.Compile(fmt.Sprintf("^%s-.*", input.Username))
		if err != nil {
			return &ClustersDTO{
				PrivateError: err,
			}
		}
		c = c.FilterByName(regexp)
	}

	return &ClustersDTO{
		Data: &c,
	}
}

// GetCluster returns a cluster.
func (s *Service) GetCluster(
	ctx context.Context, l *slog.Logger, input InputGetClusterDTO,
) *ClusterDTO {

	c, err := s._store.GetCluster(ctx, input.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return &ClusterDTO{
				PublicError: ErrClusterNotFound,
			}
		}
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	return &ClusterDTO{
		Data: &c,
	}
}

// CreateCluster creates a cluster.
func (s *Service) CreateCluster(
	ctx context.Context, l *slog.Logger, input InputCreateClusterDTO,
) *ClusterDTO {

	l.Debug("Creating cluster", slog.String("name", input.Name), slog.Any("data", input.Cluster))

	// Check that the cluster does not already exist.
	_, err := s._store.GetCluster(ctx, input.Name)
	if err == nil {
		l.Debug("Cluster with this name already exists", slog.String("name", input.Name))
		return &ClusterDTO{
			PublicError: ErrClusterAlreadyExists,
		}
	}

	// Create the operation.
	op := OperationCreate{
		Cluster: input.Cluster,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, s._store)
	if err != nil {
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	// If we are syncing, we need to keep track of the operation
	// to be able to replay it at the end of the sync.
	if s._syncing {
		s.maybeEnqueueOperation(op)
	}

	// Return the created cluster.
	return &ClusterDTO{
		Data: &input.Cluster,
	}
}

// UpdateCluster updates a cluster.
func (s *Service) UpdateCluster(
	ctx context.Context, l *slog.Logger, input InputUpdateClusterDTO,
) *ClusterDTO {

	// Check that the cluster exists.
	_, err := s._store.GetCluster(ctx, input.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return &ClusterDTO{
				PublicError: ErrClusterNotFound,
			}
		}
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	// Create the operation.
	op := OperationUpdate{
		Cluster: input.Cluster,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, s._store)
	if err != nil {
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	s.maybeEnqueueOperation(OperationUpdate{
		Cluster: input.Cluster,
	})

	return &ClusterDTO{
		Data: &input.Cluster,
	}
}

// DeleteCluster deletes a cluster.
func (s *Service) DeleteCluster(
	ctx context.Context, l *slog.Logger, input InputDeleteClusterDTO,
) *ClusterDTO {

	// Check that the cluster exists.
	c, err := s._store.GetCluster(ctx, input.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return &ClusterDTO{
				PublicError: ErrClusterNotFound,
			}
		}
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	// Create the operation.
	op := OperationDelete{
		Cluster: c,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, s._store)
	if err != nil {
		return &ClusterDTO{
			PrivateError: err,
		}
	}

	s.maybeEnqueueOperation(OperationDelete{
		Cluster: c,
	})

	return &ClusterDTO{
		Data: &c,
	}
}

// sync contains the logic to sync the clusters and store them
// in the repository.
func (s *Service) sync(ctx context.Context, l *slog.Logger) (cloud.Clusters, error) {

	s._syncing = true
	defer func(s *Service) {
		l.Info("syncing clusters is over, releasing syncing flag")
		s._syncing = false
	}(s)

	crlLogger, err := (&logger.Config{}).NewLogger("")
	if err != nil {
		return nil, err
	}

	l.Info("syncing clusters")
	c, err := cloud.ListCloud(crlLogger, vm.ListOptions{
		IncludeVolumes:       true,
		ComputeEstimatedCost: true,
		IncludeProviders:     []string{"aws", "gce", "azure"},
	})
	if err != nil {
		return nil, err
	}

	// Replay the operations stack
	l.Info(
		"cloud listing done, replaying operations stack",
		slog.Int("operations", s._operationsStack.Len()),
	)
	for el := s._operationsStack.Front(); el != nil; el = el.Next() {

		// Perform the operation on the new refreshed clusters
		op := el.Value.(IOperation)
		err := op.applyOnStagingClusters(ctx, c.Clusters)
		if err != nil {
			l.Error("unable to replay operation", "operation", op, "error", err)
		}

		s.deleteStackedOperation(el)
	}

	l.Info("storing new set of clusters in the repository")
	err = s._store.StoreClusters(ctx, c.Clusters)
	if err != nil {
		return nil, err
	}

	return c.Clusters, nil
}

// enqueueOperation enqueues an operation in the stack.
func (s *Service) maybeEnqueueOperation(op IOperation) {

	// If we are not syncing, we do not need to keep track of the operation
	// to replay it on the new set of data.
	if !s._syncing {
		return
	}

	s._operationsStackMux.Lock()
	defer s._operationsStackMux.Unlock()
	s._operationsStack.PushBack(op)
}

// deleteStackedOperation deletes an operation from the stack.
func (s *Service) deleteStackedOperation(el *list.Element) {
	s._operationsStackMux.Lock()
	defer s._operationsStackMux.Unlock()
	s._operationsStack.Remove(el)
}

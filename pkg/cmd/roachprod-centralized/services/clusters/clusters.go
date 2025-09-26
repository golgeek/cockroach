// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	configtypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/config/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/models"
	sclustertasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/health"
	dnstasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/public-dns/tasks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
	cloudcluster "github.com/cockroachdb/cockroach/pkg/roachprod/cloud/types"
	rplogger "github.com/cockroachdb/cockroach/pkg/roachprod/logger"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/aws"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/azure"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/gce"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/ibm"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

const (
	// DefaultPeriodicRefreshInterval is the default interval at which the clusters are refreshed.
	DefaultPeriodicRefreshInterval = 10 * time.Minute
)

// Service implements the clusters service interface and manages cluster discovery and synchronization
// across multiple cloud providers. It coordinates with cloud APIs to discover roachprod clusters
// and maintains an up-to-date view of the distributed cluster infrastructure.
type Service struct {
	options Options

	backgroundJobsCancelFunc context.CancelFunc
	backgroundJobsWg         *sync.WaitGroup
	roachprodCloud           *cloud.Cloud

	_dnsZoneToProviders map[string][]string
	_taskService        stasks.IService
	_store              clusters.IClustersRepository
	_healthService      health.IHealthService
}

// Options contains configuration parameters for the clusters service.
type Options struct {
	// CloudProviders lists the cloud providers to monitor for clusters.
	CloudProviders []configtypes.CloudProvider
	// PeriodicRefreshEnabled controls whether background cluster synchronization is enabled.
	PeriodicRefreshEnabled bool
	// PeriodicRefreshInterval specifies how often to sync clusters from cloud providers.
	PeriodicRefreshInterval time.Duration
	// NoInitialSync skips the initial cluster synchronization on service startup.
	NoInitialSync bool
}

// NewService creates a new clusters service.
func NewService(
	store clusters.IClustersRepository,
	tasksService stasks.IService,
	healthService health.IHealthService,
	options Options,
) (*Service, error) {

	service := &Service{
		backgroundJobsWg:    &sync.WaitGroup{},
		_taskService:        tasksService,
		_store:              store,
		_healthService:      healthService,
		options:             options,
		_dnsZoneToProviders: make(map[string][]string),
	}

	cloudProviders := []vm.Provider{}
	for _, cp := range options.CloudProviders {

		var provider vm.Provider
		var err error
		switch strings.ToLower(cp.Type) {
		case gce.ProviderName:
			provider, err = gce.NewProvider(cp.GCE.ToOptions()...)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create GCE cloud provider")
			}

		case aws.ProviderName:
			provider, err = aws.NewProvider(cp.AWS.ToOptions()...)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create AWS cloud provider")
			}

		case azure.ProviderName:
			provider, err = azure.NewProvider(cp.Azure.ToOptions()...)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create Azure cloud provider")
			}

		case ibm.ProviderName:
			provider, err = ibm.NewProvider(cp.IBM.ToOptions()...)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create IBM cloud provider")
			}

		default:
			return nil, errors.Newf("unknown cloud provider type: %s", cp.Type)
		}

		cloudProviders = append(cloudProviders, provider)
		if service._dnsZoneToProviders[cp.DNSPublicZone] == nil {
			service._dnsZoneToProviders[cp.DNSPublicZone] = []string{}
		}
		service._dnsZoneToProviders[cp.DNSPublicZone] = append(service._dnsZoneToProviders[cp.DNSPublicZone], provider.String())
	}

	// Initialize roachprod Cloud with the desired providers.
	service.roachprodCloud = cloud.NewCloud(
		cloud.WithProviders(cloudProviders),
	)

	// If the periodic refresh interval is not set, use the default.
	if service.options.PeriodicRefreshInterval == 0 {
		service.options.PeriodicRefreshInterval = DefaultPeriodicRefreshInterval
	}

	return service, nil
}

// RegisterTasks registers the cluster tasks with the tasks service.
func (s *Service) RegisterTasks(ctx context.Context) error {
	// Register the cluster tasks with the tasks service.
	if s._taskService != nil {
		s._taskService.RegisterTasksService(s)
	}
	return nil
}

func (s *Service) StartService(ctx context.Context, l *logger.Logger) error {
	// Initialize the background jobs wait group.
	s.backgroundJobsWg = &sync.WaitGroup{}

	// Initial sync
	if !s.options.NoInitialSync {
		_, err := s.Sync(ctx, l)
		if err != nil {
			return err
		}
	}

	return nil
}

// StartBackgroundWork initializes the service by making the initial clusters sync
// and starting the periodic refresh if enabled.
func (s *Service) StartBackgroundWork(
	ctx context.Context, l *logger.Logger, errChan chan<- error,
) error {

	// Create a new context without the parent cancel because we prefer
	// to properly handle the context cancellation in the Shutdown method
	// that is called by our parent when the app is stopping.
	ctx, s.backgroundJobsCancelFunc = context.WithCancel(context.WithoutCancel(ctx))

	// If there is a task service and the periodic refresh is enabled,
	// start the periodic refresh.
	if s._taskService != nil {
		if s.options.PeriodicRefreshEnabled {
			s.periodicRefresh(ctx, l, errChan)
		}
	}
	return nil
}

// Shutdown contains the logic to shutdown the service.
// It waits for the background jobs to finish and cancels the periodic refresh
// context.
func (s *Service) Shutdown(ctx context.Context) error {

	done := make(chan struct{})

	// Start a goroutine to wait for the workers to finish
	go func() {
		s.backgroundJobsWg.Wait()
		close(done)
	}()

	// Cancel the background context to stop the periodic refresh.
	if s.backgroundJobsCancelFunc != nil {
		s.backgroundJobsCancelFunc()
	}

	// Wait for either the workers to finish or the context to be cancelled
	select {
	case <-ctx.Done():
		// If context is done (e.g., due to timeout), return the error
		return models.ErrShutdownTimeout
	case <-done:
		// If all workers finish successfully, return nil
		return nil
	}

}

// periodicRefresh is a routine that periodically refreshes the clusters.
func (s *Service) periodicRefresh(ctx context.Context, l *logger.Logger, errChan chan<- error) {

	refreshLogger := l.With(
		slog.String("service", "clusters"),
		slog.String("routine", "periodicRefresh"),
	)

	refreshLogger.Info(
		"starting periodic refresh routine",
		slog.Duration("interval", s.options.PeriodicRefreshInterval),
	)

	s.backgroundJobsWg.Add(1)
	go func() {
		defer s.backgroundJobsWg.Done()
		for {
			select {
			case <-ctx.Done():
				refreshLogger.Debug("Stopping periodic refresh routine")
				return

			case <-time.After(s.options.PeriodicRefreshInterval):

				refreshLogger.Debug("Periodic refresh routine triggered")

				// Check if a sync task has been created recently by another instance
				if s.hasRecentSyncTask(ctx, refreshLogger) {
					refreshLogger.Debug("Recent sync task found, skipping sync task creation")
					continue
				}

				task, err := s.SyncClouds(ctx, refreshLogger)
				if err != nil {
					errChan <- errors.Wrap(err, "error in periodic refresh routine")
					continue
				}
				refreshLogger.Info(
					"Task created from periodic refresh routine",
					slog.String("task_id", task.GetID().String()),
				)

			}
		}
	}()
}

// GetTaskServiceName returns the name of the task service.
func (s *Service) GetTaskServiceName() string {
	return "clusters"
}

// GetHandledTasks returns a map of task types to task implementations that are
// handled by this service.
func (s *Service) GetHandledTasks() map[string]stasks.ITask {
	return map[string]stasks.ITask{
		string(sclustertasks.ClustersTaskSync): &sclustertasks.TaskSync{
			Service: s,
		},
	}
}

// SyncClouds creates a task to sync the clouds.
func (s *Service) SyncClouds(ctx context.Context, l *logger.Logger) (tasks.ITask, error) {

	// Create a task to sync the clouds.
	task := &sclustertasks.TaskSync{}
	task.Type = string(sclustertasks.ClustersTaskSync)

	// Save the task.
	return s._taskService.CreateTaskIfNotAlreadyPlanned(ctx, l, task)
}

// GetAll returns all clusters.
func (s *Service) GetAllClusters(
	ctx context.Context, l *logger.Logger, input models.InputGetAllClustersDTO,
) (cloudcluster.Clusters, error) {

	c, err := s._store.GetClusters(ctx, l, input.Filters)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (s *Service) GetAllDNSZoneVMs(
	ctx context.Context, l *logger.Logger, dnsZone string,
) (vm.List, error) {

	// Build the list of providers matching this DNS zone
	var providers []string
	if p, ok := s._dnsZoneToProviders[dnsZone]; ok {
		providers = p
	}

	c, err := s._store.GetClusters(ctx, l, *filters.NewFilterSet())
	if err != nil {
		return nil, err
	}

	var vms vm.List
	for _, cluster := range c {
		for _, vm := range cluster.VMs {
			if slices.Contains(providers, fmt.Sprintf("%s-%s", vm.Provider, vm.ProviderAccountID)) {
				vms = append(vms, vm)
			}
		}
	}

	return vms, nil
}

// GetCluster returns a cluster.
func (s *Service) GetCluster(
	ctx context.Context, l *logger.Logger, input models.InputGetClusterDTO,
) (*cloudcluster.Cluster, error) {

	c, err := s._store.GetCluster(ctx, l, input.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return nil, models.ErrClusterNotFound
		}
		return nil, err
	}

	return &c, nil
}

// CreateCluster creates a cluster.
func (s *Service) CreateCluster(
	ctx context.Context, l *logger.Logger, input models.InputCreateClusterDTO,
) (*cloudcluster.Cluster, error) {

	// Check that the cluster does not already exist.
	_, err := s._store.GetCluster(ctx, l, input.Cluster.Name)
	if err == nil {
		return nil, models.ErrClusterAlreadyExists
	}

	// Create the operation.
	op := OperationCreate{
		Cluster: input.Cluster,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, l, s._store)
	if err != nil {
		return nil, err
	}

	// If another instance is syncing, enqueue this operation for later replay
	opData, err := s.operationToOperationData(op)
	if err != nil {
		l.Error("failed to convert operation to data",
			slog.Any("error", err),
			slog.String("cluster_name", input.Cluster.Name),
			slog.String("operation_type", "create"))
	} else {
		enqueued, err := s.conditionalEnqueueOperationWithHealthCheck(ctx, l, opData)
		if err != nil {
			l.Error("failed to enqueue operation",
				slog.Any("error", err),
				slog.String("cluster_name", input.Cluster.Name),
				slog.String("operation_type", "create"),
				slog.String("operation_id", opData.ID))
		} else if enqueued {
			l.Debug("operation enqueued for sync replay", slog.String("operation_id", opData.ID))
		}
	}

	// After a cluster create, we trigger a task to sync the DNS.
	err = s.maybeEnqueuePublicDNSSyncTaskService(ctx, l)
	if err != nil {
		l.Error("failed to enqueue public DNS sync task",
			slog.Any("error", err),
			slog.String("cluster_name", input.Cluster.Name),
			slog.String("trigger_operation", "create"))
	}

	// Return the created cluster.
	return &input.Cluster, nil
}

// UpdateCluster updates a cluster.
func (s *Service) UpdateCluster(
	ctx context.Context, l *logger.Logger, input models.InputUpdateClusterDTO,
) (*cloudcluster.Cluster, error) {

	// Check that the cluster exists.
	_, err := s._store.GetCluster(ctx, l, input.Cluster.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return nil, models.ErrClusterNotFound
		}
		return nil, err
	}

	// Create the operation.
	op := OperationUpdate{
		Cluster: input.Cluster,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, l, s._store)
	if err != nil {
		return nil, err
	}

	// If another instance is syncing, enqueue this operation for later replay
	opData, err := s.operationToOperationData(OperationUpdate{Cluster: input.Cluster})
	if err != nil {
		l.Error("failed to convert operation to data",
			slog.Any("error", err),
			slog.String("cluster_name", input.Cluster.Name),
			slog.String("operation_type", "update"))
	} else {
		enqueued, err := s.conditionalEnqueueOperationWithHealthCheck(ctx, l, opData)
		if err != nil {
			l.Error("failed to enqueue operation",
				slog.Any("error", err),
				slog.String("cluster_name", input.Cluster.Name),
				slog.String("operation_type", "update"),
				slog.String("operation_id", opData.ID))
		} else if enqueued {
			l.Debug("operation enqueued for sync replay", slog.String("operation_id", opData.ID))
		}
	}

	// After a cluster update, we trigger a task to sync the DNS.
	err = s.maybeEnqueuePublicDNSSyncTaskService(ctx, l)
	if err != nil {
		l.Error("failed to enqueue public DNS sync task",
			slog.Any("error", err),
			slog.String("cluster_name", input.Cluster.Name),
			slog.String("trigger_operation", "update"))
	}

	return &input.Cluster, nil
}

// DeleteCluster deletes a cluster.
func (s *Service) DeleteCluster(
	ctx context.Context, l *logger.Logger, input models.InputDeleteClusterDTO,
) error {

	// Check that the cluster exists.
	c, err := s._store.GetCluster(ctx, l, input.Name)
	if err != nil {
		if errors.Is(err, clusters.ErrClusterNotFound) {
			return models.ErrClusterNotFound
		}
		return err
	}

	// Create the operation.
	op := OperationDelete{
		Cluster: c,
	}

	// Apply the operation on the current repository.
	err = op.applyOnRepository(ctx, l, s._store)
	if err != nil {
		return err
	}

	// If another instance is syncing, enqueue this operation for later replay
	opData, err := s.operationToOperationData(OperationDelete{Cluster: c})
	if err != nil {
		l.Error("failed to convert operation to data",
			slog.Any("error", err),
			slog.String("cluster_name", input.Name),
			slog.String("operation_type", "delete"))
	} else {
		enqueued, err := s.conditionalEnqueueOperationWithHealthCheck(ctx, l, opData)
		if err != nil {
			l.Error("failed to enqueue operation",
				slog.Any("error", err),
				slog.String("cluster_name", input.Name),
				slog.String("operation_type", "delete"),
				slog.String("operation_id", opData.ID))
		} else if enqueued {
			l.Debug("operation enqueued for sync replay", slog.String("operation_id", opData.ID))
		}
	}

	// After a cluster delete, we trigger a task to sync the DNS.
	err = s.maybeEnqueuePublicDNSSyncTaskService(ctx, l)
	if err != nil {
		l.Error("failed to enqueue public DNS sync task",
			slog.Any("error", err),
			slog.String("cluster_name", input.Name),
			slog.String("trigger_operation", "delete"))
	}

	return nil
}

// sync contains the logic to sync the clusters and store them
// in the repository.
func (s *Service) Sync(ctx context.Context, l *logger.Logger) (cloudcluster.Clusters, error) {
	instanceID := s._healthService.GetInstanceID()

	// Try to acquire the distributed sync lock
	acquired, err := s.acquireSyncLockWithHealthCheck(ctx, l, instanceID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to acquire sync lock")
	}

	if !acquired {
		l.Info("another instance is already syncing, skipping sync")
		return s._store.GetClusters(ctx, l, *filters.NewFilterSet())
	}

	l.Debug("acquired sync lock, starting cluster sync", slog.String("instance_id", instanceID))
	defer func() {
		if err := s._store.ReleaseSyncLock(ctx, l, instanceID); err != nil {
			l.Error("failed to release sync lock",
				slog.Any("error", err),
				slog.String("instance_id", instanceID))
		} else {
			l.Debug("released sync lock")
		}
	}()

	crlLogger, err := (&rplogger.Config{Stdout: l, Stderr: l}).NewLogger("")
	if err != nil {
		return nil, err
	}

	l.Info("syncing clusters")
	err = s.roachprodCloud.ListCloud(crlLogger, vm.ListOptions{
		IncludeVolumes:       true,
		ComputeEstimatedCost: false,
	})
	if err != nil {
		return nil, err
	}

	// Get pending operations from the repository
	pendingOps, err := s._store.GetPendingOperations(ctx, l)
	if err != nil {
		l.Error("failed to get pending operations",
			slog.Any("error", err),
			slog.String("instance_id", instanceID))
		pendingOps = []clusters.OperationData{} // Continue with empty operations
	}

	// Replay the pending operations on the fresh cluster data
	l.Debug("cloud listing done, replaying pending operations", slog.Int("operations", len(pendingOps)))
	for _, opData := range pendingOps {
		op, err := s.operationDataToOperation(opData)
		if err != nil {
			l.Error("failed to convert operation data",
				slog.Any("error", err),
				slog.String("operation_id", opData.ID),
				slog.String("operation_type", string(opData.Type)))
			continue
		}

		err = op.applyOnStagingClusters(ctx, l, s.roachprodCloud.Clusters)
		if err != nil {
			l.Error("unable to replay operation",
				slog.Any("error", err),
				slog.String("operation_id", opData.ID),
				slog.String("operation_type", string(opData.Type)),
				slog.String("cluster_name", opData.ClusterName))
		}
	}

	l.Debug("storing new set of clusters in the repository")
	err = s._store.StoreClusters(ctx, l, s.roachprodCloud.Clusters)
	if err != nil {
		return nil, err
	}

	// Clear the operations queue after successful sync
	if err := s._store.ClearPendingOperations(ctx, l); err != nil {
		l.Error("failed to clear pending operations",
			slog.Any("error", err),
			slog.String("instance_id", instanceID))
	}

	// After a successful sync, we trigger a task to sync the DNS.
	err = s.maybeEnqueuePublicDNSSyncTaskService(ctx, l)
	if err != nil {
		l.Error("failed to enqueue public DNS sync task",
			slog.Any("error", err),
			slog.String("trigger_operation", "sync"),
			slog.String("instance_id", instanceID))
	}

	return s.roachprodCloud.Clusters, nil
}

// hasRecentSyncTask checks if a sync task has been created recently by another instance.
func (s *Service) hasRecentSyncTask(ctx context.Context, l *logger.Logger) bool {
	if s._taskService == nil {
		return false
	}

	// Check for sync tasks created within the periodic refresh interval
	pendingTasks, err := s._taskService.GetTasks(
		ctx,
		l,
		stasks.InputGetAllTasksDTO{
			Filters: *filters.NewFilterSet().
				AddFilter("Type", filtertypes.OpEqual, string(sclustertasks.ClustersTaskSync)).
				AddFilter("State", filtertypes.OpNotEqual, string(tasks.TaskStateFailed)).
				AddFilter("CreationDatetime", filtertypes.OpGreater, timeutil.Now().Add(-s.options.PeriodicRefreshInterval)),
		},
	)
	if err != nil {
		l.Error("failed to check for recent sync tasks",
			slog.Any("error", err),
			slog.Duration("interval", s.options.PeriodicRefreshInterval))
		return false
	}

	return len(pendingTasks) > 0
}

// operationToOperationData converts an IOperation to OperationData for storage.
func (s *Service) operationToOperationData(op IOperation) (clusters.OperationData, error) {
	var opType clusters.OperationType
	var cluster cloudcluster.Cluster

	switch o := op.(type) {
	case OperationCreate:
		opType = clusters.OperationTypeCreate
		cluster = o.Cluster
	case OperationUpdate:
		opType = clusters.OperationTypeUpdate
		cluster = o.Cluster
	case OperationDelete:
		opType = clusters.OperationTypeDelete
		cluster = o.Cluster
	default:
		return clusters.OperationData{}, fmt.Errorf("unknown operation type: %T", op)
	}

	clusterData, err := json.Marshal(cluster)
	if err != nil {
		return clusters.OperationData{}, errors.Wrap(err, "failed to marshal cluster data")
	}

	return clusters.OperationData{
		Type:        opType,
		ClusterName: cluster.Name,
		ClusterData: clusterData,
		Timestamp:   timeutil.Now(),
	}, nil
}

// operationDataToOperation converts OperationData back to an IOperation.
func (s *Service) operationDataToOperation(opData clusters.OperationData) (IOperation, error) {
	var cluster cloudcluster.Cluster
	if err := json.Unmarshal(opData.ClusterData, &cluster); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cluster data")
	}

	switch opData.Type {
	case clusters.OperationTypeCreate:
		return OperationCreate{Cluster: cluster}, nil
	case clusters.OperationTypeUpdate:
		return OperationUpdate{Cluster: cluster}, nil
	case clusters.OperationTypeDelete:
		return OperationDelete{Cluster: cluster}, nil
	default:
		return nil, fmt.Errorf("unknown operation type: %s", opData.Type)
	}
}

func (s *Service) maybeEnqueuePublicDNSSyncTaskService(
	ctx context.Context, l *logger.Logger,
) error {
	if s._taskService == nil {
		return nil
	}

	// Create a task to sync the public DNS.
	task := &dnstasks.TaskSync{}
	task.Type = string(dnstasks.PublicDNSTaskSync)

	// Save the task.
	_, err := s._taskService.CreateTaskIfNotAlreadyPlanned(ctx, l, task)
	if err != nil {
		return err
	}
	return nil
}

// acquireSyncLockWithHealthCheck attempts to acquire the sync lock with health verification logic in the service layer.
func (s *Service) acquireSyncLockWithHealthCheck(
	ctx context.Context, l *logger.Logger, instanceID string,
) (bool, error) {
	// First check if sync is already in progress by a healthy instance
	status, err := s._store.GetSyncStatus(ctx, l)
	if err != nil {
		return false, errors.Wrap(err, "failed to get sync status")
	}

	if status.InProgress && status.InstanceID != "" {
		// Check if the syncing instance is healthy using the health service
		healthy, err := s._healthService.IsInstanceHealthy(ctx, l, status.InstanceID)
		if err != nil {
			return false, errors.Wrap(err, "failed to check instance health")
		}
		if healthy {
			// Another healthy instance is syncing
			return false, nil
		}
	}

	// Try to acquire the lock
	return s._store.AcquireSyncLock(ctx, l, instanceID)
}

// conditionalEnqueueOperationWithHealthCheck enqueues operation with health verification logic in the service layer.
func (s *Service) conditionalEnqueueOperationWithHealthCheck(
	ctx context.Context, l *logger.Logger, operation clusters.OperationData,
) (bool, error) {
	// First get the sync status
	status, err := s._store.GetSyncStatus(ctx, l)
	if err != nil {
		return false, errors.Wrap(err, "failed to get sync status")
	}

	if !status.InProgress || status.InstanceID == "" {
		return false, nil // No sync in progress
	}

	// Check if the syncing instance is healthy using the health service
	healthy, err := s._healthService.IsInstanceHealthy(ctx, l, status.InstanceID)
	if err != nil {
		return false, errors.Wrap(err, "failed to check instance health")
	}

	if !healthy {
		return false, nil // Syncing instance is not healthy
	}

	// Enqueue the operation
	return s._store.ConditionalEnqueueOperation(ctx, l, operation)
}

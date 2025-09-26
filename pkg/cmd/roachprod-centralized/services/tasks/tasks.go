// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	tasksrepo "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// DefaultTasksTimeout is the default timeout for tasks.
	DefaultTasksTimeout = 30 * time.Second
	// DefaultTasksWorkers is the default number of workers processing tasks.
	DefaultTasksWorkers = 1
)

// Service implements the tasks service interface and manages background task processing.
// It coordinates between task producers (other services) and task consumers (workers)
// to ensure efficient and reliable task execution across the system.
type Service struct {
	options Options

	consumerID uuid.UUID
	store      tasksrepo.ITasksRepository
	metrics    *metrics

	managedPkgs  map[string]bool
	managedTasks map[string]types.ITask

	backgroundJobsCancelFunc context.CancelFunc
	backgroundJobsWg         *sync.WaitGroup
}

// Options contains configuration parameters for the tasks service.
type Options struct {
	// Workers specifies the number of concurrent workers that process tasks.
	// Higher values increase throughput but consume more resources.
	Workers int

	// CollectMetrics is a flag to enable metrics collection.
	CollectMetrics bool

	// TasksTimeout is the timeout for tasks.
	DefaultTasksTimeout time.Duration

	// PurgeDoneTaskOlderThan is the duration after which tasks in done state
	// are purged from the repository.
	PurgeDoneTaskOlderThan time.Duration
	// PurgeFailedTaskOlderThan is the duration after which tasks in failed
	// state are purged from the repository.
	PurgeFailedTaskOlderThan time.Duration
	// PurgeTaskOlderThan is the value for how often tasks are purged from the
	// repository.
	PurgeTasksInterval time.Duration
	// StatisticsUpdateInterval is the value for how often the tasks statistics
	// are updated.
	StatisticsUpdateInterval time.Duration
}

// metrics contains Prometheus metrics for monitoring tasks service performance and health.
// These metrics provide visibility into task processing rates, queue depths, and system status.
type metrics struct {
	processedTasksProcessed prometheus.Counter
	totalTasksPending       prometheus.Gauge
	totalTasksRunning       prometheus.Gauge
	totalTasksDone          prometheus.Gauge
	totalTasksFailed        prometheus.Gauge
}

// NewService creates a new tasks service.
func NewService(store tasksrepo.ITasksRepository, options Options) *Service {

	consumerID := uuid.MakeV4()

	if options.Workers == 0 {
		options.Workers = DefaultTasksWorkers
	}
	if options.DefaultTasksTimeout == 0 {
		options.DefaultTasksTimeout = DefaultTasksTimeout
	}
	if options.PurgeDoneTaskOlderThan == 0 {
		options.PurgeDoneTaskOlderThan = DefaultPurgeDoneTaskOlderThan
	}
	if options.PurgeFailedTaskOlderThan == 0 {
		options.PurgeFailedTaskOlderThan = DefaultPurgeFailedTaskOlderThan
	}
	if options.PurgeTasksInterval == 0 {
		options.PurgeTasksInterval = DefaultPurgeTasksInterval
	}
	if options.StatisticsUpdateInterval == 0 {
		options.StatisticsUpdateInterval = DefaultStatisticsUpdateInterval
	}

	s := &Service{
		options:          options,
		consumerID:       consumerID,
		store:            store,
		managedPkgs:      make(map[string]bool),
		managedTasks:     make(map[string]types.ITask),
		backgroundJobsWg: &sync.WaitGroup{},
	}

	if options.CollectMetrics {
		s.metrics = s.newMetrics()
	}

	return s
}

// newMetrics creates a new Prometheus metrics instance.
func (s *Service) newMetrics() *metrics {

	promGlobalLabels := prometheus.Labels{
		"consumer": s.consumerID.String(),
	}

	return &metrics{
		// Prometheus counter of tasks processed
		processedTasksProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name:        "processed_total",
			Help:        "The total number of processed tasks",
			Namespace:   types.PROMETHEUS_NAMESPACE,
			ConstLabels: promGlobalLabels,
		}),
		// Prometheus gauges of pending tasks in the database
		totalTasksPending: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        string(tasks.TaskStatePending),
			Help:        "The number of tasks to be processed in the database",
			Namespace:   types.PROMETHEUS_NAMESPACE,
			ConstLabels: promGlobalLabels,
		}),
		// Prometheus gauges of running tasks in the database
		totalTasksRunning: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        string(tasks.TaskStateRunning),
			Help:        "The number of tasks being processed in the database",
			Namespace:   types.PROMETHEUS_NAMESPACE,
			ConstLabels: promGlobalLabels,
		}),
		// Prometheus gauges of done tasks in the database
		totalTasksDone: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        string(tasks.TaskStateDone),
			Help:        "The number of tasks processed successfully in the database",
			Namespace:   types.PROMETHEUS_NAMESPACE,
			ConstLabels: promGlobalLabels,
		}),
		// Prometheus gauges of failed tasks in the database
		totalTasksFailed: promauto.NewGauge(prometheus.GaugeOpts{
			Name:        string(tasks.TaskStateFailed),
			Help:        "The number of tasks that failed to be processed in the database",
			Namespace:   types.PROMETHEUS_NAMESPACE,
			ConstLabels: promGlobalLabels,
		}),
	}
}

// RegisterTasks registers the tasks with the tasks service.
func (s *Service) RegisterTasks(ctx context.Context) error {
	return nil
}

// StartService is a nil-op for this service as it does not require initialization.
func (s *Service) StartService(ctx context.Context, l *logger.Logger) error {
	return nil
}

// Init initializes the tasks service; it starts:
// - the task processing routine
// - the task maintenance routine
// - the task statistics update routine if metrics collection is enabled
func (s *Service) StartBackgroundWork(
	ctx context.Context, l *logger.Logger, errChan chan<- error,
) error {

	// Create a new context without the parent cancel because we prefer
	// to properly handle the context cancellation in the Shutdown method
	// that is called by our parent when the app is stopping.
	ctx, s.backgroundJobsCancelFunc = context.WithCancel(context.WithoutCancel(ctx))

	err := s.processTaskRoutine(ctx, l, errChan)
	if err != nil {
		return err
	}

	err = s.processTasksMaintenanceRoutine(ctx, l, errChan)
	if err != nil {
		return err
	}

	if s.options.CollectMetrics {
		return s.processTasksUpdateStatisticsRoutine(ctx, l, errChan)
	}

	return nil
}

// Shutdown shuts down the tasks service.
// Cancel was called on the context, so the workers should all stop processing
// tasks on their own. We just wait for the ones that are still processing.
func (s *Service) Shutdown(ctx context.Context) error {

	done := make(chan struct{})

	// Start a goroutine to wait for the workers to finish
	go func() {
		s.backgroundJobsWg.Wait()
		close(done)
	}()

	// Cancel the context to signal the workers to stop processing tasks
	s.backgroundJobsCancelFunc()

	// Wait for either the workers to finish or the context to be cancelled
	select {
	case <-ctx.Done():
		// If context is done (e.g., due to timeout), return the error
		return types.ErrShutdownTimeout
	case <-done:
		// If all workers finish successfully, return nil
		return nil
	}
}

// RegisterTasksService registers a tasks service and the tasks it handles.
func (s *Service) RegisterTasksService(tasksService types.ITasksService) {
	// Register the tasks managed by the service
	s.managedPkgs[tasksService.GetTaskServiceName()] = true
	for taskName, task := range tasksService.GetHandledTasks() {
		s.managedTasks[taskName] = task
	}
}

// GetTasks returns all tasks from the repository.
func (s *Service) GetTasks(
	ctx context.Context, l *logger.Logger, input types.InputGetAllTasksDTO,
) ([]tasks.ITask, error) {
	// Validate filters if present
	if !input.Filters.IsEmpty() {
		if err := input.Filters.Validate(); err != nil {
			return nil, errors.Wrap(err, "invalid filters")
		}
	}

	tasks, err := s.store.GetTasks(ctx, l, input.Filters)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// GetTask returns a task from the repository.
func (s *Service) GetTask(
	ctx context.Context, l *logger.Logger, input types.InputGetTaskDTO,
) (tasks.ITask, error) {
	task, err := s.store.GetTask(ctx, l, input.ID)
	if err != nil {
		if errors.Is(err, tasksrepo.ErrTaskNotFound) {
			return nil, types.ErrTaskNotFound
		}
		return nil, err
	}

	return task, nil
}

// CreateTask creates a new task in the repository.
func (s *Service) CreateTask(
	ctx context.Context, l *logger.Logger, input tasks.ITask,
) (tasks.ITask, error) {
	newTask := input

	// Generate an ID
	newTask.SetID(uuid.MakeV4())

	// Set creation and update datetime
	ctime := timeutil.Now()
	newTask.SetCreationDatetime(ctime)
	newTask.SetUpdateDatetime(ctime)

	// Set state
	newTask.SetState(tasks.TaskStatePending)

	// Save the task
	err := s.store.CreateTask(ctx, l, newTask)
	if err != nil {
		return nil, err
	}

	return newTask, nil
}

// CreateTaskIfNotAlreadyPlanned creates a new task in the repository
// if one of the same type is not already planned.
func (s *Service) CreateTaskIfNotAlreadyPlanned(
	ctx context.Context, l *logger.Logger, task tasks.ITask,
) (tasks.ITask, error) {
	// Create filters to check for existing pending tasks of the same type
	filters := filters.NewFilterSet().
		AddFilter("Type", filtertypes.OpEqual, task.GetType()).
		AddFilter("State", filtertypes.OpEqual, string(tasks.TaskStatePending))

	storedTasks, err := s.store.GetTasks(ctx, l, *filters)
	if err != nil {
		return nil, err
	}

	// If no task of the same type is already planned, create a new one
	if len(storedTasks) == 0 {
		return s.CreateTask(ctx, l, task)
	}

	// If a task of the same type is already planned, return it
	return storedTasks[0], nil
}

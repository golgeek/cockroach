// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package health

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/health"
	htasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/health/tasks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// IHealthService defines the health-specific interface.
type IHealthService interface {
	// Health-specific methods only
	RegisterInstance(ctx context.Context, l *logger.Logger, instanceID, hostname string) error
	IsInstanceHealthy(ctx context.Context, l *logger.Logger, instanceID string) (bool, error)
	GetHealthyInstances(ctx context.Context, l *logger.Logger) ([]health.InstanceInfo, error)
	GetInstanceID() string
	CleanupDeadInstances(ctx context.Context, l *logger.Logger, opts htasks.CleanupOptions) (int, error)
}

// Service implements the health service.
type Service struct {
	options    Options
	instanceID string
	hostname   string

	_repository  health.IHealthRepository
	_taskService stasks.IService

	// Background work management
	_backgroundJobsWg        *sync.WaitGroup
	backgroundJobsCtx        context.Context
	backgroundJobsCancelFunc context.CancelFunc
	shutdownOnce             sync.Once
}

// Options configures the health service.
type Options struct {
	HeartbeatInterval time.Duration // Default: 1s
	InstanceTimeout   time.Duration // Default: 3s
	CleanupInterval   time.Duration // Default: 60s
	CleanupRetention  time.Duration // Default: 24h
}

// NewService creates a new health service.
func NewService(
	repository health.IHealthRepository, taskService stasks.IService, opts Options,
) (*Service, error) {
	// Set defaults
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 1 * time.Second
	}
	if opts.InstanceTimeout == 0 {
		opts.InstanceTimeout = 3 * time.Second
	}
	if opts.CleanupInterval == 0 {
		opts.CleanupInterval = 60 * time.Second
	}
	if opts.CleanupRetention == 0 {
		opts.CleanupRetention = 24 * time.Hour
	}

	instanceID := generateInstanceID()
	hostname, _ := os.Hostname()

	return &Service{
		options:           opts,
		instanceID:        instanceID,
		hostname:          hostname,
		_repository:       repository,
		_taskService:      taskService,
		_backgroundJobsWg: &sync.WaitGroup{},
	}, nil
}

// generateInstanceID creates a unique instance ID.
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	id := uuid.MakeV4()
	return fmt.Sprintf("%s-%s", hostname, id.Short())
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
	healthLogger := l.With(
		slog.String("service", "health"),
		slog.String("instance_id", s.instanceID),
	)
	healthLogger.Info("starting health service")

	// Register this instance
	instance := health.InstanceInfo{
		InstanceID:    s.instanceID,
		Hostname:      s.hostname,
		StartedAt:     timeutil.Now(),
		LastHeartbeat: timeutil.Now(),
		Metadata:      make(map[string]string),
	}

	return s._repository.RegisterInstance(ctx, l, instance)
}

func (s *Service) StartBackgroundWork(
	ctx context.Context, l *logger.Logger, errChan chan<- error,
) error {
	s.backgroundJobsCtx, s.backgroundJobsCancelFunc = context.WithCancel(ctx)

	// Start heartbeat routine
	s.startHeartbeatRoutine(l)

	// Start cleanup task scheduling routine
	s.startCleanupScheduler(l)

	return nil
}

func (s *Service) Shutdown(ctx context.Context) error {
	var shutdownErr error
	s.shutdownOnce.Do(func() {
		// Cancel background work
		if s.backgroundJobsCancelFunc != nil {
			s.backgroundJobsCancelFunc()
		}

		// Wait for background work to finish with timeout
		done := make(chan struct{})
		go func() {
			s._backgroundJobsWg.Wait()
			close(done)
		}()

		select {
		case <-ctx.Done():
			shutdownErr = fmt.Errorf("shutdown timeout")
		case <-done:
			// Clean shutdown
		}
	})

	return shutdownErr
}

// Implements ITasksService interface

func (s *Service) GetTaskServiceName() string {
	return "health"
}

func (s *Service) GetHandledTasks() map[string]stasks.ITask {
	return map[string]stasks.ITask{
		string(htasks.HealthTaskCleanup): &htasks.TaskCleanup{
			Service: s,
			Options: htasks.CleanupOptions{
				RetentionPeriod: s.options.CleanupRetention,
				BatchSize:       100,
			},
		},
	}
}

// Background work routines

func (s *Service) startHeartbeatRoutine(l *logger.Logger) {
	heartbeatLogger := l.With(
		slog.String("service", "health"),
		slog.String("routine", "heartbeat"),
	)
	heartbeatLogger.Info("starting heartbeat routine",
		slog.Duration("interval", s.options.HeartbeatInterval),
	)

	s._backgroundJobsWg.Add(1)
	go func() {
		defer s._backgroundJobsWg.Done()

		ticker := time.NewTicker(s.options.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.backgroundJobsCtx.Done():
				heartbeatLogger.Info("stopping heartbeat routine")
				return

			case <-ticker.C:
				if err := s._repository.UpdateHeartbeat(
					s.backgroundJobsCtx, heartbeatLogger, s.instanceID,
				); err != nil {
					heartbeatLogger.Error("failed to send heartbeat",
						slog.Any("error", err),
						slog.String("instance_id", s.instanceID),
						slog.Duration("interval", s.options.HeartbeatInterval))
				}
			}
		}
	}()
}

func (s *Service) startCleanupScheduler(l *logger.Logger) {
	cleanupLogger := l.With(
		slog.String("service", "health"),
		slog.String("routine", "cleanup_scheduler"),
	)
	cleanupLogger.Info("starting cleanup scheduler",
		slog.Duration("interval", s.options.CleanupInterval))

	s._backgroundJobsWg.Add(1)
	go func() {
		defer s._backgroundJobsWg.Done()

		ticker := time.NewTicker(s.options.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-s.backgroundJobsCtx.Done():
				cleanupLogger.Info("stopping cleanup scheduler")
				return

			case <-ticker.C:
				if err := s.scheduleCleanupTaskIfNeeded(s.backgroundJobsCtx, cleanupLogger); err != nil {
					cleanupLogger.Error("failed to schedule cleanup task",
						slog.Any("error", err),
						slog.String("instance_id", s.instanceID),
						slog.Duration("cleanup_interval", s.options.CleanupInterval))
				}
			}
		}
	}()
}

func (s *Service) scheduleCleanupTaskIfNeeded(ctx context.Context, l *logger.Logger) error {
	if s._taskService == nil {
		return nil
	}

	// Check if cleanup task was recently created (within last cleanup interval)
	if s.hasRecentCleanupTask(ctx, l) {
		l.Debug("skipping cleanup task creation - recent task exists")
		return nil
	}

	l.Debug("creating cleanup task - no recent task found")
	return s.createCleanupTask(ctx, l)
}

// hasRecentCleanupTask checks if a cleanup task has been created recently
// (by another instance).
func (s *Service) hasRecentCleanupTask(ctx context.Context, l *logger.Logger) bool {
	if s._taskService == nil {
		return false
	}

	// Check for cleanup tasks created within the cleanup interval
	pendingTasks, err := s._taskService.GetTasks(
		ctx,
		l,
		stasks.InputGetAllTasksDTO{
			Filters: *filters.NewFilterSet().
				AddFilter("Type", filtertypes.OpEqual, string(htasks.HealthTaskCleanup)).
				AddFilter("State", filtertypes.OpNotEqual, string(tasks.TaskStateFailed)).
				AddFilter("CreationDatetime", filtertypes.OpGreater, timeutil.Now().Add(-s.options.CleanupInterval)),
		},
	)
	if err != nil {
		l.Error("failed to check for recent cleanup tasks",
			slog.Any("error", err),
			slog.String("instance_id", s.instanceID),
			slog.Duration("cleanup_interval", s.options.CleanupInterval))
		return false
	}

	return len(pendingTasks) > 0
}

func (s *Service) createCleanupTask(ctx context.Context, l *logger.Logger) error {
	task := &htasks.TaskCleanup{
		Service: s,
		Options: htasks.CleanupOptions{
			RetentionPeriod: s.options.CleanupRetention,
			BatchSize:       100,
		},
	}
	task.Type = string(htasks.HealthTaskCleanup)

	_, err := s._taskService.CreateTask(ctx, l, task)
	return err
}

// Implements IHealthService interface

func (s *Service) RegisterInstance(
	ctx context.Context, l *logger.Logger, instanceID, hostname string,
) error {
	instance := health.InstanceInfo{
		InstanceID:    instanceID,
		Hostname:      hostname,
		StartedAt:     timeutil.Now(),
		LastHeartbeat: timeutil.Now(),
		Metadata:      make(map[string]string),
	}

	return s._repository.RegisterInstance(ctx, l, instance)
}

func (s *Service) IsInstanceHealthy(
	ctx context.Context, l *logger.Logger, instanceID string,
) (bool, error) {
	return s._repository.IsInstanceHealthy(ctx, l, instanceID, s.options.InstanceTimeout)
}

func (s *Service) GetHealthyInstances(
	ctx context.Context, l *logger.Logger,
) ([]health.InstanceInfo, error) {
	return s._repository.GetHealthyInstances(ctx, l, s.options.InstanceTimeout)
}

func (s *Service) GetInstanceID() string {
	return s.instanceID
}

func (s *Service) CleanupDeadInstances(
	ctx context.Context, l *logger.Logger, opts htasks.CleanupOptions,
) (int, error) {
	return s._repository.CleanupDeadInstances(ctx, l, s.options.InstanceTimeout, opts.RetentionPeriod, opts.BatchSize)
}

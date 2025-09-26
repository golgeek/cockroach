// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
)

// HealthTaskType represents the type of health-related tasks.
type HealthTaskType string

const (
	// HealthTaskCleanup is the task type for cleaning up dead instances.
	HealthTaskCleanup HealthTaskType = "health_cleanup"
)

// IHealthService defines the interface that health tasks need from the health service.
type IHealthService interface {
	CleanupDeadInstances(ctx context.Context, l *logger.Logger, opts CleanupOptions) (int, error)
}

// CleanupOptions configures cleanup operations.
type CleanupOptions struct {
	RetentionPeriod time.Duration
	BatchSize       int
}

// TaskCleanup represents a task that cleans up dead instances.
type TaskCleanup struct {
	tasks.Task
	Service IHealthService
	Options CleanupOptions
}

// Process executes the cleanup task.
func (t *TaskCleanup) Process(ctx context.Context, l *logger.Logger, resChan chan<- error) {
	taskLogger := l.With(slog.String("task", "health_cleanup"))
	taskLogger.Info("starting health cleanup task")

	deletedCount, err := t.Service.CleanupDeadInstances(ctx, l, t.Options)
	if err != nil {
		taskLogger.Error("failed to cleanup dead instances", slog.Any("error", err))
		resChan <- err
		return
	}

	if deletedCount > 0 {
		taskLogger.Info("cleaned up dead instances", slog.Int("count", deletedCount))
	} else {
		taskLogger.Debug("no dead instances to cleanup")
	}

	resChan <- nil
}

// GetTimeout returns the timeout for the cleanup task.
func (t *TaskCleanup) GetTimeout() time.Duration {
	return 5 * time.Minute
}

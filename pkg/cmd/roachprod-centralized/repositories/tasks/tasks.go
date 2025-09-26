// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

var (
	// ErrTaskNotFound is returned when a task is not found.
	ErrTaskNotFound = fmt.Errorf("task not found")
)

// ITasksRepository defines the interface for task data persistence operations.
// This repository handles CRUD operations for background tasks, manages task state transitions,
// and provides querying capabilities with filtering support for task management systems.
type ITasksRepository interface {
	// GetTasks retrieves multiple tasks based on the provided filter criteria.
	GetTasks(context.Context, *logger.Logger, filtertypes.FilterSet) ([]tasks.ITask, error)
	// GetTask retrieves a single task by its unique identifier.
	GetTask(context.Context, *logger.Logger, uuid.UUID) (tasks.ITask, error)
	// CreateTask persists a new task to the repository.
	CreateTask(context.Context, *logger.Logger, tasks.ITask) error
	// UpdateState changes the state of an existing task (pending -> running -> done/failed).
	UpdateState(context.Context, *logger.Logger, uuid.UUID, tasks.TaskState) error
	// GetStatistics returns aggregated counts of tasks grouped by their current state.
	GetStatistics(context.Context, *logger.Logger) (Statistics, error)
	// PurgeTasks removes tasks in the specified state that are older than the given duration.
	// Returns the number of tasks that were purged.
	PurgeTasks(context.Context, *logger.Logger, time.Duration, tasks.TaskState) (int, error)
	// GetTasksForProcessing retrieves pending tasks and sends them to the provided channel
	// for worker processing. Uses the consumer ID for distributed coordination.
	GetTasksForProcessing(context.Context, *logger.Logger, chan<- tasks.ITask, uuid.UUID) error
}

// Statistics represents aggregated task counts grouped by task state.
// This provides a quick overview of the task system's current workload and health.
type Statistics map[tasks.TaskState]int

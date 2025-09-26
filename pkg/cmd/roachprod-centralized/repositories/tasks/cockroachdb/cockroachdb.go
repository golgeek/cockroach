// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cockroachdb

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	rtasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
)

// CRDBTasksRepo is a CockroachDB implementation of the tasks repository.
type CRDBTasksRepo struct {
	db *sql.DB

	// Polling strategy for GetTasksForProcessing
	pollingMutex    sync.Mutex
	baseInterval    time.Duration
	maxInterval     time.Duration
	currentInterval time.Duration

	// Task timeout for cleanup
	taskTimeout time.Duration
}

// Options for configuring the CockroachDB tasks repository.
type Options struct {
	// BasePollingInterval is the minimum polling interval (default: 100ms)
	BasePollingInterval time.Duration
	// MaxPollingInterval is the maximum polling interval (default: 5s)
	MaxPollingInterval time.Duration
	// TaskTimeout is how long a task can run before being considered stale (default: 10m)
	TaskTimeout time.Duration
}

// NewTasksRepository creates a new CockroachDB tasks repository.
func NewTasksRepository(db *sql.DB, opts Options) *CRDBTasksRepo {
	if opts.BasePollingInterval == 0 {
		opts.BasePollingInterval = 100 * time.Millisecond
	}
	if opts.MaxPollingInterval == 0 {
		opts.MaxPollingInterval = 5 * time.Second
	}
	if opts.TaskTimeout == 0 {
		opts.TaskTimeout = 10 * time.Minute
	}

	return &CRDBTasksRepo{
		db:              db,
		baseInterval:    opts.BasePollingInterval,
		maxInterval:     opts.MaxPollingInterval,
		currentInterval: opts.BasePollingInterval,
		taskTimeout:     opts.TaskTimeout,
	}
}

// GetTasks returns tasks based on the provided filters.
func (r *CRDBTasksRepo) GetTasks(
	ctx context.Context, l *logger.Logger, filterSet filtertypes.FilterSet,
) ([]tasks.ITask, error) {
	baseQuery := "SELECT id, type, state, consumer_id, creation_datetime, update_datetime FROM tasks"

	// Build WHERE clause using the filtering framework
	qb := filters.NewSQLQueryBuilder()
	whereClause, args, err := qb.BuildWhere(&filterSet)
	if err != nil {
		return nil, errors.Wrap(err, "failed to build query filters")
	}

	// Construct final query
	query := baseQuery
	if whereClause != "" {
		query += " " + whereClause
	}
	query += " ORDER BY creation_datetime"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query tasks")
	}
	defer rows.Close()

	var result []tasks.ITask
	for rows.Next() {
		task, err := r.scanTask(rows)
		if err != nil {
			return nil, errors.Wrap(err, "failed to scan task")
		}
		result = append(result, task)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	return result, nil
}

// GetTask returns a single task by ID.
func (r *CRDBTasksRepo) GetTask(
	ctx context.Context, l *logger.Logger, taskID uuid.UUID,
) (tasks.ITask, error) {
	query := "SELECT id, type, state, consumer_id, creation_datetime, update_datetime FROM tasks WHERE id = $1"

	row := r.db.QueryRowContext(ctx, query, taskID.String())
	task, err := r.scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, rtasks.ErrTaskNotFound
		}
		return nil, errors.Wrap(err, "failed to get task")
	}

	return task, nil
}

// CreateTask creates a new task in the database.
func (r *CRDBTasksRepo) CreateTask(ctx context.Context, l *logger.Logger, task tasks.ITask) error {
	query := `
		INSERT INTO tasks (id, type, state, consumer_id, creation_datetime, update_datetime)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	now := timeutil.Now()
	task.SetCreationDatetime(now)
	task.SetUpdateDatetime(now)

	var consumerID *string
	// Note: consumer_id is NULL for pending tasks, only set when claimed

	_, err := r.db.ExecContext(ctx, query,
		task.GetID().String(),
		task.GetType(),
		string(task.GetState()),
		consumerID, // NULL for new tasks
		task.GetCreationDatetime(),
		task.GetUpdateDatetime(),
	)

	if err != nil {
		return errors.Wrap(err, "failed to create task")
	}

	return nil
}

// UpdateState updates the state of a task.
func (r *CRDBTasksRepo) UpdateState(
	ctx context.Context, l *logger.Logger, taskID uuid.UUID, state tasks.TaskState,
) error {
	query := "UPDATE tasks SET state = $1, update_datetime = $2 WHERE id = $3"

	result, err := r.db.ExecContext(ctx, query, string(state), timeutil.Now(), taskID.String())
	if err != nil {
		return errors.Wrap(err, "failed to update task state")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "failed to get rows affected")
	}

	if rowsAffected == 0 {
		return rtasks.ErrTaskNotFound
	}

	return nil
}

// GetStatistics returns task statistics grouped by state.
func (r *CRDBTasksRepo) GetStatistics(
	ctx context.Context, l *logger.Logger,
) (rtasks.Statistics, error) {
	query := "SELECT state, COUNT(*) FROM tasks GROUP BY state"

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get statistics")
	}
	defer rows.Close()

	stats := make(rtasks.Statistics)
	for rows.Next() {
		var state string
		var count int
		if err := rows.Scan(&state, &count); err != nil {
			return nil, errors.Wrap(err, "failed to scan statistics")
		}
		stats[tasks.TaskState(state)] = count
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating statistics rows")
	}

	return stats, nil
}

// PurgeTasks deletes tasks that are in the specified state and older than the given duration.
func (r *CRDBTasksRepo) PurgeTasks(
	ctx context.Context, l *logger.Logger, olderThan time.Duration, state tasks.TaskState,
) (int, error) {
	query := "DELETE FROM tasks WHERE state = $1 AND update_datetime < $2"
	cutoff := timeutil.Now().Add(-olderThan)

	result, err := r.db.ExecContext(ctx, query, string(state), cutoff)
	if err != nil {
		return 0, errors.Wrap(err, "failed to purge tasks")
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, errors.Wrap(err, "failed to get purged count")
	}

	return int(rowsAffected), nil
}

// GetTasksForProcessing implements distributed task processing with intelligent polling.
func (r *CRDBTasksRepo) GetTasksForProcessing(
	ctx context.Context, l *logger.Logger, taskChan chan<- tasks.ITask, consumerID uuid.UUID,
) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Clean up stale tasks (distributed cleanup)
			r.cleanupStaleTasks(ctx, l, 2)

			// Try to claim a task
			task, found, err := r.claimNextTask(ctx, l, consumerID)
			if err != nil {
				return errors.Wrap(err, "failed to claim task")
			}

			if found {
				taskChan <- task
				r.updatePollingInterval(true) // Reset to fast polling
			} else {
				r.updatePollingInterval(false) // Increase polling interval
			}

			// Wait before next poll
			interval := r.getCurrentInterval()
			timer := time.NewTimer(interval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
}

// claimNextTask atomically claims the next pending task.
func (r *CRDBTasksRepo) claimNextTask(
	ctx context.Context, l *logger.Logger, consumerID uuid.UUID,
) (tasks.ITask, bool, error) {
	query := `
		UPDATE tasks 
		SET state = $1, consumer_id = $2, update_datetime = $3
		WHERE id = (
			SELECT id FROM tasks 
			WHERE state = $4 
			ORDER BY creation_datetime 
			LIMIT 1
		)
		RETURNING id, type, state, consumer_id, creation_datetime, update_datetime
	`

	now := timeutil.Now()
	row := r.db.QueryRowContext(ctx, query,
		string(tasks.TaskStateRunning),
		consumerID.String(),
		now,
		string(tasks.TaskStatePending),
	)

	task, err := r.scanTask(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil // No tasks available
		}
		return nil, false, errors.Wrap(err, "failed to claim task")
	}

	return task, true, nil
}

// cleanupStaleTasks removes tasks that have been running too long (distributed cleanup).
func (r *CRDBTasksRepo) cleanupStaleTasks(ctx context.Context, l *logger.Logger, limit int) {
	query := `
		UPDATE tasks 
		SET state = $1, consumer_id = NULL, update_datetime = $2
		WHERE id IN (
			SELECT id FROM tasks 
			WHERE state = $3 AND update_datetime < $4
			LIMIT $5
		)
	`

	cutoff := timeutil.Now().Add(-r.taskTimeout)
	_, err := r.db.ExecContext(ctx, query,
		string(tasks.TaskStatePending),
		timeutil.Now(),
		string(tasks.TaskStateRunning),
		cutoff,
		limit,
	)
	if err != nil {
		l.Warn("failed to cleanup stale tasks",
			slog.String("operation", "cleanupStaleTasks"),
			slog.Int("limit", limit),
			slog.Any("error", err))
	}
}

// updatePollingInterval adjusts the polling interval based on task availability.
func (r *CRDBTasksRepo) updatePollingInterval(foundTasks bool) {
	r.pollingMutex.Lock()
	defer r.pollingMutex.Unlock()

	if foundTasks {
		r.currentInterval = r.baseInterval
	} else {
		r.currentInterval = time.Duration(float64(r.currentInterval) * 1.5)
		if r.currentInterval > r.maxInterval {
			r.currentInterval = r.maxInterval
		}
	}
}

// getCurrentInterval returns the current polling interval (thread-safe).
func (r *CRDBTasksRepo) getCurrentInterval() time.Duration {
	r.pollingMutex.Lock()
	defer r.pollingMutex.Unlock()
	return r.currentInterval
}

// scanTask scans a database row into a Task struct.
func (r *CRDBTasksRepo) scanTask(
	scanner interface {
		Scan(dest ...interface{}) error
	},
) (tasks.ITask, error) {
	var id, taskType, state string
	var consumerID *string
	var creationTime, updateTime time.Time

	err := scanner.Scan(&id, &taskType, &state, &consumerID, &creationTime, &updateTime)
	if err != nil {
		return nil, err
	}

	taskID, err := uuid.FromString(id)
	if err != nil {
		return nil, errors.Wrap(err, "invalid task ID")
	}

	task := &tasks.Task{
		ID:               taskID,
		Type:             taskType,
		State:            tasks.TaskState(state),
		CreationDatetime: creationTime,
		UpdateDatetime:   updateTime,
	}

	return task, nil
}

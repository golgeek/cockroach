// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	slogmulti "github.com/samber/slog-multi"
)

const (
	PurgeTaskOlderThan       = 2 * time.Hour
	PurgeTasksInterval       = 10 * time.Minute
	StatisticsUpdateInterval = 30 * time.Second
)

// ProcessTaskRoutine will process tasks in a loop
func (s *Service) ProcessTaskRoutine(ctx context.Context, l *slog.Logger) (err error) {

	l.Info("Starting tasks processing routine")

	taskChan := make(chan tasks.ITask)

	// Get tasks for processing from repository
	go func() {
		err = s._store.GetTasksForProcessing(
			ctx,
			l,
			taskChan,
			s._consumerID,
		)
		if err != nil {
			l.Error("Unable to get tasks for processing", "error", err)
			return
		}
	}()

	go func() {
		// Process tasks
		timerPurge := time.Tick(PurgeTasksInterval)
		timerStats := time.Tick(StatisticsUpdateInterval)

		for {
			select {
			case <-ctx.Done():
				l.Info("Stopping task processing routine")
				return

			case <-timerPurge:

				l.Info("Purging tasks in done state from the database")
				err = s._store.PurgeTasks(ctx, PurgeTaskOlderThan)
				if err != nil {
					l.Error("Unable to purge tasks", slog.Any("error", err))
				}

			case <-timerStats:

				l.Info("Getting tasks statistics from the database")
				stats, err := s._store.GetStatistics(ctx)
				if err != nil {
					l.Error("Unable to get tasks statistics", slog.Any("error", err))
					continue
				}

				l.Debug("Updating tasks statistics", slog.Any("stats", stats))

				// Pening
				qtPending := float64(stats[tasks.TaskStatePending])
				s._metrics._totalTasksPending.Set(qtPending)

				// Running
				qtRunning := float64(stats[tasks.TaskStateRunning])
				s._metrics._totalTasksRunning.Set(qtRunning)

				// Done
				qtDone := float64(stats[tasks.TaskStateDone])
				s._metrics._totalTasksDone.Set(qtDone)

				// Failed
				qtFailed := float64(stats[tasks.TaskStateFailed])
				s._metrics._totalTasksFailed.Set(qtFailed)
			}
		}
	}()

	for i := 0; i < s.options.Workers; i++ {
		go func() {

			for {
				select {
				case <-ctx.Done():
					l.Info("Stopping task processing routine")
					return

				case task := <-taskChan:

					l.Debug("task to process received", slog.Any("task", task))

					taskLogs := new(strings.Builder)

					taskLogger := slog.New(
						slogmulti.Fanout(
							// We force the level to debug to log everything in the stored task logs
							slog.NewTextHandler(taskLogs, &slog.HandlerOptions{Level: slog.LevelDebug}),
							l.With(slog.String("task_id", task.GetID().String())).Handler(),
						),
					)

					errStatus := s.markTaskAs(ctx, task.GetID(), tasks.TaskStateRunning)
					if errStatus != nil {
						l.Error(
							"Failed to update task status",
							slog.Any("task_id", task.GetID()),
							slog.String("status", string(tasks.TaskStateRunning)),
							slog.Any("error", errStatus),
						)
					}

					err := s.processTask(
						ctx,
						taskLogger,
						task,
					)
					if err != nil {
						taskLogger.Error(
							"Unable to process task",
							slog.Any("task_id", task.GetID()),
							slog.Any("error", err),
						)

						errFailedStatus := s.markTaskAs(ctx, task.GetID(), tasks.TaskStateFailed)
						if errFailedStatus != nil {
							l.Error(
								"Failed to update task status",
								slog.Any("task_id", task.GetID()),
								slog.String("status", string(tasks.TaskStateFailed)),
								slog.Any("error", errFailedStatus),
							)
						}
						continue
					}

					taskLogger.Info(
						"Task processed successfully",
						slog.Any("task_id", task.GetID()),
					)

					errDoneStatus := s.markTaskAs(ctx, task.GetID(), tasks.TaskStateDone)
					if errDoneStatus != nil {
						l.Error(
							"Failed to update task status",
							slog.Any("task_id", task.GetID()),
							slog.String("status", string(tasks.TaskStateDone)),
							slog.Any("error", errDoneStatus),
						)
					}
				}
			}
		}()
	}

	return nil
}

func (s *Service) processTask(ctx context.Context, l *slog.Logger, task tasks.ITask) (err error) {

	switch {
	case s._managedTasks[task.GetType()] != nil:
		err = s._managedTasks[task.GetType()].Process(
			ctx,
			l,
		)
	default:
		err = ErrTaskTypeNotManaged
	}

	// Increment the counter of tasks processed
	s._metrics._processedTasksProcessed.Inc()

	return
}

func (s *Service) markTaskAs(
	ctx context.Context,
	id uuid.UUID,
	status tasks.TaskState,
) (err error) {
	return s._store.UpdateState(ctx, id, status)
}

// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package memory

import (
	"context"
	"log/slog"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	rtasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"golang.org/x/exp/maps"
)

type MemTasksRepo struct {
	tasks                    map[uuid.UUID]tasks.ITask
	lock                     syncutil.Mutex
	tasksQueuedForProcessing chan uuid.UUID
}

func NewTasksRepository() *MemTasksRepo {
	return &MemTasksRepo{
		tasks:                    make(map[uuid.UUID]tasks.ITask),
		tasksQueuedForProcessing: make(chan uuid.UUID),
	}
}

func (s *MemTasksRepo) GetTasks(ctx context.Context) ([]tasks.ITask, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	return maps.Values(s.tasks), nil
}

func (s *MemTasksRepo) GetTask(ctx context.Context, taskID uuid.UUID) (tasks.ITask, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	if t, ok := s.tasks[taskID]; !ok {
		return nil, rtasks.ErrTaskNotFound
	} else {
		return t, nil
	}
}

func (s *MemTasksRepo) CreateTask(ctx context.Context, task tasks.ITask) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.tasks[task.GetID()] = task

	// If the task is pending, we queue it for processing
	if task.GetState() == tasks.TaskStatePending {
		s.tasksQueuedForProcessing <- task.GetID()
	}

	return nil
}

func (s *MemTasksRepo) UpdateState(ctx context.Context, taskID uuid.UUID, state tasks.TaskState) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	if _, ok := s.tasks[taskID]; !ok {
		return rtasks.ErrTaskNotFound
	}
	s.tasks[taskID].SetState(state)
	return nil
}

func (s *MemTasksRepo) GetStatistics(ctx context.Context) (rtasks.Statistics, error) {

	s.lock.Lock()
	defer s.lock.Unlock()

	stats := make(rtasks.Statistics)
	for _, task := range s.tasks {
		stats[task.GetState()]++
	}

	return stats, nil

}

func (s *MemTasksRepo) PurgeTasks(ctx context.Context, olderThan time.Duration) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, task := range s.tasks {

		// We only delete tasks that have been processed
		if task.GetState() == tasks.TaskStatePending || task.GetState() == tasks.TaskStateRunning {
			continue
		}

		if time.Since(task.GetUpdateDatetime()) > olderThan {
			delete(s.tasks, task.GetID())
		}
	}

	return nil
}

func (s *MemTasksRepo) GetTasksForProcessing(
	ctx context.Context, l *slog.Logger, taskChan chan<- tasks.ITask, consumerID uuid.UUID,
) error {

	s.enqueueExistingTasksForProcessing(l)

	for {
		select {
		case <-ctx.Done():
			return nil
		case taskID := <-s.tasksQueuedForProcessing:
			l.Info("Task queued for processing, sending to taskChan", "taskID", taskID)
			taskChan <- s.tasks[taskID]
		}
	}

}

func (s *MemTasksRepo) enqueueExistingTasksForProcessing(l *slog.Logger) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, task := range s.tasks {
		if task.GetState() == tasks.TaskStatePending {
			l.Info("Task is pending, queueing for processing", "taskID", task.GetID())
			s.tasksQueuedForProcessing <- task.GetID()
		}
	}
}

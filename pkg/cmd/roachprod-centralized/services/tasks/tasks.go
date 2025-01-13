// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	tasksrepo "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	PROMETHEUS_NAMESPACE = "roachprod_centralized"
)

var (
	ErrTaskTypeNotManaged = fmt.Errorf("task type not managed")
	ErrTaskNotFound       = fmt.Errorf("task not found")
)

// IService is the interface for the tasks service.
type IService interface {
	GetTasks(context.Context, *slog.Logger, InputGetAllTasksDTO) *TasksDTO
	GetTask(context.Context, *slog.Logger, InputGetTaskDTO) *TaskDTO
	CreateTask(context.Context, *slog.Logger, tasks.ITask) *TaskDTO
	CreateTaskIfNotAlreadyPlanned(context.Context, *slog.Logger, tasks.ITask) *TaskDTO
	RegisterTasksService(ITasksService) error
}

type ITasksService interface {
	GetTaskServiceName() string
	GetTasks() (tasks map[string]ITask)
}

type ITask interface {
	Process(context.Context, *slog.Logger) error
}

// Service is the implementation of the tasks service.
type Service struct {
	options Options

	_managedPkgs  map[string]bool
	_managedTasks map[string]ITask

	_consumerID     uuid.UUID
	_store          tasksrepo.ITasksRepository
	_tasksProcessed prometheus.Counter
	_metrics        struct {
		_processedTasksProcessed prometheus.Counter
		_totalTasksPending       prometheus.Gauge
		_totalTasksRunning       prometheus.Gauge
		_totalTasksDone          prometheus.Gauge
		_totalTasksFailed        prometheus.Gauge
	}
}

type Options struct {
	Workers int
}

func NewService(
	ctx context.Context, l *slog.Logger, store tasksrepo.ITasksRepository, options Options,
) *Service {

	consumerID := uuid.MakeV4()
	promGlobalLabels := prometheus.Labels{
		"consumer": consumerID.String(),
	}

	return &Service{
		options:       options,
		_consumerID:   consumerID,
		_store:        store,
		_managedPkgs:  make(map[string]bool),
		_managedTasks: make(map[string]ITask),
		_metrics: struct {
			_processedTasksProcessed prometheus.Counter
			_totalTasksPending       prometheus.Gauge
			_totalTasksRunning       prometheus.Gauge
			_totalTasksDone          prometheus.Gauge
			_totalTasksFailed        prometheus.Gauge
		}{
			// Prometheus counter of tasks processed
			_processedTasksProcessed: promauto.NewCounter(prometheus.CounterOpts{
				Name:        "processed_total",
				Help:        "The total number of processed tasks",
				Namespace:   PROMETHEUS_NAMESPACE,
				ConstLabels: promGlobalLabels,
			}),
			// Prometheus gauges of pending tasks in the database
			_totalTasksPending: promauto.NewGauge(prometheus.GaugeOpts{
				Name:        string(tasks.TaskStatePending),
				Help:        "The number of tasks to be processed in the database",
				Namespace:   PROMETHEUS_NAMESPACE,
				ConstLabels: promGlobalLabels,
			}),
			// Prometheus gauges of running tasks in the database
			_totalTasksRunning: promauto.NewGauge(prometheus.GaugeOpts{
				Name:        string(tasks.TaskStateRunning),
				Help:        "The number of tasks being processed in the database",
				Namespace:   PROMETHEUS_NAMESPACE,
				ConstLabels: promGlobalLabels,
			}),
			// Prometheus gauges of done tasks in the database
			_totalTasksDone: promauto.NewGauge(prometheus.GaugeOpts{
				Name:        string(tasks.TaskStateDone),
				Help:        "The number of tasks processed successfully in the database",
				Namespace:   PROMETHEUS_NAMESPACE,
				ConstLabels: promGlobalLabels,
			}),
			// Prometheus gauges of failed tasks in the database
			_totalTasksFailed: promauto.NewGauge(prometheus.GaugeOpts{
				Name:        string(tasks.TaskStateFailed),
				Help:        "The number of tasks that failed to be processed in the database",
				Namespace:   PROMETHEUS_NAMESPACE,
				ConstLabels: promGlobalLabels,
			}),
		},
	}
}

func (s *Service) RegisterTasksService(tasksService ITasksService) error {
	// Register the tasks managed by the service
	s._managedPkgs[tasksService.GetTaskServiceName()] = true
	for taskName, task := range tasksService.GetTasks() {
		s._managedTasks[taskName] = task
	}
	return nil
}

func (s *Service) GetTasks(
	ctx context.Context, l *slog.Logger, input InputGetAllTasksDTO,
) *TasksDTO {
	tasks, err := s._store.GetTasks(ctx)
	if err != nil {
		return &TasksDTO{
			PrivateError: err,
		}
	}

	return &TasksDTO{
		Data: tasks,
	}
}

func (s *Service) GetTask(ctx context.Context, l *slog.Logger, input InputGetTaskDTO) *TaskDTO {
	task, err := s._store.GetTask(ctx, input.ID)
	if err != nil {
		if errors.Is(err, tasksrepo.ErrTaskNotFound) {
			return &TaskDTO{
				PublicError: ErrTaskNotFound,
			}
		}
		return &TaskDTO{
			PrivateError: err,
		}
	}

	return &TaskDTO{
		Data: task,
	}
}

func (s *Service) CreateTask(
	ctx context.Context, l *slog.Logger, input tasks.ITask,
) *TaskDTO {
	newTask := input

	// Generate an ID
	newTask.SetID(uuid.MakeV4())

	// Set creation and update datetime
	ctime := time.Now()
	newTask.SetCreationDatetime(ctime)
	newTask.SetUpdateDatetime(ctime)

	// Set state
	newTask.SetState(tasks.TaskStatePending)

	// Save the task
	err := s._store.CreateTask(ctx, newTask)
	if err != nil {
		return &TaskDTO{
			PrivateError: err,
		}
	}

	return &TaskDTO{
		Data: newTask,
	}
}

func (s *Service) CreateTaskIfNotAlreadyPlanned(
	ctx context.Context, l *slog.Logger, task tasks.ITask,
) *TaskDTO {
	storedTasks, err := s._store.GetTasks(ctx)
	if err != nil {
		return &TaskDTO{
			PrivateError: err,
		}
	}
	for _, t := range storedTasks {
		if t.GetState() != tasks.TaskStatePending {
			continue
		}
		if t.GetType() == task.GetType() {
			return &TaskDTO{
				Data: t,
			}
		}
	}
	return s.CreateTask(ctx, l, task)
}

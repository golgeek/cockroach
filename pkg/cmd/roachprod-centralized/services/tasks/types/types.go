package types

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
	filtertypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/filters/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

const (
	PROMETHEUS_NAMESPACE = "roachprod"
)

var (
	// ErrTaskNotFound is the error returned when a task is not found.
	ErrTaskNotFound = utils.NewPublicError(fmt.Errorf("task not found"))
	// ErrTaskTypeNotManaged is the error returned for unmanaged task types.
	ErrTaskTypeNotManaged = fmt.Errorf("task type not managed")
	// ErrTaskTimeout is the error returned when a task processing times out.
	ErrTaskTimeout = fmt.Errorf("task processing timeout")
	// ErrShutdownTimeout is the error returned when the service shutdown times out.
	ErrShutdownTimeout = fmt.Errorf("service shutdown timeout")
	// ErrMetricsCollectionDisabled is returned when metrics collection is disabled
	// and a metrics-related operation is attempted.
	ErrMetricsCollectionDisabled = fmt.Errorf("metrics collection is disabled")
)

// IService defines the interface for the tasks service, which manages background task processing
// and provides CRUD operations for tasks. This service handles task lifecycle management,
// worker orchestration, and integration with other services that need to schedule work.
type IService interface {
	// GetTasks retrieves multiple tasks based on the provided filters and pagination parameters.
	GetTasks(context.Context, *logger.Logger, InputGetAllTasksDTO) ([]tasks.ITask, error)
	// GetTask retrieves a single task by its ID.
	GetTask(context.Context, *logger.Logger, InputGetTaskDTO) (tasks.ITask, error)
	// CreateTask creates a new task and stores it in the repository for processing.
	CreateTask(context.Context, *logger.Logger, tasks.ITask) (tasks.ITask, error)
	// CreateTaskIfNotAlreadyPlanned creates a new task only if a similar task isn't already pending.
	// This prevents duplicate work from being scheduled.
	CreateTaskIfNotAlreadyPlanned(context.Context, *logger.Logger, tasks.ITask) (tasks.ITask, error)
	// RegisterTasksService registers a service that provides background tasks for processing.
	RegisterTasksService(ITasksService)
}

// InputGetAllTasksDTO is the data transfer object to get all tasks.
type InputGetAllTasksDTO struct {
	Filters filtertypes.FilterSet `json:"filters,omitempty"`
}

// InputGetTaskDTO is the data transfer object to get a task.
type InputGetTaskDTO struct {
	ID uuid.UUID `json:"id" binding:"required"`
}

// ITasksService is the interface for a service that handles tasks.
type ITasksService interface {
	GetTaskServiceName() string
	GetHandledTasks() (tasks map[string]ITask)
}

// ITask is the interface for a task that can be processed by the service.
type ITask interface {
	Process(context.Context, *logger.Logger, chan<- error)
}

// ITaskWithTimeout is an interface for tasks that have a timeout.
type ITaskWithTimeout interface {
	GetTimeout() time.Duration
}

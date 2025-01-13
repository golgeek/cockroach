package tasks

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

type TaskState string

const (
	TaskStatePending TaskState = "pending"
	TaskStateRunning TaskState = "running"
	TaskStateDone    TaskState = "done"
	TaskStateFailed  TaskState = "failed"
)

type ITask interface {
	GetID() uuid.UUID
	SetID(uuid.UUID)
	GetType() string
	GetCreationDatetime() time.Time
	SetCreationDatetime(time.Time)
	GetUpdateDatetime() time.Time
	SetUpdateDatetime(time.Time)
	GetState() TaskState
	SetState(state TaskState)
}

type Task struct {
	ID               uuid.UUID
	Type             string
	State            TaskState
	CreationDatetime time.Time
	UpdateDatetime   time.Time
}

func (t *Task) GetID() uuid.UUID {
	return t.ID
}
func (t *Task) SetID(id uuid.UUID) {
	t.ID = id
}

func (t *Task) GetType() string {
	return t.Type
}

func (t *Task) GetState() TaskState {
	return t.State
}

func (t *Task) SetState(state TaskState) {
	t.State = state
}

func (t *Task) GetCreationDatetime() time.Time {
	return t.CreationDatetime
}

func (t *Task) SetCreationDatetime(ctime time.Time) {
	t.CreationDatetime = ctime
}

func (t *Task) GetUpdateDatetime() time.Time {
	return t.UpdateDatetime
}

func (t *Task) SetUpdateDatetime(utime time.Time) {
	t.UpdateDatetime = utime
}

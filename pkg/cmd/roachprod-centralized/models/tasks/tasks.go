// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// TaskState represents the current execution state of a background task.
// Tasks progress through these states during their lifecycle: pending -> running -> done/failed.
type TaskState string

const (
	// TaskStatePending indicates a task is queued and waiting to be processed by a worker.
	TaskStatePending TaskState = "pending"
	// TaskStateRunning indicates a task is currently being executed by a worker.
	TaskStateRunning TaskState = "running"
	// TaskStateDone indicates a task completed successfully.
	TaskStateDone TaskState = "done"
	// TaskStateFailed indicates a task encountered an error and could not complete.
	TaskStateFailed TaskState = "failed"
)

// ITask defines the interface for background tasks in the roachprod-centralized system.
// All task implementations must provide these methods for lifecycle management and persistence.
// This interface enables polymorphic handling of different task types within the task processing system.
type ITask interface {
	// GetID returns the unique identifier for this task.
	GetID() uuid.UUID
	// SetID assigns a unique identifier to this task.
	SetID(uuid.UUID)
	// GetType returns a string identifying the task type (e.g., "cluster_sync", "dns_update").
	GetType() string
	// GetCreationDatetime returns when this task was originally created.
	GetCreationDatetime() time.Time
	// SetCreationDatetime sets the task creation timestamp.
	SetCreationDatetime(time.Time)
	// GetUpdateDatetime returns when this task was last modified.
	GetUpdateDatetime() time.Time
	// SetUpdateDatetime sets the task last-modified timestamp.
	SetUpdateDatetime(time.Time)
	// GetState returns the current execution state of this task.
	GetState() TaskState
	// SetState updates the task's execution state.
	SetState(state TaskState)
}

// Task provides a basic implementation of the ITask interface.
// Specific task types should embed this struct and implement their own processing logic.
type Task struct {
	ID               uuid.UUID
	Type             string
	State            TaskState
	CreationDatetime time.Time
	UpdateDatetime   time.Time
}

// GetID returns the task ID.
func (t *Task) GetID() uuid.UUID {
	return t.ID
}

// SetID sets the task ID.
func (t *Task) SetID(id uuid.UUID) {
	t.ID = id
}

// GetType returns the task type.
func (t *Task) GetType() string {
	return t.Type
}

// GetState returns the task state.
func (t *Task) GetState() TaskState {
	return t.State
}

// SetState sets the task state.
func (t *Task) SetState(state TaskState) {
	t.State = state
}

// GetCreationDatetime returns the task creation datetime.
func (t *Task) GetCreationDatetime() time.Time {
	return t.CreationDatetime
}

// SetCreationDatetime sets the task creation datetime.
func (t *Task) SetCreationDatetime(ctime time.Time) {
	t.CreationDatetime = ctime
}

// GetUpdateDatetime returns the task update datetime.
func (t *Task) GetUpdateDatetime() time.Time {
	return t.UpdateDatetime
}

// SetUpdateDatetime sets the task update datetime.
func (t *Task) SetUpdateDatetime(utime time.Time) {
	t.UpdateDatetime = utime
}

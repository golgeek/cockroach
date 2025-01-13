// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// TasksDTO is the data transfer object for a Tasks list.
type TasksDTO struct {
	Data         []tasks.ITask
	PublicError  error
	PrivateError error
}

// TaskDTO is the data transfer object for a task.
type TaskDTO struct {
	Data         tasks.ITask
	PublicError  error
	PrivateError error
}

// InputGetAllDTO is the data transfer object to get all tasks.
type InputGetAllTasksDTO struct {
	Type  string `json:"type" binding:"omitempty,alphanum"`
	State string `json:"state" binding:"omitempty,alpha"`
}

// InputGetTaskDTO is the data transfer object to get a task.
type InputGetTaskDTO struct {
	ID uuid.UUID `json:"id" binding:"required"`
}

// InputCreateTaskDTO is the data transfer object to create a new task.
type InputCreateTaskDTO struct {
	tasks.ITask
}

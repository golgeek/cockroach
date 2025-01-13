// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"errors"
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks"
)

// InputGetAllDTO is input data transfer object that handles requests parameters
// for the tasks.GetAll() controller.
type InputGetAllDTO struct {
	Type  string `json:"type" binding:"omitempty,alphanum"`
	State string `json:"state" binding:"omitempty,alpha"`
}

// ToServiceInputGetAllDTO converts the InputGetAllDTO data transfer object
// to a tasks' service InputGetAllDTO.
func (dto *InputGetAllDTO) ToServiceInputGetAllDTO() stasks.InputGetAllTasksDTO {
	return stasks.InputGetAllTasksDTO{
		Type:  dto.Type,
		State: dto.State,
	}
}

type TasksErrorDTO struct {
	PublicError  error `json:"public_error"`
	PrivateError error `json:"private_error"`
}

// GetPublicError returns the public error from the TasksErrorDTO.
func (dto *TasksErrorDTO) GetPublicError() error {
	return dto.PublicError
}

// GetPrivateError returns the private error from the TasksErrorDTO.
func (dto *TasksErrorDTO) GetPrivateError() error {
	return dto.PrivateError
}

// GetAssociatedStatusCode returns the status code associated
// with the TasksDTO.
func (dto *TasksErrorDTO) GetAssociatedStatusCode() (int, error) {

	if dto.GetPrivateError() != nil {
		return http.StatusInternalServerError, nil
	}

	err := dto.GetPublicError()
	switch {
	case err == nil:
		return http.StatusOK, nil
	case errors.Is(err, stasks.ErrTaskNotFound):
		return http.StatusNotFound, nil
	default:
		return http.StatusInternalServerError, nil
	}
}

// TasksDTO is the output data transfer object for the tasks controller.
type TasksDTO struct {
	TasksErrorDTO
	Data []tasks.ITask `json:"data,omitempty"`
}

// GetData returns the data from the TasksDTO
func (dto *TasksDTO) GetData() any {
	return dto.Data
}

// FromServiceDTO converts a tasks service DTO to a TasksDTO.
func (dto *TasksDTO) FromServiceDTO(tasksDto *stasks.TasksDTO) *TasksDTO {
	dto.Data = tasksDto.Data
	dto.PublicError = tasksDto.PublicError
	dto.PrivateError = tasksDto.PrivateError
	return dto
}

// TaskDTO is the output data transfer object for the tasks controller.
type TaskDTO struct {
	TasksErrorDTO
	Data tasks.ITask `json:"data,omitempty"`
}

// GetData returns the data from the TaskDTO
func (dto *TaskDTO) GetData() any {
	return dto.Data
}

// FromServiceDTO converts a tasks service DTO to a TaskDTO.
func (dto *TaskDTO) FromServiceDTO(taskDto *stasks.TaskDTO) *TaskDTO {
	dto.Data = taskDto.Data
	dto.PublicError = taskDto.PublicError
	dto.PrivateError = taskDto.PrivateError
	return dto
}

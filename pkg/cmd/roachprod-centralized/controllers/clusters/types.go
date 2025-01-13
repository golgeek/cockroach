// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"errors"
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters"
	"github.com/cockroachdb/cockroach/pkg/roachprod/cloud"
)

// InputGetAllDTO is input data transfer object that handles requests parameters
// for the clusters.GetAll() controller.
type InputGetAllDTO struct {
	Username string `form:"username" binding:"omitempty,alphanum"`
}

// ToServiceInputGetAllDTO converts the InputGetAllDTO data transfer object
// to a clusters' service InputGetAllDTO.
func (dto *InputGetAllDTO) ToServiceInputGetAllDTO() clusters.InputGetAllClustersDTO {
	return clusters.InputGetAllClustersDTO{
		Username: dto.Username,
	}
}

// InputCreateClusterDTO is input data transfer object that handles requests
// parameters for the clusters.Create() controller.
type InputCreateClusterDTO struct {
	cloud.Cluster
}

// ToServiceInputCreateClusterDTO converts the InputCreateClusterDTO data transfer object
// to a clusters' service InputCreateClusterDTO.
func (dto *InputCreateClusterDTO) ToServiceInputCreateClusterDTO() clusters.InputCreateClusterDTO {
	return clusters.InputCreateClusterDTO{
		Cluster: dto.Cluster,
	}
}

// InputUpdateClusterDTO is input data transfer object that handles requests
// parameters for the clusters.Create() controller.
type InputUpdateClusterDTO struct {
	cloud.Cluster
}

// ToServiceInputCreateClusterDTO converts the InputUpdateClusterDTO data transfer object
// to a clusters' service InputCreateClusterDTO.
func (dto *InputUpdateClusterDTO) ToServiceInputUpdateClusterDTO() clusters.InputUpdateClusterDTO {
	return clusters.InputUpdateClusterDTO{
		Cluster: dto.Cluster,
	}
}

type ClustersErrorDTO struct {
	PublicError  error `json:"public_error"`
	PrivateError error `json:"private_error"`
}

// GetPublicError returns the public error from the ClustersDTO.
func (dto *ClustersErrorDTO) GetPublicError() error {
	return dto.PublicError
}

// GetPrivateError returns the private error from the ClustersDTO.
func (dto *ClustersErrorDTO) GetPrivateError() error {
	return dto.PrivateError
}

// GetAssociatedStatusCode returns the status code associated
// with the ClustersDTO.
func (dto *ClustersErrorDTO) GetAssociatedStatusCode() (int, error) {

	if dto.GetPrivateError() != nil {
		return http.StatusInternalServerError, nil
	}

	err := dto.GetPublicError()
	switch {
	case err == nil:
		return http.StatusOK, nil
	case errors.Is(err, clusters.ErrClusterNotFound):
		return http.StatusNotFound, nil
	case errors.Is(err, clusters.ErrClusterAlreadyExists):
		return http.StatusConflict, nil
	default:
		return http.StatusInternalServerError, nil
	}
}

// ClustersDTO is the output data transfer object for the clusters controller.
type ClustersDTO struct {
	ClustersErrorDTO
	Data *cloud.Clusters `json:"data,omitempty"`
}

// GetData returns the data from the ClustersDTO
func (dto *ClustersDTO) GetData() any {
	return dto.Data
}

// FromServiceDTO converts a clusters service DTO to a ClustersDTO.
func (dto *ClustersDTO) FromServiceDTO(clustersDto *clusters.ClustersDTO) *ClustersDTO {
	dto.Data = clustersDto.Data
	dto.PublicError = clustersDto.PublicError
	dto.PrivateError = clustersDto.PrivateError
	return dto
}

// ClustersDTO is the output data transfer object for the clusters controller.
type ClusterDTO struct {
	ClustersErrorDTO
	Data *cloud.Cluster `json:"data,omitempty"`
}

// GetData returns the data from the ClustersDTO
func (dto *ClusterDTO) GetData() any {
	return dto.Data
}

// FromServiceDTO converts a clusters service DTO to a ClustersDTO.
func (dto *ClusterDTO) FromServiceDTO(clusterDto *clusters.ClusterDTO) *ClusterDTO {
	dto.Data = clusterDto.Data
	dto.PublicError = clusterDto.PublicError
	dto.PrivateError = clusterDto.PrivateError
	return dto
}

type TaskDTO struct {
	ClustersErrorDTO
	Data tasks.ITask `json:"data,omitempty"`
}

// GetData returns the data from the ClustersDTO
func (dto *TaskDTO) GetData() any {
	return dto.Data
}

// FromServiceDTO converts a clusters service DTO to a ClustersDTO.
func (dto *TaskDTO) FromServiceDTO(clusterDto *clusters.TaskDTO) *TaskDTO {
	dto.Data = clusterDto.Data
	dto.PublicError = clusterDto.PublicError
	dto.PrivateError = clusterDto.PrivateError
	return dto
}

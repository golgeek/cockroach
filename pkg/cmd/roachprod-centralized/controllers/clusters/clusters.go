// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package clusters

import (
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters"
	"github.com/gin-gonic/gin"
)

// Controller is the clusters controller.
type Controller struct {
	*controllers.Controller
	service  clusters.IService
	handlers []controllers.IControllerHandler
}

// NewController creates a new clusters controller.
func NewController(service clusters.IService) *Controller {
	ctrl := &Controller{
		Controller: &controllers.Controller{},
		service:    service,
	}
	ctrl.handlers = []controllers.IControllerHandler{
		&controllers.ControllerHandler{
			Method: "GET",
			Path:   "/clusters",
			Func:   ctrl.GetAll,
		},
		&controllers.ControllerHandler{
			Method: "GET",
			Path:   "/clusters/:name",
			Func:   ctrl.GetOne,
		},
		&controllers.ControllerHandler{
			Method: "POST",
			Path:   "/clusters",
			Func:   ctrl.Create,
		},
		&controllers.ControllerHandler{
			Method: "PUT",
			Path:   "/clusters/:name",
			Func:   ctrl.Update,
		},
		&controllers.ControllerHandler{
			Method: "POST",
			Path:   "/clusters/sync",
			Func:   ctrl.Sync,
		},
	}
	return ctrl
}

// GetHandlers returns the controller's handlers, as required
// by the controllers.IController interface.
func (ctrl *Controller) GetHandlers() []controllers.IControllerHandler {
	return ctrl.handlers
}

// GetAll returns all clusters from the clusters service.
func (ctrl *Controller) GetAll(c *gin.Context) {

	l := controllers.ReqLogger(c)

	var inputDTO InputGetAllDTO
	if err := c.ShouldBindQuery(&inputDTO); err != nil {
		ctrl.Render(c, l, &controllers.BadRequestResultDTO{Error: err})
		return
	}

	clustersDto := ctrl.service.GetAllClusters(
		c.Request.Context(),
		l,
		inputDTO.ToServiceInputGetAllDTO(),
	)

	ctrl.Render(c, l, (&ClustersDTO{}).FromServiceDTO(clustersDto))
}

// GetOne returns a cluster from the clusters service.
func (ctrl *Controller) GetOne(c *gin.Context) {
	l := controllers.ReqLogger(c)

	clusterDto := ctrl.service.GetCluster(
		c.Request.Context(),
		l,
		clusters.InputGetClusterDTO{
			Name: c.Param("name"),
		},
	)

	ctrl.Render(c, l, (&ClusterDTO{}).FromServiceDTO(clusterDto))
}

// Create creates a cluster in the clusters service.
func (ctrl *Controller) Create(c *gin.Context) {

	l := controllers.ReqLogger(c)

	var inputDTO InputCreateClusterDTO
	if err := c.ShouldBindJSON(&inputDTO); err != nil {
		ctrl.Render(c, l, &controllers.BadRequestResultDTO{Error: err})
		return
	}

	clusterDto := ctrl.service.CreateCluster(
		c.Request.Context(),
		l,
		inputDTO.ToServiceInputCreateClusterDTO(),
	)

	ctrl.Render(c, l, (&ClusterDTO{}).FromServiceDTO(clusterDto))
}

// Update creates a cluster in the clusters service.
func (ctrl *Controller) Update(c *gin.Context) {

	l := controllers.ReqLogger(c)

	var inputDTO InputUpdateClusterDTO
	if err := c.ShouldBindJSON(&inputDTO); err != nil {
		ctrl.Render(c, l, &controllers.BadRequestResultDTO{Error: err})
		return
	}

	// Make sure we force the name to be the one in the URL
	inputDTO.Name = c.Param("name")

	clusterDto := ctrl.service.UpdateCluster(
		c.Request.Context(),
		l,
		inputDTO.ToServiceInputUpdateClusterDTO(),
	)

	ctrl.Render(c, l, (&ClusterDTO{}).FromServiceDTO(clusterDto))
}

// Delete deletes a cluster in the clusters service.
func (ctrl *Controller) Delete(c *gin.Context) {

	l := controllers.ReqLogger(c)

	clusterDto := ctrl.service.DeleteCluster(
		c.Request.Context(),
		l,
		clusters.InputDeleteClusterDTO{
			Name: c.Param("name"),
		},
	)

	ctrl.Render(c, l, (&ClusterDTO{}).FromServiceDTO(clusterDto))
}

// Sync triggers a clusters sync to the store.
func (ctrl *Controller) Sync(c *gin.Context) {

	l := controllers.ReqLogger(c)

	clustersTaskDto := ctrl.service.SyncClouds(
		c.Request.Context(),
		l,
	)

	ctrl.Render(c, l, (&TaskDTO{}).FromServiceDTO(clustersTaskDto))
}

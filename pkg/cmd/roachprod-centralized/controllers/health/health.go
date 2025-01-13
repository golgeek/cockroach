// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package health

import (
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers"
	"github.com/gin-gonic/gin"
)

type Controller struct {
	*controllers.Controller
	handlers []controllers.IControllerHandler
}

func NewController() (ctrl *Controller) {
	ctrl = &Controller{
		Controller: &controllers.Controller{},
	}
	ctrl.handlers = []controllers.IControllerHandler{
		&controllers.ControllerHandler{
			Method: "GET",
			Path:   "/ping",
			Func:   ctrl.Ping,
		},
	}
	return
}

func (ctrl *Controller) GetHandlers() []controllers.IControllerHandler {
	return ctrl.handlers
}

// Ping returns pong
func (ctrl *Controller) Ping(c *gin.Context) {
	ctrl.Render(c, controllers.ReqLogger(c), &HealthDTO{})
}

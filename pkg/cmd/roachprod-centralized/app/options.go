// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package app

import (
	"log/slog"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers"
)

type IAppOption interface {
	apply(app *App)
}

type WithApiControllerOption struct {
	value controllers.IController
}

func (o WithApiControllerOption) apply(a *App) {
	if a.api.controllers == nil {
		a.api.controllers = make([]controllers.IController, 0)
	}
	a.api.controllers = append(a.api.controllers, o.value)
}

func WithApiController(value controllers.IController) IAppOption {
	return WithApiControllerOption{value: value}
}

type WithApiBaseURLOption struct {
	value string
}

func (o WithApiBaseURLOption) apply(a *App) {
	a.api.baseURL = o.value
}

func WithApiBaseURL(baseURL string) IAppOption {
	return WithApiBaseURLOption{value: baseURL}
}

type WithApiMetricsOption struct {
	value bool
}

func (o WithApiMetricsOption) apply(a *App) {
	a.api.metrics = o.value
}

func WithApiMetrics(metrics bool) IAppOption {
	return WithApiMetricsOption{value: metrics}
}

type WithApiPortOption struct {
	value int
}

func (o WithApiPortOption) apply(a *App) {
	a.api.port = o.value
}

func WithApiPort(port int) IAppOption {
	return WithApiPortOption{value: port}
}

type WithApiMetricsPortOption struct {
	value int
}

func (o WithApiMetricsPortOption) apply(a *App) {
	a.api.metricsPort = o.value
}

func WithApiMetricsPort(port int) IAppOption {
	return WithApiMetricsPortOption{value: port}
}

type WithLoggerOption struct {
	value *slog.Logger
}

func (o WithLoggerOption) apply(a *App) {
	a.logger = o.value
}

func WithLogger(logger *slog.Logger) IAppOption {
	return WithLoggerOption{value: logger}
}

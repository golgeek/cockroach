// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/app"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/clusters"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/health"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/tasks"
	cstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters/memory"
	tstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks/memory"
	sclusters "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils"
)

func main() {

	appCtx := context.Background()
	appLogger := &utils.Logger{
		Logger: slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: slog.LevelInfo,
			}),
		),
	}

	taskService := stasks.NewService(
		tstore.NewTasksRepository(),
		stasks.Options{
			Workers: 1,
		},
	)

	clustersService := sclusters.NewService(
		cstore.NewClustersRepository(),
		taskService,
		sclusters.Options{
			PeriodicRefreshEnabled: true,
		},
	)

	appLogger.Info("Starting roachprod-centralized app")
	app, err := app.NewApp(
		app.WithLogger(appLogger),
		app.WithService(clustersService),
		app.WithService(taskService),
		app.WithApiPort(8080),
		app.WithApiMetricsPort(8081),
		app.WithApiController(health.NewController()),
		app.WithApiController(clusters.NewController(clustersService)),
		app.WithApiController(tasks.NewController(taskService)),
	)
	if err != nil {
		os.Exit(1)
	}

	// Start the server
	err = app.Start(appCtx)
	if err != nil {
		os.Exit(1)
	}
}

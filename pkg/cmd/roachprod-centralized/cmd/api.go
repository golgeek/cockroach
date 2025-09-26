// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cmd

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/app"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/config"
	clusterscontroller "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/clusters"
	healthcontroller "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/health"
	publicdns "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/public-dns"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/controllers/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Start the API server",
	Long: `Start the roachprod-centralized API server which provides HTTP endpoints
for managing roachprod clusters, tasks, and related operations.`,
	RunE: runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)

	// Add all configuration flags to the API command so they show up in help
	err := config.AddFlagsToFlagSet(apiCmd.Flags())
	if err != nil {
		panic(fmt.Sprintf("Error adding flags to API command: %v", err))
	}
}

func runAPI(cmd *cobra.Command, args []string) error {
	cfg, err := config.InitConfigWithFlagSet(cmd.Flags())
	if err != nil {
		return errors.Wrap(err, "error initializing config")
	}

	l := logger.NewLogger(cfg.Log.Level)

	// Create all services using the factory
	services, err := app.NewServicesFromConfig(cfg, l)
	if err != nil {
		return errors.Wrap(err, "error creating services")
	}

	// Create the application instance with the configured services and controllers.
	// The application instance is responsible for starting the API server and
	// handling requests.
	application, err := app.NewApp(
		app.WithLogger(l),
		app.WithApiPort(cfg.Api.Port),
		app.WithApiBaseURL(cfg.Api.BasePath),
		app.WithApiMetrics(cfg.Api.Metrics.Enabled),
		app.WithApiMetricsPort(cfg.Api.Metrics.Port),
		app.WithApiAuthenticationDisabled(cfg.Api.Authentication.Disabled),
		app.WithApiAuthenticationHeader(cfg.Api.Authentication.JWT.Header),
		app.WithApiAuthenticationAudience(cfg.Api.Authentication.JWT.Audience),
		app.WithServices(services),
		app.WithApiController(healthcontroller.NewController()),
		app.WithApiController(clusterscontroller.NewController(services.Clusters)),
		app.WithApiController(publicdns.NewController(services.DNS)),
		app.WithApiController(tasks.NewController(services.Task)),
	)
	if err != nil {
		return errors.Wrap(err, "error creating application")
	}

	// Start the application.
	// This will start the API server and all services.
	err = application.Start(context.Background())
	if err != nil {
		return errors.Wrap(err, "error starting application")
	}

	return nil
}

// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package app

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	configtypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/config/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters"
	ccrdbstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters/cockroachdb"
	cmemstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/clusters/memory"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/health"
	hcrdbstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/health/cockroachdb"
	hmemstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/health/memory"
	rtasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks"
	tcrdbstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks/cockroachdb"
	tmemstore "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/repositories/tasks/memory"
	sclusters "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters"
	shealth "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/health"
	spublicdns "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/public-dns"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/database"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/errors"
	_ "github.com/lib/pq"
)

// Services holds all the application services
type Services struct {
	Task     *stasks.Service
	Health   *shealth.Service
	Clusters *sclusters.Service
	DNS      *spublicdns.Service
}

// NewServicesFromConfig creates and initializes all services from configuration
func NewServicesFromConfig(cfg *configtypes.Config, l *logger.Logger) (*Services, error) {
	appCtx := context.Background()

	// Initialize repositories based on configuration.
	var clustersRepository clusters.IClustersRepository
	var tasksRepository rtasks.ITasksRepository
	var healthRepository health.IHealthRepository

	switch strings.ToLower(cfg.Database.Type) {
	case "cockroachdb":
		// Create database connection
		db, err := database.NewConnection(appCtx, database.ConnectionConfig{
			URL:         cfg.Database.URL,
			MaxConns:    cfg.Database.MaxConns,
			MaxIdleTime: cfg.Database.MaxIdleTime,
		})
		if err != nil {
			l.Error("failed to connect to database",
				slog.Any("error", err),
				slog.String("database_type", cfg.Database.Type),
				slog.Int("max_conns", cfg.Database.MaxConns))
			return nil, errors.Wrap(err, "error connecting to database")
		}

		// Run database migrations for tasks repository
		if err := database.RunMigrationsForRepository(appCtx, l, db, "tasks", tcrdbstore.GetTasksMigrations()); err != nil {
			l.Error("failed to run tasks migrations",
				slog.Any("error", err),
				slog.String("repository", "tasks"))
			return nil, errors.Wrap(err, "error running tasks migrations")
		}

		// Run database migrations for health repository
		if err := database.RunMigrationsForRepository(appCtx, l, db, "health", hcrdbstore.GetHealthMigrations()); err != nil {
			l.Error("failed to run health migrations",
				slog.Any("error", err),
				slog.String("repository", "health"))
			return nil, errors.Wrap(err, "error running health migrations")
		}

		// Run database migrations for clusters repository
		if err := database.RunMigrationsForRepository(appCtx, l, db, "clusters", ccrdbstore.GetClustersMigrations()); err != nil {
			l.Error("failed to run clusters migrations",
				slog.Any("error", err),
				slog.String("repository", "clusters"))
			return nil, errors.Wrap(err, "error running clusters migrations")
		}

		tasksRepository = tcrdbstore.NewTasksRepository(db, tcrdbstore.Options{
			BasePollingInterval: 100000000,    // 100ms in nanoseconds
			MaxPollingInterval:  5000000000,   // 5s in nanoseconds
			TaskTimeout:         600000000000, // 10m in nanoseconds
		})

		healthRepository = hcrdbstore.NewHealthRepository(db)
		clustersRepository = ccrdbstore.NewClustersRepository(db)

	case "memory", "":
		tasksRepository = tmemstore.NewTasksRepository()
		healthRepository = hmemstore.NewHealthRepository()
		clustersRepository = cmemstore.NewClustersRepository()

	default:
		return nil, fmt.Errorf("unsupported database type: %s", cfg.Database.Type)
	}

	// Create the task service.
	// This service is responsible for managing tasks, which are used to perform
	// operations like syncing clusters or DNS.
	// The service is used by other services to schedule and perform background tasks.
	taskService := stasks.NewService(
		tasksRepository,
		stasks.Options{
			Workers: cfg.Tasks.Workers,
		},
	)

	// Create the health service.
	// This service is responsible for tracking instance health and cleanup.
	healthService, err := shealth.NewService(
		healthRepository,
		taskService,
		shealth.Options{
			HeartbeatInterval: time.Second,
			InstanceTimeout:   time.Second * 3,
			CleanupInterval:   time.Minute,
			CleanupRetention:  time.Hour * 24,
		},
	)
	if err != nil {
		l.Error("failed to create health service",
			slog.Any("error", err),
			slog.String("database_type", cfg.Database.Type))
		return nil, errors.Wrap(err, "error creating health service")
	}

	// Create the clusters service.
	// This service is responsible for managing clusters.
	clustersService, err := sclusters.NewService(
		clustersRepository,
		taskService,
		healthService,
		sclusters.Options{
			PeriodicRefreshEnabled: true,
			CloudProviders:         cfg.CloudProviders,
		},
	)
	if err != nil {
		l.Error("failed to create clusters service",
			slog.Any("error", err),
			slog.Int("cloud_providers", len(cfg.CloudProviders)),
			slog.String("database_type", cfg.Database.Type))
		return nil, errors.Wrap(err, "error creating clusters service")
	}

	// Create the DNS service.
	// This service is responsible for managing public DNS records for clusters.
	dnsService, err := spublicdns.NewService(
		clustersService,
		taskService,
		spublicdns.Options{
			DNSProviders: cfg.DNSProviders,
		},
	)
	if err != nil {
		l.Error("failed to create DNS service",
			slog.Any("error", err),
			slog.Int("dns_providers", len(cfg.DNSProviders)),
			slog.String("database_type", cfg.Database.Type))
		return nil, errors.Wrap(err, "error creating DNS service")
	}

	return &Services{
		Task:     taskService,
		Health:   healthService,
		Clusters: clustersService,
		DNS:      dnsService,
	}, nil
}

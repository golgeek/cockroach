// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package publicdns

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	configtypes "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/config/types"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	sclustermodels "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/models"
	dnstasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/public-dns/tasks"
	stasks "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/tasks/types"
	logger "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	rplogger "github.com/cockroachdb/cockroach/pkg/roachprod/logger"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm"
	"github.com/cockroachdb/cockroach/pkg/roachprod/vm/gce"
	"github.com/cockroachdb/errors"
)

const (
	ROACHPROD_DIR = ".roachprod"
)

// Service is the implementation of the clusters service.
type Service struct {
	options Options

	infraProviders []gce.InfraProvider
	dnsProviders   []vm.DNSProvider

	_taskService     stasks.IService
	_clustersService sclustermodels.IService
	_syncing         bool
}

// Options contains the options for the clusters service.
type Options struct {
	DNSProviders []configtypes.CloudProvider
}

// NewService creates a new clusters service.
func NewService(
	clustersService sclustermodels.IService, tasksService stasks.IService, options Options,
) (*Service, error) {

	service := &Service{
		_taskService:     tasksService,
		_clustersService: clustersService,
		options:          options,
	}

	for _, cp := range options.DNSProviders {

		switch strings.ToLower(cp.Type) {
		// Only GCE is supported for now.
		case gce.ProviderName:
			provider, err := gce.NewProvider(cp.GCE.ToOptions()...)
			if err != nil {
				return nil, errors.Wrapf(err, "unable to create GCE cloud provider")
			}

			service.dnsProviders = append(service.dnsProviders, provider)
			service.infraProviders = append(service.infraProviders, provider)

		default:
			return nil, errors.Newf("unknown DNS provider type: %s", cp.Type)
		}

	}

	return service, nil
}

// RegisterTasks registers the DNS tasks with the tasks service.
func (s *Service) RegisterTasks(ctx context.Context) error {
	// Register the DNS tasks with the tasks service.
	if s._taskService != nil {
		s._taskService.RegisterTasksService(s)
	}
	return nil
}

// StartService initializes the service and registers it with the tasks service.
func (s *Service) StartService(ctx context.Context, l *logger.Logger) error {

	// SyncDNS writes the DNS records to the .roachprod directory in the user's
	// home directory. We make sure the directory exists before starting the service.
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrapf(err, "failed to get the user's home directory")
	}

	err = os.MkdirAll(fmt.Sprintf("%s/%s", homeDir, ROACHPROD_DIR), 0755)
	if err != nil {
		return errors.Wrapf(err, "failed to create service directory %s", ROACHPROD_DIR)
	}
	return nil
}

// StartBackgroundWork is a nil-op for this service as it does not require background work.
func (s *Service) StartBackgroundWork(
	ctx context.Context, l *logger.Logger, errChan chan<- error,
) error {
	return nil
}

// Shutdown is a nil-op as the service does not require shutdown logic.
func (s *Service) Shutdown(ctx context.Context) error {
	return nil
}

// GetTaskServiceName returns the name of the task service.
func (s *Service) GetTaskServiceName() string {
	return string(dnstasks.PublicDNSTask)
}

// GetHandledTasks returns a map of task types to task implementations that are
// handled by this service.
func (s *Service) GetHandledTasks() map[string]stasks.ITask {
	return map[string]stasks.ITask{
		string(dnstasks.PublicDNSTaskSync): &dnstasks.TaskSync{
			Service: s,
		},
	}
}

// SyncDNS creates a task to sync the DNS.
func (s *Service) SyncDNS(ctx context.Context, l *logger.Logger) (tasks.ITask, error) {

	// Create a task to sync the clouds.
	task := &dnstasks.TaskSync{}
	task.Type = string(dnstasks.PublicDNSTaskSync)

	// Save the task.
	return s._taskService.CreateTaskIfNotAlreadyPlanned(ctx, l, task)
}

// sync contains the logic to sync the clusters and store them
// in the repository.
func (s *Service) Sync(ctx context.Context, l *logger.Logger) error {

	s._syncing = true
	defer func(s *Service) {
		l.Info("syncing DNS across all providers is over, releasing syncing flag")
		s._syncing = false
	}(s)

	crlLogger, err := (&rplogger.Config{Stdout: l, Stderr: l}).NewLogger("")
	if err != nil {
		return err
	}

	l.Info("syncing DNS across all providers")

	for _, p := range s.infraProviders {

		vms, err := s._clustersService.GetAllDNSZoneVMs(ctx, l, p.DNSDomain())
		if err != nil {
			l.Error("failed to get DNS zone VMs",
				slog.Any("error", err),
				slog.String("dns_zone", p.DNSDomain()))
			return err
		}

		l.Info("syncing DNS for zone",
			slog.String("zone", p.DNSDomain()),
			slog.Int("vms", len(vms)))

		if err := p.SyncDNS(crlLogger, vms); err != nil {
			l.Error("failed to sync DNS",
				slog.Any("error", err),
				slog.String("dns_zone", p.DNSDomain()),
				slog.Int("vm_count", len(vms)))
			return err
		}
	}
	return nil
}

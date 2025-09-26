// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	dnsmodels "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/public-dns/models"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
)

// PublicDNSTaskType is the type of the task.
type PublicDNSTaskType string

const (
	// PublicDNSTaskSync is the task type for synchronizing the public DNS.
	PublicDNSTask     PublicDNSTaskType = "public_dns"
	PublicDNSTaskSync PublicDNSTaskType = "public_dns_sync"
)

// TaskSync is the task that synchronizes the DNS.
type TaskSync struct {
	tasks.Task
	Service dnsmodels.IService
}

// Process will synchronize the DNS.
func (t *TaskSync) Process(ctx context.Context, l *logger.Logger, resChan chan<- error) {
	resChan <- t.Service.Sync(ctx, l)
}

func (t *TaskSync) GetTimeout() time.Duration {
	return time.Minute
}

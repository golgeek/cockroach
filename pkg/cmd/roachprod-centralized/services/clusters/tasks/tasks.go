// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tasks

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/services/clusters/models"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
)

// CustersTaskType is the type of the task.
type ClustersTaskType string

const (
	// ClustersTaskSync is the task type for synchronizing the clusters.
	ClustersTaskSync ClustersTaskType = "clusters_sync"
)

// TaskSync is the task that synchronizes the clusters.
type TaskSync struct {
	tasks.Task
	Service models.IService
}

// Process will synchronize the clusters.
func (t *TaskSync) Process(ctx context.Context, l *logger.Logger, resChan chan<- error) {
	_, err := t.Service.Sync(ctx, l)
	if err != nil {
		resChan <- err
		return
	}
	resChan <- nil
}

func (t *TaskSync) GetTimeout() time.Duration {
	return time.Minute
}

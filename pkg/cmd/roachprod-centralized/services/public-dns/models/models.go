// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package models

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/models/tasks"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
)

// IService is the interface for the clusters service.
type IService interface {
	SyncDNS(context.Context, *logger.Logger) (tasks.ITask, error)
	Sync(ctx context.Context, l *logger.Logger) error
}

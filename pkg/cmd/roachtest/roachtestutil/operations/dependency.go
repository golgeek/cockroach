// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package operations

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/cluster"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/option"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/registry"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachtest/roachtestflags"
	"github.com/cockroachdb/cockroach/pkg/roachprod/logger"
)

// CheckDependencies returns true if an operation with the provided spec
// can be run on the specified cluster.
func CheckDependencies(
	ctx context.Context, c cluster.Cluster, l *logger.Logger, spec *registry.OperationSpec,
) (ok bool, err error) {
	for _, dep := range spec.Dependencies {
		switch dep {
		case registry.OperationRequiresNodes:
			if len(c.All()) == 0 {
				return false, nil
			}

		case registry.OperationRequiresPopulatedDatabase:
			ok, err := checkPopulatedDatabase(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		case registry.OperationRequiresZeroUnavailableRanges:
			ok, err := checkZeroUnavailableRanges(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		case registry.OperationRequiresZeroUnderreplicatedRanges:
			ok, err := checkZeroUnderreplicatedRanges(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		case registry.OperationRequiresLDRJobRunning:
			ok, err := checkLDRJobRunning(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		case registry.OperationRequiresRunningBackupJob:
			ok, err := checkBackupJobRunning(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		case registry.OperationRequiresRunningRestoreJob:
			ok, err := checkRestoreJobRunning(ctx, c, l)
			if err != nil || !ok {
				return ok, err
			}

		default:
			panic(fmt.Sprintf("unknown operation dependency %d", dep))
		}
	}
	return true, nil
}

func checkPopulatedDatabase(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName(roachtestflags.VirtualCluster))
	defer conn.Close()

	dbsCount, err := conn.QueryContext(ctx, "SELECT count(database_name) FROM [SHOW DATABASES] WHERE database_name NOT IN ('postgres', 'system')")
	if err != nil {
		return false, err
	}
	dbsCount.Next()
	var count int
	if err := dbsCount.Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func checkZeroUnavailableRanges(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName("system"))
	defer conn.Close()

	rangesCur, err := conn.QueryContext(ctx, "SELECT sum(unavailable_ranges) FROM system.replication_stats")
	if err != nil {
		return false, err
	}
	rangesCur.Next()
	var count int
	if err := rangesCur.Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func checkZeroUnderreplicatedRanges(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName("system"))
	defer conn.Close()

	rangesCur, err := conn.QueryContext(ctx, "SELECT sum(under_replicated_ranges) FROM system.replication_stats")
	if err != nil {
		return false, err
	}
	rangesCur.Next()
	var count int
	if err := rangesCur.Scan(&count); err != nil {
		return false, err
	}
	return count == 0, nil
}

func checkLDRJobRunning(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName("system"))
	defer conn.Close()

	jobsCur, err := conn.QueryContext(ctx, "(WITH x AS (SHOW JOBS) SELECT job_id FROM x WHERE job_type = 'LOGICAL REPLICATION' AND status = 'running' limit 1)")
	if err != nil {
		return false, err
	}
	jobsCur.Next()
	var jobId string
	_ = jobsCur.Scan(&jobId)
	return jobId != "", nil
}

func checkBackupJobRunning(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName("system"))
	defer conn.Close()

	jobsCur, err := conn.QueryContext(ctx,
		"(WITH x AS (SHOW JOBS) SELECT job_id FROM x WHERE job_type = 'BACKUP' AND status = 'running' LIMIT 1)")
	if err != nil {
		return false, err
	}
	jobsCur.Next()
	var jobId string
	_ = jobsCur.Scan(&jobId)
	return jobId != "", nil
}

func checkRestoreJobRunning(
	ctx context.Context, c cluster.Cluster, l *logger.Logger,
) (ok bool, _ error) {
	conn := c.Conn(ctx, l, 1, option.VirtualClusterName("system"))
	defer conn.Close()

	jobsCur, err := conn.QueryContext(ctx,
		"WITH x AS (SHOW JOBS) SELECT job_id FROM x WHERE job_type = 'RESTORE' AND status = 'running' LIMIT 1)")
	if err != nil {
		return false, err
	}
	jobsCur.Next()
	var jobId string
	_ = jobsCur.Scan(&jobId)
	return jobId != "", nil
}

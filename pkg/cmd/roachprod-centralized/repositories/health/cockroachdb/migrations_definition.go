// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cockroachdb

import "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/database"

// GetHealthMigrations returns all migrations for the health repository.
func GetHealthMigrations() []database.Migration {
	return []database.Migration{
		{
			Version:     1,
			Description: "Create instance health table for heartbeat tracking",
			SQL: `
-- Instance health tracking table
-- Health is determined by heartbeat timing, not status column
CREATE TABLE IF NOT EXISTS instance_health (
    instance_id VARCHAR(255) PRIMARY KEY,
    hostname VARCHAR(255) NOT NULL,
    started_at TIMESTAMPTZ NOT NULL,
    last_heartbeat TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    metadata JSONB
);

-- Index for efficient health checks and cleanup queries
CREATE INDEX IF NOT EXISTS idx_instance_health_heartbeat ON instance_health(last_heartbeat);
			`,
		},
	}
}

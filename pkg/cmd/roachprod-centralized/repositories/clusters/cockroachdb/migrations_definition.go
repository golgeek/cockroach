// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cockroachdb

import "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/database"

// GetClustersMigrations returns the database migrations for the clusters repository.
func GetClustersMigrations() []database.Migration {
	return []database.Migration{
		{
			Version:     1,
			Description: "Create clusters table",
			SQL: `
				CREATE TABLE IF NOT EXISTS clusters (
					name VARCHAR(255) PRIMARY KEY,
					data JSONB NOT NULL,
					created_at TIMESTAMPTZ DEFAULT NOW(),
					updated_at TIMESTAMPTZ DEFAULT NOW()
				);
			`,
		},
		{
			Version:     2,
			Description: "Create cluster sync state table",
			SQL: `
				CREATE TABLE IF NOT EXISTS cluster_sync_state (
					id INT PRIMARY KEY DEFAULT 1,
					in_progress BOOLEAN NOT NULL DEFAULT FALSE,
					instance_id VARCHAR(255),
					started_at TIMESTAMPTZ,
					CONSTRAINT single_sync_state CHECK (id = 1)
				);
			`,
		},
		{
			Version:     3,
			Description: "Create cluster operations queue table",
			SQL: `
				CREATE TABLE IF NOT EXISTS cluster_operations (
					id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
					operation_type VARCHAR(50) NOT NULL,
					cluster_name VARCHAR(255) NOT NULL,
					cluster_data JSONB NOT NULL,
					created_at TIMESTAMPTZ DEFAULT NOW(),
					INDEX idx_cluster_operations_created_at (created_at)
				);
			`,
		},
	}
}

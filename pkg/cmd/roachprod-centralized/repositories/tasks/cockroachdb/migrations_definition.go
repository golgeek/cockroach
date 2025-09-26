// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cockroachdb

import "github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/database"

// GetTasksMigrations returns all migrations for the tasks repository.
func GetTasksMigrations() []database.Migration {
	return []database.Migration{
		{
			Version:     1,
			Description: "Initial tasks table and indexes",
			SQL: `
-- Tasks table for storing task information
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    type VARCHAR(255) NOT NULL,
    state VARCHAR(50) NOT NULL CHECK (state IN ('pending', 'running', 'done', 'failed')),
    consumer_id UUID NULL, -- NULL for pending tasks, set when claimed by a worker
    creation_datetime TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    update_datetime TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_tasks_state_creation ON tasks (state, creation_datetime);
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks (type);
CREATE INDEX IF NOT EXISTS idx_tasks_consumer_id ON tasks (consumer_id) WHERE consumer_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_update_datetime ON tasks (update_datetime);

-- Index for cleanup queries (finding stale running tasks)
CREATE INDEX IF NOT EXISTS idx_tasks_stale_cleanup ON tasks (state, update_datetime) WHERE state = 'running';
			`,
		},
		// Future migrations go here...
		// {
		//     Version:     2,
		//     Description: "Add payload column to tasks",
		//     SQL:         "ALTER TABLE tasks ADD COLUMN IF NOT EXISTS payload JSONB",
		// },
	}
}

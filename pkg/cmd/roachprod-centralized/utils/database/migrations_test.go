// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package database

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	_ "github.com/lib/pq"
)

func TestMigrationRunner_Basic(t *testing.T) {
	// This test requires a running CockroachDB instance
	db, err := NewConnection(context.Background(), ConnectionConfig{
		URL: "postgresql://root@localhost:26257/defaultdb?sslmode=disable",
	})
	if err != nil {
		t.Skipf("CockroachDB not available: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test migrations
	testMigrations := []Migration{
		{
			Version:     1,
			Description: "Create test table",
			SQL:         "CREATE TABLE IF NOT EXISTS test_table (id UUID PRIMARY KEY DEFAULT gen_random_uuid())",
		},
	}

	// Clean up
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS test_table")

	// Run migrations
	err = RunMigrationsForRepository(ctx, logger.DefaultLogger, db, "test", testMigrations)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify migration table exists
	var exists bool
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'schema_migrations'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check migration table: %v", err)
	}
	if !exists {
		t.Error("Migration table was not created")
	}

	// Verify test table exists
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'test_table'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check test table: %v", err)
	}
	if !exists {
		t.Error("Test table was not created")
	}

	// Clean up
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS test_table")
}

// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package cockroachdb

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/database"
	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	_ "github.com/lib/pq"
)

func TestMigrationRunner_RunMigrations(t *testing.T) {
	// This test requires a running CockroachDB instance
	// Skip if not available
	db, err := database.NewConnection(context.Background(), database.ConnectionConfig{
		URL: "postgresql://root@localhost:26257/defaultdb?sslmode=disable",
	})
	if err != nil {
		t.Skipf("CockroachDB not available: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Clean up any existing tables
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS tasks")

	// Run migrations using shared database package
	err = database.RunMigrationsForRepository(ctx, logger.DefaultLogger, db, "tasks_test", GetTasksMigrations())
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

	// Verify tasks table exists
	err = db.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_name = 'tasks'
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("Failed to check tasks table: %v", err)
	}
	if !exists {
		t.Error("Tasks table was not created")
	}

	// Verify migration was recorded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check migration record: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}

	// Run migrations again - should be idempotent
	err = database.RunMigrationsForRepository(ctx, logger.DefaultLogger, db, "tasks_test", GetTasksMigrations())
	if err != nil {
		t.Fatalf("Failed to run migrations second time: %v", err)
	}

	// Verify still only one migration record
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check migration record after second run: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration record after second run, got %d", count)
	}

	// Clean up
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS tasks")
}

func TestMigrationRunner_ConcurrentMigrations(t *testing.T) {
	// This test requires a running CockroachDB instance
	db, err := database.NewConnection(context.Background(), database.ConnectionConfig{
		URL: "postgresql://root@localhost:26257/defaultdb?sslmode=disable",
	})
	if err != nil {
		t.Skipf("CockroachDB not available: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Clean up any existing tables
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS tasks")

	// Run multiple migration runners concurrently
	errChan := make(chan error, 3)

	for i := 0; i < 3; i++ {
		go func() {
			errChan <- database.RunMigrationsForRepository(ctx, logger.DefaultLogger, db, "tasks_concurrent_test", GetTasksMigrations())
		}()
	}

	// Wait for all to complete
	var errors []error
	for i := 0; i < 3; i++ {
		if err := <-errChan; err != nil {
			errors = append(errors, err)
		}
	}

	// Should have no errors
	if len(errors) > 0 {
		t.Fatalf("Concurrent migrations failed: %v", errors)
	}

	// Verify only one migration record exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to check migration record: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 migration record, got %d", count)
	}

	// Clean up
	_, _ = db.Exec("DROP TABLE IF EXISTS schema_migrations")
	_, _ = db.Exec("DROP TABLE IF EXISTS tasks")
}

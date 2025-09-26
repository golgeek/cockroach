// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

// Migration represents a single database migration.
type Migration struct {
	Version     int    // Unique version number (e.g., 1, 2, 3...)
	Description string // Human-readable description
	SQL         string // SQL to execute
}

// MigrationRunner handles database migrations with distributed locking.
type MigrationRunner struct {
	db         *sql.DB
	repository string
	migrations []Migration
	lockID     int64 // Advisory lock ID for this migration set
}

// NewMigrationRunner creates a new migration runner with a specific lock ID.
// Different repositories should use different lock IDs to avoid conflicts.
func NewMigrationRunner(
	db *sql.DB, repository string, lockID int64, migrations []Migration,
) *MigrationRunner {
	return &MigrationRunner{
		db:         db,
		repository: repository,
		migrations: migrations,
		lockID:     lockID,
	}
}

// RunMigrations executes all pending migrations with distributed locking.
func (mr *MigrationRunner) RunMigrations(ctx context.Context, l *logger.Logger) error {
	// Try to acquire advisory lock for this repository
	acquired, err := mr.tryAcquireLock(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to acquire migration lock")
	}

	if !acquired {
		// Another instance is running migrations, wait for completion
		l.Info("waiting for another instance to complete migrations",
			slog.String("repository", mr.repository))
		return mr.waitForMigrations(ctx)
	}

	l.Debug("acquired migration lock, starting migrations",
		slog.String("repository", mr.repository),
		slog.Int64("lock_id", mr.lockID))

	// Ensure lock is released when done
	defer func() {
		if err := mr.releaseLock(ctx); err != nil {
			l.Warn("failed to release migration lock",
				slog.String("repository", mr.repository),
				slog.Int64("lock_id", mr.lockID),
				slog.Any("error", err))
		}
	}()

	// Now we have exclusive access - run migrations safely
	return mr.runMigrationsWithLock(ctx, l)
}

// runMigrationsWithLock executes migrations while holding the advisory lock.
func (mr *MigrationRunner) runMigrationsWithLock(ctx context.Context, l *logger.Logger) error {
	// Ensure migration tracking table exists
	if err := mr.ensureMigrationTable(ctx); err != nil {
		return errors.Wrap(err, "failed to create migration table")
	}

	// Get completed migrations
	completed, err := mr.getCompletedMigrations(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get completed migrations")
	}

	// Sort migrations by version
	sort.Slice(mr.migrations, func(i, j int) bool {
		return mr.migrations[i].Version < mr.migrations[j].Version
	})

	// Run pending migrations
	pendingCount := 0
	for _, migration := range mr.migrations {
		if !completed[migration.Version] {
			pendingCount++
		}
	}

	if pendingCount == 0 {
		l.Info("no pending migrations to run",
			slog.String("repository", mr.repository))
		return nil
	}

	l.Info("running pending migrations",
		slog.String("repository", mr.repository),
		slog.Int("pending_count", pendingCount))

	for _, migration := range mr.migrations {
		if completed[migration.Version] {
			continue // Already completed
		}

		l.Info("running migration",
			slog.String("repository", mr.repository),
			slog.Int("version", migration.Version),
			slog.String("description", migration.Description))

		if err := mr.runMigration(ctx, migration); err != nil {
			return errors.Wrapf(err, "failed to run migration %d (%s)", migration.Version, migration.Description)
		}

		l.Info("migration completed successfully",
			slog.String("repository", mr.repository),
			slog.Int("version", migration.Version))
	}

	l.Info("all migrations completed successfully",
		slog.String("repository", mr.repository),
		slog.Int("completed_count", pendingCount))

	return nil
}

// tryAcquireLock attempts to acquire an advisory lock for this repository.
func (mr *MigrationRunner) tryAcquireLock(ctx context.Context) (bool, error) {
	var acquired bool
	err := mr.db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", mr.lockID).Scan(&acquired)
	return acquired, err
}

// releaseLock releases the advisory lock for this repository.
func (mr *MigrationRunner) releaseLock(ctx context.Context) error {
	var released bool
	err := mr.db.QueryRowContext(ctx, "SELECT pg_advisory_unlock($1)", mr.lockID).Scan(&released)
	if err != nil {
		return err
	}
	if !released {
		return fmt.Errorf("failed to release advisory lock %d", mr.lockID)
	}
	return nil
}

// waitForMigrations waits for another instance to complete migrations.
func (mr *MigrationRunner) waitForMigrations(ctx context.Context) error {
	// Try to acquire the lock in blocking mode to wait for completion
	var acquired bool
	err := mr.db.QueryRowContext(ctx, "SELECT pg_advisory_lock($1)", mr.lockID).Scan(&acquired)
	if err != nil {
		return errors.Wrap(err, "failed to wait for migration completion")
	}
	
	// Immediately release the lock since we just wanted to wait
	return mr.releaseLock(ctx)
}

// ensureMigrationTable creates the migration tracking table if it doesn't exist.
func (mr *MigrationRunner) ensureMigrationTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			repository STRING NOT NULL,
			version INT NOT NULL,
			description STRING NOT NULL,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			PRIMARY KEY (repository, version)
		)
	`
	
	_, err := mr.db.ExecContext(ctx, query)
	return err
}

// getCompletedMigrations returns a map of completed migration versions for this repository.
func (mr *MigrationRunner) getCompletedMigrations(ctx context.Context) (map[int]bool, error) {
	rows, err := mr.db.QueryContext(ctx, "SELECT version FROM schema_migrations WHERE repository = $1", mr.repository)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	completed := make(map[int]bool)
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		completed[version] = true
	}

	return completed, rows.Err()
}

// runMigration executes a single migration and records its completion atomically.
func (mr *MigrationRunner) runMigration(ctx context.Context, migration Migration) (retErr error) {
	// Start transaction for atomic migration + completion record
	tx, err := mr.db.BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to begin transaction for migration %d", migration.Version)
	}
	
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				if retErr != nil {
					retErr = errors.CombineErrors(retErr, errors.Wrapf(rollbackErr, "rollback error for migration %d", migration.Version))
				} else {
					retErr = errors.Wrapf(rollbackErr, "rollback error for migration %d", migration.Version)
				}
			}
		}
	}()

	// Execute migration SQL within transaction
	if _, err := tx.ExecContext(ctx, migration.SQL); err != nil {
		return errors.Wrapf(err, "migration SQL execution failed for version %d", migration.Version)
	}

	// Record migration completion within same transaction
	_, err = tx.ExecContext(ctx, 
		`INSERT INTO schema_migrations (repository, version, description, applied_at) 
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (repository, version) DO NOTHING`,
		mr.repository, migration.Version, migration.Description, timeutil.Now())
	if err != nil {
		return errors.Wrapf(err, "failed to record migration %d completion", migration.Version)
	}

	// Commit transaction - both migration and record are atomic
	if err := tx.Commit(); err != nil {
		return errors.Wrapf(err, "failed to commit migration %d transaction", migration.Version)
	}
	
	committed = true
	return nil
}
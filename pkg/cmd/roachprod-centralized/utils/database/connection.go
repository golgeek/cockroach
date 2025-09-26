// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cmd/roachprod-centralized/utils/logger"
	"github.com/cockroachdb/errors"
	_ "github.com/lib/pq"
)

// ConnectionConfig holds database connection configuration.
type ConnectionConfig struct {
	URL         string
	MaxConns    int
	MaxIdleTime int // seconds
}

// NewConnection creates and configures a new database connection.
func NewConnection(ctx context.Context, config ConnectionConfig) (*sql.DB, error) {
	if config.URL == "" {
		return nil, fmt.Errorf("database URL is required")
	}

	db, err := sql.Open("postgres", config.URL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open database connection")
	}

	// Configure connection pool
	if config.MaxConns > 0 {
		db.SetMaxOpenConns(config.MaxConns)
		db.SetMaxIdleConns(config.MaxConns / 2)
	}
	
	if config.MaxIdleTime > 0 {
		db.SetConnMaxIdleTime(time.Duration(config.MaxIdleTime) * time.Second)
	}

	// Test the connection
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "failed to connect to database")
	}

	return db, nil
}

// RunMigrationsForRepository runs migrations for a specific repository.
// Each repository gets its own advisory lock ID to prevent conflicts.
func RunMigrationsForRepository(
	ctx context.Context, l *logger.Logger, db *sql.DB, repositoryName string, migrations []Migration,
) error {
	// Generate unique lock ID based on repository name hash
	lockID := hashString(repositoryName)

	runner := NewMigrationRunner(db, repositoryName, lockID, migrations)
	if err := runner.RunMigrations(ctx, l); err != nil {
		return errors.Wrapf(err, "failed to run %s repository migrations", repositoryName)
	}
	return nil
}

// hashString creates a simple hash of a string for use as lock ID.
func hashString(s string) int64 {
	var hash int64 = 5381
	for _, c := range s {
		hash = ((hash << 5) + hash) + int64(c)
	}
	// Ensure positive value
	if hash < 0 {
		hash = -hash
	}
	return hash
}
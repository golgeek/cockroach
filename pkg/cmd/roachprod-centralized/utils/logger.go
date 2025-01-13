// Copyright 2025 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package utils

import (
	"context"
	"log/slog"
	"strings"
)

var (
	// DefaultLogger is the default logger used by the application
	DefaultLogger = &Logger{
		Logger: slog.Default(),
	}
)

// Logger is simply a wrapper around slog.Logger that implements
// the io.Writer interface. This allows to use slog and its attributes
// for the code that relies on the CockroachDB logger.
type Logger struct {
	*slog.Logger
}

// Write implements the io.Writer interface.
// It writes the data to the slog.Logger with slog.LevelInfo log level
// and adds a source attribute set to "crdb_logger".
// It also removes the trailing newline character from the input data.
func (l *Logger) Write(p []byte) (n int, err error) {
	length := len(p)
	l.Logger.Log(
		context.Background(),
		slog.LevelInfo,
		strings.TrimSuffix(string(p), "\n"),
		slog.String("source", "crdb_logger"),
	)
	return length, nil
}

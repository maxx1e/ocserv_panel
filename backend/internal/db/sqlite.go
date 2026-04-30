// Package db contains local SQLite connection helpers.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// ErrEmptyDSN is returned when no SQLite path or DSN is configured.
var ErrEmptyDSN = errors.New("sqlite dsn is empty")

// OpenSQLite opens a SQLite database using the pure-Go SQLite driver.
func OpenSQLite(ctx context.Context, dsn string) (*sql.DB, error) {
	database, err := Open(ctx, "sqlite", withForeignKeysPragma(dsn))
	if err != nil {
		return nil, err
	}

	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	database.SetConnMaxLifetime(30 * time.Minute)

	return database, nil
}

func withForeignKeysPragma(dsn string) string {
	if strings.TrimSpace(dsn) == "" {
		return dsn
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}

	return dsn + separator + "_pragma=foreign_keys(1)"
}

// Open opens and pings a database connection using database/sql.
func Open(ctx context.Context, driverName string, dsn string) (*sql.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, ErrEmptyDSN
	}

	database, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite database: %w", err)
	}

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("ping sqlite database: %w", err)
	}

	return database, nil
}

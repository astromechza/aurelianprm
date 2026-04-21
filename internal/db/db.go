// Package db manages SQLite database connections.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // SQLite driver for database/sql
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open returns a ready-to-use SQLite database connection with migrations applied.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := conn.ExecContext(context.Background(), pragma); err != nil {
			if closeErr := conn.Close(); closeErr != nil {
				err = fmt.Errorf("%w; close: %v", err, closeErr)
			}
			return nil, fmt.Errorf("apply %q: %w", pragma, err)
		}
	}

	fsys, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = fmt.Errorf("%w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("get migrations subdir: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, fsys, goose.WithLogger(goose.NopLogger()))
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = fmt.Errorf("%w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			err = fmt.Errorf("%w; close: %v", err, closeErr)
		}
		return nil, fmt.Errorf("goose up: %w", err)
	}
	return conn, nil
}

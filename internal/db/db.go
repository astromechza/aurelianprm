// Package db provides SQLite connection setup and schema migration.
package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/astromechza/aurelianprm/migrations"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

// Open opens a SQLite database at path, enables WAL mode and foreign keys,
// then runs any pending Goose migrations. The caller is responsible for
// closing the returned *sql.DB.
func Open(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := configure(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := Migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}

// configure applies SQLite PRAGMAs that must be set before migrations run.
func configure(db *sql.DB) error {
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(context.Background(), p); err != nil {
			return fmt.Errorf("applying pragma %q: %w", p, err)
		}
	}
	return nil
}

// Migrate runs all pending Goose migrations from the embedded SQL files against
// the given database connection. It is exported so that tests can call it
// directly on a pre-opened connection.
func Migrate(db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}
	return nil
}

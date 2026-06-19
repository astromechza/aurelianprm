// Package db manages SQLite database connections.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // SQLite driver for database/sql
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open returns a ready-to-use SQLite database connection with migrations applied.
func Open(path string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(1)

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

// buildDSN appends the required PRAGMAs as `_pragma` query parameters so the
// modernc.org/sqlite driver applies them to *every* connection it opens. A
// PRAGMA run via ExecContext on the *sql.DB pool only affects the single
// connection it happened to run on, which would leave foreign-key enforcement
// (and thus ON DELETE CASCADE) silently disabled on any other pooled
// connection. Setting them in the DSN keeps the pragmas correct independent of
// the pool size, so the SetMaxOpenConns(1) cap above is no longer required for
// correctness.
func buildDSN(path string) string {
	q := url.Values{}
	for _, p := range []string{
		"foreign_keys(1)",
		"journal_mode(WAL)",
		"busy_timeout(5000)",
	} {
		q.Add("_pragma", p)
	}
	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	return path + sep + q.Encode()
}

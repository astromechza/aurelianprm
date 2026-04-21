// Package dal provides data access layer functionality with transaction support.
package dal

import (
	"context"
	"database/sql"
	"fmt"
)

// DBTX is satisfied by both *sql.DB and *sql.Tx.
type DBTX interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// Queries wraps a DBTX and provides all data access methods.
type Queries struct {
	db DBTX
}

// Exec is a pass-through used in tests.
func (q *Queries) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return q.db.ExecContext(ctx, query, args...)
}

// DAL owns the database connection.
type DAL struct {
	db *sql.DB
}

// New creates a DAL from an open *sql.DB.
func New(db *sql.DB) *DAL {
	return &DAL{db: db}
}

// DB returns the underlying *sql.DB (used in tests).
func (d *DAL) DB() *sql.DB {
	return d.db
}

// WithTx runs fn inside a transaction. Commits on success, rolls back on error.
func (d *DAL) WithTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck
	if err := fn(&Queries{db: tx}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

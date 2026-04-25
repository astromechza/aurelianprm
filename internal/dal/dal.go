// Package dal provides data access layer functionality with transaction support.
package dal

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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
	db     *sql.DB
	dbPath string
}

// New creates a DAL from an open *sql.DB and the path that was used to open it.
// Pass ":memory:" for in-memory databases. dbPath is used to determine where
// to place temporary backup files so that a read-only root filesystem is supported.
func New(db *sql.DB, dbPath string) *DAL {
	return &DAL{db: db, dbPath: dbPath}
}

// DB returns the underlying *sql.DB (used in tests).
func (d *DAL) DB() *sql.DB {
	return d.db
}

// Backup writes a transactionally-consistent snapshot of the database to w
// using VACUUM INTO. The temporary file is created alongside the database file
// so that the system /tmp directory does not need to be writable (compatible
// with a read-only root filesystem). For in-memory databases the system temp
// directory is used instead.
func (d *DAL) Backup(ctx context.Context, w io.Writer) error {
	// Choose a writable directory: same directory as the DB for file-backed
	// databases, or os.TempDir() for in-memory / special-path databases.
	tmpDir := os.TempDir()
	if d.dbPath != "" && d.dbPath != ":memory:" {
		tmpDir = filepath.Dir(d.dbPath)
	}

	// os.CreateTemp gives us a unique name; we close and remove the placeholder
	// immediately because VACUUM INTO requires the destination not to exist.
	f, err := os.CreateTemp(tmpDir, "aurelianprm-backup-*.db")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := f.Name()
	_ = f.Close()
	_ = os.Remove(tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := d.db.ExecContext(ctx, "VACUUM INTO ?", tmpPath); err != nil {
		return fmt.Errorf("vacuum into: %w", err)
	}

	backupFile, err := os.Open(tmpPath)
	if err != nil {
		return fmt.Errorf("open backup file: %w", err)
	}
	defer func() {
		if cerr := backupFile.Close(); cerr != nil {
			log.Printf("backup file close: %v", cerr)
		}
	}()

	if _, err := io.Copy(w, backupFile); err != nil {
		return fmt.Errorf("stream backup: %w", err)
	}
	return nil
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

// Package store implements the data-access layer for AurelianPRM.
// Each entity has its own file; this file provides shared helpers.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// newID generates a new random UUID string.
func newID() string {
	return uuid.New().String()
}

// now returns the current UTC time truncated to millisecond precision,
// matching SQLite's strftime millisecond resolution.
func now() time.Time {
	return time.Now().UTC().Truncate(time.Millisecond)
}

// scanNullString converts a sql.NullString to *string.
func scanNullString(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

// scanNullInt converts a sql.NullInt64 to *int.
func scanNullInt(ni sql.NullInt64) *int {
	if !ni.Valid {
		return nil
	}
	v := int(ni.Int64)
	return &v
}

// scanNullFloat converts a sql.NullFloat64 to *float64.
func scanNullFloat(nf sql.NullFloat64) *float64 {
	if !nf.Valid {
		return nil
	}
	v := nf.Float64
	return &v
}

// nullString converts *string to sql.NullString.
func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

// nullInt converts *int to sql.NullInt64.
func nullInt(i *int) sql.NullInt64 {
	if i == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*i), Valid: true}
}

// nullFloat converts *float64 to sql.NullFloat64.
func nullFloat(f *float64) sql.NullFloat64 {
	if f == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *f, Valid: true}
}

const timeLayout = "2006-01-02T15:04:05.000Z"

// parseTime parses an SQLite datetime string produced by strftime('%Y-%m-%dT%H:%M:%fZ', ...).
// The fallback handles the edge case where SQLite omits the fractional seconds
// entirely when the value falls exactly on a whole second (e.g. "2006-01-02T15:04:05Z").
func parseTime(s string) (time.Time, error) {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05Z", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("parsing time %q: %w", s, err)
		}
	}
	return t, nil
}

// execContext is a convenience wrapper for INSERT statements that discards the
// sql.Result. RowsAffected is not checked because INSERT failures (e.g. FK
// violations, UNIQUE conflicts) always surface as a non-nil error from
// modernc/sqlite — a silent zero-rows insert cannot occur.
func execContext(ctx context.Context, db *sql.DB, query string, args ...any) error {
	_, err := db.ExecContext(ctx, query, args...)
	return err
}

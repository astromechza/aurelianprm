package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateAddress inserts a new addresses row and returns it with its generated ID.
func CreateAddress(ctx context.Context, db *sql.DB, a model.Address) (model.Address, error) {
	a.ID = newID()
	a.CreatedAt = now()
	const q = `
		INSERT INTO addresses (id, location_id, label, notes, created_at)
		VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		a.ID, a.LocationID, nullString(a.Label), nullString(a.Notes),
		a.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Address{}, fmt.Errorf("store.CreateAddress: %w", err)
	}
	return a, nil
}

// GetAddress fetches a single addresses row by ID.
func GetAddress(ctx context.Context, db *sql.DB, id string) (model.Address, error) {
	const q = `SELECT id, location_id, label, notes, created_at FROM addresses WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	a, err := scanAddress(row)
	if err != nil {
		return model.Address{}, fmt.Errorf("store.GetAddress: %w", err)
	}
	return a, nil
}

// UpdateAddress replaces all mutable fields of an addresses row.
func UpdateAddress(ctx context.Context, db *sql.DB, a model.Address) error {
	const q = `UPDATE addresses SET location_id = ?, label = ?, notes = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, a.LocationID, nullString(a.Label), nullString(a.Notes), a.ID)
	if err != nil {
		return fmt.Errorf("store.UpdateAddress: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateAddress: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteAddress removes an addresses row by ID.
func DeleteAddress(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM addresses WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteAddress: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteAddress: %w", sql.ErrNoRows)
	}
	return nil
}

func scanAddress(row *sql.Row) (model.Address, error) {
	var a model.Address
	var label, notes sql.NullString
	var createdAt string
	if err := row.Scan(&a.ID, &a.LocationID, &label, &notes, &createdAt); err != nil {
		return model.Address{}, err
	}
	a.Label = scanNullString(label)
	a.Notes = scanNullString(notes)
	var err error
	if a.CreatedAt, err = parseTime(createdAt); err != nil {
		return model.Address{}, err
	}
	return a, nil
}

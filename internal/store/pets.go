package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreatePet inserts a new pets row and returns it with its generated ID.
func CreatePet(ctx context.Context, db DBTX, p model.Pet) (model.Pet, error) {
	p.ID = newID()
	p.CreatedAt = now()
	const q = `
		INSERT INTO pets (id, name, notes, created_at)
		VALUES (?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		p.ID, p.Name, nullString(p.Notes),
		p.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Pet{}, fmt.Errorf("store.CreatePet: %w", err)
	}
	return p, nil
}

// GetPet fetches a single pets row by ID.
func GetPet(ctx context.Context, db DBTX, id string) (model.Pet, error) {
	const q = `SELECT id, name, notes, created_at FROM pets WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	p, err := scanPet(row)
	if err != nil {
		return model.Pet{}, fmt.Errorf("store.GetPet: %w", err)
	}
	return p, nil
}

// UpdatePet replaces all mutable fields of a pets row.
func UpdatePet(ctx context.Context, db DBTX, p model.Pet) error {
	const q = `UPDATE pets SET name = ?, notes = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, p.Name, nullString(p.Notes), p.ID)
	if err != nil {
		return fmt.Errorf("store.UpdatePet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdatePet: %w", sql.ErrNoRows)
	}
	return nil
}

// DeletePet removes a pets row by ID.
func DeletePet(ctx context.Context, db DBTX, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM pets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeletePet: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeletePet: %w", sql.ErrNoRows)
	}
	return nil
}

func scanPet(row *sql.Row) (model.Pet, error) {
	var p model.Pet
	var notes sql.NullString
	var createdAt string
	if err := row.Scan(&p.ID, &p.Name, &notes, &createdAt); err != nil {
		return model.Pet{}, err
	}
	p.Notes = scanNullString(notes)
	var err error
	if p.CreatedAt, err = parseTime(createdAt); err != nil {
		return model.Pet{}, err
	}
	return p, nil
}

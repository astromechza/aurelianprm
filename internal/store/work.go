package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateWork inserts a new work row and returns it with its generated ID.
func CreateWork(ctx context.Context, db DBTX, w model.Work) (model.Work, error) {
	w.ID = newID()
	w.CreatedAt = now()
	const q = `
		INSERT INTO work (id, name, website, notes, created_at)
		VALUES (?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		w.ID, w.Name, nullString(w.Website), nullString(w.Notes),
		w.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Work{}, fmt.Errorf("store.CreateWork: %w", err)
	}
	return w, nil
}

// GetWork fetches a single work row by ID.
func GetWork(ctx context.Context, db DBTX, id string) (model.Work, error) {
	const q = `SELECT id, name, website, notes, created_at FROM work WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	w, err := scanWork(row)
	if err != nil {
		return model.Work{}, fmt.Errorf("store.GetWork: %w", err)
	}
	return w, nil
}

// UpdateWork replaces all mutable fields of a work row.
func UpdateWork(ctx context.Context, db DBTX, w model.Work) error {
	const q = `UPDATE work SET name = ?, website = ?, notes = ? WHERE id = ?`
	res, err := db.ExecContext(ctx, q, w.Name, nullString(w.Website), nullString(w.Notes), w.ID)
	if err != nil {
		return fmt.Errorf("store.UpdateWork: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateWork: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteWork removes a work row by ID.
func DeleteWork(ctx context.Context, db DBTX, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM work WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteWork: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteWork: %w", sql.ErrNoRows)
	}
	return nil
}

func scanWork(row *sql.Row) (model.Work, error) {
	var w model.Work
	var website, notes sql.NullString
	var createdAt string
	if err := row.Scan(&w.ID, &w.Name, &website, &notes, &createdAt); err != nil {
		return model.Work{}, err
	}
	w.Website = scanNullString(website)
	w.Notes = scanNullString(notes)
	var err error
	if w.CreatedAt, err = parseTime(createdAt); err != nil {
		return model.Work{}, err
	}
	return w, nil
}

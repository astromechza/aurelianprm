package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreatePerson inserts a new people row and returns it with generated ID and timestamps.
func CreatePerson(ctx context.Context, db *sql.DB, p model.Person) (model.Person, error) {
	p.ID = newID()
	p.CreatedAt = now()
	p.UpdatedAt = p.CreatedAt
	const q = `
		INSERT INTO people (id, name, nickname, notes, birthday_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?), strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		p.ID, p.Name, nullString(p.Nickname), nullString(p.Notes), nullString(p.BirthdayID),
		p.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
		p.UpdatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Person{}, fmt.Errorf("store.CreatePerson: %w", err)
	}
	return p, nil
}

// GetPerson fetches a single people row by ID.
func GetPerson(ctx context.Context, db *sql.DB, id string) (model.Person, error) {
	const q = `
		SELECT id, name, nickname, notes, birthday_id, created_at, updated_at
		FROM people WHERE id = ?`
	row := db.QueryRowContext(ctx, q, id)
	p, err := scanPerson(row)
	if err != nil {
		return model.Person{}, fmt.Errorf("store.GetPerson: %w", err)
	}
	return p, nil
}

// ListPeople returns all people, optionally filtered by a substring match on name or nickname.
// An empty query string returns all records ordered by name.
func ListPeople(ctx context.Context, db *sql.DB, nameQuery string) ([]model.Person, error) {
	var (
		rows *sql.Rows
		err  error
	)
	const base = `
		SELECT id, name, nickname, notes, birthday_id, created_at, updated_at
		FROM people`
	if nameQuery == "" {
		rows, err = db.QueryContext(ctx, base+` ORDER BY name`)
	} else {
		like := "%" + nameQuery + "%"
		rows, err = db.QueryContext(ctx,
			base+` WHERE name LIKE ? OR nickname LIKE ? ORDER BY name`, like, like)
	}
	if err != nil {
		return nil, fmt.Errorf("store.ListPeople: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return scanPeople(rows)
}

// UpdatePerson replaces all mutable fields of a people row and bumps updated_at.
func UpdatePerson(ctx context.Context, db *sql.DB, p model.Person) (model.Person, error) {
	p.UpdatedAt = now()
	const q = `
		UPDATE people
		SET name = ?, nickname = ?, notes = ?, birthday_id = ?,
		    updated_at = strftime('%Y-%m-%dT%H:%M:%fZ', ?)
		WHERE id = ?`
	res, err := db.ExecContext(ctx, q,
		p.Name, nullString(p.Nickname), nullString(p.Notes), nullString(p.BirthdayID),
		p.UpdatedAt.Format("2006-01-02T15:04:05.000Z"), p.ID,
	)
	if err != nil {
		return model.Person{}, fmt.Errorf("store.UpdatePerson: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.Person{}, fmt.Errorf("store.UpdatePerson: %w", sql.ErrNoRows)
	}
	return p, nil
}

// DeletePerson removes a people row by ID (cascades to contact_methods, interactions, relationships).
func DeletePerson(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeletePerson: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeletePerson: %w", sql.ErrNoRows)
	}
	return nil
}

func scanPerson(row *sql.Row) (model.Person, error) {
	var p model.Person
	var nickname, notes, birthdayID sql.NullString
	var createdAt, updatedAt string
	if err := row.Scan(&p.ID, &p.Name, &nickname, &notes, &birthdayID, &createdAt, &updatedAt); err != nil {
		return model.Person{}, err
	}
	p.Nickname = scanNullString(nickname)
	p.Notes = scanNullString(notes)
	p.BirthdayID = scanNullString(birthdayID)
	var err error
	if p.CreatedAt, err = parseTime(createdAt); err != nil {
		return model.Person{}, err
	}
	if p.UpdatedAt, err = parseTime(updatedAt); err != nil {
		return model.Person{}, err
	}
	return p, nil
}

func scanPeople(rows *sql.Rows) ([]model.Person, error) {
	var people []model.Person
	for rows.Next() {
		var p model.Person
		var nickname, notes, birthdayID sql.NullString
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &nickname, &notes, &birthdayID, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("store.scanPeople: %w", err)
		}
		p.Nickname = scanNullString(nickname)
		p.Notes = scanNullString(notes)
		p.BirthdayID = scanNullString(birthdayID)
		var err error
		if p.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		if p.UpdatedAt, err = parseTime(updatedAt); err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.scanPeople: %w", err)
	}
	return people, nil
}

package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateInteraction inserts a new interactions row and returns it with its generated ID.
func CreateInteraction(ctx context.Context, db *sql.DB, i model.Interaction) (model.Interaction, error) {
	i.ID = newID()
	i.CreatedAt = now()
	const q = `
		INSERT INTO interactions (id, person_id, occurred_at, channel, notes, created_at)
		VALUES (?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?), ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		i.ID, i.PersonID,
		i.OccurredAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		nullString(i.Channel), nullString(i.Notes),
		i.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Interaction{}, fmt.Errorf("store.CreateInteraction: %w", err)
	}
	return i, nil
}

// ListInteractions returns all interactions for a person, newest first.
func ListInteractions(ctx context.Context, db *sql.DB, personID string) ([]model.Interaction, error) {
	const q = `
		SELECT id, person_id, occurred_at, channel, notes, created_at
		FROM interactions WHERE person_id = ? ORDER BY occurred_at DESC`
	rows, err := db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, fmt.Errorf("store.ListInteractions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []model.Interaction
	for rows.Next() {
		var i model.Interaction
		var channel, notes sql.NullString
		var occurredAt, createdAt string
		if err := rows.Scan(&i.ID, &i.PersonID, &occurredAt, &channel, &notes, &createdAt); err != nil {
			return nil, fmt.Errorf("store.ListInteractions: %w", err)
		}
		i.Channel = scanNullString(channel)
		i.Notes = scanNullString(notes)
		if i.OccurredAt, err = parseTime(occurredAt); err != nil {
			return nil, err
		}
		if i.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.ListInteractions: %w", err)
	}
	return out, nil
}

// DeleteInteraction removes an interactions row by ID.
func DeleteInteraction(ctx context.Context, db *sql.DB, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM interactions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteInteraction: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteInteraction: %w", sql.ErrNoRows)
	}
	return nil
}

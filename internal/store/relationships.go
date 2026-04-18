package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateRelationship inserts a new relationships row and returns it with its generated ID.
func CreateRelationship(ctx context.Context, db DBTX, r model.Relationship) (model.Relationship, error) {
	r.ID = newID()
	r.CreatedAt = now()
	const q = `
		INSERT INTO relationships (id, person_id, to_type, to_id, label, since_id, until_id, notes, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		r.ID, r.PersonID, string(r.ToType), r.ToID, r.Label,
		nullString(r.SinceID), nullString(r.UntilID), nullString(r.Notes),
		r.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.Relationship{}, fmt.Errorf("store.CreateRelationship: %w", err)
	}
	return r, nil
}

// ListRelationships returns all relationships for a person, ordered by created_at.
func ListRelationships(ctx context.Context, db DBTX, personID string) ([]model.Relationship, error) {
	const q = `
		SELECT id, person_id, to_type, to_id, label, since_id, until_id, notes, created_at
		FROM relationships WHERE person_id = ? ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, fmt.Errorf("store.ListRelationships: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return scanRelationships(rows)
}

// ListRelationshipsToEntity returns all relationships pointing to a specific entity.
func ListRelationshipsToEntity(ctx context.Context, db DBTX, toType model.RelationshipTarget, toID string) ([]model.Relationship, error) {
	const q = `
		SELECT id, person_id, to_type, to_id, label, since_id, until_id, notes, created_at
		FROM relationships WHERE to_type = ? AND to_id = ? ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, string(toType), toID)
	if err != nil {
		return nil, fmt.Errorf("store.ListRelationshipsToEntity: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return scanRelationships(rows)
}

// DeleteRelationship removes a relationships row by ID.
func DeleteRelationship(ctx context.Context, db DBTX, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM relationships WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteRelationship: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteRelationship: %w", sql.ErrNoRows)
	}
	return nil
}

// UpdateRelationshipDates updates the since_id and until_id foreign keys on a
// relationships row without changing any other fields.
func UpdateRelationshipDates(ctx context.Context, db DBTX, id string, sinceID, untilID *string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE relationships SET since_id = ?, until_id = ? WHERE id = ?`,
		nullString(sinceID), nullString(untilID), id,
	)
	if err != nil {
		return fmt.Errorf("store.UpdateRelationshipDates: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateRelationshipDates: %w", sql.ErrNoRows)
	}
	return nil
}

func scanRelationships(rows *sql.Rows) ([]model.Relationship, error) {
	var out []model.Relationship
	for rows.Next() {
		var r model.Relationship
		var sinceID, untilID, notes sql.NullString
		var createdAt string
		if err := rows.Scan(
			&r.ID, &r.PersonID, (*string)(&r.ToType), &r.ToID, &r.Label,
			&sinceID, &untilID, &notes, &createdAt,
		); err != nil {
			return nil, fmt.Errorf("store.scanRelationships: %w", err)
		}
		r.SinceID = scanNullString(sinceID)
		r.UntilID = scanNullString(untilID)
		r.Notes = scanNullString(notes)
		var err error
		if r.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.scanRelationships: %w", err)
	}
	return out, nil
}

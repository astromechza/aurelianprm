package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// PersonRelationshipRow is a joined view of a relationship row with its
// since/until blurry_date fields expanded to raw integers. ToName is not
// resolved here — callers in the web layer use resolveEntityName for that.
type PersonRelationshipRow struct {
	Relationship model.Relationship
	SinceYear    *int
	SinceMonth   *int
	SinceDay     *int
	UntilYear    *int
	UntilMonth   *int
	UntilDay     *int
}

const personRelationshipSelect = `
	SELECT r.id, r.person_id, r.to_type, r.to_id, r.label,
	       r.since_id, r.until_id, r.notes, r.created_at,
	       bd_since.year, bd_since.month, bd_since.day,
	       bd_until.year, bd_until.month, bd_until.day
	FROM relationships r
	LEFT JOIN blurry_dates bd_since ON bd_since.id = r.since_id
	LEFT JOIN blurry_dates bd_until ON bd_until.id = r.until_id`

// ListPersonRelationships returns all relationships for a person, ordered by
// created_at, with since/until blurry_date integers joined in. Returns an
// empty slice (not nil) when no relationships exist.
func ListPersonRelationships(ctx context.Context, db DBTX, personID string) ([]PersonRelationshipRow, error) {
	q := personRelationshipSelect + `
	WHERE r.person_id = ?
	ORDER BY r.created_at`
	rows, err := db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, fmt.Errorf("store.ListPersonRelationships: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	out := []PersonRelationshipRow{}
	for rows.Next() {
		row, err := scanPersonRelationshipRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("store.ListPersonRelationships: %w", err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.ListPersonRelationships: %w", err)
	}
	return out, nil
}

// GetPersonRelationshipRow fetches a single relationship for a person by
// relationship ID. Returns a wrapped sql.ErrNoRows if not found or if the
// relationship's person_id does not match (prevents cross-person access).
func GetPersonRelationshipRow(ctx context.Context, db DBTX, personID, relID string) (PersonRelationshipRow, error) {
	q := personRelationshipSelect + `
	WHERE r.id = ? AND r.person_id = ?`
	row := db.QueryRowContext(ctx, q, relID, personID)
	prr, err := scanPersonRelationshipRow(row.Scan)
	if err != nil {
		return PersonRelationshipRow{}, fmt.Errorf("store.GetPersonRelationshipRow: %w", err)
	}
	return prr, nil
}

// UpdateRelationship updates the label, notes, since_id, and until_id of a
// relationship row. Returns a wrapped sql.ErrNoRows if the row does not exist.
func UpdateRelationship(ctx context.Context, db DBTX, id, label string, notes *string, sinceID, untilID *string) error {
	res, err := db.ExecContext(ctx,
		`UPDATE relationships SET label = ?, notes = ?, since_id = ?, until_id = ? WHERE id = ?`,
		label, nullString(notes), nullString(sinceID), nullString(untilID), id,
	)
	if err != nil {
		return fmt.Errorf("store.UpdateRelationship: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.UpdateRelationship: %w", sql.ErrNoRows)
	}
	return nil
}

// scanPersonRelationshipRow accepts either rows.Scan or row.Scan and
// populates a PersonRelationshipRow.
func scanPersonRelationshipRow(scanFn func(dest ...any) error) (PersonRelationshipRow, error) {
	var prr PersonRelationshipRow
	var sinceID, untilID, notes sql.NullString
	var createdAt string
	var sinceYear, sinceMonth, sinceDay sql.NullInt64
	var untilYear, untilMonth, untilDay sql.NullInt64

	if err := scanFn(
		&prr.Relationship.ID, &prr.Relationship.PersonID,
		(*string)(&prr.Relationship.ToType), &prr.Relationship.ToID,
		&prr.Relationship.Label,
		&sinceID, &untilID, &notes, &createdAt,
		&sinceYear, &sinceMonth, &sinceDay,
		&untilYear, &untilMonth, &untilDay,
	); err != nil {
		return PersonRelationshipRow{}, err
	}
	prr.Relationship.SinceID = scanNullString(sinceID)
	prr.Relationship.UntilID = scanNullString(untilID)
	prr.Relationship.Notes = scanNullString(notes)
	prr.SinceYear = scanNullInt(sinceYear)
	prr.SinceMonth = scanNullInt(sinceMonth)
	prr.SinceDay = scanNullInt(sinceDay)
	prr.UntilYear = scanNullInt(untilYear)
	prr.UntilMonth = scanNullInt(untilMonth)
	prr.UntilDay = scanNullInt(untilDay)
	var err error
	if prr.Relationship.CreatedAt, err = parseTime(createdAt); err != nil {
		return PersonRelationshipRow{}, fmt.Errorf("parsing created_at: %w", err)
	}
	return prr, nil
}

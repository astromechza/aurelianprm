package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
)

// CreateContactMethod inserts a new contact_methods row and returns it with its generated ID.
func CreateContactMethod(ctx context.Context, db DBTX, cm model.ContactMethod) (model.ContactMethod, error) {
	cm.ID = newID()
	cm.CreatedAt = now()
	const q = `
		INSERT INTO contact_methods (id, person_id, type, value, label, created_at)
		VALUES (?, ?, ?, ?, ?, strftime('%Y-%m-%dT%H:%M:%fZ', ?))`
	if err := execContext(ctx, db, q,
		cm.ID, cm.PersonID, string(cm.Type), cm.Value, nullString(cm.Label),
		cm.CreatedAt.Format("2006-01-02T15:04:05.000Z"),
	); err != nil {
		return model.ContactMethod{}, fmt.Errorf("store.CreateContactMethod: %w", err)
	}
	return cm, nil
}

// ListContactMethods returns all contact methods for the given person, ordered by created_at.
func ListContactMethods(ctx context.Context, db DBTX, personID string) ([]model.ContactMethod, error) {
	const q = `
		SELECT id, person_id, type, value, label, created_at
		FROM contact_methods WHERE person_id = ? ORDER BY created_at`
	rows, err := db.QueryContext(ctx, q, personID)
	if err != nil {
		return nil, fmt.Errorf("store.ListContactMethods: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var out []model.ContactMethod
	for rows.Next() {
		var cm model.ContactMethod
		var label sql.NullString
		var createdAt string
		if err := rows.Scan(&cm.ID, &cm.PersonID, (*string)(&cm.Type), &cm.Value, &label, &createdAt); err != nil {
			return nil, fmt.Errorf("store.ListContactMethods: %w", err)
		}
		cm.Label = scanNullString(label)
		if cm.CreatedAt, err = parseTime(createdAt); err != nil {
			return nil, err
		}
		out = append(out, cm)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store.ListContactMethods: %w", err)
	}
	return out, nil
}

// DeleteContactMethod removes a contact_methods row by ID.
func DeleteContactMethod(ctx context.Context, db DBTX, id string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM contact_methods WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store.DeleteContactMethod: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store.DeleteContactMethod: %w", sql.ErrNoRows)
	}
	return nil
}

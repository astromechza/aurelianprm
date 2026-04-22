package dal

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/astromechza/aurelianprm/internal/schema"
)

// Relationship represents a row in the relationships table.
type Relationship struct {
	ID        string
	EntityAID string
	EntityBID string
	Type      string
	DateFrom  *string // partial ISO 8601: "YYYY", "YYYY-MM", or "YYYY-MM-DD"
	DateTo    *string // partial ISO 8601: "YYYY", "YYYY-MM", or "YYYY-MM-DD"
	Note      *string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// partialDateRE matches YYYY, YYYY-MM, or YYYY-MM-DD.
var partialDateRE = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

func validatePartialDate(field, value string) error {
	if !partialDateRE.MatchString(value) {
		return fmt.Errorf("%s %q must be YYYY, YYYY-MM, or YYYY-MM-DD", field, value)
	}
	return nil
}

// validateDateRange checks that from <= to using lexicographic order, which is
// correct for partial ISO 8601 strings of any precision.
func validateDateRange(from, to string) error {
	if from > to {
		return fmt.Errorf("date_from %q must not be after date_to %q", from, to)
	}
	return nil
}

// CreateRelationshipParams holds fields for inserting a new relationship.
type CreateRelationshipParams struct {
	EntityAID string
	EntityBID string
	Type      string
	DateFrom  *string
	DateTo    *string
	Note      *string
}

// NeighbourResult is returned by GetNeighbours and pairs an entity with the
// relationship that connects it to the query entity.
type NeighbourResult struct {
	Entity       Entity
	Relationship Relationship
	Direction    string // "outbound" (query entity is entity_a) or "inbound" (query entity is entity_b)
}

func scanRelationship(row interface{ Scan(...any) error }) (Relationship, error) {
	var r Relationship
	err := row.Scan(
		&r.ID, &r.EntityAID, &r.EntityBID, &r.Type,
		&r.DateFrom, &r.DateTo, &r.Note,
		&r.CreatedAt, &r.UpdatedAt,
	)
	return r, err
}

// CreateRelationship validates the relationship type constraints and inserts a new edge.
func (q *Queries) CreateRelationship(ctx context.Context, params CreateRelationshipParams) (Relationship, error) {
	entityA, err := q.GetEntity(ctx, params.EntityAID)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: fetch entity_a: %w", err)
	}
	entityB, err := q.GetEntity(ctx, params.EntityBID)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: fetch entity_b: %w", err)
	}
	if err := schema.ValidateRelationship(params.Type, entityA.Type, entityB.Type); err != nil {
		return Relationship{}, fmt.Errorf("create relationship: %w", err)
	}
	if params.DateFrom != nil {
		if err := validatePartialDate("date_from", *params.DateFrom); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}
	if params.DateTo != nil {
		if err := validatePartialDate("date_to", *params.DateTo); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}
	if params.DateFrom != nil && params.DateTo != nil {
		if err := validateDateRange(*params.DateFrom, *params.DateTo); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: %w", err)
		}
	}

	// For symmetric relationship types, block reverse-direction duplicates that
	// the UNIQUE constraint alone cannot catch.
	def := schema.RelationshipTypes[params.Type]
	if def.Symmetric {
		var n int
		if err := q.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM relationships
			WHERE type=? AND (
				(entity_a_id=? AND entity_b_id=?) OR
				(entity_a_id=? AND entity_b_id=?)
			)`, params.Type,
			params.EntityAID, params.EntityBID,
			params.EntityBID, params.EntityAID,
		).Scan(&n); err != nil {
			return Relationship{}, fmt.Errorf("create relationship: duplicate check: %w", err)
		}
		if n > 0 {
			return Relationship{}, fmt.Errorf("create relationship: %q relationship already exists between these entities", params.Type)
		}
	}

	const query = `
		INSERT INTO relationships (id, entity_a_id, entity_b_id, type, date_from, date_to, note)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		RETURNING id, entity_a_id, entity_b_id, type, date_from, date_to, note, created_at, updated_at`
	row := q.db.QueryRowContext(ctx, query,
		newID(), params.EntityAID, params.EntityBID, params.Type,
		params.DateFrom, params.DateTo, params.Note,
	)
	r, err := scanRelationship(row)
	if err != nil {
		return Relationship{}, fmt.Errorf("create relationship: %w", err)
	}
	return r, nil
}

// DeleteRelationship removes a relationship by ID.
func (q *Queries) DeleteRelationship(ctx context.Context, id string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM relationships WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete relationship: %w", err)
	}
	return nil
}

// GetNeighbours returns all entities directly connected to entityID via any
// relationship, along with the relationship and its direction relative to entityID.
func (q *Queries) GetNeighbours(ctx context.Context, entityID string) ([]NeighbourResult, error) {
	const query = `
		SELECT
			e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at,
			r.id, r.entity_a_id, r.entity_b_id, r.type, r.date_from, r.date_to, r.note, r.created_at, r.updated_at,
			CASE WHEN r.entity_a_id = ? THEN 'outbound' ELSE 'inbound' END AS direction
		FROM relationships r
		JOIN entities e ON e.id = CASE
			WHEN r.entity_a_id = ? THEN r.entity_b_id
			ELSE r.entity_a_id
		END
		WHERE r.entity_a_id = ? OR r.entity_b_id = ?
		ORDER BY r.created_at`

	rows, err := q.db.QueryContext(ctx, query, entityID, entityID, entityID, entityID)
	if err != nil {
		return nil, fmt.Errorf("get neighbours: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var results []NeighbourResult
	for rows.Next() {
		var nr NeighbourResult
		var entityData string
		err := rows.Scan(
			&nr.Entity.ID, &nr.Entity.Type, &nr.Entity.DisplayName, &entityData,
			&nr.Entity.CreatedAt, &nr.Entity.UpdatedAt,
			&nr.Relationship.ID, &nr.Relationship.EntityAID, &nr.Relationship.EntityBID,
			&nr.Relationship.Type, &nr.Relationship.DateFrom, &nr.Relationship.DateTo,
			&nr.Relationship.Note, &nr.Relationship.CreatedAt, &nr.Relationship.UpdatedAt,
			&nr.Direction,
		)
		if err != nil {
			return nil, fmt.Errorf("scan neighbour: %w", err)
		}
		nr.Entity.Data = []byte(entityData)
		results = append(results, nr)
	}
	return results, rows.Err()
}

// UpdateRelationshipParams holds editable fields for an existing relationship.
type UpdateRelationshipParams struct {
	ID       string
	DateFrom *string // partial ISO 8601 or nil to clear
	DateTo   *string // partial ISO 8601 or nil to clear
	Note     *string // nil to clear
}

// UpdateRelationship updates the date range and note on an existing relationship.
// The entity pair and type are immutable.
func (q *Queries) UpdateRelationship(ctx context.Context, params UpdateRelationshipParams) error {
	if params.DateFrom != nil {
		if err := validatePartialDate("date_from", *params.DateFrom); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	if params.DateTo != nil {
		if err := validatePartialDate("date_to", *params.DateTo); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	if params.DateFrom != nil && params.DateTo != nil {
		if err := validateDateRange(*params.DateFrom, *params.DateTo); err != nil {
			return fmt.Errorf("update relationship: %w", err)
		}
	}
	const stmt = `
		UPDATE relationships
		SET date_from=?, date_to=?, note=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	res, err := q.db.ExecContext(ctx, stmt, params.DateFrom, params.DateTo, params.Note, params.ID)
	if err != nil {
		return fmt.Errorf("update relationship: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update relationship: id %q not found", params.ID)
	}
	return nil
}

// GetRelationship fetches a single relationship by ID. Returns sql.ErrNoRows if not found.
func (q *Queries) GetRelationship(ctx context.Context, id string) (Relationship, error) {
	const query = `
		SELECT id, entity_a_id, entity_b_id, type, date_from, date_to, note, created_at, updated_at
		FROM relationships WHERE id = ?`
	r, err := scanRelationship(q.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Relationship{}, err
	}
	return r, nil
}

// GetNeighboursByRelType returns neighbours connected via a specific relationship type.
func (q *Queries) GetNeighboursByRelType(ctx context.Context, entityID, relType string) ([]NeighbourResult, error) {
	const stmt = `
		SELECT
			e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at,
			r.id, r.entity_a_id, r.entity_b_id, r.type, r.date_from, r.date_to, r.note, r.created_at, r.updated_at,
			CASE WHEN r.entity_a_id = ? THEN 'outbound' ELSE 'inbound' END AS direction
		FROM relationships r
		JOIN entities e ON e.id = CASE
			WHEN r.entity_a_id = ? THEN r.entity_b_id
			ELSE r.entity_a_id
		END
		WHERE (r.entity_a_id = ? OR r.entity_b_id = ?) AND r.type = ?
		ORDER BY r.created_at`

	rows, err := q.db.QueryContext(ctx, stmt, entityID, entityID, entityID, entityID, relType)
	if err != nil {
		return nil, fmt.Errorf("get neighbours by rel type: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var results []NeighbourResult
	for rows.Next() {
		var nr NeighbourResult
		var entityData string
		err := rows.Scan(
			&nr.Entity.ID, &nr.Entity.Type, &nr.Entity.DisplayName, &entityData,
			&nr.Entity.CreatedAt, &nr.Entity.UpdatedAt,
			&nr.Relationship.ID, &nr.Relationship.EntityAID, &nr.Relationship.EntityBID,
			&nr.Relationship.Type, &nr.Relationship.DateFrom, &nr.Relationship.DateTo,
			&nr.Relationship.Note, &nr.Relationship.CreatedAt, &nr.Relationship.UpdatedAt,
			&nr.Direction,
		)
		if err != nil {
			return nil, fmt.Errorf("scan neighbour: %w", err)
		}
		nr.Entity.Data = []byte(entityData)
		results = append(results, nr)
	}
	return results, rows.Err()
}

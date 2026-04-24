package dal

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/schema"
	"github.com/oklog/ulid/v2"
)

// Entity represents a row in the entities table.
type Entity struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	DisplayName *string         `json:"display_name"`
	Data        json.RawMessage `json:"data"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// CreateEntityParams holds fields for inserting a new entity.
type CreateEntityParams struct {
	Type        string
	DisplayName *string
	Data        json.RawMessage
}

// UpdateEntityParams holds fields for updating an existing entity.
type UpdateEntityParams struct {
	ID          string
	DisplayName *string
	Data        json.RawMessage
}

func newID() string {
	return ulid.Make().String()
}

func scanEntity(row interface{ Scan(...any) error }) (Entity, error) {
	var e Entity
	var data string
	err := row.Scan(&e.ID, &e.Type, &e.DisplayName, &data, &e.CreatedAt, &e.UpdatedAt)
	if err != nil {
		return Entity{}, err
	}
	e.Data = json.RawMessage(data)
	return e, nil
}

// CreateEntity validates data against the entity type schema and inserts a new entity.
func (q *Queries) CreateEntity(ctx context.Context, params CreateEntityParams) (Entity, error) {
	if params.DisplayName != nil && *params.DisplayName == "" {
		return Entity{}, fmt.Errorf("create entity: display_name must not be empty string; use nil to omit")
	}
	if err := schema.ValidateEntity(params.Type, params.Data); err != nil {
		return Entity{}, fmt.Errorf("create entity: %w", err)
	}
	const query = `
		INSERT INTO entities (id, type, display_name, data)
		VALUES (?, ?, ?, ?)
		RETURNING id, type, display_name, data, created_at, updated_at`
	row := q.db.QueryRowContext(ctx, query, newID(), params.Type, params.DisplayName, string(params.Data))
	e, err := scanEntity(row)
	if err != nil {
		return Entity{}, fmt.Errorf("create entity: %w", err)
	}
	return e, nil
}

// GetEntity fetches a single entity by ID. Returns sql.ErrNoRows if not found.
func (q *Queries) GetEntity(ctx context.Context, id string) (Entity, error) {
	const query = `
		SELECT id, type, display_name, data, created_at, updated_at
		FROM entities WHERE id = ?`
	e, err := scanEntity(q.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Entity{}, err
	}
	return e, nil
}

// UpdateEntity validates data and updates display_name and data for an existing entity.
func (q *Queries) UpdateEntity(ctx context.Context, params UpdateEntityParams) error {
	if params.DisplayName != nil && *params.DisplayName == "" {
		return fmt.Errorf("update entity: display_name must not be empty string; use nil to omit")
	}
	existing, err := q.GetEntity(ctx, params.ID)
	if err != nil {
		return fmt.Errorf("update entity: fetch existing: %w", err)
	}
	if err := schema.ValidateEntity(existing.Type, params.Data); err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	const query = `
		UPDATE entities
		SET display_name=?, data=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	res, err := q.db.ExecContext(ctx, query, params.DisplayName, string(params.Data), params.ID)
	if err != nil {
		return fmt.Errorf("update entity: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update entity: id %q not found", params.ID)
	}
	return nil
}

// DeleteEntity removes an entity by ID. Cascades to relationships.
func (q *Queries) DeleteEntity(ctx context.Context, id string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM entities WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete entity: %w", err)
	}
	return nil
}

// ListEntitiesByType returns all entities of a given type ordered by display_name, then id.
func (q *Queries) ListEntitiesByType(ctx context.Context, entityType string) ([]Entity, error) {
	const query = `
		SELECT id, type, display_name, data, created_at, updated_at
		FROM entities WHERE type=?
		ORDER BY display_name, id`
	rows, err := q.db.QueryContext(ctx, query, entityType)
	if err != nil {
		return nil, fmt.Errorf("list entities: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entities []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

// sanitizeFTSQuery escapes double-quotes and wraps input as a prefix phrase
// query safe for FTS5 MATCH expressions.
func sanitizeFTSQuery(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"*`
}

// SearchEntities performs a full-text prefix search over entity display names,
// returning results ordered by BM25 relevance rank.
// Returns nil, nil for a blank query.
func (q *Queries) SearchEntities(ctx context.Context, query string) ([]Entity, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	const stmt = `
		SELECT e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at
		FROM entities_fts f
		JOIN entities e ON e.id = f.entity_id
		WHERE entities_fts MATCH ?
		ORDER BY rank`
	rows, err := q.db.QueryContext(ctx, stmt, sanitizeFTSQuery(query))
	if err != nil {
		return nil, fmt.Errorf("search entities: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var entities []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan entity: %w", err)
		}
		entities = append(entities, e)
	}
	return entities, rows.Err()
}

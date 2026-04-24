# JSON API Design

**Date:** 2026-04-24
**Status:** Approved

## Goal

Add a JSON REST API under `/api/` for personal scripting and MCP server integration. All routes live alongside the existing HTMX web routes in `internal/web/`, registered in the same chi router.

## Architecture

New file `internal/web/handlers_api.go` in the existing `web` package. Routes registered under `/api/` prefix in `server.go`. Reuses existing DAL, helpers, and chi router — no new packages or structs.

All endpoints accept and return `application/json`. No auth — local tool.

## Routes

```
POST   /api/search-entities          Search entities by query + optional type filter
GET    /api/entities/{id}            Fetch entity by ID
POST   /api/entities                 Create entity
PUT    /api/entities/{id}            Update entity display_name and data
DELETE /api/entities/{id}            Delete entity (cascades relationships)

GET    /api/relationships/{id}       Fetch relationship by ID
POST   /api/relationships            Create relationship
PUT    /api/relationships/{id}       Update date_from, date_to, note
DELETE /api/relationships/{id}       Delete relationship
```

## Request/Response Shapes

### Entity object (all responses)
```json
{
  "id": "01KPWPP2ME0DWYGS7Z4VTQ8KR4",
  "type": "Person",
  "display_name": "Alice Example",
  "data": { "name": "Alice Example", "birthYear": 1990 },
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

### Relationship object (all responses)
```json
{
  "id": "01KPWPP2ME0DWYGS7Z4VTQ8KR5",
  "entity_a_id": "...",
  "entity_b_id": "...",
  "type": "knows",
  "date_from": "2020",
  "date_to": null,
  "note": null,
  "created_at": "2026-04-24T10:00:00Z",
  "updated_at": "2026-04-24T10:00:00Z"
}
```

### POST /api/search-entities
Request:
```json
{ "q": "alice", "type": "Person" }
```
- `q`: required, full-text prefix search (delegates to `SearchEntities`). Empty string returns `[]`.
- `type`: optional, filters results to that entity type.

Response `200`:
```json
{ "results": [ /* entity objects */ ] }
```

### POST /api/entities
Request:
```json
{ "type": "Person", "display_name": "Alice Example", "data": { "name": "Alice Example" } }
```
- `type`: required.
- `display_name`: optional string (must be non-empty if provided).
- `data`: required, validated against entity type JSON Schema.

Response `201`: entity object.

### PUT /api/entities/{id}
Request:
```json
{ "display_name": "Alice Updated", "data": { "name": "Alice Updated" } }
```
- `display_name`: optional string or explicit JSON `null` to clear it.
- `data`: required.

Response `200`: updated entity object.

### DELETE /api/entities/{id}
Response `204` (no body).

### POST /api/relationships
Request:
```json
{
  "entity_a_id": "...",
  "entity_b_id": "...",
  "type": "knows",
  "date_from": "2020",
  "date_to": null,
  "note": "met at conference"
}
```
- `entity_a_id`, `entity_b_id`, `type`: required.
- `date_from`, `date_to`, `note`: optional, null to omit.

Response `201`: relationship object.

### PUT /api/relationships/{id}
Request:
```json
{ "date_from": "2020-03", "date_to": null, "note": "updated note" }
```
All fields optional; null clears the field.

Response `200`: updated relationship object.

### DELETE /api/relationships/{id}
Response `204` (no body).

## Error Responses

All errors return `{"error": "human-readable message"}` with the appropriate HTTP status:

| Status | When |
|--------|------|
| 400    | Malformed JSON, missing required fields, schema validation failure |
| 404    | Entity or relationship not found |
| 500    | Unexpected server error |

## Implementation Notes

- `json.NewDecoder(r.Body).Decode(&req)` for request parsing; return 400 on decode error.
- `errors.Is(err, sql.ErrNoRows)` → 404, same pattern as web handlers.
- DAL schema validation errors (from `schema.ValidateEntity` / `schema.ValidateRelationship`) → 400.
- `w.Header().Set("Content-Type", "application/json")` before every response.
- Helper `apiError(w, status, msg)` and `apiJSON(w, status, v)` to avoid repetition.

## Testing

Integration tests in `internal/web/web_test.go` using `httptest`:
- `TestAPISearchEntities` — POST search, verify result count and type filter
- `TestAPIGetEntity` — GET existing entity, verify fields
- `TestAPICreateEntity` — POST person, verify 201 + body
- `TestAPIUpdateEntity` — PUT, verify updated fields
- `TestAPIDeleteEntity` — DELETE, verify 204 then GET returns 404
- `TestAPICreateRelationship` — POST, verify 201 + body
- `TestAPIUpdateRelationship` — PUT date_from/note, verify 200
- `TestAPIDeleteRelationship` — DELETE, verify 204

## File Changes

| File | Change |
|------|--------|
| `internal/web/handlers_api.go` | Create: all 9 API handlers + `apiError`/`apiJSON` helpers |
| `internal/web/server.go` | Add `/api/` route group |
| `internal/web/web_test.go` | Add 8 integration tests |

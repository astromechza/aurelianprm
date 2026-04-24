# Changelog

## Unreleased

### Phase 2 — HTMX Web UI

- Added `main.go` HTTP entrypoint: `--db` and `--addr` flags, `PORT` env var fallback
- Added chi v5 HTTP router with logger + recoverer middleware
- Person list page with live search (HTMX partial, `hx-push-url`)
- Person create page
- Person detail page with inline edit of header fields
- Contact entity sections (Email, Phone, Website, Social Network, Address, Career, Pet)
  - Inline add / edit / delete via HTMX partials
  - Optional `date_from` / `date_to` on every entity relationship
- People section (flat table of all person-to-person relationships)
  - Relation types: Knows, Friend, Spouse, Sibling, Parent Of, Child Of
  - Child Of is the inbound direction of `parentOf` (entity IDs swapped on create)
  - Inline add / edit / delete
  - Person search with Select button to populate hidden `person_b_id` field
- Clickable rows on person list (whole `<tr>`, delete button stops propagation)
- Pet birth date accepts partial ISO 8601 (YYYY / YYYY-MM / YYYY-MM-DD)
- All templates embedded in binary via `go:embed` — no build step
- Integration tests: `httptest` + in-memory SQLite covering all major routes
- `.claude/launch.json` dev server config with `autoPort` for preview

### Phase 1 — Data Layer

- SQLite database via `modernc.org/sqlite` with goose migrations
- `entities` table with JSON data + FTS5 full-text search index (porter stemmer)
- `relationships` table with partial ISO 8601 date range and symmetric-duplicate guard
- DAL: `CreateEntity`, `GetEntity`, `UpdateEntity`, `DeleteEntity`, `ListEntitiesByType`, `SearchEntities`
- DAL: `CreateRelationship`, `GetRelationship`, `UpdateRelationship`, `DeleteRelationship`, `GetNeighbours`, `GetNeighboursByRelType`
- JSON Schema validation for all entity types (Person, EmailAddress, PhoneNumber, WebSite, SocialNetwork, PostalAddress, Career, Pet)
- Relationship type registry with directionality and allowed entity pair constraints

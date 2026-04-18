# Relationship Improvements — Design Spec

**Date:** 2026-04-18  
**Scope:** Person-type relationship UX on person detail page

## Background

The current relationship add-form requires the user to paste a raw entity UUID into
a free-text field. Relationships also have no display for since/until dates or notes,
and no inline edit capability. This spec covers:

1. Since/until date fields (year+month+day precision) on the add and edit forms.
2. Notes field on relationships.
3. Inline edit/delete on relationship rows (same card-swap pattern as addresses).
4. Person picker: live-search input for person-type relationships instead of raw UUID.
5. "Create new person" inline mode (stub person with name+notes, then link).

Non-person relationship types (address, pet, work) keep the raw ID input for now.

---

## Data Model Changes

### No schema changes

`relationships` already has `label`, `since_id`, `until_id`, `notes`. These fields are
already in `CreateRelationship` insert but not surfaced in the add-form or displayed
in the row.

---

## Store Layer Changes (`internal/store/`)

### New: `PersonRelationshipRow`

```go
// PersonRelationshipRow is the denormalised result of a JOIN across relationships
// and their since/until blurry_dates. Raw integers are returned; formatting
// happens in the web layer.
type PersonRelationshipRow struct {
    Relationship  model.Relationship
    ToName        string  // resolved display name (person.name, pet.name, etc.)
    SinceYear     *int
    SinceMonth    *int
    SinceDay      *int
    UntilYear     *int
    UntilMonth    *int
    UntilDay      *int
}
```

### New: `ListPersonRelationships`

Replaces raw `ListRelationships` in the detail-page loader. Uses two LEFT JOINs on
`blurry_dates` (aliased `bd_since`, `bd_until`) to pull year/month/day alongside
each relationship row. **Does not** resolve `ToName` — that stays in the web layer
via `resolveEntityName`.

```sql
SELECT r.id, r.person_id, r.to_type, r.to_id, r.label,
       r.since_id, r.until_id, r.notes, r.created_at,
       bd_since.year, bd_since.month, bd_since.day,
       bd_until.year, bd_until.month, bd_until.day
FROM relationships r
LEFT JOIN blurry_dates bd_since ON bd_since.id = r.since_id
LEFT JOIN blurry_dates bd_until ON bd_until.id = r.until_id
WHERE r.person_id = ?
ORDER BY r.created_at
```

### New: `GetPersonRelationshipRow`

```go
func GetPersonRelationshipRow(ctx, db, personID, relID string) (PersonRelationshipRow, error)
```

Fetches a single row with the same JOIN. Returns `sql.ErrNoRows` if not found or
if the relationship's `person_id` doesn't match (security: prevents cross-person access).

### New: `UpdateRelationship`

```go
func UpdateRelationship(ctx, db, id, label string, notes *string, sinceID, untilID *string) error
```

Updates `label`, `notes`, `since_id`, `until_id` in a single UPDATE. Returns
`sql.ErrNoRows` if the row doesn't exist. The existing `UpdateRelationshipDates`
can be removed (absorbed into this function, used in address handlers).

> **Note:** `addresses.go` calls `UpdateRelationshipDates` directly. We keep
> `UpdateRelationshipDates` as-is to avoid churn in address handlers. `UpdateRelationship`
> is additive.

---

## Web Layer Changes (`internal/web/`)

### Updated `relationshipRow` (in `people.go`)

```go
type relationshipRow struct {
    Relationship model.Relationship
    PersonID     string
    ToName       string
    SinceDisplay string // formatted, empty if not set
    UntilDisplay string // formatted, empty if not set
}
```

`SinceDisplay` / `UntilDisplay` are built by a call to `formatBlurryDate` in
`buildPersonDetailData`, so the template stays logic-free.

### Updated `buildPersonDetailData`

Replace `store.ListRelationships` with `store.ListPersonRelationships`. For each row,
call `resolveEntityName` as before, then format since/until via `formatBlurryDate`.

### New: `relationshipEditData`

```go
type relationshipEditData struct {
    PersonID   string
    Row        store.PersonRelationshipRow
    SinceYear  string
    SinceMonth string
    SinceDay   string
    UntilYear  string
    UntilMonth string
    UntilDay   string
    Error      string
}
```

### New: `parseBlurryDateForm(r *http.Request, yearKey, monthKey, dayKey string) *model.BlurryDate`

Generalises `parseMonthYearForm` to also read a day field. Returns nil if all three
are blank. Existing `parseMonthYearForm` calls in `addresses.go` are left untouched
(addresses only use year+month).

### New file: `internal/web/relationships.go`

Handlers:

| Handler | Route | Method | Description |
|---------|-------|--------|-------------|
| `handleRelationshipCard` | `/people/{id}/relationships/{rel_id}` | GET | Returns `relationship_row.html` fragment (cancel target) |
| `handleRelationshipEditForm` | `/people/{id}/relationships/{rel_id}/edit` | GET | Returns `relationship_edit_form.html` fragment |
| `handleRelationshipUpdate` | `/people/{id}/relationships/{rel_id}/edit` | POST | Saves edits, returns `relationship_row.html` fragment |

`handleRelationshipDelete` stays in `people.go` but is moved to `relationships.go`
for consistency. Route changes from `/relationships/{id}` to
`/people/{id}/relationships/{rel_id}` so we have the personID for the card-fetch after
edit-cancel (consistent with address pattern). **Breaking change:** delete the old
route and register the new one.

`handleRelationshipCreate` is updated in place (still in `people.go` → move to
`relationships.go`):
- Reads `notes`, `since_year`/`since_month`/`since_day`, `until_year`/`until_month`/`until_day`
- If `to_type == "person"` and `create_mode == "new"`: calls `store.CreatePerson` with
  `name` + `notes` from form, uses resulting ID as `to_id`
- Creates since/until blurry_date rows via `createOptionalBlurryDate`
- Returns `relationship_row.html` fragment (not just the model row)

### New: `handlePersonSearch`

```
GET /people/search?name=...
```

HTMX-only endpoint. Calls `store.ListPeople(ctx, db, name)` (already exists).
Returns `person_picker_results.html` fragment: an ordered list of buttons, one per
matched person. Each button uses `hx-on:click` to set the value of the adjacent
hidden `to_id` input and update a "selected" label element. Requires `HX-Request`
header; returns 400 otherwise.

---

## Template Changes (`assets/templates/`)

### Updated `relationship_row.html`

```html
{{define "relationship_row.html"}}
<tr>
  <td>{{.Relationship.Label}}</td>
  <td>{{.Relationship.ToType}}</td>
  <td>{{.ToName}}</td>
  <td>
    {{if or .SinceDisplay .UntilDisplay}}
    <small>
      {{- if and .SinceDisplay .UntilDisplay}}{{.SinceDisplay}} – {{.UntilDisplay}}
      {{- else if .SinceDisplay}}since {{.SinceDisplay}}
      {{- else}}until {{.UntilDisplay}}
      {{- end}}
    </small>
    {{end}}
  </td>
  <td>{{if .Relationship.Notes}}{{deref .Relationship.Notes}}{{end}}</td>
  <td>
    <div class="flex-gap">
      <button hx-get="/people/{{.PersonID}}/relationships/{{.Relationship.ID}}/edit"
              hx-target="closest tr" hx-swap="outerHTML" class="outline">Edit</button>
      <button hx-delete="/people/{{.PersonID}}/relationships/{{.Relationship.ID}}"
              hx-target="closest tr" hx-swap="outerHTML"
              hx-confirm="Remove this relationship?"
              class="outline secondary">&#x2715;</button>
    </div>
  </td>
</tr>
{{end}}
```

### New `relationship_edit_form.html`

Inline edit row (replaces `<tr>` via outerHTML swap). Fields: label, notes,
since year/month/day, until year/month/day. Save → POST to edit endpoint.
Cancel → GET card endpoint.

### New `person_picker_results.html`

Fragment returned by `/people/search`. A `<ul>` of person name buttons. Each button:
- Sets `to_id` hidden input value to the person's ID
- Sets a `#person-search-selected` span text to the person's name
- Clears the search results list

### Updated `people_detail.html` — Relationships section

Replace current table headers to add Since, Until, Notes columns.

Replace the add-form with an accordion `<details>`. The form has:

**For all types:** label (required), notes, since year/month/day, until year/month/day.

**`to_type` select** drives conditional rendering:
- `person`: shows live-search block (search input + results div + hidden `to_id` + "Create new" radio toggle)
- Other types: shows plain `to_id` text input

The "Create new person" toggle (shown only when `to_type == person`):
- Radio: "Link existing" | "Create new person" 
- When "Create new": show `new_person_name` (required) and `new_person_notes` inputs; hide search block
- When "Link existing": show search block; hide new-person fields
- Hidden field `create_mode` = `"existing"` or `"new"` to tell the handler

HTMX swaps for `to_type` switching: `hx-on:change` on the select triggers a JS
snippet that shows/hides the relevant block (no server round-trip needed for a simple
show/hide).

---

## Implementation Tasks

### Task 1: Store layer
- `PersonRelationshipRow`, `ListPersonRelationships`, `GetPersonRelationshipRow`
- `UpdateRelationship`
- Tests for all new store functions (real SQLite, `package store_test`)

### Task 2: Template data types and templates
- Update `relationshipRow` struct, add `relationshipEditData`
- Add `parseBlurryDateForm` helper
- Create `relationship_edit_form.html`, `person_picker_results.html`
- Update `relationship_row.html`, `people_detail.html` (relationship section only)
- Update `buildPersonDetailData` to use `ListPersonRelationships`

### Task 3: Handlers and routes
- `handleRelationshipCard`, `handleRelationshipEditForm`, `handleRelationshipUpdate`
- `handlePersonSearch`
- Updated `handleRelationshipCreate` (notes, dates, create-new flow)
- Updated `handleRelationshipDelete` (new route with personID)
- Register all new routes; deregister old `/relationships/{id}` DELETE route
- Handler tests with `net/http/httptest`

### Task 4: Final verification
- `go build ./...`
- `golangci-lint run`
- `go test -race -cover ./...`
- CHANGELOG.md updated

---

## Out of Scope

- Edit/delete for non-person relationship targets (pet, work) via picker — raw ID input is acceptable for now
- Relationship notes on the reminder digest
- Bulk relationship management

# HTMX Frontend Design

**Date:** 2026-04-22
**Status:** Approved

## Goal

Add a web UI to aurelianprm using HTMX + Pico CSS served by a Go HTTP server (chi). Three main pages: Person list/search, Person create, Person detail with grouped relationships and inline edit/add/delete.

## Architecture

Single binary. `main.go` parses a `--db` flag, opens the SQLite database via `internal/db.Open`, constructs a `web.Server`, and calls `http.ListenAndServe`.

New dependency (requires approval): `github.com/go-chi/chi/v5`

All templates embedded in the binary via `go:embed`. No build step, no external assets beyond CDN-loaded HTMX and Pico CSS.

## File Structure

```
main.go                          # --db flag, db.Open, web.Server, ListenAndServe
internal/web/
  server.go                      # Server struct, NewServer, chi router wiring, middleware
  handlers_persons.go            # person list, search, create, detail, update, delete
  handlers_entities.go           # add/edit/delete contact entities
  handlers_relationships.go      # add/edit/delete person-to-person relationships
  viewmodels.go                  # typed view-model structs for templates
  templates/
    layout.html                  # base layout: Pico CSS CDN, HTMX CDN, nav bar
    persons_list.html            # search box + results, "New Person" button
    persons_create.html          # Person creation form
    persons_detail.html          # person header + grouped entity sections
    partials/
      persons_rows.html          # HTMX partial: table rows for search results
      entity_section.html        # one grouped section (e.g. "Email Addresses")
      entity_row.html            # single entity row with edit/delete
      entity_form.html           # add/edit contact entity form (parametric by type)
      relationship_row.html      # person-to-person row with edit/delete
      relationship_form.html     # search existing person + choose rel type form
      person_edit_form.html      # inline edit form for person header fields
```

## Routes

```
GET  /                           → redirect /persons
GET  /persons                    → full page (persons_list.html)
GET  /persons?q=alice            → HX-Request header present: partial persons_rows.html
                                   otherwise: full page with pre-populated results
GET  /persons/new                → full page (persons_create.html)
POST /persons                    → create → redirect /persons/{id}
GET  /persons/{id}               → full page (persons_detail.html)
PUT  /persons/{id}               → update person fields → redirect /persons/{id}
DELETE /persons/{id}             → delete → redirect /persons

POST   /persons/{id}/entities    → create entity + relationship → partial entity_section.html
GET    /entities/{eid}/edit      → inline edit form partial (entity_form.html in edit mode)
PUT    /entities/{eid}           → update entity data → partial entity_row.html
DELETE /entities/{eid}           → delete entity (cascade rel) → empty 200 (row removed via hx-swap)

POST   /persons/{id}/relationships  → link existing person → partial relationship_row.html
GET    /relationships/{rid}/edit    → inline edit form partial
PUT    /relationships/{rid}         → update date_from/date_to/note → partial relationship_row.html
DELETE /relationships/{rid}         → delete → empty 200
```

## HTMX Patterns

**Search:** Input with `hx-get="/persons"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#person-results"`, `hx-push-url="true"`. Server returns `persons_rows.html` partial when `HX-Request` header is present.

**Add contact entity:** Each type-section embeds an add form inside a `<details>` element (always in DOM, no round-trip needed). Form has a hidden `type` field. Submits via `hx-post="/persons/{id}/entities"`. Response returns updated `entity_section.html` replacing the whole section (collapses the `<details>` back).

**Edit entity:** Edit button on a row triggers `hx-get="/entities/{eid}/edit"` → swaps `entity_row.html` with an inline edit form. Save via `hx-put` → returns updated `entity_row.html`.

**Delete entity/relationship:** `hx-delete` + `hx-confirm="Delete?"` + `hx-target="closest [data-row]"` + `hx-swap="outerHTML"` with empty response removes the element.

**Person-to-person link:** Section has a search input: `hx-get="/persons?q=..."` → returns `persons_rows.html` with each row having a "Select" button. Clicking "Select" populates a hidden `person_b_id` field and shows the chosen name. User then picks rel type from a dropdown and submits via `hx-post="/persons/{id}/relationships"` → returns `relationship_row.html` partial appended to the list.

**Edit person header:** Edit button swaps person header with `person_edit_form.html` via `hx-get`. Submit via `hx-put=/persons/{id}` → redirect to full detail page.

## View Models

```go
// PersonListView — persons_list.html
type PersonListView struct {
    Query   string
    Persons []dal.Entity
}

// PersonDetailView — persons_detail.html
type PersonDetailView struct {
    Person   dal.Entity
    Sections []EntitySection
    People   []PersonRelSection
}

// EntitySection — one grouped block of contact entities
type EntitySection struct {
    RelType  string   // e.g. "hasEmail"
    Label    string   // e.g. "Email Addresses"
    Entities []EntityWithRel
}

type EntityWithRel struct {
    Entity       dal.Entity
    Relationship dal.Relationship
}

// PersonRelSection — person-to-person relationships
type PersonRelSection struct {
    RelType      string
    Label        string
    Persons      []PersonWithRel
}

type PersonWithRel struct {
    Person       dal.Entity
    Relationship dal.Relationship
    Direction    string // "outbound" or "inbound"
}
```

## Pages

### Person List (`/persons`)

- Search input at top; pre-populated from `?q=` query param
- Results table: Name | Nickname | Birth Year | View link | Delete button
- "New Person" link → `/persons/new`
- Empty-state row when no results

### Person Create (`/persons/new`)

Full-page form with:
- Name (required text input)
- Nickname (optional)
- Birth Year / Birth Month / Birth Day (three optional number inputs on one row)
- Note (optional textarea)
- Submit → POST /persons → redirect /persons/{id}
- Cancel → link back to /persons

### Person Detail (`/persons/{id}`)

**Header section:** Name, nickname, birth info, note. Edit button → inline form swap.

**Grouped entity sections** (one `<article>` per group, only shown if has entries or always shown with "Add" button):
- Email Addresses (`hasEmail`)
- Phone Numbers (`hasPhone`)
- Websites (`hasWebSite`)
- Social Networks (`hasSocialNetwork`)
- Postal Addresses (`hasAddress`)
- Career (`hasCareer`)
- Pets (`hasPet`)

Each section shows a list of entity rows. Each row: display fields + Edit + Delete. Below list: "Add" button/form.

**People section:** Person-to-person relationships grouped by type (knows, friendOf, spouseOf, siblingOf, parentOf). Each shows linked person name (clickable → their detail page) + dates + note + Edit + Delete. "Link person" form: search input → selectable results → rel type dropdown → Submit.

## Error Handling

- Handler errors: log to stderr, render `500` plain text for HTMX partials, full error page for full requests
- Validation errors: re-render form with `<small class="error">` message near the field
- Not found: 404 page
- All form POSTs use `application/x-www-form-urlencoded` (standard HTML forms, no JSON)

## Template Conventions

- `{{define "layout"}}` in `layout.html` with `{{block "content" .}}` placeholder
- Full-page templates: `{{template "layout" .}}` wrapper
- Partials: standalone `{{define "partial-name"}}` templates, executed via `t.ExecuteTemplate`
- HTMX detection in handlers: `r.Header.Get("HX-Request") == "true"`
- Pico CSS: `<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">`
- HTMX: `<script src="https://unpkg.com/htmx.org@2"></script>`
- No inline styles. Use Pico CSS semantic HTML classes and `aria-*` attributes.

## Chi Middleware

```go
r.Use(middleware.Logger)
r.Use(middleware.Recoverer)
```

No auth — local tool, single user.

## Testing

Handler integration tests using `httptest.NewRecorder` and an in-memory SQLite DB (via `db.Open(":memory:")`). Test at least: list returns 200, search returns partial, create redirects, detail returns 200, add entity returns partial, delete returns 200.

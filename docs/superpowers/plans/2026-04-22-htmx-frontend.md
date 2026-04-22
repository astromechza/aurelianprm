# HTMX Frontend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a three-page HTMX + Pico CSS web UI (person list/search, person create, person detail with inline entity/relationship management) served by a Go/chi HTTP server.

**Architecture:** Single binary; `main.go` parses `--db`, opens SQLite via `db.Open`, constructs `web.Server`, and calls `http.ListenAndServe`. All HTML is embedded via `go:embed`; no build step. Templates split into `layout-head`/`layout-foot` helpers (avoids the shared `content` block collision in `html/template`) so each page template is a self-contained named template.

**Tech Stack:** Go 1.26, `github.com/go-chi/chi/v5`, `html/template`, HTMX 2 + Pico CSS 2 (CDN), `modernc.org/sqlite` (already present), `httptest` for integration tests.

---

## File Map

| File | Role |
|---|---|
| `main.go` | `--db` flag, `db.Open`, `web.NewServer`, `http.ListenAndServe` |
| `internal/dal/relationships.go` | Add `GetRelationship(ctx, id)` method |
| `internal/web/server.go` | `Server` struct, `NewServer`, `Handler()` with chi router + middleware, error helpers |
| `internal/web/viewmodels.go` | All view-model structs + decode helpers |
| `internal/web/helpers.go` | `parseEntityFormData`, `relTypeForEntityType`, `labelForRelType`, `nilOrStr`, `strOrEmpty` |
| `internal/web/handlers_persons.go` | List, search, create, detail, update, delete person handlers |
| `internal/web/handlers_entities.go` | Create, edit-form, update, delete contact entity handlers |
| `internal/web/handlers_relationships.go` | Create, edit-form, update, delete person↔person relationship handlers |
| `internal/web/templates/layout.html` | `layout-head` and `layout-foot` template blocks |
| `internal/web/templates/persons_list.html` | Full page: search input + results table |
| `internal/web/templates/persons_create.html` | Full page: person creation form |
| `internal/web/templates/persons_detail.html` | Full page: person header + entity sections + people sections |
| `internal/web/templates/partials/persons_rows.html` | HTMX partial: `<tr>` rows for person search results |
| `internal/web/templates/partials/person_edit_form.html` | HTMX partial: inline edit form for person header |
| `internal/web/templates/partials/entity_section.html` | HTMX partial: one grouped section (list + add form) |
| `internal/web/templates/partials/entity_row.html` | HTMX partial: single entity row with Edit/Delete |
| `internal/web/templates/partials/entity_form.html` | HTMX partial: parametric add/edit form for contact entities |
| `internal/web/templates/partials/relationship_row.html` | HTMX partial: one person↔person row with Edit/Delete |
| `internal/web/templates/partials/relationship_form.html` | HTMX partial: search + select person + rel-type form |
| `internal/web/web_test.go` | Integration tests using `httptest` + in-memory SQLite |

---

## Task 1: Add chi/v5 dependency and `GetRelationship` DAL method

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `internal/dal/relationships.go`

- [ ] **Step 1: Add chi/v5 and run go get**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm2
go get github.com/go-chi/chi/v5@latest
```

Expected: `go.mod` now contains `github.com/go-chi/chi/v5` in `require`.

- [ ] **Step 2: Add `GetRelationship` to the DAL**

Append to `internal/dal/relationships.go` (before the final closing of the file):

```go
// GetRelationship fetches a single relationship by ID. Returns sql.ErrNoRows if not found.
func (q *Queries) GetRelationship(ctx context.Context, id string) (Relationship, error) {
	const query = `
		SELECT id, entity_a_id, entity_b_id, type, date_from, date_to, note, created_at, updated_at
		FROM relationships WHERE id = ?`
	r, err := scanRelationship(q.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return Relationship{}, fmt.Errorf("get relationship: %w", err)
	}
	return r, nil
}
```

- [ ] **Step 3: Verify build**

```bash
go build ./...
```

Expected: exits 0, no errors.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/dal/relationships.go
git commit -m "feat: add chi/v5 dependency and GetRelationship DAL method"
```

---

## Task 2: View models (`internal/web/viewmodels.go`)

**Files:**
- Create: `internal/web/viewmodels.go`

- [ ] **Step 1: Create the file**

```go
package web

import (
	"encoding/json"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// PersonListItem is a decoded row for the persons list/search page.
type PersonListItem struct {
	ID        string
	Name      string
	NickName  string
	BirthYear int
}

// PersonListView is the view model for the persons list page.
type PersonListView struct {
	Query   string
	Persons []PersonListItem
}

// PersonData holds decoded JSON fields of a Person entity.
type PersonData struct {
	Name       string `json:"name"`
	NickName   string `json:"nickName"`
	BirthYear  int    `json:"birthYear"`
	BirthMonth int    `json:"birthMonth"`
	BirthDay   int    `json:"birthDay"`
	Note       string `json:"note"`
}

// EntityWithRel pairs a contact entity with its connecting relationship and decoded data.
type EntityWithRel struct {
	Entity       dal.Entity
	Relationship dal.Relationship
	DataMap      map[string]any
}

// EntitySection is one grouped block of contact entities shown on the person detail page.
type EntitySection struct {
	PersonID   string
	RelType    string
	Label      string
	EntityType string // e.g. "EmailAddress"
	Entities   []EntityWithRel
}

// PersonWithRel pairs a person entity with its connecting relationship.
type PersonWithRel struct {
	Person       dal.Entity
	Relationship dal.Relationship
	Direction    string // "outbound" or "inbound"
}

// PersonRelSection groups person-to-person relationships by type.
type PersonRelSection struct {
	PersonID string
	RelType  string
	Label    string
	Persons  []PersonWithRel
}

// PersonDetailView is the view model for the person detail page.
type PersonDetailView struct {
	Person     dal.Entity
	PersonData PersonData
	Sections   []EntitySection
	People     []PersonRelSection
}

// EntityFormView is the view model for the add/edit contact entity partial.
type EntityFormView struct {
	PersonID string // for create mode (form action)
	EntityID string // for edit mode
	RelID    string // for edit mode (unused in form but available)
	Type     string // entity type e.g. "EmailAddress"
	DataMap  map[string]any
	EditMode bool
	Error    string
}

// RelationshipRowView is the view model for a single person-to-person relationship row partial.
type RelationshipRowView struct {
	PersonID      string
	PersonWithRel PersonWithRel
}

// RelationshipFormView is the view model for the add/edit person relationship partial.
type RelationshipFormView struct {
	PersonID    string
	RelID       string
	EditMode    bool
	DateFrom    string
	DateTo      string
	Note        string
	OtherPerson *dal.Entity
	Error       string
}

// decodeDataMap decodes entity JSON data into a plain map for template use.
func decodeDataMap(data json.RawMessage) map[string]any {
	var m map[string]any
	_ = json.Unmarshal(data, &m)
	if m == nil {
		m = make(map[string]any)
	}
	return m
}

// decodePersonData decodes a Person entity's JSON data into a typed struct.
func decodePersonData(data json.RawMessage) PersonData {
	var p PersonData
	_ = json.Unmarshal(data, &p)
	return p
}

// strOrEmpty dereferences a *string, returning "" for nil.
func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./internal/web/...
```

Expected: exits 0 (package compiles, even though no .go files besides this one yet — add a stub `package web` file if needed, but viewmodels.go alone is fine as long as server.go is added next).

Note: `go build ./internal/web/...` may warn about no Go files yet if this is the only file — that's fine for now; it will compile once server.go is added.

- [ ] **Step 3: Commit**

```bash
git add internal/web/viewmodels.go
git commit -m "feat: add web view models"
```

---

## Task 3: Server + helpers (`internal/web/server.go`, `internal/web/helpers.go`)

**Files:**
- Create: `internal/web/server.go`
- Create: `internal/web/helpers.go`

- [ ] **Step 1: Create `internal/web/helpers.go`**

```go
package web

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// entityRelTypes defines ordered display of contact entity sections.
var entityRelTypes = []struct {
	RelType    string
	EntityType string
	Label      string
}{
	{"hasEmail", "EmailAddress", "Email Addresses"},
	{"hasPhone", "PhoneNumber", "Phone Numbers"},
	{"hasWebSite", "WebSite", "Websites"},
	{"hasSocialNetwork", "SocialNetwork", "Social Networks"},
	{"hasAddress", "PostalAddress", "Postal Addresses"},
	{"hasCareer", "Career", "Career"},
	{"hasPet", "Pet", "Pets"},
}

// personRelTypes defines ordered display of person-to-person sections.
var personRelTypes = []struct {
	RelType string
	Label   string
}{
	{"knows", "Knows"},
	{"friendOf", "Friends"},
	{"spouseOf", "Spouse"},
	{"siblingOf", "Siblings"},
	{"parentOf", "Parent Of"},
}

// relTypeForEntityType maps an entity type to its relationship type.
func relTypeForEntityType(entityType string) string {
	for _, rt := range entityRelTypes {
		if rt.EntityType == entityType {
			return rt.RelType
		}
	}
	return ""
}

// labelForRelType returns the display label for a relationship type.
func labelForRelType(relType string) string {
	for _, rt := range entityRelTypes {
		if rt.RelType == relType {
			return rt.Label
		}
	}
	for _, rt := range personRelTypes {
		if rt.RelType == relType {
			return rt.Label
		}
	}
	return relType
}

// entityTypeForRelType maps a relationship type to its entity type.
func entityTypeForRelType(relType string) string {
	for _, rt := range entityRelTypes {
		if rt.RelType == relType {
			return rt.EntityType
		}
	}
	return ""
}

// nilOrStr returns nil for empty string, otherwise a pointer to the string.
func nilOrStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// parseEntityFormData parses form values into validated JSON for the given entity type.
func parseEntityFormData(entityType string, r *http.Request) (json.RawMessage, error) {
	data := make(map[string]any)
	switch entityType {
	case "EmailAddress":
		email := r.FormValue("email")
		if email == "" {
			return nil, fmt.Errorf("email is required")
		}
		data["email"] = email
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "PhoneNumber":
		telephone := r.FormValue("telephone")
		if telephone == "" {
			return nil, fmt.Errorf("telephone is required")
		}
		data["telephone"] = telephone
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "WebSite":
		u := r.FormValue("url")
		if u == "" {
			return nil, fmt.Errorf("url is required")
		}
		data["url"] = u
		if title := r.FormValue("title"); title != "" {
			data["title"] = title
		}
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "SocialNetwork":
		platform := r.FormValue("platform")
		if platform == "" {
			return nil, fmt.Errorf("platform is required")
		}
		data["platform"] = platform
		if handle := r.FormValue("handle"); handle != "" {
			data["handle"] = handle
		}
		if u := r.FormValue("url"); u != "" {
			data["url"] = u
		}
	case "PostalAddress":
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
		if country := r.FormValue("country"); country != "" {
			data["country"] = country
		}
		if city := r.FormValue("city"); city != "" {
			data["city"] = city
		}
		if street := r.FormValue("streetAddress"); street != "" {
			data["streetAddress"] = street
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("at least one address field is required")
		}
	case "Career":
		role := r.FormValue("role")
		org := r.FormValue("organization")
		if role == "" || org == "" {
			return nil, fmt.Errorf("role and organization are required")
		}
		data["role"] = role
		data["organization"] = org
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	case "Pet":
		name := r.FormValue("name")
		if name == "" {
			return nil, fmt.Errorf("name is required")
		}
		data["name"] = name
		if species := r.FormValue("species"); species != "" {
			data["species"] = species
		}
		if bd := r.FormValue("birthDate"); bd != "" {
			data["birthDate"] = bd
		}
		if label := r.FormValue("label"); label != "" {
			data["label"] = label
		}
	default:
		return nil, fmt.Errorf("unknown entity type %q", entityType)
	}
	return json.Marshal(data)
}
```

- [ ] **Step 2: Create `internal/web/server.go`**

```go
package web

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/astromechza/aurelianprm/internal/dal"
)

//go:embed templates
var templatesFS embed.FS

// Server holds application state and serves HTTP.
type Server struct {
	dal  *dal.DAL
	tmpl *template.Template
}

// NewServer constructs a Server, parsing all embedded templates.
func NewServer(d *dal.DAL) (*Server, error) {
	tmpl, err := template.New("").ParseFS(
		templatesFS,
		"templates/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{dal: d, tmpl: tmpl}, nil
}

// Handler returns the chi router with all routes registered.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/persons", http.StatusFound)
	})

	r.Get("/persons", s.handlePersonsList)
	r.Get("/persons/new", s.handlePersonsNew)
	r.Post("/persons", s.handlePersonsCreate)
	r.Get("/persons/{id}", s.handlePersonsDetail)
	r.Put("/persons/{id}", s.handlePersonsUpdate)
	r.Delete("/persons/{id}", s.handlePersonsDelete)

	r.Post("/persons/{id}/entities", s.handleEntitiesCreate)
	r.Get("/entities/{eid}/edit", s.handleEntitiesEditForm)
	r.Put("/entities/{eid}", s.handleEntitiesUpdate)
	r.Delete("/entities/{eid}", s.handleEntitiesDelete)

	r.Post("/persons/{id}/relationships", s.handleRelationshipsCreate)
	r.Get("/relationships/{rid}/edit", s.handleRelationshipsEditForm)
	r.Put("/relationships/{rid}", s.handleRelationshipsUpdate)
	r.Delete("/relationships/{rid}", s.handleRelationshipsDelete)

	return r
}

// render executes a named template, writing to w. Errors are logged.
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		log.Printf("template %q: %v", name, err)
	}
}

// serverError logs err and responds with a plain 500.
func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("handler error: %v", err)
	if r.Header.Get("HX-Request") == "true" {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	http.Error(w, "Internal Server Error", http.StatusInternalServerError)
}

// isHTMX returns true when the request was triggered by HTMX.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}
```

- [ ] **Step 3: Create placeholder handler files so the package compiles**

Create `internal/web/handlers_persons.go`:

```go
package web

import "net/http"

func (s *Server) handlePersonsList(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handlePersonsNew(w http.ResponseWriter, r *http.Request)    {}
func (s *Server) handlePersonsCreate(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handlePersonsDetail(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handlePersonsUpdate(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handlePersonsDelete(w http.ResponseWriter, r *http.Request) {}
```

Create `internal/web/handlers_entities.go`:

```go
package web

import "net/http"

func (s *Server) handleEntitiesCreate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleEntitiesEditForm(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleEntitiesUpdate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleEntitiesDelete(w http.ResponseWriter, r *http.Request)   {}
```

Create `internal/web/handlers_relationships.go`:

```go
package web

import "net/http"

func (s *Server) handleRelationshipsCreate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleRelationshipsEditForm(w http.ResponseWriter, r *http.Request) {}
func (s *Server) handleRelationshipsUpdate(w http.ResponseWriter, r *http.Request)   {}
func (s *Server) handleRelationshipsDelete(w http.ResponseWriter, r *http.Request)   {}
```

- [ ] **Step 4: Create minimal template stubs so `ParseFS` succeeds**

Create `internal/web/templates/layout.html`:
```html
{{define "layout-head"}}<!DOCTYPE html><html lang="en"><head><meta charset="UTF-8"><title>AurelianPRM</title></head><body><main class="container">{{end}}
{{define "layout-foot"}}</main></body></html>{{end}}
```

Create `internal/web/templates/persons_list.html`:
```html
{{define "persons-list"}}{{template "layout-head" .}}persons-list{{template "layout-foot" .}}{{end}}
```

Create `internal/web/templates/persons_create.html`:
```html
{{define "persons-create"}}{{template "layout-head" .}}persons-create{{template "layout-foot" .}}{{end}}
```

Create `internal/web/templates/persons_detail.html`:
```html
{{define "persons-detail"}}{{template "layout-head" .}}persons-detail{{template "layout-foot" .}}{{end}}
```

Create `internal/web/templates/partials/persons_rows.html`:
```html
{{define "persons-rows"}}{{end}}
```

Create `internal/web/templates/partials/person_edit_form.html`:
```html
{{define "person-edit-form"}}{{end}}
```

Create `internal/web/templates/partials/entity_section.html`:
```html
{{define "entity-section"}}{{end}}
```

Create `internal/web/templates/partials/entity_row.html`:
```html
{{define "entity-row"}}{{end}}
```

Create `internal/web/templates/partials/entity_form.html`:
```html
{{define "entity-form"}}{{end}}
```

Create `internal/web/templates/partials/relationship_row.html`:
```html
{{define "relationship-row"}}{{end}}
```

Create `internal/web/templates/partials/relationship_form.html`:
```html
{{define "relationship-form"}}{{end}}
```

- [ ] **Step 5: Verify build**

```bash
go build ./...
```

Expected: exits 0.

- [ ] **Step 6: Commit**

```bash
git add internal/web/
git commit -m "feat: add web server scaffold with stub handlers and templates"
```

---

## Task 4: Layout template and test harness

**Files:**
- Modify: `internal/web/templates/layout.html`
- Create: `internal/web/web_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/web/web_test.go`:

```go
package web_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServer(t *testing.T) *web.Server {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)
	return s
}

func doGet(t *testing.T, s *web.Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doGetHX(t *testing.T, s *web.Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doPost(t *testing.T, s *web.Server, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

func doMethod(t *testing.T, s *web.Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	return w
}

// createTestPerson inserts a Person entity directly via the DAL and returns its ID.
func createTestPerson(t *testing.T, sqlDB *sql.DB, name string) string {
	t.Helper()
	d := dal.New(sqlDB)
	var id string
	err := d.WithTx(t.Context(), func(q *dal.Queries) error {
		e, err := q.CreateEntity(t.Context(), dal.CreateEntityParams{
			Type: "Person",
			Data: []byte(`{"name":"` + name + `"}`),
		})
		if err != nil {
			return err
		}
		id = e.ID
		return nil
	})
	require.NoError(t, err)
	return id
}

func TestRootRedirects(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/")
	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/persons", w.Header().Get("Location"))
}
```

- [ ] **Step 2: Run the test to verify it fails or passes with current state**

```bash
go test ./internal/web/... -run TestRootRedirects -v
```

Expected: PASS (redirect is already in server.go).

- [ ] **Step 3: Replace `layout.html` with the real implementation**

Replace `internal/web/templates/layout.html`:

```html
{{define "layout-head"}}
<!DOCTYPE html>
<html lang="en" data-theme="light">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>AurelianPRM</title>
  <link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/@picocss/pico@2/css/pico.min.css">
  <script src="https://unpkg.com/htmx.org@2" defer></script>
</head>
<body>
<header class="container-fluid">
  <nav>
    <ul>
      <li><strong><a href="/persons" class="contrast">AurelianPRM</a></strong></li>
    </ul>
    <ul>
      <li><a href="/persons">People</a></li>
    </ul>
  </nav>
</header>
<main class="container">
{{end}}

{{define "layout-foot"}}
</main>
</body>
</html>
{{end}}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./internal/web/... -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/templates/layout.html internal/web/web_test.go
git commit -m "feat: add layout template and test harness"
```

---

## Task 5: Person list and search

**Files:**
- Modify: `internal/web/handlers_persons.go`
- Modify: `internal/web/templates/persons_list.html`
- Modify: `internal/web/templates/partials/persons_rows.html`
- Modify: `internal/web/web_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/web/web_test.go`:

```go
func TestPersonsList_Empty(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "People")
	assert.Contains(t, w.Body.String(), "New Person")
}

func TestPersonsList_ShowsPersons(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	createTestPerson(t, sqlDB, "Alice Example")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGet(t, s, "/persons")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Alice Example")
}

func TestPersonsSearch_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	createTestPerson(t, sqlDB, "Bob Builder")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGetHX(t, s, "/persons?q=Bob")
	assert.Equal(t, http.StatusOK, w.Code)
	// Partial must not contain full layout
	assert.NotContains(t, w.Body.String(), "<!DOCTYPE")
	assert.Contains(t, w.Body.String(), "Bob Builder")
}
```

- [ ] **Step 2: Run to verify tests fail**

```bash
go test ./internal/web/... -run "TestPersonsList|TestPersonsSearch" -v
```

Expected: FAIL — handlers return empty response.

- [ ] **Step 3: Implement person list/search handlers in `internal/web/handlers_persons.go`**

Replace the entire file:

```go
package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/astromechza/aurelianprm/internal/dal"
)

func (s *Server) handlePersonsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	q := r.URL.Query().Get("q")

	var persons []dal.Entity
	err := s.dal.WithTx(ctx, func(queries *dal.Queries) error {
		var e error
		if q == "" {
			persons, e = queries.ListEntitiesByType(ctx, "Person")
		} else {
			all, searchErr := queries.SearchEntities(ctx, q)
			if searchErr != nil {
				return searchErr
			}
			for _, p := range all {
				if p.Type == "Person" {
					persons = append(persons, p)
				}
			}
		}
		return e
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	items := make([]PersonListItem, 0, len(persons))
	for _, e := range persons {
		pd := decodePersonData(e.Data)
		items = append(items, PersonListItem{
			ID:        e.ID,
			Name:      pd.Name,
			NickName:  pd.NickName,
			BirthYear: pd.BirthYear,
		})
	}
	view := PersonListView{Query: q, Persons: items}

	if isHTMX(r) {
		s.render(w, "persons-rows", view)
	} else {
		s.render(w, "persons-list", view)
	}
}

func (s *Server) handlePersonsNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, "persons-create", nil)
}

func (s *Server) handlePersonsCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		// Re-render form with error
		s.render(w, "persons-create", map[string]any{"Error": "Name is required"})
		return
	}

	data := map[string]any{"name": name}
	if v := r.FormValue("nickName"); v != "" {
		data["nickName"] = v
	}
	if v := r.FormValue("note"); v != "" {
		data["note"] = v
	}
	addIntField(data, "birthYear", r.FormValue("birthYear"))
	addIntField(data, "birthMonth", r.FormValue("birthMonth"))
	addIntField(data, "birthDay", r.FormValue("birthDay"))

	raw, err := json.Marshal(data)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	var id string
	err = s.dal.WithTx(ctx, func(q *dal.Queries) error {
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: raw,
		})
		if err != nil {
			return err
		}
		id = e.ID
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/persons/"+id, http.StatusSeeOther)
}

func (s *Server) handlePersonsDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")

	var view PersonDetailView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		person, err := q.GetEntity(ctx, id)
		if err != nil {
			return err
		}
		if person.Type != "Person" {
			return fmt.Errorf("entity %q is not a Person", id)
		}
		view.Person = person
		view.PersonData = decodePersonData(person.Data)

		neighbours, err := q.GetNeighbours(ctx, id)
		if err != nil {
			return err
		}

		// Partition neighbours into contact entities and person-to-person
		entityByRelType := make(map[string][]dal.NeighbourResult)
		personByRelType := make(map[string][]dal.NeighbourResult)
		for _, nr := range neighbours {
			if nr.Entity.Type == "Person" {
				personByRelType[nr.Relationship.Type] = append(personByRelType[nr.Relationship.Type], nr)
			} else {
				entityByRelType[nr.Relationship.Type] = append(entityByRelType[nr.Relationship.Type], nr)
			}
		}

		for _, rt := range entityRelTypes {
			section := EntitySection{
				PersonID:   id,
				RelType:    rt.RelType,
				EntityType: rt.EntityType,
				Label:      rt.Label,
			}
			for _, nr := range entityByRelType[rt.RelType] {
				section.Entities = append(section.Entities, EntityWithRel{
					Entity:       nr.Entity,
					Relationship: nr.Relationship,
					DataMap:      decodeDataMap(nr.Entity.Data),
				})
			}
			view.Sections = append(view.Sections, section)
		}

		for _, rt := range personRelTypes {
			section := PersonRelSection{
				PersonID: id,
				RelType:  rt.RelType,
				Label:    rt.Label,
			}
			for _, nr := range personByRelType[rt.RelType] {
				section.Persons = append(section.Persons, PersonWithRel{
					Person:       nr.Entity,
					Relationship: nr.Relationship,
					Direction:    nr.Direction,
				})
			}
			view.People = append(view.People, section)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.render(w, "persons-detail", view)
}

func (s *Server) handlePersonsUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	if name == "" {
		http.Error(w, "name is required", http.StatusUnprocessableEntity)
		return
	}

	data := map[string]any{"name": name}
	if v := r.FormValue("nickName"); v != "" {
		data["nickName"] = v
	}
	if v := r.FormValue("note"); v != "" {
		data["note"] = v
	}
	addIntField(data, "birthYear", r.FormValue("birthYear"))
	addIntField(data, "birthMonth", r.FormValue("birthMonth"))
	addIntField(data, "birthDay", r.FormValue("birthDay"))

	raw, err := json.Marshal(data)
	if err != nil {
		s.serverError(w, r, err)
		return
	}

	err = s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateEntity(ctx, dal.UpdateEntityParams{ID: id, Data: raw})
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/persons/"+id, http.StatusSeeOther)
}

func (s *Server) handlePersonsDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, id)
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	http.Redirect(w, r, "/persons", http.StatusSeeOther)
}

// addIntField parses s as an integer and adds it to data[key] if non-empty and valid.
func addIntField(data map[string]any, key, s string) {
	if s == "" {
		return
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err == nil {
		data[key] = v
	}
}
```

- [ ] **Step 4: Implement `persons_list.html`**

Replace `internal/web/templates/persons_list.html`:

```html
{{define "persons-list"}}
{{template "layout-head" .}}
<article>
  <header>
    <div style="display:flex;justify-content:space-between;align-items:center;">
      <h2 style="margin:0;">People</h2>
      <a href="/persons/new" role="button">+ New Person</a>
    </div>
  </header>
  <input
    type="search"
    name="q"
    placeholder="Search by name…"
    value="{{.Query}}"
    hx-get="/persons"
    hx-trigger="keyup changed delay:300ms"
    hx-target="#person-results"
    hx-push-url="true"
    aria-label="Search people"
  >
  <table>
    <thead>
      <tr><th>Name</th><th>Nickname</th><th>Birth Year</th><th></th></tr>
    </thead>
    <tbody id="person-results">
      {{template "persons-rows" .}}
    </tbody>
  </table>
</article>
{{template "layout-foot" .}}
{{end}}
```

- [ ] **Step 5: Implement `partials/persons_rows.html`**

Replace `internal/web/templates/partials/persons_rows.html`:

```html
{{define "persons-rows"}}
{{range .Persons}}
<tr>
  <td>{{.Name}}</td>
  <td>{{.NickName}}</td>
  <td>{{if .BirthYear}}{{.BirthYear}}{{end}}</td>
  <td>
    <a href="/persons/{{.ID}}" role="button" class="outline">View</a>
    <button
      hx-delete="/persons/{{.ID}}"
      hx-confirm="Delete {{.Name}}? This cannot be undone."
      hx-target="closest tr"
      hx-swap="outerHTML"
      class="secondary outline"
    >Delete</button>
  </td>
</tr>
{{else}}
<tr><td colspan="4"><em>No results.</em></td></tr>
{{end}}
{{end}}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/web/... -run "TestPersonsList|TestPersonsSearch" -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/web/handlers_persons.go internal/web/templates/persons_list.html internal/web/templates/partials/persons_rows.html internal/web/web_test.go
git commit -m "feat: person list and search with HTMX partial"
```

---

## Task 6: Person create page

**Files:**
- Modify: `internal/web/templates/persons_create.html`
- Modify: `internal/web/web_test.go`

(Handler `handlePersonsCreate` and `handlePersonsNew` are already implemented in Task 5.)

- [ ] **Step 1: Write failing tests**

Append to `internal/web/web_test.go`:

```go
func TestPersonsNew_ReturnsForm(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons/new")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `action="/persons"`)
}

func TestPersonsCreate_RedirectsToDetail(t *testing.T) {
	s := newTestServer(t)
	w := doPost(t, s, "/persons", "name=Alice+Test")
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Contains(t, w.Header().Get("Location"), "/persons/")
}

func TestPersonsCreate_RequiresName(t *testing.T) {
	s := newTestServer(t)
	w := doPost(t, s, "/persons", "name=")
	assert.Equal(t, http.StatusOK, w.Code) // re-renders form
	assert.Contains(t, w.Body.String(), "required")
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/web/... -run "TestPersonsNew|TestPersonsCreate" -v
```

Expected: `TestPersonsNew_ReturnsForm` fails (template is a stub).

- [ ] **Step 3: Implement `persons_create.html`**

Replace `internal/web/templates/persons_create.html`:

```html
{{define "persons-create"}}
{{template "layout-head" .}}
<article>
  <header><h2>New Person</h2></header>
  {{if .}}{{with .Error}}<p class="error"><small>{{.}}</small></p>{{end}}{{end}}
  <form action="/persons" method="post">
    <label>
      Name <span aria-hidden="true">*</span>
      <input type="text" name="name" required placeholder="Full name" autofocus>
    </label>
    <label>
      Nickname
      <input type="text" name="nickName" placeholder="Preferred name or alias">
    </label>
    <div class="grid">
      <label>
        Birth Year
        <input type="number" name="birthYear" min="1" max="9999" placeholder="YYYY">
      </label>
      <label>
        Birth Month
        <input type="number" name="birthMonth" min="1" max="12" placeholder="MM">
      </label>
      <label>
        Birth Day
        <input type="number" name="birthDay" min="1" max="31" placeholder="DD">
      </label>
    </div>
    <label>
      Note
      <textarea name="note" placeholder="Any notes about this person"></textarea>
    </label>
    <div style="display:flex;gap:1rem;">
      <button type="submit">Create Person</button>
      <a href="/persons" role="button" class="secondary outline">Cancel</a>
    </div>
  </form>
</article>
{{template "layout-foot" .}}
{{end}}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/web/... -run "TestPersonsNew|TestPersonsCreate" -v
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/web/templates/persons_create.html internal/web/web_test.go
git commit -m "feat: person create page"
```

---

## Task 7: Person detail page, edit form, update, delete

**Files:**
- Modify: `internal/web/templates/persons_detail.html`
- Modify: `internal/web/templates/partials/person_edit_form.html`
- Modify: `internal/web/web_test.go`

(Handlers are already implemented in Task 5.)

- [ ] **Step 1: Write failing tests**

Append to `internal/web/web_test.go`:

```go
func TestPersonsDetail_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Carol Detail")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doGet(t, s, "/persons/"+id)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Carol Detail")
}

func TestPersonsDetail_NotFound(t *testing.T) {
	s := newTestServer(t)
	w := doGet(t, s, "/persons/NOTEXIST")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPersonsUpdate_Redirects(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Dave Old")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doMethod(t, s, http.MethodPut, "/persons/"+id, "name=Dave+New")
	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestPersonsDelete_Redirects(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	id := createTestPerson(t, sqlDB, "Eve Delete")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	w := doMethod(t, s, http.MethodDelete, "/persons/"+id, "")
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/persons", w.Header().Get("Location"))
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/web/... -run "TestPersonsDetail|TestPersonsUpdate|TestPersonsDelete" -v
```

Expected: `TestPersonsDetail_Returns200` fails (stub template).

- [ ] **Step 3: Implement `persons_detail.html`**

Replace `internal/web/templates/persons_detail.html`:

```html
{{define "persons-detail"}}
{{template "layout-head" .}}

{{/* Person Header */}}
<article id="person-header">
  <header>
    <div style="display:flex;justify-content:space-between;align-items:center;">
      <div>
        <h2 style="margin:0;">{{.PersonData.Name}}
          {{if .PersonData.NickName}}<small>({{.PersonData.NickName}})</small>{{end}}
        </h2>
        {{if .PersonData.BirthYear}}
        <small>Born: {{.PersonData.BirthYear}}
          {{if .PersonData.BirthMonth}}-{{printf "%02d" .PersonData.BirthMonth}}{{end}}
          {{if .PersonData.BirthDay}}-{{printf "%02d" .PersonData.BirthDay}}{{end}}
        </small>
        {{end}}
      </div>
      <button
        hx-get="/persons/{{.Person.ID}}/edit-form"
        hx-target="#person-header"
        hx-swap="outerHTML"
        class="outline"
      >Edit</button>
    </div>
    {{if .PersonData.Note}}<p>{{.PersonData.Note}}</p>{{end}}
  </header>
</article>

{{/* Contact Entity Sections */}}
{{range .Sections}}
{{template "entity-section" .}}
{{end}}

{{/* Person-to-Person Sections */}}
{{range .People}}
<article id="rel-section-{{.RelType}}">
  <header><h3>{{.Label}}</h3></header>
  <table>
    <tbody id="rel-list-{{.RelType}}">
      {{range .Persons}}
      {{template "relationship-row" (dict "PersonID" $.Person.ID "PersonWithRel" .)}}
      {{end}}
    </tbody>
  </table>
  <details>
    <summary>+ Link person</summary>
    {{template "relationship-form" (dict "PersonID" $.Person.ID "RelType" .RelType "EditMode" false)}}
  </details>
</article>
{{end}}

<p><a href="/persons" class="secondary">← Back to People</a></p>

{{template "layout-foot" .}}
{{end}}
```

Note: The `dict` template function is not built in to `html/template`. Replace the approach: pass a struct wrapper or use a simpler approach. Since `html/template` has no `dict`, restructure to avoid it. Update the template to iterate over relationship rows without `dict`:

Replace `internal/web/templates/persons_detail.html` with this corrected version:

```html
{{define "persons-detail"}}
{{template "layout-head" .}}

<article id="person-header">
  <header>
    <div style="display:flex;justify-content:space-between;align-items:center;">
      <div>
        <h2 style="margin:0;">{{.PersonData.Name}}
          {{if .PersonData.NickName}}<small>({{.PersonData.NickName}})</small>{{end}}
        </h2>
        {{if .PersonData.BirthYear}}
        <small>Born: {{.PersonData.BirthYear}}{{if .PersonData.BirthMonth}}-{{printf "%02d" .PersonData.BirthMonth}}{{end}}{{if .PersonData.BirthDay}}-{{printf "%02d" .PersonData.BirthDay}}{{end}}</small>
        {{end}}
      </div>
      <button
        hx-get="/persons/{{.Person.ID}}/edit-form"
        hx-target="#person-header"
        hx-swap="outerHTML"
        class="outline secondary"
      >Edit</button>
    </div>
    {{if .PersonData.Note}}<p>{{.PersonData.Note}}</p>{{end}}
  </header>
</article>

{{range .Sections}}
{{template "entity-section" .}}
{{end}}

{{range .People}}
<article id="rel-section-{{.RelType}}">
  <header><h3>{{.Label}}</h3></header>
  {{if .Persons}}
  <table>
    <tbody>
      {{range .Persons}}
      <tr data-row>
        <td><a href="/persons/{{.Person.ID}}">{{personName .Person}}</a></td>
        <td>{{.Relationship.Type}}</td>
        <td>{{strOrEmpty .Relationship.DateFrom}}{{if .Relationship.DateTo}} – {{strOrEmpty .Relationship.DateTo}}{{end}}</td>
        <td>{{strOrEmpty .Relationship.Note}}</td>
        <td>
          <button
            hx-get="/relationships/{{.Relationship.ID}}/edit?person_id={{$.PersonID}}"
            hx-target="closest [data-row]"
            hx-swap="outerHTML"
            class="outline secondary"
          >Edit</button>
          <button
            hx-delete="/relationships/{{.Relationship.ID}}"
            hx-confirm="Remove this link?"
            hx-target="closest [data-row]"
            hx-swap="outerHTML"
            class="outline secondary"
          >Delete</button>
        </td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{end}}
  <details>
    <summary>+ Link person ({{.RelType}})</summary>
    <form
      hx-post="/persons/{{.PersonID}}/relationships"
      hx-target="#rel-section-{{.RelType}} tbody"
      hx-swap="beforeend"
    >
      <input type="hidden" name="rel_type" value="{{.RelType}}">
      <div style="display:flex;gap:0.5rem;align-items:flex-end;">
        <label style="flex:1;">
          Search person
          <input type="search"
            name="q"
            placeholder="Type name…"
            hx-get="/persons"
            hx-trigger="keyup changed delay:300ms"
            hx-target="#person-search-results-{{.RelType}}"
          >
        </label>
      </div>
      <div id="person-search-results-{{.RelType}}"></div>
      <input type="hidden" name="person_b_id" id="person-b-id-{{.RelType}}">
      <p id="selected-person-{{.RelType}}"><em>No person selected</em></p>
      <label>
        Date From <input type="text" name="date_from" placeholder="YYYY or YYYY-MM or YYYY-MM-DD">
      </label>
      <label>
        Date To <input type="text" name="date_to" placeholder="YYYY or YYYY-MM or YYYY-MM-DD">
      </label>
      <label>
        Note <input type="text" name="note">
      </label>
      <button type="submit">Link Person</button>
    </form>
  </details>
</article>
{{end}}

<p style="margin-top:2rem;"><a href="/persons">← Back to People</a></p>

{{template "layout-foot" .}}
{{end}}
```

Note: `personName` is a template function — add it to the server template FuncMap. See Step 4.

- [ ] **Step 4: Add template functions to `server.go`**

Update `NewServer` in `internal/web/server.go` to register template functions:

```go
func NewServer(d *dal.DAL) (*Server, error) {
	funcs := template.FuncMap{
		"personName": func(e dal.Entity) string {
			var data struct {
				Name string `json:"name"`
			}
			_ = json.Unmarshal(e.Data, &data)
			return data.Name
		},
		"strOrEmpty": strOrEmpty,
	}
	tmpl, err := template.New("").Funcs(funcs).ParseFS(
		templatesFS,
		"templates/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}
	return &Server{dal: d, tmpl: tmpl}, nil
}
```

Add `"encoding/json"` to server.go imports.

Also add `GET /persons/{id}/edit-form` route to `Handler()` in server.go:

```go
r.Get("/persons/{id}/edit-form", s.handlePersonsEditForm)
```

Add the handler stub to `handlers_persons.go`:

```go
func (s *Server) handlePersonsEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	var view PersonDetailView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		person, err := q.GetEntity(ctx, id)
		if err != nil {
			return err
		}
		view.Person = person
		view.PersonData = decodePersonData(person.Data)
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "person-edit-form", view)
}
```

- [ ] **Step 5: Implement `partials/person_edit_form.html`**

Replace `internal/web/templates/partials/person_edit_form.html`:

```html
{{define "person-edit-form"}}
<article id="person-header">
  <form
    hx-put="/persons/{{.Person.ID}}"
    hx-target="body"
    hx-push-url="true"
  >
    <div class="grid">
      <label>
        Name *
        <input type="text" name="name" value="{{.PersonData.Name}}" required>
      </label>
      <label>
        Nickname
        <input type="text" name="nickName" value="{{.PersonData.NickName}}">
      </label>
    </div>
    <div class="grid">
      <label>
        Birth Year
        <input type="number" name="birthYear" value="{{if .PersonData.BirthYear}}{{.PersonData.BirthYear}}{{end}}" min="1" max="9999">
      </label>
      <label>
        Birth Month
        <input type="number" name="birthMonth" value="{{if .PersonData.BirthMonth}}{{.PersonData.BirthMonth}}{{end}}" min="1" max="12">
      </label>
      <label>
        Birth Day
        <input type="number" name="birthDay" value="{{if .PersonData.BirthDay}}{{.PersonData.BirthDay}}{{end}}" min="1" max="31">
      </label>
    </div>
    <label>
      Note
      <textarea name="note">{{.PersonData.Note}}</textarea>
    </label>
    <div style="display:flex;gap:1rem;">
      <button type="submit">Save</button>
      <button
        type="button"
        hx-get="/persons/{{.Person.ID}}"
        hx-target="body"
        hx-push-url="true"
        class="secondary outline"
      >Cancel</button>
    </div>
  </form>
</article>
{{end}}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/web/... -run "TestPersonsDetail|TestPersonsUpdate|TestPersonsDelete" -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/web/handlers_persons.go internal/web/server.go internal/web/templates/persons_detail.html internal/web/templates/partials/person_edit_form.html internal/web/web_test.go
git commit -m "feat: person detail page with edit form and delete"
```

---

## Task 8: Contact entity handlers and templates

**Files:**
- Modify: `internal/web/handlers_entities.go`
- Modify: `internal/web/templates/partials/entity_section.html`
- Modify: `internal/web/templates/partials/entity_row.html`
- Modify: `internal/web/templates/partials/entity_form.html`
- Modify: `internal/web/web_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/web/web_test.go`:

```go
func TestEntitiesCreate_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Frank Entity")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	body := "type=EmailAddress&email=frank%40example.com&label=work"
	w := doPost(t, s, "/persons/"+personID+"/entities", body)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "frank@example.com")
}

func TestEntitiesDelete_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Grace Delete")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	// Create entity first
	body := "type=EmailAddress&email=grace%40example.com"
	w := doPost(t, s, "/persons/"+personID+"/entities", body)
	require.Equal(t, http.StatusOK, w.Code)

	// Extract entity ID from response (look for /entities/ID/edit pattern)
	respBody := w.Body.String()
	eidStart := strings.Index(respBody, "/entities/")
	require.NotEqual(t, -1, eidStart)
	eidEnd := strings.Index(respBody[eidStart+len("/entities/"):], "/")
	require.NotEqual(t, -1, eidEnd)
	eid := respBody[eidStart+len("/entities/") : eidStart+len("/entities/")+eidEnd]

	w2 := doMethod(t, s, http.MethodDelete, "/entities/"+eid, "")
	assert.Equal(t, http.StatusOK, w2.Code)
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/web/... -run "TestEntities" -v
```

Expected: FAIL.

- [ ] **Step 3: Implement `handlers_entities.go`**

Replace the entire file:

```go
package web

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/astromechza/aurelianprm/internal/dal"
)

func (s *Server) handleEntitiesCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	personID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	entityType := r.FormValue("type")
	data, err := parseEntityFormData(entityType, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}

	relType := relTypeForEntityType(entityType)
	if relType == "" {
		http.Error(w, "unknown entity type", http.StatusBadRequest)
		return
	}

	var section EntitySection
	err = s.dal.WithTx(ctx, func(q *dal.Queries) error {
		entity, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: entityType,
			Data: data,
		})
		if err != nil {
			return err
		}
		if _, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: personID,
			EntityBID: entity.ID,
			Type:      relType,
		}); err != nil {
			return err
		}
		neighbours, err := q.GetNeighboursByRelType(ctx, personID, relType)
		if err != nil {
			return err
		}
		section = EntitySection{
			PersonID:   personID,
			RelType:    relType,
			EntityType: entityType,
			Label:      labelForRelType(relType),
		}
		for _, nr := range neighbours {
			section.Entities = append(section.Entities, EntityWithRel{
				Entity:       nr.Entity,
				Relationship: nr.Relationship,
				DataMap:      decodeDataMap(nr.Entity.Data),
			})
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "entity-section", section)
}

func (s *Server) handleEntitiesEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eid := chi.URLParam(r, "eid")

	var view EntityFormView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		entity, err := q.GetEntity(ctx, eid)
		if err != nil {
			return err
		}
		neighbours, err := q.GetNeighbours(ctx, eid)
		if err != nil {
			return err
		}
		var relID string
		for _, nr := range neighbours {
			if nr.Entity.Type == "Person" {
				relID = nr.Relationship.ID
				break
			}
		}
		view = EntityFormView{
			EntityID: eid,
			RelID:    relID,
			Type:     entity.Type,
			DataMap:  decodeDataMap(entity.Data),
			EditMode: true,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.render(w, "entity-form", view)
}

func (s *Server) handleEntitiesUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eid := chi.URLParam(r, "eid")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var view EntityWithRel
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		entity, err := q.GetEntity(ctx, eid)
		if err != nil {
			return err
		}
		data, err := parseEntityFormData(entity.Type, r)
		if err != nil {
			return err
		}
		if err := q.UpdateEntity(ctx, dal.UpdateEntityParams{ID: eid, Data: data}); err != nil {
			return err
		}
		entity, err = q.GetEntity(ctx, eid)
		if err != nil {
			return err
		}
		neighbours, err := q.GetNeighbours(ctx, eid)
		if err != nil {
			return err
		}
		var rel dal.Relationship
		for _, nr := range neighbours {
			if nr.Entity.Type == "Person" {
				rel = nr.Relationship
				break
			}
		}
		view = EntityWithRel{
			Entity:       entity,
			Relationship: rel,
			DataMap:      decodeDataMap(entity.Data),
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "entity-row", view)
}

func (s *Server) handleEntitiesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eid := chi.URLParam(r, "eid")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, eid)
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Implement `entity_section.html`**

Replace `internal/web/templates/partials/entity_section.html`:

```html
{{define "entity-section"}}
<article id="entity-section-{{.RelType}}">
  <header>
    <h3>{{.Label}}</h3>
  </header>
  {{if .Entities}}
  <table>
    <tbody id="entity-list-{{.RelType}}">
      {{range .Entities}}
      {{template "entity-row" .}}
      {{end}}
    </tbody>
  </table>
  {{end}}
  <details>
    <summary>+ Add</summary>
    <form
      hx-post="/persons/{{.PersonID}}/entities"
      hx-target="#entity-section-{{.RelType}}"
      hx-swap="outerHTML"
    >
      <input type="hidden" name="type" value="{{.EntityType}}">
      {{template "entity-form" (entityFormCreate .PersonID .EntityType)}}
    </form>
  </details>
</article>
{{end}}
```

Note: `entityFormCreate` is a template function we need to add to server.go. It returns an `EntityFormView` with `PersonID` and `Type` set and `EditMode: false`.

Add to `NewServer` FuncMap in `server.go`:

```go
"entityFormCreate": func(personID, entityType string) EntityFormView {
    return EntityFormView{
        PersonID: personID,
        Type:     entityType,
        DataMap:  make(map[string]any),
        EditMode: false,
    }
},
```

- [ ] **Step 5: Implement `entity_row.html`**

Replace `internal/web/templates/partials/entity_row.html`:

```html
{{define "entity-row"}}
<tr data-row>
  <td>
    {{$type := .Entity.Type}}
    {{if eq $type "EmailAddress"}}{{index .DataMap "email"}}{{if index .DataMap "label"}} <small>({{index .DataMap "label"}})</small>{{end}}
    {{else if eq $type "PhoneNumber"}}{{index .DataMap "telephone"}}{{if index .DataMap "label"}} <small>({{index .DataMap "label"}})</small>{{end}}
    {{else if eq $type "WebSite"}}<a href="{{index .DataMap "url"}}" target="_blank">{{if index .DataMap "title"}}{{index .DataMap "title"}}{{else}}{{index .DataMap "url"}}{{end}}</a>{{if index .DataMap "label"}} <small>({{index .DataMap "label"}})</small>{{end}}
    {{else if eq $type "SocialNetwork"}}{{index .DataMap "platform"}}{{if index .DataMap "handle"}}: {{index .DataMap "handle"}}{{end}}
    {{else if eq $type "PostalAddress"}}{{if index .DataMap "streetAddress"}}{{index .DataMap "streetAddress"}}, {{end}}{{if index .DataMap "city"}}{{index .DataMap "city"}}{{end}}{{if index .DataMap "country"}} {{index .DataMap "country"}}{{end}}
    {{else if eq $type "Career"}}{{index .DataMap "role"}} @ {{index .DataMap "organization"}}{{if index .DataMap "label"}} <small>({{index .DataMap "label"}})</small>{{end}}
    {{else if eq $type "Pet"}}{{index .DataMap "name"}}{{if index .DataMap "species"}} ({{index .DataMap "species"}}){{end}}
    {{else}}{{.Entity.Type}}{{end}}
  </td>
  <td>
    <button
      hx-get="/entities/{{.Entity.ID}}/edit"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
      class="outline secondary"
    >Edit</button>
    <button
      hx-delete="/entities/{{.Entity.ID}}"
      hx-confirm="Delete this item?"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
      class="outline secondary"
    >Delete</button>
  </td>
</tr>
{{end}}
```

- [ ] **Step 6: Implement `entity_form.html`**

Replace `internal/web/templates/partials/entity_form.html`:

```html
{{define "entity-form"}}
{{$type := .Type}}
{{$dm := .DataMap}}
{{if .Error}}<p class="error"><small>{{.Error}}</small></p>{{end}}

{{if eq $type "EmailAddress"}}
<div class="grid">
  <label>Email * <input type="email" name="email" value="{{index $dm "email"}}" required></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="work, personal…"></label>
</div>

{{else if eq $type "PhoneNumber"}}
<div class="grid">
  <label>Telephone * <input type="tel" name="telephone" value="{{index $dm "telephone"}}" required></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="mobile, home…"></label>
</div>

{{else if eq $type "WebSite"}}
<div class="grid">
  <label>URL * <input type="url" name="url" value="{{index $dm "url"}}" required></label>
  <label>Title <input type="text" name="title" value="{{index $dm "title"}}"></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}"></label>
</div>

{{else if eq $type "SocialNetwork"}}
<div class="grid">
  <label>Platform * <input type="text" name="platform" value="{{index $dm "platform"}}" required placeholder="LinkedIn, GitHub…"></label>
  <label>Handle <input type="text" name="handle" value="{{index $dm "handle"}}"></label>
  <label>URL <input type="url" name="url" value="{{index $dm "url"}}"></label>
</div>

{{else if eq $type "PostalAddress"}}
<div class="grid">
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="home, work…"></label>
  <label>Country (ISO) <input type="text" name="country" value="{{index $dm "country"}}" maxlength="2" placeholder="US"></label>
</div>
<div class="grid">
  <label>City <input type="text" name="city" value="{{index $dm "city"}}"></label>
  <label>Street Address <input type="text" name="streetAddress" value="{{index $dm "streetAddress"}}"></label>
</div>

{{else if eq $type "Career"}}
<div class="grid">
  <label>Role * <input type="text" name="role" value="{{index $dm "role"}}" required placeholder="Software Engineer"></label>
  <label>Organization * <input type="text" name="organization" value="{{index $dm "organization"}}" required placeholder="Acme Corp"></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="current, former…"></label>
</div>

{{else if eq $type "Pet"}}
<div class="grid">
  <label>Name * <input type="text" name="name" value="{{index $dm "name"}}" required></label>
  <label>Species <input type="text" name="species" value="{{index $dm "species"}}" placeholder="dog, cat…"></label>
  <label>Birth Date <input type="date" name="birthDate" value="{{index $dm "birthDate"}}"></label>
</div>

{{end}}

{{if .EditMode}}
<div style="display:flex;gap:0.5rem;">
  <button type="submit"
    hx-put="/entities/{{.EntityID}}"
    hx-target="closest [data-row]"
    hx-swap="outerHTML"
  >Save</button>
  <button type="button"
    hx-get="/entities/{{.EntityID}}/cancel"
    hx-target="closest [data-row]"
    hx-swap="outerHTML"
    class="secondary outline"
  >Cancel</button>
</div>
{{else}}
<button type="submit">Add</button>
{{end}}
{{end}}
```

Note: the edit form wraps with `<form hx-put...>` or uses button-level attributes. Since the entity_form partial is embedded inside the add `<form>` in entity_section.html, for edit mode the form should be standalone. Add `GET /entities/{eid}/cancel` route that returns the entity row (to cancel edit). Add this handler to `handlers_entities.go`:

```go
func (s *Server) handleEntitiesCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	eid := chi.URLParam(r, "eid")
	var view EntityWithRel
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		entity, err := q.GetEntity(ctx, eid)
		if err != nil {
			return err
		}
		neighbours, err := q.GetNeighbours(ctx, eid)
		if err != nil {
			return err
		}
		var rel dal.Relationship
		for _, nr := range neighbours {
			if nr.Entity.Type == "Person" {
				rel = nr.Relationship
				break
			}
		}
		view = EntityWithRel{
			Entity:       entity,
			Relationship: rel,
			DataMap:      decodeDataMap(entity.Data),
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "entity-row", view)
}
```

Register in `server.go Handler()`:
```go
r.Get("/entities/{eid}/cancel", s.handleEntitiesCancel)
```

And for edit mode, entity_form.html in edit mode renders a standalone form wrapping the fields. Update `entity_form.html` to wrap edit mode in its own `<form>`:

The entity_form template in edit mode is rendered standalone (from `handleEntitiesEditForm`), so it should include the `<form>` tag itself. Restructure as:

```html
{{define "entity-form"}}
{{if .EditMode}}
<tr data-row>
  <td colspan="2">
    <form
      hx-put="/entities/{{.EntityID}}"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
    >
      {{template "entity-form-fields" .}}
      <div style="display:flex;gap:0.5rem;">
        <button type="submit">Save</button>
        <button type="button"
          hx-get="/entities/{{.EntityID}}/cancel"
          hx-target="closest [data-row]"
          hx-swap="outerHTML"
          class="secondary outline"
        >Cancel</button>
      </div>
    </form>
  </td>
</tr>
{{else}}
{{template "entity-form-fields" .}}
<button type="submit">Add</button>
{{end}}
{{end}}
```

And extract the fields into a sub-template `entity-form-fields`:

Append to `entity_form.html`:

```html
{{define "entity-form-fields"}}
{{$type := .Type}}
{{$dm := .DataMap}}
{{if .Error}}<p class="error"><small>{{.Error}}</small></p>{{end}}

{{if eq $type "EmailAddress"}}
<div class="grid">
  <label>Email * <input type="email" name="email" value="{{index $dm "email"}}" required></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="work, personal…"></label>
</div>
{{else if eq $type "PhoneNumber"}}
<div class="grid">
  <label>Telephone * <input type="tel" name="telephone" value="{{index $dm "telephone"}}" required></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="mobile, home…"></label>
</div>
{{else if eq $type "WebSite"}}
<div class="grid">
  <label>URL * <input type="url" name="url" value="{{index $dm "url"}}" required></label>
  <label>Title <input type="text" name="title" value="{{index $dm "title"}}"></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}"></label>
</div>
{{else if eq $type "SocialNetwork"}}
<div class="grid">
  <label>Platform * <input type="text" name="platform" value="{{index $dm "platform"}}" required placeholder="LinkedIn, GitHub…"></label>
  <label>Handle <input type="text" name="handle" value="{{index $dm "handle"}}"></label>
  <label>URL <input type="url" name="url" value="{{index $dm "url"}}"></label>
</div>
{{else if eq $type "PostalAddress"}}
<div class="grid">
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="home, work…"></label>
  <label>Country (ISO 2) <input type="text" name="country" value="{{index $dm "country"}}" maxlength="2" placeholder="US"></label>
</div>
<div class="grid">
  <label>City <input type="text" name="city" value="{{index $dm "city"}}"></label>
  <label>Street Address <input type="text" name="streetAddress" value="{{index $dm "streetAddress"}}"></label>
</div>
{{else if eq $type "Career"}}
<div class="grid">
  <label>Role * <input type="text" name="role" value="{{index $dm "role"}}" required></label>
  <label>Organization * <input type="text" name="organization" value="{{index $dm "organization"}}" required></label>
  <label>Label <input type="text" name="label" value="{{index $dm "label"}}" placeholder="current, former…"></label>
</div>
{{else if eq $type "Pet"}}
<div class="grid">
  <label>Name * <input type="text" name="name" value="{{index $dm "name"}}" required></label>
  <label>Species <input type="text" name="species" value="{{index $dm "species"}}" placeholder="dog, cat…"></label>
  <label>Birth Date <input type="date" name="birthDate" value="{{index $dm "birthDate"}}"></label>
</div>
{{end}}
{{end}}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/web/... -run "TestEntities" -v
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/web/handlers_entities.go internal/web/server.go internal/web/templates/partials/entity_section.html internal/web/templates/partials/entity_row.html internal/web/templates/partials/entity_form.html internal/web/web_test.go
git commit -m "feat: contact entity CRUD handlers and templates"
```

---

## Task 9: Person-to-person relationship handlers and templates

**Files:**
- Modify: `internal/web/handlers_relationships.go`
- Modify: `internal/web/templates/partials/relationship_row.html`
- Modify: `internal/web/templates/partials/relationship_form.html`
- Modify: `internal/web/web_test.go`

- [ ] **Step 1: Write failing tests**

Append to `internal/web/web_test.go`:

```go
func TestRelationshipsCreate_ReturnsPartial(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personAID := createTestPerson(t, sqlDB, "Henry A")
	personBID := createTestPerson(t, sqlDB, "Ida B")
	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)

	body := "person_b_id=" + personBID + "&rel_type=knows"
	w := doPost(t, s, "/persons/"+personAID+"/relationships", body)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Ida B")
}

func TestRelationshipsDelete_Returns200(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	personAID := createTestPerson(t, sqlDB, "Jack A")
	personBID := createTestPerson(t, sqlDB, "Kim B")

	// Create relationship via DAL
	var relID string
	d := dal.New(sqlDB)
	err = d.WithTx(t.Context(), func(q *dal.Queries) error {
		rel, err := q.CreateRelationship(t.Context(), dal.CreateRelationshipParams{
			EntityAID: personAID,
			EntityBID: personBID,
			Type:      "knows",
		})
		relID = rel.ID
		return err
	})
	require.NoError(t, err)

	s, err := web.NewServer(dal.New(sqlDB))
	require.NoError(t, err)
	w := doMethod(t, s, http.MethodDelete, "/relationships/"+relID, "")
	assert.Equal(t, http.StatusOK, w.Code)
}
```

- [ ] **Step 2: Run to verify failures**

```bash
go test ./internal/web/... -run "TestRelationships" -v
```

Expected: FAIL.

- [ ] **Step 3: Implement `handlers_relationships.go`**

Replace the entire file:

```go
package web

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/astromechza/aurelianprm/internal/dal"
)

func (s *Server) handleRelationshipsCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	personID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	personBID := r.FormValue("person_b_id")
	relType := r.FormValue("rel_type")
	if personBID == "" || relType == "" {
		http.Error(w, "person_b_id and rel_type are required", http.StatusBadRequest)
		return
	}

	var view RelationshipRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		rel, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: personID,
			EntityBID: personBID,
			Type:      relType,
			DateFrom:  nilOrStr(r.FormValue("date_from")),
			DateTo:    nilOrStr(r.FormValue("date_to")),
			Note:      nilOrStr(r.FormValue("note")),
		})
		if err != nil {
			return err
		}
		other, err := q.GetEntity(ctx, personBID)
		if err != nil {
			return err
		}
		view = RelationshipRowView{
			PersonID: personID,
			PersonWithRel: PersonWithRel{
				Person:       other,
				Relationship: rel,
				Direction:    "outbound",
			},
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "relationship-row", view)
}

func (s *Server) handleRelationshipsEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := chi.URLParam(r, "rid")
	personID := r.URL.Query().Get("person_id")

	var view RelationshipFormView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		rel, err := q.GetRelationship(ctx, rid)
		if err != nil {
			return err
		}
		otherPersonID := rel.EntityBID
		if rel.EntityBID == personID {
			otherPersonID = rel.EntityAID
		}
		other, err := q.GetEntity(ctx, otherPersonID)
		if err != nil {
			return err
		}
		view = RelationshipFormView{
			PersonID:    personID,
			RelID:       rid,
			EditMode:    true,
			DateFrom:    strOrEmpty(rel.DateFrom),
			DateTo:      strOrEmpty(rel.DateTo),
			Note:        strOrEmpty(rel.Note),
			OtherPerson: &other,
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.render(w, "relationship-form", view)
}

func (s *Server) handleRelationshipsUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := chi.URLParam(r, "rid")
	personID := r.URL.Query().Get("person_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	var view RelationshipRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if err := q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       rid,
			DateFrom: nilOrStr(r.FormValue("date_from")),
			DateTo:   nilOrStr(r.FormValue("date_to")),
			Note:     nilOrStr(r.FormValue("note")),
		}); err != nil {
			return err
		}
		rel, err := q.GetRelationship(ctx, rid)
		if err != nil {
			return err
		}
		otherPersonID := rel.EntityBID
		if rel.EntityBID == personID {
			otherPersonID = rel.EntityAID
		}
		direction := "outbound"
		if rel.EntityBID == personID {
			direction = "inbound"
		}
		other, err := q.GetEntity(ctx, otherPersonID)
		if err != nil {
			return err
		}
		view = RelationshipRowView{
			PersonID: personID,
			PersonWithRel: PersonWithRel{
				Person:       other,
				Relationship: rel,
				Direction:    direction,
			},
		}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "relationship-row", view)
}

func (s *Server) handleRelationshipsDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := chi.URLParam(r, "rid")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteRelationship(ctx, rid)
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 4: Implement `relationship_row.html`**

Replace `internal/web/templates/partials/relationship_row.html`:

```html
{{define "relationship-row"}}
<tr data-row>
  <td><a href="/persons/{{.PersonWithRel.Person.ID}}">{{personName .PersonWithRel.Person}}</a></td>
  <td>{{.PersonWithRel.Relationship.Type}}</td>
  <td>
    {{strOrEmpty .PersonWithRel.Relationship.DateFrom}}
    {{if .PersonWithRel.Relationship.DateTo}} – {{strOrEmpty .PersonWithRel.Relationship.DateTo}}{{end}}
  </td>
  <td>{{strOrEmpty .PersonWithRel.Relationship.Note}}</td>
  <td>
    <button
      hx-get="/relationships/{{.PersonWithRel.Relationship.ID}}/edit?person_id={{.PersonID}}"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
      class="outline secondary"
    >Edit</button>
    <button
      hx-delete="/relationships/{{.PersonWithRel.Relationship.ID}}"
      hx-confirm="Remove this link?"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
      class="outline secondary"
    >Delete</button>
  </td>
</tr>
{{end}}
```

- [ ] **Step 5: Implement `relationship_form.html`**

Replace `internal/web/templates/partials/relationship_form.html`:

```html
{{define "relationship-form"}}
{{if .EditMode}}
<tr data-row>
  <td colspan="5">
    <form
      hx-put="/relationships/{{.RelID}}?person_id={{.PersonID}}"
      hx-target="closest [data-row]"
      hx-swap="outerHTML"
    >
      {{if .OtherPerson}}<strong>{{personName .OtherPerson}}</strong>{{end}}
      <div class="grid">
        <label>Date From <input type="text" name="date_from" value="{{.DateFrom}}" placeholder="YYYY or YYYY-MM or YYYY-MM-DD"></label>
        <label>Date To <input type="text" name="date_to" value="{{.DateTo}}" placeholder="YYYY or YYYY-MM or YYYY-MM-DD"></label>
        <label>Note <input type="text" name="note" value="{{.Note}}"></label>
      </div>
      {{if .Error}}<p class="error"><small>{{.Error}}</small></p>{{end}}
      <button type="submit">Save</button>
    </form>
  </td>
</tr>
{{end}}
{{end}}
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/web/... -run "TestRelationships" -v
```

Expected: all PASS.

- [ ] **Step 7: Run full test suite**

```bash
go test ./internal/web/... -v
```

Expected: all PASS.

- [ ] **Step 8: Commit**

```bash
git add internal/web/handlers_relationships.go internal/web/templates/partials/relationship_row.html internal/web/templates/partials/relationship_form.html internal/web/web_test.go
git commit -m "feat: person relationship CRUD handlers and templates"
```

---

## Task 10: Wire up `main.go`

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Write the implementation**

Replace `main.go`:

```go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "Path to SQLite database file (required)")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	if *dbPath == "" {
		return fmt.Errorf("--db flag is required")
	}

	sqlDB, err := db.Open(*dbPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer sqlDB.Close()

	srv, err := web.NewServer(dal.New(sqlDB))
	if err != nil {
		return fmt.Errorf("create server: %w", err)
	}

	log.Printf("listening on %s", *addr)
	return http.ListenAndServe(*addr, srv.Handler())
}
```

- [ ] **Step 2: Build binary**

```bash
go build -o aurelianprm ./...
```

Expected: exits 0, binary `aurelianprm` created.

- [ ] **Step 3: Smoke test**

```bash
./aurelianprm --db /tmp/test-prm.db &
sleep 1
curl -s http://localhost:8080/ -L -o /dev/null -w "%{http_code}"
kill %1
rm -f /tmp/test-prm.db aurelianprm
```

Expected: `200`.

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: wire up main.go with --db flag and HTTP server"
```

---

## Task 11: Run full test suite and verify

- [ ] **Step 1: Run all tests**

```bash
go test ./... -v
```

Expected: all PASS, no failures.

- [ ] **Step 2: Build final binary**

```bash
go build ./...
```

Expected: exits 0.

- [ ] **Step 3: Commit if any fixes were needed**

```bash
git add -A
git commit -m "chore: fix any test or build issues found during final sweep"
```

---

## Self-Review

**Spec Coverage Check:**

| Spec requirement | Covered by task |
|---|---|
| GET / → redirect /persons | Task 3 |
| GET /persons full page | Task 5 |
| GET /persons?q= partial (HX-Request) | Task 5 |
| GET /persons/new | Task 6 |
| POST /persons → redirect /persons/{id} | Task 5 |
| GET /persons/{id} detail page | Task 7 |
| PUT /persons/{id} → redirect | Task 5/7 |
| DELETE /persons/{id} → redirect /persons | Task 5 |
| POST /persons/{id}/entities | Task 8 |
| GET /entities/{eid}/edit | Task 8 |
| PUT /entities/{eid} → entity-row partial | Task 8 |
| DELETE /entities/{eid} → empty 200 | Task 8 |
| POST /persons/{id}/relationships | Task 9 |
| GET /relationships/{rid}/edit | Task 9 |
| PUT /relationships/{rid} | Task 9 |
| DELETE /relationships/{rid} → empty 200 | Task 9 |
| chi middleware Logger + Recoverer | Task 3 |
| Embedded templates go:embed | Task 3 |
| Pico CSS CDN + HTMX CDN | Task 4 |
| No inline styles | All templates use Pico CSS classes |
| Handler integration tests | Tasks 4-9, 11 |
| main.go --db flag | Task 10 |
| 500 plain-text for HTMX errors | Task 3 (serverError) |
| 404 for missing entities | Task 7 |
| Person list: search with delay:300ms debounce | Task 5 template |
| Person detail: inline edit form via hx-get | Task 7 |
| Entity add in `<details>` element | Task 8 (entity_section.html) |
| Delete via hx-delete + hx-confirm + hx-swap outerHTML | Tasks 5, 8, 9 |
| GetRelationship DAL method | Task 1 |

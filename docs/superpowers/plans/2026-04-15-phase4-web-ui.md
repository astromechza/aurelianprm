# Phase 4 — Web UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a server-side-rendered HTMX-powered web UI for people, contact methods, interactions, and relationships on top of the existing API/store layer.

**Architecture:** All web handlers live in `internal/web/people.go` and call `internal/store` directly (no API layer). Templates are in `assets/templates/` using Go's `html/template`. The existing `Render(w, r, fragment, data)` helper handles HTMX vs. full-page automatically. HTMX is used for live search, interaction logging, contact method management, and relationship management — avoiding full-page reloads for these operations. Create/edit/delete person uses standard POST-redirect-GET.

**Tech Stack:** Go 1.26, `html/template`, HTMX v2 (vendored), Pico CSS v2 (vendored), `github.com/stretchr/testify`

---

## File Map

| File | Action | Purpose |
|---|---|---|
| `internal/web/render.go` | Modify | Add `deref` template func to `ParseTemplates` |
| `internal/web/people.go` | Create | All people-related handlers, data types, and helpers |
| `internal/web/server.go` | Modify | Register all new routes |
| `assets/templates/people_list.html` | Create | People list page + `people_list_rows.html` sub-template |
| `assets/templates/people_form.html` | Create | Create/edit person form |
| `assets/templates/people_detail.html` | Create | Person detail page + `interaction_row.html`, `contact_method_row.html`, `relationship_row.html` sub-templates |
| `internal/web/people_test.go` | Create | httptest-based tests for all web handlers |

---

## Task 1: Template funcmap + people list

**Files:**
- Modify: `internal/web/render.go`
- Create: `assets/templates/people_list.html`
- Create: `internal/web/people.go` (stub + list handler)
- Modify: `internal/web/server.go`

### Background

`ParseTemplates` currently calls `template.Must(template.ParseFS(...))`. To use `{{deref .Field}}` in templates (for `*string` fields), we need a funcmap. The trick is to call `template.New("").Funcs(...).ParseFS(...)` — the funcmap must be registered before `ParseFS` is called.

- [ ] **Update `ParseTemplates` in `internal/web/render.go`**

Replace the body of `ParseTemplates`:

```go
// ParseTemplates parses all templates from the given filesystem.
// It panics (via template.Must) if any template is malformed.
func ParseTemplates(fsys fs.FS) *Templates {
	return &Templates{
		tmpl: template.Must(template.New("").Funcs(template.FuncMap{
			// deref dereferences a *string, returning "" for nil.
			"deref": func(s *string) string {
				if s == nil {
					return ""
				}
				return *s
			},
		}).ParseFS(fsys, "templates/*.html")),
	}
}
```

- [ ] **Create `assets/templates/people_list.html`**

```html
{{define "people_list.html"}}
<hgroup>
  <h1>People</h1>
</hgroup>
<div class="grid">
  <input type="search" name="name" placeholder="Search by name…"
    value="{{.Query}}"
    hx-get="/people"
    hx-target="#people-list"
    hx-trigger="input changed delay:300ms, search"
    aria-label="Search people">
  <p><a href="/people/new" role="button">+ Add Person</a></p>
</div>
<div id="people-list">
  {{template "people_list_rows.html" .}}
</div>
{{end}}

{{define "people_list_rows.html"}}
{{if .People}}
<table>
  <thead>
    <tr>
      <th>Name</th>
      <th>Nickname</th>
      <th></th>
    </tr>
  </thead>
  <tbody>
    {{range .People}}
    <tr>
      <td><a href="/people/{{.ID}}">{{.Name}}</a></td>
      <td>{{deref .Nickname}}</td>
      <td><a href="/people/{{.ID}}" role="button" class="outline">View →</a></td>
    </tr>
    {{end}}
  </tbody>
</table>
{{else}}
<p>
  {{if .Query}}No people found matching "{{.Query}}".{{else}}No people yet. <a href="/people/new">Add someone</a>!{{end}}
</p>
{{end}}
{{end}}
```

- [ ] **Create `internal/web/people.go`** with data types and the list handler

```go
package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// peopleListData is passed to people_list.html and people_list_rows.html.
type peopleListData struct {
	People []model.Person
	Query  string
}

// personDetailData is passed to people_detail.html.
type personDetailData struct {
	Person          model.Person
	BirthdayDisplay string // pre-formatted, empty if no birthday
	ContactMethods  []model.ContactMethod
	Interactions    []model.Interaction
	Relationships   []relationshipRow
}

// relationshipRow pairs a Relationship with its resolved target display name.
type relationshipRow struct {
	Relationship model.Relationship
	ToName       string
}

// personFormData is passed to people_form.html for create and edit.
type personFormData struct {
	Person   *model.Person    // nil when creating
	Birthday *model.BlurryDate
	Error    string
}

// handlePeopleList renders the people list. HTMX live-search requests receive
// only the rows fragment; full-page loads receive the complete list template.
func (s *Server) handlePeopleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("name")
	people, err := store.ListPeople(r.Context(), s.db, query)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := peopleListData{People: people, Query: query}
	if r.Header.Get("HX-Request") != "" {
		// Live search swap — return only the rows, not the search input.
		s.templates.Render(w, r, "people_list_rows.html", data)
		return
	}
	s.templates.Render(w, r, "people_list.html", data)
}

// handlePersonDetail renders the full detail page for a person.
func (s *Server) handlePersonDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := store.GetPerson(r.Context(), s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data, err := s.buildPersonDetailData(r.Context(), p)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "people_detail.html", data)
}

// buildPersonDetailData loads all related data for a person and assembles the
// template data struct.
func (s *Server) buildPersonDetailData(ctx context.Context, p model.Person) (personDetailData, error) {
	data := personDetailData{
		Person:         p,
		ContactMethods: []model.ContactMethod{},
		Interactions:   []model.Interaction{},
		Relationships:  []relationshipRow{},
	}

	if p.BirthdayID != nil {
		bd, err := store.GetBlurryDate(ctx, s.db, *p.BirthdayID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return personDetailData{}, fmt.Errorf("loading birthday: %w", err)
		}
		if err == nil {
			data.BirthdayDisplay = formatBlurryDate(bd)
		}
	}

	cms, err := store.ListContactMethods(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading contact methods: %w", err)
	}
	data.ContactMethods = cms

	interactions, err := store.ListInteractions(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading interactions: %w", err)
	}
	data.Interactions = interactions

	rels, err := store.ListRelationships(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading relationships: %w", err)
	}
	for _, rel := range rels {
		name, nameErr := s.resolveEntityName(ctx, rel.ToType, rel.ToID)
		if nameErr != nil {
			name = rel.ToID // fall back to raw ID if entity not found
		}
		data.Relationships = append(data.Relationships, relationshipRow{
			Relationship: rel,
			ToName:       name,
		})
	}
	return data, nil
}

// resolveEntityName returns a human-readable name for a relationship target.
func (s *Server) resolveEntityName(ctx context.Context, toType model.RelationshipTarget, toID string) (string, error) {
	switch toType {
	case model.RelationshipTargetPerson:
		p, err := store.GetPerson(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetPet:
		p, err := store.GetPet(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetWork:
		w, err := store.GetWork(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return w.Name, nil
	case model.RelationshipTargetAddress:
		a, err := store.GetAddress(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		if a.Label != nil {
			return *a.Label, nil
		}
		return "address", nil
	default:
		return toID, nil
	}
}

// handlePersonNew renders the create-person form.
func (s *Server) handlePersonNew(w http.ResponseWriter, r *http.Request) {
	s.templates.Render(w, r, "people_form.html", personFormData{})
}

// handlePersonCreate processes the create-person form submission.
func (s *Server) handlePersonCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.templates.Render(w, r, "people_form.html", personFormData{Error: "Name is required."})
		return
	}

	var birthdayID *string
	if bd := parseBirthdayForm(r); bd != nil {
		created, err := store.CreateBlurryDate(r.Context(), s.db, *bd)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		birthdayID = &created.ID
	}

	p, err := store.CreatePerson(r.Context(), s.db, model.Person{
		Name:       name,
		Nickname:   nullableFormValue(r, "nickname"),
		Notes:      nullableFormValue(r, "notes"),
		BirthdayID: birthdayID,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people/"+p.ID, http.StatusSeeOther)
}

// handlePersonEdit renders the edit-person form pre-populated with existing data.
func (s *Server) handlePersonEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := store.GetPerson(r.Context(), s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := personFormData{Person: &p}
	if p.BirthdayID != nil {
		bd, bdErr := store.GetBlurryDate(r.Context(), s.db, *p.BirthdayID)
		if bdErr == nil {
			data.Birthday = &bd
		}
	}
	s.templates.Render(w, r, "people_form.html", data)
}

// handlePersonUpdate processes the edit-person form submission.
func (s *Server) handlePersonUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := store.GetPerson(r.Context(), s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.templates.Render(w, r, "people_form.html", personFormData{Person: &existing, Error: "Name is required."})
		return
	}

	newBirthdayID, err := updateBlurryDate(r.Context(), s.db, parseBirthdayForm(r), existing.BirthdayID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	existing.Name = name
	existing.Nickname = nullableFormValue(r, "nickname")
	existing.Notes = nullableFormValue(r, "notes")
	existing.BirthdayID = newBirthdayID
	if _, err := store.UpdatePerson(r.Context(), s.db, existing); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people/"+id, http.StatusSeeOther)
}

// handlePersonDelete deletes a person and redirects to the people list.
func (s *Server) handlePersonDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := store.DeletePerson(r.Context(), s.db, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people", http.StatusSeeOther)
}

// handleInteractionCreate logs a new interaction. Called by HTMX; returns a
// single table row to be prepended to the interactions list.
func (s *Server) handleInteractionCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// HTML datetime-local inputs produce "2006-01-02T15:04" format.
	occurredAt, err := time.Parse("2006-01-02T15:04", r.FormValue("occurred_at"))
	if err != nil {
		http.Error(w, "invalid occurred_at: expected format 2006-01-02T15:04", http.StatusBadRequest)
		return
	}
	i, err := store.CreateInteraction(r.Context(), s.db, model.Interaction{
		PersonID:   id,
		OccurredAt: occurredAt,
		Channel:    nullableFormValue(r, "channel"),
		Notes:      nullableFormValue(r, "notes"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "interaction_row.html", i)
}

// handleContactMethodCreate adds a contact method. Called by HTMX; returns a
// single table row to be appended to the contact methods list.
func (s *Server) handleContactMethodCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cm, err := store.CreateContactMethod(r.Context(), s.db, model.ContactMethod{
		PersonID: id,
		Type:     model.ContactMethodType(r.FormValue("type")),
		Value:    r.FormValue("value"),
		Label:    nullableFormValue(r, "label"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "contact_method_row.html", cm)
}

// handleContactMethodDelete removes a contact method. Called by HTMX with
// hx-swap="outerHTML" on the table row; returns an empty 200 so HTMX removes
// the row from the DOM.
func (s *Server) handleContactMethodDelete(w http.ResponseWriter, r *http.Request) {
	methodID := r.PathValue("method_id")
	if err := store.DeleteContactMethod(r.Context(), s.db, methodID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleRelationshipCreate adds a relationship. Called by HTMX; returns a
// single table row to be appended to the relationships list.
func (s *Server) handleRelationshipCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rel, err := store.CreateRelationship(r.Context(), s.db, model.Relationship{
		PersonID: id,
		ToType:   model.RelationshipTarget(r.FormValue("to_type")),
		ToID:     r.FormValue("to_id"),
		Label:    r.FormValue("label"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	toName, _ := s.resolveEntityName(r.Context(), rel.ToType, rel.ToID)
	s.templates.Render(w, r, "relationship_row.html", relationshipRow{
		Relationship: rel,
		ToName:       toName,
	})
}

// handleRelationshipDelete removes a relationship. Called by HTMX; returns
// empty 200 so HTMX removes the row.
func (s *Server) handleRelationshipDelete(w http.ResponseWriter, r *http.Request) {
	relID := r.PathValue("id")
	if err := store.DeleteRelationship(r.Context(), s.db, relID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// nullableFormValue returns a pointer to the trimmed form value if non-empty,
// otherwise nil. Used to convert optional HTML form fields to *string.
func nullableFormValue(r *http.Request, key string) *string {
	v := r.FormValue(key)
	if v == "" {
		return nil
	}
	return &v
}

// parseBirthdayForm reads birthday fields from the request form and returns a
// BlurryDate, or nil if all birthday fields are blank (no birthday provided).
func parseBirthdayForm(r *http.Request) *model.BlurryDate {
	yearStr := r.FormValue("birthday_year")
	monthStr := r.FormValue("birthday_month")
	dayStr := r.FormValue("birthday_day")
	if yearStr == "" && monthStr == "" && dayStr == "" {
		return nil
	}
	reminderEnabled := r.FormValue("reminder_enabled") == "on"
	reminderDaysBefore := 1
	if v, err := strconv.Atoi(r.FormValue("reminder_days_before")); err == nil {
		reminderDaysBefore = v
	}
	bd := &model.BlurryDate{
		ReminderEnabled:    reminderEnabled,
		ReminderDaysBefore: reminderDaysBefore,
	}
	if v, err := strconv.Atoi(yearStr); err == nil {
		bd.Year = &v
	}
	if v, err := strconv.Atoi(monthStr); err == nil {
		bd.Month = &v
	}
	if v, err := strconv.Atoi(dayStr); err == nil {
		bd.Day = &v
	}
	return bd
}

// updateBlurryDate handles create/update/delete lifecycle for a blurry date
// linked to a parent entity.
//   - input nil, existingID nil  → no-op, returns nil
//   - input nil, existingID set  → deletes orphaned row, returns nil
//   - input set, existingID nil  → creates new row, returns its ID
//   - input set, existingID set  → updates row in place, returns its ID
func updateBlurryDate(ctx context.Context, db *sql.DB, input *model.BlurryDate, existingID *string) (*string, error) {
	if input == nil {
		if existingID != nil {
			_ = store.DeleteBlurryDate(ctx, db, *existingID) // ignore not-found
		}
		return nil, nil
	}
	if existingID != nil {
		input.ID = *existingID
		if err := store.UpdateBlurryDate(ctx, db, *input); err != nil {
			return nil, err
		}
		return existingID, nil
	}
	created, err := store.CreateBlurryDate(ctx, db, *input)
	if err != nil {
		return nil, err
	}
	return &created.ID, nil
}

// formatBlurryDate produces a human-readable string from a BlurryDate.
// Returns empty string for a nil input.
func formatBlurryDate(d model.BlurryDate) string {
	if d.Day != nil && d.Month != nil && d.Year != nil {
		return fmt.Sprintf("%d %s %d", *d.Day, time.Month(*d.Month).String(), *d.Year)
	}
	if d.Day != nil && d.Month != nil {
		return fmt.Sprintf("%d %s", *d.Day, time.Month(*d.Month).String())
	}
	if d.Month != nil && d.Year != nil {
		return fmt.Sprintf("%s %d", time.Month(*d.Month).String(), *d.Year)
	}
	if d.Year != nil {
		return strconv.Itoa(*d.Year)
	}
	return ""
}
```

- [ ] **Add routes to `internal/web/server.go`**

Inside `New()`, after the existing `protected.HandleFunc("GET /", s.handleIndex)` line, add:

```go
protected.HandleFunc("GET /people", s.handlePeopleList)
protected.HandleFunc("GET /people/new", s.handlePersonNew)
protected.HandleFunc("POST /people/new", s.handlePersonCreate)
protected.HandleFunc("GET /people/{id}", s.handlePersonDetail)
protected.HandleFunc("GET /people/{id}/edit", s.handlePersonEdit)
protected.HandleFunc("POST /people/{id}/edit", s.handlePersonUpdate)
protected.HandleFunc("POST /people/{id}/delete", s.handlePersonDelete)
protected.HandleFunc("POST /people/{id}/interactions", s.handleInteractionCreate)
protected.HandleFunc("POST /people/{id}/contact-methods", s.handleContactMethodCreate)
protected.HandleFunc("DELETE /people/{id}/contact-methods/{method_id}", s.handleContactMethodDelete)
protected.HandleFunc("POST /people/{id}/relationships", s.handleRelationshipCreate)
protected.HandleFunc("DELETE /relationships/{id}", s.handleRelationshipDelete)
```

- [ ] **Verify build**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go build ./...
```

Expected: clean build.

---

## Task 2: Person detail + form templates

**Files:**
- Create: `assets/templates/people_detail.html`
- Create: `assets/templates/people_form.html`

- [ ] **Create `assets/templates/people_detail.html`**

This file defines five named templates: the main detail page, plus four row partials used by HTMX swaps.

```html
{{define "people_detail.html"}}
<div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:1rem">
  <hgroup>
    <h1>{{.Person.Name}}</h1>
    {{if .Person.Nickname}}<p>{{deref .Person.Nickname}}</p>{{end}}
  </hgroup>
  <div style="display:flex; gap:0.5rem; align-items:center">
    <a href="/people/{{.Person.ID}}/edit" role="button" class="outline">Edit</a>
    <form method="POST" action="/people/{{.Person.ID}}/delete" style="margin:0">
      <button type="submit" class="secondary outline"
        onclick="return confirm('Delete {{.Person.Name}}? This cannot be undone.')">Delete</button>
    </form>
  </div>
</div>

{{if .Person.Notes}}<p>{{deref .Person.Notes}}</p>{{end}}
{{if .BirthdayDisplay}}<p><strong>Birthday:</strong> {{.BirthdayDisplay}}</p>{{end}}

<article>
  <header><strong>Contact Methods</strong></header>
  <table>
    <tbody id="contact-methods-list">
      {{range .ContactMethods}}{{template "contact_method_row.html" .}}{{end}}
    </tbody>
  </table>
  <form hx-post="/people/{{.Person.ID}}/contact-methods"
        hx-target="#contact-methods-list"
        hx-swap="beforeend"
        hx-on::after-request="this.reset()">
    <div class="grid">
      <select name="type" required>
        <option value="email">Email</option>
        <option value="phone">Phone</option>
        <option value="social">Social</option>
        <option value="website">Website</option>
      </select>
      <input name="value" placeholder="Value" required>
      <input name="label" placeholder="Label (optional)">
      <button type="submit">Add</button>
    </div>
  </form>
</article>

<article>
  <header><strong>Interactions</strong></header>
  <table>
    <thead><tr><th>Date</th><th>Channel</th><th>Notes</th></tr></thead>
    <tbody id="interactions-list">
      {{range .Interactions}}{{template "interaction_row.html" .}}{{end}}
    </tbody>
  </table>
  <form hx-post="/people/{{.Person.ID}}/interactions"
        hx-target="#interactions-list"
        hx-swap="afterbegin"
        hx-on::after-request="this.reset()">
    <div class="grid">
      <input type="datetime-local" name="occurred_at" required>
      <input name="channel" placeholder="Channel (e.g. phone)">
      <input name="notes" placeholder="Notes">
      <button type="submit">Log</button>
    </div>
  </form>
</article>

<article>
  <header><strong>Relationships</strong></header>
  <table>
    <thead><tr><th>Label</th><th>Type</th><th>Name / ID</th><th></th></tr></thead>
    <tbody id="relationships-list">
      {{range .Relationships}}{{template "relationship_row.html" .}}{{end}}
    </tbody>
  </table>
  <form hx-post="/people/{{.Person.ID}}/relationships"
        hx-target="#relationships-list"
        hx-swap="beforeend"
        hx-on::after-request="this.reset()">
    <div class="grid">
      <input name="label" placeholder="Label (e.g. friend)" required>
      <select name="to_type" required>
        <option value="person">Person</option>
        <option value="address">Address</option>
        <option value="pet">Pet</option>
        <option value="work">Work</option>
      </select>
      <input name="to_id" placeholder="Entity ID" required>
      <button type="submit">Add</button>
    </div>
  </form>
</article>
{{end}}

{{define "interaction_row.html"}}
<tr>
  <td>{{.OccurredAt.Format "2006-01-02 15:04"}}</td>
  <td>{{deref .Channel}}</td>
  <td>{{deref .Notes}}</td>
</tr>
{{end}}

{{define "contact_method_row.html"}}
<tr>
  <td>{{.Type}}</td>
  <td>{{.Value}}</td>
  <td>{{deref .Label}}</td>
  <td>
    <button hx-delete="/people/{{.PersonID}}/contact-methods/{{.ID}}"
            hx-target="closest tr"
            hx-swap="outerHTML"
            hx-confirm="Remove this contact method?"
            class="outline secondary">✕</button>
  </td>
</tr>
{{end}}

{{define "relationship_row.html"}}
<tr>
  <td>{{.Relationship.Label}}</td>
  <td>{{.Relationship.ToType}}</td>
  <td>{{.ToName}}</td>
  <td>
    <button hx-delete="/relationships/{{.Relationship.ID}}"
            hx-target="closest tr"
            hx-swap="outerHTML"
            hx-confirm="Remove this relationship?"
            class="outline secondary">✕</button>
  </td>
</tr>
{{end}}
```

- [ ] **Create `assets/templates/people_form.html`**

```html
{{define "people_form.html"}}
<hgroup>
  <h1>{{if .Person}}Edit {{.Person.Name}}{{else}}New Person{{end}}</h1>
</hgroup>
{{if .Error}}<p><mark>{{.Error}}</mark></p>{{end}}
<form method="POST" action="{{if .Person}}/people/{{.Person.ID}}/edit{{else}}/people/new{{end}}">
  <label>
    Name *
    <input name="name" value="{{if .Person}}{{.Person.Name}}{{end}}" required autofocus>
  </label>
  <label>
    Nickname
    <input name="nickname" value="{{if .Person}}{{deref .Person.Nickname}}{{end}}">
  </label>
  <label>
    Notes
    <textarea name="notes">{{if .Person}}{{deref .Person.Notes}}{{end}}</textarea>
  </label>
  <fieldset>
    <legend>Birthday (all fields optional)</legend>
    <div class="grid">
      <label>Year
        <input type="number" name="birthday_year" min="1900" max="2100"
          value="{{if .Birthday}}{{if .Birthday.Year}}{{.Birthday.Year}}{{end}}{{end}}">
      </label>
      <label>Month
        <input type="number" name="birthday_month" min="1" max="12"
          value="{{if .Birthday}}{{if .Birthday.Month}}{{.Birthday.Month}}{{end}}{{end}}">
      </label>
      <label>Day
        <input type="number" name="birthday_day" min="1" max="31"
          value="{{if .Birthday}}{{if .Birthday.Day}}{{.Birthday.Day}}{{end}}{{end}}">
      </label>
    </div>
    <label>
      <input type="checkbox" name="reminder_enabled" role="switch"
        {{if .Birthday}}{{if .Birthday.ReminderEnabled}}checked{{end}}{{else}}checked{{end}}>
      Send reminders
    </label>
    <label>
      Days before reminder
      <input type="number" name="reminder_days_before" min="0"
        value="{{if .Birthday}}{{.Birthday.ReminderDaysBefore}}{{else}}1{{end}}">
    </label>
  </fieldset>
  <div class="grid">
    <button type="submit">{{if .Person}}Save{{else}}Create{{end}}</button>
    <a href="{{if .Person}}/people/{{.Person.ID}}{{else}}/people{{end}}"
       role="button" class="secondary outline">Cancel</a>
  </div>
</form>
{{end}}
```

- [ ] **Verify build + template parsing**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go build ./...
go test -buildvcs=false -run TestTemplates ./internal/web/...
```

The existing `TestTemplates` test (which calls `ParseTemplates`) will catch any malformed templates. Expected: PASS.

- [ ] **Commit**

```bash
git add internal/web/render.go internal/web/people.go internal/web/server.go \
        assets/templates/people_list.html assets/templates/people_detail.html \
        assets/templates/people_form.html
git commit -m "feat: phase 4 web UI — people CRUD with HTMX interactions"
```

---

## Task 3: Write web handler tests

**Files:**
- Create: `internal/web/people_test.go`

Tests use a real in-memory SQLite database and the full server stack with `DisableBasicAuth: true`.

- [ ] **Create `internal/web/people_test.go`**

```go
package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/assets"
	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/web"
)

func newWebTestServer(t *testing.T) *web.Server {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	srv, err := web.New(web.Config{
		Listen:       ":0",
		PasswordHash: "",
		AppConfig: &config.Config{
			Listen:           ":0",
			DBPath:           ":memory:",
			DisableBasicAuth: true,
			ReminderEmail:    "test@example.com",
			ReminderSendTime: "08:00",
			ReminderTimezone: "UTC",
		},
	}, sqlDB, assets.FS)
	require.NoError(t, err)
	return srv
}

func doWeb(t *testing.T, srv *web.Server, method, path string, body url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, path, strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, path, nil)
		require.NoError(t, err)
	}
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func doHTMX(t *testing.T, srv *web.Server, method, path string, body url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	var err error
	if body != nil {
		req, err = http.NewRequest(method, path, strings.NewReader(body.Encode()))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, path, nil)
		require.NoError(t, err)
	}
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	return rec
}

func TestPeopleList_Empty(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "No people yet")
}

func TestPeopleList_HTMX_LiveSearch(t *testing.T) {
	srv := newWebTestServer(t)

	// Create two people via the form.
	doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Alice"}})
	doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Bob"}})

	// HTMX search for "ali" should return only the rows fragment containing Alice.
	rec := doHTMX(t, srv, http.MethodGet, "/people?name=ali", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "Alice")
	assert.NotContains(t, body, "Bob")
	// Full base layout should NOT be present in HTMX response.
	assert.NotContains(t, body, "<html")
}

func TestPersonNew_Form(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people/new", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "New Person")
}

func TestPersonCreate_And_Detail(t *testing.T) {
	srv := newWebTestServer(t)

	// POST creates the person and redirects.
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{
		"name":     {"Carol"},
		"nickname": {"CC"},
		"notes":    {"test notes"},
	})
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	location := rec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "/people/"), "redirect should be to /people/{id}")

	// Follow redirect to detail page.
	rec2 := doWeb(t, srv, http.MethodGet, location, nil)
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "Carol")
	assert.Contains(t, body, "CC")
	assert.Contains(t, body, "test notes")
}

func TestPersonCreate_EmptyName_Returns400(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {""}})
	// Returns the form with an error, not a redirect.
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Name is required")
}

func TestPersonDetail_NotFound(t *testing.T) {
	srv := newWebTestServer(t)
	rec := doWeb(t, srv, http.MethodGet, "/people/nonexistent-id", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestPersonEdit_And_Update(t *testing.T) {
	srv := newWebTestServer(t)

	// Create.
	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Dave"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	// GET edit form.
	rec2 := doWeb(t, srv, http.MethodGet, "/people/"+id+"/edit", nil)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Contains(t, rec2.Body.String(), "Edit Dave")

	// POST update.
	rec3 := doWeb(t, srv, http.MethodPost, "/people/"+id+"/edit", url.Values{
		"name":  {"David"},
		"notes": {"updated"},
	})
	assert.Equal(t, http.StatusSeeOther, rec3.Code)

	// Verify update.
	rec4 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Contains(t, rec4.Body.String(), "David")
	assert.Contains(t, rec4.Body.String(), "updated")
}

func TestPersonDelete(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Eve"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodPost, "/people/"+id+"/delete", nil)
	assert.Equal(t, http.StatusSeeOther, rec2.Code)
	assert.Equal(t, "/people", rec2.Header().Get("Location"))

	rec3 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Equal(t, http.StatusNotFound, rec3.Code)
}

func TestInteractionCreate_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Frank"}})
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doHTMX(t, srv, http.MethodPost, "/people/"+id+"/interactions", url.Values{
		"occurred_at": {"2025-06-01T10:00"},
		"channel":     {"phone"},
		"notes":       {"called about lunch"},
	})
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "2025-06-01")
	assert.Contains(t, body, "phone")
}

func TestContactMethod_CreateAndDelete_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Grace"}})
	personID := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	// Add contact method via HTMX.
	rec2 := doHTMX(t, srv, http.MethodPost, "/people/"+personID+"/contact-methods", url.Values{
		"type":  {"email"},
		"value": {"grace@example.com"},
	})
	assert.Equal(t, http.StatusOK, rec2.Code)
	body := rec2.Body.String()
	assert.Contains(t, body, "grace@example.com")

	// Extract contact method ID from the hx-delete attribute in the row.
	// The row contains: hx-delete="/people/{personID}/contact-methods/{id}"
	prefix := `/people/` + personID + `/contact-methods/`
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "hx-delete attribute not found in response")
	rest := body[idx+len(prefix):]
	methodID := rest[:strings.IndexAny(rest, `"' `)]

	// Delete via HTMX.
	rec3 := doHTMX(t, srv, http.MethodDelete, "/people/"+personID+"/contact-methods/"+methodID, nil)
	assert.Equal(t, http.StatusOK, rec3.Code)
}

func TestRelationship_CreateAndDelete_HTMX(t *testing.T) {
	srv := newWebTestServer(t)

	rec1 := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Heidi"}})
	p1ID := strings.TrimPrefix(rec1.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{"name": {"Ivan"}})
	p2ID := strings.TrimPrefix(rec2.Header().Get("Location"), "/people/")

	// Add relationship via HTMX.
	rec3 := doHTMX(t, srv, http.MethodPost, "/people/"+p1ID+"/relationships", url.Values{
		"label":   {"friend"},
		"to_type": {"person"},
		"to_id":   {p2ID},
	})
	assert.Equal(t, http.StatusOK, rec3.Code)
	body := rec3.Body.String()
	assert.Contains(t, body, "friend")
	assert.Contains(t, body, "Ivan")

	// Extract relationship ID from hx-delete="/relationships/{id}".
	prefix := `/relationships/`
	idx := strings.Index(body, prefix)
	require.True(t, idx >= 0, "hx-delete attribute not found")
	rest := body[idx+len(prefix):]
	relID := rest[:strings.IndexAny(rest, `"' `)]

	// Delete via HTMX.
	rec4 := doHTMX(t, srv, http.MethodDelete, "/relationships/"+relID, nil)
	assert.Equal(t, http.StatusOK, rec4.Code)
}

func TestPersonCreate_WithBirthday(t *testing.T) {
	srv := newWebTestServer(t)

	rec := doWeb(t, srv, http.MethodPost, "/people/new", url.Values{
		"name":                 {"Julia"},
		"birthday_year":        {"1990"},
		"birthday_month":       {"6"},
		"birthday_day":         {"15"},
		"reminder_enabled":     {"on"},
		"reminder_days_before": {"3"},
	})
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	id := strings.TrimPrefix(rec.Header().Get("Location"), "/people/")

	rec2 := doWeb(t, srv, http.MethodGet, "/people/"+id, nil)
	assert.Contains(t, rec2.Body.String(), "15 June 1990")
}
```

- [ ] **Run the tests**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test -buildvcs=false -race -cover ./internal/web/...
```

Expected: all tests pass. Fix any failures before proceeding.

---

## Task 4: Final verification and commit

**Files:**
- No new files — verification only

- [ ] **Run full test suite**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
go test -buildvcs=false -race -cover ./...
```

Expected: all packages pass.

- [ ] **Run linter**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
golangci-lint run
```

Expected: 0 issues. Common issues to watch for:
- Unused imports
- `godot` lint for comments missing periods
- `revive` for exported function comments

- [ ] **Smoke test the running server**

The dev server should be running (`go run ./cmd/aurelianprm serve --config ./config.dev.json`). Visit:
- `http://localhost:8080/people` — should show empty list with "+ Add Person"
- `http://localhost:8080/people/new` — should show the create form
- Create a person, verify redirect to detail page
- Add a contact method, interaction, and relationship from the detail page

- [ ] **Commit**

```bash
cd /Users/ben/projects/github.com/astromechza/aurelianprm
git add internal/web/ assets/templates/ go.mod go.sum
git commit -m "feat: phase 4 web UI — people CRUD with HTMX interactions"
```

---

## Self-Review

**Spec coverage:**
- ✅ People list with search (HTMX live search on `?name=`)
- ✅ Person detail page (name, nickname, notes, birthday, contact methods, interactions, relationships)
- ✅ Create person form (with birthday)
- ✅ Edit person form (with birthday update/clear)
- ✅ Delete person (POST + redirect)
- ✅ HTMX interaction logging (POST appends row without page reload)
- ✅ HTMX contact method add/delete
- ✅ HTMX relationship add/delete (with entity name resolution)
- ✅ Pico CSS + HTMX (already vendored in static assets)
- ✅ `HX-Request` header check (live search returns rows-only fragment)
- ✅ All templates parsed eagerly via `template.Must` (server refuses to start on malformed template)

**No placeholders:** All code is complete and concrete.

**Type consistency:**
- `personDetailData.BirthdayDisplay string` used in template as `{{.BirthdayDisplay}}` ✅
- `relationshipRow{Relationship model.Relationship, ToName string}` used in template as `{{.Relationship.Label}}`, `{{.ToName}}` ✅
- `personFormData{Person *model.Person, Birthday *model.BlurryDate}` — template checks `{{if .Person}}` and `{{if .Birthday}}` ✅
- `formatBlurryDate` takes `model.BlurryDate` (not pointer) — `buildPersonDetailData` passes `bd` (value, after nil check) ✅
- `updateBlurryDate` takes `*model.BlurryDate` as input — `parseBirthdayForm` returns `*model.BlurryDate` ✅

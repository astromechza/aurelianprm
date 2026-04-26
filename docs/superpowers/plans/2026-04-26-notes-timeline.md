# Notes Timeline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a per-person timeline of editable notes (LIFE_EVENT, CONVERSATION, HANGOUT) with multi-person linking, inline HTMX editing, and a JSON API.

**Architecture:** New `notes` + `note_persons` tables via goose migration. New `internal/dal/notes.go` with full CRUD + list methods. New `internal/web/handlers_notes.go` for HTMX web handlers. Five JSON API handlers added to `handlers_api.go`. Templates follow existing patterns: `<tr data-note-row>` partials, inline HTMX swap.

**Tech Stack:** Go 1.26, SQLite (goose migrations), HTMX 2, Pico CSS 2, chi router, testify.

---

## File Map

| File | Action | Purpose |
|------|--------|---------|
| `internal/db/migrations/00004_create_notes.sql` | Create | Schema migration |
| `internal/dal/notes.go` | Create | Note types, DAL methods |
| `internal/dal/dal_test.go` | Modify | Append note DAL tests |
| `internal/web/viewmodels.go` | Modify | Add NoteRowView, NoteFormView; extend PersonDetailView |
| `internal/web/server.go` | Modify | Register routes; add noteIcon/noteLabel/noteRow/notePersonsExcept to FuncMap |
| `internal/web/handlers_notes.go` | Create | 5 HTMX web handlers |
| `internal/web/handlers_persons.go` | Modify | handlePersonsList: add link_rel=note/note-edit cases; handlePersonsDetail: load notes + today |
| `internal/web/handlers_api.go` | Modify | 5 JSON API handlers |
| `internal/web/templates/partials/note_row.html` | Create | Read-only note table row |
| `internal/web/templates/partials/note_edit_form.html` | Create | Inline edit form table row |
| `internal/web/templates/partials/note_person_search_rows.html` | Create | Person search results for note create form |
| `internal/web/templates/partials/note_person_search_rows_edit.html` | Create | Person search results for note edit form |
| `internal/web/templates/persons_detail.html` | Modify | Add Timeline article section |
| `internal/web/web_test.go` | Modify | Append API integration tests |

---

## Task 1: Database migration

**Files:**
- Create: `internal/db/migrations/00004_create_notes.sql`

- [ ] **Step 1: Write the migration file**

```sql
-- +goose Up
CREATE TABLE notes (
    id         TEXT     PRIMARY KEY,
    type       TEXT     NOT NULL CHECK(type IN ('LIFE_EVENT','CONVERSATION','HANGOUT')),
    date       TEXT     NOT NULL,
    content    TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX notes_date ON notes(date);

CREATE TABLE note_persons (
    note_id   TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
    person_id TEXT NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    PRIMARY KEY (note_id, person_id)
);
CREATE INDEX note_persons_person ON note_persons(person_id);

-- +goose Down
DROP TABLE note_persons;
DROP TABLE notes;
```

- [ ] **Step 2: Verify schema tests pass**

```bash
go test ./internal/db/... ./internal/schema/... -v -count=1
```

Expected: all PASS (goose picks up the new migration automatically).

- [ ] **Step 3: Commit**

```bash
git add internal/db/migrations/00004_create_notes.sql
git commit -m "feat: add notes and note_persons schema migration"
```

---

## Task 2: DAL — `internal/dal/notes.go`

**Files:**
- Create: `internal/dal/notes.go`
- Modify: `internal/dal/dal_test.go`

- [ ] **Step 1: Write the failing DAL tests first**

Append to `internal/dal/dal_test.go`:

```go
func TestCreateNote_and_GetNote(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Alice"}`),
		})
		personID = p.ID
		return err
	})
	require.NoError(t, err)

	var note dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		note, err = q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "CONVERSATION",
			Date:      "2026-04-26",
			Content:   "Caught up over coffee.",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, note.ID)
	require.Equal(t, "CONVERSATION", note.Type)
	require.Equal(t, "2026-04-26", note.Date)
	require.Equal(t, "Caught up over coffee.", note.Content)
	require.Len(t, note.Persons, 1)
	require.Equal(t, personID, note.Persons[0].ID)

	var fetched dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetNote(ctx, note.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, note.ID, fetched.ID)
	require.Len(t, fetched.Persons, 1)
}

func TestGetNote_notFound(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetNote(ctx, "NOTEXIST")
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestCreateNote_invalidType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Bob"}`),
		})
		personID = p.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "INVALID",
			Date:      "2026-04-26",
			Content:   "test",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateNote_invalidDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Carol"}`),
		})
		personID = p.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "HANGOUT",
			Date:      "2026-4-1", // wrong format
			Content:   "test",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateNote_nonPersonEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var emailID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress", Data: json.RawMessage(`{"email":"x@x.com"}`),
		})
		emailID = e.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "CONVERSATION",
			Date:      "2026-04-26",
			Content:   "test",
			PersonIDs: []string{emailID},
		})
		return err
	})
	require.Error(t, err)
}

func TestUpdateNote_replacesPersons(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var p1ID, p2ID, noteID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p1, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Dave"}`),
		})
		if err != nil {
			return err
		}
		p1ID = p1.ID
		p2, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Eve"}`),
		})
		if err != nil {
			return err
		}
		p2ID = p2.ID
		note, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "HANGOUT",
			Date:      "2026-04-26",
			Content:   "initial",
			PersonIDs: []string{p1ID},
		})
		noteID = note.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateNote(ctx, dal.UpdateNoteParams{
			ID:        noteID,
			Type:      "CONVERSATION",
			Date:      "2026-04-27",
			Content:   "updated",
			PersonIDs: []string{p1ID, p2ID},
		})
	})
	require.NoError(t, err)

	var fetched dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetNote(ctx, noteID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, "CONVERSATION", fetched.Type)
	require.Equal(t, "2026-04-27", fetched.Date)
	require.Equal(t, "updated", fetched.Content)
	require.Len(t, fetched.Persons, 2)
}

func TestDeleteNote_cascades(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var noteID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Frank"}`),
		})
		if err != nil {
			return err
		}
		note, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "LIFE_EVENT",
			Date:      "2026-04-26",
			Content:   "graduated",
			PersonIDs: []string{p.ID},
		})
		noteID = note.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteNote(ctx, noteID)
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM note_persons WHERE note_id=?`, noteID).Scan(&count))
	require.Equal(t, 0, count)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetNote(ctx, noteID)
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestListNotesForPerson_orderedByDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Grace"}`),
		})
		if err != nil {
			return err
		}
		personID = p.ID
		for _, date := range []string{"2026-01-01", "2026-03-15", "2026-02-10"} {
			if _, err := q.CreateNote(ctx, dal.CreateNoteParams{
				Type:      "HANGOUT",
				Date:      date,
				Content:   "note on " + date,
				PersonIDs: []string{personID},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	var notes []dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		notes, err = q.ListNotesForPerson(ctx, personID)
		return err
	})
	require.NoError(t, err)
	require.Len(t, notes, 3)
	// Most recent first
	require.Equal(t, "2026-03-15", notes[0].Date)
	require.Equal(t, "2026-02-10", notes[1].Date)
	require.Equal(t, "2026-01-01", notes[2].Date)
}
```

- [ ] **Step 2: Run tests — expect FAIL (notes.go not yet written)**

```bash
go test ./internal/dal/... -count=1 -run "TestCreateNote|TestGetNote|TestUpdateNote|TestDeleteNote|TestListNotes" -v 2>&1 | tail -20
```

Expected: FAIL with "undefined: dal.NoteWithPersons" or similar.

- [ ] **Step 3: Create `internal/dal/notes.go`**

```go
package dal

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"
)

// NoteTypes maps type constants to display icon and label.
var NoteTypes = map[string]struct{ Icon, Label string }{
	"LIFE_EVENT":   {Icon: "🎯", Label: "Life Event"},
	"CONVERSATION": {Icon: "💬", Label: "Conversation"},
	"HANGOUT":      {Icon: "🤝", Label: "Hangout"},
}

var noteDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Note represents a row in the notes table.
type Note struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Date      string    `json:"date"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// NoteWithPersons pairs a note with its linked person entities.
type NoteWithPersons struct {
	Note
	Persons []Entity `json:"persons"`
}

// CreateNoteParams holds fields for inserting a new note.
type CreateNoteParams struct {
	Type      string
	Date      string   // YYYY-MM-DD
	Content   string
	PersonIDs []string // must be non-empty; all must resolve to type=Person
}

// UpdateNoteParams holds fields for updating an existing note.
type UpdateNoteParams struct {
	ID        string
	Type      string
	Date      string
	Content   string
	PersonIDs []string
}

func validateNoteFields(noteType, date, content string, personIDs []string) error {
	if _, ok := NoteTypes[noteType]; !ok {
		return fmt.Errorf("unknown note type %q", noteType)
	}
	if !noteDateRE.MatchString(date) {
		return fmt.Errorf("date %q must be YYYY-MM-DD", date)
	}
	if content == "" {
		return fmt.Errorf("content must not be empty")
	}
	if len(personIDs) == 0 {
		return fmt.Errorf("at least one person_id is required")
	}
	return nil
}

func scanNote(row interface{ Scan(...any) error }) (Note, error) {
	var n Note
	err := row.Scan(&n.ID, &n.Type, &n.Date, &n.Content, &n.CreatedAt, &n.UpdatedAt)
	return n, err
}

// fetchNotePersons retrieves all person entities linked to a note.
func (q *Queries) fetchNotePersons(ctx context.Context, noteID string) ([]Entity, error) {
	const stmt = `
		SELECT e.id, e.type, e.display_name, e.data, e.created_at, e.updated_at
		FROM entities e
		JOIN note_persons np ON np.person_id = e.id
		WHERE np.note_id = ?
		ORDER BY e.display_name, e.id`
	rows, err := q.db.QueryContext(ctx, stmt, noteID)
	if err != nil {
		return nil, fmt.Errorf("fetch note persons: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var persons []Entity
	for rows.Next() {
		e, err := scanEntity(rows)
		if err != nil {
			return nil, fmt.Errorf("scan note person: %w", err)
		}
		persons = append(persons, e)
	}
	return persons, rows.Err()
}

// insertNotePersons inserts note_persons rows. Deduplicates and validates each person.
func (q *Queries) insertNotePersons(ctx context.Context, noteID string, personIDs []string) error {
	seen := make(map[string]bool)
	for _, pid := range personIDs {
		if seen[pid] {
			continue
		}
		seen[pid] = true
		e, err := q.GetEntity(ctx, pid)
		if err != nil {
			return fmt.Errorf("note person %q: %w", pid, err)
		}
		if e.Type != "Person" {
			return fmt.Errorf("note person %q has type %q, want Person", pid, e.Type)
		}
		if _, err := q.db.ExecContext(ctx,
			`INSERT INTO note_persons (note_id, person_id) VALUES (?, ?)`,
			noteID, pid,
		); err != nil {
			return fmt.Errorf("insert note_person: %w", err)
		}
	}
	return nil
}

// CreateNote validates and inserts a note with its person links.
func (q *Queries) CreateNote(ctx context.Context, params CreateNoteParams) (NoteWithPersons, error) {
	if err := validateNoteFields(params.Type, params.Date, params.Content, params.PersonIDs); err != nil {
		return NoteWithPersons{}, fmt.Errorf("create note: %w", err)
	}
	const stmt = `
		INSERT INTO notes (id, type, date, content)
		VALUES (?, ?, ?, ?)
		RETURNING id, type, date, content, created_at, updated_at`
	note, err := scanNote(q.db.QueryRowContext(ctx, stmt, newID(), params.Type, params.Date, params.Content))
	if err != nil {
		return NoteWithPersons{}, fmt.Errorf("create note: %w", err)
	}
	if err := q.insertNotePersons(ctx, note.ID, params.PersonIDs); err != nil {
		return NoteWithPersons{}, fmt.Errorf("create note: %w", err)
	}
	persons, err := q.fetchNotePersons(ctx, note.ID)
	if err != nil {
		return NoteWithPersons{}, err
	}
	return NoteWithPersons{Note: note, Persons: persons}, nil
}

// GetNote fetches a note with its linked persons. Returns sql.ErrNoRows if not found.
func (q *Queries) GetNote(ctx context.Context, id string) (NoteWithPersons, error) {
	const stmt = `SELECT id, type, date, content, created_at, updated_at FROM notes WHERE id = ?`
	note, err := scanNote(q.db.QueryRowContext(ctx, stmt, id))
	if err != nil {
		return NoteWithPersons{}, err
	}
	persons, err := q.fetchNotePersons(ctx, note.ID)
	if err != nil {
		return NoteWithPersons{}, err
	}
	return NoteWithPersons{Note: note, Persons: persons}, nil
}

// UpdateNote validates and updates a note's fields and person links atomically.
func (q *Queries) UpdateNote(ctx context.Context, params UpdateNoteParams) error {
	if err := validateNoteFields(params.Type, params.Date, params.Content, params.PersonIDs); err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	const stmt = `
		UPDATE notes SET type=?, date=?, content=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?`
	res, err := q.db.ExecContext(ctx, stmt, params.Type, params.Date, params.Content, params.ID)
	if err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("update note: id %q not found", params.ID)
	}
	if _, err := q.db.ExecContext(ctx, `DELETE FROM note_persons WHERE note_id=?`, params.ID); err != nil {
		return fmt.Errorf("update note: clear persons: %w", err)
	}
	return q.insertNotePersons(ctx, params.ID, params.PersonIDs)
}

// DeleteNote removes a note by ID. Cascades to note_persons.
func (q *Queries) DeleteNote(ctx context.Context, id string) error {
	if _, err := q.db.ExecContext(ctx, `DELETE FROM notes WHERE id=?`, id); err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	return nil
}

// ListNotesForPerson returns all notes linked to personID, ordered date DESC, created_at DESC.
func (q *Queries) ListNotesForPerson(ctx context.Context, personID string) ([]NoteWithPersons, error) {
	const stmt = `
		SELECT DISTINCT n.id, n.type, n.date, n.content, n.created_at, n.updated_at
		FROM notes n
		JOIN note_persons np ON np.note_id = n.id
		WHERE np.person_id = ?
		ORDER BY n.date DESC, n.created_at DESC`
	rows, err := q.db.QueryContext(ctx, stmt, personID)
	if err != nil {
		return nil, fmt.Errorf("list notes for person: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return q.scanNotesWithPersons(ctx, rows)
}

// ListAllNotes returns all notes ordered date DESC, created_at DESC.
func (q *Queries) ListAllNotes(ctx context.Context) ([]NoteWithPersons, error) {
	const stmt = `SELECT id, type, date, content, created_at, updated_at FROM notes ORDER BY date DESC, created_at DESC`
	rows, err := q.db.QueryContext(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("list all notes: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	return q.scanNotesWithPersons(ctx, rows)
}

func (q *Queries) scanNotesWithPersons(ctx context.Context, rows *sql.Rows) ([]NoteWithPersons, error) {
	var notes []NoteWithPersons
	for rows.Next() {
		note, err := scanNote(rows)
		if err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		persons, err := q.fetchNotePersons(ctx, note.ID)
		if err != nil {
			return nil, err
		}
		notes = append(notes, NoteWithPersons{Note: note, Persons: persons})
	}
	return notes, rows.Err()
}
```

- [ ] **Step 4: Run all DAL tests**

```bash
go test ./internal/dal/... -count=1 -v 2>&1 | tail -30
```

Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/dal/notes.go internal/dal/dal_test.go
git commit -m "feat: add notes DAL with CRUD, list, and multi-person linking"
```

---

## Task 3: View models, server routes, and handlePersonsList

**Files:**
- Modify: `internal/web/viewmodels.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/handlers_persons.go`

- [ ] **Step 1: Extend `viewmodels.go`**

Add after the `RelationshipFormView` struct (before `decodeDataMap`):

```go
// NoteRowView is the view model for a single note row partial.
type NoteRowView struct {
	Note            dal.NoteWithPersons
	CurrentPersonID string
}

// NoteFormView is the view model for the add/edit note form partial.
type NoteFormView struct {
	NoteID          string
	CurrentPersonID string
	EditMode        bool
	Type            string
	Date            string
	Content         string
	SelectedPersons []dal.Entity
}
```

Also extend `PersonDetailView` — add two fields:

```go
// PersonDetailView is the view model for the person detail page.
type PersonDetailView struct {
	Person     dal.Entity
	PersonData PersonData
	Sections   []EntitySection
	People     []PersonRelSection
	Notes      []dal.NoteWithPersons // timeline notes, date DESC
	Today      string                // YYYY-MM-DD for create form default
}
```

- [ ] **Step 2: Run tests to confirm no compile errors**

```bash
go build ./internal/web/... 2>&1
```

Expected: success.

- [ ] **Step 3: Add FuncMap entries and routes to `server.go`**

In `NewServer`, add to the `funcs` map (after `entityFormCreate`):

```go
"noteIcon": func(noteType string) string {
    if t, ok := dal.NoteTypes[noteType]; ok {
        return t.Icon
    }
    return "📝"
},
"noteLabel": func(noteType string) string {
    if t, ok := dal.NoteTypes[noteType]; ok {
        return t.Label
    }
    return noteType
},
"noteRow": func(note dal.NoteWithPersons, personID string) NoteRowView {
    return NoteRowView{Note: note, CurrentPersonID: personID}
},
"notePersonsExcept": func(persons []dal.Entity, excludeID string) []dal.Entity {
    var out []dal.Entity
    for _, p := range persons {
        if p.ID != excludeID {
            out = append(out, p)
        }
    }
    return out
},
```

In `Handler()`, add web routes after the relationships block:

```go
r.Post("/persons/{id}/notes", s.handleNotesCreate)
r.Get("/notes/{nid}/edit", s.handleNotesEditForm)
r.Get("/notes/{nid}/cancel", s.handleNotesCancel)
r.Put("/notes/{nid}", s.handleNotesUpdate)
r.Delete("/notes/{nid}", s.handleNotesDelete)
```

In the `/api` route group, add:

```go
r.Get("/notes", s.handleAPIListNotes)
r.Post("/notes", s.handleAPICreateNote)
r.Put("/notes/{nid}", s.handleAPIUpdateNote)
r.Delete("/notes/{nid}", s.handleAPIDeleteNote)
r.Get("/persons/{id}/notes", s.handleAPIListNotesForPerson)
```

- [ ] **Step 4: Update `handlePersonsList` in `handlers_persons.go`**

Find this block (around line 88):

```go
if isHTMX(r) {
    if linkRel != "" {
        s.render(w, "link-persons-rows", view)
    } else {
        s.render(w, "persons-rows", view)
    }
} else {
    s.render(w, "persons-list", view)
}
```

Replace with:

```go
if isHTMX(r) {
    switch linkRel {
    case "note":
        s.render(w, "note-person-search-rows", view)
    case "note-edit":
        s.render(w, "note-person-search-rows-edit", view)
    case "":
        s.render(w, "persons-rows", view)
    default:
        s.render(w, "link-persons-rows", view)
    }
} else {
    s.render(w, "persons-list", view)
}
```

- [ ] **Step 5: Build to confirm no compile errors**

```bash
go build ./internal/web/... 2>&1
```

Expected: fails with undefined `handleNotesCreate` etc. — that's fine, handlers come next.

- [ ] **Step 6: Commit viewmodels and server changes (no build yet)**

```bash
git add internal/web/viewmodels.go internal/web/server.go internal/web/handlers_persons.go
git commit -m "feat: add note view models, FuncMap helpers, routes, and person-search cases"
```

---

## Task 4: Web handlers and `handlePersonsDetail` update

**Files:**
- Create: `internal/web/handlers_notes.go`
- Modify: `internal/web/handlers_persons.go`

- [ ] **Step 1: Create `internal/web/handlers_notes.go`**

```go
package web

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/astromechza/aurelianprm/internal/dal"
)

func (s *Server) handleNotesCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	personID := chi.URLParam(r, "id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	// Collect and deduplicate person IDs; always include the page's person.
	seen := map[string]bool{personID: true}
	unique := []string{personID}
	for _, pid := range r.Form["person_id"] {
		if !seen[pid] {
			seen[pid] = true
			unique = append(unique, pid)
		}
	}
	var row NoteRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		note, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      r.FormValue("type"),
			Date:      r.FormValue("date"),
			Content:   r.FormValue("content"),
			PersonIDs: unique,
		})
		if err != nil {
			return err
		}
		row = NoteRowView{Note: note, CurrentPersonID: personID}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "note-row", row)
}

func (s *Server) handleNotesEditForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	personID := r.URL.Query().Get("person_id")
	var view NoteFormView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		note, err := q.GetNote(ctx, nid)
		if err != nil {
			return err
		}
		view = NoteFormView{
			NoteID:          nid,
			CurrentPersonID: personID,
			EditMode:        true,
			Type:            note.Type,
			Date:            note.Date,
			Content:         note.Content,
			SelectedPersons: note.Persons,
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
	s.render(w, "note-edit-form", view)
}

func (s *Server) handleNotesCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	personID := r.URL.Query().Get("person_id")
	var row NoteRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		note, err := q.GetNote(ctx, nid)
		if err != nil {
			return err
		}
		row = NoteRowView{Note: note, CurrentPersonID: personID}
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
	s.render(w, "note-row", row)
}

func (s *Server) handleNotesUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	personID := r.URL.Query().Get("person_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	personIDs := r.Form["person_id"]
	if len(personIDs) == 0 {
		personIDs = []string{personID}
	}
	var row NoteRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if err := q.UpdateNote(ctx, dal.UpdateNoteParams{
			ID:        nid,
			Type:      r.FormValue("type"),
			Date:      r.FormValue("date"),
			Content:   r.FormValue("content"),
			PersonIDs: personIDs,
		}); err != nil {
			return err
		}
		note, err := q.GetNote(ctx, nid)
		if err != nil {
			return err
		}
		row = NoteRowView{Note: note, CurrentPersonID: personID}
		return nil
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, "note-row", row)
}

func (s *Server) handleNotesDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteNote(ctx, nid)
	})
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}
```

- [ ] **Step 2: Update `handlePersonsDetail` in `handlers_persons.go`**

Add `"time"` to the imports at the top of `handlers_persons.go`.

Inside `handlePersonsDetail`, in the `s.dal.WithTx` callback, after `view.People = append(...)` loop, add:

```go
notes, err := q.ListNotesForPerson(ctx, id)
if err != nil {
    return err
}
view.Notes = notes
```

After the `s.dal.WithTx` call (before `s.render`), add:

```go
view.Today = time.Now().Format("2006-01-02")
```

- [ ] **Step 3: Build**

```bash
go build ./internal/web/... 2>&1
```

Expected: success.

- [ ] **Step 4: Run existing tests to confirm nothing broken**

```bash
go test ./... -count=1 2>&1 | tail -10
```

Expected: all PASS (template tests may fail since templates reference new template names — that's expected until Task 5).

- [ ] **Step 5: Commit**

```bash
git add internal/web/handlers_notes.go internal/web/handlers_persons.go
git commit -m "feat: add note web handlers and load notes in person detail"
```

---

## Task 5: Templates

**Files:**
- Create: `internal/web/templates/partials/note_row.html`
- Create: `internal/web/templates/partials/note_edit_form.html`
- Create: `internal/web/templates/partials/note_person_search_rows.html`
- Create: `internal/web/templates/partials/note_person_search_rows_edit.html`
- Modify: `internal/web/templates/persons_detail.html`

- [ ] **Step 1: Create `note_row.html`**

```html
{{define "note-row"}}
<tr data-note-row>
  <td>{{noteIcon .Note.Type}} {{noteLabel .Note.Type}}</td>
  <td>{{fmtDate .Note.Date}}</td>
  <td>{{.Note.Content}}</td>
  <td>
    {{$others := notePersonsExcept .Note.Persons .CurrentPersonID}}
    {{range $i, $p := $others}}{{if $i}}, {{end}}<a href="/persons/{{$p.ID}}">{{personName $p}}</a>{{end}}
  </td>
  <td>
    <button hx-get="/notes/{{.Note.ID}}/edit?person_id={{.CurrentPersonID}}"
            hx-target="closest [data-note-row]" hx-swap="outerHTML"
            class="outline secondary">Edit</button>
    <button hx-delete="/notes/{{.Note.ID}}"
            hx-confirm="Delete this note?"
            hx-target="closest [data-note-row]" hx-swap="outerHTML"
            class="outline secondary">Delete</button>
  </td>
</tr>
{{end}}
```

- [ ] **Step 2: Create `note_edit_form.html`**

```html
{{define "note-edit-form"}}
<tr data-note-row>
  <td colspan="5">
    <form hx-put="/notes/{{.NoteID}}?person_id={{.CurrentPersonID}}"
          hx-target="closest [data-note-row]" hx-swap="outerHTML">
      <div class="grid">
        <label>Type
          <select name="type">
            <option value="CONVERSATION"{{if eq .Type "CONVERSATION"}} selected{{end}}>💬 Conversation</option>
            <option value="HANGOUT"{{if eq .Type "HANGOUT"}} selected{{end}}>🤝 Hangout</option>
            <option value="LIFE_EVENT"{{if eq .Type "LIFE_EVENT"}} selected{{end}}>🎯 Life Event</option>
          </select>
        </label>
        <label>Date <input type="date" name="date" value="{{.Date}}" required></label>
      </div>
      <label>Note <textarea name="content" required>{{.Content}}</textarea></label>
      <div>
        <strong>People</strong>
        <div id="note-edit-persons-selected">
          {{range .SelectedPersons}}
          <span data-person-chip="{{.ID}}">
            {{personName .}}
            <input type="hidden" name="person_id" value="{{.ID}}">
            <button type="button" onclick="this.closest('[data-person-chip]').remove()">×</button>
          </span>
          {{end}}
        </div>
        <input type="search" placeholder="Add person…"
          hx-get="/persons?link_rel=note-edit"
          hx-trigger="input delay:300ms"
          hx-target="#note-edit-person-results"
          hx-swap="innerHTML">
        <div id="note-edit-person-results"></div>
      </div>
      <button type="submit">Save</button>
      <button type="button"
              hx-get="/notes/{{.NoteID}}/cancel?person_id={{.CurrentPersonID}}"
              hx-target="closest [data-note-row]" hx-swap="outerHTML">Cancel</button>
    </form>
    <script>
    function addNotePersonEdit(id, name) {
      var c = document.getElementById('note-edit-persons-selected');
      if (c.querySelector('[data-person-chip="'+id+'"]')) return;
      var s = document.createElement('span');
      s.setAttribute('data-person-chip', id);
      s.innerHTML = name
        + ' <input type="hidden" name="person_id" value="'+id+'">'
        + '<button type="button" onclick="this.closest(\'[data-person-chip]\').remove()">×</button>';
      c.appendChild(s);
    }
    </script>
  </td>
</tr>
{{end}}
```

- [ ] **Step 3: Create `note_person_search_rows.html`** (for create form, calls `addNotePersonCreate`)

```html
{{define "note-person-search-rows"}}
<table><tbody>
{{range .Persons}}
<tr>
  <td>{{.Name}}</td>
  <td><button type="button" onclick="addNotePersonCreate('{{.ID}}','{{.Name}}')">Add</button></td>
</tr>
{{else}}
<tr><td colspan="2"><em>No results.</em></td></tr>
{{end}}
</tbody></table>
{{end}}
```

- [ ] **Step 4: Create `note_person_search_rows_edit.html`** (for edit form, calls `addNotePersonEdit`)

```html
{{define "note-person-search-rows-edit"}}
<table><tbody>
{{range .Persons}}
<tr>
  <td>{{.Name}}</td>
  <td><button type="button" onclick="addNotePersonEdit('{{.ID}}','{{.Name}}')">Add</button></td>
</tr>
{{else}}
<tr><td colspan="2"><em>No results.</em></td></tr>
{{end}}
</tbody></table>
{{end}}
```

- [ ] **Step 5: Add the Timeline section to `persons_detail.html`**

Insert the following block immediately before the closing `<p style="margin-top:2rem;">` line (the "← Back to People" link):

```html
<article id="notes-section">
  <header><h3>Timeline</h3></header>
  <form
    hx-post="/persons/{{.Person.ID}}/notes"
    hx-target="#notes-list"
    hx-swap="afterbegin"
    hx-on:htmx:after-request="if(event.detail.successful && event.detail.elt===this){this.reset();var sel=document.getElementById('note-persons-selected');sel.innerHTML='';var h=document.createElement('span');h.setAttribute('data-person-chip','{{.Person.ID}}');h.innerHTML='{{personName .Person}}<input type=\'hidden\' name=\'person_id\' value=\'{{.Person.ID}}\'>';sel.appendChild(h);document.getElementById('note-person-results').innerHTML='';}"
  >
    <div class="grid">
      <label>Type
        <select name="type">
          <option value="CONVERSATION">💬 Conversation</option>
          <option value="HANGOUT">🤝 Hangout</option>
          <option value="LIFE_EVENT">🎯 Life Event</option>
        </select>
      </label>
      <label>Date <input type="date" name="date" value="{{.Today}}" required></label>
    </div>
    <label>Note <textarea name="content" required placeholder="What happened?"></textarea></label>
    <div>
      <strong>People involved</strong>
      <div id="note-persons-selected">
        <span data-person-chip="{{.Person.ID}}">
          {{personName .Person}}<input type="hidden" name="person_id" value="{{.Person.ID}}">
        </span>
      </div>
      <input type="search" placeholder="Add another person…"
        hx-get="/persons?link_rel=note"
        hx-trigger="input delay:300ms"
        hx-target="#note-person-results"
        hx-swap="innerHTML">
      <div id="note-person-results"></div>
    </div>
    <button type="submit">Add to Timeline</button>
  </form>
  <script>
  function addNotePersonCreate(id, name) {
    var c = document.getElementById('note-persons-selected');
    if (c.querySelector('[data-person-chip="'+id+'"]')) return;
    var s = document.createElement('span');
    s.setAttribute('data-person-chip', id);
    s.innerHTML = name
      + ' <input type="hidden" name="person_id" value="'+id+'">'
      + '<button type="button" onclick="this.closest(\'[data-person-chip]\').remove()">×</button>';
    c.appendChild(s);
  }
  </script>
  <table>
    <thead><tr><th>Type</th><th>Date</th><th>Note</th><th>Also with</th><th></th></tr></thead>
    <tbody id="notes-list">
      {{range .Notes}}{{template "note-row" (noteRow . $.Person.ID)}}{{end}}
    </tbody>
  </table>
</article>
```

- [ ] **Step 6: Run all tests**

```bash
go test ./... -count=1 2>&1 | tail -15
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/web/templates/
git commit -m "feat: add note row, edit form, and person-search templates; add timeline to person detail"
```

---

## Task 6: API handlers and integration tests

**Files:**
- Modify: `internal/web/handlers_api.go`
- Modify: `internal/web/web_test.go`

- [ ] **Step 1: Write failing API integration tests first**

Append to `internal/web/web_test.go`:

```go
func TestAPIListNotes_empty(t *testing.T) {
	s := newTestServer(t)
	w := doAPIJSON(t, s, http.MethodGet, "/api/notes", nil)
	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
	assert.Equal(t, []any{}, resp["results"])
}

func TestAPICreateNote_and_ListByPerson(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Alice Note")
	s, err := web.NewServer(dal.New(sqlDB, ":memory:"))
	require.NoError(t, err)

	w := doAPIJSON(t, s, http.MethodPost, "/api/notes", map[string]any{
		"type":       "CONVERSATION",
		"date":       "2026-04-26",
		"content":    "Caught up over coffee.",
		"person_ids": []string{personID},
	})
	assert.Equal(t, http.StatusCreated, w.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&created))
	assert.Equal(t, "CONVERSATION", created["type"])
	assert.Equal(t, "2026-04-26", created["date"])
	noteID, _ := created["id"].(string)
	require.NotEmpty(t, noteID)

	w2 := doAPIJSON(t, s, http.MethodGet, "/api/persons/"+personID+"/notes", nil)
	assert.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&resp))
	results, _ := resp["results"].([]any)
	assert.Len(t, results, 1)
}

func TestAPICreateNote_invalidType(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Bob Note")
	s, err := web.NewServer(dal.New(sqlDB, ":memory:"))
	require.NoError(t, err)

	w := doAPIJSON(t, s, http.MethodPost, "/api/notes", map[string]any{
		"type":       "INVALID",
		"date":       "2026-04-26",
		"content":    "test",
		"person_ids": []string{personID},
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAPIUpdateNote(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Carol Note")
	s, err := web.NewServer(dal.New(sqlDB, ":memory:"))
	require.NoError(t, err)

	wc := doAPIJSON(t, s, http.MethodPost, "/api/notes", map[string]any{
		"type":       "HANGOUT",
		"date":       "2026-04-26",
		"content":    "original",
		"person_ids": []string{personID},
	})
	require.Equal(t, http.StatusCreated, wc.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(wc.Body).Decode(&created))
	noteID := created["id"].(string)

	wu := doAPIJSON(t, s, http.MethodPut, "/api/notes/"+noteID, map[string]any{
		"type":       "LIFE_EVENT",
		"date":       "2026-04-27",
		"content":    "updated",
		"person_ids": []string{personID},
	})
	assert.Equal(t, http.StatusOK, wu.Code)
	var updated map[string]any
	require.NoError(t, json.NewDecoder(wu.Body).Decode(&updated))
	assert.Equal(t, "LIFE_EVENT", updated["type"])
	assert.Equal(t, "updated", updated["content"])
}

func TestAPIUpdateNote_notFound(t *testing.T) {
	s := newTestServer(t)
	w := doAPIJSON(t, s, http.MethodPut, "/api/notes/NOTEXIST", map[string]any{
		"type":       "HANGOUT",
		"date":       "2026-04-26",
		"content":    "x",
		"person_ids": []string{"x"},
	})
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestAPIDeleteNote(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	personID := createTestPerson(t, sqlDB, "Dave Note")
	s, err := web.NewServer(dal.New(sqlDB, ":memory:"))
	require.NoError(t, err)

	wc := doAPIJSON(t, s, http.MethodPost, "/api/notes", map[string]any{
		"type":       "CONVERSATION",
		"date":       "2026-04-26",
		"content":    "to delete",
		"person_ids": []string{personID},
	})
	require.Equal(t, http.StatusCreated, wc.Code)
	var created map[string]any
	require.NoError(t, json.NewDecoder(wc.Body).Decode(&created))
	noteID := created["id"].(string)

	wd := doAPIJSON(t, s, http.MethodDelete, "/api/notes/"+noteID, nil)
	assert.Equal(t, http.StatusNoContent, wd.Code)

	w2 := doAPIJSON(t, s, http.MethodGet, "/api/notes", nil)
	require.Equal(t, http.StatusOK, w2.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(w2.Body).Decode(&resp))
	assert.Equal(t, []any{}, resp["results"])
}
```

- [ ] **Step 2: Run tests — expect FAIL (handlers not yet written)**

```bash
go test ./internal/web/... -count=1 -run "TestAPIListNotes|TestAPICreateNote|TestAPIUpdateNote|TestAPIDeleteNote" -v 2>&1 | tail -20
```

Expected: FAIL — routes return 404 (not registered to real handlers yet).

- [ ] **Step 3: Add API handlers to `handlers_api.go`**

Append to `internal/web/handlers_api.go`:

```go
// -- Notes API --

type createNoteRequest struct {
	Type      string   `json:"type"`
	Date      string   `json:"date"`
	Content   string   `json:"content"`
	PersonIDs []string `json:"person_ids"`
}

type listNotesResponse struct {
	Results []dal.NoteWithPersons `json:"results"`
}

func (s *Server) handleAPIListNotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var notes []dal.NoteWithPersons
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		var e error
		notes, e = q.ListAllNotes(ctx)
		return e
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notes == nil {
		notes = []dal.NoteWithPersons{}
	}
	apiJSON(w, http.StatusOK, listNotesResponse{Results: notes})
}

func (s *Server) handleAPIListNotesForPerson(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	personID := chi.URLParam(r, "id")
	var notes []dal.NoteWithPersons
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		var e error
		notes, e = q.ListNotesForPerson(ctx, personID)
		return e
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notes == nil {
		notes = []dal.NoteWithPersons{}
	}
	apiJSON(w, http.StatusOK, listNotesResponse{Results: notes})
}

func (s *Server) handleAPICreateNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var note dal.NoteWithPersons
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		var e error
		note, e = q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      req.Type,
			Date:      req.Date,
			Content:   req.Content,
			PersonIDs: req.PersonIDs,
		})
		return e
	})
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, http.StatusCreated, note)
}

func (s *Server) handleAPIUpdateNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	var req createNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var note dal.NoteWithPersons
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if _, err := q.GetNote(ctx, nid); err != nil {
			return err
		}
		if err := q.UpdateNote(ctx, dal.UpdateNoteParams{
			ID:        nid,
			Type:      req.Type,
			Date:      req.Date,
			Content:   req.Content,
			PersonIDs: req.PersonIDs,
		}); err != nil {
			return err
		}
		var e error
		note, e = q.GetNote(ctx, nid)
		return e
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(w, http.StatusNotFound, "note not found")
			return
		}
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, http.StatusOK, note)
}

func (s *Server) handleAPIDeleteNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nid := chi.URLParam(r, "nid")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if _, err := q.GetNote(ctx, nid); err != nil {
			return err
		}
		return q.DeleteNote(ctx, nid)
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(w, http.StatusNotFound, "note not found")
			return
		}
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 4: Run all tests**

```bash
go test ./... -count=1 2>&1 | tail -15
```

Expected: all PASS.

- [ ] **Step 5: Run `make verify`**

```bash
make verify
```

Expected: 0 issues across all checks.

- [ ] **Step 6: Commit**

```bash
git add internal/web/handlers_api.go internal/web/web_test.go
git commit -m "feat: add notes JSON API handlers and integration tests"
```

---

## Final: push and verify CI

- [ ] **Push to remote**

```bash
git push https://$(gh auth token)@github.com/astromechza/aurelianprm.git
```

- [ ] **Confirm CI passes**

```bash
gh run list --limit 3
```

Watch the latest run reach `completed success`.

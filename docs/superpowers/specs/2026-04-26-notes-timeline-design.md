# Notes Timeline — Design Spec

## Goal

Add a timeline of editable notes to each person's detail page. Notes have a type (with emoji), a date (defaults to today), free-text content, and one or more associated persons. A note appears in the timeline of every person linked to it.

## Architecture

Dedicated `notes` + `note_persons` tables. New DAL file, new handler file, new templates. Follows all existing patterns (goose migrations, ULID PKs, HTMX inline-edit, `/api/` JSON endpoints).

## Tech Stack

Go 1.26, SQLite (goose migrations), HTMX 2, Pico CSS 2.

---

## Data Model

### Migration `00004_create_notes.sql`

```sql
-- +goose Up
CREATE TABLE notes (
    id         TEXT     PRIMARY KEY,
    type       TEXT     NOT NULL CHECK(type IN ('LIFE_EVENT','CONVERSATION','HANGOUT')),
    date       TEXT     NOT NULL,   -- full YYYY-MM-DD
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

`date` is a full `YYYY-MM-DD` string — notes are specific events. The DB-level `CHECK` constrains `type` to the three supported values.

### Note types

Defined as a Go map (easy to extend without a migration):

| Constant | Icon | Label |
|----------|------|-------|
| `LIFE_EVENT` | 🎯 | Life Event |
| `CONVERSATION` | 💬 | Conversation |
| `HANGOUT` | 🤝 | Hangout |

---

## DAL — `internal/dal/notes.go`

### Structs

```go
type Note struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    Date      string    `json:"date"`       // YYYY-MM-DD
    Content   string    `json:"content"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type NoteWithPersons struct {
    Note
    Persons []Entity `json:"persons"`
}

type CreateNoteParams struct {
    Type      string
    Date      string   // YYYY-MM-DD
    Content   string
    PersonIDs []string // must all resolve to type=Person; at least one required
}

type UpdateNoteParams struct {
    ID        string
    Type      string
    Date      string
    Content   string
    PersonIDs []string
}
```

### Methods on `*Queries`

- `CreateNote(ctx, CreateNoteParams) (NoteWithPersons, error)` — validates type, date format, all PersonIDs exist and are `type=Person`; inserts note then note_persons rows; returns full NoteWithPersons.
- `GetNote(ctx, id string) (NoteWithPersons, error)` — returns `sql.ErrNoRows` if not found.
- `UpdateNote(ctx, UpdateNoteParams) error` — validates same as create; updates note row; deletes and re-inserts note_persons atomically (caller wraps in `WithTx`).
- `DeleteNote(ctx, id string) error`
- `ListNotesForPerson(ctx, personID string) ([]NoteWithPersons, error)` — ordered `date DESC, created_at DESC`.
- `ListAllNotes(ctx) ([]NoteWithPersons, error)` — ordered `date DESC, created_at DESC` (for API).

Validation:
- `type` must be one of the three known constants.
- `date` must match `^\d{4}-\d{2}-\d{2}$`.
- `PersonIDs` must be non-empty; each must resolve to an entity with `type=Person`.

---

## HTTP Handlers

### Web (HTMX) — `internal/web/handlers_notes.go`

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| `POST` | `/persons/{id}/notes` | `handleNotesCreate` | Create note; returns `note-row` partial prepended to timeline |
| `GET` | `/notes/{nid}/edit` | `handleNotesEditForm` | Returns `note-edit-form` partial |
| `GET` | `/notes/{nid}/cancel` | `handleNotesCancel` | Returns `note-row` partial (read-only) |
| `PUT` | `/notes/{nid}` | `handleNotesUpdate` | Update note; returns `note-row` partial |
| `DELETE` | `/notes/{nid}` | `handleNotesDelete` | Delete note; returns 200 empty |

The create form is always visible at the top of the timeline section. It passes person IDs as repeated `person_id` form values (hidden inputs added by the person-search widget).

### JSON API — `internal/web/handlers_api.go` additions

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/notes` | List all notes with persons |
| `GET` | `/api/persons/{id}/notes` | List notes for one person |
| `POST` | `/api/notes` | Create note |
| `PUT` | `/api/notes/{nid}` | Update note |
| `DELETE` | `/api/notes/{nid}` | Delete note; 204 |

Request body for create/update:
```json
{
  "type": "CONVERSATION",
  "date": "2026-04-26",
  "content": "Caught up over coffee.",
  "person_ids": ["01JQFM..."]
}
```

Response is `NoteWithPersons` (or array thereof). Errors follow existing `{"error": "..."}` convention.

---

## UI — Templates

### `persons_detail.html` addition

New "Timeline" `<article>` section below the People sections. Contains:
1. Create form (always visible, no `<details>` toggle).
2. List of `note-row` partials, ordered date DESC.

### `partials/note_row.html`

Read-only row:
- Icon (from type map) + type label
- Date (formatted as "26 Apr 2026")
- Content (paragraph)
- Linked persons other than the current person (as name links)
- Edit / Delete buttons (HTMX, same pattern as `relationship_row.html`)

### `partials/note_edit_form.html`

Inline edit form:
- `<select>` for type (with icon in option label)
- `<input type="date">` for date
- `<textarea>` for content
- Person search widget (same HTMX search as existing link-person) — adds hidden `person_id` inputs; shows currently linked persons with remove buttons
- Save / Cancel buttons

### `partials/note_create_form.html`

Same fields as edit form. Date defaults to today (`YYYY-MM-DD` passed from handler). Clears on successful submit (`hx-on:htmx:after-request` guard `event.detail.elt===this`). Person search widget works the same way.

---

## View Structs — `internal/web/views.go` (or equivalent)

```go
type NoteRow struct {
    Note           dal.NoteWithPersons
    CurrentPersonID string
    AllPersons     []dal.Entity // for person search widget
    Today          string       // YYYY-MM-DD, for create form default
}

type PersonDetailView struct {
    // ... existing fields ...
    Notes []dal.NoteWithPersons
    Today string
}
```

---

## Testing

- DAL: `TestCreateNote`, `TestListNotesForPerson`, `TestUpdateNote_replacesPersons`, `TestDeleteNote`, `TestCreateNote_invalidType`, `TestCreateNote_invalidDate`, `TestCreateNote_nonPersonEntity`.
- Web: API integration tests in `web_test.go` for create, list-by-person, update, delete — following existing `doAPIJSON` helper pattern.
- No browser/HTMX tests (consistent with existing approach).

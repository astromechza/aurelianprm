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
	Date      string // YYYY-MM-DD
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

// CreateNote inserts a new note and its person associations.
// Multi-table write: call inside WithTx for atomicity.
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

// UpdateNote replaces a note's fields and its person associations.
// Multi-table write: call inside WithTx for atomicity.
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
		return fmt.Errorf("update note: %w", sql.ErrNoRows)
	}
	if _, err := q.db.ExecContext(ctx, `DELETE FROM note_persons WHERE note_id=?`, params.ID); err != nil {
		return fmt.Errorf("update note: clear persons: %w", err)
	}
	if err := q.insertNotePersons(ctx, params.ID, params.PersonIDs); err != nil {
		return fmt.Errorf("update note: %w", err)
	}
	return nil
}

// DeleteNote removes a note by ID. Cascades to note_persons.
func (q *Queries) DeleteNote(ctx context.Context, id string) error {
	res, err := q.db.ExecContext(ctx, `DELETE FROM notes WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete note: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("delete note: %w", sql.ErrNoRows)
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

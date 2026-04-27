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
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.render(w, "note-row", row)
}

func (s *Server) handleNotesDelete(w http.ResponseWriter, r *http.Request) {
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
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

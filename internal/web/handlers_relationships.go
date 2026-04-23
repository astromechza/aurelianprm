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
	dateFrom := r.FormValue("date_from")
	dateTo := r.FormValue("date_to")
	note := r.FormValue("note")

	if personBID == "" || relType == "" {
		view := RelationshipFormView{
			PersonID: personID,
			EditMode: false,
			Error:    "Please select a person and a relationship type.",
		}
		s.render(w, "relationship-form", view)
		return
	}

	var row RelationshipRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		rel, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: personID,
			EntityBID: personBID,
			Type:      relType,
			DateFrom:  nilOrStr(dateFrom),
			DateTo:    nilOrStr(dateTo),
			Note:      nilOrStr(note),
		})
		if err != nil {
			return err
		}
		otherPerson, err := q.GetEntity(ctx, personBID)
		if err != nil {
			return err
		}
		row = RelationshipRowView{
			PersonID: personID,
			PersonWithRel: PersonWithRel{
				Person:       otherPerson,
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
	s.render(w, "relationship-row", row)
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
		otherPerson, err := q.GetEntity(ctx, otherPersonID)
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
			OtherPerson: &otherPerson,
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

func (s *Server) handleRelationshipsCancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := chi.URLParam(r, "rid")
	personID := r.URL.Query().Get("person_id")

	var row RelationshipRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		rel, err := q.GetRelationship(ctx, rid)
		if err != nil {
			return err
		}
		otherPersonID := rel.EntityBID
		direction := "outbound"
		if rel.EntityBID == personID {
			otherPersonID = rel.EntityAID
			direction = "inbound"
		}
		otherPerson, err := q.GetEntity(ctx, otherPersonID)
		if err != nil {
			return err
		}
		row = RelationshipRowView{
			PersonID: personID,
			PersonWithRel: PersonWithRel{
				Person:       otherPerson,
				Relationship: rel,
				Direction:    direction,
			},
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
	s.render(w, "relationship-row", row)
}

func (s *Server) handleRelationshipsUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rid := chi.URLParam(r, "rid")
	personID := r.URL.Query().Get("person_id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	dateFrom := r.FormValue("date_from")
	dateTo := r.FormValue("date_to")
	note := r.FormValue("note")

	var row RelationshipRowView
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if err := q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       rid,
			DateFrom: nilOrStr(dateFrom),
			DateTo:   nilOrStr(dateTo),
			Note:     nilOrStr(note),
		}); err != nil {
			return err
		}
		rel, err := q.GetRelationship(ctx, rid)
		if err != nil {
			return err
		}
		otherPersonID := rel.EntityBID
		direction := "outbound"
		if rel.EntityBID == personID {
			otherPersonID = rel.EntityAID
			direction = "inbound"
		}
		otherPerson, err := q.GetEntity(ctx, otherPersonID)
		if err != nil {
			return err
		}
		row = RelationshipRowView{
			PersonID: personID,
			PersonWithRel: PersonWithRel{
				Person:       otherPerson,
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
	s.render(w, "relationship-row", row)
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

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
			EntityType: entityTypeForRelType(relType),
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
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	s.render(w, "entity-row", view)
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
		if errors.Is(err, sql.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		s.serverError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

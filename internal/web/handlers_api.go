package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/astromechza/aurelianprm/internal/dal"
)

// apiJSON writes v as JSON with the given HTTP status code.
func apiJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// apiError writes {"error": msg} with the given HTTP status code.
func apiError(w http.ResponseWriter, status int, msg string) {
	apiJSON(w, status, map[string]string{"error": msg})
}

// -- Search --

type searchEntitiesRequest struct {
	Q    string `json:"q"`
	Type string `json:"type"`
}

type searchEntitiesResponse struct {
	Results []dal.Entity `json:"results"`
}

func (s *Server) handleAPISearchEntities(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req searchEntitiesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	var results []dal.Entity
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		all, err := q.SearchEntities(ctx, req.Q)
		if err != nil {
			return err
		}
		for _, e := range all {
			if req.Type == "" || e.Type == req.Type {
				results = append(results, e)
			}
		}
		return nil
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []dal.Entity{}
	}
	apiJSON(w, http.StatusOK, searchEntitiesResponse{Results: results})
}

// -- Entity CRUD --

func (s *Server) handleAPIGetEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	var entity dal.Entity
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		var e error
		entity, e = q.GetEntity(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(w, http.StatusNotFound, "entity not found")
			return
		}
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	apiJSON(w, http.StatusOK, entity)
}

type createEntityRequest struct {
	Type        string          `json:"type"`
	DisplayName *string         `json:"display_name"`
	Data        json.RawMessage `json:"data"`
}

func (s *Server) handleAPICreateEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req createEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Type == "" {
		apiError(w, http.StatusBadRequest, "type is required")
		return
	}
	if len(req.Data) == 0 {
		apiError(w, http.StatusBadRequest, "data is required")
		return
	}
	var entity dal.Entity
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		var e error
		entity, e = q.CreateEntity(ctx, dal.CreateEntityParams{
			Type:        req.Type,
			DisplayName: req.DisplayName,
			Data:        req.Data,
		})
		return e
	})
	if err != nil {
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, http.StatusCreated, entity)
}

type updateEntityRequest struct {
	DisplayName *string         `json:"display_name"`
	Data        json.RawMessage `json:"data"`
}

func (s *Server) handleAPIUpdateEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	var req updateEntityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apiError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Data) == 0 {
		apiError(w, http.StatusBadRequest, "data is required")
		return
	}
	var entity dal.Entity
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		if err := q.UpdateEntity(ctx, dal.UpdateEntityParams{
			ID:          id,
			DisplayName: req.DisplayName,
			Data:        req.Data,
		}); err != nil {
			return err
		}
		var e error
		entity, e = q.GetEntity(ctx, id)
		return e
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			apiError(w, http.StatusNotFound, "entity not found")
			return
		}
		apiError(w, http.StatusBadRequest, err.Error())
		return
	}
	apiJSON(w, http.StatusOK, entity)
}

func (s *Server) handleAPIDeleteEntity(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	err := s.dal.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, id)
	})
	if err != nil {
		apiError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

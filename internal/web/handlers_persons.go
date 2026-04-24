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
			ID:         e.ID,
			Name:       pd.Name,
			NickName:   pd.NickName,
			BirthYear:  pd.BirthYear,
			BirthMonth: pd.BirthMonth,
		})
	}
	linkRel := r.URL.Query().Get("link_rel")
	view := PersonListView{Query: q, LinkRel: linkRel, Persons: items}

	if isHTMX(r) {
		if linkRel != "" {
			s.render(w, "link-persons-rows", view)
		} else {
			s.render(w, "persons-rows", view)
		}
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
			Type:        "Person",
			DisplayName: &name,
			Data:        raw,
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
		return q.UpdateEntity(ctx, dal.UpdateEntityParams{ID: id, DisplayName: &name, Data: raw})
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
	if isHTMX(r) {
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/persons", http.StatusSeeOther)
}

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

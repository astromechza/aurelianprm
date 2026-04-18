package web

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// handleRelationshipCard returns a relationship_row.html fragment.
// Called by the Cancel button on the edit form to restore the row display.
func (s *Server) handleRelationshipCard(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	relID := r.PathValue("rel_id")

	row, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, relID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	toName, _ := s.resolveEntityName(r.Context(), row.Relationship.ToType, row.Relationship.ToID)
	s.templates.Render(w, r, "relationship_row.html", buildRelationshipRow(personID, row, toName))
}

// handleRelationshipEditForm returns the inline edit form for a relationship row.
func (s *Server) handleRelationshipEditForm(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	relID := r.PathValue("rel_id")

	row, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, relID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "relationship_edit_form.html", buildRelationshipEditData(personID, row, ""))
}

// handleRelationshipUpdate saves edits to a relationship row.
// Returns a relationship_row.html fragment on success.
func (s *Server) handleRelationshipUpdate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	relID := r.PathValue("rel_id")

	row, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, relID)
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

	label := r.FormValue("label")
	if label == "" {
		s.templates.Render(w, r, "relationship_edit_form.html", buildRelationshipEditData(personID, row, "Label is required."))
		return
	}

	newSinceBD := parseBlurryDateForm(r, "since_year", "since_month", "since_day")
	newUntilBD := parseBlurryDateForm(r, "until_year", "until_month", "until_day")

	var newSinceID, newUntilID *string
	txErr := store.WithTx(r.Context(), s.db, func(tx store.DBTX) error {
		var err error
		newSinceID, err = applyBlurryDateCreate(r.Context(), tx, newSinceBD, row.Relationship.SinceID)
		if err != nil {
			return err
		}
		newUntilID, err = applyBlurryDateCreate(r.Context(), tx, newUntilBD, row.Relationship.UntilID)
		if err != nil {
			return err
		}
		return store.UpdateRelationship(r.Context(), tx, relID, label, nullableFormValue(r, "notes"), newSinceID, newUntilID)
	})
	if txErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Delete orphaned blurry_date rows after FK is NULLed by UpdateRelationship.
	if newSinceBD == nil && row.Relationship.SinceID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.Relationship.SinceID)
	}
	if newUntilBD == nil && row.Relationship.UntilID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.Relationship.UntilID)
	}

	updated, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, relID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	toName, _ := s.resolveEntityName(r.Context(), updated.Relationship.ToType, updated.Relationship.ToID)
	s.templates.Render(w, r, "relationship_row.html", buildRelationshipRow(personID, updated, toName))
}

// handleRelationshipCreate adds a relationship for a person.
// Supports since/until dates, notes, and an optional "create new person" flow.
// Returns a relationship_row.html fragment on success.
func (s *Server) handleRelationshipCreate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	label := r.FormValue("label")
	if label == "" {
		http.Error(w, "label is required", http.StatusUnprocessableEntity)
		return
	}

	toType := model.RelationshipTarget(r.FormValue("to_type"))

	// Resolve the target ID: either pick an existing entity or create a new person.
	var toID string
	if toType == model.RelationshipTargetPerson && r.FormValue("create_new_person") == "on" {
		newName := r.FormValue("new_person_name")
		if newName == "" {
			http.Error(w, "new person name is required", http.StatusUnprocessableEntity)
			return
		}
		created, err := store.CreatePerson(r.Context(), s.db, model.Person{
			Name:  newName,
			Notes: nullableFormValue(r, "new_person_notes"),
		})
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		toID = created.ID
	} else {
		toID = r.FormValue("to_id")
		if toID == "" {
			// fall back to raw ID field used for non-person types
			toID = r.FormValue("to_id_raw")
		}
		if toID == "" {
			http.Error(w, "entity ID is required", http.StatusUnprocessableEntity)
			return
		}
	}

	sinceBD := parseBlurryDateForm(r, "since_year", "since_month", "since_day")
	untilBD := parseBlurryDateForm(r, "until_year", "until_month", "until_day")

	var (
		rel     model.Relationship
		sinceID *string
		untilID *string
	)
	txErr := store.WithTx(r.Context(), s.db, func(tx store.DBTX) error {
		var err error
		sinceID, err = createOptionalBlurryDate(r.Context(), tx, sinceBD)
		if err != nil {
			return err
		}
		untilID, err = createOptionalBlurryDate(r.Context(), tx, untilBD)
		if err != nil {
			return err
		}
		rel, err = store.CreateRelationship(r.Context(), tx, model.Relationship{
			PersonID: personID,
			ToType:   toType,
			ToID:     toID,
			Label:    label,
			Notes:    nullableFormValue(r, "notes"),
			SinceID:  sinceID,
			UntilID:  untilID,
		})
		return err
	})
	if txErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Fetch the enriched row so since/until integers are available for display.
	prr, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, rel.ID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	toName, _ := s.resolveEntityName(r.Context(), prr.Relationship.ToType, prr.Relationship.ToID)
	s.templates.Render(w, r, "relationship_row.html", buildRelationshipRow(personID, prr, toName))
}

// handleRelationshipDelete removes a relationship and its orphaned blurry_dates.
func (s *Server) handleRelationshipDelete(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	relID := r.PathValue("rel_id")

	row, err := store.GetPersonRelationshipRow(r.Context(), s.db, personID, relID)
	if errors.Is(err, sql.ErrNoRows) {
		// Already gone — treat as success so HTMX can still swap out the row.
		w.WriteHeader(http.StatusOK)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	txErr := store.WithTx(r.Context(), s.db, func(tx store.DBTX) error {
		if err := store.DeleteRelationship(r.Context(), tx, relID); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if row.Relationship.SinceID != nil {
			if err := store.DeleteBlurryDate(r.Context(), tx, *row.Relationship.SinceID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if row.Relationship.UntilID != nil {
			if err := store.DeleteBlurryDate(r.Context(), tx, *row.Relationship.UntilID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		return nil
	})
	if txErr != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handlePersonSearch returns a person_picker_results.html fragment for the
// relationship add-form person picker. Requires an HX-Request header.
func (s *Server) handlePersonSearch(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("HX-Request") == "" {
		http.Error(w, "HTMX request required", http.StatusBadRequest)
		return
	}
	query := r.URL.Query().Get("_search")
	people, err := store.ListPeople(r.Context(), s.db, query)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "person_picker_results.html", people)
}


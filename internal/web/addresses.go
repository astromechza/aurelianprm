package web

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// handleAddressCard returns an address_card.html fragment.
// Called by the Cancel button on the edit form to restore the card display.
func (s *Server) handleAddressCard(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, row))
}

// handleAddressEditForm returns the inline edit form for an address.
func (s *Server) handleAddressEditForm(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_edit_form.html", buildAddressEditData(personID, row, ""))
}

// handleAddressCreate creates a new address entry for a person.
// On success returns an address_card.html fragment for HTMX to append.
func (s *Server) handleAddressCreate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	country := r.FormValue("country")
	if country == "" {
		http.Error(w, "country is required", http.StatusUnprocessableEntity)
		return
	}

	loc, err := store.CreateBlurryLocation(r.Context(), s.db, model.BlurryLocation{
		Country: country,
		City:    nullableFormValue(r, "city"),
		Street:  nullableFormValue(r, "street"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	addr, err := store.CreateAddress(r.Context(), s.db, model.Address{
		LocationID: loc.ID,
		Label:      nullableFormValue(r, "label"),
		Notes:      nullableFormValue(r, "notes"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	sinceBD := parseMonthYearForm(r, "from_year", "from_month")
	sinceID, err := createOptionalBlurryDate(r, s, sinceBD)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	untilBD := parseMonthYearForm(r, "to_year", "to_month")
	untilID, err := createOptionalBlurryDate(r, s, untilBD)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rel, err := store.CreateRelationship(r.Context(), s.db, model.Relationship{
		PersonID: personID,
		ToType:   model.RelationshipTargetAddress,
		ToID:     addr.ID,
		SinceID:  sinceID,
		UntilID:  untilID,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	row := store.PersonAddressRow{
		RelationshipID: rel.ID,
		SinceID:        sinceID,
		UntilID:        untilID,
		Address:        addr,
		Location:       loc,
	}
	if sinceBD != nil {
		row.SinceYear = sinceBD.Year
		row.SinceMonth = sinceBD.Month
	}
	if untilBD != nil {
		row.UntilYear = untilBD.Year
		row.UntilMonth = untilBD.Month
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, row))
}

// handleAddressUpdate saves edits to an address entry.
// Returns an address_card.html fragment on success.
func (s *Server) handleAddressUpdate(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
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

	country := r.FormValue("country")
	if country == "" {
		s.templates.Render(w, r, "address_edit_form.html", buildAddressEditData(personID, row, "Country is required."))
		return
	}

	loc := row.Location
	loc.Country = country
	loc.City = nullableFormValue(r, "city")
	loc.Street = nullableFormValue(r, "street")
	if err := store.UpdateBlurryLocation(r.Context(), s.db, loc); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	addr := row.Address
	addr.Label = nullableFormValue(r, "label")
	addr.Notes = nullableFormValue(r, "notes")
	if err := store.UpdateAddress(r.Context(), s.db, addr); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Compute new blurry-date IDs. To avoid FK violations when clearing a date
	// (FK still points at the old row), we must NULL the relationship FK before
	// deleting the old blurry_date rows. We therefore:
	//   1. Create any new blurry_date rows (updates happen in-place, deletes are deferred).
	//   2. Update the relationship FKs to the new IDs (possibly NULL).
	//   3. Delete any old blurry_date rows that were removed.
	newSinceBD := parseMonthYearForm(r, "from_year", "from_month")
	newUntilBD := parseMonthYearForm(r, "to_year", "to_month")

	newSinceID, err := applyBlurryDateCreate(r.Context(), s.db, newSinceBD, row.SinceID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	newUntilID, err := applyBlurryDateCreate(r.Context(), s.db, newUntilBD, row.UntilID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := store.UpdateRelationshipDates(r.Context(), s.db, row.RelationshipID, newSinceID, newUntilID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Now safe to delete orphaned blurry_date rows (FK is already NULLed).
	if newSinceBD == nil && row.SinceID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.SinceID)
	}
	if newUntilBD == nil && row.UntilID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.UntilID)
	}

	updated, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "address_card.html", buildAddressCardData(personID, updated))
}

// handleAddressDelete removes an address entry (relationship + blurry_dates +
// address + blurry_location).
func (s *Server) handleAddressDelete(w http.ResponseWriter, r *http.Request) {
	personID := r.PathValue("id")
	addrID := r.PathValue("addr_id")

	row, err := store.GetPersonAddressRow(r.Context(), s.db, personID, addrID)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := store.DeleteRelationship(r.Context(), s.db, row.RelationshipID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if row.SinceID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.SinceID)
	}
	if row.UntilID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *row.UntilID)
	}
	if err := store.DeleteAddress(r.Context(), s.db, addrID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := store.DeleteBlurryLocation(r.Context(), s.db, row.Location.ID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// parseMonthYearForm parses year and month from two form fields into a
// BlurryDate. Returns nil if both fields are blank.
func parseMonthYearForm(r *http.Request, yearKey, monthKey string) *model.BlurryDate {
	yearStr := r.FormValue(yearKey)
	monthStr := r.FormValue(monthKey)
	if yearStr == "" && monthStr == "" {
		return nil
	}
	bd := &model.BlurryDate{}
	if v, err := strconv.Atoi(yearStr); err == nil {
		bd.Year = &v
	}
	if v, err := strconv.Atoi(monthStr); err == nil {
		bd.Month = &v
	}
	return bd
}

// buildAddressEditData assembles data for the address_edit_form.html template.
func buildAddressEditData(personID string, row store.PersonAddressRow, errMsg string) addressEditData {
	d := addressEditData{PersonID: personID, Row: row, Error: errMsg}
	if row.SinceYear != nil {
		d.FromYear = strconv.Itoa(*row.SinceYear)
	}
	if row.SinceMonth != nil {
		d.FromMonth = strconv.Itoa(*row.SinceMonth)
	}
	if row.UntilYear != nil {
		d.ToYear = strconv.Itoa(*row.UntilYear)
	}
	if row.UntilMonth != nil {
		d.ToMonth = strconv.Itoa(*row.UntilMonth)
	}
	return d
}

// createOptionalBlurryDate creates a blurry_date row if bd is non-nil and
// returns its ID. Returns nil, nil when bd is nil.
func createOptionalBlurryDate(r *http.Request, s *Server, bd *model.BlurryDate) (*string, error) {
	if bd == nil {
		return nil, nil
	}
	created, err := store.CreateBlurryDate(r.Context(), s.db, *bd)
	if err != nil {
		return nil, err
	}
	return &created.ID, nil
}

// applyBlurryDateCreate resolves the new blurry-date ID for an update operation
// without deleting the old row (deletion must happen after the FK is NULLed).
//
//   - input == nil  → return nil (caller must delete old row after NULLing FK)
//   - input != nil, existingID != nil → update in-place, return existingID
//   - input != nil, existingID == nil → create new row, return new ID
func applyBlurryDateCreate(ctx context.Context, db *sql.DB, input *model.BlurryDate, existingID *string) (*string, error) {
	if input == nil {
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

package web

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// peopleListData is passed to people_list.html and people_list_rows.html.
type peopleListData struct {
	People []model.Person
	Query  string
}

// addressCardData is passed to address_card.html.
type addressCardData struct {
	PersonID        string
	Row             store.PersonAddressRow
	LocationDisplay string // e.g. "42 Baker St, London, UK"
	SinceDisplay    string // e.g. "March 2021", empty if not set
	UntilDisplay    string // e.g. "December 2023", empty if not set
}

// addressEditData is passed to address_edit_form.html.
// Used by handlers in addresses.go (same package).
//
//nolint:unused
type addressEditData struct {
	PersonID  string
	Row       store.PersonAddressRow
	FromYear  string // pre-formatted for <input value>, e.g. "2021" or ""
	FromMonth string // pre-formatted for <input value>, e.g. "3" or ""
	ToYear    string
	ToMonth   string
	Error     string
}

// personDetailData is passed to people_detail.html.
type personDetailData struct {
	Person           model.Person
	BirthdayDisplay  string // pre-formatted, empty if no birthday
	ContactMethods   []model.ContactMethod
	Interactions     []model.Interaction
	Relationships    []relationshipRow
	CurrentAddresses []addressCardData // relationships where until_id IS NULL
	PastAddresses    []addressCardData // relationships where until_id IS NOT NULL
}

// relationshipRow pairs a Relationship with its resolved target display name.
type relationshipRow struct {
	Relationship model.Relationship
	ToName       string
}

// personFormData is passed to people_form.html for create and edit.
type personFormData struct {
	Person   *model.Person    // nil when creating
	Birthday *model.BlurryDate
	Error    string
}

// handlePeopleList renders the people list. HTMX live-search requests receive
// only the rows fragment; full-page loads receive the complete list template.
func (s *Server) handlePeopleList(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("name")
	people, err := store.ListPeople(r.Context(), s.db, query)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := peopleListData{People: people, Query: query}
	if r.Header.Get("HX-Request") != "" {
		s.templates.Render(w, r, "people_list_rows.html", data)
		return
	}
	s.templates.Render(w, r, "people_list.html", data)
}

// handlePersonDetail renders the full detail page for a person.
func (s *Server) handlePersonDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := store.GetPerson(r.Context(), s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data, err := s.buildPersonDetailData(r.Context(), p)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "people_detail.html", data)
}

// buildPersonDetailData loads all related data for a person and assembles the
// template data struct.
func (s *Server) buildPersonDetailData(ctx context.Context, p model.Person) (personDetailData, error) {
	data := personDetailData{
		Person:           p,
		ContactMethods:   []model.ContactMethod{},
		Interactions:     []model.Interaction{},
		Relationships:    []relationshipRow{},
		CurrentAddresses: []addressCardData{},
		PastAddresses:    []addressCardData{},
	}

	if p.BirthdayID != nil {
		bd, err := store.GetBlurryDate(ctx, s.db, *p.BirthdayID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return personDetailData{}, fmt.Errorf("loading birthday: %w", err)
		}
		if err == nil {
			data.BirthdayDisplay = formatBlurryDate(bd)
		}
	}

	cms, err := store.ListContactMethods(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading contact methods: %w", err)
	}
	data.ContactMethods = cms

	interactions, err := store.ListInteractions(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading interactions: %w", err)
	}
	data.Interactions = interactions

	rels, err := store.ListRelationships(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading relationships: %w", err)
	}
	for _, rel := range rels {
		name, nameErr := s.resolveEntityName(ctx, rel.ToType, rel.ToID)
		if nameErr != nil {
			name = rel.ToID
		}
		data.Relationships = append(data.Relationships, relationshipRow{
			Relationship: rel,
			ToName:       name,
		})
	}
	addrs, err := store.ListPersonAddresses(ctx, s.db, p.ID)
	if err != nil {
		return personDetailData{}, fmt.Errorf("loading addresses: %w", err)
	}
	for _, addr := range addrs {
		card := buildAddressCardData(p.ID, addr)
		if addr.UntilID == nil {
			data.CurrentAddresses = append(data.CurrentAddresses, card)
		} else {
			data.PastAddresses = append(data.PastAddresses, card)
		}
	}

	return data, nil
}

// resolveEntityName returns a human-readable name for a relationship target.
func (s *Server) resolveEntityName(ctx context.Context, toType model.RelationshipTarget, toID string) (string, error) {
	switch toType {
	case model.RelationshipTargetPerson:
		p, err := store.GetPerson(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetPet:
		p, err := store.GetPet(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetWork:
		w, err := store.GetWork(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		return w.Name, nil
	case model.RelationshipTargetAddress:
		a, err := store.GetAddress(ctx, s.db, toID)
		if err != nil {
			return "", err
		}
		if a.Label != nil {
			return *a.Label, nil
		}
		return "address", nil
	default:
		return toID, nil
	}
}

// handlePersonNew renders the create-person form.
func (s *Server) handlePersonNew(w http.ResponseWriter, r *http.Request) {
	s.templates.Render(w, r, "people_form.html", personFormData{})
}

// handlePersonCreate processes the create-person form submission.
func (s *Server) handlePersonCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	if name == "" {
		s.templates.Render(w, r, "people_form.html", personFormData{Error: "Name is required."})
		return
	}

	var birthdayID *string
	if bd := parseBirthdayForm(r); bd != nil {
		created, err := store.CreateBlurryDate(r.Context(), s.db, *bd)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		birthdayID = &created.ID
	}

	p, err := store.CreatePerson(r.Context(), s.db, model.Person{
		Name:       name,
		Nickname:   nullableFormValue(r, "nickname"),
		Notes:      nullableFormValue(r, "notes"),
		BirthdayID: birthdayID,
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people/"+p.ID, http.StatusSeeOther)
}

// handlePersonEdit renders the edit-person form pre-populated with existing data.
func (s *Server) handlePersonEdit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, err := store.GetPerson(r.Context(), s.db, id)
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	data := personFormData{Person: &p}
	if p.BirthdayID != nil {
		bd, bdErr := store.GetBlurryDate(r.Context(), s.db, *p.BirthdayID)
		if bdErr == nil {
			data.Birthday = &bd
		}
	}
	s.templates.Render(w, r, "people_form.html", data)
}

// handlePersonUpdate processes the edit-person form submission.
func (s *Server) handlePersonUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := store.GetPerson(r.Context(), s.db, id)
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
	name := r.FormValue("name")
	if name == "" {
		s.templates.Render(w, r, "people_form.html", personFormData{Person: &existing, Error: "Name is required."})
		return
	}

	newBirthdayID, err := updateBlurryDate(r.Context(), s.db, parseBirthdayForm(r), existing.BirthdayID)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	existing.Name = name
	existing.Nickname = nullableFormValue(r, "nickname")
	existing.Notes = nullableFormValue(r, "notes")
	existing.BirthdayID = newBirthdayID
	if _, err := store.UpdatePerson(r.Context(), s.db, existing); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people/"+id, http.StatusSeeOther)
}

// handlePersonDelete deletes a person and redirects to the people list.
func (s *Server) handlePersonDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := store.GetPerson(r.Context(), s.db, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err == nil && existing.BirthdayID != nil {
		_ = store.DeleteBlurryDate(r.Context(), s.db, *existing.BirthdayID)
	}
	if err := store.DeletePerson(r.Context(), s.db, id); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/people", http.StatusSeeOther)
}

// handleInteractionCreate logs a new interaction. Called by HTMX; returns a
// single table row to be prepended to the interactions list.
func (s *Server) handleInteractionCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	occurredAt, err := time.Parse("2006-01-02T15:04", r.FormValue("occurred_at"))
	if err != nil {
		http.Error(w, "invalid occurred_at: expected format 2006-01-02T15:04", http.StatusBadRequest)
		return
	}
	i, err := store.CreateInteraction(r.Context(), s.db, model.Interaction{
		PersonID:   id,
		OccurredAt: occurredAt,
		Channel:    nullableFormValue(r, "channel"),
		Notes:      nullableFormValue(r, "notes"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "interaction_row.html", i)
}

// handleContactMethodCreate adds a contact method. Called by HTMX; returns a
// single table row to be appended to the contact methods list.
func (s *Server) handleContactMethodCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	cm, err := store.CreateContactMethod(r.Context(), s.db, model.ContactMethod{
		PersonID: id,
		Type:     model.ContactMethodType(r.FormValue("type")),
		Value:    r.FormValue("value"),
		Label:    nullableFormValue(r, "label"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	s.templates.Render(w, r, "contact_method_row.html", cm)
}

// handleContactMethodDelete removes a contact method. Called by HTMX with
// hx-swap="outerHTML" on the table row; returns an empty 200 so HTMX removes
// the row from the DOM.
func (s *Server) handleContactMethodDelete(w http.ResponseWriter, r *http.Request) {
	methodID := r.PathValue("method_id")
	if err := store.DeleteContactMethod(r.Context(), s.db, methodID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// handleRelationshipCreate adds a relationship. Called by HTMX; returns a
// single table row to be appended to the relationships list.
func (s *Server) handleRelationshipCreate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	rel, err := store.CreateRelationship(r.Context(), s.db, model.Relationship{
		PersonID: id,
		ToType:   model.RelationshipTarget(r.FormValue("to_type")),
		ToID:     r.FormValue("to_id"),
		Label:    r.FormValue("label"),
	})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	toName, _ := s.resolveEntityName(r.Context(), rel.ToType, rel.ToID)
	s.templates.Render(w, r, "relationship_row.html", relationshipRow{
		Relationship: rel,
		ToName:       toName,
	})
}

// handleRelationshipDelete removes a relationship. Called by HTMX; returns
// empty 200 so HTMX removes the row.
func (s *Server) handleRelationshipDelete(w http.ResponseWriter, r *http.Request) {
	relID := r.PathValue("id")
	if err := store.DeleteRelationship(r.Context(), s.db, relID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// nullableFormValue returns a pointer to the trimmed form value if non-empty,
// otherwise nil.
func nullableFormValue(r *http.Request, key string) *string {
	v := r.FormValue(key)
	if v == "" {
		return nil
	}
	return &v
}

// parseBirthdayForm reads birthday fields from the request form and returns a
// BlurryDate, or nil if all birthday fields are blank.
func parseBirthdayForm(r *http.Request) *model.BlurryDate {
	yearStr := r.FormValue("birthday_year")
	monthStr := r.FormValue("birthday_month")
	dayStr := r.FormValue("birthday_day")
	if yearStr == "" && monthStr == "" && dayStr == "" {
		return nil
	}
	reminderEnabled := r.FormValue("reminder_enabled") == "on"
	reminderDaysBefore := 1
	if v, err := strconv.Atoi(r.FormValue("reminder_days_before")); err == nil {
		reminderDaysBefore = v
	}
	bd := &model.BlurryDate{
		ReminderEnabled:    reminderEnabled,
		ReminderDaysBefore: reminderDaysBefore,
	}
	if v, err := strconv.Atoi(yearStr); err == nil {
		bd.Year = &v
	}
	if v, err := strconv.Atoi(monthStr); err == nil {
		bd.Month = &v
	}
	if v, err := strconv.Atoi(dayStr); err == nil {
		bd.Day = &v
	}
	return bd
}

// updateBlurryDate handles the create/update/delete lifecycle for a blurry date.
func updateBlurryDate(ctx context.Context, db *sql.DB, input *model.BlurryDate, existingID *string) (*string, error) {
	if input == nil {
		if existingID != nil {
			if err := store.DeleteBlurryDate(ctx, db, *existingID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
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

// formatLocation builds a comma-separated display string for a BlurryLocation,
// omitting blank optional parts.
func formatLocation(loc model.BlurryLocation) string {
	var parts []string
	if loc.Street != nil && *loc.Street != "" {
		parts = append(parts, *loc.Street)
	}
	if loc.City != nil && *loc.City != "" {
		parts = append(parts, *loc.City)
	}
	parts = append(parts, loc.Country)
	return strings.Join(parts, ", ")
}

// formatMonthYear returns a human-readable month+year string (e.g. "March 2021").
// Returns an empty string if both inputs are nil.
func formatMonthYear(year, month *int) string {
	if year == nil && month == nil {
		return ""
	}
	return formatBlurryDate(model.BlurryDate{Year: year, Month: month})
}

// buildAddressCardData assembles display data for the address_card.html template.
func buildAddressCardData(personID string, row store.PersonAddressRow) addressCardData {
	return addressCardData{
		PersonID:        personID,
		Row:             row,
		LocationDisplay: formatLocation(row.Location),
		SinceDisplay:    formatMonthYear(row.SinceYear, row.SinceMonth),
		UntilDisplay:    formatMonthYear(row.UntilYear, row.UntilMonth),
	}
}

// formatBlurryDate produces a human-readable string from a BlurryDate.
func formatBlurryDate(d model.BlurryDate) string {
	if d.Day != nil && d.Month != nil && d.Year != nil {
		return fmt.Sprintf("%d %s %d", *d.Day, time.Month(*d.Month).String(), *d.Year)
	}
	if d.Day != nil && d.Month != nil {
		return fmt.Sprintf("%d %s", *d.Day, time.Month(*d.Month).String())
	}
	if d.Month != nil && d.Year != nil {
		return fmt.Sprintf("%s %d", time.Month(*d.Month).String(), *d.Year)
	}
	if d.Year != nil {
		return strconv.Itoa(*d.Year)
	}
	return ""
}

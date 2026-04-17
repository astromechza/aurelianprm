package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// mapBlurryDate converts a model.BlurryDate to the API BlurryDate type.
func mapBlurryDate(d model.BlurryDate) BlurryDate {
	return BlurryDate{
		Id:                 d.ID,
		Year:               d.Year,
		Month:              d.Month,
		Day:                d.Day,
		ReminderEnabled:    d.ReminderEnabled,
		ReminderDaysBefore: d.ReminderDaysBefore,
	}
}

// mapBlurryLocation converts a model.BlurryLocation to the API BlurryLocation type.
func mapBlurryLocation(l model.BlurryLocation) BlurryLocation {
	return BlurryLocation{
		Id:        l.ID,
		Country:   l.Country,
		City:      l.City,
		Street:    l.Street,
		Latitude:  l.Latitude,
		Longitude: l.Longitude,
	}
}

// mapPersonSummary converts a model.Person to the API PersonSummary type.
func mapPersonSummary(p model.Person) PersonSummary {
	return PersonSummary{
		Id:        p.ID,
		Name:      p.Name,
		Nickname:  p.Nickname,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// mapContactMethod converts a model.ContactMethod to the API ContactMethod type.
func mapContactMethod(cm model.ContactMethod) ContactMethod {
	return ContactMethod{
		Id:        cm.ID,
		PersonId:  cm.PersonID,
		Type:      ContactMethodType(cm.Type),
		Value:     cm.Value,
		Label:     cm.Label,
		CreatedAt: cm.CreatedAt,
	}
}

// mapInteraction converts a model.Interaction to the API Interaction type.
func mapInteraction(i model.Interaction) Interaction {
	return Interaction{
		Id:         i.ID,
		PersonId:   i.PersonID,
		OccurredAt: i.OccurredAt,
		Channel:    i.Channel,
		Notes:      i.Notes,
		CreatedAt:  i.CreatedAt,
	}
}

// mapRelationship converts a model.Relationship to the API Relationship type.
func mapRelationship(r model.Relationship) Relationship {
	return Relationship{
		Id:        r.ID,
		PersonId:  r.PersonID,
		ToType:    RelationshipTargetType(r.ToType),
		ToId:      r.ToID,
		Label:     r.Label,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
	}
}

// mapPet converts a model.Pet to the API Pet type.
func mapPet(p model.Pet) Pet {
	return Pet{
		Id:        p.ID,
		Name:      p.Name,
		Notes:     p.Notes,
		CreatedAt: p.CreatedAt,
	}
}

// mapWork converts a model.Work to the API Work type.
func mapWork(w model.Work) Work {
	return Work{
		Id:        w.ID,
		Name:      w.Name,
		Website:   w.Website,
		Notes:     w.Notes,
		CreatedAt: w.CreatedAt,
	}
}

// mapAddress converts a model.Address + model.BlurryLocation to the API Address type.
func mapAddress(a model.Address, l model.BlurryLocation) Address {
	return Address{
		Id:        a.ID,
		Label:     a.Label,
		Notes:     a.Notes,
		Location:  mapBlurryLocation(l),
		CreatedAt: a.CreatedAt,
	}
}

// buildPersonDetail assembles a PersonDetail response for a given person,
// loading all related data (birthday, contact methods, interactions, relationships).
func buildPersonDetail(ctx context.Context, db *sql.DB, p model.Person) (PersonDetail, error) {
	detail := PersonDetail{
		Id:             p.ID,
		Name:           p.Name,
		Nickname:       p.Nickname,
		Notes:          p.Notes,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
		ContactMethods: []ContactMethod{},
		Interactions:   []Interaction{},
		Relationships:  []RelationshipDetail{},
	}

	if p.BirthdayID != nil {
		bd, err := store.GetBlurryDate(ctx, db, *p.BirthdayID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return PersonDetail{}, fmt.Errorf("loading birthday: %w", err)
		}
		if err == nil {
			mapped := mapBlurryDate(bd)
			detail.Birthday = &mapped
		}
	}

	cms, err := store.ListContactMethods(ctx, db, p.ID)
	if err != nil {
		return PersonDetail{}, fmt.Errorf("loading contact methods: %w", err)
	}
	for _, cm := range cms {
		detail.ContactMethods = append(detail.ContactMethods, mapContactMethod(cm))
	}

	interactions, err := store.ListInteractions(ctx, db, p.ID)
	if err != nil {
		return PersonDetail{}, fmt.Errorf("loading interactions: %w", err)
	}
	for _, i := range interactions {
		detail.Interactions = append(detail.Interactions, mapInteraction(i))
	}

	rels, err := store.ListRelationships(ctx, db, p.ID)
	if err != nil {
		return PersonDetail{}, fmt.Errorf("loading relationships: %w", err)
	}
	for _, r := range rels {
		rd, err := buildRelationshipDetail(ctx, db, r)
		if err != nil {
			return PersonDetail{}, err
		}
		detail.Relationships = append(detail.Relationships, rd)
	}

	return detail, nil
}

// buildRelationshipDetail resolves the target entity name for a relationship.
func buildRelationshipDetail(ctx context.Context, db *sql.DB, r model.Relationship) (RelationshipDetail, error) {
	toName, err := resolveEntityName(ctx, db, r.ToType, r.ToID)
	if err != nil {
		return RelationshipDetail{}, fmt.Errorf("resolving entity name: %w", err)
	}

	rd := RelationshipDetail{
		Id:        r.ID,
		PersonId:  r.PersonID,
		ToType:    RelationshipTargetType(r.ToType),
		ToId:      r.ToID,
		ToName:    toName,
		Label:     r.Label,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
	}

	if r.SinceID != nil {
		bd, err := store.GetBlurryDate(ctx, db, *r.SinceID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return RelationshipDetail{}, fmt.Errorf("loading since date: %w", err)
		}
		if err == nil {
			mapped := mapBlurryDate(bd)
			rd.Since = &mapped
		}
	}

	if r.UntilID != nil {
		bd, err := store.GetBlurryDate(ctx, db, *r.UntilID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return RelationshipDetail{}, fmt.Errorf("loading until date: %w", err)
		}
		if err == nil {
			mapped := mapBlurryDate(bd)
			rd.Until = &mapped
		}
	}

	return rd, nil
}

// resolveEntityName returns a human-readable display name for a related entity.
func resolveEntityName(ctx context.Context, db *sql.DB, toType model.RelationshipTarget, toID string) (string, error) {
	switch toType {
	case model.RelationshipTargetPerson:
		p, err := store.GetPerson(ctx, db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetPet:
		p, err := store.GetPet(ctx, db, toID)
		if err != nil {
			return "", err
		}
		return p.Name, nil
	case model.RelationshipTargetWork:
		w, err := store.GetWork(ctx, db, toID)
		if err != nil {
			return "", err
		}
		return w.Name, nil
	case model.RelationshipTargetAddress:
		a, err := store.GetAddress(ctx, db, toID)
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

// blurryDateInputToModel converts an API BlurryDateInput to a model.BlurryDate
// (without ID — the store will assign one).
func blurryDateInputToModel(b BlurryDateInput) model.BlurryDate {
	return model.BlurryDate{
		Year:               b.Year,
		Month:              b.Month,
		Day:                b.Day,
		ReminderEnabled:    b.ReminderEnabled,
		ReminderDaysBefore: b.ReminderDaysBefore,
	}
}

// blurryLocationInputToModel converts an API BlurryLocationInput to a model.BlurryLocation.
func blurryLocationInputToModel(l BlurryLocationInput) model.BlurryLocation {
	return model.BlurryLocation{
		Country:   l.Country,
		City:      l.City,
		Street:    l.Street,
		Latitude:  l.Latitude,
		Longitude: l.Longitude,
	}
}

// createOrUpdateBlurryDate manages a blurry_date record tied to a parent entity.
//   - input nil, existingID nil  → no-op, returns nil
//   - input nil, existingID set  → deletes the orphaned row, returns nil
//   - input set, existingID nil  → creates a new row, returns its ID
//   - input set, existingID set  → updates the existing row in-place, returns its ID
func createOrUpdateBlurryDate(ctx context.Context, db *sql.DB, input *BlurryDateInput, existingID *string) (*string, error) {
	if input == nil {
		if existingID != nil {
			if err := store.DeleteBlurryDate(ctx, db, *existingID); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
		}
		return nil, nil
	}
	m := blurryDateInputToModel(*input)
	if existingID != nil {
		m.ID = *existingID
		if err := store.UpdateBlurryDate(ctx, db, m); err != nil {
			return nil, err
		}
		return existingID, nil
	}
	created, err := store.CreateBlurryDate(ctx, db, m)
	if err != nil {
		return nil, err
	}
	return &created.ID, nil
}

// ptrString returns nil if s is empty or nil, otherwise a pointer to s.
// Used to normalise optional string fields from request bodies.
func ptrString(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

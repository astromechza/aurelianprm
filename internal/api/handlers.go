package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/astromechza/aurelianprm/internal/config"
	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

// Handlers implements StrictServerInterface by delegating to the store layer.
type Handlers struct {
	db  *sql.DB
	cfg *config.Config
}

// NewHandlers creates a new Handlers instance.
func NewHandlers(db *sql.DB, cfg *config.Config) *Handlers {
	return &Handlers{db: db, cfg: cfg}
}

// ---- People ----.

// ListPeople lists all people.
func (h *Handlers) ListPeople(ctx context.Context, req ListPeopleRequestObject) (ListPeopleResponseObject, error) {
	query := ""
	if req.Params.Name != nil {
		query = *req.Params.Name
	}
	people, err := store.ListPeople(ctx, h.db, query)
	if err != nil {
		return nil, fmt.Errorf("listing people: %w", err)
	}
	out := make([]PersonSummary, len(people))
	for i, p := range people {
		out[i] = mapPersonSummary(p)
	}
	return ListPeople200JSONResponse(out), nil
}

// CreatePerson creates a new person.
func (h *Handlers) CreatePerson(ctx context.Context, req CreatePersonRequestObject) (CreatePersonResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return CreatePerson400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}

	var birthdayID *string
	if body.Birthday != nil {
		id, err := createOrUpdateBlurryDate(ctx, h.db, body.Birthday, nil)
		if err != nil {
			return nil, fmt.Errorf("creating birthday: %w", err)
		}
		birthdayID = id
	}

	p, err := store.CreatePerson(ctx, h.db, model.Person{
		Name:       body.Name,
		Nickname:   ptrString(body.Nickname),
		Notes:      ptrString(body.Notes),
		BirthdayID: birthdayID,
	})
	if err != nil {
		return nil, fmt.Errorf("creating person: %w", err)
	}

	detail, err := buildPersonDetail(ctx, h.db, p)
	if err != nil {
		return nil, err
	}
	return CreatePerson201JSONResponse(detail), nil
}

// GetPerson retrieves a person by ID.
func (h *Handlers) GetPerson(ctx context.Context, req GetPersonRequestObject) (GetPersonResponseObject, error) {
	p, err := store.GetPerson(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return GetPerson404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting person: %w", err)
	}
	detail, err := buildPersonDetail(ctx, h.db, p)
	if err != nil {
		return nil, err
	}
	return GetPerson200JSONResponse(detail), nil
}

// UpdatePerson updates an existing person.
func (h *Handlers) UpdatePerson(ctx context.Context, req UpdatePersonRequestObject) (UpdatePersonResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return UpdatePerson400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}

	existing, err := store.GetPerson(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return UpdatePerson404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting person: %w", err)
	}

	birthdayID, err := createOrUpdateBlurryDate(ctx, h.db, body.Birthday, existing.BirthdayID)
	if err != nil {
		return nil, fmt.Errorf("updating birthday: %w", err)
	}

	updated, err := store.UpdatePerson(ctx, h.db, model.Person{
		ID:         existing.ID,
		Name:       body.Name,
		Nickname:   ptrString(body.Nickname),
		Notes:      ptrString(body.Notes),
		BirthdayID: birthdayID,
		CreatedAt:  existing.CreatedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("updating person: %w", err)
	}

	detail, err := buildPersonDetail(ctx, h.db, updated)
	if err != nil {
		return nil, err
	}
	return UpdatePerson200JSONResponse(detail), nil
}

// DeletePerson deletes a person by ID.
func (h *Handlers) DeletePerson(ctx context.Context, req DeletePersonRequestObject) (DeletePersonResponseObject, error) {
	err := store.DeletePerson(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return DeletePerson404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting person: %w", err)
	}
	return DeletePerson204Response{}, nil
}

// ---- Contact Methods ----.

// CreateContactMethod creates a new contact method.
func (h *Handlers) CreateContactMethod(ctx context.Context, req CreateContactMethodRequestObject) (CreateContactMethodResponseObject, error) {
	body := req.Body
	if body.Value == "" {
		return CreateContactMethod400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "value is required"}),
		}, nil
	}

	if _, err := store.GetPerson(ctx, h.db, req.Id); errors.Is(err, sql.ErrNoRows) {
		return CreateContactMethod404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	} else if err != nil {
		return nil, err
	}

	cm, err := store.CreateContactMethod(ctx, h.db, model.ContactMethod{
		PersonID: req.Id,
		Type:     model.ContactMethodType(body.Type),
		Value:    body.Value,
		Label:    ptrString(body.Label),
	})
	if err != nil {
		return nil, fmt.Errorf("creating contact method: %w", err)
	}
	return CreateContactMethod201JSONResponse(mapContactMethod(cm)), nil
}

// DeleteContactMethod deletes a contact method by ID.
func (h *Handlers) DeleteContactMethod(ctx context.Context, req DeleteContactMethodRequestObject) (DeleteContactMethodResponseObject, error) {
	err := store.DeleteContactMethod(ctx, h.db, req.MethodId)
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteContactMethod404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "contact method not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting contact method: %w", err)
	}
	return DeleteContactMethod204Response{}, nil
}

// ---- Interactions ----.

// ListInteractions lists all interactions for a person.
func (h *Handlers) ListInteractions(ctx context.Context, req ListInteractionsRequestObject) (ListInteractionsResponseObject, error) {
	if _, err := store.GetPerson(ctx, h.db, req.Id); errors.Is(err, sql.ErrNoRows) {
		return ListInteractions404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	} else if err != nil {
		return nil, err
	}

	interactions, err := store.ListInteractions(ctx, h.db, req.Id)
	if err != nil {
		return nil, fmt.Errorf("listing interactions: %w", err)
	}
	out := make([]Interaction, len(interactions))
	for i, interaction := range interactions {
		out[i] = mapInteraction(interaction)
	}
	return ListInteractions200JSONResponse(out), nil
}

// CreateInteraction creates a new interaction.
func (h *Handlers) CreateInteraction(ctx context.Context, req CreateInteractionRequestObject) (CreateInteractionResponseObject, error) {
	body := req.Body

	if _, err := store.GetPerson(ctx, h.db, req.Id); errors.Is(err, sql.ErrNoRows) {
		return CreateInteraction404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "person not found"}),
		}, nil
	} else if err != nil {
		return nil, err
	}

	// body.OccurredAt is time.Time — oapi-codegen parses date-time automatically.
	i, err := store.CreateInteraction(ctx, h.db, model.Interaction{
		PersonID:   req.Id,
		OccurredAt: body.OccurredAt,
		Channel:    ptrString(body.Channel),
		Notes:      ptrString(body.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("creating interaction: %w", err)
	}
	return CreateInteraction201JSONResponse(mapInteraction(i)), nil
}

// ---- Relationships ----.

// CreateRelationship creates a new relationship.
func (h *Handlers) CreateRelationship(ctx context.Context, req CreateRelationshipRequestObject) (CreateRelationshipResponseObject, error) {
	body := req.Body
	if body.Label == "" {
		return CreateRelationship400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "label is required"}),
		}, nil
	}

	var sinceID *string
	if body.Since != nil {
		id, err := createOrUpdateBlurryDate(ctx, h.db, body.Since, nil)
		if err != nil {
			return nil, fmt.Errorf("creating since date: %w", err)
		}
		sinceID = id
	}

	var untilID *string
	if body.Until != nil {
		id, err := createOrUpdateBlurryDate(ctx, h.db, body.Until, nil)
		if err != nil {
			return nil, fmt.Errorf("creating until date: %w", err)
		}
		untilID = id
	}

	r, err := store.CreateRelationship(ctx, h.db, model.Relationship{
		PersonID: body.PersonId,
		ToType:   model.RelationshipTarget(body.ToType),
		ToID:     body.ToId,
		Label:    body.Label,
		SinceID:  sinceID,
		UntilID:  untilID,
		Notes:    ptrString(body.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("creating relationship: %w", err)
	}
	return CreateRelationship201JSONResponse(mapRelationship(r)), nil
}

// DeleteRelationship deletes a relationship by ID.
func (h *Handlers) DeleteRelationship(ctx context.Context, req DeleteRelationshipRequestObject) (DeleteRelationshipResponseObject, error) {
	err := store.DeleteRelationship(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteRelationship404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "relationship not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting relationship: %w", err)
	}
	return DeleteRelationship204Response{}, nil
}

// ---- Addresses ----.

// CreateAddress creates a new address.
func (h *Handlers) CreateAddress(ctx context.Context, req CreateAddressRequestObject) (CreateAddressResponseObject, error) {
	body := req.Body

	loc, err := store.CreateBlurryLocation(ctx, h.db, blurryLocationInputToModel(body.Location))
	if err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}

	a, err := store.CreateAddress(ctx, h.db, model.Address{
		LocationID: loc.ID,
		Label:      ptrString(body.Label),
		Notes:      ptrString(body.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("creating address: %w", err)
	}
	return CreateAddress201JSONResponse(mapAddress(a, loc)), nil
}

// GetAddress retrieves an address by ID.
func (h *Handlers) GetAddress(ctx context.Context, req GetAddressRequestObject) (GetAddressResponseObject, error) {
	a, err := store.GetAddress(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return GetAddress404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "address not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting address: %w", err)
	}
	loc, err := store.GetBlurryLocation(ctx, h.db, a.LocationID)
	if err != nil {
		return nil, fmt.Errorf("getting location: %w", err)
	}
	return GetAddress200JSONResponse(mapAddress(a, loc)), nil
}

// UpdateAddress updates an existing address.
func (h *Handlers) UpdateAddress(ctx context.Context, req UpdateAddressRequestObject) (UpdateAddressResponseObject, error) {
	body := req.Body

	existing, err := store.GetAddress(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return UpdateAddress404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "address not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting address: %w", err)
	}

	loc := blurryLocationInputToModel(body.Location)
	loc.ID = existing.LocationID
	if err := store.UpdateBlurryLocation(ctx, h.db, loc); err != nil {
		return nil, fmt.Errorf("updating location: %w", err)
	}

	existing.Label = ptrString(body.Label)
	existing.Notes = ptrString(body.Notes)
	if err := store.UpdateAddress(ctx, h.db, existing); err != nil {
		return nil, fmt.Errorf("updating address: %w", err)
	}

	updatedLoc, err := store.GetBlurryLocation(ctx, h.db, existing.LocationID)
	if err != nil {
		return nil, fmt.Errorf("reloading location: %w", err)
	}
	return UpdateAddress200JSONResponse(mapAddress(existing, updatedLoc)), nil
}

// DeleteAddress deletes an address by ID.
func (h *Handlers) DeleteAddress(ctx context.Context, req DeleteAddressRequestObject) (DeleteAddressResponseObject, error) {
	err := store.DeleteAddress(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteAddress404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "address not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting address: %w", err)
	}
	return DeleteAddress204Response{}, nil
}

// ---- Pets ----.

// CreatePet creates a new pet.
func (h *Handlers) CreatePet(ctx context.Context, req CreatePetRequestObject) (CreatePetResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return CreatePet400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}
	p, err := store.CreatePet(ctx, h.db, model.Pet{
		Name:  body.Name,
		Notes: ptrString(body.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("creating pet: %w", err)
	}
	return CreatePet201JSONResponse(mapPet(p)), nil
}

// GetPet retrieves a pet by ID.
func (h *Handlers) GetPet(ctx context.Context, req GetPetRequestObject) (GetPetResponseObject, error) {
	p, err := store.GetPet(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return GetPet404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "pet not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting pet: %w", err)
	}
	return GetPet200JSONResponse(mapPet(p)), nil
}

// UpdatePet updates an existing pet.
func (h *Handlers) UpdatePet(ctx context.Context, req UpdatePetRequestObject) (UpdatePetResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return UpdatePet400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}
	existing, err := store.GetPet(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return UpdatePet404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "pet not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting pet: %w", err)
	}
	existing.Name = body.Name
	existing.Notes = ptrString(body.Notes)
	if err := store.UpdatePet(ctx, h.db, existing); err != nil {
		return nil, fmt.Errorf("updating pet: %w", err)
	}
	return UpdatePet200JSONResponse(mapPet(existing)), nil
}

// DeletePet deletes a pet by ID.
func (h *Handlers) DeletePet(ctx context.Context, req DeletePetRequestObject) (DeletePetResponseObject, error) {
	err := store.DeletePet(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return DeletePet404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "pet not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting pet: %w", err)
	}
	return DeletePet204Response{}, nil
}

// ---- Work ----.

// CreateWork creates a new work entity.
func (h *Handlers) CreateWork(ctx context.Context, req CreateWorkRequestObject) (CreateWorkResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return CreateWork400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}
	w, err := store.CreateWork(ctx, h.db, model.Work{
		Name:    body.Name,
		Website: ptrString(body.Website),
		Notes:   ptrString(body.Notes),
	})
	if err != nil {
		return nil, fmt.Errorf("creating work: %w", err)
	}
	return CreateWork201JSONResponse(mapWork(w)), nil
}

// GetWork retrieves a work entity by ID.
func (h *Handlers) GetWork(ctx context.Context, req GetWorkRequestObject) (GetWorkResponseObject, error) {
	w, err := store.GetWork(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return GetWork404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "work not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting work: %w", err)
	}
	return GetWork200JSONResponse(mapWork(w)), nil
}

// UpdateWork updates an existing work entity.
func (h *Handlers) UpdateWork(ctx context.Context, req UpdateWorkRequestObject) (UpdateWorkResponseObject, error) {
	body := req.Body
	if body.Name == "" {
		return UpdateWork400JSONResponse{
			BadRequestJSONResponse: BadRequestJSONResponse(ErrorResponse{Error: "name is required"}),
		}, nil
	}
	existing, err := store.GetWork(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return UpdateWork404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "work not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting work: %w", err)
	}
	existing.Name = body.Name
	existing.Website = ptrString(body.Website)
	existing.Notes = ptrString(body.Notes)
	if err := store.UpdateWork(ctx, h.db, existing); err != nil {
		return nil, fmt.Errorf("updating work: %w", err)
	}
	return UpdateWork200JSONResponse(mapWork(existing)), nil
}

// DeleteWork deletes a work entity by ID.
func (h *Handlers) DeleteWork(ctx context.Context, req DeleteWorkRequestObject) (DeleteWorkResponseObject, error) {
	err := store.DeleteWork(ctx, h.db, req.Id)
	if errors.Is(err, sql.ErrNoRows) {
		return DeleteWork404JSONResponse{
			NotFoundJSONResponse: NotFoundJSONResponse(ErrorResponse{Error: "work not found"}),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("deleting work: %w", err)
	}
	return DeleteWork204Response{}, nil
}

// ---- Config ----.

// GetConfig retrieves the server configuration.
func (h *Handlers) GetConfig(_ context.Context, _ GetConfigRequestObject) (GetConfigResponseObject, error) {
	return GetConfig200JSONResponse{
		Listen:           h.cfg.Listen,
		DbPath:           h.cfg.DBPath,
		ReminderEmail:    h.cfg.ReminderEmail,
		ReminderSendTime: h.cfg.ReminderSendTime,
		ReminderTimezone: h.cfg.ReminderTimezone,
	}, nil
}

package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

func TestRelationship_CreateAndList(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	alice, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)
	bob, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)

	notes := "met in 2010"
	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: alice.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     bob.ID,
		Label:    "friend",
		Notes:    &notes,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, rel.ID)
	assert.Equal(t, model.RelationshipTargetPerson, rel.ToType)

	list, err := store.ListRelationships(ctx, db, alice.ID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "friend", list[0].Label)
	assert.Equal(t, &notes, list[0].Notes)
	assert.Nil(t, list[0].SinceID)
}

func TestRelationship_WithSinceID(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Ivan"})
	require.NoError(t, err)
	employer, err := store.CreateWork(ctx, db, model.Work{Name: "ACME"})
	require.NoError(t, err)

	year := 2020
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{
		Year: &year, Month: intPtr(3), Day: intPtr(1),
		ReminderEnabled: false, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetWork,
		ToID:     employer.ID,
		Label:    "employee",
		SinceID:  &since.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, &since.ID, rel.SinceID)
	assert.Nil(t, rel.UntilID)
}

func TestRelationship_ListToEntity(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	alice, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)
	bob, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)
	pet, err := store.CreatePet(ctx, db, model.Pet{Name: "Rex"})
	require.NoError(t, err)

	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: alice.ID, ToType: model.RelationshipTargetPet, ToID: pet.ID, Label: "owner",
	})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: bob.ID, ToType: model.RelationshipTargetPet, ToID: pet.ID, Label: "owner",
	})
	require.NoError(t, err)

	owners, err := store.ListRelationshipsToEntity(ctx, db, model.RelationshipTargetPet, pet.ID)
	require.NoError(t, err)
	assert.Len(t, owners, 2)
}

func TestRelationship_Delete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Judy"})
	require.NoError(t, err)
	other, err := store.CreatePerson(ctx, db, model.Person{Name: "Karl"})
	require.NoError(t, err)
	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetPerson,
		ToID: other.ID, Label: "sibling",
	})
	require.NoError(t, err)
	require.NoError(t, store.DeleteRelationship(ctx, db, rel.ID))

	list, err := store.ListRelationships(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestRelationship_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteRelationship(ctx, db, "nope"), sql.ErrNoRows)
}

func TestRelationship_CascadeOnPersonDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Leo"})
	require.NoError(t, err)
	other, err := store.CreatePerson(ctx, db, model.Person{Name: "Mia"})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetPerson,
		ToID: other.ID, Label: "partner",
	})
	require.NoError(t, err)
	require.NoError(t, store.DeletePerson(ctx, db, person.ID))
	list, err := store.ListRelationships(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

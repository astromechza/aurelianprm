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

func TestContactMethod_CreateAndList(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)

	label := "personal"
	cm1, err := store.CreateContactMethod(ctx, db, model.ContactMethod{
		PersonID: person.ID, Type: model.ContactMethodEmail,
		Value: "alice@example.com", Label: &label,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, cm1.ID)
	assert.Equal(t, model.ContactMethodEmail, cm1.Type)

	_, err = store.CreateContactMethod(ctx, db, model.ContactMethod{
		PersonID: person.ID, Type: model.ContactMethodPhone,
		Value: "+44 7700 900000",
	})
	require.NoError(t, err)

	list, err := store.ListContactMethods(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "alice@example.com", list[0].Value)
	assert.Equal(t, &label, list[0].Label)
	assert.Nil(t, list[1].Label)
}

func TestContactMethod_ListEmpty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)
	list, err := store.ListContactMethods(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestContactMethod_Delete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Carol"})
	require.NoError(t, err)
	cm, err := store.CreateContactMethod(ctx, db, model.ContactMethod{
		PersonID: person.ID, Type: model.ContactMethodWebsite, Value: "https://carol.dev",
	})
	require.NoError(t, err)
	require.NoError(t, store.DeleteContactMethod(ctx, db, cm.ID))

	list, err := store.ListContactMethods(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestContactMethod_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteContactMethod(ctx, db, "nonexistent"), sql.ErrNoRows)
}

func TestContactMethod_CascadeOnPersonDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Dave"})
	require.NoError(t, err)
	_, err = store.CreateContactMethod(ctx, db, model.ContactMethod{
		PersonID: person.ID, Type: model.ContactMethodSocial, Value: "@dave",
	})
	require.NoError(t, err)

	require.NoError(t, store.DeletePerson(ctx, db, person.ID))

	// Contact method should be gone via CASCADE.
	list, err := store.ListContactMethods(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

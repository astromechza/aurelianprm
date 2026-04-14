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

func TestAddress_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "DE"})
	require.NoError(t, err)

	label := "home"
	notes := "ground floor"
	addr, err := store.CreateAddress(ctx, db, model.Address{
		LocationID: loc.ID, Label: &label, Notes: &notes,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, addr.ID)
	assert.Equal(t, loc.ID, addr.LocationID)

	got, err := store.GetAddress(ctx, db, addr.ID)
	require.NoError(t, err)
	assert.Equal(t, &label, got.Label)

	got.Notes = nil
	require.NoError(t, store.UpdateAddress(ctx, db, got))
	updated, err := store.GetAddress(ctx, db, addr.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.Notes)

	require.NoError(t, store.DeleteAddress(ctx, db, addr.ID))
	_, err = store.GetAddress(ctx, db, addr.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestAddress_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "US"})
	require.NoError(t, err)
	assert.ErrorIs(t, store.UpdateAddress(ctx, db, model.Address{ID: "nope", LocationID: loc.ID}), sql.ErrNoRows)
}

func TestAddress_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteAddress(ctx, db, "nope"), sql.ErrNoRows)
}

func TestPet_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	notes := "golden retriever"
	pet, err := store.CreatePet(ctx, db, model.Pet{Name: "Biscuit", Notes: &notes})
	require.NoError(t, err)
	assert.NotEmpty(t, pet.ID)
	assert.Equal(t, "Biscuit", pet.Name)

	got, err := store.GetPet(ctx, db, pet.ID)
	require.NoError(t, err)
	assert.Equal(t, &notes, got.Notes)

	got.Name = "Biscuit II"
	got.Notes = nil
	require.NoError(t, store.UpdatePet(ctx, db, got))
	updated, err := store.GetPet(ctx, db, got.ID)
	require.NoError(t, err)
	assert.Equal(t, "Biscuit II", updated.Name)
	assert.Nil(t, updated.Notes)

	require.NoError(t, store.DeletePet(ctx, db, pet.ID))
	_, err = store.GetPet(ctx, db, pet.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPet_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.UpdatePet(ctx, db, model.Pet{ID: "nope", Name: "X"}), sql.ErrNoRows)
}

func TestPet_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeletePet(ctx, db, "nope"), sql.ErrNoRows)
}

func TestWork_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	site := "https://acme.com"
	notes := "big corp"
	w, err := store.CreateWork(ctx, db, model.Work{Name: "Acme Corp", Website: &site, Notes: &notes})
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
	assert.Equal(t, "Acme Corp", w.Name)

	got, err := store.GetWork(ctx, db, w.ID)
	require.NoError(t, err)
	assert.Equal(t, &site, got.Website)

	got.Website = nil
	require.NoError(t, store.UpdateWork(ctx, db, got))
	updated, err := store.GetWork(ctx, db, got.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.Website)

	require.NoError(t, store.DeleteWork(ctx, db, w.ID))
	_, err = store.GetWork(ctx, db, w.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestWork_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.UpdateWork(ctx, db, model.Work{ID: "nope", Name: "X"}), sql.ErrNoRows)
}

func TestWork_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteWork(ctx, db, "nope"), sql.ErrNoRows)
}

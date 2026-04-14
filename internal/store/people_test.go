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

func TestPerson_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	nick := "Johnny"
	notes := "old friend"

	created, err := store.CreatePerson(ctx, db, model.Person{
		Name: "John Smith", Nickname: &nick, Notes: &notes,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "John Smith", created.Name)
	assert.Equal(t, &nick, created.Nickname)
	assert.False(t, created.CreatedAt.IsZero())
	assert.Equal(t, created.CreatedAt, created.UpdatedAt)

	// Get.
	got, err := store.GetPerson(ctx, db, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, got.ID)
	assert.Equal(t, created.Name, got.Name)

	// Update.
	got.Name = "Jonathan Smith"
	got.Notes = nil
	updated, err := store.UpdatePerson(ctx, db, got)
	require.NoError(t, err)
	assert.Equal(t, "Jonathan Smith", updated.Name)
	assert.Nil(t, updated.Notes)
	assert.True(t, updated.UpdatedAt.After(updated.CreatedAt) || updated.UpdatedAt.Equal(updated.CreatedAt))

	// Verify via Get.
	refetched, err := store.GetPerson(ctx, db, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Jonathan Smith", refetched.Name)

	// Delete.
	require.NoError(t, store.DeletePerson(ctx, db, created.ID))
	_, err = store.GetPerson(ctx, db, created.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPerson_List(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	nick := "ace"
	_, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, db, model.Person{Name: "Bob", Nickname: &nick})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, db, model.Person{Name: "Charlie"})
	require.NoError(t, err)

	// List all.
	all, err := store.ListPeople(ctx, db, "")
	require.NoError(t, err)
	assert.Len(t, all, 3)
	assert.Equal(t, "Alice", all[0].Name)

	// Search by name substring.
	found, err := store.ListPeople(ctx, db, "ali")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "Alice", found[0].Name)

	// Search matching nickname.
	found, err = store.ListPeople(ctx, db, "ace")
	require.NoError(t, err)
	assert.Len(t, found, 1)
	assert.Equal(t, "Bob", found[0].Name)

	// Search with no match.
	found, err = store.ListPeople(ctx, db, "zzz")
	require.NoError(t, err)
	assert.Empty(t, found)
}

func TestPerson_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	_, err := store.UpdatePerson(ctx, db, model.Person{ID: "nonexistent", Name: "X"})
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestPerson_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeletePerson(ctx, db, "nonexistent"), sql.ErrNoRows)
}

func TestPerson_GetNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	_, err := store.GetPerson(ctx, db, "nonexistent")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

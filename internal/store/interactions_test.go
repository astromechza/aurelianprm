package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

func TestInteraction_CreateAndList(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Eve"})
	require.NoError(t, err)

	older := time.Date(2025, 1, 10, 9, 0, 0, 0, time.UTC)
	newer := time.Date(2025, 3, 5, 14, 30, 0, 0, time.UTC)

	channel := "phone"
	i1, err := store.CreateInteraction(ctx, db, model.Interaction{
		PersonID: person.ID, OccurredAt: older, Channel: &channel,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, i1.ID)

	notes := "met for coffee"
	i2, err := store.CreateInteraction(ctx, db, model.Interaction{
		PersonID: person.ID, OccurredAt: newer, Notes: &notes,
	})
	require.NoError(t, err)
	assert.Nil(t, i2.Channel)

	// Listed newest-first.
	list, err := store.ListInteractions(ctx, db, person.ID)
	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, i2.ID, list[0].ID)
	assert.Equal(t, i1.ID, list[1].ID)
	assert.Equal(t, newer.Truncate(time.Millisecond), list[0].OccurredAt)
}

func TestInteraction_ListEmpty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Frank"})
	require.NoError(t, err)
	list, err := store.ListInteractions(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInteraction_Delete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Grace"})
	require.NoError(t, err)
	i, err := store.CreateInteraction(ctx, db, model.Interaction{
		PersonID: person.ID, OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NoError(t, store.DeleteInteraction(ctx, db, i.ID))
	list, err := store.ListInteractions(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestInteraction_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteInteraction(ctx, db, "nonexistent"), sql.ErrNoRows)
}

func TestInteraction_CascadeOnPersonDelete(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Hank"})
	require.NoError(t, err)
	_, err = store.CreateInteraction(ctx, db, model.Interaction{
		PersonID: person.ID, OccurredAt: time.Now().UTC(),
	})
	require.NoError(t, err)
	require.NoError(t, store.DeletePerson(ctx, db, person.ID))
	list, err := store.ListInteractions(ctx, db, person.ID)
	require.NoError(t, err)
	assert.Empty(t, list)
}

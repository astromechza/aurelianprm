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

func TestBlurryLocation_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	city := "London"
	street := "10 Downing St"
	lat := 51.5034
	lon := -0.1276

	created, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{
		Country: "GB", City: &city, Street: &street,
		Latitude: &lat, Longitude: &lon,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, "GB", created.Country)
	assert.Equal(t, &city, created.City)
	assert.Equal(t, &street, created.Street)
	assert.InDelta(t, lat, *created.Latitude, 1e-9)
	assert.InDelta(t, lon, *created.Longitude, 1e-9)

	// Get.
	got, err := store.GetBlurryLocation(ctx, db, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Country, got.Country)
	assert.Equal(t, created.City, got.City)

	// Update — clear optional fields.
	got.City = nil
	got.Street = nil
	got.Latitude = nil
	got.Longitude = nil
	require.NoError(t, store.UpdateBlurryLocation(ctx, db, got))
	updated, err := store.GetBlurryLocation(ctx, db, got.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.City)
	assert.Nil(t, updated.Latitude)

	// Delete.
	require.NoError(t, store.DeleteBlurryLocation(ctx, db, created.ID))
	_, err = store.GetBlurryLocation(ctx, db, created.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestBlurryLocation_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := store.UpdateBlurryLocation(ctx, db, model.BlurryLocation{ID: "nonexistent", Country: "US"})
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestBlurryLocation_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	assert.ErrorIs(t, store.DeleteBlurryLocation(ctx, db, "nonexistent"), sql.ErrNoRows)
}

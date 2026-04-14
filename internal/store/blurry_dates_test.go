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

func intPtr(v int) *int { return &v }

func TestBlurryDate_CRUD(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Create — full date.
	year, month, day := 1990, 6, 15
	created, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{
		Year: &year, Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 3,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, created.ID)
	assert.Equal(t, &year, created.Year)
	assert.Equal(t, &month, created.Month)
	assert.Equal(t, &day, created.Day)
	assert.True(t, created.ReminderEnabled)
	assert.Equal(t, 3, created.ReminderDaysBefore)

	// Get.
	got, err := store.GetBlurryDate(ctx, db, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, got)

	// Update.
	got.ReminderDaysBefore = 7
	got.Year = nil // make year fuzzy
	require.NoError(t, store.UpdateBlurryDate(ctx, db, got))
	updated, err := store.GetBlurryDate(ctx, db, got.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.Year)
	assert.Equal(t, 7, updated.ReminderDaysBefore)

	// Delete.
	require.NoError(t, store.DeleteBlurryDate(ctx, db, created.ID))
	_, err = store.GetBlurryDate(ctx, db, created.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestBlurryDate_NullFields(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	// Month-only blurry date (no year, no day).
	month := 12
	created, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{
		Month: &month, ReminderEnabled: false, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)
	assert.Nil(t, created.Year)
	assert.Equal(t, &month, created.Month)
	assert.Nil(t, created.Day)
}

func TestBlurryDate_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := store.UpdateBlurryDate(ctx, db, model.BlurryDate{ID: "nonexistent"})
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestBlurryDate_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := store.DeleteBlurryDate(ctx, db, "nonexistent")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

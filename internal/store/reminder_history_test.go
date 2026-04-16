package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/store"
)

func TestGetLastSuccessfulSend_Empty(t *testing.T) {
	db := openTestDB(t)
	got, err := store.GetLastSuccessfulSend(context.Background(), db)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestCreateReminderHistory_Success(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	dateStart := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	dateEnd := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	err := store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status:    "success",
		DateStart: dateStart,
		DateEnd:   dateEnd,
	})
	require.NoError(t, err)

	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "success", got.Status)
	assert.Nil(t, got.Error)
	assert.Equal(t, dateStart, got.DateStart)
	assert.Equal(t, dateEnd, got.DateEnd)
}

func TestGetLastSuccessfulSend_ReturnsLatest(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	d1 := time.Date(2026, 4, 14, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 4, 15, 0, 0, 0, 0, time.UTC)
	d3 := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: d1, DateEnd: d1,
	}))
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: d2, DateEnd: d2,
	}))
	// A failure after the successes — should NOT be returned.
	errMsg := "dial failed"
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "failure", Error: &errMsg, DateStart: d3, DateEnd: d3,
	}))

	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, d2, got.DateStart)
}

func TestDeleteOldReminderHistory(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	recent := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)

	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: old, DateEnd: old,
	}))
	require.NoError(t, store.CreateReminderHistory(ctx, db, model.ReminderHistory{
		Status: "success", DateStart: recent, DateEnd: recent,
	}))

	cutoff := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, store.DeleteOldReminderHistory(ctx, db, cutoff))

	got, err := store.GetLastSuccessfulSend(ctx, db)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, recent, got.DateStart)
}

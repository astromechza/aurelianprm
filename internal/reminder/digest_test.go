package reminder_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/astromechza/aurelianprm/internal/model"
	"github.com/astromechza/aurelianprm/internal/reminder"
	"github.com/astromechza/aurelianprm/internal/store"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

func TestComputeReminders_Birthday(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// Birthday on June 15; reminder 2 days before.
	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)

	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	// Trigger date = June 13 → June 13 + 2 = June 15 ✓
	trigger := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Alice Smith \u2014 birthday", items[0].Label)
	assert.Equal(t, time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), items[0].OccursOn)
}

func TestComputeReminders_NotDueYet(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	// Trigger date = June 12 → June 12 + 2 = June 14 ≠ June 15 → no items.
	trigger := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestComputeReminders_ReminderDisabled(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 6, 15
	bd, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: false, ReminderDaysBefore: 2,
	})
	require.NoError(t, err)
	_, err = store.CreatePerson(ctx, sqlDB, model.Person{
		Name:       "Alice Smith",
		BirthdayID: &bd.ID,
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 6, 13, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // reminder_enabled=false → not returned by store query
}

func TestComputeReminders_RelationshipAnniversary_WithYears(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// Anniversary on April 20, 2020; reminder 1 day before.
	year, month, day := 2020, 4, 20
	since, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Year: &year, Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	person, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Bob Jones"})
	require.NoError(t, err)

	other, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Carol Jones"})
	require.NoError(t, err)

	_, err = store.CreateRelationship(ctx, sqlDB, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "married",
		SinceID:  &since.ID,
	})
	require.NoError(t, err)

	// Trigger = April 19, 2026 → April 19 + 1 = April 20 ✓ (6 years)
	trigger := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "Bob Jones & Carol Jones \u2014 married, 6 years", items[0].Label)
	assert.Equal(t, time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC), items[0].OccursOn)
}

func TestComputeReminders_EndedRelationship_Excluded(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	month, day := 4, 20
	since, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	// Set until_id to a non-null value so the relationship is ended.
	untilMonth, untilDay := 1, 1
	until, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &untilMonth, Day: &untilDay,
		ReminderEnabled: false, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	person, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Dave"})
	require.NoError(t, err)

	other, err := store.CreatePerson(ctx, sqlDB, model.Person{Name: "Eve"})
	require.NoError(t, err)

	_, err = store.CreateRelationship(ctx, sqlDB, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "married",
		SinceID:  &since.ID,
		UntilID:  &until.ID, // ended relationship
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 4, 19, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // until_id is set → excluded
}

func TestComputeReminders_OrphanedBlurryDate_Skipped(t *testing.T) {
	ctx := context.Background()
	sqlDB := openTestDB(t)

	// A blurry_date with no person birthday and no relationship since_id.
	month, day := 6, 15
	_, err := store.CreateBlurryDate(ctx, sqlDB, model.BlurryDate{
		Month: &month, Day: &day,
		ReminderEnabled: true, ReminderDaysBefore: 1,
	})
	require.NoError(t, err)

	trigger := time.Date(2026, 6, 14, 0, 0, 0, 0, time.UTC)
	items, err := reminder.ComputeReminders(ctx, sqlDB, trigger, time.UTC)
	require.NoError(t, err)
	assert.Empty(t, items) // no context → silently skipped
}

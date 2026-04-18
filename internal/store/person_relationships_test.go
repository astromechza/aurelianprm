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

func TestListPersonRelationships_Empty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)

	rows, err := store.ListPersonRelationships(ctx, db, person.ID)
	require.NoError(t, err)
	assert.NotNil(t, rows) // must be [] not nil
	assert.Empty(t, rows)
}

func TestListPersonRelationships_WithDates(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	alice, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)
	bob, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)
	employer, err := store.CreateWork(ctx, db, model.Work{Name: "ACME"})
	require.NoError(t, err)

	// Relationship with since+until dates and day precision.
	yr, mo, dy := 2018, 6, 15
	untilYr, untilMo, untilDy := 2023, 12, 31
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr, Month: &mo, Day: &dy})
	require.NoError(t, err)
	until, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &untilYr, Month: &untilMo, Day: &untilDy})
	require.NoError(t, err)
	notes := "former employer"
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: alice.ID,
		ToType:   model.RelationshipTargetWork,
		ToID:     employer.ID,
		Label:    "employee",
		SinceID:  &since.ID,
		UntilID:  &until.ID,
		Notes:    &notes,
	})
	require.NoError(t, err)

	// Relationship with no dates.
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: alice.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     bob.ID,
		Label:    "friend",
	})
	require.NoError(t, err)

	rows, err := store.ListPersonRelationships(ctx, db, alice.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	// First row: work relationship with dates.
	work := rows[0]
	assert.Equal(t, "employee", work.Relationship.Label)
	assert.Equal(t, &notes, work.Relationship.Notes)
	assert.Equal(t, &yr, work.SinceYear)
	assert.Equal(t, &mo, work.SinceMonth)
	assert.Equal(t, &dy, work.SinceDay)
	assert.Equal(t, &untilYr, work.UntilYear)
	assert.Equal(t, &untilMo, work.UntilMonth)
	assert.Equal(t, &untilDy, work.UntilDay)

	// Second row: person relationship with no dates.
	friend := rows[1]
	assert.Equal(t, "friend", friend.Relationship.Label)
	assert.Nil(t, friend.SinceYear)
	assert.Nil(t, friend.UntilYear)
	assert.Nil(t, friend.Relationship.Notes)
}

func TestGetPersonRelationshipRow_Found(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Carol"})
	require.NoError(t, err)
	other, err := store.CreatePerson(ctx, db, model.Person{Name: "Dave"})
	require.NoError(t, err)

	yr, mo, dy := 2020, 1, 5
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr, Month: &mo, Day: &dy})
	require.NoError(t, err)

	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "sibling",
		SinceID:  &since.ID,
	})
	require.NoError(t, err)

	row, err := store.GetPersonRelationshipRow(ctx, db, person.ID, rel.ID)
	require.NoError(t, err)
	assert.Equal(t, rel.ID, row.Relationship.ID)
	assert.Equal(t, "sibling", row.Relationship.Label)
	assert.Equal(t, &yr, row.SinceYear)
	assert.Equal(t, &mo, row.SinceMonth)
	assert.Equal(t, &dy, row.SinceDay)
	assert.Nil(t, row.UntilYear)
}

func TestGetPersonRelationshipRow_WrongPerson(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person1, err := store.CreatePerson(ctx, db, model.Person{Name: "Eve"})
	require.NoError(t, err)
	person2, err := store.CreatePerson(ctx, db, model.Person{Name: "Frank"})
	require.NoError(t, err)
	other, err := store.CreatePerson(ctx, db, model.Person{Name: "George"})
	require.NoError(t, err)

	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person1.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "friend",
	})
	require.NoError(t, err)

	_, err = store.GetPersonRelationshipRow(ctx, db, person2.ID, rel.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestGetPersonRelationshipRow_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Helen"})
	require.NoError(t, err)

	_, err = store.GetPersonRelationshipRow(ctx, db, person.ID, "nonexistent")
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateRelationship(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Ivan"})
	require.NoError(t, err)
	other, err := store.CreatePerson(ctx, db, model.Person{Name: "Jane"})
	require.NoError(t, err)

	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID,
		ToType:   model.RelationshipTargetPerson,
		ToID:     other.ID,
		Label:    "colleague",
	})
	require.NoError(t, err)
	assert.Nil(t, rel.SinceID)

	yr, mo, dy := 2022, 3, 10
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr, Month: &mo, Day: &dy})
	require.NoError(t, err)

	notes := "met at conference"
	require.NoError(t, store.UpdateRelationship(ctx, db, rel.ID, "friend", &notes, &since.ID, nil))

	row, err := store.GetPersonRelationshipRow(ctx, db, person.ID, rel.ID)
	require.NoError(t, err)
	assert.Equal(t, "friend", row.Relationship.Label)
	assert.Equal(t, &notes, row.Relationship.Notes)
	assert.NotNil(t, row.Relationship.SinceID)
	assert.Nil(t, row.Relationship.UntilID)
	assert.Equal(t, &yr, row.SinceYear)
	assert.Equal(t, &mo, row.SinceMonth)
	assert.Equal(t, &dy, row.SinceDay)
	assert.Nil(t, row.UntilYear)
}

func TestUpdateRelationship_NotFound(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	err := store.UpdateRelationship(ctx, db, "nonexistent", "label", nil, nil, nil)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

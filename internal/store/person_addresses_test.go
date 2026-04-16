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

func TestListPersonAddresses_Empty(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Alice"})
	require.NoError(t, err)

	rows, err := store.ListPersonAddresses(ctx, db, person.ID)
	require.NoError(t, err)
	assert.NotNil(t, rows) // must be [] not nil
	assert.Empty(t, rows)
}

func TestListPersonAddresses_CurrentAndPast(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Bob"})
	require.NoError(t, err)

	// Current address — no until_id.
	city1 := "London"
	loc1, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "UK", City: &city1})
	require.NoError(t, err)
	lbl1 := "home"
	addr1, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc1.ID, Label: &lbl1})
	require.NoError(t, err)
	yr1, mo1 := 2021, 3
	since1, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr1, Month: &mo1})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress,
		ToID: addr1.ID, SinceID: &since1.ID,
	})
	require.NoError(t, err)

	// Past address — with until_id.
	loc2, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "DE"})
	require.NoError(t, err)
	addr2, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc2.ID})
	require.NoError(t, err)
	yr2, mo2, yr2u, mo2u := 2019, 1, 2021, 2
	since2, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr2, Month: &mo2})
	require.NoError(t, err)
	until2, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr2u, Month: &mo2u})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress,
		ToID: addr2.ID, SinceID: &since2.ID, UntilID: &until2.ID,
	})
	require.NoError(t, err)

	rows, err := store.ListPersonAddresses(ctx, db, person.ID)
	require.NoError(t, err)
	require.Len(t, rows, 2)

	current := rows[0]
	assert.Equal(t, addr1.ID, current.Address.ID)
	assert.Nil(t, current.UntilID)
	assert.Equal(t, "UK", current.Location.Country)
	assert.Equal(t, &yr1, current.SinceYear)
	assert.Equal(t, &mo1, current.SinceMonth)
	assert.Nil(t, current.UntilYear)

	past := rows[1]
	assert.Equal(t, addr2.ID, past.Address.ID)
	assert.NotNil(t, past.UntilID)
	assert.Equal(t, &yr2u, past.UntilYear)
	assert.Equal(t, &mo2u, past.UntilMonth)
}

func TestGetPersonAddressRow_Found(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Carol"})
	require.NoError(t, err)
	city := "Paris"
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "FR", City: &city})
	require.NoError(t, err)
	notes := "nice area"
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID, Notes: &notes})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)

	row, err := store.GetPersonAddressRow(ctx, db, person.ID, addr.ID)
	require.NoError(t, err)
	assert.Equal(t, addr.ID, row.Address.ID)
	assert.Equal(t, "FR", row.Location.Country)
	require.NotNil(t, row.Address.Notes)
	assert.Equal(t, "nice area", *row.Address.Notes)
	assert.Nil(t, row.SinceYear)
	assert.Nil(t, row.UntilYear)
}

func TestGetPersonAddressRow_WrongPerson(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person1, err := store.CreatePerson(ctx, db, model.Person{Name: "Dave"})
	require.NoError(t, err)
	person2, err := store.CreatePerson(ctx, db, model.Person{Name: "Eve"})
	require.NoError(t, err)
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "US"})
	require.NoError(t, err)
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID})
	require.NoError(t, err)
	_, err = store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person1.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)

	_, err = store.GetPersonAddressRow(ctx, db, person2.ID, addr.ID)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}

func TestUpdateRelationshipDates(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	person, err := store.CreatePerson(ctx, db, model.Person{Name: "Frank"})
	require.NoError(t, err)
	loc, err := store.CreateBlurryLocation(ctx, db, model.BlurryLocation{Country: "AU"})
	require.NoError(t, err)
	addr, err := store.CreateAddress(ctx, db, model.Address{LocationID: loc.ID})
	require.NoError(t, err)
	rel, err := store.CreateRelationship(ctx, db, model.Relationship{
		PersonID: person.ID, ToType: model.RelationshipTargetAddress, ToID: addr.ID,
	})
	require.NoError(t, err)
	assert.Nil(t, rel.SinceID)

	yr, mo := 2020, 5
	since, err := store.CreateBlurryDate(ctx, db, model.BlurryDate{Year: &yr, Month: &mo})
	require.NoError(t, err)

	require.NoError(t, store.UpdateRelationshipDates(ctx, db, rel.ID, &since.ID, nil))

	row, err := store.GetPersonAddressRow(ctx, db, person.ID, addr.ID)
	require.NoError(t, err)
	assert.Equal(t, &yr, row.SinceYear)
	assert.Equal(t, &mo, row.SinceMonth)
	assert.Nil(t, row.UntilYear)
}

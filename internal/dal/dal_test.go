package dal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/astromechza/aurelianprm/internal/dal"
	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

func newTestDAL(t *testing.T) *dal.DAL {
	t.Helper()
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })
	return dal.New(conn, ":memory:")
}

func TestWithTx_commit(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.Exec(ctx, `INSERT INTO entities (id, type, data) VALUES ('01TEST0000000000000000001A', 'Person', '{}')`)
		return err
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestWithTx_rollbackOnError(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		if _, err := q.Exec(ctx, `INSERT INTO entities (id, type, data) VALUES ('01TEST0000000000000000001B', 'Person', '{}')`); err != nil {
			return err
		}
		return fmt.Errorf("simulated failure")
	})
	require.Error(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM entities`).Scan(&count))
	require.Equal(t, 0, count)
}

func TestCreateEntity_and_GetEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var created dal.Entity
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		created, err = q.CreateEntity(ctx, dal.CreateEntityParams{
			Type:        "Person",
			DisplayName: ptr("Alice Smith"),
			Data:        json.RawMessage(`{"name":"Alice Smith"}`),
		})
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "Person", created.Type)
	require.Equal(t, ptr("Alice Smith"), created.DisplayName)

	var fetched dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetEntity(ctx, created.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, fetched.ID)
}

func TestCreateEntity_validationFails(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"unknownField":"oops"}`),
		})
		return err
	})
	require.Error(t, err)
}

func TestUpdateEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var created dal.Entity
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		created, err = q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Bob"}`),
		})
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateEntity(ctx, dal.UpdateEntityParams{
			ID:          created.ID,
			DisplayName: ptr("Bob Jones"),
			Data:        json.RawMessage(`{"name":"Bob Jones"}`),
		})
	})
	require.NoError(t, err)

	var fetched dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetEntity(ctx, created.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, ptr("Bob Jones"), fetched.DisplayName)
}

func TestDeleteEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var id string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Carol"}`),
		})
		id = e.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, id)
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetEntity(ctx, id)
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestListEntitiesByType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		for _, name := range []string{"Dave", "Eve"} {
			if _, err := q.CreateEntity(ctx, dal.CreateEntityParams{
				Type: "Person",
				Data: json.RawMessage(`{"name":"` + name + `"}`),
			}); err != nil {
				return err
			}
		}
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress",
			Data: json.RawMessage(`{"email":"dave@example.com"}`),
		})
		return err
	})
	require.NoError(t, err)

	var people []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		people, err = q.ListEntitiesByType(ctx, "Person")
		return err
	})
	require.NoError(t, err)
	require.Len(t, people, 2)
}

func TestSearchEntities(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		for _, p := range []struct{ display, data string }{
			{"Alice Smith", `{"name":"Alice Smith"}`},
			{"Bob Jones", `{"name":"Bob Jones"}`},
			{"Alicia Keys", `{"name":"Alicia Keys"}`},
		} {
			if _, err := q.CreateEntity(ctx, dal.CreateEntityParams{
				Type:        "Person",
				DisplayName: ptr(p.display),
				Data:        json.RawMessage(p.data),
			}); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	var results []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "alice")
		return err
	})
	require.NoError(t, err)
	require.Len(t, results, 2)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "jones")
		return err
	})
	require.NoError(t, err)
	require.Len(t, results, 1)
	require.Equal(t, ptr("Bob Jones"), results[0].DisplayName)
}

func TestSearchEntities_noDisplayName_notIndexed(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Dave"}`),
		})
		return err
	})
	require.NoError(t, err)

	var results []dal.Entity
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		results, err = q.SearchEntities(ctx, "dave")
		return err
	})
	require.NoError(t, err)
	require.Empty(t, results)
}

func ptr(s string) *string { return &s }

func TestCreateRelationship_and_GetNeighbors(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID, emailID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person",
			Data: json.RawMessage(`{"name":"Frank"}`),
		})
		if err != nil {
			return err
		}
		personID = p.ID
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress",
			Data: json.RawMessage(`{"email":"frank@example.com"}`),
		})
		emailID = e.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: personID,
			EntityBID: emailID,
			Type:      "hasEmail",
			Note:      ptr("primary email"),
		})
		return err
	})
	require.NoError(t, err)

	var neighbours []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		neighbours, err = q.GetNeighbours(ctx, personID)
		return err
	})
	require.NoError(t, err)
	require.Len(t, neighbours, 1)
	require.Equal(t, emailID, neighbours[0].Entity.ID)
	require.Equal(t, "hasEmail", neighbours[0].Relationship.Type)
	require.Equal(t, "outbound", neighbours[0].Direction)
}

func TestCreateRelationship_validationFails(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Greta"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Hank"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	// hasEmail requires object to be EmailAddress, not Person — must fail.
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: idA,
			EntityBID: idB,
			Type:      "hasEmail",
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateRelationship_partialDates(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	for _, tc := range []struct{ from, to string }{
		{"2019", "2023"},
		{"2019-06", "2023-01"},
		{"2019-06-15", "2023-01-20"},
	} {
		tc := tc
		t.Run(tc.from+"_"+tc.to, func(t *testing.T) {
			var idA, idB string
			err := d.WithTx(ctx, func(q *dal.Queries) error {
				a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Pat"}`)})
				if err != nil {
					return err
				}
				idA = a.ID
				b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Quinn"}`)})
				idB = b.ID
				return err
			})
			require.NoError(t, err)

			err = d.WithTx(ctx, func(q *dal.Queries) error {
				_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
					EntityAID: idA, EntityBID: idB, Type: "knows",
					DateFrom: &tc.from, DateTo: &tc.to,
				})
				return err
			})
			require.NoError(t, err, "expected valid for from=%s to=%s", tc.from, tc.to)
		})
	}

	// Invalid formats must be rejected.
	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Pat2"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Quinn2"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	for _, bad := range []string{"01-2019", "2019/06/15", "not-a-date", "20190615"} {
		bad := bad
		err = d.WithTx(ctx, func(q *dal.Queries) error {
			_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
				EntityAID: idA, EntityBID: idB, Type: "knows",
				DateFrom: &bad,
			})
			return err
		})
		require.Error(t, err, "expected error for date_from=%q", bad)
	}
}

func TestCreateRelationship_symmetric_queryBothDirections(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var idA, idB string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		a, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Iris"}`)})
		if err != nil {
			return err
		}
		idA = a.ID
		b, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Jack"}`)})
		idB = b.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: idA,
			EntityBID: idB,
			Type:      "knows",
		})
		return err
	})
	require.NoError(t, err)

	// GetNeighbours from idB should surface the relationship as "inbound".
	var neighbours []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		neighbours, err = q.GetNeighbours(ctx, idB)
		return err
	})
	require.NoError(t, err)
	require.Len(t, neighbours, 1)
	require.Equal(t, idA, neighbours[0].Entity.ID)
	require.Equal(t, "inbound", neighbours[0].Direction)
}

func TestDeleteRelationship_cascadesWithEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Kate"}`)})
		if err != nil {
			return err
		}
		personID = p.ID
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"kate@example.com"}`)})
		if err != nil {
			return err
		}
		_, err = q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: personID, EntityBID: e.ID, Type: "hasEmail"})
		return err
	})
	require.NoError(t, err)

	// Deleting the person should cascade to the relationship.
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteEntity(ctx, personID)
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM relationships`).Scan(&count))
	require.Equal(t, 0, count)
}

func TestUpdateRelationship(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var relID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Leo"}`)})
		if err != nil {
			return err
		}
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"leo@example.com"}`)})
		if err != nil {
			return err
		}
		r, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: p.ID, EntityBID: e.ID, Type: "hasEmail",
		})
		relID = r.ID
		return err
	})
	require.NoError(t, err)

	dateFrom := "2020-03"
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       relID,
			DateFrom: &dateFrom,
			Note:     ptr("primary work email"),
		})
	})
	require.NoError(t, err)

	var note, df string
	require.NoError(t, d.DB().QueryRowContext(ctx,
		`SELECT note, date_from FROM relationships WHERE id=?`, relID,
	).Scan(&note, &df))
	require.Equal(t, "primary work email", note)
	require.Equal(t, "2020-03", df)
}

func TestUpdateRelationship_invalidDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var relID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Mia"}`)})
		if err != nil {
			return err
		}
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"mia@example.com"}`)})
		if err != nil {
			return err
		}
		r, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{
			EntityAID: p.ID, EntityBID: e.ID, Type: "hasEmail",
		})
		relID = r.ID
		return err
	})
	require.NoError(t, err)

	bad := "not-a-date"
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateRelationship(ctx, dal.UpdateRelationshipParams{
			ID:       relID,
			DateFrom: &bad,
		})
	})
	require.Error(t, err)
}

func TestGetNeighboursByRelType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "Person", Data: json.RawMessage(`{"name":"Nina"}`)})
		if err != nil {
			return err
		}
		personID = p.ID
		email, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "EmailAddress", Data: json.RawMessage(`{"email":"nina@example.com"}`)})
		if err != nil {
			return err
		}
		phone, err := q.CreateEntity(ctx, dal.CreateEntityParams{Type: "PhoneNumber", Data: json.RawMessage(`{"telephone":"+1 555 000 1234"}`)})
		if err != nil {
			return err
		}
		if _, err := q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: p.ID, EntityBID: email.ID, Type: "hasEmail"}); err != nil {
			return err
		}
		_, err = q.CreateRelationship(ctx, dal.CreateRelationshipParams{EntityAID: p.ID, EntityBID: phone.ID, Type: "hasPhone"})
		return err
	})
	require.NoError(t, err)

	var emails []dal.NeighbourResult
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		emails, err = q.GetNeighboursByRelType(ctx, personID, "hasEmail")
		return err
	})
	require.NoError(t, err)
	require.Len(t, emails, 1)
	require.Equal(t, "EmailAddress", emails[0].Entity.Type)
}

func TestCreateNote_and_GetNote(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Alice"}`),
		})
		personID = p.ID
		return err
	})
	require.NoError(t, err)

	var note dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		note, err = q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "CONVERSATION",
			Date:      "2026-04-26",
			Content:   "Caught up over coffee.",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.NoError(t, err)
	require.NotEmpty(t, note.ID)
	require.Equal(t, "CONVERSATION", note.Type)
	require.Equal(t, "2026-04-26", note.Date)
	require.Equal(t, "Caught up over coffee.", note.Content)
	require.Len(t, note.Persons, 1)
	require.Equal(t, personID, note.Persons[0].ID)

	var fetched dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetNote(ctx, note.ID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, note.ID, fetched.ID)
	require.Len(t, fetched.Persons, 1)
}

func TestGetNote_notFound(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetNote(ctx, "NOTEXIST")
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestCreateNote_invalidType(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Bob"}`),
		})
		personID = p.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "INVALID",
			Date:      "2026-04-26",
			Content:   "test",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateNote_invalidDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Carol"}`),
		})
		personID = p.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "HANGOUT",
			Date:      "2026-4-1", // wrong format
			Content:   "test",
			PersonIDs: []string{personID},
		})
		return err
	})
	require.Error(t, err)
}

func TestCreateNote_nonPersonEntity(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var emailID string
	_ = d.WithTx(ctx, func(q *dal.Queries) error {
		e, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "EmailAddress", Data: json.RawMessage(`{"email":"x@x.com"}`),
		})
		emailID = e.ID
		return err
	})

	err := d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "CONVERSATION",
			Date:      "2026-04-26",
			Content:   "test",
			PersonIDs: []string{emailID},
		})
		return err
	})
	require.Error(t, err)
}

func TestUpdateNote_replacesPersons(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var p1ID, p2ID, noteID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p1, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Dave"}`),
		})
		if err != nil {
			return err
		}
		p1ID = p1.ID
		p2, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Eve"}`),
		})
		if err != nil {
			return err
		}
		p2ID = p2.ID
		note, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "HANGOUT",
			Date:      "2026-04-26",
			Content:   "initial",
			PersonIDs: []string{p1ID},
		})
		noteID = note.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.UpdateNote(ctx, dal.UpdateNoteParams{
			ID:        noteID,
			Type:      "CONVERSATION",
			Date:      "2026-04-27",
			Content:   "updated",
			PersonIDs: []string{p1ID, p2ID},
		})
	})
	require.NoError(t, err)

	var fetched dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		fetched, err = q.GetNote(ctx, noteID)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, "CONVERSATION", fetched.Type)
	require.Equal(t, "2026-04-27", fetched.Date)
	require.Equal(t, "updated", fetched.Content)
	require.Len(t, fetched.Persons, 2)
}

func TestDeleteNote_cascades(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var noteID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Frank"}`),
		})
		if err != nil {
			return err
		}
		note, err := q.CreateNote(ctx, dal.CreateNoteParams{
			Type:      "LIFE_EVENT",
			Date:      "2026-04-26",
			Content:   "graduated",
			PersonIDs: []string{p.ID},
		})
		noteID = note.ID
		return err
	})
	require.NoError(t, err)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		return q.DeleteNote(ctx, noteID)
	})
	require.NoError(t, err)

	var count int
	require.NoError(t, d.DB().QueryRowContext(ctx,
		`SELECT COUNT(*) FROM note_persons WHERE note_id=?`, noteID).Scan(&count))
	require.Equal(t, 0, count)

	err = d.WithTx(ctx, func(q *dal.Queries) error {
		_, err := q.GetNote(ctx, noteID)
		return err
	})
	require.ErrorIs(t, err, sql.ErrNoRows)
}

func TestListNotesForPerson_orderedByDate(t *testing.T) {
	d := newTestDAL(t)
	ctx := context.Background()

	var personID string
	err := d.WithTx(ctx, func(q *dal.Queries) error {
		p, err := q.CreateEntity(ctx, dal.CreateEntityParams{
			Type: "Person", Data: json.RawMessage(`{"name":"Grace"}`),
		})
		if err != nil {
			return err
		}
		personID = p.ID
		for _, date := range []string{"2026-01-01", "2026-03-15", "2026-02-10"} {
			if _, err := q.CreateNote(ctx, dal.CreateNoteParams{
				Type:      "HANGOUT",
				Date:      date,
				Content:   "note on " + date,
				PersonIDs: []string{personID},
			}); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	var notes []dal.NoteWithPersons
	err = d.WithTx(ctx, func(q *dal.Queries) error {
		var err error
		notes, err = q.ListNotesForPerson(ctx, personID)
		return err
	})
	require.NoError(t, err)
	require.Len(t, notes, 3)
	// Most recent first
	require.Equal(t, "2026-03-15", notes[0].Date)
	require.Equal(t, "2026-02-10", notes[1].Date)
	require.Equal(t, "2026-01-01", notes[2].Date)
}

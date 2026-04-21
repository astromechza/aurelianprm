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
	return dal.New(conn)
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

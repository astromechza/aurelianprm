package dal_test

import (
	"context"
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

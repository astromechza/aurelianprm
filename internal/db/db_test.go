package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

// TestOpen_CascadeOnSecondaryConnection guards against a latent bug where
// foreign-key enforcement is applied via a PRAGMA on the *sql.DB pool, which
// only affects the single connection it ran on. It uses a file-backed database
// (in-memory DBs collapse the pool to one connection), raises the max open
// connections above one, and pins the initial connection so the cascading
// delete is forced onto a *second* connection — asserting ON DELETE CASCADE
// still fires there. This proves the pragmas are applied per-connection (via
// the DSN) and not merely to whichever connection happened to run them first.
func TestOpen_CascadeOnSecondaryConnection(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "prm.db")

	conn, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	conn.SetMaxOpenConns(2)

	// Pin connection #1 so subsequent pool operations must use connection #2.
	pinned, err := conn.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pinned.Close() })

	_, err = conn.ExecContext(ctx, `INSERT INTO entities (id, type, data) VALUES
		('01JTESTCASCADE0000000000A', 'Person', '{}'),
		('01JTESTCASCADE0000000000B', 'EmailAddress', '{}')`)
	require.NoError(t, err)
	_, err = conn.ExecContext(ctx, `INSERT INTO relationships (id, entity_a_id, entity_b_id, type)
		VALUES ('01JTESTCASCADE0000000000R', '01JTESTCASCADE0000000000A', '01JTESTCASCADE0000000000B', 'hasEmail')`)
	require.NoError(t, err)

	// Deleting entity A must cascade to the relationship referencing it, but
	// only if FKs are enforced on whichever connection the pool hands us here.
	_, err = conn.ExecContext(ctx, `DELETE FROM entities WHERE id = '01JTESTCASCADE0000000000A'`)
	require.NoError(t, err)

	var count int
	require.NoError(t, conn.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM relationships WHERE id = '01JTESTCASCADE0000000000R'`).Scan(&count))
	require.Equal(t, 0, count, "ON DELETE CASCADE must fire on a non-initial pooled connection")
}

func TestOpen_pragmas(t *testing.T) {
	// Use a file-based database since :memory: doesn't support WAL mode
	tmpFile := t.TempDir() + "/test.db"

	conn, err := db.Open(tmpFile)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	var journalMode string
	require.NoError(t, conn.QueryRow("PRAGMA journal_mode").Scan(&journalMode))
	require.Equal(t, "wal", journalMode)

	var fkEnabled int
	require.NoError(t, conn.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled))
	require.Equal(t, 1, fkEnabled)
}

func TestOpen_entitiesTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec(
		`INSERT INTO entities (id, type, display_name, data) VALUES ('01JTEST00000000000000000001', 'Person', 'Alice', '{"name":"Alice"}')`,
	)
	require.NoError(t, err)

	var typ, name string
	require.NoError(t, conn.QueryRow(
		`SELECT type, display_name FROM entities WHERE id='01JTEST00000000000000000001'`,
	).Scan(&typ, &name))
	require.Equal(t, "Person", typ)
	require.Equal(t, "Alice", name)
}

func TestOpen_relationshipsTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec(`INSERT INTO entities (id, type, data) VALUES
		('01JTEST00000000000000000002', 'Person', '{}'),
		('01JTEST00000000000000000003', 'EmailAddress', '{}')`)
	require.NoError(t, err)

	_, err = conn.Exec(`INSERT INTO relationships (id, entity_a_id, entity_b_id, type)
		VALUES ('01JTEST00000000000000000004', '01JTEST00000000000000000002', '01JTEST00000000000000000003', 'hasEmail')`)
	require.NoError(t, err)

	_, err = conn.Exec(`INSERT INTO relationships (id, entity_a_id, entity_b_id, type)
		VALUES ('01JTEST00000000000000000005', '01JTEST00000000000000000002', '01JTEST00000000000000000002', 'knows')`)
	require.Error(t, err)
}

func TestOpen_entitiesFTSTable(t *testing.T) {
	conn, err := db.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.Exec(`INSERT INTO entities (id, type, display_name, data)
		VALUES ('01JTEST00000000000000000010', 'Person', 'Alice Smith', '{"name":"Alice Smith"}')`)
	require.NoError(t, err)

	var entityID string
	require.NoError(t, conn.QueryRow(
		`SELECT entity_id FROM entities_fts WHERE display_name MATCH 'alice'`,
	).Scan(&entityID))
	require.Equal(t, "01JTEST00000000000000000010", entityID)
}

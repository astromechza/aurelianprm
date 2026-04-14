package db_test

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"

	"github.com/astromechza/aurelianprm/internal/db"
)

func TestOpen_InMemory(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer sqlDB.Close() //nolint:errcheck

	require.NoError(t, sqlDB.Ping())
}

func TestOpen_SchemaCreated(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer sqlDB.Close() //nolint:errcheck

	tables := []string{
		"blurry_dates",
		"blurry_locations",
		"people",
		"contact_methods",
		"addresses",
		"pets",
		"work",
		"interactions",
		"relationships",
	}

	for _, table := range tables {
		var name string
		row := sqlDB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		)
		err := row.Scan(&name)
		require.NoError(t, err, "table %q should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestOpen_ForeignKeysEnabled(t *testing.T) {
	sqlDB, err := db.Open(":memory:")
	require.NoError(t, err)
	defer sqlDB.Close() //nolint:errcheck

	var enabled int
	require.NoError(t, sqlDB.QueryRow("PRAGMA foreign_keys").Scan(&enabled))
	assert.Equal(t, 1, enabled)
}

func TestOpen_InvalidPath(t *testing.T) {
	// A directory path is not a valid SQLite file.
	_, err := db.Open("/nonexistent/deep/path/prm.db")
	require.Error(t, err)
}

func TestMigrate_ClosedDB(t *testing.T) {
	// A closed *sql.DB causes provider.Up to fail, exercising the migrate error path.
	sqlDB, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	err = db.Migrate(sqlDB)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "running migrations")
}

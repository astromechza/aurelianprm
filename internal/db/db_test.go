package db_test

import (
	"testing"

	"github.com/astromechza/aurelianprm/internal/db"
	"github.com/stretchr/testify/require"
)

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

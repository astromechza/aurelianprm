package store_test

import (
	"database/sql"
	"testing"

	"github.com/astromechza/aurelianprm/internal/db"
)

// openTestDB opens a fresh in-memory SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("openTestDB: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return sqlDB
}

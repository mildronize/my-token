package usage

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver for these tests
)

// repoRootForTests resolves the module root from this test file's own
// location (mirrors internal/domain/todo's todo_testutil_test.go), so
// tests work regardless of the directory `go test` is invoked from.
func repoRootForTests(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "could not determine this test file's own location")
	// this file lives at <root>/internal/domain/usage/<this file>.go.
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(thisFile))))
}

// newTestDB opens a fresh temp-file SQLite database and applies every
// migration under db/migrations via goose — the same migrations `goose
// up` applies in production.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")
	conn, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	require.NoError(t, goose.SetDialect("sqlite3"))
	migrationsDir := filepath.Join(repoRootForTests(t), "db", "migrations")
	require.NoError(t, goose.Up(conn, migrationsDir))

	return conn
}

// countRows returns table's current row count — mirrors
// internal/domain/todo's own countRows helper, used here to assert
// idempotency directly (a repeat id must not add a second row) rather
// than inferring it from a returned count alone.
func countRows(t *testing.T, conn *sql.DB, table string) int {
	t.Helper()
	var n int
	require.NoError(t, conn.QueryRow(`SELECT COUNT(*) FROM `+table).Scan(&n))
	return n
}

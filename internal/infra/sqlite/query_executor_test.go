package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newExecTestDB(t *testing.T) *ReadOnlyQueryExecutor {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(context.Background(),
		`CREATE TABLE t (id INTEGER, name TEXT, lat REAL, lon REAL)`)
	require.NoError(t, err)
	for _, r := range []struct {
		id       int
		name     string
		lat, lon float64
	}{{1, "a", 0, 0}, {2, "b", 0, 1}, {3, "c", 10, 10}} {
		_, err := db.ExecContext(context.Background(),
			`INSERT INTO t VALUES (?,?,?,?)`, r.id, r.name, r.lat, r.lon)
		require.NoError(t, err)
	}
	return NewReadOnlyQueryExecutor(db)
}

func TestReadOnlyQueryExecutor_RejectsWrites(t *testing.T) {
	e := newExecTestDB(t)
	for _, q := range []string{
		"UPDATE t SET name='x'",
		"DELETE FROM t",
		"PRAGMA table_info(t)",
		"SELECT 1; DROP TABLE t",
	} {
		_, err := e.QueryRows(context.Background(), q)
		assert.Error(t, err, "must reject non-read-only SQL: %q", q)
	}
}

func TestReadOnlyQueryExecutor_MapsRows(t *testing.T) {
	e := newExecTestDB(t)
	rows, err := e.QueryRows(context.Background(), "SELECT id, name FROM t WHERE id=1")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, int64(1), rows[0]["id"])
	assert.Equal(t, "a", rows[0]["name"], "TEXT must surface as string, not []byte")
}

func TestReadOnlyQueryExecutor_HaversineAndCTE(t *testing.T) {
	e := newExecTestDB(t)
	// CTE + registered haversine_km: points within 200km of (0,0).
	rows, err := e.QueryRows(context.Background(),
		`WITH near AS (SELECT name, haversine_km(0,0,lat,lon) AS km FROM t)
		 SELECT name FROM near WHERE km <= 200 ORDER BY name`)
	require.NoError(t, err)
	names := make([]string, len(rows))
	for i, r := range rows {
		names[i] = r["name"].(string)
	}
	assert.Equal(t, []string{"a", "b"}, names, "a(0,0) and b(0,~111km) within 200km; c excluded")
}

func TestReadOnlyQueryExecutor_RowCap(t *testing.T) {
	e := newExecTestDB(t)
	e.rowCap = 2
	rows, err := e.QueryRows(context.Background(), "SELECT id FROM t ORDER BY id")
	require.NoError(t, err)
	assert.Len(t, rows, 2, "row cap must bound results")
}

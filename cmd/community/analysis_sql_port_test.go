package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/ai/analysis"
	"github.com/Alevsk/respondent/internal/ai/schema"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// TestAnalysisDefinitions_SQLExecutesOnSQLite asserts every enabled analysis.d
// definition's data.sql executes on SQLite without a dialect/function error — i.e.
// the PostgreSQL/PostGIS port to haversine_km + json_extract + SQLite syntax is
// correct. An empty migrated DB is sufficient: a correct query returns 0 rows; a
// query with a residual ST_*/->>/LATERAL/etc. errors at execution.
func TestAnalysisDefinitions_SQLExecutesOnSQLite(t *testing.T) {
	db, err := sqlitedb.Open(":memory:", zerolog.Nop())
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	require.NoError(t, db.RunMigrations())

	loader, err := analysis.NewLoader(schema.NewRegistry(), zerolog.Nop())
	require.NoError(t, err)
	defs, err := loader.LoadDefinitions(filepath.Join("..", "..", "analysis.d"))
	require.NoError(t, err)

	exec := sqlitedb.NewReadOnlyQueryExecutor(db.SqlDB())
	ran := 0
	for name, def := range defs {
		if !def.Enabled || def.Data.SQL == "" {
			continue
		}
		ran++
		if _, err := exec.QueryRows(context.Background(), def.Data.SQL); err != nil {
			t.Errorf("analysis %q data.sql failed on SQLite: %v", name, err)
		}
	}
	require.Positive(t, ran, "expected at least one enabled SQL analysis to validate")
}

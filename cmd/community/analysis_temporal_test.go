package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/ai/analysis"
	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/domain"
	sqlitedb "github.com/Alevsk/respondent/internal/infra/sqlite"
)

// loadAnalysisSQL loads one enabled analysis definition's data.sql from analysis.d/.
func loadAnalysisSQL(t *testing.T, name string) string {
	t.Helper()
	loader, err := analysis.NewLoader(schema.NewRegistry(), zerolog.Nop())
	require.NoError(t, err)
	defs, err := loader.LoadDefinitions(filepath.Join("..", "..", "analysis.d"))
	require.NoError(t, err)
	def, ok := defs[name]
	require.True(t, ok, "analysis %q not found / not enabled", name)
	require.NotEmpty(t, def.Data.SQL)
	return def.Data.SQL
}

// TestSevereWeatherAviation_StaleWeatherNotCorrelated is a regression test for the
// temporal-correlation defect: a current flight was correlated with a long-stale
// weather alert (a flight last seen 29s ago paired with a flood warning last seen
// 10h ago). The fix bounds the weather latest-observation to a 1h recency window and
// adds a cross-observation proximity clause. A FRESH alert at the flight's location
// must still correlate; a STALE one must not.
func TestSevereWeatherAviation_StaleWeatherNotCorrelated(t *testing.T) {
	sql := loadAnalysisSQL(t, "severe_weather_aviation")

	// rowsFor seeds one Extreme weather alert (observed weatherAge ago) and one
	// flight (observed 29s ago) at the SAME coordinates, runs the analysis SQL, and
	// returns the number of correlated rows.
	rowsFor := func(t *testing.T, weatherAge time.Duration) int {
		t.Helper()
		db, err := sqlitedb.Open(":memory:", zerolog.Nop())
		require.NoError(t, err)
		defer func() { _ = db.Close() }()
		require.NoError(t, db.RunMigrations())

		entityRepo := sqlitedb.NewEntityRepository(db.SqlDB(), zerolog.Nop())
		obsRepo := sqlitedb.NewObservationRepository(db.SqlDB(), zerolog.Nop())
		ctx := context.Background()
		now := time.Now().UTC()
		const lat, lon = 30.2, -93.2 // Lake Charles, LA

		weather := &domain.Entity{
			ID: uuid.New().String(), ExternalID: "W1", LayerType: "weather_alerts",
			Name: "Flood Warning",
			Metadata: map[string]string{
				"severity": "Extreme", "event": "Flood Warning",
				"area_desc": "Lake Charles LA", "urgency": "Expected", "certainty": "Likely",
			},
			AIMetadata: map[string]any{}, Source: "noaa",
		}
		require.NoError(t, entityRepo.Create(ctx, weather))
		require.NoError(t, obsRepo.Create(ctx, &domain.Observation{
			ID: uuid.New().String(), EntityID: weather.ID, Timestamp: now.Add(-weatherAge),
			Position: &domain.GeoPoint{Lat: lat, Lon: lon}, Velocity: map[string]float64{},
			Metadata: map[string]string{}, AIMetadata: map[string]any{},
			SourceType: "weather_alerts", ContentHash: uuid.New().String(), Source: "noaa",
		}))

		flight := &domain.Entity{
			ID: uuid.New().String(), ExternalID: "AB12CD", LayerType: "flights_commercial",
			Name:       "N963LT",
			Metadata:   map[string]string{"callsign": "N963LT", "type": "C172"},
			AIMetadata: map[string]any{}, Source: "adsb",
		}
		require.NoError(t, entityRepo.Create(ctx, flight))
		require.NoError(t, obsRepo.Create(ctx, &domain.Observation{
			ID: uuid.New().String(), EntityID: flight.ID, Timestamp: now.Add(-29 * time.Second),
			Position: &domain.GeoPoint{Lat: lat, Lon: lon}, AltitudeM: 500,
			Velocity: map[string]float64{"speed": 80}, Metadata: map[string]string{},
			AIMetadata: map[string]any{}, SourceType: "flights_commercial",
			ContentHash: uuid.New().String(), Source: "adsb",
		}))

		rows, err := sqlitedb.NewReadOnlyQueryExecutor(db.SqlDB()).QueryRows(ctx, sql)
		require.NoError(t, err)
		return len(rows)
	}

	assert.Equal(t, 1, rowsFor(t, 5*time.Minute),
		"a fresh Extreme alert at the flight's location must correlate")
	assert.Equal(t, 0, rowsFor(t, 10*time.Hour),
		"a 10h-stale alert must NOT correlate with a current flight (the reported bug)")
}

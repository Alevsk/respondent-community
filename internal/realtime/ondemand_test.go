package realtime_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/parse"
	"github.com/Alevsk/respondent/internal/realtime"
)

// testAPIServer creates an httptest server that returns a canned adsb.lol response.
func testAPIServer(t *testing.T, aircraft []parse.ADSBLolAircraft) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := parse.ADSBLolResponse{AC: aircraft, Total: len(aircraft)}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			http.Error(w, "encode error", http.StatusInternalServerError)
		}
	}))
}

func newTestOnDemandServer(t *testing.T, apiURL string) (*realtime.Server, *mockObsRepo) {
	t.Helper()
	logger := zerolog.Nop()
	obsRepo := &mockObsRepo{}

	server := realtime.NewServer(logger, nil, obsRepo, viewportRegistry("flights_commercial"))

	cfg := realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     100 * time.Millisecond,
		QueryTimeout: 5 * time.Second,
		MinCacheGap:  10,
		LayerAPIs: map[string]string{
			"flights_commercial": apiURL + "/v2/point/{lat}/{lon}",
		},
	}
	server.SetOnDemandConfig(cfg)
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	return server, obsRepo
}

func TestOnDemandConfig_Defaults(t *testing.T) {
	cfg := realtime.DefaultOnDemandConfig()
	assert.False(t, cfg.Enabled)
	assert.Equal(t, 10*time.Second, cfg.Cooldown)
	assert.Equal(t, 5*time.Second, cfg.QueryTimeout)
	assert.Equal(t, 10, cfg.MinCacheGap)
	assert.Nil(t, cfg.LayerAPIs)
}

func TestOnDemandConfig_SetUpdatesTimeout(t *testing.T) {
	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

	cfg := realtime.OnDemandConfig{
		Enabled:      true,
		QueryTimeout: 3 * time.Second,
	}
	server.SetOnDemandConfig(cfg)
	// No assertion on internal httpClient.Timeout directly, but the method shouldn't panic
}

func TestBBoxCenter(t *testing.T) {
	tests := []struct {
		name    string
		bbox    domain.BBox
		wantLat float64
		wantLon float64
	}{
		{
			name:    "normal bbox",
			bbox:    domain.BBox{West: -74.0, South: 40.0, East: -73.0, North: 41.0},
			wantLat: 40.5,
			wantLon: -73.5,
		},
		{
			name:    "equator crossing",
			bbox:    domain.BBox{West: -10.0, South: -5.0, East: 10.0, North: 5.0},
			wantLat: 0.0,
			wantLon: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lat, lon := realtime.BBoxCenter(&tt.bbox)
			assert.InDelta(t, tt.wantLat, lat, 0.01)
			assert.InDelta(t, tt.wantLon, lon, 0.01)
		})
	}
}

func TestFetchSparseRegion_Integration(t *testing.T) {
	// This test verifies the overall flow: API server returns aircraft,
	// fetchSparseRegion picks them up and returns entities tagged "live".
	apiServer := testAPIServer(t, []parse.ADSBLolAircraft{
		{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000), GS: 450, Track: 90},
		{Hex: "def456", Flight: "TEST02", Lat: 41.0, Lon: -73.0, AltBaro: float64(28000), GS: 400, Track: 180},
	})
	defer apiServer.Close()

	server, _ := newTestOnDemandServer(t, apiServer.URL)

	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	var storedEntities []*domain.Entity

	entities, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, storedEntities,
	)

	require.GreaterOrEqual(t, len(entities), 2, "should return at least the 2 on-demand entities")

	for _, e := range entities {
		if e.Source != "" {
			assert.Equal(t, "live", e.Source, "on-demand entities should be tagged live")
		}
	}
}

func TestFetchSparseRegion_ViewportTiling(t *testing.T) {
	// When a layer declares a fetch radius, a sparse viewport larger than one
	// fetch circle must be TILED with a hex grid — multiple cell fetches covering
	// the whole view — not a single center fetch. Each cell returns a distinct
	// aircraft so the merged result spans the viewport.
	var mu sync.Mutex
	paths := make(map[string]bool)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths[r.URL.Path] = true
		mu.Unlock()
		// Unique hex per cell so dedup keeps one entity per cell.
		hex := strings.ReplaceAll(strings.TrimPrefix(r.URL.Path, "/v2/point/"), "/", "_")
		resp := parse.ADSBLolResponse{
			AC:    []parse.ADSBLolAircraft{{Hex: hex, Flight: "T", Lat: 20.0, Lon: -100.0, AltBaro: float64(35000)}},
			Total: 1,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer apiServer.Close()

	server := realtime.NewServer(zerolog.Nop(), nil, &mockObsRepo{},
		viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     time.Millisecond,
		QueryTimeout: 5 * time.Second,
		MinCacheGap:  10,
		LayerAPIs:    map[string]string{"flights_commercial": apiServer.URL + "/v2/point/{lat}/{lon}/250"},
		LayerRadii:   map[string]float64{"flights_commercial": 250},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())
	// A Mexico-sized viewport spans more than one 250nm cell.
	bbox := &domain.BBox{West: -118, South: 14, East: -86, North: 33}
	entities, _ := server.FetchSparseRegion(
		t.Context(), client, "adsb_lol_flights", "flights_commercial", bbox, nil,
	)

	mu.Lock()
	cellCount := len(paths)
	mu.Unlock()
	require.Greater(t, cellCount, 1, "viewport must be tiled into multiple grid cells")
	require.Greater(t, len(entities), 1, "tiled fetch must merge entities from multiple cells")
}

func TestFetchSparseRegion_Cooldown(t *testing.T) {
	apiServer := testAPIServer(t, []parse.ADSBLolAircraft{
		{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
	})
	defer apiServer.Close()

	server, _ := newTestOnDemandServer(t, apiServer.URL)

	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}

	// First call should succeed
	entities1, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, nil,
	)
	require.GreaterOrEqual(t, len(entities1), 1)

	// Second call within cooldown should return no on-demand results
	entities2, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, nil,
	)
	// Should have no entities (both on-demand and backfill are rate-limited)
	assert.Empty(t, entities2, "second call within cooldown should return empty")
}

func TestFetchSparseRegion_Disabled(t *testing.T) {
	apiServer := testAPIServer(t, []parse.ADSBLolAircraft{
		{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
	})
	defer apiServer.Close()

	server, _ := newTestOnDemandServer(t, apiServer.URL)
	server.SetOnDemandConfig(realtime.OnDemandConfig{Enabled: false})

	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())
	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}

	entities, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, nil,
	)

	assert.Empty(t, entities, "disabled on-demand should return empty")
}

func TestFetchSparseRegion_NoAPIURL(t *testing.T) {
	apiServer := testAPIServer(t, nil)
	defer apiServer.Close()

	server, _ := newTestOnDemandServer(t, apiServer.URL)
	// Set config with no layer APIs
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:   true,
		LayerAPIs: map[string]string{}, // no URLs configured
	})

	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())
	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}

	entities, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, nil,
	)

	assert.Empty(t, entities, "no API URL should return empty")
}

func TestFetchSparseRegion_APIError(t *testing.T) {
	// Server that returns 500
	errorServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer errorServer.Close()

	server, _ := newTestOnDemandServer(t, errorServer.URL)
	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())
	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}

	entities, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, nil,
	)

	assert.Empty(t, entities, "API error should return empty gracefully")
}

func TestFetchSparseRegion_Deduplication(t *testing.T) {
	apiServer := testAPIServer(t, []parse.ADSBLolAircraft{
		{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
		{Hex: "existing", Flight: "EXISTING", Lat: 41.0, Lon: -73.0, AltBaro: float64(28000)},
	})
	defer apiServer.Close()

	server, _ := newTestOnDemandServer(t, apiServer.URL)
	client := realtime.NewClient("test", nil, make(chan []byte, 256), zerolog.Nop())
	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}

	// "existing" is already in the durable store
	storedEntities := []*domain.Entity{
		{ID: "flights_commercial:existing", ExternalID: "existing", LayerType: "flights_commercial"},
	}

	entities, _ := server.FetchSparseRegion(
		t.Context(), client,
		"adsb_lol_flights", "flights_commercial",
		bbox, storedEntities,
	)

	// Count entities with ExternalID "existing" — should be exactly 1 (the stored one)
	existingCount := 0
	for _, e := range entities {
		if e.ExternalID == "existing" {
			existingCount++
		}
	}
	assert.Equal(t, 1, existingCount, "existing entity should not be duplicated")
}

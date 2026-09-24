package realtime_test

// This file contains tests for the continuous on-demand polling feature:
//   - getOnDemandPollInterval
//   - startOnDemandTicker / stopOnDemandTicker / stopAllOnDemandTickers
//   - runOnDemandPoll (via integration: start ticker + wait for delivery)
//   - doOnDemandFetchDirect
//   - NewClient initialises onDemandTickers map

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// ---------------------------------------------------------------------------
// Spatial cache stub
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTickerTestServer creates a Server + Client pair pre-wired together so
// that startOnDemandTicker can call server.getOnDemandPollInterval and
// server.doOnDemandFetchDirect.
func newTickerTestServer(t *testing.T, apiURL string, pollInterval time.Duration, obsRepo *mockObsRepo) (*realtime.Server, *realtime.Client) {
	t.Helper()
	logger := zerolog.Nop()

	server := realtime.NewServer(logger, nil, obsRepo, viewportRegistry("flights_commercial"))

	cfg := realtime.OnDemandConfig{
		Enabled:      true,
		Cooldown:     100 * time.Millisecond,
		QueryTimeout: 5 * time.Second,
		MinCacheGap:  10,
		LayerAPIs: map[string]string{
			"flights_commercial": apiURL + "/v2/point/{lat}/{lon}",
		},
		PollIntervals: map[string]time.Duration{
			"flights_commercial": pollInterval,
		},
	}
	server.SetOnDemandConfig(cfg)
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	send := make(chan []byte, 256)
	client := realtime.NewClient("ticker-test", nil, send, logger)
	client.SetServer(server)

	return server, client
}

// ---------------------------------------------------------------------------
// TestGetOnDemandPollInterval
// ---------------------------------------------------------------------------

func TestGetOnDemandPollInterval(t *testing.T) {
	tests := []struct {
		name          string
		pollIntervals map[string]time.Duration // nil means no PollIntervals set
		layerType     string
		want          time.Duration
	}{
		{
			name:          "nil map returns 0",
			pollIntervals: nil,
			layerType:     "flights_commercial",
			want:          0,
		},
		{
			name:          "empty map returns 0 for any key",
			pollIntervals: map[string]time.Duration{},
			layerType:     "flights_commercial",
			want:          0,
		},
		{
			name:          "missing key returns 0",
			pollIntervals: map[string]time.Duration{"satellites": 30 * time.Second},
			layerType:     "flights_commercial",
			want:          0,
		},
		{
			name:          "matching key returns configured duration",
			pollIntervals: map[string]time.Duration{"flights_commercial": 15 * time.Second},
			layerType:     "flights_commercial",
			want:          15 * time.Second,
		},
		{
			name: "multiple keys returns correct one",
			pollIntervals: map[string]time.Duration{
				"flights_commercial": 10 * time.Second,
				"satellites":         45 * time.Second,
				"ships":              60 * time.Second,
			},
			layerType: "satellites",
			want:      45 * time.Second,
		},
		{
			name: "multiple keys selects first layer correctly",
			pollIntervals: map[string]time.Duration{
				"flights_commercial": 10 * time.Second,
				"satellites":         45 * time.Second,
			},
			layerType: "flights_commercial",
			want:      10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			server := realtime.NewServer(logger, nil, nil, domain.NewDynamicSourceRegistry())

			cfg := realtime.OnDemandConfig{
				Enabled:       true,
				PollIntervals: tt.pollIntervals,
			}
			server.SetOnDemandConfig(cfg)

			got := server.GetOnDemandPollInterval(tt.layerType)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// TestNewClient_OnDemandTickersInitialized
// ---------------------------------------------------------------------------

func TestNewClient_OnDemandTickersInitialized(t *testing.T) {
	logger := zerolog.Nop()
	send := make(chan []byte, 256)

	client := realtime.NewClient("test-id", nil, send, logger)

	// The ticker map must be non-nil so that concurrent writes inside
	// startOnDemandTicker do not panic.
	assert.Equal(t, 0, client.OnDemandTickerCount(),
		"fresh client should have zero tickers but a non-nil map")
}

// ---------------------------------------------------------------------------
// TestOnDemandTicker_ZeroInterval
// ---------------------------------------------------------------------------

func TestOnDemandTicker_ZeroInterval(t *testing.T) {
	tests := []struct {
		name          string
		pollIntervals map[string]time.Duration
	}{
		{
			name:          "nil poll intervals map",
			pollIntervals: nil,
		},
		{
			name:          "layer not present in map",
			pollIntervals: map[string]time.Duration{"other_layer": 10 * time.Second},
		},
		{
			name:          "explicit zero duration",
			pollIntervals: map[string]time.Duration{"flights_commercial": 0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zerolog.Nop()
			spatialCache := &mockObsRepo{}
			server := realtime.NewServer(logger, nil, spatialCache, viewportRegistry("flights_commercial"))

			cfg := realtime.OnDemandConfig{
				Enabled:       true,
				QueryTimeout:  5 * time.Second,
				PollIntervals: tt.pollIntervals,
				LayerAPIs:     map[string]string{"flights_commercial": "http://example.com/{lat}/{lon}"},
			}
			server.SetOnDemandConfig(cfg)

			send := make(chan []byte, 256)
			client := realtime.NewClient("zero-interval-test", nil, send, logger)
			client.SetServer(server)

			bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
			client.StartOnDemandTicker("layer1", "flights_commercial", bbox)

			// A zero-interval start should not register any ticker.
			assert.Equal(t, 0, client.OnDemandTickerCount(),
				"zero interval must not start a ticker goroutine")
		})
	}
}

// ---------------------------------------------------------------------------
// TestOnDemandTicker_StartStop
// ---------------------------------------------------------------------------

func TestOnDemandTicker_StartStop(t *testing.T) {
	t.Run("start creates ticker entry", func(t *testing.T) {
		apiServer := testAPIServer(t, nil) // empty response is fine here
		defer apiServer.Close()

		sc := &mockObsRepo{}
		sc.setBBoxSnapshots(nil, nil, nil)
		_, client := newTickerTestServer(t, apiServer.URL, 500*time.Millisecond, sc)

		bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
		client.StartOnDemandTicker("layer1", "flights_commercial", bbox)

		assert.Equal(t, 1, client.OnDemandTickerCount())
		assert.True(t, client.HasOnDemandTicker("layer1"))
	})

	t.Run("stop removes ticker entry", func(t *testing.T) {
		apiServer := testAPIServer(t, nil)
		defer apiServer.Close()

		sc := &mockObsRepo{}
		sc.setBBoxSnapshots(nil, nil, nil)
		_, client := newTickerTestServer(t, apiServer.URL, 500*time.Millisecond, sc)

		bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
		client.StartOnDemandTicker("layer1", "flights_commercial", bbox)
		require.Equal(t, 1, client.OnDemandTickerCount())

		client.StopOnDemandTicker("layer1")
		assert.Equal(t, 0, client.OnDemandTickerCount())
		assert.False(t, client.HasOnDemandTicker("layer1"))
	})

	t.Run("stop non-existent ticker is a no-op", func(t *testing.T) {
		logger := zerolog.Nop()
		client := realtime.NewClient("noop-test", nil, make(chan []byte, 256), logger)

		// Should not panic
		client.StopOnDemandTicker("nonexistent-layer")
		assert.Equal(t, 0, client.OnDemandTickerCount())
	})

	t.Run("start replaces existing ticker", func(t *testing.T) {
		apiServer := testAPIServer(t, nil)
		defer apiServer.Close()

		sc := &mockObsRepo{}
		sc.setBBoxSnapshots(nil, nil, nil)
		_, client := newTickerTestServer(t, apiServer.URL, 500*time.Millisecond, sc)

		bbox1 := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
		bbox2 := &domain.BBox{West: -80.0, South: 35.0, East: -70.0, North: 45.0}

		client.StartOnDemandTicker("layer1", "flights_commercial", bbox1)
		require.Equal(t, 1, client.OnDemandTickerCount())

		// Starting again for the same layer must replace the old ticker (not add).
		client.StartOnDemandTicker("layer1", "flights_commercial", bbox2)
		assert.Equal(t, 1, client.OnDemandTickerCount(),
			"replacing a ticker must not add a second entry")
		assert.True(t, client.HasOnDemandTicker("layer1"))
	})

	t.Run("start multiple different layers", func(t *testing.T) {
		apiServer := testAPIServer(t, nil)
		defer apiServer.Close()

		sc := &mockObsRepo{}
		sc.setBBoxSnapshots(nil, nil, nil)

		logger := zerolog.Nop()
		server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial", "satellites"))
		cfg := realtime.OnDemandConfig{
			Enabled:      true,
			QueryTimeout: 5 * time.Second,
			LayerAPIs: map[string]string{
				"flights_commercial": apiServer.URL + "/v2/point/{lat}/{lon}",
				"satellites":         apiServer.URL + "/v2/sat/{lat}/{lon}",
			},
			PollIntervals: map[string]time.Duration{
				"flights_commercial": 500 * time.Millisecond,
				"satellites":         500 * time.Millisecond,
			},
		}
		server.SetOnDemandConfig(cfg)

		client := realtime.NewClient("multi-layer", nil, make(chan []byte, 256), logger)
		client.SetServer(server)

		bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
		client.StartOnDemandTicker("layerA", "flights_commercial", bbox)
		client.StartOnDemandTicker("layerB", "satellites", bbox)

		assert.Equal(t, 2, client.OnDemandTickerCount())
		assert.True(t, client.HasOnDemandTicker("layerA"))
		assert.True(t, client.HasOnDemandTicker("layerB"))
	})
}

// ---------------------------------------------------------------------------
// TestOnDemandTicker_StopAll
// ---------------------------------------------------------------------------

func TestOnDemandTicker_StopAll(t *testing.T) {
	t.Run("stopAll cancels all running tickers", func(t *testing.T) {
		apiServer := testAPIServer(t, nil)
		defer apiServer.Close()

		sc := &mockObsRepo{}
		sc.setBBoxSnapshots(nil, nil, nil)

		logger := zerolog.Nop()
		server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial", "satellites", "ships"))
		cfg := realtime.OnDemandConfig{
			Enabled:      true,
			QueryTimeout: 5 * time.Second,
			LayerAPIs: map[string]string{
				"flights_commercial": apiServer.URL + "/v2/point/{lat}/{lon}",
				"satellites":         apiServer.URL + "/v2/sat/{lat}/{lon}",
				"ships":              apiServer.URL + "/v2/ships/{lat}/{lon}",
			},
			PollIntervals: map[string]time.Duration{
				"flights_commercial": 500 * time.Millisecond,
				"satellites":         500 * time.Millisecond,
				"ships":              500 * time.Millisecond,
			},
		}
		server.SetOnDemandConfig(cfg)

		client := realtime.NewClient("stop-all-test", nil, make(chan []byte, 256), logger)
		client.SetServer(server)

		bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
		client.StartOnDemandTicker("layerA", "flights_commercial", bbox)
		client.StartOnDemandTicker("layerB", "satellites", bbox)
		client.StartOnDemandTicker("layerC", "ships", bbox)
		require.Equal(t, 3, client.OnDemandTickerCount())

		client.StopAllOnDemandTickers()

		assert.Equal(t, 0, client.OnDemandTickerCount(), "all tickers must be removed")
	})

	t.Run("stopAll on empty map is a no-op", func(t *testing.T) {
		logger := zerolog.Nop()
		client := realtime.NewClient("empty-stop-all", nil, make(chan []byte, 256), logger)

		// Should not panic
		client.StopAllOnDemandTickers()
		assert.Equal(t, 0, client.OnDemandTickerCount())
	})
}

// ---------------------------------------------------------------------------
// TestRunOnDemandPoll_ContextCancel
// ---------------------------------------------------------------------------

func TestRunOnDemandPoll_ContextCancel(t *testing.T) {
	// Start a ticker, verify the goroutine exits promptly after stop.
	apiServer := testAPIServer(t, nil)
	defer apiServer.Close()

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	_, client := newTickerTestServer(t, apiServer.URL, 200*time.Millisecond, sc)

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	client.StartOnDemandTicker("layer1", "flights_commercial", bbox)
	require.Equal(t, 1, client.OnDemandTickerCount())

	// Cancel by stopping the ticker — this cancels the internal context.
	client.StopOnDemandTicker("layer1")

	// After stop the map entry must be gone immediately.
	assert.Equal(t, 0, client.OnDemandTickerCount())
}

// ---------------------------------------------------------------------------
// TestRunOnDemandPoll_DeliversEntities
// ---------------------------------------------------------------------------

func TestRunOnDemandPoll_DeliversEntities(t *testing.T) {
	// The API server returns one aircraft. After at least one poll tick the
	// client's Send channel should contain a snapshot message.
	aircraft := []parse.ADSBLolAircraft{
		{Hex: "aa1111", Flight: "POLL01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000), GS: 450, Track: 90},
	}
	apiServer := testAPIServer(t, aircraft)
	defer apiServer.Close()

	sc := &mockObsRepo{}
	// BBox query returns empty (no pre-existing cache entities → not deduped out).
	sc.setBBoxSnapshots(nil, nil, nil)

	// Use a short poll interval so the test completes quickly.
	_, client := newTickerTestServer(t, apiServer.URL, 80*time.Millisecond, sc)

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	client.StartOnDemandTicker("adsb_lol_flights", "flights_commercial", bbox)
	defer client.StopAllOnDemandTickers()

	// Wait for at least one snapshot to arrive on the send channel.
	var receivedMsg []byte
	assert.Eventually(t, func() bool {
		select {
		case msg := <-client.Send:
			receivedMsg = msg
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond, "expected a snapshot message to arrive within 2s")

	require.NotNil(t, receivedMsg)

	// Decode and assert basic shape.
	var wsMsg realtime.WSMessage
	require.NoError(t, json.Unmarshal(receivedMsg, &wsMsg))
	assert.Equal(t, realtime.MsgTypeLayerSnapshot, wsMsg.Type)
	assert.Equal(t, "adsb_lol_flights", wsMsg.LayerID)
}

// ---------------------------------------------------------------------------
// TestDoOnDemandFetchDirect
// ---------------------------------------------------------------------------

func TestDoOnDemandFetchDirect(t *testing.T) {
	tests := []struct {
		name             string
		enabled          bool
		spatialCache     bool // whether to provide a spatialCache impl
		aircraft         []parse.ADSBLolAircraft
		bboxEntities     []*domain.Entity // pre-existing cache entities (for dedup)
		apiStatus        int              // 0 means use normal JSON handler
		wantMinEntities  int
		wantZeroEntities bool
	}{
		{
			name:             "disabled returns empty",
			enabled:          false,
			spatialCache:     true,
			aircraft:         []parse.ADSBLolAircraft{{Hex: "abc", Flight: "T01", Lat: 40, Lon: -74, AltBaro: float64(35000)}},
			wantZeroEntities: true,
		},
		{
			name:             "no spatial cache interface returns empty",
			enabled:          true,
			spatialCache:     false, // no observation repository wired
			aircraft:         []parse.ADSBLolAircraft{{Hex: "abc", Flight: "T01", Lat: 40, Lon: -74, AltBaro: float64(35000)}},
			wantZeroEntities: true,
		},
		{
			name:            "API returns data — entities delivered",
			enabled:         true,
			spatialCache:    true,
			aircraft:        []parse.ADSBLolAircraft{{Hex: "abc123", Flight: "TEST01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000), GS: 450, Track: 90}},
			wantMinEntities: 1,
		},
		{
			name:             "API returns empty list — zero entities",
			enabled:          true,
			spatialCache:     true,
			aircraft:         []parse.ADSBLolAircraft{},
			wantZeroEntities: true,
		},
		{
			name:         "dedup: existing entity in bbox cache excluded",
			enabled:      true,
			spatialCache: true,
			aircraft: []parse.ADSBLolAircraft{
				{Hex: "new111", Flight: "NEW01", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
				{Hex: "existing", Flight: "OLD01", Lat: 41.0, Lon: -73.0, AltBaro: float64(28000)},
			},
			bboxEntities: []*domain.Entity{
				{ID: "flights_commercial:existing", ExternalID: "existing", LayerType: "flights_commercial"},
			},
			wantMinEntities: 1, // only "new111" — "existing" is deduped
		},
		{
			name:             "API returns HTTP error — zero entities",
			enabled:          true,
			spatialCache:     true,
			aircraft:         nil,
			apiStatus:        http.StatusInternalServerError,
			wantZeroEntities: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build API server
			var apiServer *httptest.Server
			if tt.apiStatus != 0 {
				apiServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Error(w, "error", tt.apiStatus)
				}))
			} else {
				apiServer = testAPIServer(t, tt.aircraft)
			}
			defer apiServer.Close()

			logger := zerolog.Nop()

			var server *realtime.Server
			if tt.spatialCache {
				sc := &mockObsRepo{}
				sc.setBBoxSnapshots(tt.bboxEntities, nil, nil)
				server = realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
			} else {
				// No observation repository wired: the spatial query the
				// on-demand path depends on is unavailable.
				server = realtime.NewServer(logger, nil, nil, viewportRegistry("flights_commercial"))
			}

			cfg := realtime.OnDemandConfig{
				Enabled:      tt.enabled,
				QueryTimeout: 3 * time.Second,
				LayerAPIs: map[string]string{
					"flights_commercial": apiServer.URL + "/v2/point/{lat}/{lon}",
				},
			}
			server.SetOnDemandConfig(cfg)
			server.SetOnDemandParser(parse.ParseADSBLolResponse)

			bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
			entities, obs := server.DoOnDemandFetchDirect(
				context.Background(),
				"adsb_lol_flights", "flights_commercial",
				bbox,
			)

			if tt.wantZeroEntities {
				assert.Empty(t, entities)
				assert.Empty(t, obs)
			} else {
				assert.GreaterOrEqual(t, len(entities), tt.wantMinEntities)
				assert.Equal(t, len(entities), len(obs))
				for _, e := range entities {
					assert.Equal(t, "live", e.Source, "entities from direct fetch must be tagged live")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestDoOnDemandFetchDirect_DeduplicationDetails
// ---------------------------------------------------------------------------

// Validates that only entities NOT already in the bbox cache are returned.
func TestDoOnDemandFetchDirect_DeduplicationDetails(t *testing.T) {
	aircraft := []parse.ADSBLolAircraft{
		{Hex: "keep1", Flight: "K1", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
		{Hex: "skip1", Flight: "S1", Lat: 41.0, Lon: -73.0, AltBaro: float64(28000)},
		{Hex: "keep2", Flight: "K2", Lat: 42.0, Lon: -72.0, AltBaro: float64(30000)},
	}
	apiServer := testAPIServer(t, aircraft)
	defer apiServer.Close()

	sc := &mockObsRepo{}
	// "skip1" is already stored for this viewport, so the fetch must not
	// re-import it. On-demand dedup reads the same bbox query doBackfill uses.
	sc.setBackfillBBox([]*domain.EntitySnapshot{
		{Entity: domain.Entity{ID: "flights_commercial:skip1", ExternalID: "skip1", LayerType: "flights_commercial"}},
	})

	logger := zerolog.Nop()
	server := realtime.NewServer(logger, nil, sc, viewportRegistry("flights_commercial"))
	server.SetOnDemandConfig(realtime.OnDemandConfig{
		Enabled:      true,
		QueryTimeout: 3 * time.Second,
		LayerAPIs:    map[string]string{"flights_commercial": apiServer.URL + "/v2/point/{lat}/{lon}"},
	})
	server.SetOnDemandParser(parse.ParseADSBLolResponse)

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	entities, _ := server.DoOnDemandFetchDirect(context.Background(), "adsb_lol_flights", "flights_commercial", bbox)

	// Expect "keep1" and "keep2" but not "skip1".
	externalIDs := make(map[string]bool, len(entities))
	for _, e := range entities {
		externalIDs[e.ExternalID] = true
	}

	assert.True(t, externalIDs["keep1"], "keep1 should be present")
	assert.True(t, externalIDs["keep2"], "keep2 should be present")
	assert.False(t, externalIDs["skip1"], "skip1 must be deduped out")
	assert.Equal(t, 2, len(entities))
}

// ---------------------------------------------------------------------------
// Concurrent ticker operations (race detector coverage)
// ---------------------------------------------------------------------------

func TestOnDemandTicker_ConcurrentStartStop(t *testing.T) {
	apiServer := testAPIServer(t, nil)
	defer apiServer.Close()

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	_, client := newTickerTestServer(t, apiServer.URL, 500*time.Millisecond, sc)

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	const goroutines = 8
	const iterations = 5

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range iterations {
				client.StartOnDemandTicker("layer1", "flights_commercial", bbox)
				client.StopOnDemandTicker("layer1")
			}
		}(g)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Pass — no race or deadlock detected.
	case <-time.After(5 * time.Second):
		t.Fatal("concurrent start/stop operations timed out")
	}
}

func TestOnDemandTicker_StopAll_WhileRunning(t *testing.T) {
	aircraft := []parse.ADSBLolAircraft{
		{Hex: "abc", Flight: "T1", Lat: 40.0, Lon: -74.0, AltBaro: float64(35000)},
	}
	apiServer := testAPIServer(t, aircraft)
	defer apiServer.Close()

	sc := &mockObsRepo{}
	sc.setBBoxSnapshots(nil, nil, nil)
	_, client := newTickerTestServer(t, apiServer.URL, 100*time.Millisecond, sc)

	bbox := &domain.BBox{West: -75.0, South: 39.0, East: -72.0, North: 42.0}
	client.StartOnDemandTicker("layerA", "flights_commercial", bbox)
	client.StartOnDemandTicker("layerB", "flights_commercial", bbox)

	// Give goroutines a moment to start.
	time.Sleep(50 * time.Millisecond)

	// StopAll must not deadlock or race.
	done := make(chan struct{})
	go func() {
		client.StopAllOnDemandTickers()
		close(done)
	}()

	select {
	case <-done:
		assert.Equal(t, 0, client.OnDemandTickerCount())
	case <-time.After(2 * time.Second):
		t.Fatal("StopAllOnDemandTickers timed out")
	}
}

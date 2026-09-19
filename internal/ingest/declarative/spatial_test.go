package declarative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest/grid"
	"github.com/Alevsk/respondent/internal/logging"
)

func TestSpatialGrid_Coverage(t *testing.T) {
	regions := grid.GenerateGlobalGrid(250)

	if len(regions) < 800 || len(regions) > 1050 {
		t.Errorf("expected 800-1050 grid regions for 250 NM radius, got %d", len(regions))
	}

	// Verify all latitude bands are covered
	bandCount := make(map[int]int)
	for _, r := range regions {
		// Group into 30-degree latitude bands
		band := int(r.Lat/30.0) * 30
		bandCount[band]++
	}

	// Should have points in at least 5 latitude bands
	if len(bandCount) < 5 {
		t.Errorf("expected at least 5 latitude bands, got %d", len(bandCount))
	}

	// Equatorial bands should have more points than polar
	equatorial := bandCount[0]
	polar := bandCount[-90] + bandCount[60]
	if equatorial <= polar && equatorial > 0 && polar > 0 {
		t.Errorf("equatorial band (%d) should have more points than polar bands (%d)", equatorial, polar)
	}
}

func TestSpatialBatch_RoundRobin(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}

	regions := []grid.Region{
		{Lat: 1, Lon: 1, Label: "R1"},
		{Lat: 2, Lon: 2, Label: "R2"},
		{Lat: 3, Lon: 3, Label: "R3"},
		{Lat: 4, Lon: 4, Label: "R4"},
		{Lat: 5, Lon: 5, Label: "R5"},
	}

	adapter.spatial = &spatialState{regions: regions, cursor: 0}
	adapter.spatialBatchSize = 2

	tests := []struct {
		name string
		want []string
	}{
		{"cycle_0", []string{"R1", "R2"}},
		{"cycle_1", []string{"R3", "R4"}},
		{"cycle_2", []string{"R5", "R1"}}, // wraps around
		{"cycle_3", []string{"R2", "R3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adapter.spatialMu.Lock()
			batch := adapter.selectSpatialBatch()
			adapter.spatialMu.Unlock()

			if len(batch) != len(tt.want) {
				t.Fatalf("expected %d regions, got %d", len(tt.want), len(batch))
			}
			for i, label := range tt.want {
				if batch[i].Label != label {
					t.Errorf("position %d: expected %s, got %s", i, label, batch[i].Label)
				}
			}
		})
	}
}

func TestEntityCache_TTLEviction(t *testing.T) {
	cache := newEntityCache(100*time.Millisecond, nil)

	cache.merge("key1", testEntity("e1"), testObservation("e1"))
	cache.merge("key2", testEntity("e2"), testObservation("e2"))

	// Verify both entries exist
	entities, _ := cache.snapshot()
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}

	// Wait for TTL expiry
	time.Sleep(150 * time.Millisecond)

	// Entries should be evicted
	entities, _ = cache.snapshot()
	if len(entities) != 0 {
		t.Errorf("expected 0 entities after TTL, got %d", len(entities))
	}
}

func TestEntityCache_Accumulate(t *testing.T) {
	cache := newEntityCache(5*time.Second, nil)

	// Simulate batch 1: region A
	cache.merge("hex1", testEntity("hex1"), testObservation("hex1"))
	cache.merge("hex2", testEntity("hex2"), testObservation("hex2"))

	// Simulate batch 2: region B (different aircraft + one overlap)
	cache.merge("hex3", testEntity("hex3"), testObservation("hex3"))
	cache.merge("hex1", testEntity("hex1_updated"), testObservation("hex1_updated"))

	entities, obs := cache.snapshot()

	// Should have 3 unique entities (hex1 was updated, not duplicated)
	if len(entities) != 3 {
		t.Errorf("expected 3 accumulated entities, got %d", len(entities))
	}
	if len(obs) != 3 {
		t.Errorf("expected 3 accumulated observations, got %d", len(obs))
	}

	// Verify hex1 was updated
	found := false
	for _, e := range entities {
		if e.ExternalID == "hex1_updated" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected hex1 to be updated to hex1_updated")
	}
}

func TestEntityCache_KeyExtraction(t *testing.T) {
	cache := newEntityCache(5*time.Second, nil)

	// Different entities with different keys
	cache.merge("abc123", testEntity("abc123"), testObservation("abc123"))
	cache.merge("def456", testEntity("def456"), testObservation("def456"))

	if cache.size() != 2 {
		t.Errorf("expected 2 entries, got %d", cache.size())
	}

	// Overwrite by same key
	cache.merge("abc123", testEntity("abc123_v2"), testObservation("abc123_v2"))

	if cache.size() != 2 {
		t.Errorf("expected 2 entries after overwrite, got %d", cache.size())
	}
}

func TestURLTemplate_Substitution(t *testing.T) {
	tests := []struct {
		name     string
		template string
		lat      float64
		lon      float64
		radius   float64
		wantLat  string
		wantLon  string
	}{
		{
			name:     "basic lat/lon",
			template: "https://api.example.com/v2/lat/{lat}/lon/{lon}/dist/250",
			lat:      40.6,
			lon:      -73.8,
			radius:   250,
			wantLat:  "40.6000",
			wantLon:  "-73.8000",
		},
		{
			name:     "negative coordinates",
			template: "https://api.example.com/v2/lat/{lat}/lon/{lon}/dist/250",
			lat:      -33.9,
			lon:      151.2,
			radius:   250,
			wantLat:  "-33.9000",
			wantLon:  "151.2000",
		},
		{
			name:     "zero coordinates",
			template: "https://api.example.com/v2/lat/{lat}/lon/{lon}/dist/250",
			lat:      0,
			lon:      0,
			radius:   250,
			wantLat:  "0.0000",
			wantLon:  "0.0000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteURLTemplate(tt.template, tt.lat, tt.lon, tt.radius)

			if result == tt.template {
				t.Error("URL template was not modified")
			}

			// Verify lat and lon were substituted
			if !contains(result, tt.wantLat) {
				t.Errorf("result %q does not contain expected lat %q", result, tt.wantLat)
			}
			if !contains(result, tt.wantLon) {
				t.Errorf("result %q does not contain expected lon %q", result, tt.wantLon)
			}

			// Verify no template markers remain
			if contains(result, "{lat}") || contains(result, "{lon}") {
				t.Errorf("result still contains template markers: %q", result)
			}
		})
	}
}

func TestURLTemplate_BBox(t *testing.T) {
	template := "https://api.example.com/bbox?south={lat_min}&north={lat_max}&west={lon_min}&east={lon_max}"
	result := SubstituteURLTemplate(template, 40.0, -74.0, 250)

	// Verify all bbox markers are replaced
	markers := []string{"{lat_min}", "{lat_max}", "{lon_min}", "{lon_max}"}
	for _, m := range markers {
		if contains(result, m) {
			t.Errorf("result still contains template marker %q: %q", m, result)
		}
	}

	// Verify the URL is reasonable (lat_min < lat < lat_max)
	if contains(result, "south=40.0000") {
		t.Error("lat_min should be less than center lat")
	}
}

func TestNonSpatialSources_Unchanged(t *testing.T) {
	// Verify that a non-spatial source (schema_version: 1) still works
	response := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{
				"id":   "test1",
				"name": "Test 1",
				"lat":  10.0,
				"lon":  20.0,
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_nonspatial
source_type: test_nonspatial
layer_type: test_layer
display_name: "Test Non-Spatial"
transport:
  type: http_poll
  url: "` + server.URL + `"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "items"
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "nonspatial.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlTmpl), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if len(entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(entities))
	}
}

func TestSpatialAdapter_Integration(t *testing.T) {
	// Mock server that returns adsb.lol-like JSON
	response := map[string]interface{}{
		"ac": []interface{}{
			map[string]interface{}{
				"hex":      "abc123",
				"flight":   "TEST01",
				"lat":      40.0,
				"lon":      -74.0,
				"alt_baro": 35000.0,
				"gs":       450.0,
				"track":    90.0,
			},
			map[string]interface{}{
				"hex":      "def456",
				"flight":   "TEST02",
				"lat":      41.0,
				"lon":      -73.0,
				"alt_baro": 28000.0,
				"gs":       400.0,
				"track":    180.0,
			},
		},
		"total": 2,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlContent := `schema_version: 2
name: test_spatial
source_type: test_spatial
layer_type: test_flights
display_name: "Test Spatial"
filtering: viewport
transport:
  type: http_poll
  url: "` + server.URL + `/v2/lat/{lat}/lon/{lon}/dist/250"
  timeout: "10s"
  interval: "30s"
  spatial:
    type: static_regions
    regions:
      - lat: 40.0
        lon: -74.0
        label: "NYC"
      - lat: 41.0
        lon: -73.0
        label: "CT"
parser:
  format: json
  records_path: "ac"
filter: 'has(record.hex) && has(record.lat) && has(record.lon)'
entity:
  external_id: 'record.hex'
  name: 'has(record.flight) ? string(record.flight) : record.hex'
  metadata:
    hex: 'record.hex'
observation:
  latitude: 'double(record.lat)'
  longitude: 'double(record.lon)'
  altitude: 'has(record.alt_baro) ? double(record.alt_baro) * 0.3048 : 0.0'
  timestamp: 'now()'
  velocity:
    speed: 'has(record.gs) ? string(double(record.gs) * 0.514444) : "0"'
    heading: 'has(record.track) ? string(record.track) : "0"'
entity_cache:
  enabled: true
  key: "record.hex"
  ttl: "300s"
  accumulate: true
recording:
  mode: append
cache:
  ttl: "300s"
display:
  icon:
    shape: flight
    rotatable: true
    interpolation: true
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 8
`

	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "spatial.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Both regions return the same 2 aircraft, but entity cache deduplicates
	if len(entities) != 2 {
		t.Errorf("expected 2 deduplicated entities, got %d", len(entities))
	}
	if len(observations) != 2 {
		t.Errorf("expected 2 deduplicated observations, got %d", len(observations))
	}

	// Verify entity properties
	for _, e := range entities {
		if e.LayerType != "test_flights" {
			t.Errorf("expected layer_type test_flights, got %s", e.LayerType)
		}
		if e.Metadata["hex"] != e.ExternalID {
			t.Errorf("expected hex metadata = external_id, got hex=%q ext_id=%q", e.Metadata["hex"], e.ExternalID)
		}
	}
}

func TestSchemaV2_Validation(t *testing.T) {
	// Verify schema_version: 2 is accepted
	yamlContent := `schema_version: 2
name: test_v2
source_type: test_v2
layer_type: test_layer
display_name: "Test V2"
filtering: viewport
backfill:
  threshold: 10
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "30s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "v2.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	def := cs.Definition()
	if def.Filtering != "viewport" {
		t.Errorf("expected filtering=viewport, got %q", def.Filtering)
	}
	if def.Backfill == nil || def.Backfill.Threshold != 10 {
		t.Error("expected backfill.threshold=10")
	}
}

func TestCoerceDouble_CELFunction(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		record  map[string]interface{}
		wantVal float64
	}{
		{
			name:    "numeric value",
			expr:    "coerce_double(record.alt, 0.0)",
			record:  map[string]interface{}{"alt": 35000.0},
			wantVal: 35000.0,
		},
		{
			name:    "string ground",
			expr:    "coerce_double(record.alt, 0.0)",
			record:  map[string]interface{}{"alt": "ground"},
			wantVal: 0.0,
		},
		{
			name:    "string number",
			expr:    "coerce_double(record.alt, 0.0)",
			record:  map[string]interface{}{"alt": "12345"},
			wantVal: 12345.0,
		},
		{
			name:    "integer value",
			expr:    "coerce_double(record.alt, 0.0)",
			record:  map[string]interface{}{"alt": int64(28000)},
			wantVal: 28000.0,
		},
		{
			name:    "custom default",
			expr:    "coerce_double(record.alt, -1.0)",
			record:  map[string]interface{}{"alt": "invalid"},
			wantVal: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := compiler.CompileExpression(tt.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			activation := map[string]interface{}{"record": tt.record}
			result, err := evalFloat(prg, activation)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			if result != tt.wantVal {
				t.Errorf("expected %f, got %f", tt.wantVal, result)
			}
		})
	}
}

// Helper functions for tests

func testEntity(id string) *domain.Entity {
	return &domain.Entity{
		ID:         "test:" + id,
		ExternalID: id,
		LayerType:  "test",
		Name:       id,
		Metadata:   map[string]string{},
	}
}

func testObservation(entityID string) *domain.Observation {
	return &domain.Observation{
		ID:        "obs_" + entityID,
		EntityID:  "test:" + entityID,
		Timestamp: time.Now(),
		Position:  &domain.GeoPoint{Lat: 40.0, Lon: -74.0},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

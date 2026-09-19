package declarative

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/ingest/grid"
	"github.com/Alevsk/respondent/internal/logging"
)

func TestSubstituteURLTemplate_PolarLatitude(t *testing.T) {
	// At lat=90 (north pole), cos(90°) = 0, so lonOffset would be Inf/NaN.
	// SubstituteURLTemplate should fall back to 360 for lonOffset.
	result := SubstituteURLTemplate("lat={lat}&lon={lon}", 90.0, 0.0, 100.0)
	if result == "" {
		t.Error("expected non-empty result for polar latitude")
	}
}

func TestFetchSpatial_NilCompiledSource(t *testing.T) {
	a := &DeclarativeAdapter{
		logger:    logging.NewNopLogger(),
		layerType: "test_nil_cs",
		name:      "test_nil_cs",
	}
	// Leave compiled as nil (zero value)
	err := a.fetchSpatial(context.Background())
	if err == nil {
		t.Error("expected error for nil compiled source, got nil")
	}
}

func TestFetchSpatial_EmptyBatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	yaml := `schema_version: 2
name: spatial_empty_batch
source_type: spatial_empty_batch
layer_type: spatial_empty_batch_layer
display_name: "Spatial Empty Batch"
transport:
  type: http_poll
  url: "` + srv.URL + `/{lat}/{lon}"
  method: GET
  timeout: "10s"
  interval: "10s"
  spatial:
    type: static_regions
    regions: []
    batch_size: 10
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
	yamlPath := filepath.Join(dir, "spatial_empty_batch.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	adapter, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	err = adapter.fetchSpatial(context.Background())
	if err != nil {
		t.Errorf("fetchSpatial with empty regions: expected nil, got %v", err)
	}
}

func TestFetchSpatial_FetchURLError(t *testing.T) {
	name := "spatial_fetch_err"
	dir := t.TempDir()
	// Use an invalid URL that will fail fetch.
	yamlContent := fmt.Sprintf(`schema_version: 2
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Spatial Fetch Error"
transport:
  type: http_poll
  url: "https://127.0.0.1:19999/{lat}"
  method: GET
  timeout: "1s"
  interval: "60s"
  spatial:
    type: static_regions
    regions:
      - lat: 10.0
        lon: 20.0
        label: "test_region"
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
    color: "#00ff9d"
  style:
    color: "#00ff9d"
    point_size: 6
`, name, name, name)

	yamlPath := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	// fetchSpatial should log a warning and continue when fetch fails.
	err = adapter.fetchSpatial(context.Background())
	// No error returned; the region fetch error is logged as warning, not propagated.
	_ = err
}

func TestFetchSpatial_ParseBodyError(t *testing.T) {
	// Server that returns invalid JSON.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`not-valid-json{}`))
	}))
	defer srv.Close()

	name := "spatial_parse_err"
	dir := t.TempDir()
	yamlContent := fmt.Sprintf(`schema_version: 2
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Spatial Parse Error"
transport:
  type: http_poll
  url: "%s/{lat}"
  method: GET
  timeout: "10s"
  interval: "60s"
  spatial:
    type: static_regions
    regions:
      - lat: 10.0
        lon: 20.0
        label: "test_region"
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
    color: "#00ff9d"
  style:
    color: "#00ff9d"
    point_size: 6
`, name, name, name, srv.URL)

	yamlPath := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	err = adapter.fetchSpatial(context.Background())
	_ = err // Parse error is logged as warning, not propagated.
}

func TestSelectSpatialBatch_PriorityCapBreak(t *testing.T) {
	// This test ensures round-robin batch selection works with a small batch.
	adapter := &DeclarativeAdapter{
		logger:           logging.NewNopLogger(),
		layerType:        "priority_cap_break",
		spatialBatchSize: 2,
	}
	adapter.spatial = &spatialState{
		regions: []grid.Region{
			{Lat: 1.0, Lon: 1.0},
			{Lat: 2.0, Lon: 2.0},
		},
		cursor: 0,
	}

	batch := adapter.selectSpatialBatch()
	if len(batch) != 2 {
		t.Errorf("expected 2 regions, got %d", len(batch))
	}
}

func TestSubstituteURLTemplate_NormalLat(t *testing.T) {
	result := SubstituteURLTemplate(
		"https://api.example.com/{lat}/{lon}",
		45.0, 90.0, 100.0,
	)
	if strings.Contains(result, "{lat}") || strings.Contains(result, "{lon}") {
		t.Errorf("expected lat/lon substitutions, got %q", result)
	}
}

func TestSelectSpatialBatch_Phase2ContinueSeen(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger:           logging.NewNopLogger(),
		layerType:        "phase2_seen_test",
		spatialBatchSize: 2,
	}

	// Three regions: [0], [1], [2].
	adapter.spatial = &spatialState{
		regions: []grid.Region{
			{Lat: 10.0, Lon: 20.0},
			{Lat: 30.0, Lon: 40.0},
			{Lat: 50.0, Lon: 60.0},
		},
		cursor: 0,
	}

	// batchSize=2, n=3. Phase 2 selects 2 of the 3 regions.
	batch := adapter.selectSpatialBatch()
	if len(batch) != 2 {
		t.Errorf("expected 2 regions (batchSize=2), got %d", len(batch))
	}
}

func TestSelectSpatialBatch_Phase2Continue_AllSeen(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger:           logging.NewNopLogger(),
		layerType:        "phase2_allseen_test",
		spatialBatchSize: 3, // request 3, but only 2 regions exist
	}

	adapter.spatial = &spatialState{
		regions: []grid.Region{
			{Lat: 10.0, Lon: 20.0},
			{Lat: 30.0, Lon: 40.0},
		},
		cursor: 0,
	}

	batch := adapter.selectSpatialBatch()
	// batchSize=3 but n=2, so count=min(3,2)=2.
	if len(batch) != 2 {
		t.Errorf("expected 2 regions (min of batchSize and n), got %d", len(batch))
	}
}

func TestSubstituteURLTemplate_PoleCase(t *testing.T) {
	// Use math.NaN() as lat to force lonOffset = NaN (the IsNaN branch).
	nanLat := math.NaN()

	result := SubstituteURLTemplate("{lon_min},{lon_max}", nanLat, 0.0, 100.0)
	// With lonOffset capped to 360: lon_min = 0-360 = -360.0000, lon_max = 0+360 = 360.0000
	if !strings.Contains(result, "-360.0000") {
		t.Errorf("expected -360.0000 in pole/NaN case result, got: %s", result)
	}
	if !strings.Contains(result, "360.0000") {
		t.Errorf("expected 360.0000 in pole/NaN case result, got: %s", result)
	}
}

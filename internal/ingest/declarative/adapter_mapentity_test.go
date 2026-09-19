package declarative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// TestDeclarativeAdapter_SupportsSourceType_NilCS verifies that a nil compiled source
// returns false from SupportsSourceType.
func TestDeclarativeAdapter_SupportsSourceType_NilCS(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	// compiled is an atomic.Pointer[CompiledSource]; zero value is nil.
	result := adapter.SupportsSourceType(domain.SourceType("any_type"))
	if result {
		t.Error("expected false for nil compiled source, got true")
	}
}

// TestDeclarativeAdapter_MapEntityWithFieldMappings_Entity verifies that entity.* field
// mappings are applied to the entity metadata.
func TestDeclarativeAdapter_MapEntityWithFieldMappings_Entity(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":       "fm1",
					"name":     "FieldMap Entity 1",
					"lat":      10.0,
					"lon":      20.0,
					"category": "type_A",
					"count":    float64(42),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	name := "fieldmap_entity_test"
	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: fieldmap_entity_test
source_type: fieldmap_entity_test
layer_type: fieldmap_entity_layer
display_name: "FieldMap Entity Test"
transport:
  type: http_poll
  url: "` + srv.URL + `"
  method: GET
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
field_mappings:
  - source: 'record.category'
    target: 'entity.category'
    type: 'string'
  - source: 'record.count'
    target: 'entity.count'
    type: 'integer'
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
	yamlPath := filepath.Join(dir, name+".yaml")
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

	entities, _, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}

	e := entities[0]
	if e.Metadata["category"] != "type_A" {
		t.Errorf("expected metadata category='type_A', got %q", e.Metadata["category"])
	}
	if e.Metadata["count"] != "42" {
		t.Errorf("expected metadata count='42', got %q", e.Metadata["count"])
	}
}

// TestDeclarativeAdapter_MapObservation_ObservationFieldMapping verifies that
// observation.* field mappings are applied to observation metadata.
func TestDeclarativeAdapter_MapObservation_ObservationFieldMapping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":       "obs1",
					"name":     "Obs Entity 1",
					"lat":      10.0,
					"lon":      20.0,
					"severity": "high",
					"duration": 3.5,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: obs_fieldmap_test
source_type: obs_fieldmap_test
layer_type: obs_fieldmap_layer
display_name: "Obs FieldMap Test"
transport:
  type: http_poll
  url: "` + srv.URL + `"
  method: GET
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
field_mappings:
  - source: 'record.severity'
    target: 'observation.severity'
    type: 'string'
  - source: 'record.duration'
    target: 'observation.duration'
    type: 'float'
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
	yamlPath := filepath.Join(dir, "obs_fieldmap.yaml")
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

	_, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}

	obs := observations[0]
	if obs.Metadata["severity"] != "high" {
		t.Errorf("expected observation metadata severity='high', got %q", obs.Metadata["severity"])
	}
	if obs.Metadata["duration"] != "3.5" {
		t.Errorf("expected observation metadata duration='3.5', got %q", obs.Metadata["duration"])
	}
}

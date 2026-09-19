package declarative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

// TestDeclarativeAdapter_IndicatorEntityType verifies that indicator entities
// have no geographic position (lat/lon are not evaluated).
func TestDeclarativeAdapter_IndicatorEntityType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":    "kp_global",
					"name":  "Kp Index",
					"value": "3.5",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: indicator_entity_test
source_type: indicator_entity_test
layer_type: indicator_entity_layer
display_name: "Indicator Entity Test"
entity_type: global_indicator
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
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: gauge
    scale: 1.0
  trail:
    color: "#ff0000"
  style:
    color: "#ff0000"
    point_size: 6
`
	yamlPath := filepath.Join(dir, "indicator_entity.yaml")
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
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(entities))
	}
	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}

	// Indicator observation should have no position (nil Position field).
	obs := observations[0]
	if obs.Position != nil {
		t.Errorf("expected nil Position for indicator entity, got %v", obs.Position)
	}
}

// TestDeclarativeAdapter_ContentHash verifies that content_hash is evaluated and stored.
func TestDeclarativeAdapter_ContentHash(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":       "hash_e1",
					"name":     "Hash Entity",
					"lat":      10.0,
					"lon":      20.0,
					"checksum": "abc123",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: content_hash_test
source_type: content_hash_test
layer_type: content_hash_layer
display_name: "Content Hash Test"
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
  content_hash: 'record.checksum'
recording:
  mode: upsert
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
	yamlPath := filepath.Join(dir, "content_hash.yaml")
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
	if obs.ContentHash != "abc123" {
		t.Errorf("expected ContentHash='abc123', got %q", obs.ContentHash)
	}
}

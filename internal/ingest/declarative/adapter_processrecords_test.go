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

// TestDeclarativeAdapter_ProcessRecords_DuplicateRecords verifies that duplicate
// records (same entity ID) are deduplicated with the last record winning.
func TestDeclarativeAdapter_ProcessRecords_DuplicateRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":   "dup1",
					"name": "First Occurrence",
					"lat":  10.0,
					"lon":  20.0,
				},
				map[string]interface{}{
					"id":   "dup1", // Same ID - duplicate
					"name": "Second Occurrence",
					"lat":  11.0,
					"lon":  21.0,
				},
				map[string]interface{}{
					"id":   "unique1",
					"name": "Unique Entity",
					"lat":  30.0,
					"lon":  40.0,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: dedup_test
source_type: dedup_test
layer_type: dedup_layer
display_name: "Dedup Test"
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
	yamlPath := filepath.Join(dir, "dedup_test.yaml")
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

	// 3 records with 1 duplicate -> 2 unique entities, 3 observations (all kept)
	if len(entities) != 2 {
		t.Fatalf("expected 2 deduplicated entities, got %d", len(entities))
	}
	if len(observations) != 3 {
		t.Fatalf("expected 3 observations (all records kept), got %d", len(observations))
	}

	// Find the deduplicated entity - should have the second occurrence's name.
	for _, e := range entities {
		if e.ExternalID == "dup1" && e.Name != "Second Occurrence" {
			t.Errorf("expected last occurrence to win: got name %q, want 'Second Occurrence'", e.Name)
		}
	}
}

// TestDeclarativeAdapter_ProcessRecords_FilterErrors verifies records with
// filter eval errors are skipped.
func TestDeclarativeAdapter_ProcessRecords_FilterErrors(t *testing.T) {
	// Server returns mixed valid and invalid records.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				// Valid record passes filter.
				map[string]interface{}{
					"id":   "valid1",
					"name": "Valid Entity",
					"lat":  10.0,
					"lon":  20.0,
					"mag":  3.5,
				},
				// Invalid record: filter tries to compare non-numeric to float - should be skipped.
				map[string]interface{}{
					"id":   "invalid1",
					"name": "Invalid Entity",
					"lat":  10.0,
					"lon":  20.0,
					"mag":  "not-a-number",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: filter_error_test
source_type: filter_error_test
layer_type: filter_error_layer
display_name: "Filter Error Test"
transport:
  type: http_poll
  url: "` + srv.URL + `"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "items"
filter: 'double(record.mag) >= 2.0'
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
	yamlPath := filepath.Join(dir, "filter_error_test.yaml")
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

	// Only the valid record should pass (the invalid one causes a filter eval error and is skipped).
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity (invalid record skipped), got %d", len(entities))
	}
	if entities[0].ExternalID != "valid1" {
		t.Errorf("expected valid1, got %q", entities[0].ExternalID)
	}
}

// TestDeclarativeAdapter_MapEntity_EmptyExternalID verifies records with empty
// external_id are skipped.
func TestDeclarativeAdapter_MapEntity_EmptyExternalID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"items": []interface{}{
				// This record has an empty id - should be skipped.
				map[string]interface{}{
					"id":   "",
					"name": "Empty ID",
					"lat":  10.0,
					"lon":  20.0,
				},
				// Valid record.
				map[string]interface{}{
					"id":   "valid_id",
					"name": "Valid",
					"lat":  10.0,
					"lon":  20.0,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: empty_id_test
source_type: empty_id_test
layer_type: empty_id_layer
display_name: "Empty ID Test"
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
	yamlPath := filepath.Join(dir, "empty_id_test.yaml")
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

	// Only the valid record should be returned (empty ID is skipped).
	if len(entities) != 1 {
		t.Fatalf("expected 1 entity (empty ID skipped), got %d", len(entities))
	}
	if entities[0].ExternalID != "valid_id" {
		t.Errorf("expected valid_id, got %q", entities[0].ExternalID)
	}
}

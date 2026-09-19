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

func TestDeclarativeAdapter_Mappings_StandardFields(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":          "1",
					"name":        "Test 1",
					"lat":         10.0,
					"lon":         20.0,
					"ts":          "2025-01-01T00:00:00Z",
					"evt_time":    float64(1700000000), // Valid UNIX timestamp
					"evt_end":     float64(1700000100), // Valid UNIX timestamp
					"entity_meta": "entity_value_1",
					"obs_meta":    "obs_value_1",
				},
				map[string]interface{}{
					"id":          "2",
					"name":        "Test 2",
					"lat":         15.0,
					"lon":         25.0,
					"ts":          "2025-01-01T01:00:00Z",
					"evt_time":    "invalid_type", // Will cause CEL evaluation error for unix_s
					"evt_end":     nil,            // Missing, should be gracefully handled
					"entity_meta": "entity_value_2",
					"obs_meta":    "obs_value_2",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_mappings
source_type: test_mappings
layer_type: test_layer
display_name: "Test Mappings"
transport:
  type: http_poll
  url: "%s"
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
  timestamp: 'record.ts'
  event_time: 'unix_s(record.evt_time)'
  event_end: 'unix_s(record.evt_end)'
field_mappings:
  - source: 'record.entity_meta'
    target: 'entity.meta_val'
    type: 'string'
  - source: 'record.obs_meta'
    target: 'observation.meta_val'
    type: 'string'
recording:
  mode: append
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ff0000"
  style:
    color: "#ffffff"
    point_size: 5
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlPath, []byte(replaceURL(yamlTmpl, server.URL)), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)
	ctx := context.Background()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("adapter.Snapshot: %v", err)
	}

	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(observations))
	}

	// Check entity 1
	e1 := entities[0]
	if e1.ExternalID != "1" {
		t.Errorf("expected entity 1 external_id '1', got %q", e1.ExternalID)
	}
	if e1.Metadata["meta_val"] != "entity_value_1" {
		t.Errorf("expected entity 1 metadata meta_val 'entity_value_1', got %q", e1.Metadata["meta_val"])
	}

	// Check observation 1
	o1 := observations[0]
	if o1.Metadata["meta_val"] != "obs_value_1" {
		t.Errorf("expected obs 1 metadata meta_val 'obs_value_1', got %q", o1.Metadata["meta_val"])
	}
	if o1.EventTime == nil {
		t.Error("expected obs 1 EventTime to be non-nil")
	} else if o1.EventTime.Unix() != 1700000000 {
		t.Errorf("expected obs 1 EventTime unix 1700000000, got %v", o1.EventTime.Unix())
	}
	if o1.EventEnd == nil {
		t.Error("expected obs 1 EventEnd to be non-nil")
	} else if o1.EventEnd.Unix() != 1700000100 {
		t.Errorf("expected obs 1 EventEnd unix 1700000100, got %v", o1.EventEnd.Unix())
	}

	// Check entity 2
	e2 := entities[1]
	if e2.ExternalID != "2" {
		t.Errorf("expected entity 2 external_id '2', got %q", e2.ExternalID)
	}
	if e2.Metadata["meta_val"] != "entity_value_2" {
		t.Errorf("expected entity 2 metadata meta_val 'entity_value_2', got %q", e2.Metadata["meta_val"])
	}

	// Check observation 2 (invalid event_time CEL falls back to observation timestamp)
	o2 := observations[1]
	if o2.Metadata["meta_val"] != "obs_value_2" {
		t.Errorf("expected obs 2 metadata meta_val 'obs_value_2', got %q", o2.Metadata["meta_val"])
	}
	if o2.EventTime == nil {
		t.Error("expected obs 2 EventTime to be non-nil (defaulted to observation timestamp)")
	} else if !o2.EventTime.Equal(o2.Timestamp) {
		t.Errorf("expected obs 2 EventTime to equal observation timestamp %v, got %v", o2.Timestamp, *o2.EventTime)
	}
	if o2.EventEnd != nil {
		t.Errorf("expected obs 2 EventEnd to be nil (due to missing field), got %v", o2.EventEnd)
	}
}

func TestDeclarativeAdapter_Mappings_FieldMappingsOverrides(t *testing.T) {
	// Setup mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":      "1",
					"name":    "Test 1",
					"lat":     10.0,
					"lon":     20.0,
					"ts":      "2025-01-01T00:00:00Z",
					"fm_time": float64(1700000000),
					"fm_end":  float64(1700000100),
				},
				map[string]interface{}{
					"id":      "2",
					"name":    "Test 2",
					"lat":     15.0,
					"lon":     25.0,
					"ts":      "2025-01-01T01:00:00Z",
					"fm_time": "invalid",
					"fm_end":  nil,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_fm_overrides
source_type: test_fm_overrides
layer_type: test_layer
display_name: "Test FM Overrides"
transport:
  type: http_poll
  url: "%s"
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
  timestamp: 'record.ts'
field_mappings:
  - source: 'unix_s(record.fm_time)'
    target: 'observation.event_time'
  - source: 'unix_s(record.fm_end)'
    target: 'observation.event_end'
recording:
  mode: append
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ff0000"
  style:
    color: "#ffffff"
    point_size: 5
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlPath, []byte(replaceURL(yamlTmpl, server.URL)), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)
	ctx := context.Background()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	_, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("adapter.Snapshot: %v", err)
	}

	if len(observations) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(observations))
	}

	// Check observation 1
	o1 := observations[0]
	if o1.EventTime == nil {
		t.Error("expected obs 1 EventTime to be non-nil")
	} else if o1.EventTime.Unix() != 1700000000 {
		t.Errorf("expected obs 1 EventTime unix 1700000000, got %v", o1.EventTime.Unix())
	}
	if o1.EventEnd == nil {
		t.Error("expected obs 1 EventEnd to be non-nil")
	} else if o1.EventEnd.Unix() != 1700000100 {
		t.Errorf("expected obs 1 EventEnd unix 1700000100, got %v", o1.EventEnd.Unix())
	}

	// Check observation 2 (invalid field_mapping CEL falls back to observation timestamp)
	o2 := observations[1]
	if o2.EventTime == nil {
		t.Error("expected obs 2 EventTime to be non-nil (defaulted to observation timestamp)")
	} else if !o2.EventTime.Equal(o2.Timestamp) {
		t.Errorf("expected obs 2 EventTime to equal observation timestamp %v, got %v", o2.Timestamp, *o2.EventTime)
	}
	if o2.EventEnd != nil {
		t.Errorf("expected obs 2 EventEnd to be nil (due to missing field), got %v", o2.EventEnd)
	}
}

func TestDeclarativeAdapter_Mappings_DefaultEventTime(t *testing.T) {
	// When no observation.event_time or field_mappings override is set,
	// event_time should default to observation.timestamp.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{
					"id":   "1",
					"name": "No event_time configured",
					"lat":  10.0,
					"lon":  20.0,
					"ts":   "2025-06-15T12:00:00Z",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// YAML with NO event_time or field_mappings — event_time should default to timestamp
	yamlTmpl := `schema_version: 1
name: test_default_event_time
source_type: test_default_event_time
layer_type: test_layer
display_name: "Test Default EventTime"
transport:
  type: http_poll
  url: "%s"
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
  timestamp: 'record.ts'
recording:
  mode: append
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ff0000"
  style:
    color: "#ffffff"
    point_size: 5
`
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlPath, []byte(replaceURL(yamlTmpl, server.URL)), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)
	ctx := context.Background()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("adapter.Start: %v", err)
	}

	_, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("adapter.Snapshot: %v", err)
	}

	if len(observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(observations))
	}

	o := observations[0]
	if o.EventTime == nil {
		t.Fatal("expected EventTime to be non-nil (defaulted to observation timestamp)")
	}
	if !o.EventTime.Equal(o.Timestamp) {
		t.Errorf("expected EventTime %v to equal observation timestamp %v", *o.EventTime, o.Timestamp)
	}
	if o.EventEnd != nil {
		t.Errorf("expected EventEnd to be nil, got %v", o.EventEnd)
	}
}

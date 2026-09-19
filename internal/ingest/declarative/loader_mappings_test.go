package declarative

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoader_LoadFile_WithMappings(t *testing.T) {
	fullYAML := `
schema_version: 1
name: test_mappings
source_type: test_mappings
layer_type: test_layer
display_name: "Test Mappings"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
  event_time: 'unix_ms(record.event_time)'
  event_end: 'unix_ms(record.event_end)'
field_mappings:
  - source: 'record.mag'
    target: 'magnitude'
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
    color: "#ff0000"
  style:
    color: "#ffffff"
    point_size: 5
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.ObservationEventTime() == nil {
		t.Error("expected non-nil observation event_time program")
	}
	if cs.ObservationEventEnd() == nil {
		t.Error("expected non-nil observation event_end program")
	}
	fms := cs.FieldMappings()
	if len(fms) != 1 {
		t.Errorf("expected 1 field mapping program, got %d", len(fms))
	} else {
		if fms[0].Program == nil {
			t.Error("expected non-nil field mapping program")
		}
		if fms[0].Target != "magnitude" {
			t.Errorf("expected target 'magnitude', got %q", fms[0].Target)
		}
		if fms[0].Type != "float" {
			t.Errorf("expected type 'float', got %q", fms[0].Type)
		}
	}
}

func TestLoader_LoadFile_InvalidMappingCEL(t *testing.T) {
	fullYAML := `
schema_version: 1
name: test_mappings
source_type: test_mappings
layer_type: test_layer
display_name: "Test Mappings"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
field_mappings:
  - source: 'invalid syntax +++'
    target: 'magnitude'
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
    color: "#ff0000"
  style:
    color: "#ffffff"
    point_size: 5
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid field mapping CEL, got nil")
	}
}

func TestLoader_LoadFile_InvalidEventTimeCEL(t *testing.T) {
	fullYAML := `
schema_version: 1
name: test_mappings
source_type: test_mappings
layer_type: test_layer
display_name: "Test Mappings"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
  event_time: 'invalid syntax +++'
recording:
  mode: append
cache:
  ttl: "60s"
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
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid event_time CEL, got nil")
	}
}

func TestLoader_LoadFile_InvalidEventEndCEL(t *testing.T) {
	fullYAML := `
schema_version: 1
name: test_mappings
source_type: test_mappings
layer_type: test_layer
display_name: "Test Mappings"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
  event_end: 'invalid syntax +++'
recording:
  mode: append
cache:
  ttl: "60s"
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
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(fullYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid event_end CEL, got nil")
	}
}

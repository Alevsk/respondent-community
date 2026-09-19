package declarative_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/ingest/declarative"
	"github.com/Alevsk/respondent/internal/logging"
)

func TestDeclarativeAdapter_LoadAndCreate(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []interface{}{
			map[string]interface{}{"id": "ok", "name": "OK", "lat": 10.0, "lon": 20.0},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create a declarative source definition
	yamlContent := `schema_version: 1
name: custom_source
source_type: custom_source
layer_type: custom_layer
display_name: "Custom Source"
transport:
  type: http_poll
  url: "` + server.URL + `"
  timeout: "10s"
  interval: "60s"
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
	yamlPath := filepath.Join(tmpDir, "custom.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	// Load and compile
	compiler, err := declarative.NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := declarative.NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Create adapter directly from compiled source
	adapter, err := declarative.NewDeclarativeAdapter(cs, logger)
	if err != nil {
		t.Fatalf("NewDeclarativeAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.Name() != "custom_source" {
		t.Errorf("Name = %q, want %q", adapter.Name(), "custom_source")
	}
	if adapter.LayerType() != "custom_layer" {
		t.Errorf("LayerType = %q, want %q", adapter.LayerType(), "custom_layer")
	}
}

func TestDeclarativeAdapter_Registry(t *testing.T) {
	// Create a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []interface{}{
			map[string]interface{}{"id": "test1", "name": "Test", "lat": 1.0, "lon": 2.0},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlContent := `schema_version: 1
name: registry_test_source
source_type: registry_test
layer_type: test_layer
display_name: "Registry Test"
transport:
  type: http_poll
  url: "` + server.URL + `"
  timeout: "10s"
  interval: "60s"
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
	yamlPath := filepath.Join(tmpDir, "test.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := declarative.NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	logger := logging.NewNopLogger()
	loader, err := declarative.NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// Register in declarative registry and verify lookup
	reg := declarative.NewRegistry()
	reg.Add("registry_test_source", cs)

	got, ok := reg.Get("registry_test_source")
	if !ok {
		t.Fatal("expected source to be in registry")
	}
	if got.Definition().Name != "registry_test_source" {
		t.Errorf("Definition().Name = %q, want %q", got.Definition().Name, "registry_test_source")
	}

	// Verify unknown source returns false
	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("expected false for unknown source")
	}
}

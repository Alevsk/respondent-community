package declarative

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestLoadDir_SkipsDisabledSource(t *testing.T) {
	dir := t.TempDir()
	// A disabled source (dry_run: true means it loads but is disabled).
	// Actually we need to find a way to create a disabled source.
	// dry_run doesn't disable; we need enabled: false.
	yaml := `schema_version: 1
name: disabled_src
source_type: disabled_src
layer_type: disabled_src_layer
display_name: "Disabled"
enabled: false
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
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
	yamlPath := filepath.Join(dir, "disabled_src.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources (disabled skipped), got %d", len(sources))
	}
}

func TestLoadDir_WithSubdirectory(t *testing.T) {
	dir := t.TempDir()
	// Create a subdirectory inside the sources dir.
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	// Write a valid YAML file in root dir.
	yamlContent := `schema_version: 1
name: loaddir_subdir_test
source_type: loaddir_subdir_test
layer_type: loaddir_subdir_test_layer
display_name: "LoadDir Subdir Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
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
	yamlPath := filepath.Join(dir, "loaddir_subdir_test.yaml")
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

	// LoadDir should skip the subdirectory and only load the YAML file.
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}

func TestLoadFile_GeoEntityEmptyLatitude(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: geo_no_lat
source_type: geo_no_lat
layer_type: geo_no_lat_layer
display_name: "Geo No Lat"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: ''
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
	yamlPath := filepath.Join(dir, "geo_no_lat.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for geo_entity with empty latitude, got nil")
	}
}

func TestLoadFile_SpatialRequiresV2(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: spatial_v1
source_type: spatial_v1
layer_type: spatial_v1_layer
display_name: "Spatial V1"
transport:
  type: http_poll
  url: "https://example.com/api/{lat}"
  method: GET
  timeout: "10s"
  interval: "60s"
  spatial:
    type: hex_grid
    radius_nm: 250
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
	yamlPath := filepath.Join(dir, "spatial_v1.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for transport.spatial with schema_version=1, got nil")
	}
}

func TestLoadFile_PasswordGrantMissingPassword(t *testing.T) {
	dir := t.TempDir()
	// Set username but use env_var for password with an unset env var.
	yamlContent := `schema_version: 1
name: oauth2_no_pass
source_type: oauth2_no_pass
layer_type: oauth2_no_pass_layer
display_name: "OAuth2 No Pass"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
  auth:
    type: oauth2
    oauth2:
      grant_type: password
      token_url: "https://example.com/token"
      credentials:
        username:
          value: "myuser"
        password:
          env_var: "NONEXISTENT_PASS_ENV_VAR_XYZ_ABC_123"
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
	yamlPath := filepath.Join(dir, "oauth2_no_pass.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	// Use an env resolver that returns empty for all vars.
	loader, err := NewLoader(compiler, func(string) string { return "" }, true, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}

	_, err = loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for empty password env var, got nil")
	}
}

func TestLoadFile_LookupTableFileNotFound(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `schema_version: 1
name: lookup_file_err
source_type: lookup_file_err
layer_type: lookup_file_err_layer
display_name: "Lookup File Error"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
lookup_tables:
  - name: categories
    key_field: id
    file: "nonexistent_file.json"
    format: json
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
	yamlPath := filepath.Join(dir, "lookup_file_err.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for missing lookup table file, got nil")
	}
}

func TestLoadFile_InvalidEntityCacheKey(t *testing.T) {
	dir := t.TempDir()
	// schema_version 2 with invalid entity_cache.key CEL expression.
	yamlContent := `schema_version: 2
name: entity_cache_bad_key
source_type: entity_cache_bad_key
layer_type: entity_cache_bad_key_layer
display_name: "Entity Cache Bad Key"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
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
entity_cache:
  enabled: true
  key: '!!!INVALID_CEL_EXPRESSION!!!'
  ttl: "300s"
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
	yamlPath := filepath.Join(dir, "entity_cache_bad_key.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid entity_cache.key CEL expression, got nil")
	}
}

func TestLoadFile_NonURLTransportWithURL(t *testing.T) {
	dir := t.TempDir()

	// Set an env var to confirm expansion works.
	t.Setenv("TEST_EXTRA_URL", "amqp://extra.example.com/vhost")

	yaml := `schema_version: 2
name: amqp_url_expand
source_type: amqp_url_expand
layer_type: amqp_url_expand_layer
display_name: "AMQP URL Expand"
transport:
  type: amqp
  url: "${TEST_EXTRA_URL}"
  timeout: "10s"
  amqp:
    url: "amqp://localhost:5672/"
    queue: "test_queue"
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

	yamlPath := filepath.Join(dir, "amqp_url_expand.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newDevLoader(t)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	// Verify the URL was expanded from env var.
	if got := cs.Definition().Transport.URL; got != "amqp://extra.example.com/vhost" {
		t.Errorf("expected expanded URL, got %q", got)
	}
}

func TestLoadFile_PaginationStopWhenCompileError(t *testing.T) {
	dir := t.TempDir()

	yaml := `schema_version: 1
name: stopwhen_invalid
source_type: stopwhen_invalid
layer_type: stopwhen_invalid_layer
display_name: "StopWhen Invalid"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
  pagination:
    type: page_number
    page_param: "page"
    size_param: "per_page"
    size: 100
    max_pages: 10
    stop_when: "!!! INVALID CEL EXPRESSION !!!"
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

	yamlPath := filepath.Join(dir, "stopwhen_invalid.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	loader := newTestLoader(t, true)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Fatal("expected compile error for invalid stop_when CEL, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

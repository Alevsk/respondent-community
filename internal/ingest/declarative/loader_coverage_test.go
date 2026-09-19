package declarative

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func newDevLoader(t *testing.T) *Loader {
	t.Helper()
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	return loader
}

// ---------------------------------------------------------------------------
// loader.go – cross-field validation: dedupe mode + empty content_hash (line 133)
// ---------------------------------------------------------------------------

func TestLoadFile_DedupeEmptyContentHash(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: dedupe_no_hash
source_type: dedupe_no_hash
layer_type: dedupe_no_hash_layer
display_name: "Dedupe No Hash"
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
  mode: dedupe
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
	yamlPath := filepath.Join(dir, "dedupe_no_hash.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for dedupe mode with empty content_hash, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – cross-field validation: geo_entity with empty longitude (line 136)
// ---------------------------------------------------------------------------

func TestLoadFile_GeoEntityEmptyLongitude(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: geo_no_lon
source_type: geo_no_lon
layer_type: geo_no_lon_layer
display_name: "Geo No Lon"
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
  longitude: ''
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
	yamlPath := filepath.Join(dir, "geo_no_lon.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for geo_entity with empty longitude, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – schema v1 with backfill field (line 159)
// ---------------------------------------------------------------------------

func TestLoadFile_SchemaV1WithBackfill(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: v1_backfill
source_type: v1_backfill
layer_type: v1_backfill_layer
display_name: "V1 Backfill"
backfill:
  threshold: 100
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
	yamlPath := filepath.Join(dir, "v1_backfill.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for schema_version 1 with backfill field, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – schema v1 with filtering field (line 159 area)
// ---------------------------------------------------------------------------

func TestLoadFile_SchemaV1WithFiltering(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: v1_filtering
source_type: v1_filtering
layer_type: v1_filtering_layer
display_name: "V1 Filtering"
filtering: "some_filter_type"
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
	yamlPath := filepath.Join(dir, "v1_filtering.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for schema_version 1 with filtering field, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – buildTokenProvider api_key with empty env var (line 309)
// ---------------------------------------------------------------------------

func TestLoadFile_ApiKeyEmptyEnvVar(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: apikey_empty_env
source_type: apikey_empty_env
layer_type: apikey_empty_env_layer
display_name: "API Key Empty Env"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
  auth:
    type: api_key
    header: X-Api-Key
    env_var: NONEXISTENT_API_KEY_FOR_TEST_12345
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
	yamlPath := filepath.Join(dir, "apikey_empty_env.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	// Use a resolver that always returns empty to trigger the empty env var error.
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, func(string) string { return "" }, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for api_key with empty env var, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – buildTokenProvider OAuth2 resolve credentials error (line 325)
// password grant with env var that is empty
// ---------------------------------------------------------------------------

func TestLoadFile_OAuth2EmptyUsernameEnvVar(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: oauth2_empty_user
source_type: oauth2_empty_user
layer_type: oauth2_empty_user_layer
display_name: "OAuth2 Empty User"
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
          env_var: NONEXISTENT_OAUTH2_USER_ENV_12345
        password:
          value: "mypassword"
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
	yamlPath := filepath.Join(dir, "oauth2_empty_user.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, func(string) string { return "" }, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for OAuth2 with empty username env var, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – buildTokenProvider OAuth2 client_credentials with empty env var
// for client_id (line 386-394 area)
// ---------------------------------------------------------------------------

func TestLoadFile_OAuth2ClientCredentialsEmptyClientID(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: oauth2_empty_client_id
source_type: oauth2_empty_client_id
layer_type: oauth2_empty_client_id_layer
display_name: "OAuth2 Empty Client ID"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
  auth:
    type: oauth2
    oauth2:
      grant_type: client_credentials
      token_url: "https://example.com/token"
      credentials:
        client_id:
          env_var: NONEXISTENT_CLIENT_ID_ENV_12345
        client_secret:
          value: "mysecret"
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
	yamlPath := filepath.Join(dir, "oauth2_empty_client_id.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, func(string) string { return "" }, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for OAuth2 client_credentials with empty client_id env var, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – LoadDir skips disabled source (line 63 area)
// ---------------------------------------------------------------------------

func TestLoadDir_SkipsDisabledSources(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: disabled_for_loaddir
source_type: disabled_for_loaddir
layer_type: disabled_for_loaddir_layer
display_name: "Disabled For LoadDir"
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
	yamlPath := filepath.Join(dir, "disabled_for_loaddir.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources (disabled skipped), got %d", len(sources))
	}
}

// ---------------------------------------------------------------------------
// loader.go – compileCEL entity cache key compile error (line 561-569 area)
// ---------------------------------------------------------------------------

func TestLoadFile_InvalidEntityCacheKeyCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 2
name: bad_cache_key
source_type: bad_cache_key
layer_type: bad_cache_key_layer
display_name: "Bad Cache Key"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
  spatial:
    type: static_regions
    regions:
      - lat: 0.0
        lon: 0.0
        label: "test"
    batch_size: 1
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
entity_cache:
  enabled: true
  ttl: "5m"
  key: '!!! bad cache key @@@'
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
	yamlPath := filepath.Join(dir, "bad_cache_key.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid entity_cache.key CEL, got nil")
	}
}

// ---------------------------------------------------------------------------
// loader.go – compileCEL field_mappings compile error (line 547 area)
// ---------------------------------------------------------------------------

func TestLoadFile_InvalidFieldMappingCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_field_mapping
source_type: bad_field_mapping
layer_type: bad_field_mapping_layer
display_name: "Bad Field Mapping"
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
field_mappings:
  - source: '!!! bad mapping @@@'
    target: 'entity.callsign'
    type: string
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
	yamlPath := filepath.Join(dir, "bad_field_mapping.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	loader := newDevLoader(t)
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid field_mappings CEL, got nil")
	}
}

// ---------------------------------------------------------------------------
// safeclient.go – DNS lookup failure (line 55)
// ---------------------------------------------------------------------------

func TestNewSSRFSafeClient_DNSLookupFails(t *testing.T) {
	client := NewSSRFSafeClient(2 * time.Second)
	// Use a clearly non-existent domain to trigger DNS failure.
	_, err := client.Get("http://this.host.does.not.exist.invalid/")
	if err == nil {
		t.Error("expected DNS error for non-existent host, got nil")
	}
}

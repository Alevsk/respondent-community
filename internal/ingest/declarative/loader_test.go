package declarative

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/config"
	"github.com/Alevsk/respondent/internal/logging"
)

// validSourceYAML is a minimal valid source definition for testing.
const validSourceYAML = `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test Source"
transport:
  type: http_poll
  url: "https://example.com/api/data"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "data.items"
entity:
  external_id: 'record.id'
  name: 'has(record.name) ? record.name : record.id'
  metadata:
    type: 'has(record.type) ? record.type : "unknown"'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  altitude: 'record.alt'
  timestamp: 'unix_ms(record.time)'
  velocity:
    speed_mps: 'record.speed'
  metadata:
    status: 'has(record.status) ? record.status : "active"'
recording:
  mode: append
cache:
  ttl: "300s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 8
`

// newTestLoader creates a Loader using config.EnvResolve which prepends
// "RESPONDENT_" to env var names — tests must use t.Setenv("RESPONDENT_<NAME>", ...).
func newTestLoader(t *testing.T, devMode bool) *Loader {
	t.Helper()
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("failed to create CEL compiler: %v", err)
	}
	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, config.EnvResolve, devMode, logger)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	return loader
}

func TestLoader_LoadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(path, []byte(validSourceYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.Definition().Name != "test_source" {
		t.Errorf("expected name 'test_source', got %q", cs.Definition().Name)
	}
	if cs.Definition().SourceType != "test_source" {
		t.Errorf("expected source_type 'test_source', got %q", cs.Definition().SourceType)
	}
	if cs.EntityID() == nil {
		t.Error("expected non-nil entity ID program")
	}
	if cs.EntityName() == nil {
		t.Error("expected non-nil entity name program")
	}
	if cs.ObservationLat() == nil {
		t.Error("expected non-nil observation latitude program")
	}
	if cs.ObservationLon() == nil {
		t.Error("expected non-nil observation longitude program")
	}
	if cs.ObservationAlt() == nil {
		t.Error("expected non-nil observation altitude program")
	}
	if cs.ObservationTS() == nil {
		t.Error("expected non-nil observation timestamp program")
	}
	if cs.Filter() != nil {
		t.Error("expected nil filter (no filter in YAML)")
	}
	if len(cs.EntityMeta()) != 1 {
		t.Errorf("expected 1 entity metadata program, got %d", len(cs.EntityMeta()))
	}
	if len(cs.ObservationVelocity()) != 1 {
		t.Errorf("expected 1 velocity program, got %d", len(cs.ObservationVelocity()))
	}
	if len(cs.ObservationMeta()) != 1 {
		t.Errorf("expected 1 observation metadata program, got %d", len(cs.ObservationMeta()))
	}
	if cs.ContentHash() != nil {
		t.Error("expected nil content hash (not defined)")
	}
}

func TestLoader_LoadFile_WithFilter(t *testing.T) {
	dir := t.TempDir()

	// Build a self-contained YAML with a top-level filter field.
	fullYAML := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test Source"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
filter: 'record.active == true'
entity:
  external_id: 'record.id'
  name: 'record.id'
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
	path2 := filepath.Join(dir, "test2.yaml")
	if err := os.WriteFile(path2, []byte(fullYAML), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cs.Filter() == nil {
		t.Error("expected non-nil filter program")
	}
}

func TestLoader_LoadFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(path, []byte("not: [valid: yaml: {"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoader_LoadFile_MissingRequiredField(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected validation error for missing source_type/layer_type, got nil")
	}
}

func TestLoader_LoadFile_InvalidCEL(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: '"hello" + 42'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "badcel.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected CEL compile error, got nil")
	}
}

func TestLoader_LoadFile_HTTPNotAllowed(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "http://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "http.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Production mode: http not allowed
	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected URL validation error for http in production, got nil")
	}

	// Dev mode: http allowed
	loaderDev := newTestLoader(t, true)
	cs, err := loaderDev.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error in dev mode: %v", err)
	}
	if cs == nil {
		t.Fatal("expected non-nil compiled source in dev mode")
	}
}

func TestLoader_LoadFile_Auth_Bearer(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: bearer
    env_var: TEST_API_KEY
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Setenv("RESPONDENT_TEST_API_KEY", "secret-token-value")
	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp := cs.TokenProvider()
	if tp == nil {
		t.Fatal("expected non-nil TokenProvider for bearer auth")
	}
	hName, hValue, err := tp.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("GetHeader error: %v", err)
	}
	if hName != "Authorization" || hValue != "Bearer secret-token-value" {
		t.Errorf("expected Authorization: Bearer secret-token-value, got %s: %s", hName, hValue)
	}
}

func TestLoader_LoadFile_Auth_ApiKey(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: api_key
    header: "X-Api-Key"
    env_var: MY_API_KEY
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "apikey.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Setenv("RESPONDENT_MY_API_KEY", "key123")
	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp := cs.TokenProvider()
	if tp == nil {
		t.Fatal("expected non-nil TokenProvider for api_key auth")
	}
	hName, hValue, err := tp.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("GetHeader error: %v", err)
	}
	if hName != "X-Api-Key" || hValue != "key123" {
		t.Errorf("expected X-Api-Key: key123, got %s: %s", hName, hValue)
	}
}

func TestLoader_LoadFile_Auth_OAuth2(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: oauth2
    oauth2:
      token_url: "https://example.com/oauth/token"
      grant_type: password
      credentials:
        client_id:
          value: "myapp"
        username:
          env_var: "TEST_USER"
        password:
          env_var: "TEST_PASS"
      refresh_before_expiry: "300s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "oauth2.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Setenv("RESPONDENT_TEST_USER", "user@example.com")
	t.Setenv("RESPONDENT_TEST_PASS", "secret123")
	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tp := cs.TokenProvider()
	if tp == nil {
		t.Fatal("expected non-nil TokenProvider for oauth2 auth")
	}
	// Verify it's an oauth2 provider (not static).
	if _, ok := tp.(*oauth2TokenProvider); !ok {
		t.Errorf("expected *oauth2TokenProvider, got %T", tp)
	}
	// Static headers should not contain auth.
	if _, hasAuth := cs.ResolvedHeaders()["Authorization"]; hasAuth {
		t.Error("ResolvedHeaders should not contain Authorization for oauth2")
	}
}

func TestLoader_LoadFile_Auth_OAuth2_MissingBlock(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: oauth2
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "oauth2_missing.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for oauth2 without oauth2 block")
	}
	if !strings.Contains(err.Error(), "oauth2") {
		t.Errorf("error should mention oauth2, got: %v", err)
	}
}

func TestLoader_LoadFile_Auth_OAuth2_MissingCredentials(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: oauth2
    oauth2:
      token_url: "https://example.com/oauth/token"
      grant_type: password
      credentials:
        client_id:
          value: "myapp"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "oauth2_nocreds.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for password grant without username/password")
	}
	if !strings.Contains(err.Error(), "password grant") {
		t.Errorf("error should mention password grant, got: %v", err)
	}
}

func TestLoader_LoadFile_Auth_EmptyEnvVar(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
  auth:
    type: bearer
    env_var: NONEXISTENT_KEY_FOR_TEST
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "emptyenv.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Setenv("RESPONDENT_NONEXISTENT_KEY_FOR_TEST", "")
	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for empty env var, got nil")
	}
}

func TestLoader_LoadDir(t *testing.T) {
	dir := t.TempDir()

	// Write a valid YAML file
	if err := os.WriteFile(filepath.Join(dir, "good.yaml"), []byte(validSourceYAML), 0644); err != nil {
		t.Fatalf("failed to write good.yaml: %v", err)
	}

	// Write an invalid YAML file (should be skipped, not fail the whole load)
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("not: [valid: yaml:"), 0644); err != nil {
		t.Fatalf("failed to write bad.yaml: %v", err)
	}

	// Write a non-YAML file (should be ignored)
	if err := os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("not yaml"), 0644); err != nil {
		t.Fatalf("failed to write readme.txt: %v", err)
	}

	loader := newTestLoader(t, false)
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only load the valid file, skip the bad one
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
	if len(sources) > 0 && sources[0].Definition().Name != "test_source" {
		t.Errorf("expected source name 'test_source', got %q", sources[0].Definition().Name)
	}
}

func TestLoader_LoadDir_NotExists(t *testing.T) {
	loader := newTestLoader(t, false)
	_, err := loader.LoadDir("/nonexistent/path/for/test")
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestLoader_LoadDir_YMLExtension(t *testing.T) {
	dir := t.TempDir()

	// .yml should also be loaded
	if err := os.WriteFile(filepath.Join(dir, "source.yml"), []byte(validSourceYAML), 0644); err != nil {
		t.Fatalf("failed to write source.yml: %v", err)
	}

	loader := newTestLoader(t, false)
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source from .yml file, got %d", len(sources))
	}
}

func TestLoader_ValidateURL(t *testing.T) {
	loaderProd := newTestLoader(t, false)
	loaderDev := newTestLoader(t, true)

	tests := []struct {
		name    string
		url     string
		loader  *Loader
		wantErr bool
	}{
		{name: "https allowed in prod", url: "https://example.com/api", loader: loaderProd, wantErr: false},
		{name: "http blocked in prod", url: "http://example.com/api", loader: loaderProd, wantErr: true},
		{name: "http allowed in dev", url: "http://localhost:8080/api", loader: loaderDev, wantErr: false},
		{name: "ftp blocked", url: "ftp://example.com/data", loader: loaderProd, wantErr: true},
		{name: "empty scheme", url: "example.com/api", loader: loaderProd, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.loader.validateURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for URL %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for URL %q: %v", tc.url, err)
			}
		})
	}
}

func TestLoader_ResolveAuth_UnsupportedType(t *testing.T) {
	t.Setenv("RESPONDENT_SOME_KEY", "value")
	loader := newTestLoader(t, false)
	_, _, err := loader.buildTokenProvider(&AuthSpec{
		Type:   "custom_oauth",
		EnvVar: "SOME_KEY",
	}, nil, "test_source")
	if err == nil {
		t.Fatal("expected error for unsupported auth type, got nil")
	}
}

func TestLoader_ResolveAuth_ApiKeyMissingHeader(t *testing.T) {
	t.Setenv("RESPONDENT_SOME_KEY", "value")
	loader := newTestLoader(t, false)
	_, _, err := loader.buildTokenProvider(&AuthSpec{
		Type:   "api_key",
		Header: "",
		EnvVar: "SOME_KEY",
	}, nil, "test_source")
	if err == nil {
		t.Fatal("expected error for api_key without header name, got nil")
	}
}

func TestLoader_URLEnvExpansion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		resolver EnvResolver
		want     string
	}{
		{
			name:     "no variables",
			input:    "https://example.com/api",
			resolver: func(string) string { return "" },
			want:     "https://example.com/api",
		},
		{
			name:  "single variable",
			input: "https://example.com/api/${MY_KEY}/data",
			resolver: func(name string) string {
				if name == "MY_KEY" {
					return "secret123"
				}
				return ""
			},
			want: "https://example.com/api/secret123/data",
		},
		{
			name:  "multiple variables",
			input: "https://${HOST}:${PORT}/api",
			resolver: func(name string) string {
				switch name {
				case "HOST":
					return "example.com"
				case "PORT":
					return "8080"
				}
				return ""
			},
			want: "https://example.com:8080/api",
		},
		{
			name:     "unset variable resolves to empty",
			input:    "https://example.com/${UNSET_VAR}/data",
			resolver: func(string) string { return "" },
			want:     "https://example.com//data",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := os.Expand(tc.input, tc.resolver)
			if got != tc.want {
				t.Errorf("os.Expand(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestLoader_LoadFile_URLEnvVarSubstitution(t *testing.T) {
	t.Setenv("RESPONDENT_TEST_API_KEY_FOR_URL", "my-secret-key")

	yamlContent := `
schema_version: 1
name: test_env_url
source_type: test_env_url
layer_type: test_env_layer
display_name: "Test Env URL"
transport:
  type: http_poll
  url: "https://example.com/api/${TEST_API_KEY_FOR_URL}/data"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "envurl.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "https://example.com/api/my-secret-key/data"
	if cs.Definition().Transport.URL != want {
		t.Errorf("URL = %q, want %q", cs.Definition().Transport.URL, want)
	}
}

// --- FIX 7: Validate dedupe mode requires content_hash ---

func TestLoader_LoadFile_DedupeRequiresContentHash(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_dedupe
source_type: test_dedupe
layer_type: test_layer
display_name: "Test Dedupe"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
  content_hash: ""
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
	dir := t.TempDir()
	path := filepath.Join(dir, "dedupe.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for dedupe mode with empty content_hash, got nil")
	}
}

func TestLoader_LoadFile_DedupeWithContentHash(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_dedupe_ok
source_type: test_dedupe_ok
layer_type: test_layer
display_name: "Test Dedupe OK"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
  content_hash: 'record.id'
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
	dir := t.TempDir()
	path := filepath.Join(dir, "dedupe_ok.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- FIX 8: Gate v2 features behind schema_version ---

func TestLoader_LoadFile_V1CannotUseFiltering(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_v1_filtering
source_type: test_v1_filtering
layer_type: test_layer
display_name: "Test V1 Filtering"
filtering: viewport
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1filtering.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for v1 source using 'filtering', got nil")
	}
}

func TestLoader_LoadFile_V1CannotUseEntityCache(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_v1_ecache
source_type: test_v1_ecache
layer_type: test_layer
display_name: "Test V1 Entity Cache"
entity_cache:
  enabled: true
  key: 'record.id'
  ttl: "60s"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1ecache.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for v1 source using 'entity_cache', got nil")
	}
}

func TestLoader_LoadFile_V1CannotUseOnDemandURL(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_v1_ondemand
source_type: test_v1_ondemand
layer_type: test_layer
display_name: "Test V1 On-Demand"
transport:
  type: http_poll
  url: "https://example.com/api"
  on_demand_url: "https://example.com/ondemand"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1ondemand.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for v1 source using 'on_demand_url', got nil")
	}
}

func TestLoader_LoadFile_V2CanUseFiltering(t *testing.T) {
	yamlContent := `
schema_version: 2
name: test_v2_filtering
source_type: test_v2_filtering
layer_type: test_layer
display_name: "Test V2 Filtering"
filtering: viewport
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v2filtering.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cs.Definition().Filtering != "viewport" {
		t.Errorf("filtering = %q, want %q", cs.Definition().Filtering, "viewport")
	}
}

func TestLoader_LoadFile_V1CanUseArrayColumns(t *testing.T) {
	// array_columns is NOT a v2-gated feature (it's a parser convenience).
	yamlContent := `
schema_version: 1
name: test_v1_arraycol
source_type: test_v1_arraycol
layer_type: test_layer
display_name: "Test V1 Array Columns"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "states"
  array_columns:
    - col_a
    - col_b
entity:
  external_id: 'record.col_a'
  name: 'record.col_a'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1arraycol.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cs.Definition().Parser.ArrayColumns) != 2 {
		t.Errorf("array_columns len = %d, want 2", len(cs.Definition().Parser.ArrayColumns))
	}
}

func TestLoader_LoadFile_V1RejectsObjectToRecords(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_v1_obj2rec
source_type: test_v1_obj2rec
layer_type: test_layer
display_name: "Test V1 Obj2Rec"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  object_to_records: true
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1obj2rec.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for object_to_records with schema_version 1")
	}
	if !strings.Contains(err.Error(), "schema_version >= 2") {
		t.Errorf("error %q does not mention schema_version >= 2", err.Error())
	}
}

func TestLoader_LoadFile_V1RejectsArrayOfArrays(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_v1_aoa
source_type: test_v1_aoa
layer_type: test_layer
display_name: "Test V1 AoA"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  array_of_arrays: true
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v1aoa.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for array_of_arrays with schema_version 1")
	}
	if !strings.Contains(err.Error(), "schema_version >= 2") {
		t.Errorf("error %q does not mention schema_version >= 2", err.Error())
	}
}

func TestLoader_LoadFile_V2AcceptsObjectToRecords(t *testing.T) {
	yamlContent := `
schema_version: 2
name: test_v2_obj2rec
source_type: test_v2_obj2rec
layer_type: test_layer
display_name: "Test V2 Obj2Rec"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  object_to_records: true
  object_key_field: "_key"
entity:
  external_id: 'record._key'
  name: 'record._key'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v2obj2rec.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cs.Definition().Parser.ObjectToRecords {
		t.Error("expected ObjectToRecords to be true")
	}
	if cs.Definition().Parser.ObjectKeyField != "_key" {
		t.Errorf("ObjectKeyField = %q, want %q", cs.Definition().Parser.ObjectKeyField, "_key")
	}
}

func TestLoader_LoadFile_V2AcceptsArrayOfArrays(t *testing.T) {
	yamlContent := `
schema_version: 2
name: test_v2_aoa
source_type: test_v2_aoa
layer_type: test_layer
display_name: "Test V2 AoA"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  array_of_arrays: true
entity:
  external_id: 'record.id'
  name: 'record.id'
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
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	dir := t.TempDir()
	path := filepath.Join(dir, "v2aoa.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cs.Definition().Parser.ArrayOfArrays {
		t.Error("expected ArrayOfArrays to be true")
	}
}

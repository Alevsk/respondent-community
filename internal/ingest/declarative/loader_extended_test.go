package declarative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoader_ValidateOAuth2Spec tests oauth2 validation error paths.
func TestLoader_ValidateOAuth2Spec(t *testing.T) {
	loader := newTestLoader(t, false)

	tests := []struct {
		name    string
		spec    *OAuth2Spec
		wantErr string
	}{
		{
			name:    "missing token_url",
			spec:    &OAuth2Spec{GrantType: "password"},
			wantErr: "token_url is required",
		},
		{
			name: "password grant missing username",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "password",
				Credentials: OAuth2Credentials{
					Password: OAuth2Value{Value: "secret"},
				},
			},
			wantErr: "username credential",
		},
		{
			name: "password grant missing password",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "password",
				Credentials: OAuth2Credentials{
					Username: OAuth2Value{Value: "user"},
				},
			},
			wantErr: "password credential",
		},
		{
			name: "client_credentials missing client_id",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "client_credentials",
				Credentials: OAuth2Credentials{
					ClientSecret: OAuth2Value{Value: "secret"},
				},
			},
			wantErr: "client_id credential",
		},
		{
			name: "client_credentials missing client_secret",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "client_credentials",
				Credentials: OAuth2Credentials{
					ClientID: OAuth2Value{Value: "myapp"},
				},
			},
			wantErr: "client_secret credential",
		},
		{
			name: "unsupported grant type",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "implicit",
			},
			wantErr: "unsupported grant_type",
		},
		{
			name: "valid password grant",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "password",
				Credentials: OAuth2Credentials{
					Username: OAuth2Value{Value: "user"},
					Password: OAuth2Value{Value: "secret"},
				},
			},
			wantErr: "",
		},
		{
			name: "valid client_credentials",
			spec: &OAuth2Spec{
				TokenURL:  "https://example.com/token",
				GrantType: "client_credentials",
				Credentials: OAuth2Credentials{
					ClientID:     OAuth2Value{Value: "myapp"},
					ClientSecret: OAuth2Value{Value: "secret"},
				},
			},
			wantErr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := loader.validateOAuth2Spec(tc.spec)
			if tc.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
				}
			}
		})
	}
}

// TestLoader_ResolveOAuth2Credentials_LiteralValues tests credential resolution with literal values.
func TestLoader_ResolveOAuth2Credentials_LiteralValues(t *testing.T) {
	loader := newTestLoader(t, false)

	t.Run("password grant with literal values", func(t *testing.T) {
		creds := OAuth2Credentials{
			Username: OAuth2Value{Value: "literaluser"},
			Password: OAuth2Value{Value: "literalpass"},
			ClientID: OAuth2Value{Value: "optionalclient"},
		}
		resolved, err := loader.resolveOAuth2Credentials(creds, "password")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved.Username != "literaluser" {
			t.Errorf("Username = %q, want literaluser", resolved.Username)
		}
		if resolved.Password != "literalpass" {
			t.Errorf("Password = %q, want literalpass", resolved.Password)
		}
		if resolved.ClientID != "optionalclient" {
			t.Errorf("ClientID = %q, want optionalclient", resolved.ClientID)
		}
	})

	t.Run("client_credentials grant with literal values", func(t *testing.T) {
		creds := OAuth2Credentials{
			ClientID:     OAuth2Value{Value: "myapp"},
			ClientSecret: OAuth2Value{Value: "mysecret"},
		}
		resolved, err := loader.resolveOAuth2Credentials(creds, "client_credentials")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved.ClientID != "myapp" {
			t.Errorf("ClientID = %q, want myapp", resolved.ClientID)
		}
		if resolved.ClientSecret != "mysecret" {
			t.Errorf("ClientSecret = %q, want mysecret", resolved.ClientSecret)
		}
	})
}

// TestLoader_ResolveOAuth2Credentials_EnvVars tests credential resolution with env var references.
func TestLoader_ResolveOAuth2Credentials_EnvVars(t *testing.T) {
	// Set env vars using t.Setenv (which uses RESPONDENT_ prefix via config.EnvResolve)
	t.Setenv("RESPONDENT_TEST_USER", "envuser")
	t.Setenv("RESPONDENT_TEST_PASS", "envpass")

	loader := newTestLoader(t, false)

	creds := OAuth2Credentials{
		Username: OAuth2Value{EnvVar: "TEST_USER"},
		Password: OAuth2Value{EnvVar: "TEST_PASS"},
	}
	resolved, err := loader.resolveOAuth2Credentials(creds, "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved.Username != "envuser" {
		t.Errorf("Username = %q, want envuser", resolved.Username)
	}
	if resolved.Password != "envpass" {
		t.Errorf("Password = %q, want envpass", resolved.Password)
	}
}

// TestLoader_ResolveOAuth2Credentials_MissingEnvVar tests error when env var is not set.
func TestLoader_ResolveOAuth2Credentials_MissingEnvVar(t *testing.T) {
	loader := newTestLoader(t, false)

	t.Run("missing username env var", func(t *testing.T) {
		creds := OAuth2Credentials{
			Username: OAuth2Value{EnvVar: "NONEXISTENT_USER_VAR_12345"},
			Password: OAuth2Value{Value: "pass"},
		}
		_, err := loader.resolveOAuth2Credentials(creds, "password")
		if err == nil {
			t.Fatal("expected error for missing env var, got nil")
		}
	})

	t.Run("missing client_secret env var", func(t *testing.T) {
		creds := OAuth2Credentials{
			ClientID:     OAuth2Value{Value: "myapp"},
			ClientSecret: OAuth2Value{EnvVar: "NONEXISTENT_SECRET_VAR_12345"},
		}
		_, err := loader.resolveOAuth2Credentials(creds, "client_credentials")
		if err == nil {
			t.Fatal("expected error for missing client_secret env var, got nil")
		}
	})
}

// TestLoader_ValidateURL_Extended tests URL validation for additional cases.
func TestLoader_ValidateURL_Extended(t *testing.T) {
	tests := []struct {
		name    string
		devMode bool
		url     string
		wantErr bool
	}{
		{name: "https always allowed", devMode: false, url: "https://example.com/api", wantErr: false},
		{name: "http in dev mode", devMode: true, url: "http://example.com/api", wantErr: false},
		{name: "http in prod mode", devMode: false, url: "http://example.com/api", wantErr: true},
		{name: "ws always allowed", devMode: false, url: "ws://example.com/ws", wantErr: false},
		{name: "wss always allowed", devMode: false, url: "wss://example.com/ws", wantErr: false},
		{name: "ftp not supported", devMode: false, url: "ftp://example.com/file", wantErr: true},
		{name: "invalid url", devMode: false, url: "://invalid", wantErr: true}, // url.Parse errors on this
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			loader := newTestLoader(t, tc.devMode)
			err := loader.validateURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("expected error for URL %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error for URL %q: %v", tc.url, err)
			}
		})
	}
}

// TestLoader_BuildTokenProvider_Bearer tests bearer auth token provider.
func TestLoader_BuildTokenProvider_Bearer(t *testing.T) {
	t.Setenv("RESPONDENT_TEST_BEARER_TOKEN", "mybearer123")

	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type:   "bearer",
		EnvVar: "TEST_BEARER_TOKEN",
	}
	tp, headers, err := loader.buildTokenProvider(auth, nil, "test_src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil token provider")
	}
	if len(headers) != 0 {
		t.Errorf("expected empty headers, got %v", headers)
	}

	// Verify the token provider returns the right header
	name, val, err := tp.GetHeader(nil) //nolint:staticcheck
	if err != nil {
		t.Fatalf("GetHeader: %v", err)
	}
	if name != "Authorization" {
		t.Errorf("header name = %q, want Authorization", name)
	}
	if val != "Bearer mybearer123" {
		t.Errorf("header value = %q, want 'Bearer mybearer123'", val)
	}
}

// TestLoader_BuildTokenProvider_Bearer_MissingEnvVar tests error when env var is missing.
func TestLoader_BuildTokenProvider_Bearer_MissingEnvVar(t *testing.T) {
	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type:   "bearer",
		EnvVar: "NONEXISTENT_BEARER_VAR_99999",
	}
	_, _, err := loader.buildTokenProvider(auth, nil, "test_src")
	if err == nil {
		t.Fatal("expected error for missing env var, got nil")
	}
}

// TestLoader_BuildTokenProvider_APIKey tests api_key auth token provider.
func TestLoader_BuildTokenProvider_APIKey(t *testing.T) {
	t.Setenv("RESPONDENT_TEST_API_KEY", "my-api-key-value")

	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type:   "api_key",
		Header: "X-API-Key",
		EnvVar: "TEST_API_KEY",
	}
	tp, _, err := loader.buildTokenProvider(auth, map[string]string{"User-Agent": "test"}, "test_src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp == nil {
		t.Fatal("expected non-nil token provider")
	}
}

// TestLoader_BuildTokenProvider_APIKey_MissingHeader tests error when header is missing.
func TestLoader_BuildTokenProvider_APIKey_MissingHeader(t *testing.T) {
	t.Setenv("RESPONDENT_TEST_API_KEY_2", "value")
	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type:   "api_key",
		EnvVar: "TEST_API_KEY_2",
		// Header intentionally empty
	}
	_, _, err := loader.buildTokenProvider(auth, nil, "test_src")
	if err == nil {
		t.Fatal("expected error for missing header name, got nil")
	}
	if !strings.Contains(err.Error(), "header name") {
		t.Errorf("expected error about header name, got: %v", err)
	}
}

// TestLoader_BuildTokenProvider_NilAuth verifies nil auth returns nil provider.
func TestLoader_BuildTokenProvider_NilAuth(t *testing.T) {
	loader := newTestLoader(t, false)
	tp, headers, err := loader.buildTokenProvider(nil, map[string]string{"X-Test": "val"}, "test_src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tp != nil {
		t.Error("expected nil token provider for nil auth")
	}
	if headers["X-Test"] != "val" {
		t.Errorf("expected headers to be copied, got %v", headers)
	}
}

// TestLoader_BuildTokenProvider_UnsupportedType tests error for unknown auth type.
func TestLoader_BuildTokenProvider_UnsupportedType(t *testing.T) {
	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type: "custom_auth",
	}
	_, _, err := loader.buildTokenProvider(auth, nil, "test_src")
	if err == nil {
		t.Fatal("expected error for unsupported auth type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported auth type") {
		t.Errorf("expected 'unsupported auth type' error, got: %v", err)
	}
}

// TestLoader_BuildTokenProvider_OAuth2_MissingSpec tests error when oauth2 block is missing.
func TestLoader_BuildTokenProvider_OAuth2_MissingSpec(t *testing.T) {
	loader := newTestLoader(t, false)
	auth := &AuthSpec{
		Type:   "oauth2",
		OAuth2: nil, // OAuth2 block missing
	}
	_, _, err := loader.buildTokenProvider(auth, nil, "test_src")
	if err == nil {
		t.Fatal("expected error when oauth2 block is nil, got nil")
	}
}

// TestLoader_LoadFile_DisabledSource verifies disabled sources are not returned.
func TestLoader_LoadFile_DisabledSource(t *testing.T) {
	yaml := `schema_version: 1
name: disabled_src_test
source_type: disabled_src_test
layer_type: disabled_layer
display_name: "Disabled Source"
enabled: false
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
	dir := t.TempDir()
	path := filepath.Join(dir, "disabled.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// LoadFile succeeds but returns the source (it's IsEnabled that determines processing)
	if cs == nil {
		t.Fatal("expected non-nil compiled source even for disabled source")
	}
	if cs.Definition().IsEnabled() {
		t.Error("expected disabled source to report IsEnabled() == false")
	}
}

// TestLoader_LoadDir_SkipsDisabled verifies LoadDir skips disabled sources.
func TestLoader_LoadDir_SkipsDisabled(t *testing.T) {
	yamlEnabled := `schema_version: 1
name: enabled_src_dirtest
source_type: enabled_src_dirtest
layer_type: enabled_layer_dirtest
display_name: "Enabled Source"
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
	yamlDisabled := strings.Replace(yamlEnabled,
		"name: enabled_src_dirtest\nsource_type: enabled_src_dirtest\nlayer_type: enabled_layer_dirtest",
		"name: disabled_src_dirtest\nsource_type: disabled_src_dirtest\nlayer_type: disabled_layer_dirtest",
		1)
	yamlDisabled = strings.Replace(yamlDisabled,
		`display_name: "Enabled Source"`, `display_name: "Disabled Source"
enabled: false`, 1)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "enabled.yaml"), []byte(yamlEnabled), 0644); err != nil {
		t.Fatalf("write enabled: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "disabled.yaml"), []byte(yamlDisabled), 0644); err != nil {
		t.Fatalf("write disabled: %v", err)
	}

	loader := newTestLoader(t, false)
	sources, err := loader.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(sources) != 1 {
		t.Errorf("expected 1 source (disabled skipped), got %d", len(sources))
	}
	if sources[0].Definition().Name != "enabled_src_dirtest" {
		t.Errorf("expected enabled source, got %q", sources[0].Definition().Name)
	}
}

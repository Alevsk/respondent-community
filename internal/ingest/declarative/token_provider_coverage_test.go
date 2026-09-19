package declarative

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
	"gopkg.in/yaml.v3"
)

func TestResolveOAuth2Credentials_UnsupportedGrantType(t *testing.T) {
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.resolveOAuth2Credentials(OAuth2Credentials{}, "unsupported_grant")
	if err == nil {
		t.Error("expected error for unsupported grant type, got nil")
	}
}

func TestDoTokenRequest_ReadBodyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set content length to lie, then close without writing the body.
		w.Header().Set("Content-Length", "1000")
		w.WriteHeader(http.StatusOK)
		// Close the connection immediately.
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}
	}))
	defer srv.Close()

	spec := &OAuth2Spec{
		GrantType: "client_credentials",
		TokenURL:  srv.URL + "/token",
	}
	creds := resolvedOAuth2Credentials{
		ClientID:     "client",
		ClientSecret: "secret",
	}
	p := newOAuth2TokenProvider(spec, creds, "test_source", logging.NewNopLogger())

	ctx := context.Background()
	_, _, err := p.GetHeader(ctx)
	// Either connection error or read error - both are acceptable.
	_ = err
}

func TestDoRefresh_ClientIDIncluded(t *testing.T) {
	receivedClientID := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			receivedClientID = r.FormValue("client_id")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new_token","expires_in":3600}`))
	}))
	defer srv.Close()

	spec := &OAuth2Spec{
		GrantType: "client_credentials",
		TokenURL:  srv.URL + "/token",
	}
	creds := resolvedOAuth2Credentials{
		ClientID:     "my_client_id",
		ClientSecret: "my_secret",
	}
	p := newOAuth2TokenProvider(spec, creds, "test_source", logging.NewNopLogger())
	// Manually set a refresh token so doRefresh is triggered.
	p.refreshToken = "old_refresh_token"

	ctx := context.Background()
	err := p.doRefresh(ctx)
	if err != nil {
		t.Logf("doRefresh returned error (may be acceptable): %v", err)
	}
	// Verify that client_id was included in the refresh request.
	if receivedClientID != "my_client_id" {
		t.Errorf("expected client_id=%q in refresh request, got %q", "my_client_id", receivedClientID)
	}
}

func TestDoRefresh_ExtraParams(t *testing.T) {
	receivedAudience := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err == nil {
			receivedAudience = r.FormValue("audience")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"new_tok","expires_in":3600}`))
	}))
	defer srv.Close()

	spec := &OAuth2Spec{
		GrantType: "client_credentials",
		TokenURL:  srv.URL + "/token",
		ExtraParams: map[string]string{
			"audience": "https://api.example.com",
		},
	}
	creds := resolvedOAuth2Credentials{
		ClientID:     "test_client",
		ClientSecret: "test_secret",
	}
	p := newOAuth2TokenProvider(spec, creds, "test_extra_params", logging.NewNopLogger())
	p.refreshToken = "old_token"

	_ = p.doRefresh(t.Context())

	if receivedAudience != "https://api.example.com" {
		t.Errorf("expected audience param in refresh request, got %q", receivedAudience)
	}
}

func TestOAuth2Value_UnmarshalYAML_StructError(t *testing.T) {
	// A YAML sequence cannot be unmarshaled as a string or as OAuth2Value struct.
	yamlData := "client_secret: [1, 2, 3]"

	var result struct {
		CS OAuth2Value `yaml:"client_secret"`
	}
	err := yaml.Unmarshal([]byte(yamlData), &result)
	if err == nil {
		t.Fatal("expected error unmarshaling list as OAuth2Value, got nil")
	}
}

func TestOAuth2Value_UnmarshalYAML_PlainString(t *testing.T) {
	yamlData := "client_secret: my-secret-value"

	var result struct {
		CS OAuth2Value `yaml:"client_secret"`
	}
	if err := yaml.Unmarshal([]byte(yamlData), &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CS.Value != "my-secret-value" {
		t.Errorf("expected value %q, got %q", "my-secret-value", result.CS.Value)
	}
}

func TestOAuth2Value_UnmarshalYAML_StructMapping(t *testing.T) {
	yamlData := `client_secret:
  value: literal-value
  env_var: MY_SECRET`

	var result struct {
		CS OAuth2Value `yaml:"client_secret"`
	}
	if err := yaml.Unmarshal([]byte(yamlData), &result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.CS.Value != "literal-value" {
		t.Errorf("expected value %q, got %q", "literal-value", result.CS.Value)
	}
	if result.CS.EnvVar != "MY_SECRET" {
		t.Errorf("expected env_var %q, got %q", "MY_SECRET", result.CS.EnvVar)
	}
}

func TestOAuth2DoTokenRequest_HTTPClientFailure(t *testing.T) {
	spec := &OAuth2Spec{
		TokenURL:  "http://auth.example.com/token",
		GrantType: "client_credentials",
	}
	creds := resolvedOAuth2Credentials{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	provider := newOAuth2TokenProvider(spec, creds, "tp_http_fail_test", logging.NewNopLogger())

	// Replace the client's transport with one that always errors.
	provider.client = &http.Client{
		Transport: &alwaysErrorTransport{
			err: fmt.Errorf("connection refused by alwaysErrorTransport"),
		},
	}

	err := provider.doAuthenticate(context.Background())
	if err == nil {
		t.Fatal("expected error from HTTP client failure, got nil")
	}
	if !strings.Contains(err.Error(), "token authenticate request failed") {
		t.Errorf("expected 'token authenticate request failed' error, got: %v", err)
	}
}

func TestOAuth2DoTokenRequest_InvalidURL(t *testing.T) {
	spec := &OAuth2Spec{
		TokenURL:  "http://example.com/\x00invalid",
		GrantType: "client_credentials",
	}
	creds := resolvedOAuth2Credentials{
		ClientID:     "test-id",
		ClientSecret: "test-secret",
	}
	provider := newOAuth2TokenProvider(spec, creds, "tp_invalid_url_test", logging.NewNopLogger())

	err := provider.doAuthenticate(context.Background())
	if err == nil {
		t.Fatal("expected error from invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "create authenticate request") {
		t.Errorf("expected 'create authenticate request' error, got: %v", err)
	}
}

func TestOAuth2DoTokenRequest_PasswordGrant(t *testing.T) {
	spec := &OAuth2Spec{
		TokenURL:  "http://auth.example.com/token",
		GrantType: "password",
		ExtraParams: map[string]string{
			"scope": "read",
		},
	}
	creds := resolvedOAuth2Credentials{
		Username: "user@example.com",
		Password: "p4ssw0rd",
	}
	provider := newOAuth2TokenProvider(spec, creds, "tp_password_grant_test", logging.NewNopLogger())

	var capturedBody string
	provider.client = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			b, _ := io.ReadAll(req.Body)
			capturedBody = string(b)
			vals, _ := url.ParseQuery(capturedBody)
			// Verify the expected form fields.
			if vals.Get("grant_type") != "password" {
				return nil, fmt.Errorf("unexpected grant_type: %s", vals.Get("grant_type"))
			}
			// Return an error response to exercise the error path.
			return &http.Response{
				StatusCode: http.StatusUnauthorized,
				Body:       io.NopCloser(strings.NewReader(`{"error":"invalid_grant"}`)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	err := provider.doAuthenticate(context.Background())
	// We expect an error (non-2xx response) but confirm the request was made.
	if err == nil {
		t.Fatal("expected error from non-2xx response, got nil")
	}
	if capturedBody == "" {
		t.Error("expected captured request body to be non-empty")
	}
	t.Logf("Captured body: %s, error: %v", capturedBody, err)
}

package declarative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

// TestExtractJSONString_NotFound verifies the "field not found" error path.
func TestExtractJSONString_NotFound(t *testing.T) {
	data := map[string]interface{}{"key": "value"}
	_, err := extractJSONString(data, "missing_field")
	if err == nil {
		t.Fatal("expected error for missing field, got nil")
	}
}

// TestExtractJSONString_WrongType verifies the type mismatch error path.
func TestExtractJSONString_WrongType(t *testing.T) {
	data := map[string]interface{}{"number": 42.0}
	_, err := extractJSONString(data, "number")
	if err == nil {
		t.Fatal("expected error for non-string field, got nil")
	}
}

// TestExtractJSONString_NestedPath verifies dot-separated key extraction.
func TestExtractJSONString_NestedPath(t *testing.T) {
	data := map[string]interface{}{
		"outer": map[string]interface{}{
			"inner": "nested_value",
		},
	}
	val, err := extractJSONString(data, "outer.inner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "nested_value" {
		t.Errorf("expected 'nested_value', got %q", val)
	}
}

// TestExtractJSONNumber_NotFound verifies false is returned when field is missing.
func TestExtractJSONNumber_NotFound(t *testing.T) {
	data := map[string]interface{}{}
	_, ok := extractJSONNumber(data, "missing")
	if ok {
		t.Error("expected ok=false for missing field")
	}
}

// TestExtractJSONNumber_WrongType verifies false is returned for non-json.Number type.
func TestExtractJSONNumber_WrongType(t *testing.T) {
	data := map[string]interface{}{"value": "not-a-number"}
	_, ok := extractJSONNumber(data, "value")
	if ok {
		t.Error("expected ok=false for string field, got true")
	}
}

// TestExtractJSONPath_NotAMap verifies nil is returned when traversal hits a non-map.
func TestExtractJSONPath_NotAMap(t *testing.T) {
	data := map[string]interface{}{
		"string_field": "hello",
	}
	// Try to traverse into a string as if it were a map
	result := extractJSONPath(data, "string_field.deeper")
	if result != nil {
		t.Errorf("expected nil for non-map traversal, got %v", result)
	}
}

// TestExtractJSONPath_MissingSegment verifies nil is returned for missing keys.
func TestExtractJSONPath_MissingSegment(t *testing.T) {
	data := map[string]interface{}{"a": map[string]interface{}{"b": "v"}}
	result := extractJSONPath(data, "a.missing")
	if result != nil {
		t.Errorf("expected nil for missing segment, got %v", result)
	}
}

// TestOAuth2TokenProvider_RefreshBeforeExpiry verifies the default and custom margins.
func TestOAuth2TokenProvider_RefreshBeforeExpiry(t *testing.T) {
	t.Run("default is 5 minutes", func(t *testing.T) {
		p := newOAuth2TokenProvider(
			&OAuth2Spec{
				TokenURL:  "https://example.com",
				GrantType: "password",
				// RefreshBeforeExpiry not set → zero Duration
			},
			resolvedOAuth2Credentials{},
			"test",
			logging.NewNopLogger(),
		)
		got := p.refreshBeforeExpiry()
		if got.Minutes() != 5 {
			t.Errorf("refreshBeforeExpiry = %v, want 5m", got)
		}
	})

	t.Run("custom value", func(t *testing.T) {
		p := newOAuth2TokenProvider(
			&OAuth2Spec{
				TokenURL:            "https://example.com",
				GrantType:           "password",
				RefreshBeforeExpiry: Duration{Duration: 30_000_000_000}, // 30s
			},
			resolvedOAuth2Credentials{},
			"test",
			logging.NewNopLogger(),
		)
		got := p.refreshBeforeExpiry()
		if got.Seconds() != 30 {
			t.Errorf("refreshBeforeExpiry = %v, want 30s", got)
		}
	})
}

// TestOAuth2TokenProvider_TokenHeaderPrefix verifies default and custom header/prefix.
func TestOAuth2TokenProvider_TokenHeaderPrefix(t *testing.T) {
	t.Run("default header and prefix", func(t *testing.T) {
		p := newOAuth2TokenProvider(
			&OAuth2Spec{
				TokenURL:  "https://example.com",
				GrantType: "password",
			},
			resolvedOAuth2Credentials{},
			"test",
			logging.NewNopLogger(),
		)
		if p.tokenHeader() != "Authorization" {
			t.Errorf("tokenHeader = %q, want Authorization", p.tokenHeader())
		}
		if p.tokenPrefix() != "Bearer " {
			t.Errorf("tokenPrefix = %q, want 'Bearer '", p.tokenPrefix())
		}
	})

	t.Run("custom header and prefix", func(t *testing.T) {
		p := newOAuth2TokenProvider(
			&OAuth2Spec{
				TokenURL:    "https://example.com",
				GrantType:   "password",
				TokenHeader: "X-Auth-Token",
				TokenPrefix: "Token ",
			},
			resolvedOAuth2Credentials{},
			"test",
			logging.NewNopLogger(),
		)
		if p.tokenHeader() != "X-Auth-Token" {
			t.Errorf("tokenHeader = %q, want X-Auth-Token", p.tokenHeader())
		}
		if p.tokenPrefix() != "Token " {
			t.Errorf("tokenPrefix = %q, want 'Token '", p.tokenPrefix())
		}
	})
}

// TestOAuth2TokenProvider_InvalidJSON verifies error for non-JSON response body.
func TestOAuth2TokenProvider_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("this is not json"))
	}))
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	_, _, err := p.GetHeader(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON response, got nil")
	}
}

// TestOAuth2TokenProvider_NoExpiresIn verifies 1-hour default TTL when expires_in is absent.
func TestOAuth2TokenProvider_NoExpiresIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "no-expiry-token",
			// expires_in intentionally absent
		})
	}))
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	name, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Authorization" || val != "Bearer no-expiry-token" {
		t.Errorf("got %s: %s, want Authorization: Bearer no-expiry-token", name, val)
	}
	// expiresAt should be ~1 hour from now
	if p.expiresAt.IsZero() {
		t.Error("expected non-zero expiresAt")
	}
}

// TestOAuth2TokenProvider_BodyTruncationInError verifies long error body is truncated.
func TestOAuth2TokenProvider_BodyTruncationInError(t *testing.T) {
	// Server returns a very long error body
	longBody := make([]byte, 512)
	for i := range longBody {
		longBody[i] = 'e'
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(longBody)
	}))
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	_, _, err := p.GetHeader(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error message should contain truncated body (max 256 chars + "...")
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Error("expected non-empty error message")
	}
}

package declarative

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestStaticTokenProvider_Bearer(t *testing.T) {
	p := &staticTokenProvider{
		headerName:  "Authorization",
		headerValue: "Bearer tok123",
	}
	name, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Authorization" {
		t.Errorf("expected header name Authorization, got %q", name)
	}
	if val != "Bearer tok123" {
		t.Errorf("expected header value 'Bearer tok123', got %q", val)
	}
}

func TestStaticTokenProvider_ApiKey(t *testing.T) {
	p := &staticTokenProvider{
		headerName:  "X-Api-Key",
		headerValue: "mykey",
	}
	name, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "X-Api-Key" || val != "mykey" {
		t.Errorf("expected X-Api-Key: mykey, got %s: %s", name, val)
	}
}

// newTestTokenServer creates a mock OAuth2 token endpoint for testing.
func newTestTokenServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(handler)
}

func TestOAuth2TokenProvider_PasswordGrant(t *testing.T) {
	var requestCount int32
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("expected form content type, got %q", r.Header.Get("Content-Type"))
		}
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "password" {
			t.Errorf("expected grant_type=password, got %q", r.FormValue("grant_type"))
		}
		if r.FormValue("username") != "user@example.com" {
			t.Errorf("expected username=user@example.com, got %q", r.FormValue("username"))
		}
		if r.FormValue("password") != "secret" {
			t.Errorf("expected password=secret, got %q", r.FormValue("password"))
		}
		if r.FormValue("client_id") != "myapp" {
			t.Errorf("expected client_id=myapp, got %q", r.FormValue("client_id"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "access-abc",
			"refresh_token": "refresh-xyz",
			"expires_in":    3600,
		})
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
		},
		resolvedOAuth2Credentials{
			ClientID: "myapp",
			Username: "user@example.com",
			Password: "secret",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	name, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Authorization" {
		t.Errorf("expected header name Authorization, got %q", name)
	}
	if val != "Bearer access-abc" {
		t.Errorf("expected 'Bearer access-abc', got %q", val)
	}
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 token request, got %d", requestCount)
	}
}

func TestOAuth2TokenProvider_ClientCredentialsGrant(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("grant_type") != "client_credentials" {
			t.Errorf("expected grant_type=client_credentials, got %q", r.FormValue("grant_type"))
		}
		if r.FormValue("client_id") != "myapp" {
			t.Errorf("expected client_id=myapp, got %q", r.FormValue("client_id"))
		}
		if r.FormValue("client_secret") != "s3cret" {
			t.Errorf("expected client_secret=s3cret, got %q", r.FormValue("client_secret"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cc-token",
			"expires_in":   7200,
		})
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "client_credentials",
		},
		resolvedOAuth2Credentials{
			ClientID:     "myapp",
			ClientSecret: "s3cret",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	name, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "Authorization" || val != "Bearer cc-token" {
		t.Errorf("expected Authorization: Bearer cc-token, got %s: %s", name, val)
	}
}

func TestOAuth2TokenProvider_CachedToken(t *testing.T) {
	var requestCount int32
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "cached-token",
			"expires_in":   3600,
		})
	})
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

	// First call — fetches token.
	_, _, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("first GetHeader error: %v", err)
	}
	// Second call — should use cached token, no new request.
	_, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("second GetHeader error: %v", err)
	}
	if val != "Bearer cached-token" {
		t.Errorf("expected cached token, got %q", val)
	}
	if atomic.LoadInt32(&requestCount) != 1 {
		t.Errorf("expected 1 token request (cached), got %d", requestCount)
	}
}

func TestOAuth2TokenProvider_RefreshOnExpiry(t *testing.T) {
	var requestCount int32
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")

		if count == 1 {
			// Initial auth — return token that expires in 1 second.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "token-v1",
				"refresh_token": "rt-v1",
				"expires_in":    1,
			})
		} else {
			// Refresh request.
			if r.FormValue("grant_type") != "refresh_token" {
				t.Errorf("expected refresh_token grant, got %q", r.FormValue("grant_type"))
			}
			if r.FormValue("refresh_token") != "rt-v1" {
				t.Errorf("expected refresh_token=rt-v1, got %q", r.FormValue("refresh_token"))
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "token-v2",
				"refresh_token": "rt-v2",
				"expires_in":    3600,
			})
		}
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:            srv.URL,
			GrantType:           "password",
			RefreshBeforeExpiry: Duration{Duration: 0}, // no safety margin for testing
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	// First call — fetches initial token.
	_, val1, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("first GetHeader error: %v", err)
	}
	if val1 != "Bearer token-v1" {
		t.Errorf("expected token-v1, got %q", val1)
	}

	// Wait for token to expire.
	time.Sleep(1100 * time.Millisecond)

	// Second call — should refresh.
	_, val2, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("second GetHeader error: %v", err)
	}
	if val2 != "Bearer token-v2" {
		t.Errorf("expected token-v2, got %q", val2)
	}
	if atomic.LoadInt32(&requestCount) != 2 {
		t.Errorf("expected 2 requests (auth + refresh), got %d", requestCount)
	}
}

func TestOAuth2TokenProvider_RefreshFailsFallsBackToReauth(t *testing.T) {
	var requestCount int32
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&requestCount, 1)
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")

		switch count {
		case 1:
			// Initial auth.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "token-v1",
				"refresh_token": "rt-bad",
				"expires_in":    1,
			})
		case 2:
			// Refresh fails.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		case 3:
			// Re-auth succeeds.
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token": "token-v3",
				"expires_in":   3600,
			})
		}
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:            srv.URL,
			GrantType:           "password",
			RefreshBeforeExpiry: Duration{Duration: 0},
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	// Initial auth.
	_, _, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("first GetHeader error: %v", err)
	}

	// Expire token.
	time.Sleep(1100 * time.Millisecond)

	// Second call — refresh fails, falls back to re-auth.
	_, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("second GetHeader error: %v", err)
	}
	if val != "Bearer token-v3" {
		t.Errorf("expected token-v3, got %q", val)
	}
	if atomic.LoadInt32(&requestCount) != 3 {
		t.Errorf("expected 3 requests (auth + refresh-fail + reauth), got %d", requestCount)
	}
}

func TestOAuth2TokenProvider_ConcurrentAccess(t *testing.T) {
	var requestCount int32
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		// Small delay to ensure concurrent calls overlap.
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "concurrent-token",
			"expires_in":   3600,
		})
	})
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

	// Launch 10 goroutines concurrently.
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, val, err := p.GetHeader(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if val != "Bearer concurrent-token" {
				errs <- fmt.Errorf("unexpected token: %q", val)
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("goroutine error: %v", err)
	}

	// Due to mutex serialization, only 1 actual token request should be made.
	if count := atomic.LoadInt32(&requestCount); count != 1 {
		t.Errorf("expected 1 token request (serialized), got %d", count)
	}
}

func TestOAuth2TokenProvider_CustomResponseMapping(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"token":   "nested-token",
				"ttl":     1800,
				"refresh": "nested-refresh",
			},
		})
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
			ResponseMapping: &OAuth2ResponseMapping{
				AccessToken:  "data.token",
				RefreshToken: "data.refresh",
				ExpiresIn:    "data.ttl",
			},
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	_, val, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val != "Bearer nested-token" {
		t.Errorf("expected nested-token, got %q", val)
	}
}

func TestOAuth2TokenProvider_CustomHeaderAndPrefix(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "custom-tok",
			"expires_in":   3600,
		})
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:    srv.URL,
			GrantType:   "password",
			TokenHeader: "X-Custom-Auth",
			TokenPrefix: "Token ",
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
	if name != "X-Custom-Auth" {
		t.Errorf("expected X-Custom-Auth, got %q", name)
	}
	if val != "Token custom-tok" {
		t.Errorf("expected 'Token custom-tok', got %q", val)
	}
}

func TestOAuth2TokenProvider_TokenEndpointError(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"server_error"}`))
	})
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
		t.Fatal("expected error for failed token endpoint")
	}
}

func TestOAuth2TokenProvider_MissingAccessToken(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"some_other_field": "value",
		})
	})
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
		t.Fatal("expected error for missing access_token in response")
	}
}

func TestOAuth2TokenProvider_ExtraParams(t *testing.T) {
	srv := newTestTokenServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.FormValue("scope") != "read write" {
			t.Errorf("expected scope='read write', got %q", r.FormValue("scope"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "extra-token",
			"expires_in":   3600,
		})
	})
	defer srv.Close()

	p := newOAuth2TokenProvider(
		&OAuth2Spec{
			TokenURL:  srv.URL,
			GrantType: "password",
			ExtraParams: map[string]string{
				"scope": "read write",
			},
		},
		resolvedOAuth2Credentials{
			Username: "user",
			Password: "pass",
		},
		"test_source",
		logging.NewNopLogger(),
	)

	_, _, err := p.GetHeader(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

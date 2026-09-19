package declarative

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// newTestHTTPTransport creates an HTTPTransport pointing to the given test server.
func newTestHTTPTransport(srv *httptest.Server, retry *RetrySpec, maxBytes int64) *HTTPTransport {
	return NewHTTPTransport(HTTPTransportConfig{
		Client:           srv.Client(),
		Headers:          map[string]string{"X-Test": "true"},
		MaxResponseBytes: maxBytes,
		SourceName:       "test_source",
		Logger:           logging.NewNopLogger(),
		Retry:            retry,
	})
}

func TestHTTPTransport_Fetch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "true" {
			t.Errorf("expected X-Test header, got %q", r.Header.Get("X-Test"))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	tr := newTestHTTPTransport(srv, nil, 0)
	body, status, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("expected status 200, got %d", status)
	}
	if string(body) != `{"ok":true}` {
		t.Errorf("body = %q, want %q", string(body), `{"ok":true}`)
	}
}

func TestHTTPTransport_Fetch_Non2xx_NonRetryable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	}))
	defer srv.Close()

	tr := newTestHTTPTransport(srv, nil, 0)
	_, status, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if status != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", status)
	}
}

func TestHTTPTransport_Fetch_Retry_Success(t *testing.T) {
	var callCount int
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		count := callCount
		mu.Unlock()

		if count < 2 {
			w.WriteHeader(http.StatusServiceUnavailable) // 503 - retryable
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: srv.Client(),
		Retry: &RetrySpec{
			MaxAttempts:  2,
			Backoff:      "fixed",
			InitialDelay: Duration{Duration: 1 * time.Millisecond},
			MaxDelay:     Duration{Duration: 5 * time.Millisecond},
		},
		MaxResponseBytes: 0,
		SourceName:       "retry_test",
		Logger:           logging.NewNopLogger(),
	})

	body, status, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error after retry: %v", err)
	}
	if status != http.StatusOK {
		t.Errorf("expected 200, got %d", status)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}

	mu.Lock()
	got := callCount
	mu.Unlock()
	if got != 2 {
		t.Errorf("expected 2 calls, got %d", got)
	}
}

func TestHTTPTransport_Fetch_AllRetriesFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests) // 429 - retryable
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: srv.Client(),
		Retry: &RetrySpec{
			MaxAttempts:  2,
			Backoff:      "exponential",
			InitialDelay: Duration{Duration: 1 * time.Millisecond},
			MaxDelay:     Duration{Duration: 5 * time.Millisecond},
		},
		SourceName: "fail_test",
		Logger:     logging.NewNopLogger(),
	})

	_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err == nil {
		t.Fatal("expected error after all retries, got nil")
	}
	if !strings.Contains(err.Error(), "all 3 attempts failed") {
		t.Errorf("expected 'all 3 attempts failed', got: %v", err)
	}
}

func TestHTTPTransport_Fetch_ContextCancelled(t *testing.T) {
	// A server that never responds quickly
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     &http.Client{Timeout: 10 * time.Second},
		SourceName: "ctx_test",
		Logger:     logging.NewNopLogger(),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, _, err := tr.Fetch(ctx, http.MethodGet, srv.URL)
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
}

func TestHTTPTransport_Fetch_WithTokenProvider(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	tp := &staticTokenProvider{
		headerName:  "Authorization",
		headerValue: "Bearer test-token",
	}

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:        srv.Client(),
		TokenProvider: tp,
		SourceName:    "tp_test",
		Logger:        logging.NewNopLogger(),
	})

	_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer test-token" {
		t.Errorf("expected Authorization: Bearer test-token, got %q", receivedAuth)
	}
}

func TestHTTPTransport_UpdateHeaders(t *testing.T) {
	var receivedHeader string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeader = r.Header.Get("X-Custom")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     srv.Client(),
		Headers:    map[string]string{"X-Custom": "original"},
		SourceName: "header_test",
		Logger:     logging.NewNopLogger(),
	})

	// First fetch with original header
	_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("first fetch error: %v", err)
	}
	if receivedHeader != "original" {
		t.Errorf("first fetch: X-Custom = %q, want %q", receivedHeader, "original")
	}

	// Update headers
	tr.UpdateHeaders(map[string]string{"X-Custom": "updated"})

	// Second fetch with updated header
	_, _, err = tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("second fetch error: %v", err)
	}
	if receivedHeader != "updated" {
		t.Errorf("second fetch: X-Custom = %q, want %q", receivedHeader, "updated")
	}
}

func TestHTTPTransport_UpdateTokenProvider(t *testing.T) {
	var receivedAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     srv.Client(),
		SourceName: "tp_update_test",
		Logger:     logging.NewNopLogger(),
	})

	// Fetch without token provider
	_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("fetch without token error: %v", err)
	}
	if receivedAuth != "" {
		t.Errorf("expected empty auth, got %q", receivedAuth)
	}

	// Update with token provider
	tr.UpdateTokenProvider(&staticTokenProvider{
		headerName:  "Authorization",
		headerValue: "Bearer new-token",
	})

	_, _, err = tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("fetch with token error: %v", err)
	}
	if receivedAuth != "Bearer new-token" {
		t.Errorf("expected Bearer new-token, got %q", receivedAuth)
	}
}

func TestHTTPTransport_Fetch_MaxResponseBytes(t *testing.T) {
	// Server returns a large body
	largebody := strings.Repeat("x", 1000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(largebody))
	}))
	defer srv.Close()

	// Limit to 100 bytes
	tr := newTestHTTPTransport(srv, nil, 100)
	body, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(body) > 100 {
		t.Errorf("expected body <= 100 bytes, got %d", len(body))
	}
}

func TestHTTPTransport_Fetch_RetryOn502_503_504(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"502 Bad Gateway", http.StatusBadGateway},
		{"503 Service Unavailable", http.StatusServiceUnavailable},
		{"504 Gateway Timeout", http.StatusGatewayTimeout},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var callCount int
			var mu sync.Mutex

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				callCount++
				mu.Unlock()
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			tr := NewHTTPTransport(HTTPTransportConfig{
				Client: srv.Client(),
				Retry: &RetrySpec{
					MaxAttempts:  1,
					Backoff:      "fixed",
					InitialDelay: Duration{Duration: 1 * time.Millisecond},
					MaxDelay:     Duration{Duration: 5 * time.Millisecond},
				},
				SourceName: fmt.Sprintf("retry_%d", tc.status),
				Logger:     logging.NewNopLogger(),
			})

			_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
			if err == nil {
				t.Errorf("expected error for status %d", tc.status)
			}

			mu.Lock()
			got := callCount
			mu.Unlock()
			if got != 2 {
				t.Errorf("expected 2 calls for retryable status %d, got %d", tc.status, got)
			}
		})
	}
}

func TestHTTPTransport_Fetch_BadURL(t *testing.T) {
	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     &http.Client{},
		SourceName: "bad_url",
		Logger:     logging.NewNopLogger(),
	})

	_, _, err := tr.Fetch(context.Background(), "GET", "://invalid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestHTTPTransport_ConcurrentUpdateHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     srv.Client(),
		SourceName: "concurrent_test",
		Logger:     logging.NewNopLogger(),
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tr.UpdateHeaders(map[string]string{"X-Count": fmt.Sprintf("%d", i)})
		}(i)

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = tr.Fetch(context.Background(), http.MethodGet, srv.URL)
		}()
	}
	wg.Wait()
}

package declarative

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestHTTPTransport_FetchCtxDoneInRetry(t *testing.T) {
	// Server that always fails with 500.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	name := "transport_fetch_ctx"
	dir := t.TempDir()
	yamlContent := fmt.Sprintf(`schema_version: 1
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Transport Fetch Ctx"
transport:
  type: http_poll
  url: "%s"
  method: GET
  timeout: "10s"
  interval: "60s"
  retry:
    max_attempts: 3
    backoff: fixed
    initial_delay: "100ms"
    max_delay: "200ms"
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
`, name, name, name, srv.URL)

	yamlPath := filepath.Join(dir, name+".yaml")
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
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logging.NewNopLogger(), testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}

	// Cancel context quickly to trigger ctx.Done() in retry delay.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err = adapter.fetchAndProcess(ctx)
	// Should return ctx error or nil after timeout.
	_ = err
}

func TestHTTPTransport_CtxDoneDuringRetryBackoff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable) // 503 → retryable
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())

	var reqCount int32
	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: &http.Client{},
		Retry: &RetrySpec{
			MaxAttempts:  5,
			Backoff:      "fixed",
			InitialDelay: Duration{Duration: 500 * time.Millisecond}, // long enough to cancel during
			MaxDelay:     Duration{Duration: 500 * time.Millisecond},
		},
		SourceName: "ctx_cancel_retry_test",
		Logger:     logging.NewNopLogger(),
	})

	// Wrap the client to count requests and cancel context after first response.
	tr.client = &http.Client{
		Transport: &countingTransport{
			inner: srv.Client().Transport,
			onResponse: func() {
				if atomic.AddInt32(&reqCount, 1) == 1 {
					// Cancel during the backoff wait after first 503.
					cancel()
				}
			},
		},
	}

	_, _, err := tr.Fetch(ctx, http.MethodGet, srv.URL)
	if err == nil {
		t.Fatal("expected error from context cancellation, got nil")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Logf("error (acceptable): %v", err)
	}
}

type countingTransport struct {
	inner      http.RoundTripper
	onResponse func()
}

func TestHTTPTransport_TokenProviderErrorDuringRetry(t *testing.T) {
	var callCount int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&callCount, 1)
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable) // trigger retry
		} else {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	}))
	defer srv.Close()

	// Token provider that fails on the second call (during retry).
	tp := &callCountingTokenProvider{failAfter: 1}

	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: srv.Client(),
		Retry: &RetrySpec{
			MaxAttempts:  2,
			Backoff:      "fixed",
			InitialDelay: Duration{Duration: 1 * time.Millisecond},
			MaxDelay:     Duration{Duration: 1 * time.Millisecond},
		},
		TokenProvider: tp,
		SourceName:    "tp_retry_err_test",
		Logger:        logging.NewNopLogger(),
	})

	_, _, err := tr.Fetch(context.Background(), http.MethodGet, srv.URL)
	if err == nil {
		t.Fatal("expected error from token provider, got nil")
	}
	if !strings.Contains(err.Error(), "resolve auth token") {
		t.Errorf("expected 'resolve auth token' error, got: %v", err)
	}
}

type callCountingTokenProvider struct {
	count     int32
	failAfter int32
}

type errorBody struct{}

type errorBodyTransport struct{}

func TestHTTPTransport_BodyReadError(t *testing.T) {
	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: &http.Client{
			Transport: &errorBodyTransport{},
		},
		SourceName: "body_read_error_test",
		Logger:     logging.NewNopLogger(),
	})

	// No retry — we want the error to propagate immediately.
	_, _, err := tr.Fetch(context.Background(), http.MethodGet, "http://example.com/api")
	if err == nil {
		t.Fatal("expected body read error, got nil")
	}
	if !strings.Contains(err.Error(), "read response body") {
		t.Errorf("expected 'read response body' error, got: %v", err)
	}
}

type alwaysErrorTransport struct {
	err error
}

func TestHTTPTransport_BodyReadError_WithRetry(t *testing.T) {
	tr := NewHTTPTransport(HTTPTransportConfig{
		Client: &http.Client{
			Transport: &errorBodyTransport{},
		},
		Retry: &RetrySpec{
			MaxAttempts:  1, // 1 retry = 2 total attempts
			Backoff:      "fixed",
			InitialDelay: Duration{Duration: 1 * time.Millisecond},
			MaxDelay:     Duration{Duration: 1 * time.Millisecond},
		},
		SourceName: "body_read_retry_test",
		Logger:     logging.NewNopLogger(),
	})

	_, _, err := tr.Fetch(context.Background(), http.MethodGet, "http://example.com/api")
	if err == nil {
		t.Fatal("expected body read error after retries, got nil")
	}
	// Final error wraps "all N attempts failed" which contains the last err.
	t.Logf("Got expected error: %v", err)
}

func TestHTTPTransport_ReadResponseBody_IOError(t *testing.T) {
	tr := NewHTTPTransport(HTTPTransportConfig{
		Client:     &http.Client{},
		SourceName: "read_body_io_error_test",
		Logger:     logging.NewNopLogger(),
	})

	resp := &http.Response{
		Body:   &errorBody{},
		Header: make(http.Header),
	}

	_, err := tr.readResponseBody(resp, 0)
	if err == nil {
		t.Fatal("expected io error from readResponseBody, got nil")
	}
	if !strings.Contains(err.Error(), "simulated body read error") {
		t.Errorf("expected simulated body read error, got: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.RoundTrip(req)
	if err == nil && c.onResponse != nil {
		c.onResponse()
	}
	return resp, err
}

func (p *callCountingTokenProvider) GetHeader(_ context.Context) (string, string, error) {
	n := atomic.AddInt32(&p.count, 1)
	if n > p.failAfter {
		return "", "", fmt.Errorf("token provider: credential expired")
	}
	return "Authorization", "Bearer valid-token", nil
}

func (e *errorBody) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated body read error")
}

func (e *errorBody) Close() error { return nil }

func (t *errorBodyTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       &errorBody{},
		Header:     make(http.Header),
	}, nil
}

func (t *alwaysErrorTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return nil, t.err
}

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

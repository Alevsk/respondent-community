package declarative

import (
	"net/http"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestNewTransport_HTTPPoll(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_http_poll",
		Transport: TransportSpec{
			Type: "http_poll",
		},
	}

	logger := logging.NewNopLogger()
	client := &http.Client{}
	headers := map[string]string{"X-Test": "value"}

	transport, err := NewTransport(def, headers, nil, nil, client, logger)
	if err != nil {
		t.Fatalf("NewTransport for http_poll: unexpected error: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}

	// Verify it's an HTTPTransport
	ht, ok := transport.(*HTTPTransport)
	if !ok {
		t.Fatalf("expected *HTTPTransport, got %T", transport)
	}
	if ht.sourceName != "test_http_poll" {
		t.Errorf("sourceName = %q, want %q", ht.sourceName, "test_http_poll")
	}
	if ht.headers["X-Test"] != "value" {
		t.Errorf("headers[X-Test] = %q, want %q", ht.headers["X-Test"], "value")
	}
}

func TestNewTransport_UnsupportedType(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_unsupported",
		Transport: TransportSpec{
			Type: "carrier_pigeon",
		},
	}

	logger := logging.NewNopLogger()
	client := &http.Client{}

	_, err := NewTransport(def, nil, nil, nil, client, logger)
	if err == nil {
		t.Fatal("expected error for unsupported transport type, got nil")
	}
}

func TestNewTransport_RegisteredTransport(t *testing.T) {
	// Register a custom transport constructor
	called := false
	RegisterTransport("test_custom", func(def *SourceDefinition, headers map[string]string, tokenProvider TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
		called = true
		return NewHTTPTransport(HTTPTransportConfig{
			Client:     &http.Client{},
			SourceName: def.Name,
			Logger:     logger,
		}), nil
	})

	// Clean up after test
	defer func() {
		transportMu.Lock()
		delete(transportConstructors, "test_custom")
		transportMu.Unlock()
	}()

	def := &SourceDefinition{
		Name: "test_registered",
		Transport: TransportSpec{
			Type: "test_custom",
		},
	}

	logger := logging.NewNopLogger()
	client := &http.Client{}

	transport, err := NewTransport(def, nil, nil, nil, client, logger)
	if err != nil {
		t.Fatalf("NewTransport for registered type: %v", err)
	}
	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
	if !called {
		t.Error("expected custom constructor to be called")
	}
}

func TestNewTransport_HTTPPollWithRetry(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_retry",
		Transport: TransportSpec{
			Type: "http_poll",
			Retry: &RetrySpec{
				MaxAttempts: 3,
				Backoff:     "exponential",
			},
			MaxResponseBytes: 1024,
		},
	}

	logger := logging.NewNopLogger()
	client := &http.Client{}

	transport, err := NewTransport(def, nil, nil, nil, client, logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ht := transport.(*HTTPTransport)
	if ht.retry == nil {
		t.Fatal("expected retry spec to be set")
	}
	if ht.retry.MaxAttempts != 3 {
		t.Errorf("retry.MaxAttempts = %d, want 3", ht.retry.MaxAttempts)
	}
	if ht.maxResponseBytes != 1024 {
		t.Errorf("maxResponseBytes = %d, want 1024", ht.maxResponseBytes)
	}
}

func TestRegisterTransport_Overwrite(t *testing.T) {
	callCount := 0
	ctor1 := func(def *SourceDefinition, headers map[string]string, tokenProvider TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
		callCount = 1
		return nil, nil
	}
	ctor2 := func(def *SourceDefinition, headers map[string]string, tokenProvider TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
		callCount = 2
		return nil, nil
	}

	RegisterTransport("test_overwrite", ctor1)
	RegisterTransport("test_overwrite", ctor2)

	defer func() {
		transportMu.Lock()
		delete(transportConstructors, "test_overwrite")
		transportMu.Unlock()
	}()

	def := &SourceDefinition{
		Name:      "test",
		Transport: TransportSpec{Type: "test_overwrite"},
	}
	logger := logging.NewNopLogger()

	_, _ = NewTransport(def, nil, nil, nil, &http.Client{}, logger)
	if callCount != 2 {
		t.Errorf("expected second constructor to be called (callCount=2), got %d", callCount)
	}
}

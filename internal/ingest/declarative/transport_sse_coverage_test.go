package declarative

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestNewSSETransport_MissingURL(t *testing.T) {
	def := &SourceDefinition{
		Name: "no_url_sse",
		Transport: TransportSpec{
			Type: "sse",
			SSE:  &SSESpec{},
			// URL is intentionally empty
		},
	}
	_, err := newSSETransport(def, nil, nil, nil, logging.NewNopLogger())
	if err == nil {
		t.Fatal("expected error for missing URL, got nil")
	}
	if !strings.Contains(err.Error(), "URL") {
		t.Errorf("error should mention URL, got: %v", err)
	}
}

func TestSSETransport_Connect_PreviousConnection(t *testing.T) {
	connectCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectCount++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprintf(w, "data: connect%d\n\n", connectCount)
		flusher.Flush()
	}))
	defer srv.Close()

	transport := newTestSSETransport(srv.URL, nil, nil)
	ctx := context.Background()

	// First connect.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("first Connect: %v", err)
	}

	// Second connect while previous is open — exercises the t.resp != nil branch.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Should receive event from the second connection.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty data from second connection")
	}
}

func TestReadEvent_RetryField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// retry field followed by a data event.
		_, _ = fmt.Fprint(w, "retry: 3000\ndata: after retry\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	transport := newTestSSETransport(srv.URL, nil, nil)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "after retry" {
		t.Errorf("got %q, want %q", string(data), "after retry")
	}
}

func TestReadEvent_IDWithNullByte(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// id with null byte should be ignored; data event should still be returned.
		_, _ = fmt.Fprint(w, "id: null\x00byte\ndata: valid data\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	transport := newTestSSETransport(srv.URL, &SSESpec{LastEventID: true}, nil)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "valid data" {
		t.Errorf("got %q, want %q", string(data), "valid data")
	}
	// The null-byte id should not have been stored.
	transport.mu.Lock()
	lastID := transport.lastEventID
	transport.mu.Unlock()
	if lastID != "" {
		t.Errorf("expected empty lastEventID for null-byte id, got %q", lastID)
	}
}

func TestReadEvent_EmptyLineNoFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// Two leading empty lines (no fields), then a real event.
		_, _ = fmt.Fprint(w, "\n\ndata: real event\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	transport := newTestSSETransport(srv.URL, nil, nil)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "real event" {
		t.Errorf("got %q, want %q", string(data), "real event")
	}
}

func TestReadEvent_EOFWithAccumulatedFields(t *testing.T) {
	// Serve a stream that ends abruptly without a final blank line.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		// Write data line but NO trailing empty line — connection closes immediately.
		_, _ = fmt.Fprint(w, "data: no final blank")
		// Handler returns, closing the connection.
	}))
	defer srv.Close()

	transport := newTestSSETransport(srv.URL, nil, nil)
	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "no final blank" {
		t.Errorf("got %q, want %q", string(data), "no final blank")
	}
}

type errTokenProvider struct {
	err error
}

type valueTokenProvider struct {
	name  string
	value string
}

func TestSSETransport_Connect_TokenProviderError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	transport := &SSETransport{
		url:           srv.URL,
		headers:       make(map[string]string),
		tokenProvider: &errTokenProvider{err: fmt.Errorf("auth failed")},
		spec:          &SSESpec{},
		sourceName:    "test_sse_auth_err",
		logger:        logging.NewNopLogger(),
		client:        &http.Client{},
	}

	err := transport.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error from token provider, got nil")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestSSETransport_Connect_TokenProviderSetHeader(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: ok\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	transport := &SSETransport{
		url:     srv.URL,
		headers: make(map[string]string),
		tokenProvider: &valueTokenProvider{
			name:  "Authorization",
			value: "Bearer test-token",
		},
		spec:       &SSESpec{},
		sourceName: "test_sse_auth_header",
		logger:     logging.NewNopLogger(),
		client:     &http.Client{},
	}

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	if receivedAuth != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer test-token")
	}
}

func TestSSETransport_Connect_DialError(t *testing.T) {
	transport := &SSETransport{
		url:        "http://127.0.0.1:1", // port 1 should be unavailable
		headers:    make(map[string]string),
		spec:       &SSESpec{},
		sourceName: "test_sse_dial_err",
		logger:     logging.NewNopLogger(),
		client:     &http.Client{Timeout: 2 * time.Second},
	}

	err := transport.Connect(context.Background())
	if err == nil {
		t.Fatal("expected dial error, got nil")
	}
	if !strings.Contains(err.Error(), "SSE connect") {
		t.Errorf("expected SSE connect error, got: %v", err)
	}
}

func TestSSETransport_Recv_ContextDoneInLoop(t *testing.T) {
	recvCtx, recvCancel := context.WithCancel(context.Background())
	defer recvCancel()

	// Server sends: heartbeat (no data) → cancel recvCtx → then blocks.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// A heartbeat event has type but no data lines — data remains "".
		// Recv will do: readEvent → data=="" → continue → top of loop → select ctx.Done
		_, _ = fmt.Fprint(w, "event: heartbeat\n\n")
		flusher.Flush()
		// Cancel the recv context before the next iteration of Recv's for loop.
		recvCancel()
		// Keep connection open so readEvent blocks on scanner.Scan().
		<-r.Context().Done()
	}))
	defer srv.Close()

	// Use a background context (never cancelled) for Connect.
	transport := newTestSSETransport(srv.URL, nil, nil)
	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Recv should eventually return a context error.
	// It may return via ctx.Done() in the select OR via scanner.Err() after conn closes.
	_, err := transport.Recv(recvCtx)
	if err == nil {
		t.Fatal("expected error from cancelled/closed context, got nil")
	}
}

func TestSSETransport_Connect_InvalidURL(t *testing.T) {
	// A URL with a null byte causes http.NewRequestWithContext to return an error.
	transport := &SSETransport{
		url:        "http://example.com/\x00invalid",
		headers:    make(map[string]string),
		spec:       &SSESpec{},
		sourceName: "sse_invalid_url_test",
		logger:     logging.NewNopLogger(),
	}

	err := transport.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error from invalid URL, got nil")
	}
	if !strings.Contains(err.Error(), "create SSE request") {
		t.Errorf("expected 'create SSE request' error, got: %v", err)
	}
}

func (p *errTokenProvider) GetHeader(_ context.Context) (string, string, error) {
	return "", "", p.err
}

func (p *valueTokenProvider) GetHeader(_ context.Context) (string, string, error) {
	return p.name, p.value, nil
}

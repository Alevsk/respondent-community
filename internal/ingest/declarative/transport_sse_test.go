package declarative

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// newTestSSEServer creates an httptest.Server that streams SSE events.
// The events parameter is a slice of raw SSE event strings (including trailing blank lines).
// The server writes all events and then closes the connection.
func newTestSSEServer(events []string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		for _, event := range events {
			_, _ = fmt.Fprint(w, event)
			flusher.Flush()
		}
	}))
}

func newTestSSETransport(url string, spec *SSESpec, headers map[string]string) *SSETransport {
	if spec == nil {
		spec = &SSESpec{}
	}
	if headers == nil {
		headers = make(map[string]string)
	}
	def := &SourceDefinition{
		Name: "test_source",
		Transport: TransportSpec{
			URL:     url,
			Timeout: Duration{Duration: 5 * time.Second},
			SSE:     spec,
			Headers: headers,
		},
	}
	logger := logging.NewNopLogger()
	t, _ := newSSETransport(def, headers, nil, nil, logger)
	return t.(*SSETransport)
}

func TestSSETransport_ConnectAndRecv(t *testing.T) {
	server := newTestSSEServer([]string{
		"data: hello world\n\n",
		"data: second event\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// First event.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 1: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}

	// Second event.
	data, err = transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 2: %v", err)
	}
	if string(data) != "second event" {
		t.Errorf("got %q, want %q", string(data), "second event")
	}
}

func TestSSETransport_MultiLineData(t *testing.T) {
	server := newTestSSEServer([]string{
		"event: earthquake\nid: evt-123\ndata: {\"mag\": 4.5, \"place\": \"California\"}\ndata: more data here\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	want := "{\"mag\": 4.5, \"place\": \"California\"}\nmore data here"
	if string(data) != want {
		t.Errorf("got %q, want %q", string(data), want)
	}
}

func TestSSETransport_EventFilter(t *testing.T) {
	server := newTestSSEServer([]string{
		"event: noise\ndata: skip me\n\n",
		"event: earthquake\ndata: keep me\n\n",
		"event: noise\ndata: skip me too\n\n",
		"event: earthquake\ndata: keep me too\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, &SSESpec{
		EventFilter: []string{"earthquake"},
	}, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// First matching event.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 1: %v", err)
	}
	if string(data) != "keep me" {
		t.Errorf("got %q, want %q", string(data), "keep me")
	}

	// Second matching event.
	data, err = transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 2: %v", err)
	}
	if string(data) != "keep me too" {
		t.Errorf("got %q, want %q", string(data), "keep me too")
	}
}

func TestSSETransport_LastEventID(t *testing.T) {
	var mu sync.Mutex
	var receivedLastEventID string
	connectCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		connectCount++
		receivedLastEventID = r.Header.Get("Last-Event-ID")
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "id: evt-42\ndata: payload\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	transport := newTestSSETransport(server.URL, &SSESpec{
		LastEventID: true,
	}, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()

	// First connection: no Last-Event-ID header.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect 1: %v", err)
	}

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 1: %v", err)
	}
	if string(data) != "payload" {
		t.Errorf("got %q, want %q", string(data), "payload")
	}

	mu.Lock()
	if receivedLastEventID != "" {
		t.Errorf("first connect should not have Last-Event-ID, got %q", receivedLastEventID)
	}
	mu.Unlock()

	// Reconnect: should send Last-Event-ID: evt-42.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect 2: %v", err)
	}

	mu.Lock()
	if receivedLastEventID != "evt-42" {
		t.Errorf("second connect Last-Event-ID = %q, want %q", receivedLastEventID, "evt-42")
	}
	mu.Unlock()
}

func TestSSETransport_FetchReturnsErrNotPullBased(t *testing.T) {
	transport := newTestSSETransport("http://localhost", nil, nil)

	_, _, err := transport.Fetch(context.Background(), "GET", "http://localhost")
	if !errors.Is(err, ErrNotPullBased) {
		t.Errorf("Fetch error = %v, want ErrNotPullBased", err)
	}
}

func TestSSETransport_CommentLines(t *testing.T) {
	server := newTestSSEServer([]string{
		": this is a comment\n: another comment\ndata: actual data\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "actual data" {
		t.Errorf("got %q, want %q", string(data), "actual data")
	}
}

func TestSSETransport_EmptyData(t *testing.T) {
	// First event has event type but no data lines; second has data.
	server := newTestSSEServer([]string{
		"event: heartbeat\n\n",
		"data: real data\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// The heartbeat event with no data should be skipped.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "real data" {
		t.Errorf("got %q, want %q", string(data), "real data")
	}
}

func TestSSETransport_ServerClose(t *testing.T) {
	server := newTestSSEServer([]string{
		"data: last event\n\n",
	})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Consume the one event.
	_, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}

	// Next Recv should return an error (stream closed).
	_, err = transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error on closed stream, got nil")
	}
	if !strings.Contains(err.Error(), "SSE stream closed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSSETransport_CustomHeaders(t *testing.T) {
	var mu sync.Mutex
	var receivedHeaders http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		receivedHeaders = r.Header.Clone()
		mu.Unlock()

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, _ := w.(http.Flusher)
		_, _ = fmt.Fprint(w, "data: ok\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, map[string]string{
		"X-Custom-Header": "custom-value",
		"Authorization":   "Bearer test-token",
	})
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if got := receivedHeaders.Get("X-Custom-Header"); got != "custom-value" {
		t.Errorf("X-Custom-Header = %q, want %q", got, "custom-value")
	}
	if got := receivedHeaders.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", got, "Bearer test-token")
	}
	if got := receivedHeaders.Get("Accept"); got != "text/event-stream" {
		t.Errorf("Accept = %q, want %q", got, "text/event-stream")
	}
	if got := receivedHeaders.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want %q", got, "no-cache")
	}
}

func TestSSETransport_EventWithNoType(t *testing.T) {
	// Events with no "event:" field should have empty event type.
	// When event filter is set, events with no type should not match named filters.
	server := newTestSSEServer([]string{
		"data: no type event\n\n",
		"event: named\ndata: named event\n\n",
	})
	defer server.Close()

	// Test 1: no filter — events with no type are returned.
	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "no type event" {
		t.Errorf("got %q, want %q", string(data), "no type event")
	}

	_ = transport.Close()

	// Test 2: filter for "named" — events with no type are skipped.
	server2 := newTestSSEServer([]string{
		"data: no type event\n\n",
		"event: named\ndata: named event\n\n",
	})
	defer server2.Close()

	transport2 := newTestSSETransport(server2.URL, &SSESpec{
		EventFilter: []string{"named"},
	}, nil)
	defer func() { _ = transport2.Close() }()

	if err := transport2.Connect(ctx); err != nil {
		t.Fatalf("Connect 2: %v", err)
	}

	data, err = transport2.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv 2: %v", err)
	}
	if string(data) != "named event" {
		t.Errorf("got %q, want %q", string(data), "named event")
	}
}

func TestSSETransport_ContentTypeValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"error": "wrong content type"}`)
	}))
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	err := transport.Connect(context.Background())
	if err == nil {
		t.Fatal("expected error for wrong Content-Type")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("error should mention Content-Type, got: %v", err)
	}
}

func TestSSETransport_RecvNotConnected(t *testing.T) {
	transport := newTestSSETransport("http://localhost", nil, nil)

	_, err := transport.Recv(context.Background())
	if err == nil {
		t.Fatal("expected error when not connected")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention not connected, got: %v", err)
	}
}

func TestSSETransport_CloseMultipleTimes(t *testing.T) {
	server := newTestSSEServer([]string{"data: test\n\n"})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)

	if err := transport.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Close multiple times should not panic.
	if err := transport.Close(); err != nil {
		t.Errorf("Close 1: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Errorf("Close 2: %v", err)
	}
}

func TestSSETransport_ReconnectAfterClose(t *testing.T) {
	server := newTestSSEServer([]string{"data: reconnected\n\n"})
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)

	ctx := context.Background()

	// Connect, close, reconnect.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect 1: %v", err)
	}
	if err := transport.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect 2: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "reconnected" {
		t.Errorf("got %q, want %q", string(data), "reconnected")
	}
}

func TestSSETransport_CRLFLineEndings(t *testing.T) {
	// Simulate a server sending \r\n line endings.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		// Write with \r\n line endings.
		_, _ = fmt.Fprint(w, "data: crlf event\r\n\r\n")
		flusher.Flush()
	}))
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx := context.Background()
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv: %v", err)
	}
	if string(data) != "crlf event" {
		t.Errorf("got %q, want %q", string(data), "crlf event")
	}
}

func TestSSETransport_ContextCancellation(t *testing.T) {
	// Server that blocks indefinitely.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		flusher.Flush()
		// Block until client disconnects.
		<-r.Context().Done()
	}))
	defer server.Close()

	transport := newTestSSETransport(server.URL, nil, nil)
	defer func() { _ = transport.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	_, err := transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestParseSSELine(t *testing.T) {
	tests := []struct {
		line      string
		wantField string
		wantValue string
	}{
		{"data: hello", "data", "hello"},
		{"data:hello", "data", "hello"},
		{"data:", "data", ""},
		{"data", "data", ""},
		{"event: earthquake", "event", "earthquake"},
		{"id: 42", "id", "42"},
		{"retry: 3000", "retry", "3000"},
		{"data:  two spaces", "data", " two spaces"}, // Only first space stripped.
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			field, value := parseSSELine(tt.line)
			if field != tt.wantField {
				t.Errorf("field = %q, want %q", field, tt.wantField)
			}
			if value != tt.wantValue {
				t.Errorf("value = %q, want %q", value, tt.wantValue)
			}
		})
	}
}

func TestRegisterTransport_SSE(t *testing.T) {
	// Verify that the SSE transport was registered via init().
	def := &SourceDefinition{
		Name: "test_sse_factory",
		Transport: TransportSpec{
			Type:    "sse",
			URL:     "http://localhost:9999",
			Timeout: Duration{Duration: 5 * time.Second},
		},
	}
	logger := logging.NewNopLogger()
	st, err := NewTransport(def, nil, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("NewTransport: %v", err)
	}
	if st == nil {
		t.Fatal("expected non-nil Transport")
	}

	// Verify it implements StreamTransport.
	if _, ok := st.(StreamTransport); !ok {
		t.Fatalf("expected StreamTransport, got %T", st)
	}
}

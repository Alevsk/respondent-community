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

	"github.com/gorilla/websocket"

	"github.com/Alevsk/respondent/internal/logging"
)

// testWSUpgrader is a shared upgrader for test WebSocket servers.
var testWSUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// startTestWSServer creates an httptest server that upgrades to WebSocket
// and calls handler with the connection. Returns the server and its ws:// URL.
func startTestWSServer(t *testing.T, handler func(conn *websocket.Conn)) (*httptest.Server, string) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		handler(conn)
	}))

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

// newTestWSTransport creates a WebSocketTransport with test defaults.
func newTestWSTransport(url string, spec *WebSocketSpec, headers map[string]string) *WebSocketTransport {
	if spec == nil {
		spec = &WebSocketSpec{}
	}
	logger := logging.NewNopLogger()
	return &WebSocketTransport{
		url:        url,
		headers:    headers,
		spec:       spec,
		sourceName: "test_ws_source",
		logger:     logger,
	}
}

func TestWebSocketTransport_ConnectAndRecv(t *testing.T) {
	messages := []string{"hello", "world", "test"}

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		for _, msg := range messages {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
				return
			}
		}
		// Keep connection open until client reads all messages.
		// The test will close the connection.
		select {}
	})
	defer srv.Close()

	transport := newTestWSTransport(wsURL, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	for _, want := range messages {
		data, err := transport.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv failed: %v", err)
		}
		if string(data) != want {
			t.Errorf("Recv = %q, want %q", string(data), want)
		}
	}
}

func TestWebSocketTransport_SubscribeMessages(t *testing.T) {
	subscribeMsgs := []string{`{"action":"subscribe","channel":"trades"}`, `{"action":"subscribe","channel":"orders"}`}

	var received []string
	var mu sync.Mutex
	ready := make(chan struct{})

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		for range subscribeMsgs {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			mu.Lock()
			received = append(received, string(data))
			mu.Unlock()
		}
		close(ready)
		// Keep alive.
		select {}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		SubscribeMessages: subscribeMsgs,
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Wait for server to receive subscribe messages.
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("timed out waiting for subscribe messages")
	}

	mu.Lock()
	defer mu.Unlock()

	if len(received) != len(subscribeMsgs) {
		t.Fatalf("received %d subscribe messages, want %d", len(received), len(subscribeMsgs))
	}
	for i, want := range subscribeMsgs {
		if received[i] != want {
			t.Errorf("subscribe message[%d] = %q, want %q", i, received[i], want)
		}
	}
}

func TestWebSocketTransport_PingPong(t *testing.T) {
	pingReceived := make(chan struct{}, 10)

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		conn.SetPingHandler(func(appData string) error {
			pingReceived <- struct{}{}
			// Send pong back (default behavior).
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})
		// Must read to process control frames.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		PingInterval: Duration{Duration: 100 * time.Millisecond},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Wait for at least 2 pings.
	for i := 0; i < 2; i++ {
		select {
		case <-pingReceived:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for ping %d", i+1)
		}
	}
}

func TestWebSocketTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	transport := newTestWSTransport("ws://localhost:0", nil, nil)

	_, _, err := transport.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Errorf("Fetch error = %v, want ErrNotPullBased", err)
	}
}

func TestWebSocketTransport_Close(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		// Keep alive.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	transport := newTestWSTransport(wsURL, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close should not error.
	if err := transport.Close(); err != nil {
		t.Errorf("Close error: %v", err)
	}

	// Second Close should be safe (idempotent).
	if err := transport.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
}

func TestWebSocketTransport_ServerDisconnect(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		// Send one message then close immediately.
		_ = conn.WriteMessage(websocket.TextMessage, []byte("bye"))
		_ = conn.Close()
	})
	defer srv.Close()

	transport := newTestWSTransport(wsURL, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// First read should succeed.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("first Recv failed: %v", err)
	}
	if string(data) != "bye" {
		t.Errorf("Recv = %q, want %q", string(data), "bye")
	}

	// Second read should return error (server disconnected).
	_, err = transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from Recv after server disconnect, got nil")
	}
}

func TestWebSocketTransport_BinaryMessages(t *testing.T) {
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE}

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		if err := conn.WriteMessage(websocket.BinaryMessage, binaryData); err != nil {
			return
		}
		select {}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		MessageFormat: "binary",
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}

	if len(data) != len(binaryData) {
		t.Fatalf("received %d bytes, want %d", len(data), len(binaryData))
	}
	for i, b := range data {
		if b != binaryData[i] {
			t.Errorf("byte[%d] = %#x, want %#x", i, b, binaryData[i])
		}
	}
}

func TestWebSocketTransport_Compression(t *testing.T) {
	msg := "compressed message payload"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin:       func(r *http.Request) bool { return true },
			EnableCompression: true,
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade error: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			return
		}
		select {}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	spec := &WebSocketSpec{
		Compression: true,
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if string(data) != msg {
		t.Errorf("Recv = %q, want %q", string(data), msg)
	}
}

func TestWebSocketTransport_CustomHeaders(t *testing.T) {
	var receivedHeaders http.Header
	ready := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		close(ready)

		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		select {}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	headers := map[string]string{
		"X-Api-Key":    "test-key-123",
		"X-Custom-Hdr": "custom-value",
	}
	spec := &WebSocketSpec{
		Origin: "https://example.com",
	}
	transport := newTestWSTransport(wsURL, spec, headers)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("timed out waiting for connection")
	}

	if got := receivedHeaders.Get("X-Api-Key"); got != "test-key-123" {
		t.Errorf("X-Api-Key = %q, want %q", got, "test-key-123")
	}
	if got := receivedHeaders.Get("X-Custom-Hdr"); got != "custom-value" {
		t.Errorf("X-Custom-Hdr = %q, want %q", got, "custom-value")
	}
	if got := receivedHeaders.Get("Origin"); got != "https://example.com" {
		t.Errorf("Origin = %q, want %q", got, "https://example.com")
	}
}

func TestWebSocketTransport_ContextCancellation(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		// Don't send anything; Recv should block until context is cancelled.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	transport := newTestWSTransport(wsURL, nil, nil)

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connectCancel()

	if err := transport.Connect(connectCtx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Create a short-lived context for Recv.
	recvCtx, recvCancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer recvCancel()

	start := time.Now()
	_, err := transport.Recv(recvCtx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from Recv with cancelled context, got nil")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Recv error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("Recv took %v, expected cancellation within ~200ms", elapsed)
	}
}

func TestWebSocketTransport_ReconnectAfterClose(t *testing.T) {
	connectCount := 0
	var mu sync.Mutex

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		mu.Lock()
		connectCount++
		mu.Unlock()

		_ = conn.WriteMessage(websocket.TextMessage, []byte("connected"))
		// Keep alive.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	transport := newTestWSTransport(wsURL, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First connection.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("first Connect failed: %v", err)
	}
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("first Recv failed: %v", err)
	}
	if string(data) != "connected" {
		t.Errorf("first Recv = %q, want %q", string(data), "connected")
	}

	// Close.
	if err := transport.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reconnect.
	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("second Connect failed: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err = transport.Recv(ctx)
	if err != nil {
		t.Fatalf("second Recv failed: %v", err)
	}
	if string(data) != "connected" {
		t.Errorf("second Recv = %q, want %q", string(data), "connected")
	}

	mu.Lock()
	if connectCount != 2 {
		t.Errorf("server saw %d connections, want 2", connectCount)
	}
	mu.Unlock()
}

func TestWebSocketTransport_LargeMessages(t *testing.T) {
	// Verifies that the read buffer handles messages larger than the old
	// default of 4096 bytes. This regression caused a panic in production
	// with AISStream messages (~5.5KB).
	sizes := []int{4097, 8192, 16384, 65000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%d_bytes", size), func(t *testing.T) {
			payload := make([]byte, size)
			for i := range payload {
				payload[i] = byte('A' + (i % 26))
			}

			srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
				defer func() { _ = conn.Close() }()
				if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
					return
				}
				select {}
			})
			defer srv.Close()

			transport := newTestWSTransport(wsURL, nil, nil)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := transport.Connect(ctx); err != nil {
				t.Fatalf("Connect failed: %v", err)
			}
			defer func() { _ = transport.Close() }()

			data, err := transport.Recv(ctx)
			if err != nil {
				t.Fatalf("Recv failed for %d-byte message: %v", size, err)
			}
			if len(data) != size {
				t.Errorf("received %d bytes, want %d", len(data), size)
			}
		})
	}
}

func TestNewWebSocketTransportViaFactory(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ws",
		Transport: TransportSpec{
			Type:    "websocket",
			URL:     "ws://localhost:8080/ws",
			Headers: map[string]string{"Authorization": "Bearer token"},
			Timeout: Duration{Duration: 10 * time.Second},
			WebSocket: &WebSocketSpec{
				SubscribeMessages: []string{`{"subscribe":"test"}`},
				MessageFormat:     "text",
				PingInterval:      Duration{Duration: 15 * time.Second},
			},
		},
	}

	logger := logging.NewNopLogger()
	transport, err := NewTransport(def, def.Transport.Headers, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}

	wst, ok := transport.(*WebSocketTransport)
	if !ok {
		t.Fatalf("expected *WebSocketTransport, got %T", transport)
	}
	if wst.url != "ws://localhost:8080/ws" {
		t.Errorf("url = %q, want %q", wst.url, "ws://localhost:8080/ws")
	}
	if wst.spec.PingInterval.Duration != 15*time.Second {
		t.Errorf("ping interval = %v, want 15s", wst.spec.PingInterval.Duration)
	}
	if len(wst.spec.SubscribeMessages) != 1 {
		t.Errorf("subscribe messages count = %d, want 1", len(wst.spec.SubscribeMessages))
	}
}

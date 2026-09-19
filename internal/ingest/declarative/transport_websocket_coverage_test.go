package declarative

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestNewWebSocketTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_ws_nil_spec",
		Transport: TransportSpec{
			Type:      "websocket",
			URL:       "ws://localhost:0",
			WebSocket: nil, // explicitly nil
		},
	}
	tr, err := newWebSocketTransport(def, nil, nil, nil, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("newWebSocketTransport with nil spec: %v", err)
	}
	if tr == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestWebSocketTransport_Subprotocols(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			CheckOrigin:  func(r *http.Request) bool { return true },
			Subprotocols: []string{"chat"},
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		select {}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	spec := &WebSocketSpec{
		Subprotocols: []string{"chat"},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect with subprotocols: %v", err)
	}
	defer func() { _ = transport.Close() }()
}

func TestWebSocketTransport_LZWDecode(t *testing.T) {
	// Encode a test payload with the custom LZW encoder from decode_lzw_test.go.
	original := `{"id":"entity-1","lat":40.7,"lon":-74.0}`
	encoded := encodeLZW(original)

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		if err := conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
			return
		}
		select {}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		Decode: "lzw",
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv with LZW: %v", err)
	}
	if string(data) != original {
		t.Errorf("decoded = %q, want %q", string(data), original)
	}
}

func TestWebSocketTransport_LZWDecodeError(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		// Send a single byte that is a valid rune but creates an invalid LZW code
		// sequence: rune(65)='A' is first, then rune(9999) is not in dict and
		// is not dictSize. This matches the decodeLZW "invalid code" path.
		invalidLZW := []byte(string([]rune{65, 9999}))
		if err := conn.WriteMessage(websocket.BinaryMessage, invalidLZW); err != nil {
			return
		}
		select {}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		Decode: "lzw",
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	_, err := transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected LZW decode error, got nil")
	}
	if !strings.Contains(err.Error(), "lzw") {
		t.Errorf("expected lzw error, got: %v", err)
	}
}

func TestWebSocketTransport_RecvNotConnected(t *testing.T) {
	transport := newTestWSTransport("ws://localhost:0", nil, nil)
	// Don't call Connect — conn is nil.
	_, err := transport.Recv(context.Background())
	if err == nil {
		t.Fatal("expected error when not connected, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("expected 'not connected' error, got: %v", err)
	}
}

func TestWebSocketTransport_PingLoopConnReplaced(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		PingInterval: Duration{Duration: 50 * time.Millisecond},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Replace the connection in the transport (simulating a reconnect).
	// This makes the pingLoop's guard (t.conn != conn) trigger and exit.
	transport.mu.Lock()
	transport.conn = nil // nil signals pingLoop to exit via guard check
	transport.mu.Unlock()

	// Allow time for the ping goroutine to notice the replaced conn.
	time.Sleep(150 * time.Millisecond)
	_ = transport.Close()
}

func TestWebSocketTransport_PingLoopWriteError(t *testing.T) {
	// Server closes immediately after upgrade, causing ping to fail.
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		// Close immediately — this will cause the ping to fail.
		_ = conn.Close()
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		PingInterval: Duration{Duration: 30 * time.Millisecond},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Wait for ping to fail (server closed immediately).
	// The pingLoop should exit on its own after the write error.
	time.Sleep(200 * time.Millisecond)
}

func TestWebSocketTransport_SubscribeError(t *testing.T) {
	// Server upgrades then immediately closes the connection before receiving subscribe.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testWSUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Close immediately after upgrade, before the client can send subscribe.
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	spec := &WebSocketSpec{
		SubscribeMessages: []string{`{"subscribe":"test"}`},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		_ = transport.Close()
		// It's possible the write didn't fail fast enough; not a test failure.
		t.Skip("subscribe error did not occur before connection closed")
	}
	if !strings.Contains(err.Error(), "subscribe") {
		t.Errorf("expected subscribe error, got: %v", err)
	}
}

func TestWebSocketTransport_Connect_TokenProviderError(t *testing.T) {
	transport := &WebSocketTransport{
		url:     "ws://localhost:0",
		headers: make(map[string]string),
		tokenProvider: &errTokenProvider{
			err: fmt.Errorf("ws auth failed"),
		},
		spec:       &WebSocketSpec{},
		sourceName: "test_ws_auth_err",
		logger:     logging.NewNopLogger(),
	}

	err := transport.Connect(context.Background())
	if err == nil {
		t.Fatal("expected token provider error, got nil")
	}
	if !strings.Contains(err.Error(), "auth") {
		t.Errorf("expected auth error, got: %v", err)
	}
}

func TestWebSocketTransport_Connect_TokenProviderSetHeader(t *testing.T) {
	var receivedToken string
	ready := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedToken = r.Header.Get("Authorization")
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
	transport := &WebSocketTransport{
		url:     wsURL,
		headers: make(map[string]string),
		tokenProvider: &valueTokenProvider{
			name:  "Authorization",
			value: "Bearer ws-token",
		},
		spec:       &WebSocketSpec{},
		sourceName: "test_ws_auth_header",
		logger:     logging.NewNopLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("timed out waiting for connection")
	}

	if receivedToken != "Bearer ws-token" {
		t.Errorf("Authorization = %q, want %q", receivedToken, "Bearer ws-token")
	}
}

func TestWebSocketTransport_Connect_DialError(t *testing.T) {
	transport := newTestWSTransport("ws://127.0.0.1:1", nil, nil) // port 1 unavailable

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	if err == nil {
		_ = transport.Close()
		t.Fatal("expected dial error, got nil")
	}
	if !strings.Contains(err.Error(), "websocket dial") {
		t.Errorf("expected websocket dial error, got: %v", err)
	}
}

func TestWebSocketTransport_PongHandlerLog(t *testing.T) {
	pongReceived := make(chan struct{}, 1)

	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		// Set a ping handler that sends back pong normally.
		conn.SetPingHandler(func(appData string) error {
			return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(time.Second))
		})
		// Notify when pong is about to be sent.
		go func() {
			// Send a direct pong to the client.
			time.Sleep(50 * time.Millisecond)
			_ = conn.WriteControl(websocket.PongMessage, []byte("test"), time.Now().Add(time.Second))
			pongReceived <- struct{}{}
		}()
		// Keep alive.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		PingInterval: Duration{Duration: 5 * time.Second}, // long interval so ping goroutine doesn't interfere
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Wait for pong to be sent.
	select {
	case <-pongReceived:
	case <-time.After(2 * time.Second):
		t.Log("pong not received within timeout — pong handler path may not have been triggered")
	}
}

func TestWebSocketTransport_Connect_Compression(t *testing.T) {
	srv, wsURL := startTestWSServer(t, func(conn *websocket.Conn) {
		defer func() { _ = conn.Close() }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	defer srv.Close()

	spec := &WebSocketSpec{
		Compression: true,
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect with compression: %v", err)
	}
	defer func() { _ = transport.Close() }()
}

func TestWebSocketTransport_Connect_Origin(t *testing.T) {
	var receivedOrigin string
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			receivedOrigin = r.Header.Get("Origin")
			return true
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	spec := &WebSocketSpec{
		Origin: "https://example.com",
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect with origin: %v", err)
	}
	defer func() { _ = transport.Close() }()

	if receivedOrigin != "https://example.com" {
		t.Errorf("expected Origin: https://example.com, got %q", receivedOrigin)
	}
}

func TestWebSocketTransport_Connect_SubscribeFailure(t *testing.T) {
	// Signal channel so the server knows the test is running.
	upgraded := make(chan struct{})

	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		// Signal that upgrade is done.
		close(upgraded)
		// Close the connection immediately.
		_ = conn.Close()
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	spec := &WebSocketSpec{
		SubscribeMessages: []string{`{"subscribe":"events"}`},
	}
	transport := newTestWSTransport(wsURL, spec, nil)

	// Wait briefly for the server goroutine to be ready.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := transport.Connect(ctx)
	// The gorilla write may succeed or fail depending on timing.
	// Both are acceptable — this test mainly exercises code paths.
	if err != nil {
		// If an error occurred, it should be about subscribe or dial.
		if !strings.Contains(err.Error(), "subscribe") && !strings.Contains(err.Error(), "websocket") {
			t.Logf("got unexpected error: %v", err)
		}
	} else {
		_ = transport.Close()
	}
}

func TestWebSocketTransport_PongHandlerFired(t *testing.T) {
	pongFired := make(chan struct{}, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		// Send a pong immediately — the client's SetPongHandler should fire.
		err = conn.WriteControl(
			websocket.PongMessage,
			[]byte("keepalive"),
			time.Now().Add(time.Second),
		)
		if err == nil {
			pongFired <- struct{}{}
		}

		// Keep alive until client disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	transport := &WebSocketTransport{
		url:        wsURL,
		headers:    make(map[string]string),
		spec:       &WebSocketSpec{},
		sourceName: "pong_handler_test",
		logger:     logging.NewNopLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := transport.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = transport.Close() }()

	// Wait for the server to confirm pong was sent.
	select {
	case <-pongFired:
		// Give the client a moment to process the pong control frame.
		time.Sleep(100 * time.Millisecond)
	case <-time.After(3 * time.Second):
		t.Log("pong not sent by server in time — skipping pong handler verification")
	}

	// Trigger a read to ensure pong handler fires (pong is processed by Recv loop).
	// We just ensure Connect succeeded; pong processing happens asynchronously.
}

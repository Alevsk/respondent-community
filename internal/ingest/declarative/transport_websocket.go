package declarative

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/Alevsk/respondent/internal/logging"
)

const (
	// defaultPingInterval is the default interval for client-initiated ping frames.
	defaultPingInterval = 30 * time.Second

	// closeGracePeriod is how long to wait for a close frame response.
	closeGracePeriod = 5 * time.Second
)

// Compile-time assertion that WebSocketTransport implements StreamTransport.
var _ StreamTransport = (*WebSocketTransport)(nil)

func init() {
	RegisterTransport("websocket", newWebSocketTransport)
}

// WebSocketTransport implements StreamTransport for WebSocket sources.
// It maintains a persistent WebSocket connection and delivers messages via Recv.
type WebSocketTransport struct {
	url           string
	headers       map[string]string
	tokenProvider TokenProvider
	spec          *WebSocketSpec
	sourceName    string
	logger        *logging.Logger

	mu   sync.Mutex
	conn *websocket.Conn
	done chan struct{}
}

// newWebSocketTransport creates a WebSocketTransport from a source definition.
func newWebSocketTransport(def *SourceDefinition, resolvedHeaders map[string]string, tokenProvider TokenProvider, _ EnvResolver, logger *logging.Logger) (Transport, error) {
	spec := def.Transport.WebSocket
	if spec == nil {
		spec = &WebSocketSpec{}
	}

	headers := make(map[string]string, len(resolvedHeaders))
	for k, v := range resolvedHeaders {
		headers[k] = v
	}

	return &WebSocketTransport{
		url:           def.Transport.URL,
		headers:       headers,
		tokenProvider: tokenProvider,
		spec:          spec,
		sourceName:    def.Name,
		logger:        logger,
	}, nil
}

// Fetch returns ErrNotPullBased because WebSocket is a push-based transport.
func (t *WebSocketTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect establishes the WebSocket connection, sends subscribe messages, and starts
// the ping goroutine. Safe to call after Close for reconnection.
func (t *WebSocketTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Reset done channel for reconnection after a previous Close.
	t.done = make(chan struct{})

	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
		ReadBufferSize:   65536,
		WriteBufferSize:  4096,
	}

	// Extract timeout from context deadline if available.
	if deadline, ok := ctx.Deadline(); ok {
		dialer.HandshakeTimeout = time.Until(deadline)
	}

	if t.spec.Compression {
		dialer.EnableCompression = true
	}

	if len(t.spec.Subprotocols) > 0 {
		dialer.Subprotocols = t.spec.Subprotocols
	}

	// Build request headers.
	reqHeader := make(http.Header)
	for k, v := range t.headers {
		reqHeader.Set(k, v)
	}
	// Dynamic auth header (may trigger OAuth2 token refresh on reconnect).
	if t.tokenProvider != nil {
		hName, hValue, err := t.tokenProvider.GetHeader(ctx)
		if err != nil {
			return fmt.Errorf("resolve auth token for websocket: %w", err)
		}
		if hName != "" {
			reqHeader.Set(hName, hValue)
		}
	}
	if t.spec.Origin != "" {
		reqHeader.Set("Origin", t.spec.Origin)
	}

	t.logger.Info("connecting to WebSocket",
		logging.String("url", t.url),
		logging.String("source_name", t.sourceName),
	)

	conn, _, err := dialer.DialContext(ctx, t.url, reqHeader)
	if err != nil {
		return fmt.Errorf("websocket dial %q: %w", t.url, err)
	}

	t.conn = conn

	// Set pong handler to track connectivity.
	conn.SetPongHandler(func(appData string) error {
		t.logger.Debug("pong received", logging.String("source_name", t.sourceName))
		return nil
	})

	// Send subscribe messages.
	for _, msg := range t.spec.SubscribeMessages {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			// Close on subscribe failure; caller will retry via reconnect loop.
			_ = conn.Close()
			t.conn = nil
			return fmt.Errorf("send subscribe message: %w", err)
		}
		t.logger.Debug("sent subscribe message",
			logging.String("source_name", t.sourceName),
		)
	}

	// Start ping goroutine.
	pingInterval := defaultPingInterval
	if t.spec.PingInterval.Duration > 0 {
		pingInterval = t.spec.PingInterval.Duration
	}

	done := t.done
	go t.pingLoop(conn, pingInterval, done)

	t.logger.Info("WebSocket connected",
		logging.String("url", t.url),
		logging.String("source_name", t.sourceName),
	)

	return nil
}

// pingLoop sends WebSocket ping frames at the configured interval.
// Exits when done is closed or a write error occurs.
func (t *WebSocketTransport) pingLoop(conn *websocket.Conn, interval time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			t.mu.Lock()
			// Guard against the connection being replaced or closed.
			if t.conn != conn {
				t.mu.Unlock()
				return
			}
			err := conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(10*time.Second),
			)
			t.mu.Unlock()

			if err != nil {
				t.logger.Warn("ping write failed",
					logging.String("source_name", t.sourceName),
					logging.Err("error", err),
				)
				return
			}
		}
	}
}

// Recv blocks until a message is received from the WebSocket connection or the
// context is cancelled. Returns the raw message bytes.
func (t *WebSocketTransport) Recv(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	conn := t.conn
	t.mu.Unlock()

	if conn == nil {
		return nil, fmt.Errorf("websocket not connected")
	}

	// Use a goroutine to make ReadMessage cancellable via context.
	type readResult struct {
		data []byte
		err  error
	}
	ch := make(chan readResult, 1)

	go func() {
		_, data, err := conn.ReadMessage()
		ch <- readResult{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		// Context cancelled. Close the connection to unblock ReadMessage.
		t.mu.Lock()
		if t.conn == conn {
			_ = conn.Close()
			t.conn = nil
		}
		t.mu.Unlock()
		// Drain the read goroutine.
		<-ch
		return nil, ctx.Err()
	case res := <-ch:
		if res.err != nil {
			return nil, fmt.Errorf("websocket read: %w", res.err)
		}
		// Apply application-level decoding if configured.
		if t.spec != nil && t.spec.Decode == "lzw" {
			decoded, err := decodeLZW(res.data)
			if err != nil {
				return nil, fmt.Errorf("websocket lzw decode: %w", err)
			}
			return decoded, nil
		}
		return res.data, nil
	}
}

// Close gracefully shuts down the WebSocket connection.
// Sends a close frame, signals the ping goroutine to stop, and closes the
// underlying connection. Safe to call multiple times.
func (t *WebSocketTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Signal the ping goroutine to stop.
	if t.done != nil {
		select {
		case <-t.done:
			// Already closed.
		default:
			close(t.done)
		}
	}

	if t.conn == nil {
		return nil
	}

	conn := t.conn
	t.conn = nil

	// Attempt graceful close with a close frame.
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = conn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(closeGracePeriod))

	t.logger.Info("WebSocket connection closed",
		logging.String("source_name", t.sourceName),
	)

	return conn.Close()
}

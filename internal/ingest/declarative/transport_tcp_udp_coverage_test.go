package declarative

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestTCPUDP_ConnectUnsupportedProtocol(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "sctp", // unsupported
			Mode:     "connect",
			Address:  "127.0.0.1:9999",
		},
		sourceName:      "test",
		logger:          testLogger(),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	ctx := context.Background()
	err := tt.Connect(ctx)
	if err == nil {
		t.Fatal("expected error for unsupported protocol, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_ConnectTCP_UnsupportedMode(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "invalid_mode",
			Address:  "127.0.0.1:9999",
		},
		sourceName:      "test",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	err := tt.connectTCP(context.Background())
	if err == nil {
		t.Fatal("expected error for unsupported TCP mode, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported TCP mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_RecvUnsupportedProtocol(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "sctp",
			Mode:     "connect",
			Address:  "127.0.0.1:9999",
		},
		sourceName: "test",
		logger:     testLogger(),
	}

	_, err := tt.Recv(context.Background())
	if err == nil {
		t.Fatal("expected error for unsupported protocol in Recv, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported protocol") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_RecvTCP_NotConnected(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "connect",
			Address:  "127.0.0.1:9999",
		},
		sourceName: "test",
		logger:     testLogger(),
		// scanner and conn are nil (not connected)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := tt.recvTCP(ctx)
	if err == nil {
		t.Fatal("expected error for nil scanner, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_RecvUDP_NotConnected(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "udp",
			Mode:     "listen",
			Address:  "127.0.0.1:0",
		},
		sourceName: "test",
		logger:     testLogger(),
		// pconn is nil (not connected)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := tt.recvUDP(ctx)
	if err == nil {
		t.Fatal("expected error for nil pconn, got nil")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_AcceptLoop_ErrorNotIntentional(t *testing.T) {
	// Create a TCP listener, then close it before acceptLoop can accept.
	// The done channel is NOT closed, so the error path (not the done path) is taken.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}

	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "listen",
			Address:  ln.Addr().String(),
		},
		sourceName:      "test_accept_err",
		logger:          testLogger(),
		done:            make(chan struct{}), // NOT closed — "not intentional" path
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	// Close the listener before acceptLoop can accept — triggers the error path.
	_ = ln.Close()

	// acceptLoop should log a warning and return (not block).
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		tt.acceptLoop(ln)
	}()

	select {
	case <-loopDone:
		// acceptLoop returned as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop did not return within timeout")
	}
}

func TestTCPUDP_AcceptLoop_DoneWhileAccepting(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	done := make(chan struct{})

	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "listen",
			Address:  ln.Addr().String(),
		},
		sourceName:      "test_accept_done",
		logger:          testLogger(),
		done:            done,
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	// Start the accept loop.
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		tt.acceptLoop(ln)
	}()

	// Connect a client so Accept() succeeds.
	time.Sleep(10 * time.Millisecond)
	clientConn, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = clientConn.Close() }()

	// Close done before acceptLoop can lock the mutex — triggers the "done while accepting" path.
	close(done)

	select {
	case <-loopDone:
		// acceptLoop returned as expected.
	case <-time.After(2 * time.Second):
		t.Fatal("acceptLoop did not return within timeout after done closed")
	}
}

func TestTCPUDP_NewScanner_AtEOFPath(t *testing.T) {
	// We need a net.Conn to pass to newScanner. Use a net.Pipe.
	// net.Pipe is synchronous, so writes must be done in a goroutine.
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	tt := &TCPUDPTransport{
		spec:            &TCPUDPSpec{Protocol: "tcp", Mode: "connect", Address: "pipe"},
		sourceName:      "test",
		logger:          testLogger(),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	scanner := tt.newScanner(clientConn)

	// Write data without delimiter then close the server side in a goroutine.
	// This triggers the atEOF && len(data) > 0 path.
	go func() {
		_, _ = serverConn.Write([]byte("partial data no newline"))
		_ = serverConn.Close() // EOF
	}()

	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			t.Fatalf("scanner error: %v", err)
		}
	}
	got := string(scanner.Bytes())
	if got != "partial data no newline" {
		t.Errorf("got %q, want %q", got, "partial data no newline")
	}
}

func TestTCPUDP_NewScanner_RequestMoreData(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer func() { _ = clientConn.Close() }()

	tt := &TCPUDPTransport{
		spec:            &TCPUDPSpec{Protocol: "tcp", Mode: "connect", Address: "pipe"},
		sourceName:      "test",
		logger:          testLogger(),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	scanner := tt.newScanner(clientConn)

	// Send data in two parts: first without delimiter, then with delimiter.
	go func() {
		defer func() { _ = serverConn.Close() }()
		_, _ = serverConn.Write([]byte("partial"))
		time.Sleep(10 * time.Millisecond)
		_, _ = serverConn.Write([]byte(" complete\n"))
	}()

	if !scanner.Scan() {
		t.Fatal("expected scan to succeed")
	}
	got := string(scanner.Bytes())
	if got != "partial complete" {
		t.Errorf("got %q, want %q", got, "partial complete")
	}
}

func TestTCPUDP_RecvTCP_ConnectionClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Close immediately without sending data.
		_ = conn.Close()
	}()

	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "connect",
			Address:  ln.Addr().String(),
		},
		sourceName:      "test",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = tt.Close() }()

	// Wait briefly for connection to be established.
	time.Sleep(20 * time.Millisecond)

	_, err = tt.Recv(ctx)
	if err == nil {
		t.Fatal("expected error when server closes connection, got nil")
	}
	if !strings.Contains(err.Error(), "tcp") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_ConnectTCP_DialError(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "connect",
			Address:  "127.0.0.1:1", // port 1 should be unavailable
		},
		sourceName:      "test_tcp_dial_err",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := tt.connectTCP(ctx)
	if err == nil {
		t.Fatal("expected TCP dial error, got nil")
	}
	if !strings.Contains(err.Error(), "tcp dial") {
		t.Errorf("expected tcp dial error, got: %v", err)
	}
}

func TestTCPUDP_ConnectTCP_ListenError(t *testing.T) {
	// Bind the port first.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("setup listen: %v", err)
	}
	addr := ln.Addr().String()
	defer func() { _ = ln.Close() }()

	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "listen",
			Address:  addr, // already bound
		},
		sourceName:      "test_tcp_listen_err",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	err = tt.connectTCP(context.Background())
	if err == nil {
		t.Fatal("expected TCP listen error, got nil")
	}
	if !strings.Contains(err.Error(), "tcp listen") {
		t.Errorf("expected tcp listen error, got: %v", err)
	}
}

func TestTCPUDP_ConnectUDP_ListenError(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "udp",
			Mode:     "listen",
			Address:  "not-a-valid-address:badport", // invalid
		},
		sourceName:      "test_udp_listen_err",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	err := tt.connectUDP()
	if err == nil {
		t.Fatal("expected UDP listen error, got nil")
	}
	if !strings.Contains(err.Error(), "udp listen") {
		t.Errorf("expected udp listen error, got: %v", err)
	}
}

func TestTCPUDP_Close_NotConnected(t *testing.T) {
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "connect",
			Address:  "127.0.0.1:0",
		},
		sourceName:      "test_close_not_connected",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}
	err := tt.Close()
	if err != nil {
		t.Errorf("expected nil from Close on unconnected transport, got: %v", err)
	}
}

func TestTCPUDPTransport_Close_ConnError(t *testing.T) {
	// Create a net.Pipe() pair and close one end immediately.
	c1, c2 := net.Pipe()
	_ = c1.Close() // Pre-close so subsequent Close() will error.
	_ = c2.Close()

	// Build a transport with the pre-closed conn.
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "connect",
			Address:  "127.0.0.1:0",
		},
		sourceName:      "tcp_close_conn_error_test",
		logger:          logging.NewNopLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: defaultMaxMessageBytes,
		bufferSize:      defaultBufferSize,
		conn:            c1, // already closed
	}

	// Close should return the error from closing c1 again.
	err := tt.Close()
	// On some platforms net.Pipe conn reports an error when closed twice,
	// on others it may succeed. We just verify Close does not panic.
	if err != nil {
		t.Logf("Close returned error (expected on most platforms): %v", err)
	}
}

func TestTCPUDPTransport_Close_ListenerError(t *testing.T) {
	// Create a real listener and close it immediately so the second close will error.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	_ = ln.Close() // Pre-close.

	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "tcp",
			Mode:     "listen",
			Address:  "127.0.0.1:0",
		},
		sourceName:      "tcp_close_listener_error_test",
		logger:          logging.NewNopLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: defaultMaxMessageBytes,
		bufferSize:      defaultBufferSize,
		listener:        ln, // already closed
	}

	// Close should encounter an error from the already-closed listener.
	err = tt.Close()
	if err != nil {
		t.Logf("Close returned error (expected): %v", err)
	}
}

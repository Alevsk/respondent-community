package declarative

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestTCPUDP_RegisteredInFactory(t *testing.T) {
	transportMu.RLock()
	_, ok := transportConstructors["tcp_udp"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("tcp_udp transport not registered in factory")
	}
}

func TestTCPUDP_NewTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			// TCPUDP is nil
		},
	}

	_, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err == nil {
		t.Fatal("expected error for nil TCPUDP spec, got nil")
	}
	if !strings.Contains(err.Error(), "tcp_udp configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestTCPUDP_NewTransport_MissingProtocol(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Mode:    "connect",
				Address: "127.0.0.1:9999",
			},
		},
	}

	_, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err == nil {
		t.Fatal("expected error for missing protocol")
	}
	if !strings.Contains(err.Error(), "protocol") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_NewTransport_MissingMode(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Address:  "127.0.0.1:9999",
			},
		},
	}

	_, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err == nil {
		t.Fatal("expected error for missing mode")
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_NewTransport_MissingAddress(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "connect",
			},
		},
	}

	_, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err == nil {
		t.Fatal("expected error for missing address")
	}
	if !strings.Contains(err.Error(), "address") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTCPUDP_NewTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol:        "tcp",
				Mode:            "connect",
				Address:         "127.0.0.1:9999",
				Delimiter:       "|",
				MaxMessageBytes: 1024,
				BufferSize:      512,
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	if tt.sourceName != "test_tcp" {
		t.Errorf("expected source name test_tcp, got %s", tt.sourceName)
	}
	if string(tt.delimiter) != "|" {
		t.Errorf("expected delimiter |, got %q", string(tt.delimiter))
	}
	if tt.maxMessageBytes != 1024 {
		t.Errorf("expected maxMessageBytes 1024, got %d", tt.maxMessageBytes)
	}
	if tt.bufferSize != 512 {
		t.Errorf("expected bufferSize 512, got %d", tt.bufferSize)
	}
}

func TestTCPUDP_NewTransport_Defaults(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "connect",
				Address:  "127.0.0.1:9999",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	if string(tt.delimiter) != "\n" {
		t.Errorf("expected default delimiter \\n, got %q", string(tt.delimiter))
	}
	if tt.maxMessageBytes != defaultMaxMessageBytes {
		t.Errorf("expected default maxMessageBytes %d, got %d", defaultMaxMessageBytes, tt.maxMessageBytes)
	}
	if tt.bufferSize != defaultBufferSize {
		t.Errorf("expected default bufferSize %d, got %d", defaultBufferSize, tt.bufferSize)
	}
}

func TestTCPUDP_FetchReturnsErrNotPullBased(t *testing.T) {
	tr := &TCPUDPTransport{
		spec:   &TCPUDPSpec{Protocol: "tcp", Mode: "connect", Address: "127.0.0.1:0"},
		logger: testLogger(),
	}

	_, _, err := tr.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Fatalf("expected ErrNotPullBased, got %v", err)
	}
}

func TestTCPUDP_CloseIdempotent(t *testing.T) {
	tr := &TCPUDPTransport{
		spec:       &TCPUDPSpec{Protocol: "tcp", Mode: "connect", Address: "127.0.0.1:0"},
		sourceName: "test",
		logger:     testLogger(),
		done:       make(chan struct{}),
	}

	err := tr.Close()
	if err != nil {
		t.Fatalf("first Close() error: %v", err)
	}

	err = tr.Close()
	if err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestTCPUDP_CloseWithoutConnect(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_tcp",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "connect",
				Address:  "127.0.0.1:9999",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	err = tt.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestTCPUDP_TCPConnect_RecvMessages(t *testing.T) {
	// Start a test TCP server on an ephemeral port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test TCP server: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()

	// Server goroutine: accept one connection and send delimited messages.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()

		messages := []string{
			`{"id":1,"msg":"hello"}`,
			`{"id":2,"msg":"world"}`,
			`{"id":3,"msg":"done"}`,
		}
		for _, msg := range messages {
			_, _ = fmt.Fprintf(conn, "%s\n", msg)
		}
	}()

	// Create and connect the transport.
	def := &SourceDefinition{
		Name: "test_tcp_connect",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "connect",
				Address:  addr,
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	// Receive 3 messages.
	expected := []string{
		`{"id":1,"msg":"hello"}`,
		`{"id":2,"msg":"world"}`,
		`{"id":3,"msg":"done"}`,
	}

	for i, exp := range expected {
		data, err := tt.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() #%d error: %v", i, err)
		}
		if string(data) != exp {
			t.Errorf("Recv() #%d = %q, want %q", i, string(data), exp)
		}
	}

	<-serverDone
}

func TestTCPUDP_TCPConnect_CustomDelimiter(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test TCP server: %v", err)
	}
	defer func() { _ = ln.Close() }()

	addr := ln.Addr().String()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		_, _ = fmt.Fprint(conn, "msg1||msg2||msg3||")
	}()

	def := &SourceDefinition{
		Name: "test_tcp_delim",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol:  "tcp",
				Mode:      "connect",
				Address:   addr,
				Delimiter: "||",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	for i, exp := range []string{"msg1", "msg2", "msg3"} {
		data, err := tt.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() #%d error: %v", i, err)
		}
		if string(data) != exp {
			t.Errorf("Recv() #%d = %q, want %q", i, string(data), exp)
		}
	}
}

func TestTCPUDP_TCPListen_RecvMessages(t *testing.T) {
	// Use ephemeral port for the transport listener.
	def := &SourceDefinition{
		Name: "test_tcp_listen",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "listen",
				Address:  "127.0.0.1:0",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	// Get the actual listener address.
	tt.mu.Lock()
	listenAddr := tt.listener.Addr().String()
	tt.mu.Unlock()

	// Connect a client and send messages.
	conn, err := net.DialTimeout("tcp", listenAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to listener: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Give the accept loop time to set up the scanner.
	time.Sleep(100 * time.Millisecond)

	messages := []string{"line1", "line2"}
	for _, msg := range messages {
		_, _ = fmt.Fprintf(conn, "%s\n", msg)
	}

	for i, exp := range messages {
		data, err := tt.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() #%d error: %v", i, err)
		}
		if string(data) != exp {
			t.Errorf("Recv() #%d = %q, want %q", i, string(data), exp)
		}
	}
}

func TestTCPUDP_UDPListen_RecvDatagrams(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_udp_listen",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "udp",
				Mode:     "listen",
				Address:  "127.0.0.1:0",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	// Get the actual listener address.
	tt.mu.Lock()
	listenAddr := tt.pconn.LocalAddr().String()
	tt.mu.Unlock()

	// Send UDP datagrams to the listener.
	conn, err := net.Dial("udp", listenAddr)
	if err != nil {
		t.Fatalf("failed to create UDP sender: %v", err)
	}
	defer func() { _ = conn.Close() }()

	messages := []string{
		`{"sensor":"temp","value":42}`,
		`{"sensor":"humidity","value":65}`,
	}

	for _, msg := range messages {
		_, err := conn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("UDP send error: %v", err)
		}
	}

	for i, exp := range messages {
		data, err := tt.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv() #%d error: %v", i, err)
		}
		if string(data) != exp {
			t.Errorf("Recv() #%d = %q, want %q", i, string(data), exp)
		}
	}
}

func TestTCPUDP_RecvContextCancellation_TCP(t *testing.T) {
	// Start a TCP server that never sends anything.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test TCP server: %v", err)
	}
	defer func() { _ = ln.Close() }()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		// Hold connection open, never send data.
		<-time.After(10 * time.Second)
	}()

	def := &SourceDefinition{
		Name: "test_tcp_cancel",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "tcp",
				Mode:     "connect",
				Address:  ln.Addr().String(),
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connectCancel()

	if err := tt.Connect(connectCtx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err = tt.Recv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestTCPUDP_RecvContextCancellation_UDP(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_udp_cancel",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "udp",
				Mode:     "listen",
				Address:  "127.0.0.1:0",
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	connectCtx, connectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer connectCancel()

	if err := tt.Connect(connectCtx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}
	defer func() { _ = tt.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err = tt.Recv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

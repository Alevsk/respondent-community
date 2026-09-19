package declarative

import (
	"context"
	"testing"
	"time"
)

// TestTCPUDP_ConnectUDP_ConnectMode verifies UDP "connect" mode creates a packet connection.
func TestTCPUDP_ConnectUDP_ConnectMode(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_udp_connect",
		Transport: TransportSpec{
			Type: "tcp_udp",
			TCPUDP: &TCPUDPSpec{
				Protocol: "udp",
				Mode:     "connect",
				Address:  "127.0.0.1:9999", // Doesn't need to exist for UDP connect mode
			},
		},
	}

	tr, err := newTCPUDPTransport(def, nil, nil, nil, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tt := tr.(*TCPUDPTransport)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// In "connect" mode, UDP binds to :0 (ephemeral port) without contacting the remote.
	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error in UDP connect mode: %v", err)
	}
	defer func() { _ = tt.Close() }()

	// Verify that a packet connection was established.
	tt.mu.Lock()
	pconn := tt.pconn
	tt.mu.Unlock()

	if pconn == nil {
		t.Error("expected non-nil pconn after UDP connect mode")
	}
}

// TestTCPUDP_ConnectUDP_UnsupportedMode verifies that an unsupported UDP mode returns an error.
func TestTCPUDP_ConnectUDP_UnsupportedMode(t *testing.T) {
	// Create a transport with an unsupported UDP mode.
	tt := &TCPUDPTransport{
		spec: &TCPUDPSpec{
			Protocol: "udp",
			Mode:     "invalid_mode", // unsupported
			Address:  "127.0.0.1:9999",
		},
		sourceName:      "test",
		logger:          testLogger(),
		done:            make(chan struct{}),
		delimiter:       []byte("\n"),
		maxMessageBytes: 65536,
		bufferSize:      4096,
	}

	ctx := context.Background()
	err := tt.connectUDP()
	_ = ctx
	if err == nil {
		t.Fatal("expected error for unsupported UDP mode, got nil")
	}
}

// TestTCPUDP_AcceptLoop_CloseWhileWaiting verifies acceptLoop exits cleanly when closed.
func TestTCPUDP_AcceptLoop_CloseWhileWaiting(t *testing.T) {
	// Start a TCP server in "listen" mode then immediately close to trigger
	// the "done" channel path in acceptLoop.
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Connect starts the accept loop in a goroutine.
	if err := tt.Connect(ctx); err != nil {
		t.Fatalf("Connect() error: %v", err)
	}

	// Close immediately to trigger the "done" channel path in acceptLoop.
	if err := tt.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

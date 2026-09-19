package declarative

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"github.com/Alevsk/respondent/internal/logging"
)

// --- Constructor validation tests ---

func TestGRPCStreamTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type:       "grpc_stream",
			GRPCStream: nil,
		},
	}
	logger := logging.NewNopLogger()

	_, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err == nil {
		t.Fatal("expected error for nil grpc_stream spec, got nil")
	}
}

func TestGRPCStreamTransport_MissingAddress(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "",
				Service: "test.Service",
				Method:  "StreamData",
			},
		},
	}
	logger := logging.NewNopLogger()

	_, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err == nil {
		t.Fatal("expected error for missing address, got nil")
	}
}

func TestGRPCStreamTransport_MissingService(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "",
				Method:  "StreamData",
			},
		},
	}
	logger := logging.NewNopLogger()

	_, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err == nil {
		t.Fatal("expected error for missing service, got nil")
	}
}

func TestGRPCStreamTransport_MissingMethod(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.Service",
				Method:  "",
			},
		},
	}
	logger := logging.NewNopLogger()

	_, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err == nil {
		t.Fatal("expected error for missing method, got nil")
	}
}

func TestGRPCStreamTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.Service",
				Method:  "StreamData",
			},
		},
	}
	logger := logging.NewNopLogger()

	transport, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	_, _, fetchErr := transport.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(fetchErr, ErrNotPullBased) {
		t.Errorf("Fetch error = %v, want ErrNotPullBased", fetchErr)
	}
}

// --- Close idempotency tests ---

func TestGRPCStreamTransport_CloseBeforeConnect(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.Service",
				Method:  "StreamData",
			},
		},
	}
	logger := logging.NewNopLogger()

	transport, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	st := transport.(StreamTransport)
	if err := st.Close(); err != nil {
		t.Errorf("Close before Connect error: %v", err)
	}
}

func TestGRPCStreamTransport_CloseTwice(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.Service",
				Method:  "StreamData",
			},
		},
	}
	logger := logging.NewNopLogger()

	transport, err := newGRPCStreamTransport(def, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	st := transport.(StreamTransport)
	if err := st.Close(); err != nil {
		t.Errorf("first Close error: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
}

// --- Integration tests with bufconn ---

const bufConnSize = 1024 * 1024

// testStreamServiceServer is the interface the gRPC server checks against.
type testStreamServiceServer interface{}

// testGRPCServer is a minimal gRPC server that handles raw bytes for testing.
// It registers a service handler using the raw codec so we can stream arbitrary data.
type testGRPCServer struct {
	messages [][]byte
}

// startTestGRPCServer creates an in-memory gRPC server using bufconn that streams
// the provided messages and then closes the stream.
// Returns a bufconn listener and a cleanup function.
func startTestGRPCServer(t *testing.T, messages [][]byte) *bufconn.Listener {
	t.Helper()

	lis := bufconn.Listen(bufConnSize)

	srv := grpc.NewServer(grpc.ForceServerCodec(rawCodecV2{}))

	// Register a handler for our test service.
	streamDesc := grpc.StreamDesc{
		StreamName:    "StreamData",
		ServerStreams: true,
	}
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.StreamService",
		HandlerType: (*testStreamServiceServer)(nil),
		Methods:     []grpc.MethodDesc{},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    streamDesc.StreamName,
				Handler:       makeStreamHandler(messages),
				ServerStreams: true,
				ClientStreams: false,
			},
		},
		Metadata: "",
	}, &testGRPCServer{messages: messages})

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Listener closed is expected on shutdown.
			return
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
	})

	return lis
}

// rawCodecV2 implements grpc/encoding.CodecV2 for the server side.
type rawCodecV2 struct{}

func (rawCodecV2) Marshal(v any) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		bp, okp := v.(*[]byte)
		if !okp {
			return nil, io.ErrUnexpectedEOF
		}
		return *bp, nil
	}
	return b, nil
}

func (rawCodecV2) Unmarshal(data []byte, v any) error {
	ptr, ok := v.(*[]byte)
	if !ok {
		return io.ErrUnexpectedEOF
	}
	*ptr = data
	return nil
}

func (rawCodecV2) Name() string { return "raw" }

// makeStreamHandler returns a gRPC stream handler that sends the given messages.
func makeStreamHandler(messages [][]byte) func(srv interface{}, stream grpc.ServerStream) error {
	return func(_ interface{}, stream grpc.ServerStream) error {
		// Read the request message (the client sends one before CloseSend).
		var req []byte
		if err := stream.RecvMsg(&req); err != nil && err != io.EOF {
			return err
		}

		for _, msg := range messages {
			if err := stream.SendMsg(msg); err != nil {
				return err
			}
		}
		return nil
	}
}

// bufDialer returns a grpc.DialOption that connects via the bufconn listener.
func bufDialer(lis *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}
}

func TestGRPCStreamTransport_ConnectAndRecv(t *testing.T) {
	messages := [][]byte{
		[]byte(`{"id":1,"name":"alpha"}`),
		[]byte(`{"id":2,"name":"beta"}`),
		[]byte(`{"id":3,"name":"gamma"}`),
	}

	lis := startTestGRPCServer(t, messages)

	// Create the transport manually to inject the bufconn dialer.
	logger := logging.NewNopLogger()
	transport := &GRPCStreamTransport{
		spec: &GRPCStreamSpec{
			Address:     "bufnet",
			Service:     "test.StreamService",
			Method:      "StreamData",
			RequestJSON: `{}`,
		},
		sourceName: "test_grpc_source",
		logger:     logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect using bufconn dialer.
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(bufDialer(lis)),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}

	fullMethod := "/test.StreamService/StreamData"
	streamDesc := &grpc.StreamDesc{
		StreamName:    "StreamData",
		ServerStreams: true,
	}

	stream, err := conn.NewStream(ctx, streamDesc, fullMethod, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	// Send request and close send.
	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg failed: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend failed: %v", err)
	}

	// Inject the connection and stream into the transport.
	transport.conn = conn
	transport.stream = stream
	streamCtx, streamCancel := context.WithCancel(ctx)
	transport.cancel = streamCancel
	_ = streamCtx

	defer func() { _ = transport.Close() }()

	// Read all messages.
	for i, want := range messages {
		data, err := transport.Recv(ctx)
		if err != nil {
			t.Fatalf("Recv[%d] failed: %v", i, err)
		}
		if string(data) != string(want) {
			t.Errorf("Recv[%d] = %q, want %q", i, string(data), string(want))
		}
	}

	// Next Recv should indicate stream end.
	_, err = transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error after all messages consumed, got nil")
	}
}

func TestGRPCStreamTransport_RecvNotConnected(t *testing.T) {
	logger := logging.NewNopLogger()
	transport := &GRPCStreamTransport{
		spec: &GRPCStreamSpec{
			Address: "localhost:50051",
			Service: "test.Service",
			Method:  "StreamData",
		},
		sourceName: "test_grpc",
		logger:     logger,
	}

	ctx := context.Background()
	_, err := transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from Recv without Connect, got nil")
	}
}

func TestGRPCStreamTransport_EmptyStream(t *testing.T) {
	// Server sends zero messages.
	lis := startTestGRPCServer(t, nil)

	logger := logging.NewNopLogger()
	transport := &GRPCStreamTransport{
		spec: &GRPCStreamSpec{
			Address: "bufnet",
			Service: "test.StreamService",
			Method:  "StreamData",
		},
		sourceName: "test_grpc_empty",
		logger:     logger,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(bufDialer(lis)),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}

	fullMethod := "/test.StreamService/StreamData"
	stream, err := conn.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "StreamData",
		ServerStreams: true,
	}, fullMethod, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg failed: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend failed: %v", err)
	}

	transport.conn = conn
	transport.stream = stream
	_, streamCancel := context.WithCancel(ctx)
	transport.cancel = streamCancel

	defer func() { _ = transport.Close() }()

	// Should get EOF immediately.
	_, err = transport.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from empty stream, got nil")
	}
}

func TestGRPCStreamTransport_ViaFactory(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc_factory",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.StreamService",
				Method:  "StreamData",
				Metadata: map[string]string{
					"x-api-key": "test-key",
				},
			},
		},
	}

	logger := logging.NewNopLogger()
	transport, err := NewTransport(def, nil, nil, nil, nil, logger)
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}

	gst, ok := transport.(*GRPCStreamTransport)
	if !ok {
		t.Fatalf("expected *GRPCStreamTransport, got %T", transport)
	}
	if gst.spec.Address != "localhost:50051" {
		t.Errorf("address = %q, want %q", gst.spec.Address, "localhost:50051")
	}
	if gst.spec.Service != "test.StreamService" {
		t.Errorf("service = %q, want %q", gst.spec.Service, "test.StreamService")
	}
	if gst.spec.Method != "StreamData" {
		t.Errorf("method = %q, want %q", gst.spec.Method, "StreamData")
	}
	if gst.spec.Metadata["x-api-key"] != "test-key" {
		t.Errorf("metadata x-api-key = %q, want %q", gst.spec.Metadata["x-api-key"], "test-key")
	}
}

// --- Additional tests matching requested coverage ---

func TestGRPCStreamTransport_NewGRPCStreamTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address:     "grpc.example.com:443",
				Service:     "mypackage.DataService",
				Method:      "StreamUpdates",
				RequestJSON: `{"filter":"active"}`,
				UseTLS:      false,
				Metadata: map[string]string{
					"authorization": "Bearer tok123",
				},
			},
		},
	}

	transport, err := newGRPCStreamTransport(def, nil, nil, nil, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	// Verify it implements StreamTransport.
	st, ok := transport.(StreamTransport)
	if !ok {
		t.Fatal("expected StreamTransport implementation")
	}
	_ = st

	// Verify stored config via type assertion.
	gst := transport.(*GRPCStreamTransport)
	if gst.spec.Address != "grpc.example.com:443" {
		t.Errorf("address = %q, want %q", gst.spec.Address, "grpc.example.com:443")
	}
	if gst.spec.RequestJSON != `{"filter":"active"}` {
		t.Errorf("request_json mismatch: %q", gst.spec.RequestJSON)
	}
	if gst.sourceName != "test_grpc" {
		t.Errorf("sourceName = %q, want %q", gst.sourceName, "test_grpc")
	}
}

func TestGRPCStreamTransport_NewGRPCStreamTransport_MissingSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    *GRPCStreamSpec
		wantErr string
	}{
		{
			name:    "nil spec",
			spec:    nil,
			wantErr: "grpc_stream configuration",
		},
		{
			name: "empty address",
			spec: &GRPCStreamSpec{
				Service: "svc",
				Method:  "m",
			},
			wantErr: "address is required",
		},
		{
			name: "empty service",
			spec: &GRPCStreamSpec{
				Address: "localhost:50051",
				Method:  "m",
			},
			wantErr: "service is required",
		},
		{
			name: "empty method",
			spec: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "svc",
			},
			wantErr: "method is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			def := &SourceDefinition{
				Name: "test_grpc",
				Transport: TransportSpec{
					Type:       "grpc_stream",
					GRPCStream: tc.spec,
				},
			}

			_, err := newGRPCStreamTransport(def, nil, nil, nil, logging.NewNopLogger())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestGRPCStreamTransport_CloseIdempotent(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_grpc",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "localhost:50051",
				Service: "test.Service",
				Method:  "StreamData",
			},
		},
	}

	transport, err := newGRPCStreamTransport(def, nil, nil, nil, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("constructor failed: %v", err)
	}

	st := transport.(StreamTransport)

	// Close before any Connect.
	if err := st.Close(); err != nil {
		t.Errorf("first Close error: %v", err)
	}
	// Close again -- must not error or panic.
	if err := st.Close(); err != nil {
		t.Errorf("second Close error: %v", err)
	}
	// Third time for good measure.
	if err := st.Close(); err != nil {
		t.Errorf("third Close error: %v", err)
	}
}

func TestGRPCStreamTransport_RegisteredInFactory(t *testing.T) {
	transportMu.RLock()
	_, ok := transportConstructors["grpc_stream"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("grpc_stream transport not registered in factory")
	}

	// Verify the constructor produces a working transport.
	def := &SourceDefinition{
		Name: "test_factory",
		Transport: TransportSpec{
			Type: "grpc_stream",
			GRPCStream: &GRPCStreamSpec{
				Address: "host:1234",
				Service: "pkg.Svc",
				Method:  "Stream",
			},
		},
	}

	transport, err := NewTransport(def, nil, nil, nil, nil, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewTransport failed: %v", err)
	}

	if _, ok := transport.(StreamTransport); !ok {
		t.Fatalf("expected StreamTransport, got %T", transport)
	}
}

func TestGRPCStreamTransport_Metadata(t *testing.T) {
	// Start a test server that captures incoming metadata.
	capturedMD := make(chan metadata.MD, 1)

	lis := bufconn.Listen(bufConnSize)
	srv := grpc.NewServer(grpc.ForceServerCodec(rawCodecV2{}))
	srv.RegisterService(&grpc.ServiceDesc{
		ServiceName: "test.MetadataService",
		HandlerType: (*testStreamServiceServer)(nil),
		Methods:     []grpc.MethodDesc{},
		Streams: []grpc.StreamDesc{
			{
				StreamName:    "StreamData",
				ServerStreams: true,
				Handler: func(_ interface{}, stream grpc.ServerStream) error {
					// Capture metadata.
					md, _ := metadata.FromIncomingContext(stream.Context())
					capturedMD <- md

					// Read request.
					var req []byte
					if err := stream.RecvMsg(&req); err != nil && err != io.EOF {
						return err
					}

					// Send one response.
					return stream.SendMsg([]byte(`{"ok":true}`))
				},
			},
		},
		Metadata: "",
	}, &testGRPCServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.GracefulStop()
	})

	// Create a transport with metadata.
	transport := &GRPCStreamTransport{
		spec: &GRPCStreamSpec{
			Address: "bufnet",
			Service: "test.MetadataService",
			Method:  "StreamData",
			Metadata: map[string]string{
				"x-request-id": "req-42",
				"x-api-key":    "secret-key",
			},
			RequestJSON: `{}`,
		},
		sourceName: "test_metadata",
		logger:     logging.NewNopLogger(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect using bufconn dialer.
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(bufDialer(lis)),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient failed: %v", err)
	}

	fullMethod := "/test.MetadataService/StreamData"

	// Attach metadata to outgoing context.
	md := metadata.New(transport.spec.Metadata)
	mdCtx := metadata.NewOutgoingContext(ctx, md)

	stream, err := conn.NewStream(mdCtx, &grpc.StreamDesc{
		StreamName:    "StreamData",
		ServerStreams: true,
	}, fullMethod, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		t.Fatalf("NewStream failed: %v", err)
	}

	if err := stream.SendMsg([]byte(`{}`)); err != nil {
		t.Fatalf("SendMsg failed: %v", err)
	}
	if err := stream.CloseSend(); err != nil {
		t.Fatalf("CloseSend failed: %v", err)
	}

	transport.conn = conn
	transport.stream = stream
	_, streamCancel := context.WithCancel(ctx)
	transport.cancel = streamCancel

	defer func() { _ = transport.Close() }()

	// Read the response message.
	data, err := transport.Recv(ctx)
	if err != nil {
		t.Fatalf("Recv failed: %v", err)
	}
	if string(data) != `{"ok":true}` {
		t.Errorf("Recv = %q, want %q", string(data), `{"ok":true}`)
	}

	// Verify server received our metadata.
	select {
	case serverMD := <-capturedMD:
		if vals := serverMD.Get("x-request-id"); len(vals) == 0 || vals[0] != "req-42" {
			t.Errorf("server x-request-id = %v, want [req-42]", vals)
		}
		if vals := serverMD.Get("x-api-key"); len(vals) == 0 || vals[0] != "secret-key" {
			t.Errorf("server x-api-key = %v, want [secret-key]", vals)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for server metadata")
	}
}

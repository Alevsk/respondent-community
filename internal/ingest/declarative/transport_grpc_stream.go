package declarative

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding"
	"google.golang.org/grpc/metadata"

	"github.com/Alevsk/respondent/internal/logging"
)

// Compile-time assertion that GRPCStreamTransport implements StreamTransport.
var _ StreamTransport = (*GRPCStreamTransport)(nil)

func init() {
	RegisterTransport("grpc_stream", newGRPCStreamTransport)
}

// rawCodec is a gRPC codec that passes raw bytes without any serialization.
// This allows the transport to work generically without knowing the protobuf schema.
type rawCodec struct{}

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	b, ok := v.([]byte)
	if !ok {
		return nil, fmt.Errorf("rawCodec: expected []byte, got %T", v)
	}
	return b, nil
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	ptr, ok := v.(*[]byte)
	if !ok {
		return fmt.Errorf("rawCodec: expected *[]byte, got %T", v)
	}
	*ptr = data
	return nil
}

func (rawCodec) Name() string { return "raw" }

// Ensure rawCodec satisfies the encoding.Codec interface.
var _ encoding.Codec = rawCodec{}

// GRPCStreamTransport implements StreamTransport for gRPC server-streaming sources.
// It dials a gRPC server and reads raw bytes from a server-streaming RPC,
// allowing generic ingestion without compile-time proto knowledge.
type GRPCStreamTransport struct {
	spec       *GRPCStreamSpec
	sourceName string
	logger     *logging.Logger
	tlsConfig  *tls.Config

	mu     sync.Mutex
	conn   *grpc.ClientConn
	stream grpc.ClientStream
	cancel context.CancelFunc
}

// newGRPCStreamTransport creates a GRPCStreamTransport from a source definition.
func newGRPCStreamTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, _ EnvResolver, logger *logging.Logger) (Transport, error) {
	spec := def.Transport.GRPCStream
	if spec == nil {
		return nil, fmt.Errorf("grpc_stream transport requires grpc_stream configuration")
	}

	if spec.Address == "" {
		return nil, fmt.Errorf("grpc_stream: address is required")
	}
	if spec.Service == "" {
		return nil, fmt.Errorf("grpc_stream: service is required")
	}
	if spec.Method == "" {
		return nil, fmt.Errorf("grpc_stream: method is required")
	}

	var tlsCfg *tls.Config
	if spec.UseTLS {
		var err error
		tlsCfg, err = buildTLSConfig(spec.TLS)
		if err != nil {
			return nil, fmt.Errorf("grpc_stream TLS config: %w", err)
		}
		// When UseTLS is true but no TLS spec is provided, use default TLS.
		if tlsCfg == nil {
			tlsCfg = &tls.Config{MinVersion: tls.VersionTLS12}
		}
	}

	return &GRPCStreamTransport{
		spec:       spec,
		sourceName: def.Name,
		logger:     logger,
		tlsConfig:  tlsCfg,
	}, nil
}

// Fetch returns ErrNotPullBased because gRPC streaming is a push-based transport.
func (t *GRPCStreamTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect dials the gRPC server and opens a server-streaming RPC.
// If RequestJSON is provided, it is sent as the request message.
// Safe to call after Close for reconnection.
func (t *GRPCStreamTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Build dial options.
	var dialOpts []grpc.DialOption
	if t.tlsConfig != nil {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(t.tlsConfig)))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	t.logger.Info("connecting to gRPC server",
		logging.String("address", t.spec.Address),
		logging.String("service", t.spec.Service),
		logging.String("method", t.spec.Method),
		logging.String("source_name", t.sourceName),
	)

	conn, err := grpc.NewClient(t.spec.Address, dialOpts...)
	if err != nil {
		return fmt.Errorf("grpc_stream dial %q: %w", t.spec.Address, err)
	}

	// Build the full method name: /service/method
	fullMethod := fmt.Sprintf("/%s/%s", t.spec.Service, t.spec.Method)

	// Create a long-lived context for the stream that we can cancel on Close.
	streamCtx, streamCancel := context.WithCancel(ctx)

	// Attach gRPC metadata if configured.
	if len(t.spec.Metadata) > 0 {
		md := metadata.New(t.spec.Metadata)
		streamCtx = metadata.NewOutgoingContext(streamCtx, md)
	}

	streamDesc := &grpc.StreamDesc{
		StreamName:    t.spec.Method,
		ServerStreams: true,
	}

	stream, err := conn.NewStream(streamCtx, streamDesc, fullMethod, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		streamCancel()
		_ = conn.Close()
		return fmt.Errorf("grpc_stream new stream %q: %w", fullMethod, err)
	}

	// Send request message if provided.
	if t.spec.RequestJSON != "" {
		if err := stream.SendMsg([]byte(t.spec.RequestJSON)); err != nil {
			streamCancel()
			_ = conn.Close()
			return fmt.Errorf("grpc_stream send request: %w", err)
		}
	}

	// Close the send direction; this is a server-streaming RPC.
	if err := stream.CloseSend(); err != nil {
		streamCancel()
		_ = conn.Close()
		return fmt.Errorf("grpc_stream close send: %w", err)
	}

	t.conn = conn
	t.stream = stream
	t.cancel = streamCancel

	t.logger.Info("gRPC stream connected",
		logging.String("address", t.spec.Address),
		logging.String("method", fullMethod),
		logging.String("source_name", t.sourceName),
	)

	return nil
}

// Recv blocks until a message is received from the gRPC stream or the
// context is cancelled. Returns the raw message bytes.
// The stream's context (created in Connect) is used for cancellation;
// RecvMsg unblocks when that context is cancelled via Close().
func (t *GRPCStreamTransport) Recv(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	stream := t.stream
	t.mu.Unlock()

	if stream == nil {
		return nil, fmt.Errorf("grpc_stream not connected")
	}

	// RecvMsg blocks until a message arrives or the stream's context is cancelled.
	// The stream context (created in Connect) is a child of the caller's context,
	// so cancelling either will unblock this call.
	var msg []byte
	err := stream.RecvMsg(&msg)
	if err != nil {
		// Check if the caller's context was cancelled.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if err == io.EOF {
			return nil, fmt.Errorf("grpc_stream: server closed stream: %w", io.EOF)
		}
		return nil, fmt.Errorf("grpc_stream recv: %w", err)
	}
	return msg, nil
}

// Close gracefully shuts down the gRPC stream and connection.
// Safe to call multiple times and before Connect.
func (t *GRPCStreamTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel != nil {
		t.cancel()
		t.cancel = nil
	}

	t.stream = nil

	if t.conn == nil {
		return nil
	}

	conn := t.conn
	t.conn = nil

	t.logger.Info("gRPC stream connection closed",
		logging.String("source_name", t.sourceName),
	)

	return conn.Close()
}

package declarative

import (
	"context"
	"strings"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestBuildKafkaSASL_UnsupportedMechanism(t *testing.T) {
	spec := &KafkaSASLSpec{
		Mechanism: "KERBEROS",
		Username:  "user",
		Password:  "pass",
	}
	_, err := buildKafkaSASL(spec, func(s string) string { return s })
	if err == nil {
		t.Fatal("expected error for unsupported mechanism, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported SASL mechanism") {
		t.Errorf("expected 'unsupported SASL mechanism', got: %v", err)
	}
}

func TestBuildKafkaSASL_PLAIN(t *testing.T) {
	spec := &KafkaSASLSpec{
		Mechanism: "PLAIN",
		Username:  "test-user",
		Password:  "test-pass",
	}
	mech, err := buildKafkaSASL(spec, func(s string) string { return s })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mech == nil {
		t.Fatal("expected non-nil SASL mechanism")
	}
}

func TestBuildKafkaSASL_SCRAM256(t *testing.T) {
	spec := &KafkaSASLSpec{
		Mechanism: "SCRAM-SHA-256",
		Username:  "user",
		Password:  "pass",
	}
	mech, err := buildKafkaSASL(spec, func(s string) string { return s })
	if err != nil {
		t.Fatalf("unexpected SCRAM-256 error: %v", err)
	}
	if mech == nil {
		t.Fatal("expected non-nil SCRAM-256 mechanism")
	}
}

func TestBuildKafkaSASL_SCRAM512(t *testing.T) {
	spec := &KafkaSASLSpec{
		Mechanism: "SCRAM-SHA-512",
		Username:  "user",
		Password:  "pass",
	}
	mech, err := buildKafkaSASL(spec, func(s string) string { return s })
	if err != nil {
		t.Fatalf("unexpected SCRAM-512 error: %v", err)
	}
	if mech == nil {
		t.Fatal("expected non-nil SCRAM-512 mechanism")
	}
}

func TestKafkaTransport_Recv_NilReader(t *testing.T) {
	tr := &KafkaTransport{
		spec: &KafkaSpec{
			Brokers: []string{"localhost:9092"},
			Topic:   "test-topic",
			GroupID: "test-group",
		},
		sourceName: "kafka_nil_reader_test",
		logger:     logging.NewNopLogger(),
		// reader is nil — Connect has not been called
	}

	ctx := context.Background()
	_, err := tr.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from nil reader, got nil")
	}
	if err.Error() != "kafka reader not connected" {
		t.Errorf("expected 'kafka reader not connected', got: %v", err)
	}
}

func TestKafkaTransport_Recv_CancelledContext(t *testing.T) {
	tr := &KafkaTransport{
		spec: &KafkaSpec{
			Brokers: []string{"localhost:9092"},
			Topic:   "test-topic",
			GroupID: "test-group",
		},
		sourceName: "kafka_cancel_test",
		logger:     logging.NewNopLogger(),
		readerCfg: kafka.ReaderConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "test-topic",
			GroupID: "test-group",
		},
	}

	// Connect creates the reader without making network connections.
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer func() { _ = tr.Close() }()

	// A pre-cancelled context causes ReadMessage to return immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := tr.Recv(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// The error should wrap the cancellation.
	t.Logf("Got expected error: %v", err)
}

func TestKafkaTransport_Close_AfterConnect(t *testing.T) {
	tr := &KafkaTransport{
		spec: &KafkaSpec{
			Brokers: []string{"localhost:9092"},
			Topic:   "test-topic",
			GroupID: "test-group",
		},
		sourceName: "kafka_close_after_connect_test",
		logger:     logging.NewNopLogger(),
		readerCfg: kafka.ReaderConfig{
			Brokers: []string{"localhost:9092"},
			Topic:   "test-topic",
			GroupID: "test-group",
		},
	}

	// Connect creates a kafka.Reader (no network connections).
	if err := tr.Connect(context.Background()); err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// Close should succeed (exercises the reader.Close() path).
	if err := tr.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Idempotent: second close should be no-op.
	if err := tr.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestBuildKafkaDialer_TLSError(t *testing.T) {
	spec := &KafkaSpec{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
		GroupID: "test-group",
		TLS: &TLSSpec{
			CACert: "/nonexistent/ca-cert.pem",
		},
	}
	_, err := buildKafkaDialer(spec, func(s string) string { return s })
	if err == nil {
		t.Fatal("expected error from invalid TLS config, got nil")
	}
	if !strings.Contains(err.Error(), "build TLS config") {
		t.Errorf("expected 'build TLS config', got: %v", err)
	}
}

func TestNewKafkaTransport_BuildDialerError(t *testing.T) {
	def := &SourceDefinition{
		Name:      "kafka_dialer_error_test",
		LayerType: "test_layer",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				TLS: &TLSSpec{
					CACert: "/nonexistent/ca-cert.pem",
				},
			},
		},
	}

	_, err := newKafkaTransport(def, nil, nil, func(s string) string { return s }, logging.NewNopLogger())
	if err == nil {
		t.Fatal("expected error from build dialer, got nil")
	}
	if !strings.Contains(err.Error(), "build kafka dialer") {
		t.Errorf("expected 'build kafka dialer', got: %v", err)
	}
}

func TestBuildKafkaDialer_SASLError(t *testing.T) {
	spec := &KafkaSpec{
		Brokers: []string{"localhost:9092"},
		Topic:   "test-topic",
		GroupID: "test-group",
		SASL: &KafkaSASLSpec{
			Mechanism: "UNSUPPORTED-MECH",
			Username:  "user",
			Password:  "pass",
		},
	}
	_, err := buildKafkaDialer(spec, func(s string) string { return s })
	if err == nil {
		t.Fatal("expected error from unsupported SASL mechanism, got nil")
	}
	if !strings.Contains(err.Error(), "build SASL mechanism") {
		t.Errorf("expected 'build SASL mechanism', got: %v", err)
	}
}

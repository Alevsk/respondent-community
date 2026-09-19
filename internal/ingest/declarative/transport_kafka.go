package declarative

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/Alevsk/respondent/internal/logging"
)

// Compile-time assertion that KafkaTransport implements StreamTransport.
var _ StreamTransport = (*KafkaTransport)(nil)

func init() {
	RegisterTransport("kafka", newKafkaTransport)
}

// KafkaTransport implements StreamTransport for Kafka consumer sources.
// It wraps a segmentio/kafka-go Reader to consume messages from a topic
// within a consumer group.
type KafkaTransport struct {
	spec       *KafkaSpec
	sourceName string
	logger     *logging.Logger

	readerCfg kafka.ReaderConfig

	mu     sync.Mutex
	reader *kafka.Reader
	closed bool
}

// newKafkaTransport creates a KafkaTransport from a source definition.
func newKafkaTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.Kafka == nil {
		return nil, fmt.Errorf("kafka transport requires transport.kafka configuration")
	}

	spec := def.Transport.Kafka

	if len(spec.Brokers) == 0 {
		return nil, fmt.Errorf("kafka transport requires at least one broker")
	}
	if spec.Topic == "" {
		return nil, fmt.Errorf("kafka transport requires a topic")
	}
	if spec.GroupID == "" {
		return nil, fmt.Errorf("kafka transport requires a group_id")
	}

	// Build the dialer for SASL and TLS.
	dialer, err := buildKafkaDialer(spec, envResolve)
	if err != nil {
		return nil, fmt.Errorf("build kafka dialer: %w", err)
	}

	// Map start offset.
	startOffset := kafka.FirstOffset
	switch spec.StartOffset {
	case "latest":
		startOffset = kafka.LastOffset
	case "earliest", "":
		startOffset = kafka.FirstOffset
	}

	cfg := kafka.ReaderConfig{
		Brokers:     spec.Brokers,
		Topic:       spec.Topic,
		GroupID:     spec.GroupID,
		Dialer:      dialer,
		StartOffset: startOffset,
	}

	if spec.MaxBytes > 0 {
		cfg.MaxBytes = spec.MaxBytes
	}
	if spec.CommitInterval.Duration > 0 {
		cfg.CommitInterval = spec.CommitInterval.Duration
	}

	return &KafkaTransport{
		spec:       spec,
		sourceName: def.Name,
		logger:     logger,
		readerCfg:  cfg,
	}, nil
}

// buildKafkaDialer constructs a kafka.Dialer with optional SASL and TLS settings.
func buildKafkaDialer(spec *KafkaSpec, envResolve EnvResolver) (*kafka.Dialer, error) {
	// If no SASL or TLS, no custom dialer is needed.
	if spec.SASL == nil && spec.TLS == nil {
		return nil, nil
	}

	dialer := &kafka.Dialer{}

	// Configure TLS.
	if spec.TLS != nil {
		tlsCfg, err := buildTLSConfig(spec.TLS)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		dialer.TLS = tlsCfg
	} else if spec.SASL != nil {
		// SASL typically requires TLS; provide a default config.
		dialer.TLS = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	// Configure SASL.
	if spec.SASL != nil {
		mechanism, err := buildKafkaSASL(spec.SASL, envResolve)
		if err != nil {
			return nil, fmt.Errorf("build SASL mechanism: %w", err)
		}
		dialer.SASLMechanism = mechanism
	}

	return dialer, nil
}

// buildKafkaSASL creates the appropriate sasl.Mechanism from the spec.
func buildKafkaSASL(saslSpec *KafkaSASLSpec, envResolve EnvResolver) (sasl.Mechanism, error) {
	username := envResolve(saslSpec.Username)
	password := envResolve(saslSpec.Password)

	switch saslSpec.Mechanism {
	case "PLAIN":
		return &plain.Mechanism{
			Username: username,
			Password: password,
		}, nil
	case "SCRAM-SHA-256":
		return scram.Mechanism(scram.SHA256, username, password)
	case "SCRAM-SHA-512":
		return scram.Mechanism(scram.SHA512, username, password)
	default:
		return nil, fmt.Errorf("unsupported SASL mechanism: %q", saslSpec.Mechanism)
	}
}

// Fetch returns ErrNotPullBased because Kafka is a push-based (streaming) transport.
func (t *KafkaTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect creates the Kafka reader. The actual TCP connection to brokers is
// established lazily on the first ReadMessage call.
func (t *KafkaTransport) Connect(_ context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.reader = kafka.NewReader(t.readerCfg)
	t.closed = false

	t.logger.Info("kafka reader created",
		logging.String("source_name", t.sourceName),
		logging.String("topic", t.spec.Topic),
		logging.String("group_id", t.spec.GroupID),
	)

	return nil
}

// Recv blocks until a message is received from the Kafka topic or the
// context is cancelled. Returns the raw message value.
func (t *KafkaTransport) Recv(ctx context.Context) ([]byte, error) {
	t.mu.Lock()
	reader := t.reader
	t.mu.Unlock()

	if reader == nil {
		return nil, fmt.Errorf("kafka reader not connected")
	}

	msg, err := reader.ReadMessage(ctx)
	if err != nil {
		return nil, fmt.Errorf("kafka read: %w", err)
	}

	return msg.Value, nil
}

// Close shuts down the Kafka reader. Safe to call multiple times.
func (t *KafkaTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	if t.reader == nil {
		return nil
	}

	reader := t.reader
	t.reader = nil

	t.logger.Info("kafka reader closing",
		logging.String("source_name", t.sourceName),
		logging.String("topic", t.spec.Topic),
	)

	return reader.Close()
}

package declarative

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
)

func TestKafkaTransport_RegisteredInFactory(t *testing.T) {
	transportMu.RLock()
	_, ok := transportConstructors["kafka"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("kafka transport not registered in factory")
	}
}

func TestKafkaTransport_NewKafkaTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers:        []string{"localhost:9092"},
				Topic:          "test-topic",
				GroupID:        "test-group",
				StartOffset:    "earliest",
				MaxBytes:       1048576,
				CommitInterval: Duration{Duration: 5 * time.Second},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.sourceName != "test_kafka" {
		t.Errorf("expected source name test_kafka, got %s", kt.sourceName)
	}
	if kt.readerCfg.Topic != "test-topic" {
		t.Errorf("expected topic test-topic, got %s", kt.readerCfg.Topic)
	}
	if kt.readerCfg.GroupID != "test-group" {
		t.Errorf("expected group_id test-group, got %s", kt.readerCfg.GroupID)
	}
	if kt.readerCfg.MaxBytes != 1048576 {
		t.Errorf("expected MaxBytes 1048576, got %d", kt.readerCfg.MaxBytes)
	}
	if kt.readerCfg.CommitInterval != 5*time.Second {
		t.Errorf("expected CommitInterval 5s, got %v", kt.readerCfg.CommitInterval)
	}
	if kt.readerCfg.StartOffset != kafka.FirstOffset {
		t.Errorf("expected StartOffset FirstOffset (%d), got %d", kafka.FirstOffset, kt.readerCfg.StartOffset)
	}
}

func TestKafkaTransport_NewKafkaTransport_NilSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			// Kafka is nil
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for nil Kafka spec, got nil")
	}
	if !strings.Contains(err.Error(), "transport.kafka configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKafkaTransport_NewKafkaTransport_MissingBrokers(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{},
				Topic:   "test-topic",
				GroupID: "test-group",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for empty brokers")
	}
	if !strings.Contains(err.Error(), "broker") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKafkaTransport_NewKafkaTransport_MissingTopic(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "",
				GroupID: "test-group",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for empty topic")
	}
	if !strings.Contains(err.Error(), "topic") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKafkaTransport_NewKafkaTransport_MissingGroupID(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err == nil {
		t.Fatal("expected error for empty group_id")
	}
	if !strings.Contains(err.Error(), "group_id") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKafkaTransport_StartOffsetLatest(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers:     []string{"localhost:9092"},
				Topic:       "test-topic",
				GroupID:     "test-group",
				StartOffset: "latest",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.StartOffset != kafka.LastOffset {
		t.Errorf("expected StartOffset LastOffset (%d), got %d", kafka.LastOffset, kt.readerCfg.StartOffset)
	}
}

func TestKafkaTransport_StartOffsetDefault(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				// StartOffset not set -- defaults to earliest
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.StartOffset != kafka.FirstOffset {
		t.Errorf("expected StartOffset FirstOffset (%d), got %d", kafka.FirstOffset, kt.readerCfg.StartOffset)
	}
}

func TestKafkaTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	kt := &KafkaTransport{}

	_, _, err := kt.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Fatalf("expected ErrNotPullBased, got %v", err)
	}
}

func TestKafkaTransport_CloseIdempotent(t *testing.T) {
	kt := &KafkaTransport{
		sourceName: "test",
		logger:     testLogger(),
		spec:       &KafkaSpec{Topic: "t"},
		// reader is nil -- Close() must handle this
	}

	err := kt.Close()
	if err != nil {
		t.Fatalf("first Close() error: %v", err)
	}

	err = kt.Close()
	if err != nil {
		t.Fatalf("second Close() error: %v", err)
	}
}

func TestKafkaTransport_CloseWithoutConnect(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	err = kt.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestKafkaTransport_RecvWithoutConnect(t *testing.T) {
	kt := &KafkaTransport{}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := kt.Recv(ctx)
	if err == nil {
		t.Fatal("expected error when Recv called before Connect")
	}
	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestKafkaTransport_SASLPlain(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				SASL: &KafkaSASLSpec{
					Mechanism: "PLAIN",
					Username:  "KAFKA_USER",
					Password:  "KAFKA_PASS",
				},
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "KAFKA_USER":
			return "resolved-user"
		case "KAFKA_PASS":
			return "resolved-pass"
		default:
			return ""
		}
	}

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.Dialer == nil {
		t.Fatal("expected non-nil Dialer for SASL config")
	}
	if kt.readerCfg.Dialer.SASLMechanism == nil {
		t.Fatal("expected non-nil SASL mechanism")
	}
	if kt.readerCfg.Dialer.TLS == nil {
		t.Fatal("expected non-nil TLS config when SASL is enabled")
	}
}

func TestKafkaTransport_SASLScramSHA256(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				SASL: &KafkaSASLSpec{
					Mechanism: "SCRAM-SHA-256",
					Username:  "KAFKA_USER",
					Password:  "KAFKA_PASS",
				},
			},
		},
	}

	envResolve := func(name string) string { return "test-value" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.Dialer == nil || kt.readerCfg.Dialer.SASLMechanism == nil {
		t.Fatal("expected SASL mechanism to be configured")
	}
}

func TestKafkaTransport_SASLScramSHA512(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				SASL: &KafkaSASLSpec{
					Mechanism: "SCRAM-SHA-512",
					Username:  "KAFKA_USER",
					Password:  "KAFKA_PASS",
				},
			},
		},
	}

	envResolve := func(name string) string { return "test-value" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.Dialer == nil || kt.readerCfg.Dialer.SASLMechanism == nil {
		t.Fatal("expected SASL mechanism to be configured")
	}
}

func TestKafkaTransport_NoDialerWithoutSASLOrTLS(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.Dialer != nil {
		t.Error("expected nil Dialer when no SASL or TLS configured")
	}
}

func TestKafkaTransport_TLSWithoutSASL(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				TLS: &TLSSpec{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.Dialer == nil {
		t.Fatal("expected non-nil Dialer for TLS config")
	}
	if kt.readerCfg.Dialer.TLS == nil {
		t.Fatal("expected non-nil TLS config")
	}
	if !kt.readerCfg.Dialer.TLS.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
	if kt.readerCfg.Dialer.SASLMechanism != nil {
		t.Error("expected nil SASL mechanism when no SASL configured")
	}
}

func TestKafkaTransport_MultipleBrokers(t *testing.T) {
	brokers := []string{"broker1:9092", "broker2:9092", "broker3:9092"}
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: brokers,
				Topic:   "test-topic",
				GroupID: "test-group",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if len(kt.readerCfg.Brokers) != 3 {
		t.Errorf("expected 3 brokers, got %d", len(kt.readerCfg.Brokers))
	}
	for i, b := range brokers {
		if kt.readerCfg.Brokers[i] != b {
			t.Errorf("broker[%d]: expected %s, got %s", i, b, kt.readerCfg.Brokers[i])
		}
	}
}

func TestKafkaTransport_MaxBytesDefault(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				// MaxBytes not set
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	// When MaxBytes is 0 in our config, we don't set it -- kafka-go defaults to 1MB.
	if kt.readerCfg.MaxBytes != 0 {
		t.Errorf("expected MaxBytes 0 (kafka-go default), got %d", kt.readerCfg.MaxBytes)
	}
}

func TestKafkaTransport_NewKafkaTransport_MissingSpec(t *testing.T) {
	tests := []struct {
		name    string
		def     *SourceDefinition
		wantSub string
	}{
		{
			name: "nil_kafka_spec",
			def: &SourceDefinition{
				Name:      "test_kafka",
				Transport: TransportSpec{Type: "kafka"},
			},
			wantSub: "transport.kafka configuration",
		},
		{
			name: "empty_brokers",
			def: &SourceDefinition{
				Name: "test_kafka",
				Transport: TransportSpec{
					Type:  "kafka",
					Kafka: &KafkaSpec{Topic: "t", GroupID: "g"},
				},
			},
			wantSub: "broker",
		},
		{
			name: "empty_topic",
			def: &SourceDefinition{
				Name: "test_kafka",
				Transport: TransportSpec{
					Type:  "kafka",
					Kafka: &KafkaSpec{Brokers: []string{"b:9092"}, GroupID: "g"},
				},
			},
			wantSub: "topic",
		},
		{
			name: "empty_group_id",
			def: &SourceDefinition{
				Name: "test_kafka",
				Transport: TransportSpec{
					Type:  "kafka",
					Kafka: &KafkaSpec{Brokers: []string{"b:9092"}, Topic: "t"},
				},
			},
			wantSub: "group_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envResolve := func(name string) string { return "" }
			_, err := newKafkaTransport(tt.def, nil, nil, envResolve, testLogger())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantSub)
			}
		})
	}
}

func TestKafkaTransport_CredentialResolution(t *testing.T) {
	tests := []struct {
		name string
		sasl *KafkaSASLSpec
		env  map[string]string
	}{
		{
			name: "PLAIN_resolved",
			sasl: &KafkaSASLSpec{Mechanism: "PLAIN", Username: "U", Password: "P"},
			env:  map[string]string{"U": "user1", "P": "pass1"},
		},
		{
			name: "SCRAM-SHA-256_resolved",
			sasl: &KafkaSASLSpec{Mechanism: "SCRAM-SHA-256", Username: "U", Password: "P"},
			env:  map[string]string{"U": "user1", "P": "pass1"},
		},
		{
			name: "SCRAM-SHA-512_resolved",
			sasl: &KafkaSASLSpec{Mechanism: "SCRAM-SHA-512", Username: "U", Password: "P"},
			env:  map[string]string{"U": "user1", "P": "pass1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &SourceDefinition{
				Name: "test_kafka",
				Transport: TransportSpec{
					Type: "kafka",
					Kafka: &KafkaSpec{
						Brokers: []string{"localhost:9092"},
						Topic:   "test-topic",
						GroupID: "test-group",
						SASL:    tt.sasl,
					},
				},
			}
			envResolve := func(name string) string { return tt.env[name] }
			st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			kt := st.(*KafkaTransport)
			if kt.readerCfg.Dialer == nil {
				t.Fatal("expected Dialer to be configured for SASL")
			}
			if kt.readerCfg.Dialer.SASLMechanism == nil {
				t.Fatal("expected SASLMechanism to be set")
			}
		})
	}
}

func TestKafkaTransport_StartOffsetEarliest(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers:     []string{"localhost:9092"},
				Topic:       "test-topic",
				GroupID:     "test-group",
				StartOffset: "earliest",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.StartOffset != kafka.FirstOffset {
		t.Errorf("expected StartOffset FirstOffset (%d), got %d", kafka.FirstOffset, kt.readerCfg.StartOffset)
	}
}

func TestKafkaTransport_TLSConfig(t *testing.T) {
	tests := []struct {
		name       string
		tls        *TLSSpec
		wantDialer bool
		wantInsec  bool
	}{
		{
			name:       "enabled_minimal",
			tls:        &TLSSpec{},
			wantDialer: true,
		},
		{
			name:       "insecure_skip_verify",
			tls:        &TLSSpec{InsecureSkipVerify: true},
			wantDialer: true,
			wantInsec:  true,
		},
		{
			name:       "nil_tls",
			tls:        nil,
			wantDialer: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			def := &SourceDefinition{
				Name: "test_kafka",
				Transport: TransportSpec{
					Type: "kafka",
					Kafka: &KafkaSpec{
						Brokers: []string{"localhost:9092"},
						Topic:   "test-topic",
						GroupID: "test-group",
						TLS:     tt.tls,
					},
				},
			}

			envResolve := func(name string) string { return "" }
			st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			kt := st.(*KafkaTransport)
			if tt.wantDialer {
				if kt.readerCfg.Dialer == nil {
					t.Fatal("expected Dialer")
				}
				if kt.readerCfg.Dialer.TLS == nil {
					t.Fatal("expected TLS config")
				}
				if tt.wantInsec && !kt.readerCfg.Dialer.TLS.InsecureSkipVerify {
					t.Error("expected InsecureSkipVerify=true")
				}
			} else if kt.readerCfg.Dialer != nil {
				t.Error("expected nil Dialer")
			}
		})
	}
}

func TestKafkaTransport_CommitIntervalDefault(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_kafka",
		Transport: TransportSpec{
			Type: "kafka",
			Kafka: &KafkaSpec{
				Brokers: []string{"localhost:9092"},
				Topic:   "test-topic",
				GroupID: "test-group",
				// CommitInterval not set
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newKafkaTransport(def, nil, nil, envResolve, testLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kt := st.(*KafkaTransport)
	if kt.readerCfg.CommitInterval != 0 {
		t.Errorf("expected CommitInterval 0 (synchronous), got %v", kt.readerCfg.CommitInterval)
	}
}

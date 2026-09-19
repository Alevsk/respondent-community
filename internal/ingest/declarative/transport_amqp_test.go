package declarative

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestAMQPTransport_FetchReturnsErrNotPullBased(t *testing.T) {
	tr := &AMQPTransport{
		logger: logging.NewNopLogger(),
		msgCh:  make(chan []byte, 1),
		done:   make(chan struct{}),
	}

	_, _, err := tr.Fetch(context.Background(), "GET", "http://example.com")
	if !errors.Is(err, ErrNotPullBased) {
		t.Errorf("Fetch error = %v, want ErrNotPullBased", err)
	}
}

func TestAMQPTransport_NewAMQPTransport_ValidConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:           "amqp://guest:guest@localhost:5672/",
				Queue:         "my-queue",
				PrefetchCount: 20,
				ExchangeType:  "direct",
				AutoAck:       true,
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.URL != "amqp://guest:guest@localhost:5672/" {
		t.Errorf("expected URL amqp://guest:guest@localhost:5672/, got %s", at.spec.URL)
	}
	if at.spec.Queue != "my-queue" {
		t.Errorf("expected queue my-queue, got %s", at.spec.Queue)
	}
	if at.spec.PrefetchCount != 20 {
		t.Errorf("expected prefetch count 20, got %d", at.spec.PrefetchCount)
	}
	if at.spec.ExchangeType != "direct" {
		t.Errorf("expected exchange type direct, got %s", at.spec.ExchangeType)
	}
	if at.sourceName != "test_amqp" {
		t.Errorf("expected source name test_amqp, got %s", at.sourceName)
	}
	if !at.spec.AutoAck {
		t.Error("expected auto_ack to be true")
	}
}

func TestAMQPTransport_NewAMQPTransport_MissingSpec(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			// AMQP is nil
		},
	}

	envResolve := func(name string) string { return "" }

	_, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err == nil {
		t.Fatal("expected error for nil AMQP spec, got nil")
	}
	if !strings.Contains(err.Error(), "transport.amqp configuration") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestAMQPTransport_DefaultPrefetchCount(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:   "amqp://localhost:5672/",
				Queue: "q",
				// PrefetchCount not set -- should default to 10
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.PrefetchCount != defaultAMQPPrefetchCount {
		t.Errorf("expected default prefetch count %d, got %d", defaultAMQPPrefetchCount, at.spec.PrefetchCount)
	}
}

func TestAMQPTransport_DefaultExchangeType(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:      "amqp://localhost:5672/",
				Exchange: "my-exchange",
				// ExchangeType not set -- should default to "topic"
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.ExchangeType != defaultAMQPExchangeType {
		t.Errorf("expected default exchange type %q, got %q", defaultAMQPExchangeType, at.spec.ExchangeType)
	}
}

func TestAMQPTransport_CloseIdempotent(t *testing.T) {
	tr := &AMQPTransport{
		sourceName: "test",
		logger:     logging.NewNopLogger(),
		msgCh:      make(chan []byte, 1),
		done:       make(chan struct{}),
		// conn and channel are nil -- Close() must handle this
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

func TestAMQPTransport_RegisteredInFactory(t *testing.T) {
	// Verify the init() function registered "amqp" in the transport factory.
	transportMu.RLock()
	_, ok := transportConstructors["amqp"]
	transportMu.RUnlock()
	if !ok {
		t.Fatal("amqp transport not registered in factory")
	}
}

func TestAMQPTransport_URLEnvExpansion(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp_env",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:   "amqp://${AMQP_USER}:${AMQP_PASS}@rabbit.example.com:5672/",
				Queue: "events",
			},
		},
	}

	envResolve := func(name string) string {
		switch name {
		case "AMQP_USER":
			return "myuser"
		case "AMQP_PASS":
			return "secret123"
		default:
			return ""
		}
	}

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	expected := "amqp://myuser:secret123@rabbit.example.com:5672/"
	if at.spec.URL != expected {
		t.Errorf("expected URL %q, got %q", expected, at.spec.URL)
	}
}

func TestAMQPTransport_RecvContextCancellation(t *testing.T) {
	tr := &AMQPTransport{
		logger: logging.NewNopLogger(),
		msgCh:  make(chan []byte, 1),
		done:   make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := tr.Recv(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAMQPTransport_RecvMessage(t *testing.T) {
	tr := &AMQPTransport{
		logger: logging.NewNopLogger(),
		msgCh:  make(chan []byte, 1),
		done:   make(chan struct{}),
	}

	expected := []byte(`{"lat":42.0,"lon":-71.0}`)
	tr.msgCh <- expected

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg, err := tr.Recv(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(msg) != string(expected) {
		t.Errorf("expected %s, got %s", expected, msg)
	}
}

func TestAMQPTransport_RecvChannelClosed(t *testing.T) {
	tr := &AMQPTransport{
		logger: logging.NewNopLogger(),
		msgCh:  make(chan []byte, 1),
		done:   make(chan struct{}),
	}

	close(tr.msgCh)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := tr.Recv(ctx)
	if err == nil {
		t.Fatal("expected error on closed channel, got nil")
	}
	if !strings.Contains(err.Error(), "closed") {
		t.Errorf("expected 'closed' in error, got %v", err)
	}
}

func TestAMQPTransport_CloseWithoutConnect(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:   "amqp://localhost:5672/",
				Queue: "q",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Close without ever calling Connect -- must not panic.
	at := st.(*AMQPTransport)
	err = at.Close()
	if err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}

func TestAMQPTransport_ExchangeConfig(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp_exchange",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:          "amqp://localhost:5672/",
				Exchange:     "events",
				RoutingKey:   "sensor.data",
				ExchangeType: "topic",
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.Exchange != "events" {
		t.Errorf("expected exchange 'events', got %s", at.spec.Exchange)
	}
	if at.spec.RoutingKey != "sensor.data" {
		t.Errorf("expected routing key 'sensor.data', got %s", at.spec.RoutingKey)
	}
	if at.spec.ExchangeType != "topic" {
		t.Errorf("expected exchange type 'topic', got %s", at.spec.ExchangeType)
	}
}

func TestAMQPTransport_AutoAckDefault(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:   "amqp://localhost:5672/",
				Queue: "my-queue",
				// AutoAck not set -- defaults to false
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.AutoAck {
		t.Error("expected auto_ack to default to false")
	}
}

func TestAMQPTransport_TLSSpecStored(t *testing.T) {
	def := &SourceDefinition{
		Name: "test_amqp_tls",
		Transport: TransportSpec{
			Type:     "amqp",
			Timeout:  Duration{Duration: 10 * time.Second},
			Interval: Duration{Duration: 60 * time.Second},
			AMQP: &AMQPSpec{
				URL:   "amqps://localhost:5671/",
				Queue: "secure-queue",
				TLS: &TLSSpec{
					InsecureSkipVerify: true,
				},
			},
		},
	}

	envResolve := func(name string) string { return "" }

	st, err := newAMQPTransport(def, nil, nil, envResolve, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	at := st.(*AMQPTransport)
	if at.spec.TLS == nil {
		t.Fatal("expected TLS spec to be stored")
	}
	if !at.spec.TLS.InsecureSkipVerify {
		t.Error("expected InsecureSkipVerify=true")
	}
}

func TestResolveEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		resolver EnvResolver
		want     string
	}{
		{
			name:     "no vars",
			input:    "amqp://localhost:5672/",
			resolver: func(string) string { return "" },
			want:     "amqp://localhost:5672/",
		},
		{
			name:  "single var",
			input: "amqp://${HOST}:5672/",
			resolver: func(name string) string {
				if name == "HOST" {
					return "rabbit.local"
				}
				return ""
			},
			want: "amqp://rabbit.local:5672/",
		},
		{
			name:  "multiple vars",
			input: "${PROTO}://${USER}:${PASS}@${HOST}/",
			resolver: func(name string) string {
				m := map[string]string{
					"PROTO": "amqps",
					"USER":  "admin",
					"PASS":  "pw",
					"HOST":  "rabbit",
				}
				return m[name]
			},
			want: "amqps://admin:pw@rabbit/",
		},
		{
			name:     "unclosed brace",
			input:    "amqp://${HOST/",
			resolver: func(string) string { return "val" },
			want:     "amqp://${HOST/",
		},
		{
			name:     "empty var name",
			input:    "amqp://${}@host/",
			resolver: func(name string) string { return "resolved" },
			want:     "amqp://resolved@host/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveEnvVars(tt.input, tt.resolver)
			if got != tt.want {
				t.Errorf("resolveEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

package declarative

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"github.com/Alevsk/respondent/internal/logging"
)

// mqttMsgChSize is the buffer size for the internal message channel.
// Sized to absorb bursts without blocking the MQTT library's callback goroutine.
const mqttMsgChSize = 1000

// MQTTTransport implements StreamTransport for MQTT sources.
// Messages arrive via the MQTT library's callback and are buffered in msgCh
// for consumption via Recv.
type MQTTTransport struct {
	spec       *MQTTSpec
	sourceName string
	logger     *logging.Logger

	// Resolved credentials (env var values, not the env var names).
	username string
	password string

	mu     sync.Mutex
	client mqtt.Client
	msgCh  chan []byte
	done   chan struct{}
	closed bool
}

func init() {
	RegisterTransport("mqtt", newMQTTTransport)
}

// newMQTTTransport constructs an MQTTTransport from a source definition.
func newMQTTTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.MQTT == nil {
		return nil, fmt.Errorf("mqtt transport requires transport.mqtt configuration")
	}

	spec := def.Transport.MQTT

	// Resolve credentials: YAML stores env var names (e.g. "MQTT_USER"),
	// envResolve prepends "RESPONDENT_" and looks up the value.
	var username, password string
	if spec.Username != "" {
		username = envResolve(spec.Username)
	}
	if spec.Password != "" {
		password = envResolve(spec.Password)
	}

	return &MQTTTransport{
		spec:       spec,
		sourceName: def.Name,
		logger:     logger.WithSource(def.Name),
		username:   username,
		password:   password,
		msgCh:      make(chan []byte, mqttMsgChSize),
		done:       make(chan struct{}),
	}, nil
}

// Fetch returns ErrNotPullBased. MQTT is push-based; use Connect + Recv instead.
func (t *MQTTTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect establishes the MQTT connection and subscribes to all configured topics.
func (t *MQTTTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	opts := mqtt.NewClientOptions()
	opts.AddBroker(t.spec.Broker)

	// Client ID: use spec value or auto-generate.
	clientID := t.spec.ClientID
	if clientID == "" {
		clientID = fmt.Sprintf("%s-%s", t.sourceName, randomSuffix())
	}
	opts.SetClientID(clientID)

	// Clean session defaults to true.
	cleanSession := true
	if t.spec.CleanSession != nil {
		cleanSession = *t.spec.CleanSession
	}
	opts.SetCleanSession(cleanSession)

	// Keep alive defaults to 60s.
	keepAlive := 60 * time.Second
	if t.spec.KeepAlive > 0 {
		keepAlive = time.Duration(t.spec.KeepAlive) * time.Second
	}
	opts.SetKeepAlive(keepAlive)

	// Disable paho's auto-reconnect; our reconnect loop handles this.
	opts.SetAutoReconnect(false)

	// Credentials.
	if t.username != "" {
		opts.SetUsername(t.username)
	}
	if t.password != "" {
		opts.SetPassword(t.password)
	}

	// TLS.
	if t.spec.TLS != nil {
		tlsCfg, err := buildTLSConfig(t.spec.TLS)
		if err != nil {
			return fmt.Errorf("build TLS config: %w", err)
		}
		if tlsCfg != nil {
			opts.SetTLSConfig(tlsCfg)
		}
	}

	// Connection lost handler for logging.
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		t.logger.Warn("MQTT connection lost",
			logging.String("source_name", t.sourceName),
			logging.Err("error", err),
		)
	})

	client := mqtt.NewClient(opts)

	// Connect with context-aware timeout.
	token := client.Connect()

	// Wait for connect with context cancellation.
	connectDone := make(chan struct{})
	go func() {
		token.Wait()
		close(connectDone)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("connect cancelled: %w", ctx.Err())
	case <-connectDone:
		if err := token.Error(); err != nil {
			return fmt.Errorf("MQTT connect to %s: %w", t.spec.Broker, err)
		}
	}

	t.logger.Info("MQTT connected",
		logging.String("source_name", t.sourceName),
		logging.String("broker", t.spec.Broker),
		logging.String("client_id", clientID),
	)

	// Subscribe to all topics.
	for _, sub := range t.spec.Topics {
		qos := byte(0)
		if sub.QoS != nil {
			qos = byte(*sub.QoS)
		}

		msgCh := t.msgCh // capture for closure
		done := t.done   // capture done for closure
		subToken := client.Subscribe(sub.Topic, qos, func(_ mqtt.Client, msg mqtt.Message) {
			// Non-blocking write: drop messages if channel is full or transport
			// is shutting down, to avoid blocking the MQTT library's goroutine.
			select {
			case <-done:
				return
			default:
			}
			select {
			case msgCh <- msg.Payload():
			case <-done:
			default:
				t.logger.Warn("MQTT message dropped, channel full",
					logging.String("source_name", t.sourceName),
					logging.String("topic", msg.Topic()),
				)
			}
		})

		subDone := make(chan struct{})
		go func() {
			subToken.Wait()
			close(subDone)
		}()

		select {
		case <-ctx.Done():
			client.Disconnect(250)
			return fmt.Errorf("subscribe cancelled: %w", ctx.Err())
		case <-subDone:
			if err := subToken.Error(); err != nil {
				client.Disconnect(250)
				return fmt.Errorf("subscribe to %q: %w", sub.Topic, err)
			}
		}

		t.logger.Info("MQTT subscribed",
			logging.String("source_name", t.sourceName),
			logging.String("topic", sub.Topic),
			logging.Any("qos", qos),
		)
	}

	t.client = client
	return nil
}

// Recv blocks until a message is available, the context is cancelled, or the transport is closed.
func (t *MQTTTransport) Recv(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-t.msgCh:
		if !ok {
			return nil, fmt.Errorf("MQTT transport closed")
		}
		return msg, nil
	}
}

// Close disconnects the MQTT client and signals goroutines to stop.
// Safe to call multiple times.
func (t *MQTTTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	close(t.done)

	if t.client != nil && t.client.IsConnected() {
		t.client.Disconnect(250)
		t.logger.Info("MQTT disconnected",
			logging.String("source_name", t.sourceName),
		)
	}

	// Close msgCh after disconnecting so callbacks have stopped;
	// this unblocks any Recv() waiting on the channel.
	close(t.msgCh)

	return nil
}

// randomSuffix generates a short random hex string for client ID uniqueness.
func randomSuffix() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

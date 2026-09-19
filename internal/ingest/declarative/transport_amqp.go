package declarative

import (
	"context"
	"fmt"
	"sync"

	amqp091 "github.com/rabbitmq/amqp091-go"

	"github.com/Alevsk/respondent/internal/logging"
)

// defaultAMQPPrefetchCount is the default QoS prefetch when none is specified.
const defaultAMQPPrefetchCount = 10

// defaultAMQPExchangeType is the default exchange type when none is specified.
const defaultAMQPExchangeType = "topic"

// amqpMsgChSize is the buffer size for the internal delivery channel.
const amqpMsgChSize = 1000

// Compile-time assertion that AMQPTransport implements StreamTransport.
var _ StreamTransport = (*AMQPTransport)(nil)

func init() {
	RegisterTransport("amqp", newAMQPTransport)
}

// AMQPTransport implements StreamTransport for AMQP (RabbitMQ) sources.
// It connects to an AMQP broker, declares/binds queues and exchanges as needed,
// and delivers messages via Recv.
type AMQPTransport struct {
	spec       *AMQPSpec
	sourceName string
	logger     *logging.Logger

	mu      sync.Mutex
	conn    *amqp091.Connection
	channel *amqp091.Channel
	msgCh   chan []byte
	done    chan struct{}
	closed  bool
}

// newAMQPTransport creates an AMQPTransport from a source definition.
func newAMQPTransport(def *SourceDefinition, _ map[string]string, _ TokenProvider, envResolve EnvResolver, logger *logging.Logger) (Transport, error) {
	if def.Transport.AMQP == nil {
		return nil, fmt.Errorf("amqp transport requires transport.amqp configuration")
	}

	spec := def.Transport.AMQP

	if spec.URL == "" {
		return nil, fmt.Errorf("amqp transport requires a non-empty url")
	}

	// Resolve ${VAR_NAME} placeholders in the URL via envResolve.
	resolvedURL := resolveEnvVars(spec.URL, envResolve)

	// Apply defaults.
	exchangeType := spec.ExchangeType
	if exchangeType == "" {
		exchangeType = defaultAMQPExchangeType
	}

	prefetchCount := spec.PrefetchCount
	if prefetchCount == 0 {
		prefetchCount = defaultAMQPPrefetchCount
	}

	return &AMQPTransport{
		spec: &AMQPSpec{
			URL:           resolvedURL,
			Queue:         spec.Queue,
			Exchange:      spec.Exchange,
			RoutingKey:    spec.RoutingKey,
			ExchangeType:  exchangeType,
			PrefetchCount: prefetchCount,
			AutoAck:       spec.AutoAck,
			TLS:           spec.TLS,
		},
		sourceName: def.Name,
		logger:     logger.WithSource(def.Name),
		msgCh:      make(chan []byte, amqpMsgChSize),
		done:       make(chan struct{}),
	}, nil
}

// Fetch returns ErrNotPullBased. AMQP is push-based; use Connect + Recv instead.
func (t *AMQPTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

// Connect establishes the AMQP connection, declares exchanges/queues as needed,
// sets QoS, and starts consuming messages into the internal channel.
func (t *AMQPTransport) Connect(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Dial the broker.
	var conn *amqp091.Connection
	var err error

	if t.spec.TLS != nil {
		tlsCfg, tlsErr := buildTLSConfig(t.spec.TLS)
		if tlsErr != nil {
			return fmt.Errorf("build TLS config: %w", tlsErr)
		}
		if tlsCfg != nil {
			conn, err = amqp091.DialTLS(t.spec.URL, tlsCfg)
		} else {
			conn, err = amqp091.Dial(t.spec.URL)
		}
	} else {
		conn, err = amqp091.Dial(t.spec.URL)
	}
	if err != nil {
		return fmt.Errorf("AMQP dial %s: %w", t.spec.URL, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("AMQP open channel: %w", err)
	}

	// Set QoS (prefetch count).
	if err := ch.Qos(t.spec.PrefetchCount, 0, false); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("AMQP set QoS (prefetch=%d): %w", t.spec.PrefetchCount, err)
	}

	var consumeQueue string

	switch {
	case t.spec.Exchange != "":
		// Declare the exchange.
		if err := ch.ExchangeDeclare(
			t.spec.Exchange,
			t.spec.ExchangeType,
			true,  // durable
			false, // auto-deleted
			false, // internal
			false, // no-wait
			nil,   // args
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return fmt.Errorf("AMQP declare exchange %q: %w", t.spec.Exchange, err)
		}

		if t.spec.Queue != "" {
			// Use the named queue.
			consumeQueue = t.spec.Queue
		} else {
			// Declare a temporary (exclusive, auto-delete) queue.
			q, err := ch.QueueDeclare(
				"",    // empty name = server-generated
				false, // durable
				true,  // auto-delete
				true,  // exclusive
				false, // no-wait
				nil,   // args
			)
			if err != nil {
				_ = ch.Close()
				_ = conn.Close()
				return fmt.Errorf("AMQP declare temporary queue: %w", err)
			}
			consumeQueue = q.Name
		}

		// Bind the queue to the exchange with the routing key.
		routingKey := t.spec.RoutingKey
		if routingKey == "" {
			routingKey = "#" // match all for topic exchanges
		}

		if err := ch.QueueBind(
			consumeQueue,
			routingKey,
			t.spec.Exchange,
			false, // no-wait
			nil,   // args
		); err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return fmt.Errorf("AMQP bind queue %q to exchange %q with key %q: %w",
				consumeQueue, t.spec.Exchange, routingKey, err)
		}

		t.logger.Info("AMQP bound queue to exchange",
			logging.String("source_name", t.sourceName),
			logging.String("exchange", t.spec.Exchange),
			logging.String("routing_key", routingKey),
			logging.String("queue", consumeQueue),
		)
	case t.spec.Queue != "":
		// Consume directly from the named queue.
		consumeQueue = t.spec.Queue
	default:
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("AMQP transport requires either queue or exchange to be set")
	}

	// Start consuming.
	consumerTag := fmt.Sprintf("%s-consumer", t.sourceName)
	deliveries, err := ch.Consume(
		consumeQueue,
		consumerTag,
		t.spec.AutoAck,
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("AMQP consume from %q: %w", consumeQueue, err)
	}

	t.conn = conn
	t.channel = ch

	t.logger.Info("AMQP connected",
		logging.String("source_name", t.sourceName),
		logging.String("queue", consumeQueue),
		logging.Int("prefetch_count", t.spec.PrefetchCount),
	)

	// Relay deliveries to msgCh in a background goroutine.
	done := t.done
	autoAck := t.spec.AutoAck
	go func() {
		for {
			select {
			case <-done:
				return
			case d, ok := <-deliveries:
				if !ok {
					return
				}
				// Copy the body to decouple from the delivery.
				body := make([]byte, len(d.Body))
				copy(body, d.Body)

				// Non-blocking write to avoid blocking the AMQP library.
				select {
				case t.msgCh <- body:
					// If manual ack, acknowledge after buffering.
					if !autoAck {
						_ = d.Ack(false)
					}
				default:
					// Channel full: nack for redelivery if manual ack.
					if !autoAck {
						_ = d.Nack(false, true)
					}
					t.logger.Warn("AMQP message dropped, channel full",
						logging.String("source_name", t.sourceName),
					)
				}
			}
		}
	}()

	return nil
}

// Recv blocks until a message is available, the context is cancelled, or the transport is closed.
func (t *AMQPTransport) Recv(ctx context.Context) ([]byte, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg, ok := <-t.msgCh:
		if !ok {
			return nil, fmt.Errorf("AMQP transport closed")
		}
		return msg, nil
	}
}

// Close shuts down the AMQP channel and connection.
// Safe to call multiple times and before Connect.
func (t *AMQPTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	// Signal the relay goroutine to stop.
	select {
	case <-t.done:
		// Already closed.
	default:
		close(t.done)
	}

	if t.channel != nil {
		_ = t.channel.Close()
		t.channel = nil
	}

	if t.conn != nil {
		_ = t.conn.Close()
		t.conn = nil
	}

	t.logger.Info("AMQP disconnected",
		logging.String("source_name", t.sourceName),
	)

	return nil
}

// resolveEnvVars replaces ${VAR_NAME} placeholders in s with values from envResolve.
func resolveEnvVars(s string, envResolve EnvResolver) string {
	result := []byte(s)
	for {
		start := -1
		for i := 0; i < len(result)-1; i++ {
			if result[i] == '$' && result[i+1] == '{' {
				start = i
				break
			}
		}
		if start == -1 {
			break
		}
		end := -1
		for i := start + 2; i < len(result); i++ {
			if result[i] == '}' {
				end = i
				break
			}
		}
		if end == -1 {
			break
		}
		varName := string(result[start+2 : end])
		value := envResolve(varName)
		replaced := make([]byte, 0, len(result))
		replaced = append(replaced, result[:start]...)
		replaced = append(replaced, []byte(value)...)
		replaced = append(replaced, result[end+1:]...)
		result = replaced
	}
	return string(result)
}

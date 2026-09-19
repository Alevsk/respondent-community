package inproc

import (
	"context"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// Consumer implements domain.StreamConsumer using the shared Bus.
type Consumer struct {
	bus *Bus
}

// NewConsumer creates a Consumer backed by the given Bus.
func NewConsumer(bus *Bus) domain.StreamConsumer {
	return &Consumer{bus: bus}
}

// Consume subscribes to cfg.FilterSubject and calls handler for each received message.
// Blocks until ctx is canceled. Thread-safe: multiple calls create independent subscriptions.
func (c *Consumer) Consume(ctx context.Context, cfg domain.StreamConsumerConfig, handler func(msg domain.ConsumedMessage)) error {
	subject := cfg.FilterSubject
	if subject == "" {
		subject = cfg.StreamName
	}

	ch := c.bus.subscribe(subject)
	defer c.bus.unsubscribe(subject, ch)

	for {
		select {
		case data, ok := <-ch:
			if !ok {
				return nil
			}
			handler(&inprocMessage{data: data})
		case <-ctx.Done():
			return nil
		}
	}
}

// inprocMessage implements domain.ConsumedMessage.
// Ack, Nak, and NakWithDelay are no-ops: in-process delivery has no redelivery.
type inprocMessage struct {
	data []byte
}

func (m *inprocMessage) Data() []byte                       { return m.data }
func (m *inprocMessage) Ack() error                         { return nil }
func (m *inprocMessage) Nak() error                         { return nil }
func (m *inprocMessage) NakWithDelay(_ time.Duration) error { return nil }

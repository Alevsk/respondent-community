package domain

import (
	"context"
	"time"
)

// MessagePublisher abstracts message publishing to a broker.
// Implementations must be safe for concurrent use.
type MessagePublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}

// StreamConsumer abstracts durable stream consumption from a broker.
type StreamConsumer interface {
	Consume(ctx context.Context, cfg StreamConsumerConfig, handler func(msg ConsumedMessage)) error
}

// ConsumedMessage represents a message received from a stream.
type ConsumedMessage interface {
	Data() []byte
	Ack() error
	Nak() error
	NakWithDelay(delay time.Duration) error
}

// StreamConsumerConfig holds configuration for creating a stream consumer.
type StreamConsumerConfig struct {
	StreamName    string
	ConsumerGroup string
	FilterSubject string
	MaxDeliver    int
	AckWait       time.Duration
	MaxMessages   int
}

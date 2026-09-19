package inproc

import (
	"context"

	"github.com/Alevsk/respondent/internal/domain"
)

// Publisher implements domain.MessagePublisher using the shared Bus.
type Publisher struct {
	bus *Bus
}

// NewPublisher creates a Publisher backed by the given Bus.
func NewPublisher(bus *Bus) domain.MessagePublisher {
	return &Publisher{bus: bus}
}

// Publish sends data to all subscribers of subject.
// Blocks if any subscriber's buffer is full (backpressure).
func (p *Publisher) Publish(ctx context.Context, subject string, data []byte) error {
	return p.bus.publish(ctx, subject, data)
}

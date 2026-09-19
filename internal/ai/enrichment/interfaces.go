package enrichment

import (
	"context"
)

// JobPublisher abstracts enrichment job publishing.
// Implemented by Publisher (direct NATS) and CircuitBreakerJobPublisher (with backpressure).
type JobPublisher interface {
	Publish(ctx context.Context, job Job) error
	PublishBatch(ctx context.Context, jobs []Job) error
}

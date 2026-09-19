package enrichment

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// Compile-time interface assertion.
var _ JobPublisher = (*Publisher)(nil)

// Publisher publishes enrichment jobs to a message stream.
type Publisher struct {
	msgPublisher  domain.MessagePublisher
	subjectPrefix string // e.g., "respondent.ai.enrich"
}

// NewPublisher creates a new enrichment job publisher.
func NewPublisher(msgPublisher domain.MessagePublisher, subjectPrefix string) *Publisher {
	return &Publisher{msgPublisher: msgPublisher, subjectPrefix: subjectPrefix}
}

// Publish publishes a single enrichment job.
// Subject format: {subjectPrefix}.{sourceName}
func (p *Publisher) Publish(ctx context.Context, job Job) error {
	if job.PublishedAt.IsZero() {
		job.PublishedAt = time.Now()
	}
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal enrichment job: %w", err)
	}
	subject := fmt.Sprintf("%s.%s", p.subjectPrefix, job.SourceName)
	if err = p.msgPublisher.Publish(ctx, subject, data); err != nil {
		return fmt.Errorf("publish to %s: %w", subject, err)
	}
	return nil
}

// PublishBatch publishes multiple enrichment jobs.
func (p *Publisher) PublishBatch(ctx context.Context, jobs []Job) error {
	for i := range jobs {
		if err := p.Publish(ctx, jobs[i]); err != nil {
			return fmt.Errorf("publish job %d/%d: %w", i+1, len(jobs), err)
		}
	}
	return nil
}

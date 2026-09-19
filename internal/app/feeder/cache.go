package feeder

import (
	"context"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// FeederCache abstracts hot-cache write operations used during ingestion.
// This enables swapping the cache implementation without coupling the
// IngestionService to a concrete store.
type FeederCache interface {
	// SetEntity writes an entity and its observation to the hot cache with a TTL.
	SetEntity(ctx context.Context, entity *domain.Entity, obs *domain.Observation, ttl time.Duration) error
}

// FeederPublisher abstracts Pub/Sub publishing for real-time layer updates.
type FeederPublisher interface {
	// PublishLayerUpdate publishes a JSON-encoded update to the layer update channel.
	PublishLayerUpdate(ctx context.Context, layerType string, data []byte) error
}

package feeder

import (
	"context"
)

// FeederPublisher abstracts Pub/Sub publishing for real-time layer updates.
type FeederPublisher interface {
	// PublishLayerUpdate publishes a JSON-encoded update to the layer update channel.
	PublishLayerUpdate(ctx context.Context, layerType string, data []byte) error
}

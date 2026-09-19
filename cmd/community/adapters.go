package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// memcacheFeederCache adapts domain.CacheStorage to the feeder's FeederCache interface.
// The TTL parameter is ignored — memcache uses per-layer TTLs configured at startup.
type memcacheFeederCache struct {
	cache domain.CacheStorage
}

func (c *memcacheFeederCache) SetEntity(ctx context.Context, entity *domain.Entity, obs *domain.Observation, _ time.Duration) error {
	return c.cache.SetEntity(ctx, entity, obs)
}

// queueingBroadcaster is the minimal interface for batched message delivery.
// Satisfied by *realtime.Server.
type queueingBroadcaster interface {
	QueueUpdate(layerID string, data json.RawMessage)
}

// directFeederPublisher adapts the broadcaster to the feeder's FeederPublisher interface.
// Instead of publishing to Redis Pub/Sub, it queues updates for batched delivery
// via the 150ms flush ticker with subscription-aware, viewport-filtered routing.
type directFeederPublisher struct {
	ws queueingBroadcaster
}

func (p *directFeederPublisher) PublishLayerUpdate(_ context.Context, layerType string, data []byte) error {
	p.ws.QueueUpdate(layerType, json.RawMessage(data))
	return nil
}

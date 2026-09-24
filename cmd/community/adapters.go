package main

import (
	"context"
	"encoding/json"
)

// queueingBroadcaster is the minimal interface for batched message delivery.
// Satisfied by *realtime.Server.
type queueingBroadcaster interface {
	QueueUpdate(layerID string, data json.RawMessage)
}

// directFeederPublisher adapts the broadcaster to the feeder's FeederPublisher
// interface: it queues updates for batched delivery via the 150ms flush ticker
// with subscription-aware, viewport-filtered routing.
type directFeederPublisher struct {
	ws queueingBroadcaster
}

func (p *directFeederPublisher) PublishLayerUpdate(_ context.Context, layerType string, data []byte) error {
	p.ws.QueueUpdate(layerType, json.RawMessage(data))
	return nil
}

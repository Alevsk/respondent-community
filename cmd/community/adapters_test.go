package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockQueueingBroadcaster struct {
	mu      sync.Mutex
	updates []struct {
		layerID string
		data    json.RawMessage
	}
}

func (m *mockQueueingBroadcaster) QueueUpdate(layerID string, data json.RawMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updates = append(m.updates, struct {
		layerID string
		data    json.RawMessage
	}{layerID: layerID, data: data})
}

func TestDirectFeederPublisher_PublishLayerUpdate(t *testing.T) {
	mb := &mockQueueingBroadcaster{}
	pub := &directFeederPublisher{ws: mb}

	err := pub.PublishLayerUpdate(context.Background(), "flights", []byte(`{"type":"update"}`))
	require.NoError(t, err)

	mb.mu.Lock()
	defer mb.mu.Unlock()
	assert.Len(t, mb.updates, 1)
	assert.Equal(t, "flights", mb.updates[0].layerID)
	assert.Equal(t, `{"type":"update"}`, string(mb.updates[0].data))
}

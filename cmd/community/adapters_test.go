package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/infra/inmem"
)

func TestMemcacheFeederCache_SetEntity(t *testing.T) {
	mc := inmem.NewMemCache(0, time.Minute)
	defer func() { _ = mc.Close() }()
	adapter := &memcacheFeederCache{cache: mc}

	entity := &domain.Entity{
		ID:         "e1",
		ExternalID: "ext1",
		LayerType:  "flights",
		Name:       "Test",
	}
	obs := &domain.Observation{
		ID:       "o1",
		EntityID: "e1",
	}

	err := adapter.SetEntity(context.Background(), entity, obs, 5*time.Minute)
	require.NoError(t, err)

	// Verify entity is in cache.
	gotEntity, _, err := mc.GetEntity(context.Background(), "flights", "ext1")
	require.NoError(t, err)
	assert.Equal(t, "e1", gotEntity.ID)
}

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

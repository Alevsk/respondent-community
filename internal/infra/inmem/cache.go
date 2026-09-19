package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// cacheEntry stores an entity+observation pair with an optional expiration.
type cacheEntry struct {
	entity      *domain.Entity
	observation *domain.Observation
	expiresAt   time.Time // zero means never expires
}

func (e *cacheEntry) expired() bool {
	if e.expiresAt.IsZero() {
		return false
	}
	return time.Now().After(e.expiresAt)
}

// MemCache is a thread-safe in-memory cache implementing CacheStorage and SpatialCacheStorage.
type MemCache struct {
	mu          sync.RWMutex
	layers      map[string]map[string]*cacheEntry // layerType -> externalID -> entry
	defaultTTL  time.Duration
	layerTTLs   map[string]time.Duration // per-layer TTL overrides
	stopCh      chan struct{}
	sweepTicker *time.Ticker
}

// NewMemCache creates a new MemCache. defaultTTL=0 means entries never expire.
// sweepInterval controls how often expired entries are removed (default 10s if zero).
func NewMemCache(defaultTTL time.Duration, sweepInterval time.Duration) *MemCache {
	if sweepInterval <= 0 {
		sweepInterval = 10 * time.Second
	}
	c := &MemCache{
		layers:      make(map[string]map[string]*cacheEntry),
		defaultTTL:  defaultTTL,
		layerTTLs:   make(map[string]time.Duration),
		stopCh:      make(chan struct{}),
		sweepTicker: time.NewTicker(sweepInterval),
	}
	go c.sweepLoop()
	return c
}

// SetLayerTTL sets a TTL override for a specific layer type.
// This matches the per-layer TTL behavior of the Valkey adapter.
func (c *MemCache) SetLayerTTL(layerType string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.layerTTLs[layerType] = ttl
}

// ttlForLayer returns the effective TTL for the given layer type.
func (c *MemCache) ttlForLayer(layerType string) time.Duration {
	if ttl, ok := c.layerTTLs[layerType]; ok {
		return ttl
	}
	return c.defaultTTL
}

// sweepLoop runs in the background and removes expired entries.
func (c *MemCache) sweepLoop() {
	for {
		select {
		case <-c.sweepTicker.C:
			c.sweep()
		case <-c.stopCh:
			return
		}
	}
}

func (c *MemCache) sweep() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for layerType, entries := range c.layers {
		for key, entry := range entries {
			if entry.expired() {
				delete(entries, key)
			}
		}
		if len(entries) == 0 {
			delete(c.layers, layerType)
		}
	}
}

// SetEntity stores an entity and its latest observation in the cache.
func (c *MemCache) SetEntity(ctx context.Context, entity *domain.Entity, observation *domain.Observation) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.layers[entity.LayerType]; !ok {
		c.layers[entity.LayerType] = make(map[string]*cacheEntry)
	}

	entry := &cacheEntry{entity: entity, observation: observation}
	ttl := c.ttlForLayer(entity.LayerType)
	if ttl > 0 {
		entry.expiresAt = time.Now().Add(ttl)
	}
	c.layers[entity.LayerType][entity.ExternalID] = entry
	return nil
}

// GetEntity retrieves an entity and its observation by layer type and external ID.
func (c *MemCache) GetEntity(ctx context.Context, layerType, externalID string) (*domain.Entity, *domain.Observation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	layer, ok := c.layers[layerType]
	if !ok {
		return nil, nil, domain.NewNotFoundError(fmt.Sprintf("entity %s/%s not found in cache", layerType, externalID), nil)
	}
	entry, ok := layer[externalID]
	if !ok || entry.expired() {
		return nil, nil, domain.NewNotFoundError(fmt.Sprintf("entity %s/%s not found in cache", layerType, externalID), nil)
	}
	return entry.entity, entry.observation, nil
}

// GetLayerEntities returns a paginated slice of entities for a layer.
func (c *MemCache) GetLayerEntities(ctx context.Context, layerType string, limit, offset int) ([]*domain.Entity, []*domain.Observation, int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	layer := c.layers[layerType]
	total := int64(0)
	var entities []*domain.Entity
	var observations []*domain.Observation

	idx := 0
	for _, entry := range layer {
		if entry.expired() {
			continue
		}
		total++
		if idx < offset {
			idx++
			continue
		}
		if limit > 0 && len(entities) >= limit {
			continue
		}
		entities = append(entities, entry.entity)
		observations = append(observations, entry.observation)
		idx++
	}
	return entities, observations, total, nil
}

// GetLayerCount returns the number of non-expired entries for a layer.
func (c *MemCache) GetLayerCount(ctx context.Context, layerType string) (int64, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var count int64
	for _, entry := range c.layers[layerType] {
		if !entry.expired() {
			count++
		}
	}
	return count, nil
}

// ClearLayer removes all entries for a given layer.
func (c *MemCache) ClearLayer(ctx context.Context, layerType string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.layers, layerType)
	return nil
}

// GetStats returns diagnostic statistics.
func (c *MemCache) GetStats(ctx context.Context) (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]any)
	totalEntries := 0
	layerCounts := make(map[string]int)
	for layerType, entries := range c.layers {
		count := 0
		for _, entry := range entries {
			if !entry.expired() {
				count++
			}
		}
		layerCounts[layerType] = count
		totalEntries += count
	}
	stats["total_entries"] = totalEntries
	stats["layers"] = layerCounts
	return stats, nil
}

// HealthCheck always returns nil (in-process, always healthy).
func (c *MemCache) HealthCheck(ctx context.Context) error {
	return nil
}

// Close stops the background sweep goroutine.
func (c *MemCache) Close() error {
	c.sweepTicker.Stop()
	close(c.stopCh)
	return nil
}

// GetLayerEntitiesByBBox returns entities whose latest observation falls within the bounding box.
// Implements domain.SpatialCacheStorage.
func (c *MemCache) GetLayerEntitiesByBBox(ctx context.Context, layerType string, bbox domain.BBox, limit int) ([]*domain.Entity, []*domain.Observation, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var entities []*domain.Entity
	var observations []*domain.Observation

	for _, entry := range c.layers[layerType] {
		if entry.expired() {
			continue
		}
		if limit > 0 && len(entities) >= limit {
			break
		}
		obs := entry.observation
		if obs == nil || obs.Position == nil {
			continue
		}
		if bboxContains(bbox, obs.Position.Lat, obs.Position.Lon) {
			entities = append(entities, entry.entity)
			observations = append(observations, obs)
		}
	}
	return entities, observations, nil
}

// bboxContains checks whether (lat, lon) falls within bbox, handling antimeridian wrapping.
func bboxContains(bbox domain.BBox, lat, lon float64) bool {
	if lat < bbox.South || lat > bbox.North {
		return false
	}
	if bbox.West <= bbox.East {
		return lon >= bbox.West && lon <= bbox.East
	}
	// Antimeridian wrap: west > east (e.g., west=170, east=-170).
	return lon >= bbox.West || lon <= bbox.East
}

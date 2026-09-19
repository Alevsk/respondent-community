package layertest

import (
	"context"
	"sync"

	"github.com/Alevsk/respondent/internal/domain"
)

// FakeCacheStorage implements domain.CacheStorage for testing.
type FakeCacheStorage struct {
	mu        sync.RWMutex
	entities  map[string]*domain.Entity
	obs       map[string]*domain.Observation
	counts    map[string]int64
	setErr    error
	getErr    error
	clearErr  error
	healthErr error
}

func NewFakeCacheStorage() *FakeCacheStorage {
	return &FakeCacheStorage{
		entities: make(map[string]*domain.Entity),
		obs:      make(map[string]*domain.Observation),
		counts:   make(map[string]int64),
	}
}

// SetError configures errors returned by set, get, and clear operations.
func (f *FakeCacheStorage) SetError(setErr, getErr, clearErr error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.setErr = setErr
	f.getErr = getErr
	f.clearErr = clearErr
}

// SetHealthError configures the error returned by HealthCheck.
func (f *FakeCacheStorage) SetHealthError(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.healthErr = err
}

func (f *FakeCacheStorage) SetEntity(_ context.Context, entity *domain.Entity, obs *domain.Observation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	key := entity.LayerType + ":" + entity.ExternalID
	f.entities[key] = entity
	if obs != nil {
		f.obs[key] = obs
	}
	f.counts[entity.LayerType]++
	return nil
}

func (f *FakeCacheStorage) GetEntity(_ context.Context, layerType, externalID string) (*domain.Entity, *domain.Observation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.getErr != nil {
		return nil, nil, f.getErr
	}
	key := layerType + ":" + externalID
	e, ok := f.entities[key]
	if !ok {
		return nil, nil, domain.NewNotFoundError("not in cache", nil)
	}
	return e, f.obs[key], nil
}

func (f *FakeCacheStorage) GetLayerEntities(_ context.Context, layerType string, limit, offset int) ([]*domain.Entity, []*domain.Observation, int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.getErr != nil {
		return nil, nil, 0, f.getErr
	}
	var entities []*domain.Entity
	var observations []*domain.Observation
	for key, e := range f.entities {
		if e.LayerType == layerType {
			entities = append(entities, e)
			observations = append(observations, f.obs[key])
		}
	}
	total := int64(len(entities))
	if offset >= len(entities) {
		return []*domain.Entity{}, []*domain.Observation{}, total, nil
	}
	end := offset + limit
	if end > len(entities) {
		end = len(entities)
	}
	return entities[offset:end], observations[offset:end], total, nil
}

func (f *FakeCacheStorage) GetLayerCount(_ context.Context, layerType string) (int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.getErr != nil {
		return 0, f.getErr
	}
	return f.counts[layerType], nil
}

func (f *FakeCacheStorage) ClearLayer(_ context.Context, layerType string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.clearErr != nil {
		return f.clearErr
	}
	for key, e := range f.entities {
		if e.LayerType == layerType {
			delete(f.entities, key)
			delete(f.obs, key)
		}
	}
	delete(f.counts, layerType)
	return nil
}

func (f *FakeCacheStorage) GetStats(_ context.Context) (map[string]any, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return map[string]any{
		"entity_count":      len(f.entities),
		"observation_count": len(f.obs),
		"layer_counts":      f.counts,
	}, nil
}

func (f *FakeCacheStorage) HealthCheck(_ context.Context) error {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.healthErr
}

func (f *FakeCacheStorage) Close() error { return nil }

var _ domain.CacheStorage = (*FakeCacheStorage)(nil)

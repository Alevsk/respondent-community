package entitytest

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// FakeEntityRepo implements domain.EntityRepository for testing.
type FakeEntityRepo struct {
	mu       sync.RWMutex
	Entities map[string]*domain.Entity // keyed by ID
}

func NewFakeEntityRepo() *FakeEntityRepo {
	return &FakeEntityRepo{Entities: make(map[string]*domain.Entity)}
}

func (f *FakeEntityRepo) Create(_ context.Context, e *domain.Entity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Entities[e.ID] = e
	return nil
}

func (f *FakeEntityRepo) CreateBatch(_ context.Context, entities []*domain.Entity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range entities {
		f.Entities[e.ID] = e
	}
	return nil
}

func (f *FakeEntityRepo) GetByID(_ context.Context, id string) (*domain.Entity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.Entities[id]
	if !ok {
		return nil, domain.NewNotFoundError(fmt.Sprintf("entity %s not found", id), nil)
	}
	return e, nil
}

func (f *FakeEntityRepo) GetByIDs(_ context.Context, ids []string) ([]*domain.Entity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	var result []*domain.Entity
	for _, id := range ids {
		if e, ok := f.Entities[id]; ok {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *FakeEntityRepo) GetByExternalID(_ context.Context, layerType, externalID string) (*domain.Entity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, e := range f.Entities {
		if e.LayerType == layerType && e.ExternalID == externalID {
			return e, nil
		}
	}
	return nil, domain.NewNotFoundError(fmt.Sprintf("entity %s:%s not found", layerType, externalID), nil)
}

func (f *FakeEntityRepo) GetByExternalIDs(_ context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	idSet := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		idSet[id] = true
	}
	var result []*domain.Entity
	for _, e := range f.Entities {
		if e.LayerType == layerType && idSet[e.ExternalID] {
			result = append(result, e)
		}
	}
	return result, nil
}

func (f *FakeEntityRepo) GetDistinctLayerTypes(_ context.Context) ([]string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	seen := make(map[string]bool)
	for _, e := range f.Entities {
		seen[e.LayerType] = true
	}
	var result []string
	for lt := range seen {
		result = append(result, lt)
	}
	return result, nil
}

func (f *FakeEntityRepo) CountByLayerType(_ context.Context) (map[string]int64, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	counts := make(map[string]int64)
	for _, e := range f.Entities {
		counts[e.LayerType]++
	}
	return counts, nil
}

func (f *FakeEntityRepo) Update(_ context.Context, e *domain.Entity) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Entities[e.ID] = e
	return nil
}

func (f *FakeEntityRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.Entities, id)
	return nil
}

func (f *FakeEntityRepo) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}

func (f *FakeEntityRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

func (f *FakeEntityRepo) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

// FakeObservationRepo implements domain.ObservationRepository for testing.
type FakeObservationRepo struct {
	mu           sync.RWMutex
	Observations map[string][]*domain.Observation // keyed by EntityID
}

func NewFakeObservationRepo() *FakeObservationRepo {
	return &FakeObservationRepo{Observations: make(map[string][]*domain.Observation)}
}

func (f *FakeObservationRepo) Create(_ context.Context, obs *domain.Observation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Observations[obs.EntityID] = append(f.Observations[obs.EntityID], obs)
	return nil
}

func (f *FakeObservationRepo) CreateBatch(_ context.Context, observations []*domain.Observation) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, obs := range observations {
		f.Observations[obs.EntityID] = append(f.Observations[obs.EntityID], obs)
	}
	return nil
}

func (f *FakeObservationRepo) CreateBatchUpsert(ctx context.Context, observations []*domain.Observation) error {
	return f.CreateBatch(ctx, observations)
}

func (f *FakeObservationRepo) GetByEntityID(_ context.Context, entityID string, limit int, _ time.Time) ([]*domain.Observation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obs := f.Observations[entityID]
	if limit > 0 && len(obs) > limit {
		obs = obs[:limit]
	}
	return obs, nil
}

func (f *FakeObservationRepo) GetLatest(_ context.Context, entityID string) (*domain.Observation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	obs := f.Observations[entityID]
	if len(obs) == 0 {
		return nil, domain.NewNotFoundError("no observations", nil)
	}
	return obs[len(obs)-1], nil
}

func (f *FakeObservationRepo) CountLatestForLayer(_ context.Context, _ string) (int64, error) {
	return 0, nil
}

func (f *FakeObservationRepo) GetLatestForLayerPage(_ context.Context, _ string, _, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (f *FakeObservationRepo) GetLatestForEntityIDs(_ context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	result := make(map[string]*domain.Observation)
	for _, id := range entityIDs {
		if obs := f.Observations[id]; len(obs) > 0 {
			result[id] = obs[len(obs)-1]
		}
	}
	return result, nil
}

func (f *FakeObservationRepo) GetLatestContentHashes(_ context.Context, _ []string) (map[string]string, error) {
	return nil, nil
}

func (f *FakeObservationRepo) GetLayerSnapshotAt(_ context.Context, _ string, _ time.Time, _ time.Duration, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (f *FakeObservationRepo) GetLatestForLayerByBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (f *FakeObservationRepo) GetLatestByCurrentPositionInBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (f *FakeObservationRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	return nil
}

// Compile-time interface verification.
var (
	_ domain.EntityRepository      = (*FakeEntityRepo)(nil)
	_ domain.ObservationRepository = (*FakeObservationRepo)(nil)
)

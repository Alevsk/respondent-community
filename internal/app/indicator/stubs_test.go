package indicator

import (
	"context"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// Compile-time interface assertions.
var (
	_ domain.EntityRepository      = (*stubEntityRepo)(nil)
	_ domain.ObservationRepository = (*stubObsRepo)(nil)
)

// stubEntityRepo is a minimal in-memory stub for domain.EntityRepository.
type stubEntityRepo struct {
	mu       sync.RWMutex
	entities map[string]*domain.Entity
	getErr   error // returned by every Get call when non-nil
}

func newStubEntityRepo() *stubEntityRepo {
	return &stubEntityRepo{entities: make(map[string]*domain.Entity)}
}

func (s *stubEntityRepo) add(e *domain.Entity) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[e.ID] = e
}

func (s *stubEntityRepo) Create(_ context.Context, e *domain.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[e.ID] = e
	return nil
}

func (s *stubEntityRepo) CreateBatch(_ context.Context, entities []*domain.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range entities {
		s.entities[e.ID] = e
	}
	return nil
}

func (s *stubEntityRepo) GetByID(_ context.Context, id string) (*domain.Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	e, ok := s.entities[id]
	if !ok {
		return nil, domain.NewNotFoundError("entity not found", nil)
	}
	return e, nil
}

func (s *stubEntityRepo) GetByIDs(_ context.Context, ids []string) ([]*domain.Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	out := make([]*domain.Entity, 0, len(ids))
	for _, id := range ids {
		if e, ok := s.entities[id]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *stubEntityRepo) GetByExternalID(_ context.Context, layerType, externalID string) (*domain.Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	for _, e := range s.entities {
		if e.LayerType == layerType && e.ExternalID == externalID {
			return e, nil
		}
	}
	return nil, domain.NewNotFoundError("entity not found", nil)
}

func (s *stubEntityRepo) GetByExternalIDs(_ context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	idSet := make(map[string]bool, len(externalIDs))
	for _, id := range externalIDs {
		idSet[id] = true
	}
	var out []*domain.Entity
	for _, e := range s.entities {
		if e.LayerType == layerType && idSet[e.ExternalID] {
			out = append(out, e)
		}
	}
	return out, nil
}

func (s *stubEntityRepo) Update(_ context.Context, e *domain.Entity) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entities[e.ID] = e
	return nil
}

func (s *stubEntityRepo) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entities, id)
	return nil
}

func (s *stubEntityRepo) GetDistinctLayerTypes(_ context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	seen := make(map[string]bool)
	for _, e := range s.entities {
		seen[e.LayerType] = true
	}
	var types []string
	for lt := range seen {
		types = append(types, lt)
	}
	return types, nil
}

func (s *stubEntityRepo) CountByLayerType(_ context.Context) (map[string]int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	counts := make(map[string]int64)
	for _, e := range s.entities {
		counts[e.LayerType]++
	}
	return counts, nil
}

func (s *stubEntityRepo) SearchEntities(_ context.Context, _ string, _ string, _ int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}

func (s *stubEntityRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	panic("not implemented")
}

func (s *stubEntityRepo) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

// stubObsRepo is a minimal in-memory stub for domain.ObservationRepository.
type stubObsRepo struct {
	mu           sync.RWMutex
	observations []*domain.Observation
	getErr       error // returned by every Get call when non-nil
}

func newStubObsRepo() *stubObsRepo { return &stubObsRepo{} }

func (s *stubObsRepo) add(o *domain.Observation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, o)
}

func (s *stubObsRepo) Create(_ context.Context, o *domain.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, o)
	return nil
}

func (s *stubObsRepo) CreateBatch(_ context.Context, obs []*domain.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, obs...)
	return nil
}

func (s *stubObsRepo) GetByEntityID(_ context.Context, entityID string, limit int, before time.Time) ([]*domain.Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	var out []*domain.Observation
	for _, o := range s.observations {
		if o.EntityID == entityID && o.Timestamp.Before(before) {
			out = append(out, o)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *stubObsRepo) GetLatest(_ context.Context, entityID string) (*domain.Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	var latest *domain.Observation
	for _, o := range s.observations {
		if o.EntityID == entityID {
			if latest == nil || o.Timestamp.After(latest.Timestamp) {
				latest = o
			}
		}
	}
	if latest == nil {
		return nil, domain.NewNotFoundError("observation not found", nil)
	}
	return latest, nil
}

func (s *stubObsRepo) GetLatestForLayer(_ context.Context, layerType string, limit int) ([]*domain.Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	var out []*domain.Observation
	for _, o := range s.observations {
		if o.SourceType == layerType {
			out = append(out, o)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (s *stubObsRepo) CreateBatchUpsert(_ context.Context, obs []*domain.Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observations = append(s.observations, obs...)
	return nil
}

func (s *stubObsRepo) GetLatestForEntityIDs(_ context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	result := make(map[string]*domain.Observation, len(entityIDs))
	idSet := make(map[string]bool, len(entityIDs))
	for _, id := range entityIDs {
		idSet[id] = true
	}
	for _, o := range s.observations {
		if !idSet[o.EntityID] {
			continue
		}
		if existing, ok := result[o.EntityID]; !ok || o.Timestamp.After(existing.Timestamp) {
			result[o.EntityID] = o
		}
	}
	return result, nil
}

func (s *stubObsRepo) GetLatestContentHashes(_ context.Context, entityIDs []string) (map[string]string, error) {
	return make(map[string]string), nil
}

func (s *stubObsRepo) GetLayerSnapshotAt(_ context.Context, _ string, _ time.Time, _ time.Duration) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (s *stubObsRepo) GetLatestForLayerByBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (s *stubObsRepo) GetLatestByCurrentPositionInBBox(_ context.Context, _ string, _, _, _, _ float64, _, _ time.Time, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (s *stubObsRepo) PatchAIMetadata(_ context.Context, _ string, _ map[string]any) error {
	panic("not implemented")
}

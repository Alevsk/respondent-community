package feeder

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
)

// --- stubs: only the methods persistEntities reaches ---

type publishTestEntityRepo struct{ domain.EntityRepository }

func (r *publishTestEntityRepo) CreateBatch(_ context.Context, _ []*domain.Entity) error { return nil }

func (r *publishTestEntityRepo) GetByExternalIDs(_ context.Context, layerType string, extIDs []string) ([]*domain.Entity, error) {
	out := make([]*domain.Entity, 0, len(extIDs))
	for _, id := range extIDs {
		out = append(out, &domain.Entity{ID: "db-" + id, ExternalID: id, LayerType: layerType})
	}
	return out, nil
}

type publishTestObsRepo struct{ domain.ObservationRepository }

func (r *publishTestObsRepo) CreateBatch(_ context.Context, _ []*domain.Observation) error { return nil }
func (r *publishTestObsRepo) CreateBatchUpsert(_ context.Context, _ []*domain.Observation) error {
	return nil
}

func (r *publishTestObsRepo) GetLatestContentHashes(_ context.Context, _ []string) (map[string]string, error) {
	return map[string]string{}, nil
}

// failingCache stands in for a hot cache that is unavailable.
type failingCache struct{}

func (failingCache) SetEntity(_ context.Context, _ *domain.Entity, _ *domain.Observation, _ time.Duration) error {
	return errors.New("cache unavailable")
}

type recordingPublisher struct {
	mu      sync.Mutex
	layers  []string
	payload int
}

func (p *recordingPublisher) PublishLayerUpdate(_ context.Context, layerType string, data []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.layers = append(p.layers, layerType)
	p.payload += len(data)
	return nil
}

// The Pub/Sub publish that drives every live globe update was nested inside the
// hot-cache block, after a `continue` on cache error. A cache that is failing —
// or absent — therefore silenced the live globe entirely, even though the
// entities had already been written to the durable store. Broadcasting is not
// conditional on caching.
func TestPersistEntitiesPublishesWhenTheCacheFails(t *testing.T) {
	pub := &recordingPublisher{}
	s := &IngestionService{
		logger:        zerolog.Nop(),
		entityRepo:    &publishTestEntityRepo{},
		obsRepo:       &publishTestObsRepo{},
		cache:         failingCache{},
		publisher:     pub,
		sourceConfigs: map[string]SourceConfig{"src": {Name: "src"}},
		stopCh:        make(chan struct{}),
		sem:           make(chan struct{}, 1),
	}

	entities := []*domain.Entity{
		{ID: "e1", ExternalID: "AAA111", LayerType: "flights_commercial"},
		{ID: "e2", ExternalID: "BBB222", LayerType: "flights_commercial"},
	}
	observations := []*domain.Observation{
		{ID: "o1", EntityID: "e1", Timestamp: time.Now()},
		{ID: "o2", EntityID: "e2", Timestamp: time.Now()},
	}

	if err := s.persistEntities(context.Background(), "src", entities, observations); err != nil {
		t.Fatalf("persistEntities: %v", err)
	}

	if len(pub.layers) != len(entities) {
		t.Errorf("published %d updates, want %d: a failing cache must not silence the live globe",
			len(pub.layers), len(entities))
	}
}

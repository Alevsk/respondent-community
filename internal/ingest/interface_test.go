package ingest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/stretchr/testify/assert"
)

type mockLayerSource struct {
	name             string
	layerType        string
	supportedTypes   []domain.SourceType
	startErr         error
	stopErr          error
	snapshotEntities []*domain.Entity
	snapshotObs      []*domain.Observation
	snapshotErr      error
	streamErr        error
	startCalled      bool
	stopCalled       bool
	startMutex       sync.Mutex
	stopMutex        sync.Mutex
	blockOnStream    bool
	streamBlockChan  chan struct{}
}

func newMockLayerSource(name, layerType string) *mockLayerSource {
	return &mockLayerSource{
		name:            name,
		layerType:       layerType,
		supportedTypes:  []domain.SourceType{},
		streamBlockChan: make(chan struct{}),
	}
}

func (m *mockLayerSource) Start(ctx context.Context) error {
	m.startMutex.Lock()
	m.startCalled = true
	m.startMutex.Unlock()
	return m.startErr
}

func (m *mockLayerSource) Stop() error {
	m.stopMutex.Lock()
	m.stopCalled = true
	m.stopMutex.Unlock()
	return m.stopErr
}

func (m *mockLayerSource) Snapshot(ctx context.Context) ([]*domain.Entity, []*domain.Observation, error) {
	return m.snapshotEntities, m.snapshotObs, m.snapshotErr
}

func (m *mockLayerSource) Stream(ctx context.Context, out chan<- *domain.EntityUpdate) error {
	if m.blockOnStream {
		<-m.streamBlockChan
	}
	return m.streamErr
}

func (m *mockLayerSource) Name() string {
	return m.name
}

func (m *mockLayerSource) LayerType() string {
	return m.layerType
}

func (m *mockLayerSource) SupportsSourceType(sourceType domain.SourceType) bool {
	for _, t := range m.supportedTypes {
		if t == sourceType {
			return true
		}
	}
	return false
}

type mockEntityRepository struct {
	mu                sync.Mutex
	createFunc        func(ctx context.Context, entity *domain.Entity) error
	getByExternalID   func(ctx context.Context, layerType, externalID string) (*domain.Entity, error)
	updateFunc        func(ctx context.Context, entity *domain.Entity) error
	deleteFunc        func(ctx context.Context, id string) error
	createCount       int
	updateCount       int
	deleteCount       int
	createErr         error
	updateErr         error
	deleteErr         error
	getByExternalIDFn func(ctx context.Context, layerType, externalID string) (*domain.Entity, error)
}

func (m *mockEntityRepository) Create(ctx context.Context, entity *domain.Entity) error {
	m.mu.Lock()
	m.createCount++
	m.mu.Unlock()
	if m.createFunc != nil {
		return m.createFunc(ctx, entity)
	}
	return m.createErr
}

func (m *mockEntityRepository) CreateBatch(ctx context.Context, entities []*domain.Entity) error {
	m.mu.Lock()
	m.createCount += len(entities)
	m.mu.Unlock()
	return m.createErr
}

func (m *mockEntityRepository) GetByID(ctx context.Context, id string) (*domain.Entity, error) {
	return nil, nil
}

func (m *mockEntityRepository) GetByIDs(ctx context.Context, ids []string) ([]*domain.Entity, error) {
	return nil, nil
}

func (m *mockEntityRepository) GetByExternalID(ctx context.Context, layerType, externalID string) (*domain.Entity, error) {
	if m.getByExternalIDFn != nil {
		return m.getByExternalIDFn(ctx, layerType, externalID)
	}
	if m.getByExternalID != nil {
		return m.getByExternalID(ctx, layerType, externalID)
	}
	return nil, errors.New("not found")
}

func (m *mockEntityRepository) GetByExternalIDs(ctx context.Context, layerType string, externalIDs []string) ([]*domain.Entity, error) {
	return nil, nil
}

func (m *mockEntityRepository) GetDistinctLayerTypes(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockEntityRepository) CountByLayerType(ctx context.Context) (map[string]int64, error) {
	return nil, nil
}

func (m *mockEntityRepository) Update(ctx context.Context, entity *domain.Entity) error {
	m.mu.Lock()
	m.updateCount++
	m.mu.Unlock()
	if m.updateFunc != nil {
		return m.updateFunc(ctx, entity)
	}
	return m.updateErr
}

func (m *mockEntityRepository) Delete(ctx context.Context, id string) error {
	m.mu.Lock()
	m.deleteCount++
	m.mu.Unlock()
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return m.deleteErr
}

func (m *mockEntityRepository) SearchEntities(ctx context.Context, query string, layerType string, limit int) ([]*domain.EntitySearchResult, int, error) {
	return nil, 0, nil
}

func (m *mockEntityRepository) PatchAIMetadata(ctx context.Context, entityID string, metadata map[string]any) error {
	panic("not implemented")
}

func (m *mockEntityRepository) UpdateCoordinates(_ context.Context, _ string, _, _ float64) error {
	return nil
}

type mockObservationRepository struct {
	createFunc  func(ctx context.Context, obs *domain.Observation) error
	createCount int
	createErr   error
}

func (m *mockObservationRepository) Create(ctx context.Context, observation *domain.Observation) error {
	m.createCount++
	if m.createFunc != nil {
		return m.createFunc(ctx, observation)
	}
	return m.createErr
}

func (m *mockObservationRepository) CreateBatch(ctx context.Context, observations []*domain.Observation) error {
	m.createCount += len(observations)
	return m.createErr
}

func (m *mockObservationRepository) CreateBatchUpsert(ctx context.Context, observations []*domain.Observation) error {
	return nil
}

func (m *mockObservationRepository) GetByEntityID(ctx context.Context, entityID string, limit int, before time.Time) ([]*domain.Observation, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatest(ctx context.Context, entityID string) (*domain.Observation, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatestForLayerPage(_ context.Context, _ string, _, _ int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatestForEntityIDs(ctx context.Context, entityIDs []string) (map[string]*domain.Observation, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatestContentHashes(ctx context.Context, entityIDs []string) (map[string]string, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLayerSnapshotAt(ctx context.Context, layerType string, asOf time.Time, window time.Duration) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatestForLayerByBBox(ctx context.Context, layerType string, south, north, west, east float64, from, to time.Time, limit int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (m *mockObservationRepository) GetLatestByCurrentPositionInBBox(ctx context.Context, layerType string, south, north, west, east float64, from, to time.Time, limit int) ([]*domain.EntitySnapshot, error) {
	return nil, nil
}

func (m *mockObservationRepository) PatchAIMetadata(ctx context.Context, observationID string, metadata map[string]any) error {
	panic("not implemented")
}

func TestNewSourceRegistry(t *testing.T) {
	t.Run("creates empty registry", func(t *testing.T) {
		registry := NewSourceRegistry()
		assert.NotNil(t, registry)
		assert.NotNil(t, registry.sources)
		assert.Empty(t, registry.sources)
	})

	t.Run("creates independent registries", func(t *testing.T) {
		registry1 := NewSourceRegistry()
		registry2 := NewSourceRegistry()
		assert.NotNil(t, registry1)
		assert.NotNil(t, registry2)
		source := newMockLayerSource("test", "layer")
		registry1.Register(source)
		_, ok := registry2.Get("test")
		assert.False(t, ok)
	})
}

func TestSourceRegistry_Register(t *testing.T) {
	t.Run("registers single source", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		registry.Register(source)
		assert.Len(t, registry.sources, 1)
		assert.Contains(t, registry.sources, "test-source")
	})

	t.Run("registers multiple sources", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("source1", "layer1")
		source2 := newMockLayerSource("source2", "layer2")
		source3 := newMockLayerSource("source3", "layer3")
		registry.Register(source1)
		registry.Register(source2)
		registry.Register(source3)
		assert.Len(t, registry.sources, 3)
	})

	t.Run("overwrites existing source with same name", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("same-name", "layer1")
		source2 := newMockLayerSource("same-name", "layer2")
		registry.Register(source1)
		registry.Register(source2)
		assert.Len(t, registry.sources, 1)
		retrieved, ok := registry.sources["same-name"]
		assert.True(t, ok)
		assert.Equal(t, "layer2", retrieved.LayerType())
	})

	t.Run("is thread safe for concurrent writes", func(t *testing.T) {
		registry := NewSourceRegistry()
		var wg sync.WaitGroup
		numGoroutines := 100
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				source := newMockLayerSource("source-"+string(rune(idx)), "layer")
				registry.Register(source)
			}(i)
		}
		wg.Wait()
	})
}

func TestSourceRegistry_Get(t *testing.T) {
	t.Run("returns source when exists", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		registry.Register(source)
		retrieved, ok := registry.Get("test-source")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-source", retrieved.Name())
		assert.Equal(t, "test-layer", retrieved.LayerType())
	})

	t.Run("returns false when source does not exist", func(t *testing.T) {
		registry := NewSourceRegistry()
		retrieved, ok := registry.Get("non-existent")
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("returns correct source from multiple", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("source1", "layer1")
		source2 := newMockLayerSource("source2", "layer2")
		registry.Register(source1)
		registry.Register(source2)
		retrieved, ok := registry.Get("source2")
		assert.True(t, ok)
		assert.Equal(t, "source2", retrieved.Name())
		assert.Equal(t, "layer2", retrieved.LayerType())
	})

	t.Run("is thread safe for concurrent reads", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		registry.Register(source)
		var wg sync.WaitGroup
		numGoroutines := 100
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				retrieved, ok := registry.Get("test-source")
				assert.True(t, ok)
				assert.NotNil(t, retrieved)
			}()
		}
		wg.Wait()
	})

	t.Run("is thread safe for concurrent read and write", func(t *testing.T) {
		registry := NewSourceRegistry()
		var wg sync.WaitGroup
		numReaders := 50
		numWriters := 50
		wg.Add(numReaders + numWriters)
		for i := 0; i < numWriters; i++ {
			go func(idx int) {
				defer wg.Done()
				source := newMockLayerSource("source-"+string(rune(idx)), "layer")
				registry.Register(source)
			}(i)
		}
		for i := 0; i < numReaders; i++ {
			go func() {
				defer wg.Done()
				registry.Get("source-0")
			}()
		}
		wg.Wait()
	})
}

func TestSourceRegistry_GetByType(t *testing.T) {
	t.Run("returns source when type matches", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		source.supportedTypes = []domain.SourceType{"opensky"}
		registry.Register(source)
		retrieved, ok := registry.GetByType("opensky")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
		assert.Equal(t, "test-source", retrieved.Name())
	})

	t.Run("returns false when no source supports type", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		source.supportedTypes = []domain.SourceType{"opensky"}
		registry.Register(source)
		retrieved, ok := registry.GetByType("unknown-type")
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("returns false when registry is empty", func(t *testing.T) {
		registry := NewSourceRegistry()
		retrieved, ok := registry.GetByType("any-type")
		assert.False(t, ok)
		assert.Nil(t, retrieved)
	})

	t.Run("returns first matching source", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("source1", "layer1")
		source1.supportedTypes = []domain.SourceType{"type-a"}
		source2 := newMockLayerSource("source2", "layer2")
		source2.supportedTypes = []domain.SourceType{"type-a", "type-b"}
		source3 := newMockLayerSource("source3", "layer3")
		source3.supportedTypes = []domain.SourceType{"type-b"}
		registry.Register(source1)
		registry.Register(source2)
		registry.Register(source3)
		retrieved, ok := registry.GetByType("type-b")
		assert.True(t, ok)
		assert.NotNil(t, retrieved)
	})

	t.Run("is thread safe for concurrent reads", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("test-source", "test-layer")
		source.supportedTypes = []domain.SourceType{"test-type"}
		registry.Register(source)
		var wg sync.WaitGroup
		numGoroutines := 100
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				retrieved, ok := registry.GetByType("test-type")
				assert.True(t, ok)
				assert.NotNil(t, retrieved)
			}()
		}
		wg.Wait()
	})
}

func TestSourceRegistry_List(t *testing.T) {
	t.Run("returns empty slice for empty registry", func(t *testing.T) {
		registry := NewSourceRegistry()
		sources := registry.List()
		assert.NotNil(t, sources)
		assert.Empty(t, sources)
	})

	t.Run("returns all registered sources", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("source1", "layer1")
		source2 := newMockLayerSource("source2", "layer2")
		source3 := newMockLayerSource("source3", "layer3")
		registry.Register(source1)
		registry.Register(source2)
		registry.Register(source3)
		sources := registry.List()
		assert.Len(t, sources, 3)
		names := make(map[string]bool)
		for _, s := range sources {
			names[s.Name()] = true
		}
		assert.True(t, names["source1"])
		assert.True(t, names["source2"])
		assert.True(t, names["source3"])
	})

	t.Run("returns copy of sources slice", func(t *testing.T) {
		registry := NewSourceRegistry()
		source := newMockLayerSource("source1", "layer1")
		registry.Register(source)
		sources1 := registry.List()
		sources2 := registry.List()
		assert.NotSame(t, &sources1[0], &sources2[0])
	})

	t.Run("is thread safe for concurrent reads", func(t *testing.T) {
		registry := NewSourceRegistry()
		source1 := newMockLayerSource("source1", "layer1")
		source2 := newMockLayerSource("source2", "layer2")
		registry.Register(source1)
		registry.Register(source2)
		var wg sync.WaitGroup
		numGoroutines := 100
		wg.Add(numGoroutines)
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer wg.Done()
				sources := registry.List()
				assert.Len(t, sources, 2)
			}()
		}
		wg.Wait()
	})
}

func TestSourceConfig(t *testing.T) {
	t.Run("creates config with all fields", func(t *testing.T) {
		config := SourceConfig{
			Name:      "test-source",
			Type:      "opensky",
			Config:    map[string]any{"key": "value"},
			Enabled:   true,
			Interval:  time.Minute * 5,
			RateLimit: 100,
		}
		assert.Equal(t, "test-source", config.Name)
		assert.Equal(t, "opensky", config.Type)
		assert.Equal(t, map[string]any{"key": "value"}, config.Config)
		assert.True(t, config.Enabled)
		assert.Equal(t, time.Minute*5, config.Interval)
		assert.Equal(t, 100, config.RateLimit)
	})

	t.Run("creates config with zero values", func(t *testing.T) {
		config := SourceConfig{}
		assert.Empty(t, config.Name)
		assert.Empty(t, config.Type)
		assert.Nil(t, config.Config)
		assert.False(t, config.Enabled)
		assert.Equal(t, time.Duration(0), config.Interval)
		assert.Equal(t, 0, config.RateLimit)
	})
}

var _ domain.EntityRepository = (*mockEntityRepository)(nil)
var _ domain.ObservationRepository = (*mockObservationRepository)(nil)
var _ LayerSource = (*mockLayerSource)(nil)

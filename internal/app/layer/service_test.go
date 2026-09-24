package layer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// ---------------------------------------------------------------------------
// Inline test stubs for domain.EntityRepository and domain.ObservationRepository
//
// The existing mocks in internal/repo/mocks implement repo.EntityStorage and
// repo.ObservationStorage (different interfaces). LayerService requires the
// narrower domain.EntityRepository / domain.ObservationRepository contracts, so
// we provide lightweight, controllable stubs here.
// ---------------------------------------------------------------------------

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
	countErr error // returned by CountByLayerType when non-nil
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
	if s.countErr != nil {
		return nil, s.countErr
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
	// entities models the JOIN the real page query performs. An observation
	// whose entity is absent cannot come back from an INNER JOIN, so the stub
	// must not invent one either.
	entities *stubEntityRepo
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

func (s *stubObsRepo) GetLatestForLayerPage(_ context.Context, layerType string, limit, offset int) ([]*domain.EntitySnapshot, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.getErr != nil {
		return nil, s.getErr
	}
	var snaps []*domain.EntitySnapshot
	for _, o := range s.observations {
		snap := &domain.EntitySnapshot{Observation: *o}
		if s.entities != nil {
			s.entities.mu.RLock()
			e, ok := s.entities.entities[o.EntityID]
			s.entities.mu.RUnlock()
			if !ok || e.LayerType != layerType {
				continue
			}
			snap.Entity = *e
		}
		snaps = append(snaps, snap)
	}
	if offset >= len(snaps) {
		return nil, nil
	}
	snaps = snaps[offset:]
	if limit > 0 && limit < len(snaps) {
		snaps = snaps[:limit]
	}
	return snaps, nil
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
	// Find the latest observation per entity ID.
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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// makeEntity creates a test entity with the given ID and layer type.
func makeEntity(id, layerType, externalID string) *domain.Entity {
	return &domain.Entity{
		ID:         id,
		ExternalID: externalID,
		LayerType:  layerType,
		Name:       "Entity " + id,
		CreatedAt:  time.Now(),
	}
}

// makeObservation creates a test observation tied to entityID, with SourceType
// set to layerType so GetLatestForLayer can match it.
func makeObservation(id, entityID, layerType string) *domain.Observation {
	return &domain.Observation{
		ID:         id,
		EntityID:   entityID,
		SourceType: layerType,
		Timestamp:  time.Now(),
		Position:   &domain.GeoPoint{Lat: 1.0, Lon: 2.0},
	}
}

// ---------------------------------------------------------------------------
// Tests: NewLayerService
// ---------------------------------------------------------------------------

func TestNewLayerService(t *testing.T) {
	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	if svc == nil {
		t.Fatal("expected non-nil LayerService")
	}
	if svc.entityRepo != entityRepo {
		t.Error("entityRepo not wired correctly")
	}
	if svc.obsRepo != obsRepo {
		t.Error("obsRepo not wired correctly")
	}
}

// declaring builds a registry in which every named layer type is declared by a
// source, the way a loaded YAML source declares it. GetLayers reports only
// declared layers, so a test that expects a layer must declare it.
func declaring(layerTypes ...string) *domain.DynamicSourceRegistry {
	reg := domain.NewDynamicSourceRegistry()
	for _, lt := range layerTypes {
		reg.Register(domain.SourceType(lt), domain.LayerType(lt))
	}
	return reg
}

// ---------------------------------------------------------------------------
// Tests: GetLayers
// ---------------------------------------------------------------------------

func TestGetLayers(t *testing.T) {
	t.Run("returns_empty_when_no_entities_in_db", func(t *testing.T) {
		svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), domain.NewDynamicSourceRegistry())
		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 0 {
			t.Errorf("expected 0 layers, got %d", len(layers))
		}
	})

	t.Run("returns_layer_per_distinct_db_type", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))
		entityRepo.add(makeEntity("e2", "usgs_earthquakes", "ext2"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights", "usgs_earthquakes"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 2 {
			t.Errorf("expected 2 layers, got %d", len(layers))
		}
	})

	t.Run("layers_sorted_by_id", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "usgs_earthquakes", "ext1"))
		entityRepo.add(makeEntity("e2", "adsb_lol_flights", "ext2"))
		entityRepo.add(makeEntity("e3", "celes_trak_satellites", "ext3"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("usgs_earthquakes", "adsb_lol_flights", "celes_trak_satellites"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for i := 1; i < len(layers); i++ {
			if layers[i-1].ID >= layers[i].ID {
				t.Errorf("layers not sorted: %q >= %q", layers[i-1].ID, layers[i].ID)
			}
		}
	})

	t.Run("all_db_layers_are_enabled", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))
		entityRepo.add(makeEntity("e2", "usgs_earthquakes", "ext2"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights", "usgs_earthquakes"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, l := range layers {
			if !l.Enabled {
				t.Errorf("layer %q: expected Enabled=true, got false", l.ID)
			}
		}
	})

	t.Run("layer_fields_populated_correctly", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}

		l := layers[0]
		if l.ID != "adsb_lol_flights" {
			t.Errorf("ID: got %q, want %q", l.ID, "adsb_lol_flights")
		}
		if l.Type != "adsb_lol_flights" {
			t.Errorf("Type: got %q, want %q", l.Type, "adsb_lol_flights")
		}
		if l.Source != "feeder" {
			t.Errorf("Source: got %q, want %q", l.Source, "feeder")
		}
		if l.Mode != "full" {
			t.Errorf("Mode: got %q, want %q", l.Mode, "full")
		}
		if l.Density != 100 {
			t.Errorf("Density: got %d, want 100", l.Density)
		}
		if !l.Enabled {
			t.Error("Enabled: got false, want true")
		}
	})

	t.Run("layer_count_populated_from_db", func(t *testing.T) {
		// Count is the durable per-layer entity total from the repository — the
		// same store layer discovery uses.
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))
		entityRepo.add(makeEntity("e2", "adsb_lol_flights", "ext2"))
		entityRepo.add(makeEntity("e3", "adsb_lol_flights", "ext3"))
		entityRepo.add(makeEntity("e4", "usgs_earthquakes", "ext4"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights", "usgs_earthquakes"))
		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		counts := make(map[string]int64)
		for _, l := range layers {
			counts[l.ID] = l.Count
		}
		if counts["adsb_lol_flights"] != 3 {
			t.Errorf("adsb_lol_flights count: got %d, want 3", counts["adsb_lol_flights"])
		}
		if counts["usgs_earthquakes"] != 1 {
			t.Errorf("usgs_earthquakes count: got %d, want 1", counts["usgs_earthquakes"])
		}
	})

	t.Run("layer_count_comes_from_the_durable_store", func(t *testing.T) {
		// Regression: a layer with persisted entities must report a non-zero
		// count. The bug sourced it from a hot cache that was empty on cold
		// start, so the layer reported 0 while rendering from SQLite.
		const layerType = "adsb_lol_flights"
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", layerType, "ext1"))
		entityRepo.add(makeEntity("e2", layerType, "ext2"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights"))
		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}
		if layers[0].Count != 2 {
			t.Errorf("Count: got %d, want 2 (from the durable store)", layers[0].Count)
		}
	})

	t.Run("count_error_propagates", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))
		entityRepo.countErr = errors.New("count query failed")

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights"))
		if _, err := svc.GetLayers(context.Background()); !errors.Is(err, entityRepo.countErr) {
			t.Errorf("expected count error to propagate, got %v", err)
		}
	})

	t.Run("nil_cache_does_not_panic", func(t *testing.T) {
		// Count is sourced from the repository, so a nil cache must neither panic
		// nor zero the count — it stays the durable DB total.
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "ext1"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights"))
		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}
		if layers[0].Count != 1 {
			t.Errorf("Count: got %d, want 1 (from DB; nil cache must not zero it)", layers[0].Count)
		}
	})

	t.Run("discovers_layers_from_db", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "adsb_lol_flights", "f1"))
		entityRepo.add(makeEntity("e2", "disaster_alerts", "da1"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("adsb_lol_flights", "disaster_alerts"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 2 {
			t.Fatalf("expected 2 layers, got %d", len(layers))
		}

		byID := make(map[string]*domain.Layer)
		for _, l := range layers {
			byID[l.ID] = l
		}

		dbLayer, ok := byID["disaster_alerts"]
		if !ok {
			t.Fatal("expected to find db-discovered layer 'disaster_alerts'")
		}
		if dbLayer.Type != "disaster_alerts" {
			t.Errorf("Type: got %q, want %q", dbLayer.Type, "disaster_alerts")
		}
		if !dbLayer.Enabled {
			t.Error("db-discovered layer should be enabled by default")
		}
		if dbLayer.Source != "feeder" {
			t.Errorf("Source: got %q, want %q", dbLayer.Source, "feeder")
		}
	})

	t.Run("declarative_layers_get_default_style", func(t *testing.T) {
		// Unknown layer types must get DefaultLayerStyle (non-zero PointSize).
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "weather_alerts", "wa1"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("weather_alerts"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(layers))
		}

		l := layers[0]
		wantStyle := domain.DefaultLayerStyle
		if l.Color != wantStyle.Color {
			t.Errorf("Color: got %q, want %q", l.Color, wantStyle.Color)
		}
		if l.PointSize != wantStyle.PointSize {
			t.Errorf("PointSize: got %d, want %d", l.PointSize, wantStyle.PointSize)
		}
	})

	t.Run("duplicate_entities_with_same_layer_type_produce_one_layer", func(t *testing.T) {
		// Multiple entities with the same layer type should yield exactly one layer.
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "usgs_earthquakes", "eq1"))
		entityRepo.add(makeEntity("e2", "usgs_earthquakes", "eq2"))
		entityRepo.add(makeEntity("e3", "usgs_earthquakes", "eq3"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("usgs_earthquakes"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 1 {
			t.Errorf("expected 1 layer (deduped), got %d", len(layers))
		}
	})

	t.Run("multiple_db_discovered_layers", func(t *testing.T) {
		entityRepo := newStubEntityRepo()
		entityRepo.add(makeEntity("e1", "disaster_alerts", "da1"))
		entityRepo.add(makeEntity("e2", "radiation", "rad1"))
		entityRepo.add(makeEntity("e3", "weather_alerts", "wa1"))

		svc := NewLayerService(entityRepo, newStubObsRepo(), declaring("disaster_alerts", "radiation", "weather_alerts"))

		layers, err := svc.GetLayers(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(layers) != 3 {
			t.Fatalf("expected 3 layers, got %d", len(layers))
		}
		// Verify sorted order
		for i := 1; i < len(layers); i++ {
			if layers[i-1].ID >= layers[i].ID {
				t.Errorf("layers not sorted: %q >= %q", layers[i-1].ID, layers[i].ID)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Tests: ToggleLayer
// ---------------------------------------------------------------------------

func TestToggleLayer(t *testing.T) {
	tests := []struct {
		name          string
		toggle        *domain.LayerToggle
		wantEnabled   bool
		wantMode      string
		wantDensity   int32
		wantLayerType string
	}{
		{
			name: "toggle_enabled_true_propagates",
			toggle: &domain.LayerToggle{
				LayerID: "adsb_lol_flights",
				Enabled: true,
				Mode:    "sparse",
				Density: 50,
			},
			wantEnabled:   true,
			wantMode:      "sparse",
			wantDensity:   50,
			wantLayerType: "adsb_lol_flights",
		},
		{
			name: "toggle_enabled_false_propagates",
			toggle: &domain.LayerToggle{
				LayerID: "usgs_earthquakes",
				Enabled: false,
				Mode:    "full",
				Density: 80,
			},
			wantEnabled:   false,
			wantMode:      "full",
			wantDensity:   80,
			wantLayerType: "usgs_earthquakes",
		},
		{
			name: "custom_source_uses_toggle_enabled",
			toggle: &domain.LayerToggle{
				LayerID: "custom_source",
				Enabled: true,
				Mode:    "full",
				Density: 100,
			},
			wantEnabled:   true,
			wantMode:      "full",
			wantDensity:   100,
			wantLayerType: "custom_source",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), domain.NewDynamicSourceRegistry())

			layer, err := svc.ToggleLayer(context.Background(), tc.toggle)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if layer == nil {
				t.Fatal("expected non-nil layer")
			}
			if layer.Enabled != tc.wantEnabled {
				t.Errorf("Enabled: got %v, want %v", layer.Enabled, tc.wantEnabled)
			}
			if layer.Mode != tc.wantMode {
				t.Errorf("Mode: got %q, want %q", layer.Mode, tc.wantMode)
			}
			if layer.Density != tc.wantDensity {
				t.Errorf("Density: got %d, want %d", layer.Density, tc.wantDensity)
			}
			if layer.Type != tc.wantLayerType {
				t.Errorf("Type: got %q, want %q", layer.Type, tc.wantLayerType)
			}
			if layer.ID != tc.toggle.LayerID {
				t.Errorf("ID: got %q, want %q", layer.ID, tc.toggle.LayerID)
			}
			if layer.Source != "feeder" {
				t.Errorf("Source: got %q, want %q", layer.Source, "feeder")
			}
		})
	}
}

func TestToggleLayer_StyleApplied(t *testing.T) {
	// Register a display config for "earthquakes" so GetLayerStyle returns known values.
	reg := domain.NewDynamicSourceRegistry()
	reg.RegisterWithDisplay("test_eq_src", "earthquakes", &domain.LayerDisplayConfig{
		Style: &domain.StyleConfig{Color: "#ff006e", PointSize: 10},
	})

	tests := []struct {
		name          string
		layerID       string
		wantColor     string
		wantPointSize int32
	}{
		{
			name:          "known layer type gets configured style",
			layerID:       "earthquakes",
			wantColor:     "#ff006e",
			wantPointSize: 10,
		},
		{
			name:          "unknown layer type gets default style",
			layerID:       "disaster_alerts",
			wantColor:     domain.DefaultLayerStyle.Color,
			wantPointSize: domain.DefaultLayerStyle.PointSize,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), reg)

			layer, err := svc.ToggleLayer(context.Background(), &domain.LayerToggle{
				LayerID: tc.layerID,
				Mode:    "full",
				Density: 100,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if layer.Color != tc.wantColor {
				t.Errorf("Color: got %q, want %q", layer.Color, tc.wantColor)
			}
			if layer.PointSize != tc.wantPointSize {
				t.Errorf("PointSize: got %d, want %d", layer.PointSize, tc.wantPointSize)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Tests: GetLayerSnapshot
// ---------------------------------------------------------------------------

func TestGetLayerSnapshot_UnknownLayerID(t *testing.T) {
	svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), "nonexistent_layer", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil SnapshotResult")
	}
	if len(result.Entities) != 0 {
		t.Errorf("Entities: got %d, want 0", len(result.Entities))
	}
	if len(result.Observations) != 0 {
		t.Errorf("Observations: got %d, want 0", len(result.Observations))
	}
	if result.TotalCount != 0 {
		t.Errorf("TotalCount: got %d, want 0", result.TotalCount)
	}
	if result.HasMore {
		t.Error("HasMore: got true, want false")
	}
}

func TestGetLayerSnapshot_DeclarativeLayerID(t *testing.T) {
	// Declarative layer IDs (e.g. "disaster_alerts") are used directly as the
	// layer type when querying the DB/cache.
	const layerID = "disaster_alerts"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	entity := makeEntity("e1", layerID, "ext1")
	obs := makeObservation("o1", entity.ID, layerID)
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1", len(result.Entities))
	}
	if result.TotalCount != 1 {
		t.Errorf("TotalCount: got %d, want 1", result.TotalCount)
	}
}

func TestGetLayerSnapshot_DeclarativeLayerIDFromStore(t *testing.T) {
	const layerID = "disaster_alerts"

	ctx := context.Background()

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo
	entity := makeEntity("e1", layerID, "ext1")
	obs := makeObservation("o1", entity.ID, layerID)
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(ctx, layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1", len(result.Entities))
	}
}

func TestGetLayerSnapshot_MultipleEntities(t *testing.T) {
	const layerID = "adsb_lol_flights"

	ctx := context.Background()

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo
	for i := range 3 {
		ext := fmt.Sprintf("ext%d", i)
		entity := makeEntity(fmt.Sprintf("e%d", i), layerID, ext)
		obs := makeObservation(fmt.Sprintf("o%d", i), entity.ID, layerID)
		entityRepo.add(entity)
		obsRepo.add(obs)
	}

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(ctx, layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 3 {
		t.Errorf("Entities: got %d, want 3", len(result.Entities))
	}
	if result.TotalCount != 3 {
		t.Errorf("TotalCount: got %d, want 3", result.TotalCount)
	}
	if result.HasMore {
		t.Error("HasMore: got true, want false (all 3 fit in limit=10)")
	}
}

func TestGetLayerSnapshot_Pagination(t *testing.T) {
	const layerID = "adsb_lol_flights"

	ctx := context.Background()

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo
	total := 5
	for i := range total {
		ext := fmt.Sprintf("ext%d", i)
		entity := makeEntity(fmt.Sprintf("e%d", i), layerID, ext)
		obs := makeObservation(fmt.Sprintf("o%d", i), entity.ID, layerID)
		entityRepo.add(entity)
		obsRepo.add(obs)
	}

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	// First page: limit=2, offset=0 → 2 results, hasMore=true
	result, err := svc.GetLayerSnapshot(ctx, layerID, 2, 0)
	if err != nil {
		t.Fatalf("page 1 unexpected error: %v", err)
	}
	if len(result.Entities) != 2 {
		t.Errorf("page 1 Entities: got %d, want 2", len(result.Entities))
	}
	if !result.HasMore {
		t.Error("page 1 HasMore: got false, want true")
	}

	// Last page: limit=2, offset=4 → 1 result, hasMore=false
	result, err = svc.GetLayerSnapshot(ctx, layerID, 2, 4)
	if err != nil {
		t.Fatalf("last page unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("last page Entities: got %d, want 1", len(result.Entities))
	}
	if result.HasMore {
		t.Error("last page HasMore: got true, want false")
	}
}

func TestGetLayerSnapshot_ReturnsStoredEntities(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	// Add entities and observations directly into the DB stubs.
	for i := range 3 {
		entity := makeEntity(fmt.Sprintf("e%d", i), layerID, fmt.Sprintf("ext%d", i))
		obs := makeObservation(fmt.Sprintf("o%d", i), entity.ID, layerID)
		entityRepo.add(entity)
		obsRepo.add(obs)
	}

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The page carries each entity with its latest observation, so all 3 load.
	if len(result.Entities) != 3 {
		t.Errorf("Entities: got %d, want 3", len(result.Entities))
	}
	if result.TotalCount != 3 {
		t.Errorf("TotalCount: got %d, want 3", result.TotalCount)
	}
}

func TestGetLayerSnapshot_SingleEntity(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	entity := makeEntity("e1", layerID, "ext1")
	obs := makeObservation("o1", entity.ID, layerID)
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The store returns the single entity/observation.
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1", len(result.Entities))
	}
}

func TestGetLayerSnapshot_NoRepositoriesConfigured(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	entity := makeEntity("e1", layerID, "ext1")
	obs := makeObservation("o1", entity.ID, layerID)
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1", len(result.Entities))
	}
}

func TestGetLayerSnapshot_DBFallback_Pagination(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	total := 5
	for i := range total {
		entity := makeEntity(fmt.Sprintf("e%d", i), layerID, fmt.Sprintf("ext%d", i))
		obs := makeObservation(fmt.Sprintf("o%d", i), entity.ID, layerID)
		entityRepo.add(entity)
		obsRepo.add(obs)
	}

	// Nil cache forces DB path.
	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	ctx := context.Background()

	tests := []struct {
		limit       int
		offset      int
		wantLen     int
		wantHasMore bool
	}{
		{limit: 2, offset: 0, wantLen: 2, wantHasMore: true},
		{limit: 2, offset: 2, wantLen: 2, wantHasMore: true},
		{limit: 2, offset: 4, wantLen: 1, wantHasMore: false},
		{limit: 10, offset: 0, wantLen: 5, wantHasMore: false},
		// offset beyond total: returns empty slice, no more
		{limit: 2, offset: 10, wantLen: 0, wantHasMore: false},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("limit=%d_offset=%d", tc.limit, tc.offset), func(t *testing.T) {
			result, err := svc.GetLayerSnapshot(ctx, layerID, tc.limit, tc.offset)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Entities) != tc.wantLen {
				t.Errorf("Entities: got %d, want %d", len(result.Entities), tc.wantLen)
			}
			if result.HasMore != tc.wantHasMore {
				t.Errorf("HasMore: got %v, want %v", result.HasMore, tc.wantHasMore)
			}
			if result.TotalCount != int64(total) {
				t.Errorf("TotalCount: got %d, want %d", result.TotalCount, total)
			}
		})
	}
}

func TestGetLayerSnapshot_DBFallback_SkipsEntitiesNotInRepo(t *testing.T) {
	// Observations referencing unknown entity IDs are silently skipped.
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	// Only entity "e1" exists; observation "o2" references a missing "e2".
	entity := makeEntity("e1", layerID, "ext1")
	entityRepo.add(entity)

	obsRepo.add(makeObservation("o1", "e1", layerID))
	obsRepo.add(makeObservation("o2", "e2_missing", layerID)) // entity not in repo

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1 (missing entity silently skipped)", len(result.Entities))
	}
}

func TestGetLayerSnapshot_DBFallback_ObsRepoError(t *testing.T) {
	const layerID = "adsb_lol_flights"

	obsRepo := newStubObsRepo()
	obsRepo.getErr = errors.New("db connection lost")

	svc := NewLayerService(newStubEntityRepo(), obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != nil {
		t.Errorf("expected nil result on DB error, got %+v", result)
	}
	if !errors.Is(err, obsRepo.getErr) {
		t.Errorf("error: got %v, want %v", err, obsRepo.getErr)
	}
}

func TestGetLayerSnapshot_EmptyDB(t *testing.T) {
	const layerID = "adsb_lol_flights"

	svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Entities) != 0 {
		t.Errorf("Entities: got %d, want 0", len(result.Entities))
	}
	if result.TotalCount != 0 {
		t.Errorf("TotalCount: got %d, want 0", result.TotalCount)
	}
	if result.HasMore {
		t.Error("HasMore: got true, want false")
	}
}

func TestGetLayerSnapshot_LayerIDUsedDirectlyAsLayerType(t *testing.T) {
	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo
	ctx := context.Background()

	entity := makeEntity("e1", "adsb_lol_flights", "ext1")
	obs := makeObservation("o1", entity.ID, "adsb_lol_flights")
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())
	result, err := svc.GetLayerSnapshot(ctx, "adsb_lol_flights", 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("Entities: got %d, want 1", len(result.Entities))
	}
}

func TestGetLayers_WithDisplayConfig(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("test_display_src")
	lt := domain.LayerType("test_display_layer")
	dynReg.Register(st, lt)
	dynReg.RegisterWithDisplay("test_display_src", "test_display_layer", &domain.LayerDisplayConfig{
		Style: &domain.StyleConfig{Color: "#customcolor", PointSize: 15},
	})
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "test_display_layer", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	if layers[0].Color != "#customcolor" {
		t.Errorf("expected custom color, got %s", layers[0].Color)
	}
	if layers[0].PointSize != 15 {
		t.Errorf("expected PointSize 15, got %d", layers[0].PointSize)
	}
}

func TestGetLayers_LayerDisplayNameOverride(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("sict_traffic_stations")
	lt := domain.LayerType("traffic_stations")
	dynReg.Register(st, lt)
	dynReg.SetLayerDisplayName(lt, "Traffic Stations (Mexico)")
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "traffic_stations", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	// The override replaces the title-cased layer_type ("Traffic Stations").
	if layers[0].Name != "Traffic Stations (Mexico)" {
		t.Errorf("layer name: got %q, want %q", layers[0].Name, "Traffic Stations (Mexico)")
	}
}

func TestGetLayers_NoOverride_UsesFormattedLayerType(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("toll_booths_src")
	lt := domain.LayerType("toll_booths")
	dynReg.Register(st, lt)
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "toll_booths", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	// No override registered → fall back to title-cased layer_type.
	if layers[0].Name != "Toll Booths" {
		t.Errorf("layer name: got %q, want %q (FormatLayerName fallback)", layers[0].Name, "Toll Booths")
	}
}

func TestGetLayers_WithHistoryConfig(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("test_history_src")
	lt := domain.LayerType("test_history_layer")
	dynReg.Register(st, lt)
	dynReg.SetHistoryConfig(lt, &domain.HistoryConfig{
		MaxLookbackHours:  48,
		MaxRangeSpanHours: 24,
	})
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "test_history_layer", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	if layers[0].HistoryConfig == nil {
		t.Error("expected HistoryConfig to be set")
	}
}

func TestGetLayers_RepositoryError(t *testing.T) {
	entityRepo := newStubEntityRepo()
	entityRepo.getErr = errors.New("db connection failed")

	svc := NewLayerService(entityRepo, newStubObsRepo(), domain.NewDynamicSourceRegistry())
	_, err := svc.GetLayers(context.Background())
	if err == nil {
		t.Fatal("expected error from repository")
	}
}

func TestToggleLayer_WithRegistryConfigs(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("test_toggle_src")
	lt := domain.LayerType("test_toggle_layer")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "custom")
	dynReg.SetFilteringMode(lt, "viewport")
	defer dynReg.Unregister(st)

	svc := NewLayerService(newStubEntityRepo(), newStubObsRepo(), dynReg)
	layer, err := svc.ToggleLayer(context.Background(), &domain.LayerToggle{
		LayerID: "test_toggle_layer",
		Enabled: true,
		Mode:    "full",
		Density: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if layer.RenderingMode != "custom" {
		t.Errorf("expected RenderingMode 'custom', got %s", layer.RenderingMode)
	}
	if layer.FilteringMode != "viewport" {
		t.Errorf("expected FilteringMode 'viewport', got %s", layer.FilteringMode)
	}
}

func TestGetLayerSnapshot_ObservationMismatch(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	entityRepo.add(makeEntity("e1", layerID, "ext1"))
	obsRepo.add(makeObservation("o1", "e1", layerID))
	obsRepo.add(makeObservation("o2", "e2", layerID))

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 1 {
		t.Errorf("expected 1 entity, got %d", len(result.Entities))
	}
}

func TestGetLayerSnapshot_OffsetBeyondEntities(t *testing.T) {
	const layerID = "adsb_lol_flights"

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()
	obsRepo.entities = entityRepo

	for i := range 3 {
		entity := makeEntity(fmt.Sprintf("e%d", i), layerID, fmt.Sprintf("ext%d", i))
		obs := makeObservation(fmt.Sprintf("o%d", i), entity.ID, layerID)
		entityRepo.add(entity)
		obsRepo.add(obs)
	}

	svc := NewLayerService(entityRepo, obsRepo, domain.NewDynamicSourceRegistry())

	result, err := svc.GetLayerSnapshot(context.Background(), layerID, 2, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(result.Entities))
	}
	if result.TotalCount != 3 {
		t.Errorf("expected TotalCount 3, got %d", result.TotalCount)
	}
}

func TestGetLayers_DisplayConfigWithNilStyle(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("test_nil_style_src")
	lt := domain.LayerType("test_nil_style_layer")
	dynReg.Register(st, lt)
	dynReg.RegisterWithDisplay("test_nil_style_src", "test_nil_style_layer", &domain.LayerDisplayConfig{
		Style: nil,
	})
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "test_nil_style_layer", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	wantStyle := domain.DefaultLayerStyle
	if layers[0].Color != wantStyle.Color {
		t.Errorf("expected default color, got %s", layers[0].Color)
	}
}

func TestGetLayers_DisplayConfigWithEmptyColor(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	st := domain.SourceType("test_empty_color_src")
	lt := domain.LayerType("test_empty_color_layer")
	dynReg.Register(st, lt)
	dynReg.RegisterWithDisplay("test_empty_color_src", "test_empty_color_layer", &domain.LayerDisplayConfig{
		Style: &domain.StyleConfig{Color: "", PointSize: 20},
	})
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "test_empty_color_layer", "ext1"))

	svc := NewLayerService(entityRepo, newStubObsRepo(), dynReg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(layers) != 1 {
		t.Fatalf("expected 1 layer, got %d", len(layers))
	}
	wantStyle := domain.DefaultLayerStyle
	if layers[0].Color != wantStyle.Color {
		t.Errorf("expected default color when empty, got %s", layers[0].Color)
	}
	if layers[0].PointSize != 20 {
		t.Errorf("expected PointSize 20, got %d", layers[0].PointSize)
	}
}

// A source that changes its layer_type in YAML (cctv_austin -> cctv) leaves its
// old rows behind in SQLite forever. Layer existence follows the declarations,
// so the renamed-away layer must not surface in the panel.
func TestGetLayersOmitsLayersNoSourceDeclares(t *testing.T) {
	entityRepo := newStubEntityRepo()
	entityRepo.add(makeEntity("e1", "cctv", "ext1"))
	entityRepo.add(makeEntity("e2", "cctv_austin", "ext2"))
	entityRepo.add(makeEntity("e3", "cctv_calgary", "ext3"))

	reg := domain.NewDynamicSourceRegistry()
	reg.Register("cctv_austin", "cctv")
	reg.Register("cctv_calgary", "cctv")

	svc := NewLayerService(entityRepo, newStubObsRepo(), reg)
	layers, err := svc.GetLayers(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ids []string
	for _, l := range layers {
		ids = append(ids, l.ID)
	}
	if len(ids) != 1 || ids[0] != "cctv" {
		t.Errorf("expected only the declared layer [cctv], got %v", ids)
	}
}

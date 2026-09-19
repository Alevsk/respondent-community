package declarative

import (
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/ingest/grid"
	"github.com/Alevsk/respondent/internal/logging"
)

// TestInitSpatialState_HexGrid verifies hex_grid initialization populates regions.
func TestInitSpatialState_HexGrid(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	spatial := &SpatialSpec{
		Type:     "hex_grid",
		RadiusNM: 250,
	}
	adapter.initSpatialState(spatial, 30*time.Second)

	if adapter.spatial == nil {
		t.Fatal("expected spatial state to be initialized")
	}
	if len(adapter.spatial.regions) == 0 {
		t.Error("expected non-empty regions for hex_grid")
	}
	if adapter.spatialBatchSize <= 0 {
		t.Error("expected positive batch size")
	}
}

// TestInitSpatialState_HexGrid_DefaultRadius verifies zero radius uses 250 NM default.
func TestInitSpatialState_HexGrid_DefaultRadius(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	spatial := &SpatialSpec{
		Type:     "hex_grid",
		RadiusNM: 0, // triggers default
	}
	adapter.initSpatialState(spatial, 30*time.Second)
	if adapter.spatial == nil || len(adapter.spatial.regions) == 0 {
		t.Error("expected regions with default 250 NM radius")
	}
}

// TestInitSpatialState_HexGrid_LatFilter verifies lat_min/lat_max filtering.
func TestInitSpatialState_HexGrid_LatFilter(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	latMin := 0.0
	latMax := 60.0
	spatial := &SpatialSpec{
		Type:     "hex_grid",
		RadiusNM: 250,
		LatMin:   &latMin,
		LatMax:   &latMax,
	}
	adapter.initSpatialState(spatial, 30*time.Second)

	if adapter.spatial == nil {
		t.Fatal("expected spatial state to be initialized")
	}
	// All regions should be within the lat filter.
	for _, r := range adapter.spatial.regions {
		if r.Lat < latMin || r.Lat > latMax {
			t.Errorf("region at lat=%.2f is outside filter [%.2f, %.2f]", r.Lat, latMin, latMax)
		}
	}
}

// TestInitSpatialState_StaticRegions verifies static_regions type populates regions.
func TestInitSpatialState_StaticRegions(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	spatial := &SpatialSpec{
		Type: "static_regions",
		Regions: []StaticRegion{
			{Lat: 40.0, Lon: -74.0, Label: "NYC"},
			{Lat: 51.5, Lon: -0.1, Label: "London"},
		},
		BatchSize: 2,
	}
	adapter.initSpatialState(spatial, 30*time.Second)

	if adapter.spatial == nil {
		t.Fatal("expected spatial state to be initialized")
	}
	if len(adapter.spatial.regions) != 2 {
		t.Errorf("expected 2 static regions, got %d", len(adapter.spatial.regions))
	}
	if adapter.spatial.regions[0].Label != "NYC" {
		t.Errorf("expected first region NYC, got %q", adapter.spatial.regions[0].Label)
	}
}

// TestInitSpatialState_AlreadyInitialized verifies idempotency (no re-init).
func TestInitSpatialState_AlreadyInitialized(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	// Pre-initialize.
	adapter.spatial = &spatialState{
		regions: []grid.Region{{Lat: 10.0, Lon: 20.0, Label: "existing"}},
		cursor:  5,
	}
	spatial := &SpatialSpec{
		Type: "hex_grid",
	}
	// Should be a no-op.
	adapter.initSpatialState(spatial, 30*time.Second)

	if len(adapter.spatial.regions) != 1 || adapter.spatial.regions[0].Label != "existing" {
		t.Error("expected initSpatialState to be idempotent when already initialized")
	}
	if adapter.spatial.cursor != 5 {
		t.Errorf("expected cursor=5 to be preserved, got %d", adapter.spatial.cursor)
	}
}

// TestInitSpatialState_AutoBatchSize verifies auto batch size calculation.
func TestInitSpatialState_AutoBatchSize(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	spatial := &SpatialSpec{
		Type:          "static_regions",
		Regions:       make([]StaticRegion, 100),
		BatchSize:     0, // triggers auto calculation
		TargetRefresh: Duration{Duration: 30 * time.Minute},
	}
	// Fill in regions.
	for i := range spatial.Regions {
		spatial.Regions[i] = StaticRegion{Lat: float64(i), Lon: float64(i), Label: "r"}
	}
	adapter.initSpatialState(spatial, 30*time.Second)

	if adapter.spatialBatchSize <= 0 {
		t.Error("expected positive auto-computed batch size")
	}
}

// TestInitSpatialState_ZeroInterval verifies zero interval uses default for batch calculation.
func TestInitSpatialState_ZeroInterval(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	spatial := &SpatialSpec{
		Type:      "static_regions",
		BatchSize: 0, // auto
		Regions:   []StaticRegion{{Lat: 1.0, Lon: 1.0, Label: "r1"}},
	}
	// Zero interval should trigger the default 30s fallback.
	adapter.initSpatialState(spatial, 0)
	if adapter.spatialBatchSize <= 0 {
		t.Error("expected positive batch size with zero interval")
	}
}

// TestInitEntityCache_NilSpec verifies nil spec creates a default 5-minute TTL cache.
func TestInitEntityCache_NilSpec(t *testing.T) {
	adapter := &DeclarativeAdapter{
		clock: realClock{},
	}
	adapter.initEntityCache(nil)
	if adapter.ecache == nil {
		t.Fatal("expected non-nil entity cache")
	}
	if adapter.ecache.ttl != 5*time.Minute {
		t.Errorf("expected 5m TTL for nil spec, got %v", adapter.ecache.ttl)
	}
}

// TestInitEntityCache_DisabledSpec verifies disabled spec creates a default 5-minute TTL cache.
func TestInitEntityCache_DisabledSpec(t *testing.T) {
	adapter := &DeclarativeAdapter{
		clock: realClock{},
	}
	spec := &EntityCacheSpec{Enabled: false}
	adapter.initEntityCache(spec)
	if adapter.ecache == nil {
		t.Fatal("expected non-nil entity cache for disabled spec")
	}
	if adapter.ecache.ttl != 5*time.Minute {
		t.Errorf("expected 5m TTL for disabled spec, got %v", adapter.ecache.ttl)
	}
}

// TestInitEntityCache_EnabledSpec verifies enabled spec uses configured TTL.
func TestInitEntityCache_EnabledSpec(t *testing.T) {
	adapter := &DeclarativeAdapter{
		clock: realClock{},
	}
	spec := &EntityCacheSpec{
		Enabled: true,
		TTL:     Duration{Duration: 10 * time.Minute},
	}
	adapter.initEntityCache(spec)
	if adapter.ecache == nil {
		t.Fatal("expected non-nil entity cache")
	}
	if adapter.ecache.ttl != 10*time.Minute {
		t.Errorf("expected 10m TTL, got %v", adapter.ecache.ttl)
	}
}

// TestInitEntityCache_AlreadyInitialized verifies idempotency.
func TestInitEntityCache_AlreadyInitialized(t *testing.T) {
	adapter := &DeclarativeAdapter{
		clock: realClock{},
	}
	existing := newEntityCache(99*time.Second, nil)
	adapter.ecache = existing

	// Should be a no-op.
	adapter.initEntityCache(&EntityCacheSpec{Enabled: true, TTL: Duration{Duration: time.Hour}})
	if adapter.ecache != existing {
		t.Error("expected initEntityCache to be idempotent when already initialized")
	}
}

// TestSelectSpatialBatch_NilSpatialState verifies nil spatial state returns nil.
func TestSelectSpatialBatch_NilSpatialState(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	// spatial is nil.
	adapter.spatialMu.Lock()
	batch := adapter.selectSpatialBatch()
	adapter.spatialMu.Unlock()
	if batch != nil {
		t.Errorf("expected nil batch for nil spatial state, got %v", batch)
	}
}

// TestSelectSpatialBatch_EmptyRegions verifies empty regions returns nil.
func TestSelectSpatialBatch_EmptyRegions(t *testing.T) {
	adapter := &DeclarativeAdapter{
		logger: logging.NewNopLogger(),
	}
	adapter.spatial = &spatialState{regions: []grid.Region{}, cursor: 0}
	adapter.spatialBatchSize = 5

	adapter.spatialMu.Lock()
	batch := adapter.selectSpatialBatch()
	adapter.spatialMu.Unlock()
	if batch != nil {
		t.Errorf("expected nil batch for empty regions, got %v", batch)
	}
}

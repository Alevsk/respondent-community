package declarative

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestRegisterDisplayConfigs_V2Fields(t *testing.T) {
	// Create a temp directory with a test YAML that includes v2 fields.
	dir := t.TempDir()

	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test_layer
display_name: "Test Source"
filtering: viewport
backfill:
  threshold: 5
transport:
  type: http_poll
  url: "https://example.com/api"
  on_demand_url: "https://example.com/api/point/{lat}/{lon}"
  timeout: "10s"
  interval: "60s"
display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 6
`

	err := os.WriteFile(filepath.Join(dir, "test_source.yaml"), []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("failed to write test YAML: %v", err)
	}

	// Use a fresh registry for test isolation.
	// Since RegisterDisplayConfigs uses the global singleton, we need to
	// register and then verify + clean up.
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection

	lt := domain.LayerType("test_layer")

	// Clean up after the test.
	defer func() {
		dynReg.Unregister(domain.SourceType("test_source"))
	}()

	err = RegisterDisplayConfigs(dir, dynReg, nil)
	if err != nil {
		t.Fatalf("RegisterDisplayConfigs error: %v", err)
	}

	// Verify display config was registered.
	dc, ok := dynReg.LookupDisplayConfig(lt)
	if !ok {
		t.Fatal("expected display config to be registered")
	}
	if dc.Icon.Shape != "diamond" {
		t.Errorf("icon shape = %q, want %q", dc.Icon.Shape, "diamond")
	}

	// Verify v2 filtering mode was registered.
	mode, ok := dynReg.LookupFilteringMode(lt)
	if !ok {
		t.Fatal("expected filtering mode to be registered")
	}
	if mode != "viewport" {
		t.Errorf("filtering mode = %q, want %q", mode, "viewport")
	}

	// Verify v2 backfill threshold was registered.
	threshold, ok := dynReg.LookupBackfillThreshold(lt)
	if !ok {
		t.Fatal("expected backfill threshold to be registered")
	}
	if threshold != 5 {
		t.Errorf("backfill threshold = %d, want %d", threshold, 5)
	}

	// Verify v2 on-demand URL was registered.
	url, ok := dynReg.LookupOnDemandURL(lt)
	if !ok {
		t.Fatal("expected on-demand URL to be registered")
	}
	if url != "https://example.com/api/point/{lat}/{lon}" {
		t.Errorf("on-demand URL = %q, want %q", url, "https://example.com/api/point/{lat}/{lon}")
	}
}

func TestRegisterDisplayConfigs_HistoryConfig(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
schema_version: 2
name: marine_ais_source
source_type: marine_ais_history_test
layer_type: marine_ais_history_layer
display_name: "Marine AIS (History Test)"
history:
  max_lookback: "8760h"
  max_range_span: "168h"
transport:
  type: http_poll
  url: "https://example.com/marine"
  timeout: "10s"
  interval: "60s"
display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 6
`

	if err := os.WriteFile(filepath.Join(dir, "marine_ais_history_test.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test YAML: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("marine_ais_history_layer")

	defer func() {
		dynReg.Unregister(domain.SourceType("marine_ais_history_test"))
	}()

	if err := RegisterDisplayConfigs(dir, dynReg, nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs error: %v", err)
	}

	// History config must be registered in the dynamic registry.
	hc, ok := dynReg.LookupHistoryConfig(lt)
	if !ok {
		t.Fatal("expected history config to be registered in dynamic registry")
	}
	if hc == nil {
		t.Fatal("LookupHistoryConfig returned nil config, want non-nil")
	}

	const wantLookback int32 = 8760
	if hc.MaxLookbackHours != wantLookback {
		t.Errorf("MaxLookbackHours = %d, want %d", hc.MaxLookbackHours, wantLookback)
	}

	const wantSpan int32 = 168
	if hc.MaxRangeSpanHours != wantSpan {
		t.Errorf("MaxRangeSpanHours = %d, want %d", hc.MaxRangeSpanHours, wantSpan)
	}
}

func TestRegisterSourceMetadata_HistoryConfig_Nil(t *testing.T) {
	// When History is nil, no history config should be registered in the dynamic registry.
	const sourceType = "no_hist_src_nil_test"
	const layerType = "no_hist_lyr_nil_test"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType(layerType)
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		// History intentionally omitted (nil).
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata error: %v", err)
	}

	_, ok := dynReg.LookupHistoryConfig(lt)
	if ok {
		t.Error("LookupHistoryConfig returned true when no History was registered, want false")
	}
}

func TestRegisterDisplayConfigs_EmptyDir(t *testing.T) {
	err := RegisterDisplayConfigs("", domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("expected nil error for empty dir, got: %v", err)
	}
}

func TestRegisterDisplayConfigs_NonExistentDir(t *testing.T) {
	err := RegisterDisplayConfigs("/nonexistent/path/that/does/not/exist", domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("expected nil error for nonexistent dir, got: %v", err)
	}
}

func TestRegisterDisplayConfigs_SkipsNonYAML(t *testing.T) {
	dir := t.TempDir()

	// Create a non-YAML file
	err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	if err != nil {
		t.Fatalf("failed to write: %v", err)
	}

	err = RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// SourceMetadataFromDefinition — geo_cache field handling
// ---------------------------------------------------------------------------

func TestSourceMetadataFromDefinition_GeoCachePresent(t *testing.T) {
	// When geo_cache is fully specified, all three fields must override the defaults.
	defaults := domain.DefaultGeoCacheConfig()

	overfetch := 5.0
	aliveRatio := 0.8
	gcBatch := 250

	// Sanity: the explicit values differ from defaults so the test is meaningful.
	if overfetch == defaults.OverfetchRatio {
		t.Fatal("test setup error: overfetch_ratio must differ from the default")
	}
	if aliveRatio == defaults.AliveRatioThreshold {
		t.Fatal("test setup error: alive_ratio_threshold must differ from the default")
	}
	if gcBatch == defaults.GCBatchSize {
		t.Fatal("test setup error: gc_batch_size must differ from the default")
	}

	def := minimalSourceDefinition("geo_cache_full_src", "geo_cache_full_lyr")
	def.GeoCache = &GeoCacheSpec{
		OverfetchRatio:      overfetch,
		AliveRatioThreshold: aliveRatio,
		GCBatchSize:         gcBatch,
	}

	sm := SourceMetadataFromDefinition(def)

	if sm.GeoCacheConfig == nil {
		t.Fatal("GeoCacheConfig is nil, want non-nil when geo_cache is present")
	}
	if sm.GeoCacheConfig.OverfetchRatio != overfetch {
		t.Errorf("OverfetchRatio = %v, want %v", sm.GeoCacheConfig.OverfetchRatio, overfetch)
	}
	if sm.GeoCacheConfig.AliveRatioThreshold != aliveRatio {
		t.Errorf("AliveRatioThreshold = %v, want %v", sm.GeoCacheConfig.AliveRatioThreshold, aliveRatio)
	}
	if sm.GeoCacheConfig.GCBatchSize != gcBatch {
		t.Errorf("GCBatchSize = %v, want %v", sm.GeoCacheConfig.GCBatchSize, gcBatch)
	}
}

func TestSourceMetadataFromDefinition_GeoCacheAbsent(t *testing.T) {
	// When geo_cache is not set in the definition, GeoCacheConfig must be nil.
	def := minimalSourceDefinition("geo_cache_absent_src", "geo_cache_absent_lyr")
	// def.GeoCache is nil by default.

	sm := SourceMetadataFromDefinition(def)

	if sm.GeoCacheConfig != nil {
		t.Errorf("GeoCacheConfig = %+v, want nil when geo_cache is absent", sm.GeoCacheConfig)
	}
}

func TestSourceMetadataFromDefinition_GeoCachePartial(t *testing.T) {
	// When only overfetch_ratio is set (non-zero), the other two fields must fall
	// back to their defaults.
	defaults := domain.DefaultGeoCacheConfig()

	def := minimalSourceDefinition("geo_cache_partial_src", "geo_cache_partial_lyr")
	def.GeoCache = &GeoCacheSpec{
		OverfetchRatio: 4.5,
		// AliveRatioThreshold and GCBatchSize left as zero-values → must use defaults.
	}

	sm := SourceMetadataFromDefinition(def)

	if sm.GeoCacheConfig == nil {
		t.Fatal("GeoCacheConfig is nil, want non-nil when geo_cache is present (even partially)")
	}
	if sm.GeoCacheConfig.OverfetchRatio != 4.5 {
		t.Errorf("OverfetchRatio = %v, want 4.5", sm.GeoCacheConfig.OverfetchRatio)
	}
	if sm.GeoCacheConfig.AliveRatioThreshold != defaults.AliveRatioThreshold {
		t.Errorf("AliveRatioThreshold = %v, want default %v",
			sm.GeoCacheConfig.AliveRatioThreshold, defaults.AliveRatioThreshold)
	}
	if sm.GeoCacheConfig.GCBatchSize != defaults.GCBatchSize {
		t.Errorf("GCBatchSize = %v, want default %v",
			sm.GeoCacheConfig.GCBatchSize, defaults.GCBatchSize)
	}
}

func TestSourceMetadataFromDefinition_GeoCacheTableDriven(t *testing.T) {
	defaults := domain.DefaultGeoCacheConfig()

	tests := []struct {
		name            string
		spec            *GeoCacheSpec
		wantNil         bool
		wantOverfetch   float64
		wantAliveRatio  float64
		wantGCBatchSize int
	}{
		{
			name:    "nil spec yields nil config",
			spec:    nil,
			wantNil: true,
		},
		{
			name: "all fields set to non-zero override defaults",
			spec: &GeoCacheSpec{
				OverfetchRatio:      2.5,
				AliveRatioThreshold: 0.6,
				GCBatchSize:         100,
			},
			wantOverfetch:   2.5,
			wantAliveRatio:  0.6,
			wantGCBatchSize: 100,
		},
		{
			name: "only gc_batch_size set — other fields default",
			spec: &GeoCacheSpec{
				GCBatchSize: 1000,
			},
			wantOverfetch:   defaults.OverfetchRatio,
			wantAliveRatio:  defaults.AliveRatioThreshold,
			wantGCBatchSize: 1000,
		},
		{
			name: "only alive_ratio_threshold set — other fields default",
			spec: &GeoCacheSpec{
				AliveRatioThreshold: 0.9,
			},
			wantOverfetch:   defaults.OverfetchRatio,
			wantAliveRatio:  0.9,
			wantGCBatchSize: defaults.GCBatchSize,
		},
		{
			name: "all zero-value fields — all defaults applied",
			spec: &GeoCacheSpec{},
			// OverfetchRatio=0, AliveRatioThreshold=0, GCBatchSize=0 → all use defaults.
			wantOverfetch:   defaults.OverfetchRatio,
			wantAliveRatio:  defaults.AliveRatioThreshold,
			wantGCBatchSize: defaults.GCBatchSize,
		},
	}

	for i, tc := range tests {
		// Unique layer/source names per sub-test to avoid global registry collisions.
		srcType := fmt.Sprintf("gc_table_src_%d", i)
		lyrType := fmt.Sprintf("gc_table_lyr_%d", i)

		def := minimalSourceDefinition(srcType, lyrType)
		def.GeoCache = tc.spec

		sm := SourceMetadataFromDefinition(def)

		if tc.wantNil {
			if sm.GeoCacheConfig != nil {
				t.Errorf("[%s] GeoCacheConfig = %+v, want nil", tc.name, sm.GeoCacheConfig)
			}
			continue
		}

		if sm.GeoCacheConfig == nil {
			t.Errorf("[%s] GeoCacheConfig is nil, want non-nil", tc.name)
			continue
		}
		if sm.GeoCacheConfig.OverfetchRatio != tc.wantOverfetch {
			t.Errorf("[%s] OverfetchRatio = %v, want %v",
				tc.name, sm.GeoCacheConfig.OverfetchRatio, tc.wantOverfetch)
		}
		if sm.GeoCacheConfig.AliveRatioThreshold != tc.wantAliveRatio {
			t.Errorf("[%s] AliveRatioThreshold = %v, want %v",
				tc.name, sm.GeoCacheConfig.AliveRatioThreshold, tc.wantAliveRatio)
		}
		if sm.GeoCacheConfig.GCBatchSize != tc.wantGCBatchSize {
			t.Errorf("[%s] GCBatchSize = %v, want %v",
				tc.name, sm.GeoCacheConfig.GCBatchSize, tc.wantGCBatchSize)
		}
	}
}

// ---------------------------------------------------------------------------
// RegisterSourceMetadata — geo_cache registry wiring
// ---------------------------------------------------------------------------

func TestRegisterSourceMetadata_GeoCacheConfig_Stored(t *testing.T) {
	// When GeoCacheConfig is non-nil it must be persisted in the dynamic registry.
	const sourceType = "gc_reg_src_set"
	const layerType = "gc_reg_lyr_set"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType(layerType)
	defer dynReg.Unregister(domain.SourceType(sourceType))

	cfg := domain.GeoCacheConfig{
		OverfetchRatio:      4.0,
		AliveRatioThreshold: 0.7,
		GCBatchSize:         200,
	}

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		GeoCacheConfig: &cfg,
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata error: %v", err)
	}

	got, ok := dynReg.LookupGeoCacheConfig(lt)
	if !ok {
		t.Fatal("LookupGeoCacheConfig returned false, want registered config")
	}
	if got.OverfetchRatio != cfg.OverfetchRatio {
		t.Errorf("OverfetchRatio = %v, want %v", got.OverfetchRatio, cfg.OverfetchRatio)
	}
	if got.AliveRatioThreshold != cfg.AliveRatioThreshold {
		t.Errorf("AliveRatioThreshold = %v, want %v", got.AliveRatioThreshold, cfg.AliveRatioThreshold)
	}
	if got.GCBatchSize != cfg.GCBatchSize {
		t.Errorf("GCBatchSize = %v, want %v", got.GCBatchSize, cfg.GCBatchSize)
	}
}

func TestRegisterSourceMetadata_GeoCacheConfig_Nil(t *testing.T) {
	// When GeoCacheConfig is nil, no entry must be written to the registry
	// for the layer type under test.
	const sourceType = "gc_reg_src_nil"
	const layerType = "gc_reg_lyr_nil"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType(layerType)
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		// GeoCacheConfig intentionally omitted (nil).
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata error: %v", err)
	}

	_, ok := dynReg.LookupGeoCacheConfig(lt)
	if ok {
		t.Error("LookupGeoCacheConfig returned true, want false when GeoCacheConfig is nil")
	}
}

func TestRegisterSourceMetadata_GeoCacheConfig_DefaultsMerged(t *testing.T) {
	// Verify that when a partial GeoCacheSpec (only overfetch_ratio set) is built
	// via SourceMetadataFromDefinition, the registered config has defaults for the
	// unset fields — exercising the full definition → metadata → registry pipeline.
	const sourceType = "gc_reg_defaults_src"
	const layerType = "gc_reg_defaults_lyr"

	defaults := domain.DefaultGeoCacheConfig()

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType(layerType)
	defer dynReg.Unregister(domain.SourceType(sourceType))

	def := minimalSourceDefinition(sourceType, layerType)
	def.GeoCache = &GeoCacheSpec{
		OverfetchRatio: 6.0,
		// AliveRatioThreshold and GCBatchSize are zero → should fall back to defaults.
	}

	sm := SourceMetadataFromDefinition(def)
	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata error: %v", err)
	}

	got, ok := dynReg.LookupGeoCacheConfig(lt)
	if !ok {
		t.Fatal("LookupGeoCacheConfig returned false, want registered config")
	}
	if got.OverfetchRatio != 6.0 {
		t.Errorf("OverfetchRatio = %v, want 6.0", got.OverfetchRatio)
	}
	if got.AliveRatioThreshold != defaults.AliveRatioThreshold {
		t.Errorf("AliveRatioThreshold = %v, want default %v",
			got.AliveRatioThreshold, defaults.AliveRatioThreshold)
	}
	if got.GCBatchSize != defaults.GCBatchSize {
		t.Errorf("GCBatchSize = %v, want default %v", got.GCBatchSize, defaults.GCBatchSize)
	}
}

// ---------------------------------------------------------------------------
// RegisterDisplayConfigs — geo_cache YAML → registry pipeline
// ---------------------------------------------------------------------------

func TestRegisterDisplayConfigs_GeoCachePresent(t *testing.T) {
	// A YAML file with a fully-specified geo_cache section must result in the
	// corresponding GeoCacheConfig being stored in the dynamic registry.
	dir := t.TempDir()

	yamlContent := `
schema_version: 2
name: geo_cache_yaml_src
source_type: geo_cache_yaml_src
layer_type: geo_cache_yaml_lyr
display_name: "Geo Cache YAML Test"
geo_cache:
  overfetch_ratio: 4.0
  alive_ratio_threshold: 0.6
  gc_batch_size: 300
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 6
`

	if err := os.WriteFile(filepath.Join(dir, "geo_cache_yaml_src.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test YAML: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("geo_cache_yaml_lyr")
	defer dynReg.Unregister(domain.SourceType("geo_cache_yaml_src"))

	if err := RegisterDisplayConfigs(dir, dynReg, nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs error: %v", err)
	}

	got, ok := dynReg.LookupGeoCacheConfig(lt)
	if !ok {
		t.Fatal("LookupGeoCacheConfig returned false, want registered config after YAML load")
	}
	if got.OverfetchRatio != 4.0 {
		t.Errorf("OverfetchRatio = %v, want 4.0", got.OverfetchRatio)
	}
	if got.AliveRatioThreshold != 0.6 {
		t.Errorf("AliveRatioThreshold = %v, want 0.6", got.AliveRatioThreshold)
	}
	if got.GCBatchSize != 300 {
		t.Errorf("GCBatchSize = %v, want 300", got.GCBatchSize)
	}
}

func TestRegisterDisplayConfigs_GeoCacheAbsent(t *testing.T) {
	// A YAML file without a geo_cache section must NOT register any geo cache
	// config for its layer type.
	dir := t.TempDir()

	yamlContent := `
schema_version: 1
name: no_geo_cache_src
source_type: no_geo_cache_src
layer_type: no_geo_cache_lyr
display_name: "No Geo Cache"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 6
`

	if err := os.WriteFile(filepath.Join(dir, "no_geo_cache_src.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test YAML: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("no_geo_cache_lyr")
	defer dynReg.Unregister(domain.SourceType("no_geo_cache_src"))

	if err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs error: %v", err)
	}

	_, ok := dynReg.LookupGeoCacheConfig(lt)
	if ok {
		t.Error("LookupGeoCacheConfig returned true, want false when YAML has no geo_cache section")
	}
}

func TestRegisterDisplayConfigs_GeoCachePartialYAML(t *testing.T) {
	// A YAML file with only one geo_cache field set must fall back to defaults
	// for the unspecified fields.
	dir := t.TempDir()
	defaults := domain.DefaultGeoCacheConfig()

	yamlContent := `
schema_version: 2
name: partial_geo_cache_src
source_type: partial_geo_cache_src
layer_type: partial_geo_cache_lyr
display_name: "Partial Geo Cache"
geo_cache:
  gc_batch_size: 750
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
display:
  icon:
    shape: diamond
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 6
`

	if err := os.WriteFile(filepath.Join(dir, "partial_geo_cache_src.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test YAML: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("partial_geo_cache_lyr")
	defer dynReg.Unregister(domain.SourceType("partial_geo_cache_src"))

	if err := RegisterDisplayConfigs(dir, dynReg, nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs error: %v", err)
	}

	got, ok := dynReg.LookupGeoCacheConfig(lt)
	if !ok {
		t.Fatal("LookupGeoCacheConfig returned false, want registered config")
	}
	// Explicitly set field must be preserved.
	if got.GCBatchSize != 750 {
		t.Errorf("GCBatchSize = %v, want 750", got.GCBatchSize)
	}
	// Unset fields must be filled with defaults.
	if got.OverfetchRatio != defaults.OverfetchRatio {
		t.Errorf("OverfetchRatio = %v, want default %v", got.OverfetchRatio, defaults.OverfetchRatio)
	}
	if got.AliveRatioThreshold != defaults.AliveRatioThreshold {
		t.Errorf("AliveRatioThreshold = %v, want default %v",
			got.AliveRatioThreshold, defaults.AliveRatioThreshold)
	}
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// minimalSourceDefinition returns a SourceDefinition with the minimum set of
// fields required for SourceMetadataFromDefinition to work correctly.
// GeoCache is intentionally left nil; tests can set it themselves.
func minimalSourceDefinition(sourceType, layerType string) *SourceDefinition {
	return &SourceDefinition{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
	}
}

func TestDisplaySpecToDomain(t *testing.T) {
	// Nil input
	if got := DisplaySpecToDomain(nil); got != nil {
		t.Error("expected nil for nil input")
	}

	// Valid input with field renderers
	precision := 2
	spec := &DisplaySpec{
		Icon: IconSpec{
			Shape:         "flight",
			Rotatable:     true,
			Interpolation: true,
			Scale:         1.5,
		},
		Trail: TrailSpec{
			Color:   "#ff006e",
			Width:   2.0,
			Opacity: 0.8,
		},
		Style: StyleSpec{
			Color:     "#00ff9d",
			PointSize: 8,
		},
		FieldRenderers: []FieldRendererSpec{
			{
				Keys:     []string{"speed"},
				Label:    "SPEED",
				Priority: 0,
				Format: FormatSpec{
					Type:      "float",
					Precision: &precision,
					Suffix:    " kts",
				},
			},
		},
	}

	dc := DisplaySpecToDomain(spec)
	if dc == nil {
		t.Fatal("expected non-nil result")
	}
	if dc.Icon.Shape != "flight" {
		t.Errorf("icon shape = %q, want %q", dc.Icon.Shape, "flight")
	}
	if !dc.Icon.Rotatable {
		t.Error("expected rotatable = true")
	}
	if dc.Trail.Color != "#ff006e" {
		t.Errorf("trail color = %q, want %q", dc.Trail.Color, "#ff006e")
	}
	if len(dc.FieldRenderers) != 1 {
		t.Fatalf("field renderers len = %d, want 1", len(dc.FieldRenderers))
	}
	fr := dc.FieldRenderers[0]
	if fr.Label != "SPEED" {
		t.Errorf("field renderer label = %q, want %q", fr.Label, "SPEED")
	}
	if fr.Format.Precision == nil || *fr.Format.Precision != 2 {
		t.Errorf("field renderer precision unexpected")
	}
	// color_by absent → domain ColorBy stays nil.
	if dc.ColorBy != nil {
		t.Error("expected ColorBy nil when color_by is absent")
	}
}

func TestDisplaySpecToDomain_ColorBy(t *testing.T) {
	spec := &DisplaySpec{
		Icon:  IconSpec{Shape: "radiation"},
		Trail: TrailSpec{Color: "#ff9900"},
		Style: StyleSpec{Color: "#ff9900", PointSize: 6},
		ColorBy: &ColorBySpec{
			Field:        "status",
			Values:       map[string]string{"operational": "#00ff9d", "shutdown": "#ff9900"},
			DefaultColor: "#888888",
		},
	}

	dc := DisplaySpecToDomain(spec)
	if dc.ColorBy == nil {
		t.Fatal("expected ColorBy to be mapped")
	}
	if dc.ColorBy.Field != "status" {
		t.Errorf("ColorBy.Field = %q, want %q", dc.ColorBy.Field, "status")
	}
	if dc.ColorBy.Values["operational"] != "#00ff9d" || dc.ColorBy.Values["shutdown"] != "#ff9900" {
		t.Errorf("ColorBy.Values not mapped correctly: %v", dc.ColorBy.Values)
	}
	if dc.ColorBy.DefaultColor != "#888888" {
		t.Errorf("ColorBy.DefaultColor = %q, want %q", dc.ColorBy.DefaultColor, "#888888")
	}
}

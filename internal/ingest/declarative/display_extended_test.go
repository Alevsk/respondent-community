package declarative

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// TestSourceMetadataFromDefinition verifies the conversion from SourceDefinition to SourceMetadata.
func TestSourceMetadataFromDefinition(t *testing.T) {
	threshold := 10
	backfill := &BackfillSpec{Threshold: threshold}
	history := &HistorySpec{
		MaxLookback:  Duration{Duration: 8760 * time.Hour},
		MaxRangeSpan: Duration{Duration: 168 * time.Hour},
	}
	indicator := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{Key: "kp", Label: "Kp Index", SourceField: "kp", MaxLevel: 5},
		},
	}
	def := &SourceDefinition{
		SourceType: "test_src",
		LayerType:  "test_layer",
		Display: DisplaySpec{
			Icon:  IconSpec{Shape: "circle", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		Filtering:  "viewport",
		EntityType: "global_indicator",
		Backfill:   backfill,
		History:    history,
		Indicator:  indicator,
		Transport: TransportSpec{
			OnDemandURL: "https://example.com/on-demand",
		},
		Cache: CacheSpec{
			TTL: Duration{Duration: 5 * time.Minute},
		},
	}

	sm := SourceMetadataFromDefinition(def)

	if sm.SourceType != "test_src" {
		t.Errorf("SourceType = %q, want %q", sm.SourceType, "test_src")
	}
	if sm.LayerType != "test_layer" {
		t.Errorf("LayerType = %q, want %q", sm.LayerType, "test_layer")
	}
	if sm.Filtering != "viewport" {
		t.Errorf("Filtering = %q, want %q", sm.Filtering, "viewport")
	}
	if sm.BackfillThreshold != threshold {
		t.Errorf("BackfillThreshold = %d, want %d", sm.BackfillThreshold, threshold)
	}
	if sm.OnDemandURL != "https://example.com/on-demand" {
		t.Errorf("OnDemandURL = %q", sm.OnDemandURL)
	}
	if sm.CacheTTL != 5*time.Minute {
		t.Errorf("CacheTTL = %v, want 5m", sm.CacheTTL)
	}
	if sm.EntityType != "global_indicator" {
		t.Errorf("EntityType = %q, want global_indicator", sm.EntityType)
	}
	if sm.History == nil {
		t.Error("expected non-nil History")
	}
	if sm.Indicator == nil {
		t.Error("expected non-nil Indicator")
	}
	if sm.Display == nil {
		t.Error("expected non-nil Display")
	}
}

// TestSourceMetadataFromDefinition_NilBackfill verifies nil backfill is handled.
func TestSourceMetadataFromDefinition_NilBackfill(t *testing.T) {
	def := &SourceDefinition{
		SourceType: "src",
		LayerType:  "layer",
		Display: DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#000000"},
			Style: StyleSpec{Color: "#000000", PointSize: 4},
		},
		Backfill: nil, // explicitly nil
	}

	sm := SourceMetadataFromDefinition(def)
	if sm.BackfillThreshold != 0 {
		t.Errorf("expected BackfillThreshold = 0, got %d", sm.BackfillThreshold)
	}
}

// TestRegisterSourceMetadata_IndicatorSource verifies indicator source registration.
func TestRegisterSourceMetadata_IndicatorSource(t *testing.T) {
	const sourceType = "indicator_test_src_reg"
	const layerType = "indicator_test_layer_reg"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "gauge", Scale: 1.0},
			Trail: TrailSpec{Color: "#ff0000"},
			Style: StyleSpec{Color: "#ff0000", PointSize: 6},
		},
		EntityType: string(domain.EntityTypeIndicator),
		Indicator: &IndicatorSpec{
			Values: []IndicatorValueSpec{
				{
					Key:             "kp",
					Label:           "Kp Index",
					SourceField:     "kp_value",
					Unit:            "",
					MaxLevel:        9,
					LevelThresholds: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8},
				},
			},
		},
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata: %v", err)
	}

	lt := domain.LayerType(layerType)
	mode, ok := dynReg.LookupRenderingMode(lt)
	if !ok {
		t.Fatal("expected rendering mode to be registered")
	}
	if mode != "indicator" {
		t.Errorf("rendering mode = %q, want %q", mode, "indicator")
	}
}

// TestRegisterSourceMetadata_WithCacheTTL verifies cache TTL registration.
func TestRegisterSourceMetadata_WithCacheTTL(t *testing.T) {
	const sourceType = "cache_ttl_test_src"
	const layerType = "cache_ttl_test_layer"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		CacheTTL: 15 * time.Minute,
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata: %v", err)
	}

	lt := domain.LayerType(layerType)
	allTTLs := dynReg.AllCacheTTLs()
	ttl, ok := allTTLs[lt]
	if !ok {
		t.Fatal("expected cache TTL to be registered")
	}
	if ttl != 15*time.Minute {
		t.Errorf("CacheTTL = %v, want 15m", ttl)
	}
}

// TestRegisterSourceMetadata_WithHistoryConfig verifies history config registration.
func TestRegisterSourceMetadata_WithHistoryConfig(t *testing.T) {
	const sourceType = "hist_test_src_2"
	const layerType = "hist_test_layer_2"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "diamond", Scale: 1.0},
			Trail: TrailSpec{Color: "#00ff00"},
			Style: StyleSpec{Color: "#00ff00", PointSize: 8},
		},
		History: &HistorySpec{
			MaxLookback:  Duration{Duration: 720 * time.Hour},
			MaxRangeSpan: Duration{Duration: 24 * time.Hour},
		},
	}

	if err := RegisterSourceMetadata(sm, dynReg); err != nil {
		t.Fatalf("RegisterSourceMetadata: %v", err)
	}

	lt := domain.LayerType(layerType)
	hc, ok := dynReg.LookupHistoryConfig(lt)
	if !ok {
		t.Fatal("expected history config to be registered")
	}
	if hc.MaxLookbackHours != 720 {
		t.Errorf("MaxLookbackHours = %d, want 720", hc.MaxLookbackHours)
	}
	if hc.MaxRangeSpanHours != 24 {
		t.Errorf("MaxRangeSpanHours = %d, want 24", hc.MaxRangeSpanHours)
	}
}

// TestRegisterSourceMetadata_InvalidIndicatorCEL verifies CEL compile errors are returned.
func TestRegisterSourceMetadata_InvalidIndicatorCEL(t *testing.T) {
	const sourceType = "bad_cel_src"
	const layerType = "bad_cel_layer"

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType(sourceType))

	sm := SourceMetadata{
		SourceType: sourceType,
		LayerType:  layerType,
		Display: &DisplaySpec{
			Icon:  IconSpec{Shape: "dot", Scale: 1.0},
			Trail: TrailSpec{Color: "#ffffff"},
			Style: StyleSpec{Color: "#ffffff", PointSize: 6},
		},
		Indicator: &IndicatorSpec{
			Values: []IndicatorValueSpec{
				{
					Key:       "bad",
					LevelExpr: "this is not valid CEL !!!@@@",
				},
			},
		},
	}

	err := RegisterSourceMetadata(sm, dynReg)
	if err == nil {
		t.Fatal("expected error for invalid CEL expression, got nil")
	}
}

// TestRegisterDisplayConfigs_WithCacheTTL verifies that cache TTL from YAML is registered.
func TestRegisterDisplayConfigs_WithCacheTTL(t *testing.T) {
	dir := t.TempDir()

	yamlContent := `
schema_version: 1
name: cache_ttl_display_test
source_type: cache_ttl_display_src
layer_type: cache_ttl_display_layer
display_name: "Cache TTL Test"
cache:
  ttl: "15m0s"
display:
  icon:
    shape: circle
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.0
    opacity: 0.5
  style:
    color: "#00ff9d"
    point_size: 6
`

	if err := os.WriteFile(filepath.Join(dir, "cache_test.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType("cache_ttl_display_src"))

	if err := RegisterDisplayConfigs(dir, dynReg, nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs: %v", err)
	}

	lt := domain.LayerType("cache_ttl_display_layer")
	allTTLs := dynReg.AllCacheTTLs()
	ttl, ok := allTTLs[lt]
	if !ok {
		t.Fatal("expected cache TTL to be registered")
	}
	if ttl != 15*time.Minute {
		t.Errorf("CacheTTL = %v, want 15m", ttl)
	}
}

// TestRegisterDisplayConfigs_RegistersWithoutAuth verifies that a source's display
// and layer metadata register even when its ingestion credentials are unresolved.
// This is the root-cause fix for layers (e.g. air_quality, internet_outages) whose
// sources require an auth env_var that is unset: the full loader rejects them, but
// their static display config (color, icon, filtering mode) must still be available
// so persisted entities render with the right icon/color instead of #ffffff.
func TestRegisterDisplayConfigs_RegistersWithoutAuth(t *testing.T) {
	dir := t.TempDir()

	// Mirrors openaq_air_quality.yaml: requires an auth env_var (left UNSET here),
	// declares filtering: viewport and a display color.
	t.Setenv("DISPLAY_TEST_API_KEY", "") // explicitly unset/empty
	yamlContent := `
schema_version: 2
name: noauth_display_test
source_type: noauth_display_src
layer_type: noauth_display_layer
display_name: "No-Auth Display Test"
enabled: true
filtering: viewport
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  auth:
    type: api_key
    header: "X-API-Key"
    env_var: "DISPLAY_TEST_API_KEY"
display:
  icon:
    shape: cloud
    rotatable: false
    interpolation: false
    scale: 1.0
  style:
    color: "#88cc00"
    point_size: 6
`
	if err := os.WriteFile(filepath.Join(dir, "noauth.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	defer dynReg.Unregister(domain.SourceType("noauth_display_src"))

	if err := RegisterDisplayConfigs(dir, dynReg, nil); err != nil {
		t.Fatalf("RegisterDisplayConfigs: %v", err)
	}

	lt := domain.LayerType("noauth_display_layer")

	dc, ok := dynReg.LookupDisplayConfig(lt)
	if !ok {
		t.Fatal("expected display config to be registered despite unset auth env_var")
	}
	if dc.Style == nil || dc.Style.Color != "#88cc00" {
		t.Errorf("display color = %+v, want #88cc00", dc.Style)
	}

	style := dynReg.GetLayerStyle(lt)
	if style.Color != "#88cc00" {
		t.Errorf("GetLayerStyle color = %q, want #88cc00 (not the #ffffff fallback)", style.Color)
	}

	fm, ok := dynReg.LookupFilteringMode(lt)
	if !ok || fm != string(domain.FilteringViewport) {
		t.Errorf("filtering mode = %q (ok=%v), want %q", fm, ok, domain.FilteringViewport)
	}
}

// TestRegisterDisplayConfigs_SkipsInvalidYAML verifies bad YAML is skipped gracefully.
func TestRegisterDisplayConfigs_SkipsInvalidYAML(t *testing.T) {
	dir := t.TempDir()

	// Write invalid YAML
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("{{ invalid yaml"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should not return an error, just skip the bad file
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("expected nil error for invalid YAML file, got: %v", err)
	}
}

// TestRegisterDisplayConfigs_SkipsMissingLayerType verifies that files without layer_type or icon shape are skipped.
func TestRegisterDisplayConfigs_SkipsMissingLayerType(t *testing.T) {
	dir := t.TempDir()

	// YAML without layer_type
	yamlContent := `
schema_version: 1
name: incomplete_source
source_type: incomplete_src
display_name: "Incomplete"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	if err := os.WriteFile(filepath.Join(dir, "incomplete.yaml"), []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Should not return error, just skip
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

// TestRegisterDisplayConfigs_Subdirectory verifies subdirectories are skipped.
func TestRegisterDisplayConfigs_Subdirectory(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory (should be skipped)
	subdir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Should not error
	err := RegisterDisplayConfigs(dir, domain.NewDynamicSourceRegistry(), nil)
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

// TestDisplaySpecToDomain_NilPrecision verifies nil precision field renderer.
func TestDisplaySpecToDomain_NilPrecision(t *testing.T) {
	spec := &DisplaySpec{
		Icon:  IconSpec{Shape: "dot", Scale: 1.0},
		Trail: TrailSpec{Color: "#000000"},
		Style: StyleSpec{Color: "#000000", PointSize: 4},
		FieldRenderers: []FieldRendererSpec{
			{
				Keys:  []string{"value"},
				Label: "Value",
				Format: FormatSpec{
					Type:      "string",
					Precision: nil, // nil precision
				},
			},
		},
	}

	dc := DisplaySpecToDomain(spec)
	if dc == nil {
		t.Fatal("expected non-nil result")
	}
	if len(dc.FieldRenderers) != 1 {
		t.Fatalf("expected 1 field renderer, got %d", len(dc.FieldRenderers))
	}
	if dc.FieldRenderers[0].Format.Precision != nil {
		t.Error("expected nil precision in domain config")
	}
}

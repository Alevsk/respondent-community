package domain

import (
	"testing"
	"time"
)

func TestDynamicRegistry_RegisterAndLookup(t *testing.T) {
	tests := []struct {
		name       string
		sourceType SourceType
		layerType  LayerType
	}{
		{
			name:       "register marine AIS source",
			sourceType: SourceType("marine_ais"),
			layerType:  LayerType("marine_vessels"),
		},
		{
			name:       "register weather stations",
			sourceType: SourceType("weather_stations"),
			layerType:  LayerType("weather"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &DynamicSourceRegistry{
				sourceToLayer:  make(map[SourceType]LayerType),
				layerToSources: make(map[LayerType][]SourceType),
			}

			reg.Register(tt.sourceType, tt.layerType)

			// Verify lookup
			lt, ok := reg.LookupLayerType(tt.sourceType)
			if !ok {
				t.Fatalf("LookupLayerType(%q) returned false", tt.sourceType)
			}
			if lt != tt.layerType {
				t.Errorf("LookupLayerType(%q) = %q, want %q", tt.sourceType, lt, tt.layerType)
			}

			// Verify IsRegistered
			if !reg.IsRegistered(tt.sourceType) {
				t.Errorf("IsRegistered(%q) = false, want true", tt.sourceType)
			}

			// Verify reverse lookup
			sources := reg.LookupSourceTypes(tt.layerType)
			if len(sources) != 1 || sources[0] != tt.sourceType {
				t.Errorf("LookupSourceTypes(%q) = %v, want [%q]", tt.layerType, sources, tt.sourceType)
			}
		})
	}
}

func TestDynamicRegistry_Unregister(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	st := SourceType("test_source")
	lt := LayerType("test_layer")

	reg.Register(st, lt)

	// Verify registered
	if !reg.IsRegistered(st) {
		t.Fatal("expected source to be registered")
	}

	// Unregister
	reg.Unregister(st)

	// Verify unregistered
	if reg.IsRegistered(st) {
		t.Error("expected source to be unregistered")
	}

	_, ok := reg.LookupLayerType(st)
	if ok {
		t.Error("expected LookupLayerType to return false after unregister")
	}

	sources := reg.LookupSourceTypes(lt)
	if len(sources) != 0 {
		t.Errorf("LookupSourceTypes should be empty after unregister, got %v", sources)
	}
}

func TestDynamicRegistry_UnregisterNonExistent(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	// Should not panic
	reg.Unregister(SourceType("nonexistent"))
}

func TestDynamicRegistry_MultipleSourcesSameLayer(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	lt := LayerType("shared_layer")
	st1 := SourceType("source_a")
	st2 := SourceType("source_b")

	reg.Register(st1, lt)
	reg.Register(st2, lt)

	sources := reg.LookupSourceTypes(lt)
	if len(sources) != 2 {
		t.Errorf("expected 2 sources for layer, got %d", len(sources))
	}

	// Unregister one
	reg.Unregister(st1)
	sources = reg.LookupSourceTypes(lt)
	if len(sources) != 1 {
		t.Errorf("expected 1 source after unregister, got %d", len(sources))
	}
	if sources[0] != st2 {
		t.Errorf("remaining source = %q, want %q", sources[0], st2)
	}
}

func TestDynamicRegistry_DuplicateRegister(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	st := SourceType("dup_source")
	lt := LayerType("dup_layer")

	reg.Register(st, lt)
	reg.Register(st, lt) // Should not duplicate

	sources := reg.LookupSourceTypes(lt)
	if len(sources) != 1 {
		t.Errorf("expected 1 source (no duplicate), got %d", len(sources))
	}
}

func TestDynamicRegistry_ChangeLayerType(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	st := SourceType("changing_source")
	lt1 := LayerType("layer_v1")
	lt2 := LayerType("layer_v2")

	reg.Register(st, lt1)
	reg.Register(st, lt2) // Change layer type

	// Should be in lt2 now
	lt, ok := reg.LookupLayerType(st)
	if !ok || lt != lt2 {
		t.Errorf("LookupLayerType = (%q, %v), want (%q, true)", lt, ok, lt2)
	}

	// Should be removed from lt1
	sources := reg.LookupSourceTypes(lt1)
	if len(sources) != 0 {
		t.Errorf("old layer should have 0 sources, got %d", len(sources))
	}

	// Should be in lt2
	sources = reg.LookupSourceTypes(lt2)
	if len(sources) != 1 {
		t.Errorf("new layer should have 1 source, got %d", len(sources))
	}
}

func TestDynamicRegistry_AllSourceTypes(t *testing.T) {
	reg := &DynamicSourceRegistry{
		sourceToLayer:  make(map[SourceType]LayerType),
		layerToSources: make(map[LayerType][]SourceType),
	}

	reg.Register(SourceType("a"), LayerType("la"))
	reg.Register(SourceType("b"), LayerType("lb"))

	all := reg.AllSourceTypes()
	if len(all) != 2 {
		t.Errorf("AllSourceTypes len = %d, want 2", len(all))
	}
}

// TestDynamicRegistry_IntegrationWithSourceType verifies that source-type
// validity and layer-type resolution flow through a registry instance.
func TestDynamicRegistry_IntegrationWithSourceType(t *testing.T) {
	reg := NewDynamicSourceRegistry()
	st := SourceType("dynamic_test_source_xyz")
	lt := LayerType("dynamic_test_layer_xyz")

	// Before registration
	if reg.IsRegistered(st) {
		t.Error("expected dynamic source to be invalid before registration")
	}

	reg.Register(st, lt)

	// After registration
	if !reg.IsRegistered(st) {
		t.Error("expected dynamic source to be valid after registration")
	}

	// LayerType lookup
	gotLT, ok := reg.LookupLayerType(st)
	if !ok || gotLT != lt {
		t.Errorf("LookupLayerType() = %q, %v; want %q, true", gotLT, ok, lt)
	}
}

// newTestRegistry builds a fresh registry with all 10 map fields initialised.
func newTestRegistry() *DynamicSourceRegistry {
	return &DynamicSourceRegistry{
		sourceToLayer:      make(map[SourceType]LayerType),
		layerToSources:     make(map[LayerType][]SourceType),
		displayConfigs:     make(map[LayerType]*LayerDisplayConfig),
		filteringModes:     make(map[LayerType]string),
		backfillThresholds: make(map[LayerType]int),
		onDemandURLs:       make(map[LayerType]string),
		historyConfigs:     make(map[LayerType]*HistoryConfig),
		cacheTTLs:          make(map[LayerType]time.Duration),
		renderingModes:     make(map[LayerType]string),
		indicatorSpecs:     make(map[LayerType]*IndicatorSpec),
		geoCacheConfigs:    make(map[LayerType]GeoCacheConfig),
	}
}

func TestDynamicRegistry_HistoryConfig_RoundTrip(t *testing.T) {
	tests := []struct {
		name              string
		lt                LayerType
		maxLookbackHours  int32
		maxRangeSpanHours int32
	}{
		{
			name:              "one year lookback, one week span",
			lt:                LayerType("marine_ais"),
			maxLookbackHours:  8760,
			maxRangeSpanHours: 168,
		},
		{
			name:              "small window",
			lt:                LayerType("live_sensors"),
			maxLookbackHours:  1,
			maxRangeSpanHours: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newTestRegistry()

			hc := &HistoryConfig{
				MaxLookbackHours:  tt.maxLookbackHours,
				MaxRangeSpanHours: tt.maxRangeSpanHours,
			}
			reg.SetHistoryConfig(tt.lt, hc)

			got, ok := reg.LookupHistoryConfig(tt.lt)
			if !ok {
				t.Fatalf("LookupHistoryConfig(%q) returned false, want true", tt.lt)
			}
			if got == nil {
				t.Fatalf("LookupHistoryConfig(%q) returned nil config", tt.lt)
			}
			if got.MaxLookbackHours != tt.maxLookbackHours {
				t.Errorf("MaxLookbackHours = %d, want %d", got.MaxLookbackHours, tt.maxLookbackHours)
			}
			if got.MaxRangeSpanHours != tt.maxRangeSpanHours {
				t.Errorf("MaxRangeSpanHours = %d, want %d", got.MaxRangeSpanHours, tt.maxRangeSpanHours)
			}
		})
	}
}

func TestDynamicRegistry_LookupHistoryConfig_NotRegistered(t *testing.T) {
	reg := newTestRegistry()

	hc, ok := reg.LookupHistoryConfig(LayerType("never_registered"))
	if ok {
		t.Error("LookupHistoryConfig returned true for unregistered layer, want false")
	}
	if hc != nil {
		t.Errorf("LookupHistoryConfig returned non-nil for unregistered layer: %+v", hc)
	}
}

func TestDynamicRegistry_SetHistoryConfig_Nil(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("nil_history_layer")

	// Storing nil must not panic and must be retrievable as (nil, true).
	reg.SetHistoryConfig(lt, nil)

	got, ok := reg.LookupHistoryConfig(lt)
	if !ok {
		t.Fatal("LookupHistoryConfig returned false after SetHistoryConfig(nil), want true")
	}
	if got != nil {
		t.Errorf("LookupHistoryConfig returned %+v after nil set, want nil", got)
	}
}

func TestDynamicRegistry_SetHistoryConfig_Overwrite(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("overwrite_layer")

	first := &HistoryConfig{MaxLookbackHours: 24, MaxRangeSpanHours: 12}
	reg.SetHistoryConfig(lt, first)

	second := &HistoryConfig{MaxLookbackHours: 720, MaxRangeSpanHours: 168}
	reg.SetHistoryConfig(lt, second)

	got, ok := reg.LookupHistoryConfig(lt)
	if !ok {
		t.Fatal("LookupHistoryConfig returned false after overwrite, want true")
	}
	if got.MaxLookbackHours != second.MaxLookbackHours {
		t.Errorf("MaxLookbackHours after overwrite = %d, want %d",
			got.MaxLookbackHours, second.MaxLookbackHours)
	}
	if got.MaxRangeSpanHours != second.MaxRangeSpanHours {
		t.Errorf("MaxRangeSpanHours after overwrite = %d, want %d",
			got.MaxRangeSpanHours, second.MaxRangeSpanHours)
	}
}

func TestDynamicRegistry_AllLayerTypes(t *testing.T) {
	reg := newTestRegistry()

	// Empty registry returns empty slice.
	if got := reg.AllLayerTypes(); len(got) != 0 {
		t.Errorf("AllLayerTypes on empty registry = %v, want []", got)
	}

	// Register two sources mapping to two distinct layer types.
	reg.Register(SourceType("src_a"), LayerType("layer_a"))
	reg.Register(SourceType("src_b"), LayerType("layer_b"))

	all := reg.AllLayerTypes()
	if len(all) != 2 {
		t.Fatalf("AllLayerTypes len = %d, want 2; got %v", len(all), all)
	}

	seen := make(map[LayerType]bool)
	for _, lt := range all {
		seen[lt] = true
	}
	if !seen[LayerType("layer_a")] {
		t.Error("AllLayerTypes missing layer_a")
	}
	if !seen[LayerType("layer_b")] {
		t.Error("AllLayerTypes missing layer_b")
	}

	// After unregistering the only source for layer_a, it should disappear.
	reg.Unregister(SourceType("src_a"))
	all = reg.AllLayerTypes()
	if len(all) != 1 {
		t.Fatalf("AllLayerTypes after unregister len = %d, want 1; got %v", len(all), all)
	}
	if all[0] != LayerType("layer_b") {
		t.Errorf("AllLayerTypes after unregister = %q, want layer_b", all[0])
	}
}

func TestDynamicRegistry_GetLayerStyle(t *testing.T) {
	reg := newTestRegistry()

	lt := LayerType("my_layer")

	// Without any display config, must return DefaultLayerStyle.
	got := reg.GetLayerStyle(lt)
	if got != DefaultLayerStyle {
		t.Errorf("GetLayerStyle (no config) = %+v, want DefaultLayerStyle %+v", got, DefaultLayerStyle)
	}

	// Register a source with a display config that has a Style.
	reg.RegisterWithDisplay(SourceType("my_src"), lt, &LayerDisplayConfig{
		Style: &StyleConfig{Color: "#aabbcc", PointSize: 12},
	})

	got = reg.GetLayerStyle(lt)
	if got.Color != "#aabbcc" {
		t.Errorf("GetLayerStyle Color = %q, want #aabbcc", got.Color)
	}
	if got.PointSize != 12 {
		t.Errorf("GetLayerStyle PointSize = %d, want 12", got.PointSize)
	}

	// A display config with a nil Style must fall back to DefaultLayerStyle.
	lt2 := LayerType("nil_style_layer")
	reg.RegisterWithDisplay(SourceType("nil_src"), lt2, &LayerDisplayConfig{
		Style: nil,
	})
	got = reg.GetLayerStyle(lt2)
	if got != DefaultLayerStyle {
		t.Errorf("GetLayerStyle (nil Style) = %+v, want DefaultLayerStyle %+v", got, DefaultLayerStyle)
	}
}

func TestDynamicRegistry_LookupDisplayConfig(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("display_layer")

	_, ok := reg.LookupDisplayConfig(lt)
	if ok {
		t.Error("expected LookupDisplayConfig to return false before set")
	}

	dc := &LayerDisplayConfig{
		Style: &StyleConfig{Color: "#ff0000", PointSize: 5},
		Icon:  &IconConfig{Shape: "circle", Rotatable: true},
	}
	reg.RegisterWithDisplay(SourceType("display_src"), lt, dc)

	got, ok := reg.LookupDisplayConfig(lt)
	if !ok {
		t.Fatal("expected LookupDisplayConfig to return true after set")
	}
	if got.Style.Color != "#ff0000" {
		t.Errorf("LookupDisplayConfig Style.Color = %q, want #ff0000", got.Style.Color)
	}
}

func TestDynamicRegistry_CacheTTL(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("cache_layer")

	reg.SetCacheTTL(lt, 5*time.Minute)

	all := reg.AllCacheTTLs()
	if len(all) != 1 {
		t.Errorf("AllCacheTTLs len = %d, want 1", len(all))
	}
	if all[lt] != 5*time.Minute {
		t.Errorf("AllCacheTTLs[%q] = %v, want 5m", lt, all[lt])
	}
}

func TestDynamicRegistry_RenderingMode(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("rendering_layer")

	_, ok := reg.LookupRenderingMode(lt)
	if ok {
		t.Error("expected LookupRenderingMode to return false before set")
	}

	reg.SetRenderingMode(lt, "indicator")

	mode, ok := reg.LookupRenderingMode(lt)
	if !ok {
		t.Fatal("expected LookupRenderingMode to return true after set")
	}
	if mode != "indicator" {
		t.Errorf("LookupRenderingMode = %q, want indicator", mode)
	}
}

func TestDynamicRegistry_IndicatorSpec(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("indicator_layer")

	_, ok := reg.LookupIndicatorSpec(lt)
	if ok {
		t.Error("expected LookupIndicatorSpec to return false before set")
	}

	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{Key: "mag", Label: "Magnitude"},
		},
	}
	reg.SetIndicatorSpec(lt, spec)

	got, ok := reg.LookupIndicatorSpec(lt)
	if !ok {
		t.Fatal("expected LookupIndicatorSpec to return true after set")
	}
	if len(got.Values) != 1 {
		t.Errorf("LookupIndicatorSpec Values len = %d, want 1", len(got.Values))
	}
}

func TestDynamicRegistry_AllIndicatorLayerTypes(t *testing.T) {
	reg := newTestRegistry()

	reg.SetRenderingMode(LayerType("indicator_a"), "indicator")
	reg.SetRenderingMode(LayerType("indicator_b"), "indicator")
	reg.SetRenderingMode(LayerType("map_layer"), "map")

	all := reg.AllIndicatorLayerTypes()
	if len(all) != 2 {
		t.Errorf("AllIndicatorLayerTypes len = %d, want 2", len(all))
	}

	seen := make(map[LayerType]bool)
	for _, lt := range all {
		seen[lt] = true
	}
	if !seen[LayerType("indicator_a")] || !seen[LayerType("indicator_b")] {
		t.Error("AllIndicatorLayerTypes missing expected layers")
	}
}

func TestDynamicRegistry_GeoCacheConfig_RoundTrip(t *testing.T) {
	tests := []struct {
		name                string
		lt                  LayerType
		overfetchRatio      float64
		aliveRatioThreshold float64
		gcBatchSize         int
	}{
		{
			name:                "high-volume flights config",
			lt:                  LayerType("flights_commercial"),
			overfetchRatio:      3.0,
			aliveRatioThreshold: 0.5,
			gcBatchSize:         500,
		},
		{
			name:                "low-volume earthquakes config",
			lt:                  LayerType("earthquakes_usgs"),
			overfetchRatio:      2.0,
			aliveRatioThreshold: 0.5,
			gcBatchSize:         500,
		},
		{
			name:                "aggressive config",
			lt:                  LayerType("satellites_celestrak"),
			overfetchRatio:      5.0,
			aliveRatioThreshold: 0.3,
			gcBatchSize:         1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newTestRegistry()

			cfg := GeoCacheConfig{
				OverfetchRatio:      tt.overfetchRatio,
				AliveRatioThreshold: tt.aliveRatioThreshold,
				GCBatchSize:         tt.gcBatchSize,
			}
			reg.SetGeoCacheConfig(tt.lt, cfg)

			got, ok := reg.LookupGeoCacheConfig(tt.lt)
			if !ok {
				t.Fatalf("LookupGeoCacheConfig(%q) returned false, want true", tt.lt)
			}
			if got.OverfetchRatio != tt.overfetchRatio {
				t.Errorf("OverfetchRatio = %f, want %f", got.OverfetchRatio, tt.overfetchRatio)
			}
			if got.AliveRatioThreshold != tt.aliveRatioThreshold {
				t.Errorf("AliveRatioThreshold = %f, want %f", got.AliveRatioThreshold, tt.aliveRatioThreshold)
			}
			if got.GCBatchSize != tt.gcBatchSize {
				t.Errorf("GCBatchSize = %d, want %d", got.GCBatchSize, tt.gcBatchSize)
			}
		})
	}
}

func TestDynamicRegistry_LookupGeoCacheConfig_NotRegistered(t *testing.T) {
	reg := newTestRegistry()

	_, ok := reg.LookupGeoCacheConfig(LayerType("never_registered"))
	if ok {
		t.Error("LookupGeoCacheConfig returned true for unregistered layer, want false")
	}
}

func TestDynamicRegistry_SetGeoCacheConfig_Overwrite(t *testing.T) {
	reg := newTestRegistry()
	lt := LayerType("overwrite_gc_layer")

	first := GeoCacheConfig{OverfetchRatio: 2.0, AliveRatioThreshold: 0.5, GCBatchSize: 100}
	reg.SetGeoCacheConfig(lt, first)

	second := GeoCacheConfig{OverfetchRatio: 5.0, AliveRatioThreshold: 0.3, GCBatchSize: 1000}
	reg.SetGeoCacheConfig(lt, second)

	got, ok := reg.LookupGeoCacheConfig(lt)
	if !ok {
		t.Fatal("LookupGeoCacheConfig returned false after overwrite, want true")
	}
	if got.OverfetchRatio != second.OverfetchRatio {
		t.Errorf("OverfetchRatio after overwrite = %f, want %f",
			got.OverfetchRatio, second.OverfetchRatio)
	}
	if got.GCBatchSize != second.GCBatchSize {
		t.Errorf("GCBatchSize after overwrite = %d, want %d",
			got.GCBatchSize, second.GCBatchSize)
	}
}

func TestDefaultGeoCacheConfig(t *testing.T) {
	cfg := DefaultGeoCacheConfig()

	if cfg.OverfetchRatio != 3.0 {
		t.Errorf("DefaultGeoCacheConfig().OverfetchRatio = %f, want 3.0", cfg.OverfetchRatio)
	}
	if cfg.AliveRatioThreshold != 0.5 {
		t.Errorf("DefaultGeoCacheConfig().AliveRatioThreshold = %f, want 0.5", cfg.AliveRatioThreshold)
	}
	if cfg.GCBatchSize != 500 {
		t.Errorf("DefaultGeoCacheConfig().GCBatchSize = %d, want 500", cfg.GCBatchSize)
	}
}

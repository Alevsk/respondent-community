package indicator

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/app/layer/layertest"
	"github.com/Alevsk/respondent/internal/domain"
)

func TestIndicatorService_GetGlobalIndicators_NoIndicatorLayers(t *testing.T) {
	svc := NewIndicatorService(nil, nil, nil, domain.NewDynamicSourceRegistry())

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots, got %d", len(snapshots))
	}
}

func TestIndicatorService_GetGlobalIndicators_WithCacheData(t *testing.T) {
	// Register a global indicator layer in the dynamic registry.
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator")
	st := domain.SourceType("test_indicator_source")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "r_scale", Label: "Radio Blackout", SourceField: "r_scale", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
			{Key: "kp_index", Label: "Kp Index", SourceField: "kp", MaxLevel: 9, LevelThresholds: []float64{0, 2, 4, 5, 7, 9}, ComputeLevel: domain.DefaultComputeLevel([]float64{0, 2, 4, 5, 7, 9}, 9)},
		},
	})
	defer func() {
		dynReg.Unregister(st)
	}()

	// Set up cache with a test entity + observation.
	cache := layertest.NewFakeCacheStorage()
	entity := &domain.Entity{
		ID:         "test_indicator:noaa_space_weather",
		ExternalID: "noaa_space_weather",
		LayerType:  "test_indicator",
		Name:       "Test Indicator",
	}
	obs := &domain.Observation{
		EntityID:   "test_indicator:noaa_space_weather",
		Timestamp:  time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		SourceType: "test_indicator",
		Metadata: map[string]string{
			"r_scale": "2",
			"kp":      "3.33",
		},
	}
	if err := cache.SetEntity(context.Background(), entity, obs); err != nil {
		t.Fatalf("SetEntity: %v", err)
	}

	svc := NewIndicatorService(nil, nil, cache, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}

	snap := snapshots[0]
	if snap.LayerID != "test_indicator" {
		t.Errorf("expected layer_id 'test_indicator', got %q", snap.LayerID)
	}
	if len(snap.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(snap.Values))
	}

	// Check r_scale: value "2" with no thresholds should use direct level = 2.
	rVal := findIndicatorValue(snap.Values, "r_scale")
	if rVal == nil {
		t.Fatal("expected r_scale value")
	}
	if rVal.Value != "2" {
		t.Errorf("expected r_scale value '2', got %q", rVal.Value)
	}
	if rVal.Level != 2 {
		t.Errorf("expected r_scale level 2, got %d", rVal.Level)
	}

	// Check kp_index: value "3.33" with thresholds [0,2,4,5,7,9] -> level 1 (>= 2, < 4).
	kpVal := findIndicatorValue(snap.Values, "kp_index")
	if kpVal == nil {
		t.Fatal("expected kp_index value")
	}
	if kpVal.Value != "3.33" {
		t.Errorf("expected kp_index value '3.33', got %q", kpVal.Value)
	}
	if kpVal.Level != 1 {
		t.Errorf("expected kp_index level 1, got %d", kpVal.Level)
	}
}

func TestIndicatorService_LevelThresholds(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		thresholds []float64
		maxLevel   int
		wantLevel  int32
	}{
		{"kp_0", "0.5", []float64{0, 2, 4, 5, 7, 9}, 9, 0},
		{"kp_1", "3.33", []float64{0, 2, 4, 5, 7, 9}, 9, 1},
		{"kp_2", "4.0", []float64{0, 2, 4, 5, 7, 9}, 9, 2},
		{"kp_3", "5.5", []float64{0, 2, 4, 5, 7, 9}, 9, 3},
		{"kp_4", "7.0", []float64{0, 2, 4, 5, 7, 9}, 9, 4},
		{"kp_5", "9.0", []float64{0, 2, 4, 5, 7, 9}, 9, 5},
		{"no_thresholds_0", "0", nil, 5, 0},
		{"no_thresholds_3", "3", nil, 5, 3},
		{"no_thresholds_clamped", "10", nil, 5, 5},
		{"invalid_value", "abc", []float64{0, 2, 4}, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := domain.DefaultComputeLevel(tt.thresholds, tt.maxLevel)
			got := fn(tt.value, "0")
			if got != tt.wantLevel {
				t.Errorf("DefaultComputeLevel(%v, %d)(%q) = %d, want %d", tt.thresholds, tt.maxLevel, tt.value, got, tt.wantLevel)
			}
		})
	}
}

func TestIndicatorService_GetGlobalIndicators_FilterByLayerID(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection

	lt1 := domain.LayerType("ind_layer_1")
	st1 := domain.SourceType("ind_source_1")
	dynReg.Register(st1, lt1)
	dynReg.SetRenderingMode(lt1, "indicator")
	dynReg.SetIndicatorSpec(lt1, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Value 1", SourceField: "val1", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})

	lt2 := domain.LayerType("ind_layer_2")
	st2 := domain.SourceType("ind_source_2")
	dynReg.Register(st2, lt2)
	dynReg.SetRenderingMode(lt2, "indicator")
	dynReg.SetIndicatorSpec(lt2, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val2", Label: "Value 2", SourceField: "val2", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})

	defer func() {
		dynReg.Unregister(st1)
		dynReg.Unregister(st2)
	}()

	cache := layertest.NewFakeCacheStorage()
	for _, lt := range []string{"ind_layer_1", "ind_layer_2"} {
		entity := &domain.Entity{
			ID:         lt + ":test",
			ExternalID: "test",
			LayerType:  lt,
			Name:       "Test",
		}
		obs := &domain.Observation{
			EntityID:   lt + ":test",
			Timestamp:  time.Now(),
			SourceType: lt,
			Metadata:   map[string]string{"val1": "1", "val2": "2"},
		}
		_ = cache.SetEntity(context.Background(), entity, obs)
	}

	svc := NewIndicatorService(nil, nil, cache, dynReg)

	// Filter to only layer 1.
	snapshots, err := svc.GetGlobalIndicators(context.Background(), []string{"ind_layer_1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].LayerID != "ind_layer_1" {
		t.Errorf("expected layer_id 'ind_layer_1', got %q", snapshots[0].LayerID)
	}
}

func TestLevelToSummary(t *testing.T) {
	tests := []struct {
		level int32
		want  string
	}{
		{0, "Quiet"},
		{1, "Minor Activity"},
		{2, "Moderate"},
		{3, "Strong"},
		{4, "Severe"},
		{5, "Extreme"},
	}
	summaryFn := domain.DefaultComputeSummary()
	for _, tt := range tests {
		got := summaryFn(nil, tt.level)
		if got != tt.want {
			t.Errorf("DefaultComputeSummary()(nil, %d) = %q, want %q", tt.level, got, tt.want)
		}
	}
}

// findIndicatorValue finds a domain.IndicatorValue by key in a slice.
func findIndicatorValue(values []domain.IndicatorValue, key string) *domain.IndicatorValue {
	for i := range values {
		if values[i].Key == key {
			return &values[i]
		}
	}
	return nil
}

func TestIndicatorService_GetGlobalIndicators_EmptyCache_DBFallback(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_fallback")
	st := domain.SourceType("test_indicator_fallback_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Value 1", SourceField: "val1", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	entityRepo := newStubEntityRepo()
	obsRepo := newStubObsRepo()

	entity := &domain.Entity{
		ID:         "test_indicator_fallback:test",
		ExternalID: "test",
		LayerType:  "test_indicator_fallback",
		Name:       "Test",
	}
	obs := &domain.Observation{
		EntityID:   "test_indicator_fallback:test",
		Timestamp:  time.Now(),
		SourceType: "test_indicator_fallback",
		Metadata:   map[string]string{"val1": "3"},
	}
	entityRepo.add(entity)
	obsRepo.add(obs)

	svc := NewIndicatorService(entityRepo, obsRepo, nil, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
}

func TestIndicatorService_GetGlobalIndicators_NilRepos(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_nil")
	st := domain.SourceType("test_indicator_nil_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Value 1", SourceField: "val1", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	svc := NewIndicatorService(nil, nil, nil, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots with nil repos, got %d", len(snapshots))
	}
}

func TestIndicatorService_GetGlobalIndicators_NonIndicatorLayerFiltered(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_non_indicator")
	st := domain.SourceType("test_non_indicator_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "map")
	defer dynReg.Unregister(st)

	cache := layertest.NewFakeCacheStorage()
	svc := NewIndicatorService(nil, nil, cache, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), []string{"test_non_indicator"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots for non-indicator layer, got %d", len(snapshots))
	}
}

func TestIndicatorService_GetGlobalIndicators_NoSpec(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_no_spec")
	st := domain.SourceType("test_indicator_no_spec_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	defer dynReg.Unregister(st)

	cache := layertest.NewFakeCacheStorage()
	svc := NewIndicatorService(nil, nil, cache, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots when no spec, got %d", len(snapshots))
	}
}

func TestIndicatorService_GetGlobalIndicators_MissingField(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_missing")
	st := domain.SourceType("test_indicator_missing_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "missing_field", Label: "Missing Field", SourceField: "nonexistent", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	cache := layertest.NewFakeCacheStorage()
	entity := &domain.Entity{
		ID:         "test_indicator_missing:test",
		ExternalID: "test",
		LayerType:  "test_indicator_missing",
		Name:       "Test",
	}
	obs := &domain.Observation{
		EntityID:   "test_indicator_missing:test",
		Timestamp:  time.Now(),
		SourceType: "test_indicator_missing",
		Metadata:   map[string]string{"other_field": "value"},
	}
	_ = cache.SetEntity(context.Background(), entity, obs)

	svc := NewIndicatorService(nil, nil, cache, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected 1 snapshot, got %d", len(snapshots))
	}
	if snapshots[0].Values[0].Value != "0" {
		t.Errorf("expected default value '0' for missing field, got %s", snapshots[0].Values[0].Value)
	}
}

func TestIndicatorService_GetGlobalIndicators_DBError(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_indicator_dberr")
	st := domain.SourceType("test_indicator_dberr_src")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "val1", Label: "Value 1", SourceField: "val1", ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
		},
	})
	defer dynReg.Unregister(st)

	obsRepo := newStubObsRepo()
	obsRepo.getErr = fmt.Errorf("database error")

	svc := NewIndicatorService(nil, obsRepo, nil, dynReg)

	snapshots, err := svc.GetGlobalIndicators(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(snapshots) != 0 {
		t.Fatalf("expected 0 snapshots when db errors, got %d", len(snapshots))
	}
}

package realtime

import (
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

func TestBuildIndicatorUpdate_WithSpec(t *testing.T) {
	// Register test indicator layer.
	dynReg := domain.NewDynamicSourceRegistry() //nolint:staticcheck // TODO: migrate to constructor injection
	lt := domain.LayerType("test_ws_indicator")
	st := domain.SourceType("test_ws_indicator_source")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	dynReg.SetIndicatorSpec(lt, &domain.IndicatorSpec{
		ComputeSummary: domain.DefaultComputeSummary(),
		Values: []domain.IndicatorValueSpec{
			{Key: "r_scale", Label: "Radio Blackout", SourceField: "r_scale", MaxLevel: 5, ComputeLevel: domain.DefaultComputeLevel(nil, 5)},
			{Key: "kp_index", Label: "Kp Index", SourceField: "kp", MaxLevel: 9, LevelThresholds: []float64{0, 2, 4, 5, 7, 9}, ComputeLevel: domain.DefaultComputeLevel([]float64{0, 2, 4, 5, 7, 9}, 9)},
		},
	})
	defer dynReg.Unregister(st)

	obs := &domain.Observation{
		Timestamp: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
		Metadata: map[string]string{
			"r_scale": "2",
			"kp":      "3.33",
		},
	}

	result := buildIndicatorUpdate(dynReg, "test_ws_indicator", obs)
	if result == nil {
		t.Fatal("expected non-nil indicator update")
	}

	if result.TimestampMs != obs.Timestamp.UnixMilli() {
		t.Errorf("expected timestamp_ms %d, got %d", obs.Timestamp.UnixMilli(), result.TimestampMs)
	}

	if len(result.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(result.Values))
	}

	// Check r_scale: value "2" with no thresholds -> level 2.
	rVal := findWSIndicatorValue(result.Values, "r_scale")
	if rVal == nil {
		t.Fatal("expected r_scale value")
	}
	if rVal.Value != "2" {
		t.Errorf("expected r_scale value '2', got %q", rVal.Value)
	}
	if rVal.Level != 2 {
		t.Errorf("expected r_scale level 2, got %d", rVal.Level)
	}
	if rVal.Label != "Radio Blackout" {
		t.Errorf("expected r_scale label 'Radio Blackout', got %q", rVal.Label)
	}

	// Check kp_index: value "3.33" with thresholds [0,2,4,5,7,9] -> level 1.
	kpVal := findWSIndicatorValue(result.Values, "kp_index")
	if kpVal == nil {
		t.Fatal("expected kp_index value")
	}
	if kpVal.Level != 1 {
		t.Errorf("expected kp_index level 1, got %d", kpVal.Level)
	}

	// Overall level should be max(2, 1) = 2.
	if result.OverallLevel != 2 {
		t.Errorf("expected overall_level 2, got %d", result.OverallLevel)
	}
	if result.Summary != "Moderate" {
		t.Errorf("expected summary 'Moderate', got %q", result.Summary)
	}
}

func TestBuildIndicatorUpdate_NilObs(t *testing.T) {
	result := buildIndicatorUpdate(domain.NewDynamicSourceRegistry(), "test_ws_indicator", nil)
	if result != nil {
		t.Errorf("expected nil for nil observation, got %+v", result)
	}
}

func TestIsIndicatorLayer(t *testing.T) {
	dynReg := domain.NewDynamicSourceRegistry()
	lt := domain.LayerType("test_is_indicator")
	st := domain.SourceType("test_is_indicator_source")
	dynReg.Register(st, lt)
	dynReg.SetRenderingMode(lt, "indicator")
	defer dynReg.Unregister(st)

	if !isIndicatorLayer(dynReg, "test_is_indicator") {
		t.Error("expected isIndicatorLayer to return true")
	}
	if isIndicatorLayer(dynReg, "nonexistent_layer") {
		t.Error("expected isIndicatorLayer to return false for nonexistent")
	}
}

func TestDefaultComputeLevel(t *testing.T) {
	tests := []struct {
		name       string
		value      string
		thresholds []float64
		maxLevel   int
		want       int32
	}{
		{"no_thresholds_0", "0", nil, 5, 0},
		{"no_thresholds_3", "3", nil, 5, 3},
		{"no_thresholds_clamped", "10", nil, 5, 5},
		{"no_thresholds_max9", "8", nil, 9, 8},
		{"threshold_below", "1.5", []float64{0, 2, 4, 5, 7, 9}, 9, 0},
		{"threshold_1", "3.33", []float64{0, 2, 4, 5, 7, 9}, 9, 1},
		{"threshold_exact", "4.0", []float64{0, 2, 4, 5, 7, 9}, 9, 2},
		{"threshold_high", "9.0", []float64{0, 2, 4, 5, 7, 9}, 9, 5},
		{"invalid_value", "abc", []float64{0, 2, 4}, 5, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := domain.DefaultComputeLevel(tt.thresholds, tt.maxLevel)
			got := fn(tt.value, "0")
			if got != tt.want {
				t.Errorf("DefaultComputeLevel(%v, %d)(%q) = %d, want %d", tt.thresholds, tt.maxLevel, tt.value, got, tt.want)
			}
		})
	}
}

// findWSIndicatorValue finds an IndicatorValueWS by key.
func findWSIndicatorValue(values []IndicatorValueWS, key string) *IndicatorValueWS {
	for i := range values {
		if values[i].Key == key {
			return &values[i]
		}
	}
	return nil
}

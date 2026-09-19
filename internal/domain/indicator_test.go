package domain

import (
	"testing"
	"time"
)

func TestBuildIndicatorSnapshot_NilInputs(t *testing.T) {
	tests := []struct {
		name         string
		observations []*Observation
		spec         *IndicatorSpec
	}{
		{"nil observations", nil, &IndicatorSpec{}},
		{"empty observations", []*Observation{}, &IndicatorSpec{}},
		{"nil spec", []*Observation{{ID: "obs-1"}}, nil},
		{"both nil", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildIndicatorSnapshot("layer1", "Layer 1", tt.observations, tt.spec)
			if result != nil {
				t.Errorf("expected nil result, got %+v", result)
			}
		})
	}
}

func TestBuildIndicatorSnapshot_SingleObservation(t *testing.T) {
	ts := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	obs := []*Observation{
		{
			ID:         "obs-1",
			EntityID:   "entity-1",
			Timestamp:  ts,
			SourceType: "test_layer",
			Metadata: map[string]string{
				"temperature": "25.5",
				"humidity":    "60",
			},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{
				Key:          "temp",
				Label:        "Temperature",
				SourceField:  "temperature",
				Unit:         "C",
				ComputeLevel: DefaultComputeLevel(nil, 5),
			},
		},
	}

	result := BuildIndicatorSnapshot("test_layer", "Test Layer", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.LayerID != "test_layer" {
		t.Errorf("LayerID = %q, want %q", result.LayerID, "test_layer")
	}
	if result.LayerName != "Test Layer" {
		t.Errorf("LayerName = %q, want %q", result.LayerName, "Test Layer")
	}
	if result.TimestampMs != ts.UnixMilli() {
		t.Errorf("TimestampMs = %d, want %d", result.TimestampMs, ts.UnixMilli())
	}
	if len(result.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(result.Values))
	}
	v := result.Values[0]
	if v.Key != "temp" {
		t.Errorf("Key = %q, want %q", v.Key, "temp")
	}
	if v.Value != "25.5" {
		t.Errorf("Value = %q, want %q", v.Value, "25.5")
	}
	if v.Label != "Temperature" {
		t.Errorf("Label = %q, want %q", v.Label, "Temperature")
	}
	if v.Unit != "C" {
		t.Errorf("Unit = %q, want %q", v.Unit, "C")
	}
}

func TestBuildIndicatorSnapshot_MergesMultipleObservations(t *testing.T) {
	ts1 := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	ts2 := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC) // later timestamp

	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: ts1,
			Metadata:  map[string]string{"r_scale": "2"},
		},
		{
			ID:        "obs-2",
			Timestamp: ts2,
			Metadata:  map[string]string{"kp": "5.5"},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{Key: "r_scale", Label: "Radio Blackout", SourceField: "r_scale", ComputeLevel: DefaultComputeLevel(nil, 5)},
			{Key: "kp_index", Label: "Kp Index", SourceField: "kp", LevelThresholds: []float64{0, 2, 4, 5, 7, 9}, ComputeLevel: DefaultComputeLevel([]float64{0, 2, 4, 5, 7, 9}, 9)},
		},
	}

	result := BuildIndicatorSnapshot("space_weather", "Space Weather", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should use the latest timestamp.
	if result.TimestampMs != ts2.UnixMilli() {
		t.Errorf("TimestampMs = %d, want %d (latest)", result.TimestampMs, ts2.UnixMilli())
	}

	// Should have merged metadata from both observations.
	if len(result.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(result.Values))
	}

	// r_scale value "2" -> level 2 (no thresholds, direct).
	rVal := findValue(result.Values, "r_scale")
	if rVal == nil {
		t.Fatal("expected r_scale value")
	}
	if rVal.Value != "2" {
		t.Errorf("r_scale Value = %q, want %q", rVal.Value, "2")
	}
	if rVal.Level != 2 {
		t.Errorf("r_scale Level = %d, want 2", rVal.Level)
	}

	// kp value "5.5" with thresholds [0,2,4,5,7,9] -> level 3 (>= 5, < 7).
	kpVal := findValue(result.Values, "kp_index")
	if kpVal == nil {
		t.Fatal("expected kp_index value")
	}
	if kpVal.Value != "5.5" {
		t.Errorf("kp_index Value = %q, want %q", kpVal.Value, "5.5")
	}
	if kpVal.Level != 3 {
		t.Errorf("kp_index Level = %d, want 3", kpVal.Level)
	}
}

func TestBuildIndicatorSnapshot_MissingField_DefaultsToZero(t *testing.T) {
	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: time.Now(),
			Metadata:  map[string]string{"other_field": "value"},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{Key: "missing", Label: "Missing", SourceField: "nonexistent", ComputeLevel: DefaultComputeLevel(nil, 5)},
		},
	}

	result := BuildIndicatorSnapshot("test", "Test", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Values[0].Value != "0" {
		t.Errorf("Value = %q, want %q for missing field", result.Values[0].Value, "0")
	}
	if result.Values[0].Level != 0 {
		t.Errorf("Level = %d, want 0 for missing field", result.Values[0].Level)
	}
}

func TestBuildIndicatorSnapshot_OverallLevel(t *testing.T) {
	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: time.Now(),
			Metadata:  map[string]string{"a": "1", "b": "4"},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{Key: "a", Label: "A", SourceField: "a", ComputeLevel: DefaultComputeLevel(nil, 5)},
			{Key: "b", Label: "B", SourceField: "b", ComputeLevel: DefaultComputeLevel(nil, 5)},
		},
	}

	result := BuildIndicatorSnapshot("test", "Test", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Overall level should be the max of all value levels.
	if result.OverallLevel != 4 {
		t.Errorf("OverallLevel = %d, want 4", result.OverallLevel)
	}
	if result.Summary != "Severe" {
		t.Errorf("Summary = %q, want %q", result.Summary, "Severe")
	}
}

func TestBuildIndicatorSnapshot_SummaryLevels(t *testing.T) {
	tests := []struct {
		level       int32
		wantSummary string
	}{
		{0, "Quiet"},
		{1, "Minor Activity"},
		{2, "Moderate"},
		{3, "Strong"},
		{4, "Severe"},
		{5, "Extreme"},
		{10, "Extreme"},
	}

	for _, tt := range tests {
		obs := []*Observation{
			{
				ID:        "obs-1",
				Timestamp: time.Now(),
				Metadata:  map[string]string{"val": "0"},
			},
		}

		levelVal := tt.level
		spec := &IndicatorSpec{
			ComputeSummary: DefaultComputeSummary(),
			Values: []IndicatorValueSpec{
				{
					Key:         "val",
					Label:       "Val",
					SourceField: "val",
					ComputeLevel: func(_ string, _ string) int32 {
						return levelVal
					},
				},
			},
		}

		result := BuildIndicatorSnapshot("test", "Test", obs, spec)
		if result == nil {
			t.Fatalf("level %d: expected non-nil result", tt.level)
		}
		if result.Summary != tt.wantSummary {
			t.Errorf("level %d: Summary = %q, want %q", tt.level, result.Summary, tt.wantSummary)
		}
	}
}

func TestBuildIndicatorSnapshot_MetadataOverwrite(t *testing.T) {
	// When two observations have the same metadata key, the later one wins.
	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Metadata:  map[string]string{"key": "1"},
		},
		{
			ID:        "obs-2",
			Timestamp: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			Metadata:  map[string]string{"key": "3"},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{Key: "k", Label: "Key", SourceField: "key", ComputeLevel: DefaultComputeLevel(nil, 5)},
		},
	}

	result := BuildIndicatorSnapshot("test", "Test", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// The second observation's value should overwrite the first.
	if result.Values[0].Value != "3" {
		t.Errorf("Value = %q, want %q (last observation wins)", result.Values[0].Value, "3")
	}
}

func TestBuildIndicatorSnapshot_ExtendedFields(t *testing.T) {
	ts := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: ts,
			Metadata: map[string]string{
				"vix_value":      "24.31",
				"vix_change_pct": "8.2",
			},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{
				Key:               "vix",
				Label:             "VIX",
				SourceField:       "vix_value",
				Unit:              "",
				ChangeSourceField: "vix_change_pct",
				Format:            "number",
				Precision:         2,
				Prefix:            "",
				ComputeLevel:      DefaultComputeLevel([]float64{12, 18, 25, 35, 45}, 5),
			},
		},
	}

	result := BuildIndicatorSnapshot("markets", "Markets", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	v := result.Values[0]
	if v.ChangePct != 8.2 {
		t.Errorf("ChangePct = %f, want 8.2", v.ChangePct)
	}
	if v.ChangeAbs < 1.99 || v.ChangeAbs > 2.0 {
		t.Errorf("ChangeAbs = %f, want ~1.99", v.ChangeAbs)
	}
	if v.Format != "number" {
		t.Errorf("Format = %q, want %q", v.Format, "number")
	}
	if v.Precision != 2 {
		t.Errorf("Precision = %d, want 2", v.Precision)
	}
}

func TestBuildIndicatorSnapshot_ExtendedFields_Defaults(t *testing.T) {
	obs := []*Observation{
		{
			ID:        "obs-1",
			Timestamp: time.Now(),
			Metadata:  map[string]string{"r_scale": "2"},
		},
	}

	spec := &IndicatorSpec{
		ComputeSummary: DefaultComputeSummary(),
		Values: []IndicatorValueSpec{
			{
				Key:          "r_scale",
				Label:        "Radio Blackout",
				SourceField:  "r_scale",
				ComputeLevel: DefaultComputeLevel(nil, 5),
			},
		},
	}

	result := BuildIndicatorSnapshot("space_weather", "Space Weather", obs, spec)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	v := result.Values[0]
	if v.ChangePct != 0 {
		t.Errorf("ChangePct = %f, want 0", v.ChangePct)
	}
	if v.ChangeAbs != 0 {
		t.Errorf("ChangeAbs = %f, want 0", v.ChangeAbs)
	}
	if v.Format != "" {
		t.Errorf("Format = %q, want empty", v.Format)
	}
	if v.Precision != 0 {
		t.Errorf("Precision = %d, want 0", v.Precision)
	}
	if v.Prefix != "" {
		t.Errorf("Prefix = %q, want empty", v.Prefix)
	}
}

// findValue is a test helper to find an IndicatorValue by key.
func findValue(values []IndicatorValue, key string) *IndicatorValue {
	for i := range values {
		if values[i].Key == key {
			return &values[i]
		}
	}
	return nil
}

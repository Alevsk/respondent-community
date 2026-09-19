package declarative

import (
	"strings"
	"testing"
)

// TestIndicatorSpecToDomain_Nil verifies nil spec returns nil without error.
func TestIndicatorSpecToDomain_Nil(t *testing.T) {
	result, err := indicatorSpecToDomain(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for nil spec, got %v", result)
	}
}

// TestIndicatorSpecToDomain_NoExpressions verifies values with no CEL expressions
// use default compute functions.
func TestIndicatorSpecToDomain_NoExpressions(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key:             "kp",
				Label:           "Kp Index",
				SourceField:     "kp_value",
				MaxLevel:        9,
				LevelThresholds: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8},
			},
		},
	}
	result, err := indicatorSpecToDomain(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(result.Values) != 1 {
		t.Fatalf("expected 1 value, got %d", len(result.Values))
	}
	if result.Values[0].ComputeLevel == nil {
		t.Error("expected non-nil ComputeLevel")
	}
	if result.ComputeSummary == nil {
		t.Error("expected non-nil ComputeSummary")
	}
}

// TestIndicatorSpecToDomain_WithLevelExpr verifies a valid LevelExpr is compiled.
func TestIndicatorSpecToDomain_WithLevelExpr(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key:       "level",
				LevelExpr: `int(double(value))`,
			},
		},
	}
	result, err := indicatorSpecToDomain(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// Verify the compiled function works.
	level := result.Values[0].ComputeLevel("5", "0")
	if level != 5 {
		t.Errorf("ComputeLevel('5') = %d, want 5", level)
	}
}

// TestIndicatorSpecToDomain_InvalidLevelExpr verifies compile error is returned.
func TestIndicatorSpecToDomain_InvalidLevelExpr(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key:       "bad",
				LevelExpr: `invalid syntax !!! @@@`,
			},
		},
	}
	_, err := indicatorSpecToDomain(spec)
	if err == nil {
		t.Fatal("expected error for invalid level_expr, got nil")
	}
	if !strings.Contains(err.Error(), "bad") {
		t.Errorf("expected value key 'bad' in error, got: %v", err)
	}
}

// TestIndicatorSpecToDomain_WithSummaryExpr verifies SummaryExpr is compiled and works.
func TestIndicatorSpecToDomain_WithSummaryExpr(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key: "value",
			},
		},
		SummaryExpr: `level >= 3 ? "High" : "Low"`,
	}
	result, err := indicatorSpecToDomain(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ComputeSummary == nil {
		t.Fatal("expected non-nil ComputeSummary")
	}
	// Verify the compiled function works.
	summary := result.ComputeSummary(map[string]string{}, 5)
	if summary != "High" {
		t.Errorf("ComputeSummary(level=5) = %q, want 'High'", summary)
	}
	summary = result.ComputeSummary(map[string]string{}, 1)
	if summary != "Low" {
		t.Errorf("ComputeSummary(level=1) = %q, want 'Low'", summary)
	}
}

// TestIndicatorSpecToDomain_InvalidSummaryExpr verifies compile error is returned
// for an invalid summary_expr.
func TestIndicatorSpecToDomain_InvalidSummaryExpr(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key: "value",
			},
		},
		SummaryExpr: `invalid syntax !!!`,
	}
	_, err := indicatorSpecToDomain(spec)
	if err == nil {
		t.Fatal("expected error for invalid summary_expr, got nil")
	}
}

// TestIndicatorSpecToDomain_MultipleValues verifies multiple values are processed.
func TestIndicatorSpecToDomain_MultipleValues(t *testing.T) {
	spec := &IndicatorSpec{
		Values: []IndicatorValueSpec{
			{
				Key:   "v1",
				Label: "Value 1",
			},
			{
				Key:       "v2",
				Label:     "Value 2",
				LevelExpr: `int(double(value))`,
			},
		},
	}
	result, err := indicatorSpecToDomain(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Values) != 2 {
		t.Fatalf("expected 2 values, got %d", len(result.Values))
	}
}

package declarative

import (
	"testing"
)

func TestCompileIndicatorLevelExpr_InvalidCEL(t *testing.T) {
	_, err := compileIndicatorLevelExpr("!!! invalid @@@")
	if err == nil {
		t.Error("expected error for invalid indicator level CEL, got nil")
	}
}

func TestCompileIndicatorSummaryExpr_InvalidCEL(t *testing.T) {
	_, err := compileIndicatorSummaryExpr("!!! invalid @@@")
	if err == nil {
		t.Error("expected error for invalid indicator summary CEL, got nil")
	}
}

func TestCompileIndicatorLevelExpr_CompileError(t *testing.T) {
	_, err := compileIndicatorLevelExpr("!!! INVALID CEL !!!")
	if err == nil {
		t.Error("expected compile error for invalid level_expr, got nil")
	}
}

func TestCompileIndicatorLevelExpr_ValidExpr(t *testing.T) {
	fn, err := compileIndicatorLevelExpr("int(0)")
	if err != nil {
		t.Fatalf("compileIndicatorLevelExpr with valid expr: %v", err)
	}
	if fn == nil {
		t.Fatal("expected non-nil closure")
	}
	result := fn("any_value", "0")
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

func TestCompileIndicatorSummaryExpr_CompileError(t *testing.T) {
	_, err := compileIndicatorSummaryExpr("!!! INVALID CEL !!!")
	if err == nil {
		t.Error("expected compile error for invalid summary_expr, got nil")
	}
}

func TestCompileIndicatorSummaryExpr_ValidExpr(t *testing.T) {
	fn, err := compileIndicatorSummaryExpr(`"ok"`)
	if err != nil {
		t.Fatalf("compileIndicatorSummaryExpr with valid expr: %v", err)
	}
	if fn == nil {
		t.Fatal("expected non-nil closure")
	}
	result := fn(map[string]string{}, 0)
	if result != "ok" {
		t.Errorf("expected 'ok', got %q", result)
	}
}

func TestCompileIndicatorLevelExpr_DefaultReturn(t *testing.T) {
	// "value" evaluates to the input string - returns string, not int/float.
	fn, err := compileIndicatorLevelExpr(`value`)
	if err != nil {
		t.Fatalf("compileIndicatorLevelExpr: %v", err)
	}
	// A string result hits the default branch → returns 0.
	result := fn("some_string_value", "0")
	if result != 0 {
		t.Errorf("expected 0 for string result in default branch, got %d", result)
	}
}

func TestCompileIndicatorSummaryExpr_NonStringResult(t *testing.T) {
	// Expression that returns an integer - triggers the !ok branch.
	fn, err := compileIndicatorSummaryExpr("int(0)")
	if err != nil {
		t.Fatalf("compileIndicatorSummaryExpr: %v", err)
	}
	result := fn(map[string]string{}, 0)
	if result != "Unknown" {
		t.Errorf("expected 'Unknown' for non-string result, got %q", result)
	}
}

func TestCompileIndicatorLevelExpr_Float64Result(t *testing.T) {
	// math.ceil returns a double (float64) in CEL.
	fn, err := compileIndicatorLevelExpr("math.ceil(double(2))")
	if err != nil {
		t.Fatalf("compileIndicatorLevelExpr: %v", err)
	}
	result := fn("unused", "0")
	if result != 2 {
		t.Errorf("expected 2, got %d", result)
	}
}

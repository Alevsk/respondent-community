package declarative

import (
	"strings"
	"testing"
)

// TestCompileIndicatorLevelExpr_Float64Return verifies the float64 return path
// in the compiled closure. CEL returns a double (float64) for expressions like
// double(value) which triggers the float64 case in the type switch.
func TestCompileIndicatorLevelExpr_Float64Return(t *testing.T) {
	// This expression returns a double (float64) from CEL, exercising the
	// float64 branch in the compileIndicatorLevelExpr closure.
	fn, err := compileIndicatorLevelExpr(`double(value) * 1.0`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	got := fn("3", "0")
	if got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
}

// TestCompileIndicatorLevelExpr_DefaultZeroReturn verifies the default 0 branch
// in the compiled closure when CEL returns a non-numeric type.
func TestCompileIndicatorLevelExpr_DefaultZeroReturn(t *testing.T) {
	// Construct an expression that returns a string type -- CEL will return
	// a string ref.Val, which hits the default case in the type switch.
	fn, err := compileIndicatorLevelExpr(`string(int(double(value)))`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	// The closure returns 0 for non-numeric CEL return types.
	got := fn("5", "0")
	if got != 0 {
		t.Errorf("expected 0 for string CEL return, got %d", got)
	}
}

// TestCompileIndicatorLevelExpr_ExceedsLimit verifies the expression size limit.
func TestCompileIndicatorLevelExpr_ExceedsLimit(t *testing.T) {
	expr := strings.Repeat("x", maxExpressionSize+1)
	_, err := compileIndicatorLevelExpr(expr)
	if err == nil {
		t.Fatal("expected error for oversized expression, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

// TestCompileIndicatorSummaryExpr_ErrorReturn verifies that eval errors in the closure
// return "Unknown".
func TestCompileIndicatorSummaryExpr_ErrorReturn(t *testing.T) {
	// This expression accesses metadata["key"] which will fail when metadata is nil
	// in CEL because you can't index a nil map -- results in CEL eval error.
	fn, err := compileIndicatorSummaryExpr(`metadata["missing_key"]`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	// Pass nil metadata -- CEL eval should fail, returning "Unknown".
	got := fn(nil, 0)
	if got != "Unknown" {
		t.Errorf("expected 'Unknown' for eval error, got %q", got)
	}
}

// TestCompileIndicatorSummaryExpr_NonStringReturn verifies the non-string
// return path returns "Unknown".
func TestCompileIndicatorSummaryExpr_NonStringReturn(t *testing.T) {
	// This expression returns an integer, not a string, hitting the !ok path
	// in the type assertion in the compileIndicatorSummaryExpr closure.
	fn, err := compileIndicatorSummaryExpr(`level + 1`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	got := fn(map[string]string{}, 3)
	if got != "Unknown" {
		t.Errorf("expected 'Unknown' for non-string return, got %q", got)
	}
}

// TestCompileIndicatorSummaryExpr_ExceedsLimit verifies the expression size limit.
func TestCompileIndicatorSummaryExpr_ExceedsLimit(t *testing.T) {
	expr := strings.Repeat("y", maxExpressionSize+1)
	_, err := compileIndicatorSummaryExpr(expr)
	if err == nil {
		t.Fatal("expected error for oversized expression, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

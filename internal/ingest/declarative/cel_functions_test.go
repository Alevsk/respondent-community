package declarative

import (
	"testing"
	"time"
)

// mustNewCompilerForFuncs is an alias for test helpers.
func mustCELCompiler(t *testing.T) *CELCompiler {
	t.Helper()
	c, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// cel_env.go – unix_ms int64 branch (line 117)
// ---------------------------------------------------------------------------

// TestCELUnixMs_Int64 covers the int64 branch in the unix_ms function binding.
func TestCELUnixMs_Int64(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("unix_ms(int(record.ts))")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": float64(1700000000000)}}
	got, err := evalTimestamp(prg, activation)
	if err != nil {
		t.Fatalf("evalTimestamp: %v", err)
	}
	expected := time.UnixMilli(1700000000000)
	if !got.Equal(expected) {
		t.Errorf("unix_ms int64: got %v, want %v", got, expected)
	}
}

// TestCELUnixMs_DefaultBranch covers the default/error branch in unix_ms.
// Passing a non-numeric value triggers the default case.
func TestCELUnixMs_DefaultBranch(t *testing.T) {
	c := mustCELCompiler(t)
	// unix_ms expects a number; passing a string should trigger the default error path.
	prg, err := c.CompileExpression("unix_ms(record.ts)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": "not-a-number"}}
	// This should return an error because the binding returns types.NewErr.
	_, err = evalTimestamp(prg, activation)
	if err == nil {
		t.Error("expected error for unix_ms with string argument, got nil")
	}
}

// ---------------------------------------------------------------------------
// cel_env.go – unix_s int64 branch (line 135)
// ---------------------------------------------------------------------------

// TestCELUnixS_Int64 covers the int64 branch in the unix_s function binding.
func TestCELUnixS_Int64(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("unix_s(int(record.ts))")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": float64(1700000000)}}
	got, err := evalTimestamp(prg, activation)
	if err != nil {
		t.Fatalf("evalTimestamp: %v", err)
	}
	expected := time.Unix(1700000000, 0)
	if !got.Equal(expected) {
		t.Errorf("unix_s int64: got %v, want %v", got, expected)
	}
}

// TestCELUnixS_DefaultBranch covers the default/error branch in unix_s.
func TestCELUnixS_DefaultBranch(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("unix_s(record.ts)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": "not-a-number"}}
	_, err = evalTimestamp(prg, activation)
	if err == nil {
		t.Error("expected error for unix_s with string argument, got nil")
	}
}

// ---------------------------------------------------------------------------
// cel_env.go – unix_s float64 branch (line 133)
// ---------------------------------------------------------------------------

// TestCELUnixS_Float64 covers the float64 branch in the unix_s function binding.
func TestCELUnixS_Float64(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("unix_s(record.ts)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": float64(1700000000)}}
	got, err := evalTimestamp(prg, activation)
	if err != nil {
		t.Fatalf("evalTimestamp: %v", err)
	}
	expected := time.Unix(1700000000, 0)
	if !got.Equal(expected) {
		t.Errorf("unix_s float64: got %v, want %v", got, expected)
	}
}

// ---------------------------------------------------------------------------
// cel_env.go – parse_rfc3339 error branch (line 155)
// ---------------------------------------------------------------------------

// TestCELParseRFC3339_BadFormat covers the parse error branch in parse_rfc3339.
func TestCELParseRFC3339_BadFormat(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("parse_rfc3339(record.ts)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": "not-a-valid-rfc3339"}}
	_, err = evalTimestamp(prg, activation)
	if err == nil {
		t.Error("expected error for parse_rfc3339 with bad format, got nil")
	}
}

// ---------------------------------------------------------------------------
// cel_env.go – CompileExpression program error path (line 408)
// ---------------------------------------------------------------------------

// Note: The cel.Program error path at line 408 is not practically reachable
// since CEL programs only fail to create with invalid ASTs, which would have
// failed at Compile time. This is effectively dead code.

// ---------------------------------------------------------------------------
// cel_env.go – CompileExpressionWithLookups program error path
// Test that the normal compile path works with lookups
// ---------------------------------------------------------------------------

// TestCompileExpressionWithLookups_Success exercises the program creation path.
func TestCompileExpressionWithLookups_EmptyTables(t *testing.T) {
	c := mustCELCompiler(t)
	// Empty tables - should succeed
	prg, err := c.CompileExpressionWithLookups("record.name", map[string]*LookupTable{})
	if err != nil {
		t.Fatalf("CompileExpressionWithLookups: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"name": "test"}}
	got, err := evalString(prg, activation, 100)
	if err != nil {
		t.Fatalf("evalString: %v", err)
	}
	if got != "test" {
		t.Errorf("got %q, want %q", got, "test")
	}
}

// ---------------------------------------------------------------------------
// eval.go – evalString default ConvertToType branch (line 32)
// ---------------------------------------------------------------------------

// TestEvalString_DefaultConvertToType exercises the ConvertToType path for
// values that aren't string/int64/float64/bool but can be converted to string.
// CEL Timestamp types trigger this path.
func TestEvalString_TimestampConvertsToString(t *testing.T) {
	c := mustCELCompiler(t)
	// now() returns a Timestamp which should be convertible to string.
	prg, err := c.CompileExpression("string(now())")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	got, err := evalString(prg, map[string]interface{}{"record": map[string]interface{}{}}, 1024)
	if err != nil {
		t.Fatalf("evalString with Timestamp: %v", err)
	}
	if got == "" {
		t.Error("expected non-empty string from Timestamp.ConvertToType, got empty")
	}
}

// ---------------------------------------------------------------------------
// eval.go – evalFloat default branch (line 65) and evalTimestamp default (line 107)
// ---------------------------------------------------------------------------

// TestEvalFloat_DefaultBranch covers the default error case in evalFloat
// when the value is neither float64, int64 nor string.
func TestEvalFloat_DefaultBranch(t *testing.T) {
	c := mustCELCompiler(t)
	// now() returns a Timestamp - not a numeric type.
	prg, err := c.CompileExpression("now()")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{}}
	_, err = evalFloat(prg, activation)
	if err == nil {
		t.Error("expected error for evalFloat with Timestamp value, got nil")
	}
}

// TestEvalTimestamp_DefaultBranch covers the default error case in evalTimestamp
// when the value is not time.Time, string, int64, or float64.
func TestEvalTimestamp_DefaultBranch(t *testing.T) {
	c := mustCELCompiler(t)
	// An expression that returns bool - not a timestamp type.
	prg, err := c.CompileExpression("record.flag")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"flag": true}}
	_, err = evalTimestamp(prg, activation)
	if err == nil {
		t.Error("expected error for evalTimestamp with bool value, got nil")
	}
}

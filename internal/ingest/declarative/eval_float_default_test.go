package declarative

import (
	"strings"
	"testing"
)

// TestEvalFloat_DefaultTypeError covers the default case in evalFloat's type switch
// where the CEL result is neither float64, int64, nor string.
// We use a boolean expression which hits the default (error) branch.
func TestEvalFloat_DefaultTypeError(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// A boolean expression results in a bool return type, which is neither
	// float64, int64, nor string in evalFloat's type switch.
	prg, err := compiler.CompileExpression("record.flag")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{"flag": true}}
	_, err = evalFloat(prg, activation)
	if err == nil {
		t.Fatal("expected error for bool result in evalFloat, got nil")
	}
	if !strings.Contains(err.Error(), "expected numeric type") {
		t.Errorf("expected 'expected numeric type' error, got: %v", err)
	}
}

// TestEvalString_ConvertToType covers the default/ConvertToType branch in evalString.
// This is hit when the CEL result is a timestamp, which is not string/int64/float64/bool
// but can be converted to a string via CEL's ConvertToType.
func TestEvalString_ConvertToType(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// now() returns a CEL Timestamp type (time.Time under the hood).
	// evalString's type switch will hit the default case and call ConvertToType.
	prg, err := compiler.CompileExpression("now()")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{}}
	got, err := evalString(prg, activation, 0)
	if err != nil {
		t.Fatalf("evalString with timestamp: %v", err)
	}
	// The result should be a non-empty string (ISO 8601 format from CEL timestamp).
	if got == "" {
		t.Error("expected non-empty string from ConvertToType of timestamp")
	}
}

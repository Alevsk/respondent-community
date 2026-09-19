package declarative

import (
	"strings"
	"testing"
	"time"
)

// TestSourceClock_NilClock verifies that a nil clock in CompiledSource returns realClock.
func TestSourceClock_NilClock(t *testing.T) {
	cs := &CompiledSource{clock: nil}
	clk := cs.SourceClock()
	if clk == nil {
		t.Fatal("expected non-nil clock from SourceClock with nil internal clock")
	}
	// realClock.Now() should return a non-zero time close to now.
	now := clk.Now()
	if now.IsZero() {
		t.Error("expected non-zero time from realClock")
	}
	// Verify it's within a second of actual time.
	if diff := time.Since(now); diff < 0 || diff > time.Second {
		t.Errorf("realClock.Now() returned time too far from now: diff=%v", diff)
	}
}

// TestSourceClock_CustomClock verifies that a non-nil clock is returned as-is.
func TestSourceClock_CustomClock(t *testing.T) {
	fixedTime := time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)
	mockClk := &mockClock{fixedTime}
	cs := &CompiledSource{clock: mockClk}
	clk := cs.SourceClock()
	if clk != mockClk {
		t.Error("expected the same custom clock to be returned")
	}
	if clk.Now() != fixedTime {
		t.Errorf("expected fixed time %v, got %v", fixedTime, clk.Now())
	}
}

// mockClock is a simple test clock implementation.
type mockClock struct {
	t time.Time
}

func (m *mockClock) Now() time.Time {
	return m.t
}

// TestCompileStopWhen_ValidExpression verifies a valid stop_when expression compiles.
func TestCompileStopWhen_ValidExpression(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileStopWhen(`size(records) == 0`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if prg == nil {
		t.Fatal("expected non-nil program")
	}
}

// TestCompileStopWhen_CompileError verifies that an invalid expression returns an error.
func TestCompileStopWhen_CompileError(t *testing.T) {
	c := mustNewCompiler(t)
	_, err := c.CompileStopWhen(`invalid syntax !!! @@@`)
	if err == nil {
		t.Fatal("expected compile error, got nil")
	}
	if !strings.Contains(err.Error(), "stop_when") {
		t.Errorf("expected 'stop_when' in error message, got: %v", err)
	}
}

// TestCompileStopWhen_ExceedsLimit verifies the expression size limit is enforced.
func TestCompileStopWhen_ExceedsLimit(t *testing.T) {
	c := mustNewCompiler(t)
	expr := strings.Repeat("z", maxExpressionSize+1)
	_, err := c.CompileStopWhen(expr)
	if err == nil {
		t.Fatal("expected error for oversized stop_when expression, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

// TestCompileStopWhen_EvalWithRecords verifies that the compiled program can evaluate.
func TestCompileStopWhen_EvalWithRecords(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileStopWhen(`size(records) < 5`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}

	// Evaluate with a list of records.
	records := []interface{}{
		map[string]interface{}{"id": "1"},
		map[string]interface{}{"id": "2"},
	}
	out, _, evalErr := prg.Eval(map[string]interface{}{"records": records})
	if evalErr != nil {
		t.Fatalf("eval error: %v", evalErr)
	}
	if out.Value() != true {
		t.Errorf("expected true for size(records) < 5 with 2 records, got %v", out.Value())
	}
}

// TestNewCELCompilerWithClock_NilClock verifies that nil clock defaults to realClock.
func TestNewCELCompilerWithClock_NilClock(t *testing.T) {
	c, err := NewCELCompilerWithClock(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil compiler")
	}
	// Verify it can compile an expression (realClock is used internally).
	prg, err := c.CompileExpression(`now()`)
	if err != nil {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if prg == nil {
		t.Fatal("expected non-nil program")
	}
}

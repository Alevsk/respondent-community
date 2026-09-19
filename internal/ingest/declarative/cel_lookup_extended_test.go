package declarative

import (
	"strings"
	"testing"
)

// TestCompileExpressionWithLookups_ExceedsLimit verifies the expression size limit.
func TestCompileExpressionWithLookups_ExceedsLimit(t *testing.T) {
	c := mustNewCompiler(t)
	expr := strings.Repeat("z", maxExpressionSize+1)
	_, err := c.CompileExpressionWithLookups(expr, nil)
	if err == nil {
		t.Fatal("expected error for oversized expression, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

// TestCompileExpressionWithLookups_CompileError verifies that a compile error is returned
// for an invalid expression.
func TestCompileExpressionWithLookups_CompileError(t *testing.T) {
	c := mustNewCompiler(t)
	_, err := c.CompileExpressionWithLookups(`invalid syntax !!! @@@`, nil)
	if err == nil {
		t.Fatal("expected compile error, got nil")
	}
}

// TestCompileExpression_ExceedsLimit verifies the expression size limit in CompileExpression.
func TestCompileExpression_ExceedsLimit(t *testing.T) {
	c := mustNewCompiler(t)
	expr := strings.Repeat("w", maxExpressionSize+1)
	_, err := c.CompileExpression(expr)
	if err == nil {
		t.Fatal("expected error for oversized expression in CompileExpression, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

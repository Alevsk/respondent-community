package declarative

import (
	"fmt"
	"sync"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
)

// Indicator-specific CEL environments. These are separate from the record-mapping
// CEL environment because indicator expressions operate on different variables:
//   - level_expr:   receives "value" (string) — the raw metadata value
//   - summary_expr: receives "level" (int) and "metadata" (map) — computed state

var (
	indicatorLevelEnv    *cel.Env
	indicatorLevelErr    error
	indicatorLevelOnce   sync.Once
	indicatorSummaryEnv  *cel.Env
	indicatorSummaryErr  error
	indicatorSummaryOnce sync.Once
)

// getIndicatorLevelEnv returns a CEL environment for compiling level_expr expressions.
// The environment exposes a single "value" string variable (the raw metadata value).
func getIndicatorLevelEnv() (*cel.Env, error) {
	indicatorLevelOnce.Do(func() {
		indicatorLevelEnv, indicatorLevelErr = cel.NewEnv(
			cel.Variable("value", cel.StringType),
			cel.Variable("change_pct", cel.StringType),
			ext.Strings(),
			ext.Math(),
		)
	})
	return indicatorLevelEnv, indicatorLevelErr
}

// getIndicatorSummaryEnv returns a CEL environment for compiling summary_expr expressions.
// The environment exposes "level" (int) and "metadata" (map[string]string).
func getIndicatorSummaryEnv() (*cel.Env, error) {
	indicatorSummaryOnce.Do(func() {
		indicatorSummaryEnv, indicatorSummaryErr = cel.NewEnv(
			cel.Variable("level", cel.IntType),
			cel.Variable("metadata", cel.MapType(cel.StringType, cel.StringType)),
			ext.Strings(),
			ext.Math(),
		)
	})
	return indicatorSummaryEnv, indicatorSummaryErr
}

// compileIndicatorLevelExpr compiles a CEL level_expr into a closure.
// The closure accepts a raw value string and returns the computed level.
func compileIndicatorLevelExpr(expr string) (func(string, string) int32, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("level_expr exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	env, err := getIndicatorLevelEnv()
	if err != nil {
		return nil, fmt.Errorf("indicator level CEL env: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile level_expr %q: %w", expr, issues.Err())
	}

	prg, err := env.Program(ast, cel.CostLimit(celCostLimit))
	if err != nil {
		return nil, fmt.Errorf("program level_expr %q: %w", expr, err)
	}

	return func(rawVal string, changePct string) int32 {
		out, _, err := prg.Eval(map[string]interface{}{
			"value":      rawVal,
			"change_pct": changePct,
		})
		if err != nil {
			return 0
		}
		switch v := out.Value().(type) {
		case int64:
			return int32(v)
		case float64:
			return int32(v)
		default:
			return 0
		}
	}, nil
}

// compileIndicatorSummaryExpr compiles a CEL summary_expr into a closure.
// The closure accepts the merged metadata map and overall level, returning a summary string.
func compileIndicatorSummaryExpr(expr string) (func(map[string]string, int32) string, error) {
	if len(expr) > maxExpressionSize {
		return nil, fmt.Errorf("summary_expr exceeds %d byte limit (%d bytes)", maxExpressionSize, len(expr))
	}

	env, err := getIndicatorSummaryEnv()
	if err != nil {
		return nil, fmt.Errorf("indicator summary CEL env: %w", err)
	}

	ast, issues := env.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("compile summary_expr %q: %w", expr, issues.Err())
	}

	prg, err := env.Program(ast, cel.CostLimit(celCostLimit))
	if err != nil {
		return nil, fmt.Errorf("program summary_expr %q: %w", expr, err)
	}

	return func(metadata map[string]string, level int32) string {
		activation := map[string]interface{}{
			"level":    int64(level),
			"metadata": metadata,
		}
		out, _, err := prg.Eval(activation)
		if err != nil {
			return "Unknown"
		}
		s, ok := out.Value().(string)
		if !ok {
			return "Unknown"
		}
		return s
	}, nil
}

package declarative

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/cel-go/cel"
)

func mustNewCompiler(t *testing.T) *CELCompiler {
	t.Helper()
	c, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("failed to create CEL compiler: %v", err)
	}
	return c
}

func evalProgram(t *testing.T, prg cel.Program, record map[string]interface{}) (interface{}, error) {
	t.Helper()
	activation := map[string]interface{}{
		"record": record,
	}
	out, _, err := prg.Eval(activation)
	if err != nil {
		return nil, err
	}
	return out.Value(), nil
}

func TestCELCompile_ValidExpression(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name string
		expr string
	}{
		{name: "simple field access", expr: "record.id"},
		{name: "string concatenation", expr: `"prefix-" + record.id`},
		{name: "conditional", expr: `has(record.name) ? record.name : "default"`},
		{name: "numeric comparison", expr: "record.mag >= 2.0"},
		{name: "now function", expr: "now()"},
		{name: "unix_ms function", expr: "unix_ms(record.time)"},
		{name: "unix_s function", expr: "unix_s(record.ts)"},
		{name: "parse_rfc3339 function", expr: `parse_rfc3339("2024-01-01T00:00:00Z")`},
		{name: "parse_datetime function", expr: `parse_datetime("2024-06-15", "2006-01-02")`},
		{name: "parse_iso8601 function", expr: `parse_iso8601("2024-06-15T14:30:00")`},
		{name: "string function", expr: "string(record.value)"},
		{name: "has guard", expr: `has(record.x) ? record.x : "none"`},
		{name: "list membership", expr: `record.type in ["a", "b"]`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpression(tc.expr)
			if err != nil {
				t.Fatalf("expected no error compiling %q, got: %v", tc.expr, err)
			}
			if prg == nil {
				t.Fatal("expected non-nil program")
			}
		})
	}
}

func TestCELCompile_TypeError(t *testing.T) {
	c := mustNewCompiler(t)

	// Note: with DynType, `record.id + 42` won't fail at compile time because
	// record fields are dynamic. Instead test a clear static type error.
	_, err := c.CompileExpression(`"hello" + 42`)
	if err == nil {
		t.Fatal("expected compile error for string + int, got nil")
	}
	if !strings.Contains(err.Error(), "CEL compile error") {
		t.Errorf("expected CEL compile error, got: %v", err)
	}
}

func TestCELCompile_SyntaxError(t *testing.T) {
	c := mustNewCompiler(t)

	_, err := c.CompileExpression(`record.id ===`)
	if err == nil {
		t.Fatal("expected syntax error, got nil")
	}
	if !strings.Contains(err.Error(), "CEL compile error") {
		t.Errorf("expected CEL compile error, got: %v", err)
	}
}

func TestCELCompile_SizeLimit(t *testing.T) {
	c := mustNewCompiler(t)

	// Create expression > 4KB
	bigExpr := `"` + strings.Repeat("a", 5000) + `"`
	_, err := c.CompileExpression(bigExpr)
	if err == nil {
		t.Fatal("expected size limit error, got nil")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Errorf("expected byte limit error, got: %v", err)
	}
}

func TestCELEval_Filter(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("record.mag >= 2.0")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	tests := []struct {
		name     string
		record   map[string]interface{}
		expected bool
	}{
		{name: "passes filter", record: map[string]interface{}{"mag": 3.5}, expected: true},
		{name: "fails filter", record: map[string]interface{}{"mag": 0.5}, expected: false},
		{name: "exact boundary", record: map[string]interface{}{"mag": 2.0}, expected: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got, ok := val.(bool)
			if !ok {
				t.Fatalf("expected bool, got %T", val)
			}
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestCELEval_HasGuard(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`has(record.x) ? record.x : "default"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	tests := []struct {
		name     string
		record   map[string]interface{}
		expected string
	}{
		{name: "field present", record: map[string]interface{}{"x": "hello"}, expected: "hello"},
		{name: "field absent", record: map[string]interface{}{}, expected: "default"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got, ok := val.(string)
			if !ok {
				t.Fatalf("expected string, got %T (%v)", val, val)
			}
			if got != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestCELEval_StringConvert(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("string(record.mag)")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"mag": 3.5})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	got, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if got != "3.5" {
		t.Errorf("expected '3.5', got %q", got)
	}
}

func TestCELEval_UnixMs(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("unix_ms(record.time)")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// 1700000000000 ms = 2023-11-14T22:13:20Z
	val, err := evalProgram(t, prg, map[string]interface{}{"time": float64(1700000000000)})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	ts, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T (%v)", val, val)
	}

	expected := time.UnixMilli(1700000000000)
	if !ts.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ts)
	}
}

func TestCELEval_UnixS(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("unix_s(record.ts)")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"ts": float64(1700000000)})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	ts, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T (%v)", val, val)
	}

	expected := time.Unix(1700000000, 0)
	if !ts.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ts)
	}
}

func TestCELEval_ParseRFC3339(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("parse_rfc3339(record.ts)")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"ts": "2024-01-01T00:00:00Z"})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	ts, ok := val.(time.Time)
	if !ok {
		t.Fatalf("expected time.Time, got %T (%v)", val, val)
	}

	expected, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
	if !ts.Equal(expected) {
		t.Errorf("expected %v, got %v", expected, ts)
	}
}

func TestCELEval_ParseDatetime(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name     string
		expr     string
		record   map[string]interface{}
		expected time.Time
	}{
		{
			name:     "date and time with T separator",
			expr:     `parse_datetime(record.dt, "2006-01-02T15:04:05")`,
			record:   map[string]interface{}{"dt": "2024-06-15T14:30:00"},
			expected: time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "date and time with space separator",
			expr:     `parse_datetime(record.dt, "2006-01-02 15:04:05")`,
			record:   map[string]interface{}{"dt": "2024-06-15 14:30:00"},
			expected: time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC),
		},
		{
			name:     "US date format",
			expr:     `parse_datetime(record.dt, "01/02/2006")`,
			record:   map[string]interface{}{"dt": "06/15/2024"},
			expected: time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "FIRMS acq_date + acq_time concatenation",
			expr:     `parse_datetime(record.acq_date + " " + record.acq_time, "2006-01-02 1504")`,
			record:   map[string]interface{}{"acq_date": "2024-06-15", "acq_time": "1430"},
			expected: time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpression(tc.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			ts, ok := val.(time.Time)
			if !ok {
				t.Fatalf("expected time.Time, got %T (%v)", val, val)
			}

			if !ts.Equal(tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, ts)
			}
		})
	}
}

func TestCELEval_ParseDatetime_Error(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`parse_datetime(record.dt, "2006-01-02")`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	// Value that doesn't match the layout
	_, err = evalProgram(t, prg, map[string]interface{}{"dt": "not-a-date"})
	if err == nil {
		t.Fatal("expected runtime error for invalid date, got nil")
	}
}

func TestCELEval_ParseISO8601(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name     string
		expr     string
		record   map[string]interface{}
		expected time.Time
	}{
		{
			name:     "ISO 8601 without timezone (GDACS format)",
			expr:     `parse_iso8601(record.ts)`,
			record:   map[string]interface{}{"ts": "2026-03-11T07:55:18"},
			expected: time.Date(2026, 3, 11, 7, 55, 18, 0, time.UTC),
		},
		{
			name:     "ISO 8601 with Z suffix",
			expr:     `parse_iso8601(record.ts)`,
			record:   map[string]interface{}{"ts": "2024-01-01T00:00:00Z"},
			expected: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "ISO 8601 with positive offset",
			expr:     `parse_iso8601(record.ts)`,
			record:   map[string]interface{}{"ts": "2024-06-15T14:30:00+05:00"},
			expected: time.Date(2024, 6, 15, 14, 30, 0, 0, time.FixedZone("", 5*3600)),
		},
		{
			name:     "ISO 8601 with negative offset",
			expr:     `parse_iso8601(record.ts)`,
			record:   map[string]interface{}{"ts": "2024-06-15T14:30:00-05:00"},
			expected: time.Date(2024, 6, 15, 14, 30, 0, 0, time.FixedZone("", -5*3600)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpression(tc.expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			ts, ok := val.(time.Time)
			if !ok {
				t.Fatalf("expected time.Time, got %T (%v)", val, val)
			}

			if !ts.Equal(tc.expected) {
				t.Errorf("expected %v, got %v", tc.expected, ts)
			}
		})
	}
}

func TestCELEval_ParseISO8601_Error(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`parse_iso8601(record.ts)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	_, err = evalProgram(t, prg, map[string]interface{}{"ts": "not-a-timestamp"})
	if err == nil {
		t.Fatal("expected runtime error for invalid ISO 8601 string, got nil")
	}
}

func TestCELEval_RuntimeError_MissingField(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression("record.nonexistent.nested")
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	_, err = evalProgram(t, prg, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected runtime error for missing field, got nil")
	}
}

func TestCELEval_NoEnvFunction(t *testing.T) {
	c := mustNewCompiler(t)

	// env() should NOT be available -- attempting to compile should fail
	_, err := c.CompileExpression(`env("SECRET")`)
	if err == nil {
		t.Fatal("expected compile error for env() function, got nil")
	}
	if !strings.Contains(err.Error(), "CEL compile error") {
		t.Errorf("expected CEL compile error mentioning undeclared reference, got: %v", err)
	}
}

func TestCELEval_OutputLengthLimit(t *testing.T) {
	// This test verifies that output length limiting works when implemented.
	// For now, verify that long string evaluation succeeds (the adapter layer
	// is responsible for truncation, not the CEL compiler).
	c := mustNewCompiler(t)
	longStr := strings.Repeat("x", 2000)
	prg, err := c.CompileExpression(`"` + longStr + `"`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	got, ok := val.(string)
	if !ok {
		t.Fatalf("expected string, got %T", val)
	}
	if len(got) != 2000 {
		t.Errorf("expected string of length 2000, got %d", len(got))
	}
}

func TestParseRFC2822(t *testing.T) {
	c := mustNewCompiler(t)

	tests := []struct {
		name     string
		input    string
		expected string // RFC3339 UTC representation
	}{
		{
			name:     "Go reference time RFC1123Z",
			input:    "Mon, 02 Jan 2006 15:04:05 -0700",
			expected: "2006-01-02T22:04:05Z",
		},
		{
			name:     "UTC offset zero",
			input:    "Wed, 15 Jan 2026 10:30:00 +0000",
			expected: "2026-01-15T10:30:00Z",
		},
		{
			name:     "named timezone UTC",
			input:    "Mon, 02 Jan 2006 15:04:05 UTC",
			expected: "2006-01-02T15:04:05Z",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expr := fmt.Sprintf("parse_rfc2822(%q)", tc.input)
			prg, err := c.CompileExpression(expr)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}

			val, err := evalProgram(t, prg, map[string]interface{}{})
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}

			ts, ok := val.(time.Time)
			if !ok {
				t.Fatalf("expected time.Time, got %T (%v)", val, val)
			}

			expected, parseErr := time.Parse(time.RFC3339, tc.expected)
			if parseErr != nil {
				t.Fatalf("bad expected time %q: %v", tc.expected, parseErr)
			}

			if !ts.UTC().Equal(expected.UTC()) {
				t.Errorf("expected %v, got %v", expected.UTC(), ts.UTC())
			}
		})
	}
}

func TestParseRFC2822_InvalidInput(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`parse_rfc2822(record.ts)`)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	_, err = evalProgram(t, prg, map[string]interface{}{"ts": "not a date"})
	if err == nil {
		t.Fatal("expected runtime error for invalid RFC2822 string, got nil")
	}
}

package declarative

import (
	"testing"
	"time"
)

// TestEvalFieldMappingToString covers all branches of evalFieldMappingToString.
func TestEvalFieldMappingToString(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		typ     string
		record  map[string]interface{}
		maxLen  int
		want    string
		wantErr bool
	}{
		{
			name:   "timestamp type from unix_ms",
			expr:   "unix_ms(record.ts)",
			typ:    "timestamp",
			record: map[string]interface{}{"ts": float64(1700000000000)},
			maxLen: 1024,
			want:   time.UnixMilli(1700000000000).Format(time.RFC3339),
		},
		{
			name:   "integer type from float",
			expr:   "record.count",
			typ:    "integer",
			record: map[string]interface{}{"count": float64(42)},
			maxLen: 1024,
			want:   "42",
		},
		{
			name:   "float type",
			expr:   "record.value",
			typ:    "float",
			record: map[string]interface{}{"value": 3.14},
			maxLen: 1024,
			want:   "3.14",
		},
		{
			name:   "boolean type true",
			expr:   "record.flag",
			typ:    "boolean",
			record: map[string]interface{}{"flag": true},
			maxLen: 1024,
			want:   "true",
		},
		{
			name:   "boolean type false",
			expr:   "record.flag",
			typ:    "boolean",
			record: map[string]interface{}{"flag": false},
			maxLen: 1024,
			want:   "false",
		},
		{
			name:   "string type default",
			expr:   "record.name",
			typ:    "string",
			record: map[string]interface{}{"name": "hello"},
			maxLen: 1024,
			want:   "hello",
		},
		{
			name:   "unknown type falls through to evalString",
			expr:   "record.name",
			typ:    "unknown_type",
			record: map[string]interface{}{"name": "fallback"},
			maxLen: 1024,
			want:   "fallback",
		},
		{
			name:    "timestamp type eval error",
			expr:    "record.nonexistent",
			typ:     "timestamp",
			record:  map[string]interface{}{},
			maxLen:  1024,
			wantErr: true,
		},
		{
			name:    "integer type eval error",
			expr:    "record.name",
			typ:     "integer",
			record:  map[string]interface{}{"name": "not-a-number"},
			maxLen:  1024,
			wantErr: true,
		},
		{
			name:    "float type eval error",
			expr:    "record.name",
			typ:     "float",
			record:  map[string]interface{}{"name": "not-a-float"},
			maxLen:  1024,
			wantErr: true,
		},
		{
			name:    "boolean type eval error",
			expr:    "record.name",
			typ:     "boolean",
			record:  map[string]interface{}{"name": "not-a-bool"},
			maxLen:  1024,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := compiler.CompileExpression(tt.expr)
			if err != nil {
				t.Fatalf("CompileExpression(%q): %v", tt.expr, err)
			}

			activation := map[string]interface{}{"record": tt.record}
			got, err := evalFieldMappingToString(prg, tt.typ, activation, tt.maxLen)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("evalFieldMappingToString: %v", err)
			}
			if got != tt.want {
				t.Errorf("evalFieldMappingToString = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestEvalString_FloatConversion covers the float64 branch in evalString.
func TestEvalString_FloatConversion(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// A float expression that returns a float64 value.
	prg, err := compiler.CompileExpression("record.value")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{"value": 1.5}}
	got, err := evalString(prg, activation, 1024)
	if err != nil {
		t.Fatalf("evalString: %v", err)
	}
	if got != "1.5" {
		t.Errorf("evalString = %q, want %q", got, "1.5")
	}
}

// TestEvalFloat_StringParseable covers the string -> float64 parse path in evalFloat.
func TestEvalFloat_StringParseable(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// Expression returns a string that looks like a number.
	prg, err := compiler.CompileExpression("record.num_str")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{"num_str": "3.14"}}
	got, err := evalFloat(prg, activation)
	if err != nil {
		t.Fatalf("evalFloat: %v", err)
	}
	if got < 3.139 || got > 3.141 {
		t.Errorf("evalFloat = %v, want ~3.14", got)
	}
}

// TestEvalFloat_StringNotParseable covers the string parse error path.
func TestEvalFloat_StringNotParseable(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	prg, err := compiler.CompileExpression("record.bad")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{"bad": "notanumber"}}
	_, err = evalFloat(prg, activation)
	if err == nil {
		t.Error("expected error for non-numeric string, got nil")
	}
}

// TestEvalTimestamp_Int64 covers the int64 epoch milliseconds branch.
func TestEvalTimestamp_Int64(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// Use string() coercion to get an int64 back.
	// We force a literal int expression that returns int.
	prg, err := compiler.CompileExpression("int(record.ts)")
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
		t.Errorf("evalTimestamp = %v, want %v", got, expected)
	}
}

// TestEvalTimestamp_Float64 covers the float64 epoch milliseconds branch.
func TestEvalTimestamp_Float64(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	// A float expression.
	prg, err := compiler.CompileExpression("double(record.ts)")
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
		t.Errorf("evalTimestamp = %v, want %v", got, expected)
	}
}

// TestEvalTimestamp_StringRFC3339 covers the string -> RFC3339 parse path.
func TestEvalTimestamp_StringRFC3339(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	prg, err := compiler.CompileExpression("record.ts")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	tsStr := "2024-06-15T12:00:00Z"
	activation := map[string]interface{}{"record": map[string]interface{}{"ts": tsStr}}
	got, err := evalTimestamp(prg, activation)
	if err != nil {
		t.Fatalf("evalTimestamp: %v", err)
	}
	expected, _ := time.Parse(time.RFC3339, tsStr)
	if !got.Equal(expected) {
		t.Errorf("evalTimestamp = %v, want %v", got, expected)
	}
}

// TestEvalTimestamp_StringBadFormat covers the string parse failure path.
func TestEvalTimestamp_StringBadFormat(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	prg, err := compiler.CompileExpression("record.ts")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	activation := map[string]interface{}{"record": map[string]interface{}{"ts": "not-a-timestamp"}}
	_, err = evalTimestamp(prg, activation)
	if err == nil {
		t.Error("expected error for bad timestamp string, got nil")
	}
}

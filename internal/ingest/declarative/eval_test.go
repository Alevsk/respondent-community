package declarative

import (
	"testing"
	"time"
)

func TestEvalString(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		record  map[string]interface{}
		maxLen  int
		want    string
		wantErr bool
	}{
		{
			name:   "simple string field",
			expr:   "record.id",
			record: map[string]interface{}{"id": "abc123"},
			maxLen: 1024,
			want:   "abc123",
		},
		{
			name:   "string from int",
			expr:   "string(record.count)",
			record: map[string]interface{}{"count": float64(42)},
			maxLen: 1024,
			want:   "42",
		},
		{
			name:   "string concatenation",
			expr:   `"M" + string(record.mag) + " " + record.place`,
			record: map[string]interface{}{"mag": 3.5, "place": "California"},
			maxLen: 1024,
			want:   "M3.5 California",
		},
		{
			name:   "truncation at maxLen",
			expr:   "record.data",
			record: map[string]interface{}{"data": "abcdefghij"},
			maxLen: 5,
			want:   "abcde",
		},
		{
			name:   "no truncation when under limit",
			expr:   "record.data",
			record: map[string]interface{}{"data": "abc"},
			maxLen: 10,
			want:   "abc",
		},
		{
			name:   "conditional with has()",
			expr:   `has(record.name) && record.name != "" ? record.name : "unknown"`,
			record: map[string]interface{}{},
			maxLen: 1024,
			want:   "unknown",
		},
		{
			name:    "missing field without has() guard",
			expr:    "record.nonexistent",
			record:  map[string]interface{}{},
			maxLen:  1024,
			wantErr: true,
		},
		{
			name:   "int coercion to string",
			expr:   "record.count",
			record: map[string]interface{}{"count": float64(42)},
			maxLen: 1024,
			want:   "42",
		},
		{
			name:   "bool coercion to string",
			expr:   "record.flag",
			record: map[string]interface{}{"flag": true},
			maxLen: 1024,
			want:   "true",
		},
		{
			name:   "MMSI integer avoids scientific notation",
			expr:   "string(record.MMSI)",
			record: map[string]interface{}{"MMSI": float64(258187000)},
			maxLen: 1024,
			want:   "258187000",
		},
		{
			name:   "large integer external_id",
			expr:   "string(record.sensor_index)",
			record: map[string]interface{}{"sensor_index": float64(368352260)},
			maxLen: 1024,
			want:   "368352260",
		},
		{
			name:   "float with decimal preserved",
			expr:   "record.value",
			record: map[string]interface{}{"value": float64(3.14)},
			maxLen: 1024,
			want:   "3.14",
		},
		{
			name:   "concatenated prefix with large integer avoids scientific notation",
			expr:   `"bolt_" + string(record.time)`,
			record: map[string]interface{}{"time": float64(1775111450049258200)},
			maxLen: 1024,
			want:   "bolt_1775111450049258240",
		},
		{
			name:   "concatenated prefix with MMSI avoids scientific notation",
			expr:   `"pa_" + string(record.sensor_index)`,
			record: map[string]interface{}{"sensor_index": float64(368352260)},
			maxLen: 1024,
			want:   "pa_368352260",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prg, err := compiler.CompileExpression(tt.expr)
			if err != nil {
				t.Fatalf("CompileExpression(%q): %v", tt.expr, err)
			}

			activation := map[string]interface{}{"record": tt.record}
			got, err := evalString(prg, activation, tt.maxLen)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("evalString: %v", err)
			}
			if got != tt.want {
				t.Errorf("evalString = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"MMSI number", 258187000, "258187000"},
		{"large MMSI", 368352260, "368352260"},
		{"small integer", 42, "42"},
		{"zero", 0, "0"},
		{"negative integer", -100, "-100"},
		{"decimal", 3.14159, "3.14159"},
		{"small decimal", 0.001, "0.001"},
		{"very large integer", 9007199254740992, "9007199254740992"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatFloat(tt.input)
			if got != tt.want {
				t.Errorf("formatFloat(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEvalFloat(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		record  map[string]interface{}
		want    float64
		wantErr bool
	}{
		{
			name:   "simple float",
			expr:   "record.lat",
			record: map[string]interface{}{"lat": 37.7749},
			want:   37.7749,
		},
		{
			name:   "int to float coercion",
			expr:   "record.alt",
			record: map[string]interface{}{"alt": float64(10000)},
			want:   10000.0,
		},
		{
			name:   "arithmetic expression",
			expr:   "record.speed * 0.514444",
			record: map[string]interface{}{"speed": 250.0},
			want:   128.611,
		},
		{
			name:   "array access (GeoJSON coordinates)",
			expr:   "record.coordinates[0]",
			record: map[string]interface{}{"coordinates": []interface{}{-122.4194, 37.7749, 0.0}},
			want:   -122.4194,
		},
		{
			name:    "type mismatch",
			expr:    "record.name",
			record:  map[string]interface{}{"name": "not a number"},
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
			got, err := evalFloat(prg, activation)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("evalFloat: %v", err)
			}
			// Allow small floating point tolerance
			diff := got - tt.want
			if diff < -0.001 || diff > 0.001 {
				t.Errorf("evalFloat = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalBool(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		record  map[string]interface{}
		want    bool
		wantErr bool
	}{
		{
			name:   "filter pass",
			expr:   "record.mag >= 2.0",
			record: map[string]interface{}{"mag": 3.5},
			want:   true,
		},
		{
			name:   "filter reject",
			expr:   "record.mag >= 2.0",
			record: map[string]interface{}{"mag": 0.5},
			want:   false,
		},
		{
			name:   "has() guard on missing field",
			expr:   "has(record.mag) && record.mag >= 2.0",
			record: map[string]interface{}{},
			want:   false,
		},
		{
			name:   "list membership",
			expr:   `record.type in ["earthquake", "quarry blast"]`,
			record: map[string]interface{}{"type": "earthquake"},
			want:   true,
		},
		{
			name:   "list membership reject",
			expr:   `record.type in ["earthquake", "quarry blast"]`,
			record: map[string]interface{}{"type": "explosion"},
			want:   false,
		},
		{
			name:    "type mismatch returns error",
			expr:    "record.name",
			record:  map[string]interface{}{"name": "not a bool"},
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
			got, err := evalBool(prg, activation)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("evalBool: %v", err)
			}
			if got != tt.want {
				t.Errorf("evalBool = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvalTimestamp(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}

	tests := []struct {
		name    string
		expr    string
		record  map[string]interface{}
		check   func(time.Time) bool
		wantErr bool
	}{
		{
			name:   "unix_ms conversion",
			expr:   "unix_ms(record.time)",
			record: map[string]interface{}{"time": float64(1700000000000)},
			check: func(t time.Time) bool {
				return t.Equal(time.UnixMilli(1700000000000))
			},
		},
		{
			name:   "unix_s conversion",
			expr:   "unix_s(record.ts)",
			record: map[string]interface{}{"ts": float64(1700000000)},
			check: func(t time.Time) bool {
				return t.Equal(time.Unix(1700000000, 0))
			},
		},
		{
			name:   "parse_rfc3339",
			expr:   `parse_rfc3339(record.ts)`,
			record: map[string]interface{}{"ts": "2024-01-01T00:00:00Z"},
			check: func(t time.Time) bool {
				expected, _ := time.Parse(time.RFC3339, "2024-01-01T00:00:00Z")
				return t.Equal(expected)
			},
		},
		{
			name:   "now() function",
			expr:   "now()",
			record: map[string]interface{}{},
			check: func(t time.Time) bool {
				return time.Since(t) < 5*time.Second
			},
		},
		{
			name:    "type mismatch",
			expr:    "record.name",
			record:  map[string]interface{}{"name": "not a timestamp"},
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
			got, err := evalTimestamp(prg, activation)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("evalTimestamp: %v", err)
			}
			if !tt.check(got) {
				t.Errorf("evalTimestamp = %v, check failed", got)
			}
		})
	}
}

package declarative

import (
	"testing"
)

func TestCompileIndicatorLevelExpr(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		input   string
		want    int32
		wantErr bool
	}{
		{
			name:  "direct_int_cast",
			expr:  `int(double(value))`,
			input: "3",
			want:  3,
		},
		{
			name:  "ternary_thresholds",
			expr:  `double(value) >= 9.0 ? 5 : double(value) >= 7.0 ? 4 : double(value) >= 5.0 ? 3 : double(value) >= 4.0 ? 2 : double(value) >= 2.0 ? 1 : 0`,
			input: "5.5",
			want:  3,
		},
		{
			name:  "string_match",
			expr:  `value == "high" ? 4 : value == "medium" ? 2 : value == "low" ? 1 : 0`,
			input: "medium",
			want:  2,
		},
		{
			name:  "zero_for_invalid",
			expr:  `int(double(value))`,
			input: "not_a_number",
			want:  0, // CEL eval error returns 0
		},
		{
			name:    "compile_error",
			expr:    `invalid_syntax !!!`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := compileIndicatorLevelExpr(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected compile error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			got := fn(tt.input, "0")
			if got != tt.want {
				t.Errorf("level_expr(%q)(%q) = %d, want %d", tt.expr, tt.input, got, tt.want)
			}
		})
	}
}

func TestCompileIndicatorLevelExpr_ChangePct(t *testing.T) {
	tests := []struct {
		name      string
		expr      string
		value     string
		changePct string
		want      int32
	}{
		{
			name:      "change_pct_severity_low",
			expr:      `int(math.abs(double(change_pct))) < 1 ? 0 : int(math.abs(double(change_pct))) < 2 ? 1 : 2`,
			value:     "5400",
			changePct: "0.5",
			want:      0,
		},
		{
			name:      "change_pct_severity_medium",
			expr:      `int(math.abs(double(change_pct))) < 1 ? 0 : int(math.abs(double(change_pct))) < 2 ? 1 : 2`,
			value:     "5400",
			changePct: "-1.5",
			want:      1,
		},
		{
			name:      "change_pct_severity_high",
			expr:      `int(math.abs(double(change_pct))) < 1 ? 0 : int(math.abs(double(change_pct))) < 2 ? 1 : 2`,
			value:     "5400",
			changePct: "3.0",
			want:      2,
		},
		{
			name:      "change_pct_default_zero",
			expr:      `int(math.abs(double(change_pct)))`,
			value:     "100",
			changePct: "0",
			want:      0,
		},
		{
			name:      "value_only_still_works",
			expr:      `int(double(value))`,
			value:     "3",
			changePct: "0",
			want:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := compileIndicatorLevelExpr(tt.expr)
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			got := fn(tt.value, tt.changePct)
			if got != tt.want {
				t.Errorf("level_expr(%q)(value=%q, change_pct=%q) = %d, want %d", tt.expr, tt.value, tt.changePct, got, tt.want)
			}
		})
	}
}

func TestCompileIndicatorSummaryExpr(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		metadata map[string]string
		level    int32
		want     string
		wantErr  bool
	}{
		{
			name:     "standard_severity_scale",
			expr:     `level >= 5 ? "Extreme" : level >= 4 ? "Severe" : level >= 3 ? "Strong" : level >= 2 ? "Moderate" : level >= 1 ? "Minor Activity" : "Quiet"`,
			metadata: nil,
			level:    3,
			want:     "Strong",
		},
		{
			name:     "metadata_aware_summary",
			expr:     `"R" + metadata["r_scale"] + " S" + metadata["s_scale"] + " G" + metadata["g_scale"]`,
			metadata: map[string]string{"r_scale": "2", "s_scale": "1", "g_scale": "0"},
			level:    2,
			want:     "R2 S1 G0",
		},
		{
			name:     "level_zero",
			expr:     `level == 0 ? "All Clear" : "Active"`,
			metadata: nil,
			level:    0,
			want:     "All Clear",
		},
		{
			name:    "compile_error",
			expr:    `bad syntax !!!`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn, err := compileIndicatorSummaryExpr(tt.expr)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected compile error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected compile error: %v", err)
			}
			got := fn(tt.metadata, tt.level)
			if got != tt.want {
				t.Errorf("summary_expr(%q)(metadata, %d) = %q, want %q", tt.expr, tt.level, got, tt.want)
			}
		})
	}
}

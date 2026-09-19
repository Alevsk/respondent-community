package schema

import (
	"strings"
	"testing"
)

func TestRequiredKeys(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
		want   []string
	}{
		{"no required key", map[string]any{"type": "object"}, nil},
		{"empty required", map[string]any{"required": []any{}}, []string{}},
		{"two keys", map[string]any{"required": []any{"threats", "summary"}}, []string{"threats", "summary"}},
		{"skips non-strings and blanks", map[string]any{"required": []any{"a", 1, "", "b"}}, []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RequiredKeys(tt.schema)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestCorrectionMessage_NamesRequiredKeys(t *testing.T) {
	msg := CorrectionMessage(map[string]any{"required": []any{"threats", "summary"}})
	if !strings.Contains(msg, "threats, summary") {
		t.Errorf("correction message should name the required keys, got: %q", msg)
	}
}

func TestCorrectionMessage_FallsBackWithoutRequiredKeys(t *testing.T) {
	msg := CorrectionMessage(map[string]any{"type": "object"})
	if msg == "" || strings.Contains(msg, "EXACTLY:") {
		t.Errorf("expected generic fallback message, got: %q", msg)
	}
}

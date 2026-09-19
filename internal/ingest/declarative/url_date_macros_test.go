package declarative

import (
	"testing"
	"time"
)

func TestExpandDateMacros(t *testing.T) {
	now := time.Date(2026, 6, 25, 18, 30, 0, 0, time.UTC)
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"no macro", "https://x/api?a=1", "https://x/api?a=1"},
		{"now", "https://x?d={date:now}", "https://x?d=2026-06-25"},
		{"minus days", "https://x?s={date:-7d}", "https://x?s=2026-06-18"},
		{"plus days", "https://x?e={date:+1d}", "https://x?e=2026-06-26"},
		{"minus hours rolls date", "https://x?s={date:-24h}", "https://x?s=2026-06-24"},
		{"range", "https://x?s={date:-3d}&e={date:now}", "https://x?s=2026-06-22&e=2026-06-25"},
		{"malformed left untouched", "https://x?d={date:-7y}", "https://x?d={date:-7y}"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := expandDateMacros(tc.in, now); got != tc.want {
				t.Errorf("expandDateMacros(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandDateMacros_NoTokenIsCheap(t *testing.T) {
	in := "https://example.com/path?x=1&y=2"
	if got := expandDateMacros(in, time.Now()); got != in {
		t.Errorf("expected unchanged URL, got %q", got)
	}
}

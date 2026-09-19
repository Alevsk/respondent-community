package declarative

import (
	"strings"
	"testing"
)

// TestDecodeLZW_InvalidCode verifies the error path for an invalid code point.
func TestDecodeLZW_InvalidCode(t *testing.T) {
	// We need to construct an LZW-encoded stream where a code is invalid.
	// The simplest way: create a stream where the second code is not in the
	// dictionary and also != dictSize (which would be the "special case").
	// We'll manually craft a slice where the first char maps to dict[0]='A' (ASCII 65)
	// and the second code is way out of range (e.g., rune(9999)).

	// Build a byte slice that encodes rune(65) then rune(9999).
	// rune(65) = 'A', rune(9999) is a valid unicode but not in the dict.
	data := []byte(string([]rune{65, 9999}))

	_, err := decodeLZW(data)
	if err == nil {
		t.Fatal("expected error for invalid code, got nil")
	}
	if !strings.Contains(err.Error(), "invalid code") {
		t.Errorf("expected 'invalid code' error, got: %v", err)
	}
}

// TestDecodeLZW_SingleChar verifies decompression of a single character.
func TestDecodeLZW_SingleChar(t *testing.T) {
	// Encoding a single 'A' - just one char in the stream, nothing to iterate
	data := encodeLZW("A")
	decoded, err := decodeLZW(data)
	if err != nil {
		t.Fatalf("decodeLZW: %v", err)
	}
	if string(decoded) != "A" {
		t.Errorf("decoded = %q, want %q", string(decoded), "A")
	}
}

// TestDecodeLZW_SpecialCase verifies the case where code == dictSize.
// This happens when a sequence ends with itself: e.g. "ABAABA"
// The sequence "ABABAB" triggers the code == dictSize case.
func TestDecodeLZW_SpecialCase(t *testing.T) {
	// The "ABABAB" string reliably triggers the code == dictSize case in LZW.
	input := "ABABAB"
	encoded := encodeLZW(input)
	decoded, err := decodeLZW(encoded)
	if err != nil {
		t.Fatalf("decodeLZW(ABABAB): %v", err)
	}
	if string(decoded) != input {
		t.Errorf("decoded = %q, want %q", string(decoded), input)
	}
}

// TestDecodeLZW_LongRepetitive verifies decompression of a long, repetitive string.
func TestDecodeLZW_LongRepetitive(t *testing.T) {
	input := strings.Repeat("abcde", 100)
	encoded := encodeLZW(input)
	decoded, err := decodeLZW(encoded)
	if err != nil {
		t.Fatalf("decodeLZW: %v", err)
	}
	if string(decoded) != input {
		t.Errorf("roundtrip failed, lengths: encoded=%d, decoded=%d", len(encoded), len(decoded))
	}
}

// TestDecodeLZW_AllAscii verifies decompression of all printable ASCII chars.
func TestDecodeLZW_AllAscii(t *testing.T) {
	// Build a string with all ASCII printable chars
	var sb strings.Builder
	for i := 32; i < 127; i++ {
		sb.WriteByte(byte(i))
	}
	input := sb.String()
	encoded := encodeLZW(input)
	decoded, err := decodeLZW(encoded)
	if err != nil {
		t.Fatalf("decodeLZW: %v", err)
	}
	if string(decoded) != input {
		t.Errorf("ASCII roundtrip failed")
	}
}

// TestDecodeLZW_JSONLike verifies decompression of JSON-like data.
func TestDecodeLZW_JSONLike(t *testing.T) {
	tests := []string{
		`{"key":"value"}`,
		`[1,2,3,4,5]`,
		`{"lat":51.5074,"lon":-0.1278,"alt":1000}`,
		`{"msg":"hello world","count":42,"flag":true}`,
	}

	for _, input := range tests {
		t.Run(input[:min(len(input), 20)], func(t *testing.T) {
			encoded := encodeLZW(input)
			decoded, err := decodeLZW(encoded)
			if err != nil {
				t.Fatalf("decodeLZW: %v", err)
			}
			if string(decoded) != input {
				t.Errorf("decoded = %q, want %q", string(decoded), input)
			}
		})
	}
}

// min returns the minimum of two ints.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

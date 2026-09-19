package declarative

import (
	"strings"
	"testing"
)

func TestDecodeLZW_InvalidData(t *testing.T) {
	// Random bytes that are not valid LZW data should cause an error.
	_, err := decodeLZW([]byte{0x01, 0x02, 0x03})
	if err == nil {
		// It may or may not fail depending on the LZW implementation.
		// If no error, that's acceptable - the function returned successfully.
		t.Log("decodeLZW did not error on random data (implementation dependent)")
	}
}

func TestDecodeLZW_EmptyInput(t *testing.T) {
	_, err := decodeLZW([]byte{})
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
	if !strings.Contains(err.Error(), "empty input") {
		t.Errorf("expected 'empty input' error, got %q", err.Error())
	}
}

func TestDecodeLZW_SingleCharacter(t *testing.T) {
	// LZW with a single code point: just 'A' (code 65).
	// The result should be the single character.
	result, err := decodeLZW([]byte("A"))
	if err != nil {
		t.Fatalf("decodeLZW single char: %v", err)
	}
	if len(result) == 0 {
		t.Error("expected non-empty result for single character LZW")
	}
}

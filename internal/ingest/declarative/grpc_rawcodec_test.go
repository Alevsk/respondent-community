package declarative

import (
	"strings"
	"testing"
)

// TestRawCodec_Marshal_Success verifies that rawCodec.Marshal works with []byte input.
func TestRawCodec_Marshal_Success(t *testing.T) {
	c := rawCodec{}
	data := []byte("hello world")
	result, err := c.Marshal(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(data) {
		t.Errorf("expected %q, got %q", data, result)
	}
}

// TestRawCodec_Marshal_WrongType verifies that rawCodec.Marshal errors on non-[]byte input.
func TestRawCodec_Marshal_WrongType(t *testing.T) {
	c := rawCodec{}
	_, err := c.Marshal("not bytes")
	if err == nil {
		t.Fatal("expected error for non-[]byte input, got nil")
	}
	if !strings.Contains(err.Error(), "expected []byte") {
		t.Errorf("expected 'expected []byte' in error, got: %v", err)
	}
}

// TestRawCodec_Unmarshal_Success verifies that rawCodec.Unmarshal works with *[]byte.
func TestRawCodec_Unmarshal_Success(t *testing.T) {
	c := rawCodec{}
	data := []byte("test data")
	var result []byte
	err := c.Unmarshal(data, &result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != string(data) {
		t.Errorf("expected %q, got %q", data, result)
	}
}

// TestRawCodec_Unmarshal_WrongType verifies that rawCodec.Unmarshal errors on non-*[]byte.
func TestRawCodec_Unmarshal_WrongType(t *testing.T) {
	c := rawCodec{}
	var result string
	err := c.Unmarshal([]byte("data"), &result)
	if err == nil {
		t.Fatal("expected error for non-*[]byte input, got nil")
	}
	if !strings.Contains(err.Error(), "expected *[]byte") {
		t.Errorf("expected 'expected *[]byte' in error, got: %v", err)
	}
}

// TestRawCodec_Name verifies the codec name.
func TestRawCodec_Name(t *testing.T) {
	c := rawCodec{}
	if c.Name() != "raw" {
		t.Errorf("expected 'raw', got %q", c.Name())
	}
}

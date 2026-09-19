package declarative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoerceCSVValue_IntegerBranch verifies that integer strings are parsed as int64.
func TestCoerceCSVValue_IntegerBranch(t *testing.T) {
	tests := []struct {
		input    string
		wantType string
		wantVal  interface{}
	}{
		{"42", "int64", int64(42)},
		{"-7", "int64", int64(-7)},
		{"0", "int64", int64(0)},
		{"9999999", "int64", int64(9999999)},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := coerceCSVValue(tt.input)
			if got != tt.wantVal {
				t.Errorf("coerceCSVValue(%q) = %v (%T), want %v (%s)", tt.input, got, got, tt.wantVal, tt.wantType)
			}
		})
	}
}

// TestCoerceCSVValue_FloatBranch verifies float strings are parsed as float64.
func TestCoerceCSVValue_FloatBranch(t *testing.T) {
	tests := []struct {
		input   string
		wantVal float64
	}{
		{"3.14", 3.14},
		{"-0.5", -0.5},
		{"1.0e3", 1000.0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := coerceCSVValue(tt.input)
			f, ok := got.(float64)
			if !ok {
				t.Fatalf("expected float64, got %T", got)
			}
			if f != tt.wantVal {
				t.Errorf("coerceCSVValue(%q) = %v, want %v", tt.input, f, tt.wantVal)
			}
		})
	}
}

// TestCoerceCSVValue_BoolBranch verifies bool strings are parsed as bool.
func TestCoerceCSVValue_BoolBranch(t *testing.T) {
	tests := []struct {
		input   string
		wantVal bool
	}{
		{"true", true},
		{"false", false},
		{"TRUE", true},
		{"FALSE", false},
		{"1", false}, // "1" is parsed as int64 before bool; bool branch not reached
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := coerceCSVValue(tt.input)
			// "1" will be int64 (integer branch first), not bool
			if tt.input == "1" {
				if _, ok := got.(int64); !ok {
					t.Errorf("expected int64 for %q, got %T", tt.input, got)
				}
				return
			}
			b, ok := got.(bool)
			if !ok {
				t.Fatalf("expected bool for %q, got %T (%v)", tt.input, got, got)
			}
			if b != tt.wantVal {
				t.Errorf("coerceCSVValue(%q) = %v, want %v", tt.input, b, tt.wantVal)
			}
		})
	}
}

// TestCoerceCSVValue_StringFallback verifies non-parseable values fall back to string.
func TestCoerceCSVValue_StringFallback(t *testing.T) {
	tests := []string{"hello", "world", "", "not-a-number-or-bool", "123abc"}
	for _, s := range tests {
		t.Run(s, func(t *testing.T) {
			got := coerceCSVValue(s)
			str, ok := got.(string)
			if !ok {
				t.Fatalf("expected string for %q, got %T", s, got)
			}
			if str != s {
				t.Errorf("coerceCSVValue(%q) = %q, want %q", s, str, s)
			}
		})
	}
}

// TestParseJSONLookup_InvalidJSON verifies the error path for malformed JSON.
func TestParseJSONLookup_InvalidJSON(t *testing.T) {
	_, err := parseJSONLookup([]byte("this is not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "parse JSON") {
		t.Errorf("expected 'parse JSON' in error, got: %v", err)
	}
}

// TestParseJSONLookup_NotAnArray verifies the error path when JSON is not an array.
func TestParseJSONLookup_NotAnArray(t *testing.T) {
	// JSON object instead of array
	_, err := parseJSONLookup([]byte(`{"key": "value"}`))
	if err == nil {
		t.Fatal("expected error for non-array JSON, got nil")
	}
}

// TestParseCSVLookup_EmptyData verifies that a CSV with only header returns an error.
func TestParseCSVLookup_EmptyData(t *testing.T) {
	// Only a header row, no data rows
	_, err := parseCSVLookup([]byte("id,name\n"))
	if err == nil {
		t.Fatal("expected error for CSV with only header, got nil")
	}
	if !strings.Contains(err.Error(), "at least one data row") {
		t.Errorf("expected 'at least one data row' in error, got: %v", err)
	}
}

// TestLoadLookupFile_UnsupportedFormat verifies the unsupported format error path.
func TestLoadLookupFile_UnsupportedFormat(t *testing.T) {
	dir := t.TempDir()
	// Create a file with an unsupported format
	filePath := filepath.Join(dir, "data.xml")
	if err := os.WriteFile(filePath, []byte("<root/>"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	spec := []LookupTableSpec{
		{
			Name:     "test",
			KeyField: "id",
			File:     "data.xml",
			Format:   "xml",
		},
	}
	_, err := LoadLookupTables(spec, dir)
	if err == nil {
		t.Fatal("expected error for unsupported format, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported lookup file format") {
		t.Errorf("expected 'unsupported lookup file format' error, got: %v", err)
	}
}

// TestLoadLookupFile_InvalidJSONContent verifies JSON parse error propagation.
func TestLoadLookupFile_InvalidJSONContent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(filePath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	spec := []LookupTableSpec{
		{
			Name:     "test",
			KeyField: "id",
			File:     "bad.json",
			Format:   "json",
		},
	}
	_, err := LoadLookupTables(spec, dir)
	if err == nil {
		t.Fatal("expected error for invalid JSON file content, got nil")
	}
}

// TestParseCSVLookup_OnlyHeader verifies that a CSV with zero rows after header fails.
func TestParseCSVLookup_OnlyHeader(t *testing.T) {
	// Header only, no data rows - no trailing newline means 1 record total
	_, err := parseCSVLookup([]byte("col1,col2"))
	if err == nil {
		t.Fatal("expected error for CSV with only header and no data rows, got nil")
	}
}

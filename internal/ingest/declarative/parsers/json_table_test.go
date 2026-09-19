package parsers

import (
	"strings"
	"testing"
)

func TestJSONTableParser_Basic(t *testing.T) {
	input := `[
		["time_tag", "Kp", "a_running", "station_count"],
		["2026-03-15 21:00:00.000", "3.33", "18", "8"],
		["2026-03-16 00:00:00.000", "2.67", "12", "8"],
		["2026-03-16 03:00:00.000", "1.00", "5", "7"]
	]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}

	// Verify first record keys and values.
	r0 := records[0]
	if r0["time_tag"] != "2026-03-15 21:00:00.000" {
		t.Errorf("expected time_tag '2026-03-15 21:00:00.000', got %v", r0["time_tag"])
	}
	if r0["Kp"] != "3.33" {
		t.Errorf("expected Kp '3.33', got %v", r0["Kp"])
	}
	if r0["a_running"] != "18" {
		t.Errorf("expected a_running '18', got %v", r0["a_running"])
	}
	if r0["station_count"] != "8" {
		t.Errorf("expected station_count '8', got %v", r0["station_count"])
	}

	// Verify last record.
	r2 := records[2]
	if r2["Kp"] != "1.00" {
		t.Errorf("expected Kp '1.00', got %v", r2["Kp"])
	}
}

func TestJSONTableParser_SingleRow(t *testing.T) {
	input := `[["col1", "col2"], ["val1", "val2"]]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["col1"] != "val1" {
		t.Errorf("expected col1='val1', got %v", records[0]["col1"])
	}
}

func TestJSONTableParser_NullValues(t *testing.T) {
	input := `[["a", "b"], ["hello", null]]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["a"] != "hello" {
		t.Errorf("expected a='hello', got %v", records[0]["a"])
	}
	if records[0]["b"] != nil {
		t.Errorf("expected b=nil, got %v", records[0]["b"])
	}
}

func TestJSONTableParser_HeaderOnly(t *testing.T) {
	input := `[["a", "b"]]`

	p := &JSONTableParser{}
	_, err := p.Parse([]byte(input), ParserConfig{})
	if err == nil {
		t.Fatal("expected error for header-only table, got nil")
	}
	if !strings.Contains(err.Error(), "need at least header + 1 data row") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJSONTableParser_NonStringHeader(t *testing.T) {
	input := `[[1, 2], ["a", "b"]]`

	p := &JSONTableParser{}
	_, err := p.Parse([]byte(input), ParserConfig{})
	if err == nil {
		t.Fatal("expected error for non-string header, got nil")
	}
	if !strings.Contains(err.Error(), "header[0] is not a string") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestJSONTableParser_ShortRow(t *testing.T) {
	// Row shorter than headers: missing keys are omitted from the record.
	input := `[["a", "b", "c"], ["only_a"]]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["a"] != "only_a" {
		t.Errorf("expected a='only_a', got %v", records[0]["a"])
	}
	if _, exists := records[0]["b"]; exists {
		t.Errorf("expected key 'b' to be absent, but it exists: %v", records[0]["b"])
	}
	if _, exists := records[0]["c"]; exists {
		t.Errorf("expected key 'c' to be absent, but it exists: %v", records[0]["c"])
	}
}

func TestJSONTableParser_EmptyInput(t *testing.T) {
	p := &JSONTableParser{}
	_, err := p.Parse([]byte(""), ParserConfig{})
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestJSONTableParser_InvalidJSON(t *testing.T) {
	p := &JSONTableParser{}
	_, err := p.Parse([]byte("not json"), ParserConfig{})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

func TestJSONTableParser_NumericValues(t *testing.T) {
	// JSON numbers are float64 after unmarshal.
	input := `[["name", "value"], ["temp", 42.5]]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0]["value"] != 42.5 {
		t.Errorf("expected value=42.5, got %v", records[0]["value"])
	}
}

func TestJSONTableParser_MaxRecords(t *testing.T) {
	// Generate table with 5 data rows, limit to 2.
	input := `[["x"], ["1"], ["2"], ["3"], ["4"], ["5"]]`

	p := &JSONTableParser{}
	records, err := p.Parse([]byte(input), ParserConfig{MaxRecords: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (max_records=2), got %d", len(records))
	}
}

func TestNewParser_JSONTable(t *testing.T) {
	p, err := NewParser("json_table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p == nil {
		t.Fatal("expected non-nil parser")
	}
	if _, ok := p.(*JSONTableParser); !ok {
		t.Fatalf("expected *JSONTableParser, got %T", p)
	}
}

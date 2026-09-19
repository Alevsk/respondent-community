package declarative

import (
	"testing"
)

func TestLoadLookupFile_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := loadLookupFile("../../../etc/passwd", "json", dir)
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestParseCSVLookup_ParseError(t *testing.T) {
	// A malformed CSV: unescaped bare quote inside a field triggers a parse error.
	malformed := []byte("col1,col2\n\"val1,val2")
	_, err := parseCSVLookup(malformed)
	if err == nil {
		t.Error("expected error for malformed CSV, got nil")
	}
}

func TestParseCSVLookup_TooFewRows(t *testing.T) {
	// Only a header row, no data rows.
	_, err := parseCSVLookup([]byte("col1,col2"))
	if err == nil {
		t.Error("expected error for CSV with only header row, got nil")
	}
}

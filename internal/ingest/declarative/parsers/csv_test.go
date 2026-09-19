package parsers

import (
	"testing"
)

func TestCollapseWhitespace_BasicParsing(t *testing.T) {
	input := []byte("STN   LAT   LON\n41001  34.7  -72.7\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	assertField(t, records[0], "STN", "41001")
	assertField(t, records[0], "LAT", "34.7")
	assertField(t, records[0], "LON", "-72.7")
}

func TestCollapseWhitespace_WithSkipLines(t *testing.T) {
	// 3 non-comment header lines, skip 2, then data.
	input := []byte("--- header ---\n--- units ---\nSTN   LAT   LON\n41001  34.7  -72.7\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
			SkipLines:          2,
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	assertField(t, records[0], "STN", "41001")
	assertField(t, records[0], "LAT", "34.7")
	assertField(t, records[0], "LON", "-72.7")
}

func TestCollapseWhitespace_WithCommentPrefix(t *testing.T) {
	// Two comment lines: first becomes header (# stripped), second is skipped.
	input := []byte("#STN   LAT   LON\n#text  deg   deg\n41001  34.7  -72.7\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
			CommentPrefix:      "#",
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	// With CommentPrefix="#", the # is stripped from first line (header).
	// Second comment line (#text deg deg) is skipped entirely.
	assertField(t, records[0], "STN", "41001")
	assertField(t, records[0], "LAT", "34.7")
	assertField(t, records[0], "LON", "-72.7")
}

func TestCollapseWhitespace_MissingMeasurementValues(t *testing.T) {
	input := []byte("STN   LAT   LON   WTMP\n41001  34.7  -72.7  MM\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	assertField(t, records[0], "WTMP", "MM")
}

func TestCollapseWhitespace_EmptyLinesSkipped(t *testing.T) {
	input := []byte("STN   LAT\n\n41001  34.7\n\n41002  35.0\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	assertField(t, records[0], "STN", "41001")
	assertField(t, records[1], "STN", "41002")
}

func TestCollapseWhitespace_MixedTabsAndSpaces(t *testing.T) {
	input := []byte("STN\tLAT   LON\n41001\t34.7   -72.7\n")
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	assertField(t, records[0], "STN", "41001")
	assertField(t, records[0], "LAT", "34.7")
	assertField(t, records[0], "LON", "-72.7")
}

func TestCollapseWhitespace_NDBCRealisticData(t *testing.T) {
	// Simulates actual NOAA NDBC latest_obs.txt structure.
	input := []byte(
		"#STN       LAT      LON  YYYY MM DD hh mm WDIR WSPD   GST WVHT\n" +
			"#text      deg      deg   yr mo day hr mn degT  m/s   m/s   m\n" +
			"22101    37.24   126.02  2026 03 16 04 00  20   3.0    MM  0.0\n" +
			"TPLM2    38.535  -76.413 2026 03 16 04 00 220   4.1   6.7   MM\n",
	)
	parser := &CSVParser{}
	cfg := ParserConfig{
		CSVOptions: &CSVOpts{
			CollapseWhitespace: true,
			HasHeader:          true,
			CommentPrefix:      "#",
		},
	}
	records, err := parser.Parse(input, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	assertField(t, records[0], "STN", "22101")
	assertField(t, records[0], "LAT", "37.24")
	assertField(t, records[0], "LON", "126.02")
	assertField(t, records[0], "WSPD", "3.0")
	assertField(t, records[0], "GST", "MM")
	assertField(t, records[0], "WVHT", "0.0")

	assertField(t, records[1], "STN", "TPLM2")
	assertField(t, records[1], "LAT", "38.535")
	assertField(t, records[1], "WSPD", "4.1")
	assertField(t, records[1], "GST", "6.7")
	assertField(t, records[1], "WVHT", "MM")
}

func assertField(t *testing.T, record map[string]interface{}, key, expected string) {
	t.Helper()
	val, ok := record[key]
	if !ok {
		t.Errorf("field %q not found in record (keys: %v)", key, keys(record))
		return
	}
	if val != expected {
		t.Errorf("field %q = %q, want %q", key, val, expected)
	}
}

func keys(m map[string]interface{}) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

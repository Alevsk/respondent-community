package declarative

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testdataDir resolves the path to the testdata/ directory relative to this test file.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to determine test file path via runtime.Caller")
	}
	dir := filepath.Join(filepath.Dir(filename), "testdata")
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("failed to resolve testdata path: %v", err)
	}
	return abs
}

// --- Table Loading Tests ---

func TestLoadLookupTables_InlineEntries(t *testing.T) {
	specs := []LookupTableSpec{
		{
			Name:     "ports",
			KeyField: "code",
			Entries: []map[string]interface{}{
				{"code": "SYD", "lat": -33.8688, "lon": 151.2093},
				{"code": "LAX", "lat": 33.9425, "lon": -118.4081},
			},
		},
	}

	tables, err := LoadLookupTables(specs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}

	ports := tables["ports"]
	if ports == nil {
		t.Fatal("expected 'ports' table")
	}
	if len(ports.Data) != 2 {
		t.Errorf("expected 2 entries, got %d", len(ports.Data))
	}

	entry, ok := ports.Data["SYD"]
	if !ok {
		t.Fatal("expected key 'SYD'")
	}
	if entry["lat"] != -33.8688 {
		t.Errorf("expected lat -33.8688, got %v", entry["lat"])
	}
}

func TestLoadLookupTables_NumericKeys(t *testing.T) {
	specs := []LookupTableSpec{
		{
			Name:     "stations",
			KeyField: "id",
			Entries: []map[string]interface{}{
				{"id": 42, "name": "Station Alpha"},
				{"id": 99, "name": "Station Beta"},
			},
		},
	}

	tables, err := LoadLookupTables(specs, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	station := tables["stations"]
	// Numeric keys should be coerced to string via fmt.Sprintf("%v", val)
	if _, ok := station.Data["42"]; !ok {
		t.Error("expected key '42' (numeric coerced to string)")
	}
	if _, ok := station.Data["99"]; !ok {
		t.Error("expected key '99' (numeric coerced to string)")
	}
}

func TestLoadLookupTables_FileJSON(t *testing.T) {
	dir := testdataDir(t)

	specs := []LookupTableSpec{
		{
			Name:     "ports",
			KeyField: "port_number",
			File:     "lookups/test_ports.json",
			Format:   "json",
		},
	}

	tables, err := LoadLookupTables(specs, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ports := tables["ports"]
	if ports == nil {
		t.Fatal("expected 'ports' table")
	}
	if len(ports.Data) != 3 {
		t.Errorf("expected 3 entries, got %d", len(ports.Data))
	}

	entry, ok := ports.Data["250401"]
	if !ok {
		t.Fatal("expected key '250401'")
	}
	if entry["name"] != "San Ysidro" {
		t.Errorf("expected name 'San Ysidro', got %v", entry["name"])
	}
}

func TestLoadLookupTables_FileCSV(t *testing.T) {
	dir := testdataDir(t)

	specs := []LookupTableSpec{
		{
			Name:     "stations",
			KeyField: "station_id",
			File:     "lookups/test_stations.csv",
			Format:   "csv",
		},
	}

	tables, err := LoadLookupTables(specs, dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stations := tables["stations"]
	if stations == nil {
		t.Fatal("expected 'stations' table")
	}
	if len(stations.Data) != 3 {
		t.Errorf("expected 3 entries, got %d", len(stations.Data))
	}

	entry, ok := stations.Data["KJFK"]
	if !ok {
		t.Fatal("expected key 'KJFK'")
	}
	// CSV string values remain strings
	if entry["name"] != "John F Kennedy International" {
		t.Errorf("expected name 'John F Kennedy International', got %v", entry["name"])
	}
	// CSV numeric values are coerced to their native types
	if lat, ok := entry["lat"].(float64); !ok || lat != 40.6398 {
		t.Errorf("expected lat 40.6398 (float64), got %v (%T)", entry["lat"], entry["lat"])
	}
}

func TestLoadLookupTables_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		specs   []LookupTableSpec
		wantErr string
	}{
		{
			name: "both entries and file",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
				Entries:  []map[string]interface{}{{"id": "1"}},
				File:     "some/file.json",
				Format:   "json",
			}},
			wantErr: "exactly one of 'entries' or 'file' must be set",
		},
		{
			name: "neither entries nor file",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
			}},
			wantErr: "exactly one of 'entries' or 'file' must be set",
		},
		{
			name: "missing key_field in entry",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
				Entries:  []map[string]interface{}{{"name": "no id here"}},
			}},
			wantErr: "missing key field",
		},
		{
			name: "duplicate table names",
			specs: []LookupTableSpec{
				{Name: "dup", KeyField: "id", Entries: []map[string]interface{}{{"id": "1"}}},
				{Name: "dup", KeyField: "id", Entries: []map[string]interface{}{{"id": "2"}}},
			},
			wantErr: "duplicate table name",
		},
		{
			name: "absolute path rejected",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
				File:     "/etc/passwd",
				Format:   "json",
			}},
			wantErr: "absolute paths not allowed",
		},
		{
			name: "path traversal rejected",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
				File:     "../../../etc/passwd",
				Format:   "json",
			}},
			wantErr: "path traversal not allowed",
		},
		{
			name: "format required with file",
			specs: []LookupTableSpec{{
				Name:     "bad",
				KeyField: "id",
				File:     "lookups/test.json",
			}},
			wantErr: "'format' is required when 'file' is set",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := LoadLookupTables(tc.specs, t.TempDir())
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoadLookupTables_MemoryLimits(t *testing.T) {
	t.Run("exceed max entries", func(t *testing.T) {
		entries := make([]map[string]interface{}, maxLookupEntriesPerTable+1)
		for i := range entries {
			entries[i] = map[string]interface{}{"id": i}
		}
		specs := []LookupTableSpec{{
			Name:     "huge",
			KeyField: "id",
			Entries:  entries,
		}}
		_, err := LoadLookupTables(specs, "")
		if err == nil {
			t.Fatal("expected error for exceeding max entries")
		}
		if !strings.Contains(err.Error(), "exceeds maximum") {
			t.Errorf("expected max entries error, got: %v", err)
		}
	})

	t.Run("exceed max tables", func(t *testing.T) {
		specs := make([]LookupTableSpec, maxLookupTablesPerSource+1)
		for i := range specs {
			specs[i] = LookupTableSpec{
				Name:     "table_" + strings.Repeat("a", i),
				KeyField: "id",
				Entries:  []map[string]interface{}{{"id": "1"}},
			}
		}
		_, err := LoadLookupTables(specs, "")
		if err == nil {
			t.Fatal("expected error for exceeding max tables")
		}
		if !strings.Contains(err.Error(), "too many lookup tables") {
			t.Errorf("expected max tables error, got: %v", err)
		}
	})
}

// --- CEL Lookup Function Tests ---

func mustCompileWithLookups(t *testing.T, expr string, tables map[string]*LookupTable) func(map[string]interface{}) (interface{}, error) {
	t.Helper()
	c := mustNewCompiler(t)
	prg, err := c.CompileExpressionWithLookups(expr, tables)
	if err != nil {
		t.Fatalf("failed to compile %q: %v", expr, err)
	}
	return func(record map[string]interface{}) (interface{}, error) {
		return evalProgram(t, prg, record)
	}
}

func testTables() map[string]*LookupTable {
	return map[string]*LookupTable{
		"ports": {
			Name:     "ports",
			KeyField: "port_number",
			Data: map[string]map[string]interface{}{
				"250401": {
					"port_number": "250401",
					"lat":         32.5423,
					"lon":         -117.0292,
					"name":        "San Ysidro",
					"active":      true,
					"zone":        int64(8),
				},
				"250601": {
					"port_number": "250601",
					"lat":         32.6731,
					"lon":         -115.4983,
					"name":        "Calexico",
					"active":      true,
					"zone":        int64(7),
				},
			},
		},
	}
}

func TestCELLookup_BasicLookup(t *testing.T) {
	eval := mustCompileWithLookups(t, `lookup("ports", "250401", "name")`, testTables())
	val, err := eval(map[string]interface{}{})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	if val != "San Ysidro" {
		t.Errorf("expected 'San Ysidro', got %v", val)
	}
}

func TestCELLookup_MissingKey(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpressionWithLookups(`lookup("ports", "999999", "name")`, testTables())
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	_, err = evalProgram(t, prg, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestCELLookup_UnknownTable(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpressionWithLookups(`lookup("nonexistent", "key", "field")`, testTables())
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	_, err = evalProgram(t, prg, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for unknown table, got nil")
	}
	if !strings.Contains(err.Error(), "unknown table") {
		t.Errorf("expected 'unknown table' error, got: %v", err)
	}
}

func TestCELLookup_MissingField(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpressionWithLookups(`lookup("ports", "250401", "nonexistent_field")`, testTables())
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	_, err = evalProgram(t, prg, map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing field, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestCELLookup_HasLookup(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected bool
	}{
		{name: "existing key", expr: `has_lookup("ports", "250401")`, expected: true},
		{name: "missing key", expr: `has_lookup("ports", "999999")`, expected: false},
		{name: "unknown table", expr: `has_lookup("nonexistent", "key")`, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eval := mustCompileWithLookups(t, tc.expr, testTables())
			val, err := eval(map[string]interface{}{})
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got, ok := val.(bool)
			if !ok {
				t.Fatalf("expected bool, got %T", val)
			}
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestCELLookup_LookupOr(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected interface{}
	}{
		{name: "found", expr: `lookup_or("ports", "250401", "name", "default")`, expected: "San Ysidro"},
		{name: "not found", expr: `lookup_or("ports", "999999", "name", "default")`, expected: "default"},
		{name: "unknown table", expr: `lookup_or("nonexistent", "key", "field", "fallback")`, expected: "fallback"},
		{name: "missing field", expr: `lookup_or("ports", "250401", "nonexistent_field", "default")`, expected: "default"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eval := mustCompileWithLookups(t, tc.expr, testTables())
			val, err := eval(map[string]interface{}{})
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if val != tc.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tc.expected, tc.expected, val, val)
			}
		})
	}
}

func TestCELLookup_TypePreservation(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected interface{}
	}{
		{name: "float64", expr: `lookup("ports", "250401", "lat")`, expected: 32.5423},
		{name: "string", expr: `lookup("ports", "250401", "name")`, expected: "San Ysidro"},
		{name: "bool", expr: `lookup("ports", "250401", "active")`, expected: true},
		{name: "int64", expr: `lookup("ports", "250401", "zone")`, expected: int64(8)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eval := mustCompileWithLookups(t, tc.expr, testTables())
			val, err := eval(map[string]interface{}{})
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if val != tc.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tc.expected, tc.expected, val, val)
			}
		})
	}
}

func TestCELLookup_InFilterExpression(t *testing.T) {
	tables := testTables()

	tests := []struct {
		name     string
		record   map[string]interface{}
		expected bool
	}{
		{name: "known port", record: map[string]interface{}{"port_number": "250401"}, expected: true},
		{name: "unknown port", record: map[string]interface{}{"port_number": "999999"}, expected: false},
	}

	c := mustNewCompiler(t)
	prg, err := c.CompileExpressionWithLookups(`has_lookup("ports", record.port_number)`, tables)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			got, ok := val.(bool)
			if !ok {
				t.Fatalf("expected bool, got %T", val)
			}
			if got != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestCELLookup_InEntityMapping(t *testing.T) {
	tables := testTables()
	c := mustNewCompiler(t)

	// Simulate using lookup for latitude from a record's port_number
	prg, err := c.CompileExpressionWithLookups(`lookup("ports", record.port_number, "lat")`, tables)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}

	val, err := evalProgram(t, prg, map[string]interface{}{"port_number": "250401"})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	lat, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if lat != 32.5423 {
		t.Errorf("expected 32.5423, got %v", lat)
	}
}

func TestCELLookup_WithOtherCELFunctions(t *testing.T) {
	tables := testTables()
	c := mustNewCompiler(t)

	tests := []struct {
		name     string
		expr     string
		record   map[string]interface{}
		expected interface{}
	}{
		{
			name:     "string() coercion with lookup",
			expr:     `lookup("ports", string(record.port_number), "name")`,
			record:   map[string]interface{}{"port_number": "250401"},
			expected: "San Ysidro",
		},
		{
			name:     "lookup combined with string concat",
			expr:     `"Port: " + lookup("ports", "250401", "name")`,
			record:   map[string]interface{}{},
			expected: "Port: San Ysidro",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prg, err := c.CompileExpressionWithLookups(tc.expr, tables)
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}
			val, err := evalProgram(t, prg, tc.record)
			if err != nil {
				t.Fatalf("eval error: %v", err)
			}
			if val != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, val)
			}
		})
	}
}

// --- Integration: Source with lookup tables loads correctly ---

func TestLoader_LoadFile_WithLookupTables(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_lookup
source_type: test_lookup
layer_type: test_layer
display_name: "Test Lookup"
lookup_tables:
  - name: "ports"
    key_field: "code"
    entries:
      - code: "SYD"
        lat: -33.8688
        lon: 151.2093
      - code: "LAX"
        lat: 33.9425
        lon: -118.4081
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
filter: 'has_lookup("ports", record.code)'
entity:
  external_id: 'record.code'
  name: 'lookup("ports", record.code, "code")'
observation:
  latitude: 'lookup("ports", record.code, "lat")'
  longitude: 'lookup("ports", record.code, "lon")'
  timestamp: 'now()'
recording:
  mode: upsert
cache:
  ttl: "300s"
display:
  icon:
    shape: diamond
    scale: 1.2
  trail:
    color: "#ff9500"
  style:
    color: "#ff9500"
    point_size: 10
`
	dir := t.TempDir()
	path := filepath.Join(dir, "test_lookup.yaml")
	if err := os.WriteFile(path, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.LookupTables() == nil {
		t.Fatal("expected non-nil lookup tables")
	}
	if len(cs.LookupTables()) != 1 {
		t.Errorf("expected 1 lookup table, got %d", len(cs.LookupTables()))
	}
	if cs.Filter() == nil {
		t.Error("expected non-nil filter program")
	}
	if cs.ObservationLat() == nil {
		t.Error("expected non-nil observation latitude program")
	}
	if cs.ObservationLon() == nil {
		t.Error("expected non-nil observation longitude program")
	}
}

func TestLoader_LoadFile_LookupValidation(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "both entries and file",
			yaml: `
schema_version: 1
name: test_bad_lookup
source_type: test_bad_lookup
layer_type: test_layer
display_name: "Test Bad Lookup"
lookup_tables:
  - name: "bad"
    key_field: "id"
    entries:
      - id: "1"
    file: "some/file.json"
    format: "json"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`,
			wantErr: "exactly one of 'entries' or 'file' must be set",
		},
		{
			name: "file without format",
			yaml: `
schema_version: 1
name: test_no_format
source_type: test_no_format
layer_type: test_layer
display_name: "Test No Format"
lookup_tables:
  - name: "bad"
    key_field: "id"
    file: "some/file.json"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: '0.0'
  longitude: '0.0'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`,
			wantErr: "'format' is required when 'file' is set",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.yaml")
			if err := os.WriteFile(path, []byte(tc.yaml), 0644); err != nil {
				t.Fatalf("failed to write: %v", err)
			}
			loader := newTestLoader(t, false)
			_, err := loader.LoadFile(path)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

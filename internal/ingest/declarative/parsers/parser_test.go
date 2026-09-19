package parsers

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Factory tests
// ---------------------------------------------------------------------------

func TestNewParser(t *testing.T) {
	tests := []struct {
		name      string
		format    string
		wantErr   bool
		errSubstr string
	}{
		{name: "json", format: "json"},
		{name: "geojson", format: "geojson"},
		{name: "csv", format: "csv"},
		{name: "tle", format: "tle"},
		{name: "xml", format: "xml"},
		{name: "rss", format: "rss"},
		{name: "unsupported", format: "protobuf", wantErr: true, errSubstr: "unsupported"},
		{name: "empty", format: "", wantErr: true, errSubstr: "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewParser(tt.format)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
				if p != nil {
					t.Errorf("expected nil parser on error, got %T", p)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if p == nil {
					t.Fatalf("expected non-nil parser")
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON parser tests
// ---------------------------------------------------------------------------

func TestJSONParser_FlatArray(t *testing.T) {
	input := `[{"id": "a", "value": 1}, {"id": "b", "value": 2}]`
	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[0]["id"]; got != "a" {
		t.Errorf("records[0][id] = %v, want %q", got, "a")
	}
	if got := records[1]["id"]; got != "b" {
		t.Errorf("records[1][id] = %v, want %q", got, "b")
	}
	if got := records[0]["value"]; got != float64(1) {
		t.Errorf("records[0][value] = %v, want %v", got, float64(1))
	}
}

func TestJSONParser_NestedPath(t *testing.T) {
	input := `{
		"data": {
			"items": [
				{"name": "alpha"},
				{"name": "beta"},
				{"name": "gamma"}
			]
		}
	}`
	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		RecordsPath: "data.items",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	if got := records[0]["name"]; got != "alpha" {
		t.Errorf("records[0][name] = %v, want %q", got, "alpha")
	}
	if got := records[2]["name"]; got != "gamma" {
		t.Errorf("records[2][name] = %v, want %q", got, "gamma")
	}
}

func TestJSONParser_MissingPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		path  string
	}{
		{
			name:  "missing_leaf",
			input: `{"data": {}}`,
			path:  "data.items",
		},
		{
			name:  "missing_intermediate",
			input: `{"other": {}}`,
			path:  "data.items",
		},
		{
			name:  "empty_object",
			input: `{}`,
			path:  "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &JSONParser{}
			records, err := p.Parse([]byte(tt.input), ParserConfig{
				RecordsPath: tt.path,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(records) != 0 {
				t.Errorf("expected empty records, got %d", len(records))
			}
		})
	}
}

func TestJSONParser_MaxRecords(t *testing.T) {
	// Generate 20000 records.
	items := make([]map[string]interface{}, 20000)
	for i := 0; i < 20000; i++ {
		items[i] = map[string]interface{}{"index": i}
	}
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	p := &JSONParser{}
	records, err := p.Parse(data, ParserConfig{
		MaxRecords: 10000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 10000 {
		t.Fatalf("expected 10000 records, got %d", len(records))
	}
}

func TestJSONParser_DefaultMaxRecords(t *testing.T) {
	// Generate 15000 records (exceeds default 10000).
	items := make([]map[string]interface{}, 15000)
	for i := 0; i < 15000; i++ {
		items[i] = map[string]interface{}{"i": i}
	}
	data, err := json.Marshal(items)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	p := &JSONParser{}
	records, err := p.Parse(data, ParserConfig{
		MaxRecords: 0, // Should use default 10000.
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != DefaultMaxRecords {
		t.Fatalf("expected %d records, got %d", DefaultMaxRecords, len(records))
	}
}

func TestJSONParser_SingleObject(t *testing.T) {
	input := `{"key": "value"}`
	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0]["key"]; got != "value" {
		t.Errorf("records[0][key] = %v, want %q", got, "value")
	}
}

func TestJSONParser_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		errSubstr string
	}{
		{name: "empty", input: []byte{}, errSubstr: "empty input"},
		{name: "invalid_json", input: []byte(`{invalid`), errSubstr: "unmarshal failed"},
		{name: "top_level_string", input: []byte(`"hello"`), errSubstr: "unexpected top-level type"},
		{name: "top_level_number", input: []byte(`42`), errSubstr: "unexpected top-level type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &JSONParser{}
			_, err := p.Parse(tt.input, ParserConfig{})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON parser: ObjectToRecords transform tests
// ---------------------------------------------------------------------------

func TestJSONParser_ObjectToRecords(t *testing.T) {
	// SondeHub-style: {"serial1": {"lat": 34.7, "lon": -118.5}, "serial2": {"lat": 35.1, "lon": -117.0}}
	input := `{
		"X4716829": {"lat": 34.7, "lon": -118.5, "alt": 5000},
		"X4677065": {"lat": 35.1, "lon": -117.0, "alt": 8000}
	}`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ObjectToRecords: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify each record has lat/lon/alt (order is non-deterministic for maps).
	for _, r := range records {
		if _, ok := r["lat"]; !ok {
			t.Errorf("record missing 'lat' field: %v", r)
		}
		if _, ok := r["lon"]; !ok {
			t.Errorf("record missing 'lon' field: %v", r)
		}
	}
}

func TestJSONParser_ObjectToRecords_WithKeyField(t *testing.T) {
	input := `{
		"X4716829": {"lat": 34.7},
		"X4677065": {"lat": 35.1}
	}`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ObjectToRecords: true,
		ObjectKeyField:  "serial",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify the key was injected.
	keys := map[string]bool{}
	for _, r := range records {
		serial, ok := r["serial"].(string)
		if !ok {
			t.Errorf("record missing 'serial' field: %v", r)
			continue
		}
		keys[serial] = true
	}
	if !keys["X4716829"] || !keys["X4677065"] {
		t.Errorf("expected keys X4716829 and X4677065, got %v", keys)
	}
}

func TestJSONParser_ObjectToRecords_WithRecordsPath(t *testing.T) {
	input := `{
		"meta": {"count": 2},
		"data": {
			"station_a": {"temp": 22.5},
			"station_b": {"temp": 18.3}
		}
	}`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ObjectToRecords: true,
		RecordsPath:     "data",
		ObjectKeyField:  "station_id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	ids := map[string]bool{}
	for _, r := range records {
		ids[r["station_id"].(string)] = true
	}
	if !ids["station_a"] || !ids["station_b"] {
		t.Errorf("expected station_a and station_b, got %v", ids)
	}
}

func TestJSONParser_ObjectToRecords_SkipsNonObjects(t *testing.T) {
	input := `{
		"good": {"lat": 34.7},
		"bad_string": "not an object",
		"bad_number": 42
	}`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ObjectToRecords: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record (non-objects skipped), got %d", len(records))
	}
}

func TestJSONParser_ObjectToRecords_EmptyObject(t *testing.T) {
	p := &JSONParser{}
	records, err := p.Parse([]byte(`{}`), ParserConfig{
		ObjectToRecords: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestJSONParser_ObjectToRecords_MaxRecords(t *testing.T) {
	// Build an object with 100 entries.
	obj := make(map[string]interface{})
	for i := 0; i < 100; i++ {
		obj[fmt.Sprintf("key_%d", i)] = map[string]interface{}{"i": i}
	}
	data, _ := json.Marshal(obj)

	p := &JSONParser{}
	records, err := p.Parse(data, ParserConfig{
		ObjectToRecords: true,
		MaxRecords:      10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}
}

func TestJSONParser_ObjectToRecords_ErrorOnArray(t *testing.T) {
	p := &JSONParser{}
	_, err := p.Parse([]byte(`[1,2,3]`), ParserConfig{
		ObjectToRecords: true,
	})
	if err == nil {
		t.Fatal("expected error for array input with object_to_records")
	}
	if !strings.Contains(err.Error(), "expected object") {
		t.Errorf("error %q does not mention expected object", err.Error())
	}
}

func TestJSONParser_ObjectToRecords_MissingRecordsPath(t *testing.T) {
	p := &JSONParser{}
	records, err := p.Parse([]byte(`{"meta": "info"}`), ParserConfig{
		ObjectToRecords: true,
		RecordsPath:     "data.items",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records for missing path, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// JSON parser: ArrayOfArrays transform tests
// ---------------------------------------------------------------------------

func TestJSONParser_ArrayOfArrays(t *testing.T) {
	// NOAA SWPC-style: [["time_tag","Kp","a_running","station_count"], ["2026-03-01 00:00","2.67","12","8"]]
	input := `[
		["time_tag", "Kp", "a_running", "station_count"],
		["2026-03-01 00:00:00.000", "2.67", "12", "8"],
		["2026-03-01 03:00:00.000", "3.00", "15", "8"]
	]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	r0 := records[0]
	if got := r0["time_tag"]; got != "2026-03-01 00:00:00.000" {
		t.Errorf("r0[time_tag] = %v, want %q", got, "2026-03-01 00:00:00.000")
	}
	if got := r0["Kp"]; got != "2.67" {
		t.Errorf("r0[Kp] = %v, want %q", got, "2.67")
	}
	if got := r0["station_count"]; got != "8" {
		t.Errorf("r0[station_count] = %v, want %q", got, "8")
	}

	r1 := records[1]
	if got := r1["Kp"]; got != "3.00" {
		t.Errorf("r1[Kp] = %v, want %q", got, "3.00")
	}
}

func TestJSONParser_ArrayOfArrays_NumericValues(t *testing.T) {
	// Some APIs return numbers, not strings.
	input := `[
		["name", "value", "active"],
		["alpha", 42.5, true],
		["beta", 99.0, false]
	]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	if got := records[0]["name"]; got != "alpha" {
		t.Errorf("records[0][name] = %v, want %q", got, "alpha")
	}
	if got := records[0]["value"]; got != 42.5 {
		t.Errorf("records[0][value] = %v, want %v", got, 42.5)
	}
	if got := records[0]["active"]; got != true {
		t.Errorf("records[0][active] = %v, want %v", got, true)
	}
}

func TestJSONParser_ArrayOfArrays_WithRecordsPath(t *testing.T) {
	input := `{
		"meta": {"source": "NOAA"},
		"data": [
			["col_a", "col_b"],
			["val1", "val2"]
		]
	}`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
		RecordsPath:   "data",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0]["col_a"]; got != "val1" {
		t.Errorf("records[0][col_a] = %v, want %q", got, "val1")
	}
}

func TestJSONParser_ArrayOfArrays_HeaderOnly(t *testing.T) {
	// Only header row, no data.
	input := `[["col1", "col2"]]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestJSONParser_ArrayOfArrays_EmptyArray(t *testing.T) {
	p := &JSONParser{}
	records, err := p.Parse([]byte(`[]`), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestJSONParser_ArrayOfArrays_NonStringHeaders(t *testing.T) {
	// Non-string header values get fallback names.
	input := `[
		[42, null, "valid_name"],
		["a", "b", "c"]
	]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	r := records[0]
	if got := r["col_0"]; got != "a" {
		t.Errorf("r[col_0] = %v, want %q", got, "a")
	}
	if got := r["col_1"]; got != "b" {
		t.Errorf("r[col_1] = %v, want %q", got, "b")
	}
	if got := r["valid_name"]; got != "c" {
		t.Errorf("r[valid_name] = %v, want %q", got, "c")
	}
}

func TestJSONParser_ArrayOfArrays_MaxRecords(t *testing.T) {
	// Build array-of-arrays with 100 data rows.
	arr := make([]interface{}, 101)
	arr[0] = []interface{}{"id"}
	for i := 1; i <= 100; i++ {
		arr[i] = []interface{}{float64(i)}
	}
	data, _ := json.Marshal(arr)

	p := &JSONParser{}
	records, err := p.Parse(data, ParserConfig{
		ArrayOfArrays: true,
		MaxRecords:    10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}
}

func TestJSONParser_ArrayOfArrays_SkipsNonArrayRows(t *testing.T) {
	input := `[
		["name", "value"],
		["good", 1],
		"bad_row",
		["also_good", 2]
	]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records (non-array rows skipped), got %d", len(records))
	}
}

func TestJSONParser_ArrayOfArrays_ErrorOnObject(t *testing.T) {
	p := &JSONParser{}
	_, err := p.Parse([]byte(`{"key": "value"}`), ParserConfig{
		ArrayOfArrays: true,
	})
	if err == nil {
		t.Fatal("expected error for object input with array_of_arrays")
	}
	if !strings.Contains(err.Error(), "expected array") {
		t.Errorf("error %q does not mention expected array", err.Error())
	}
}

func TestJSONParser_ArrayOfArrays_UnevenRows(t *testing.T) {
	// Data rows shorter or longer than header.
	input := `[
		["a", "b", "c"],
		["val1"],
		["val1", "val2", "val3", "extra"]
	]`

	p := &JSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		ArrayOfArrays: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Short row: only "a" should be set.
	r0 := records[0]
	if got := r0["a"]; got != "val1" {
		t.Errorf("r0[a] = %v, want %q", got, "val1")
	}
	if _, exists := r0["b"]; exists {
		t.Errorf("r0[b] should not exist for short row, got %v", r0["b"])
	}

	// Long row: extra values beyond headers are dropped.
	r1 := records[1]
	if got := r1["a"]; got != "val1" {
		t.Errorf("r1[a] = %v, want %q", got, "val1")
	}
	if got := r1["c"]; got != "val3" {
		t.Errorf("r1[c] = %v, want %q", got, "val3")
	}
}

// ---------------------------------------------------------------------------
// GeoJSON parser tests
// ---------------------------------------------------------------------------

func TestGeoJSONParser_FeatureCollection(t *testing.T) {
	// USGS earthquake-style GeoJSON.
	input := `{
		"type": "FeatureCollection",
		"metadata": {"generated": 1234567890},
		"features": [
			{
				"type": "Feature",
				"id": "us7000abc1",
				"properties": {
					"mag": 4.5,
					"place": "10km NE of Somewhere",
					"time": 1700000000000,
					"type": "earthquake"
				},
				"geometry": {
					"type": "Point",
					"coordinates": [-118.5, 34.0, 10.0]
				}
			},
			{
				"type": "Feature",
				"id": "us7000abc2",
				"properties": {
					"mag": 2.1,
					"place": "5km S of Elsewhere"
				},
				"geometry": {
					"type": "Point",
					"coordinates": [-117.0, 33.5, 5.0]
				}
			}
		]
	}`

	p := &GeoJSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// Verify first record structure.
	r0 := records[0]
	if got := r0["type"]; got != "Feature" {
		t.Errorf("r0[type] = %v, want %q", got, "Feature")
	}
	if got := r0["id"]; got != "us7000abc1" {
		t.Errorf("r0[id] = %v, want %q", got, "us7000abc1")
	}
	if r0["geometry"] == nil {
		t.Errorf("r0[geometry] is nil, want non-nil")
	}
	if r0["properties"] == nil {
		t.Errorf("r0[properties] is nil, want non-nil")
	}

	// Verify properties are preserved as a map.
	props, ok := r0["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("r0[properties] is %T, want map[string]interface{}", r0["properties"])
	}
	if got := props["mag"]; got != 4.5 {
		t.Errorf("props[mag] = %v, want %v", got, 4.5)
	}
}

func TestGeoJSONParser_SingleFeature(t *testing.T) {
	input := `{
		"type": "Feature",
		"id": "feat1",
		"properties": {"name": "test"},
		"geometry": {"type": "Point", "coordinates": [0, 0]}
	}`

	p := &GeoJSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0]["id"]; got != "feat1" {
		t.Errorf("records[0][id] = %v, want %q", got, "feat1")
	}
}

func TestGeoJSONParser_MaxRecords(t *testing.T) {
	// Build a FeatureCollection with 5 features, limit to 3.
	features := make([]interface{}, 5)
	for i := 0; i < 5; i++ {
		features[i] = map[string]interface{}{
			"type":       "Feature",
			"id":         fmt.Sprintf("f%d", i),
			"properties": map[string]interface{}{},
			"geometry":   map[string]interface{}{"type": "Point", "coordinates": []interface{}{0.0, 0.0}},
		}
	}
	fc := map[string]interface{}{"type": "FeatureCollection", "features": features}
	data, err := json.Marshal(fc)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	p := &GeoJSONParser{}
	records, err := p.Parse(data, ParserConfig{MaxRecords: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
}

func TestGeoJSONParser_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		errSubstr string
	}{
		{name: "empty", input: []byte{}, errSubstr: "empty input"},
		{name: "invalid_json", input: []byte(`{bad`), errSubstr: "unmarshal failed"},
		{name: "unsupported_type", input: []byte(`{"type": "Geometry"}`), errSubstr: "unsupported type"},
		{name: "missing_type", input: []byte(`{"features": []}`), errSubstr: "unsupported type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &GeoJSONParser{}
			_, err := p.Parse(tt.input, ParserConfig{})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestGeoJSONParser_EmptyFeatures(t *testing.T) {
	input := `{"type": "FeatureCollection", "features": []}`
	p := &GeoJSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

func TestGeoJSONParser_MissingFeaturesKey(t *testing.T) {
	input := `{"type": "FeatureCollection"}`
	p := &GeoJSONParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// CSV parser tests
// ---------------------------------------------------------------------------

func TestCSVParser_WithHeader(t *testing.T) {
	input := "name,lat,lon\nAlpha,34.0,-118.5\nBeta,33.5,-117.0\n"

	p := &CSVParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[0]["name"]; got != "Alpha" {
		t.Errorf("records[0][name] = %v, want %q", got, "Alpha")
	}
	if got := records[0]["lat"]; got != "34.0" {
		t.Errorf("records[0][lat] = %v, want %q", got, "34.0")
	}
	if got := records[0]["lon"]; got != "-118.5" {
		t.Errorf("records[0][lon] = %v, want %q", got, "-118.5")
	}
	if got := records[1]["name"]; got != "Beta" {
		t.Errorf("records[1][name] = %v, want %q", got, "Beta")
	}
}

func TestCSVParser_CustomDelimiter(t *testing.T) {
	input := "name\tlat\tlon\nAlpha\t34.0\t-118.5\nBeta\t33.5\t-117.0\n"

	p := &CSVParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		CSVOptions: &CSVOpts{
			Delimiter: "\t",
			HasHeader: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[0]["name"]; got != "Alpha" {
		t.Errorf("records[0][name] = %v, want %q", got, "Alpha")
	}
	if got := records[0]["lat"]; got != "34.0" {
		t.Errorf("records[0][lat] = %v, want %q", got, "34.0")
	}
}

func TestCSVParser_NoHeader(t *testing.T) {
	input := "Alpha,34.0,-118.5\nBeta,33.5,-117.0\n"

	p := &CSVParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		CSVOptions: &CSVOpts{
			HasHeader: false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[0]["col_0"]; got != "Alpha" {
		t.Errorf("records[0][col_0] = %v, want %q", got, "Alpha")
	}
	if got := records[0]["col_1"]; got != "34.0" {
		t.Errorf("records[0][col_1] = %v, want %q", got, "34.0")
	}
}

func TestCSVParser_SkipLines(t *testing.T) {
	input := "# This is a comment\n# Another comment\nname,value\nfoo,bar\n"

	p := &CSVParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		CSVOptions: &CSVOpts{
			SkipLines: 2,
			HasHeader: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if got := records[0]["name"]; got != "foo" {
		t.Errorf("records[0][name] = %v, want %q", got, "foo")
	}
	if got := records[0]["value"]; got != "bar" {
		t.Errorf("records[0][value] = %v, want %q", got, "bar")
	}
}

func TestCSVParser_MaxRecords(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("id\n")
	for i := 0; i < 100; i++ {
		sb.WriteString(fmt.Sprintf("%d\n", i))
	}

	p := &CSVParser{}
	records, err := p.Parse([]byte(sb.String()), ParserConfig{
		MaxRecords: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 10 {
		t.Fatalf("expected 10 records, got %d", len(records))
	}
}

func TestCSVParser_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		errSubstr string
	}{
		{name: "empty", input: []byte{}, errSubstr: "empty input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &CSVParser{}
			_, err := p.Parse(tt.input, ParserConfig{})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

func TestCSVParser_SkipAllLines(t *testing.T) {
	input := "a,b\n1,2\n"
	p := &CSVParser{}
	records, err := p.Parse([]byte(input), ParserConfig{
		CSVOptions: &CSVOpts{
			SkipLines: 10,
			HasHeader: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected empty records, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// TLE parser tests
// ---------------------------------------------------------------------------

func TestTLEParser_ThreeLineBlocks(t *testing.T) {
	input := `ISS (ZARYA)
1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9995
2 25544  51.6400 208.9163 0006703 300.2572 209.3837 15.49560532423453
NOAA 19
1 33591U 09005A   24001.50000000  .00000042  00000-0  37723-4 0  9994
2 33591  99.1940 336.7550 0014549 186.8736 173.2228 14.12513498779901`

	p := &TLEParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}

	// First TLE: ISS.
	if got := records[0]["name"]; got != "ISS (ZARYA)" {
		t.Errorf("records[0][name] = %v, want %q", got, "ISS (ZARYA)")
	}
	line1, ok := records[0]["line1"].(string)
	if !ok {
		t.Fatalf("records[0][line1] is %T, want string", records[0]["line1"])
	}
	if !strings.Contains(line1, "25544U") {
		t.Errorf("records[0][line1] %q does not contain %q", line1, "25544U")
	}
	line2, ok := records[0]["line2"].(string)
	if !ok {
		t.Fatalf("records[0][line2] is %T, want string", records[0]["line2"])
	}
	if !strings.Contains(line2, "51.6400") {
		t.Errorf("records[0][line2] %q does not contain %q", line2, "51.6400")
	}
	if got := records[0]["norad_id"]; got != "25544" {
		t.Errorf("records[0][norad_id] = %v, want %q", got, "25544")
	}

	// Second TLE: NOAA 19.
	if got := records[1]["name"]; got != "NOAA 19" {
		t.Errorf("records[1][name] = %v, want %q", got, "NOAA 19")
	}
	if got := records[1]["norad_id"]; got != "33591" {
		t.Errorf("records[1][norad_id] = %v, want %q", got, "33591")
	}
}

func TestTLEParser_WithBlankLines(t *testing.T) {
	// Blank lines between entries should be tolerated.
	input := "\nISS (ZARYA)\n1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9995\n2 25544  51.6400 208.9163 0006703 300.2572 209.3837 15.49560532423453\n\n\nNOAA 19\n1 33591U 09005A   24001.50000000  .00000042  00000-0  37723-4 0  9994\n2 33591  99.1940 336.7550 0014549 186.8736 173.2228 14.12513498779901\n"

	p := &TLEParser{}
	records, err := p.Parse([]byte(input), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestTLEParser_MaxRecords(t *testing.T) {
	// Build 5 TLE entries, limit to 2.
	var sb strings.Builder
	for i := 0; i < 5; i++ {
		sb.WriteString(fmt.Sprintf("SAT %d\n", i))
		sb.WriteString(fmt.Sprintf("1 %05dU 24001A   24001.50000000  .00000000  00000-0  00000-0 0  9999\n", 10000+i))
		sb.WriteString(fmt.Sprintf("2 %05d  51.6400 208.9163 0006703 300.2572 209.3837 15.49560532423453\n", 10000+i))
	}

	p := &TLEParser{}
	records, err := p.Parse([]byte(sb.String()), ParserConfig{MaxRecords: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestTLEParser_Errors(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		errSubstr string
	}{
		{name: "empty", input: []byte{}, errSubstr: "empty input"},
		{name: "not_multiple_of_3", input: []byte("SAT\n1 25544U line1\n"), errSubstr: "not a multiple of 3"},
		{
			name:      "bad_line1",
			input:     []byte("SAT\nX 25544U 98067A   bad\n2 25544  51.6400 208.9163 0006703 300.2572 209.3837 15.49560532423453"),
			errSubstr: "expected line 1",
		},
		{
			name:      "bad_line2",
			input:     []byte("SAT\n1 25544U 98067A   24001.50000000  .00016717  00000-0  10270-3 0  9995\nX 25544  51.6400"),
			errSubstr: "expected line 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &TLEParser{}
			_, err := p.Parse(tt.input, ParserConfig{})
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tt.errSubstr)
			}
			if !strings.Contains(err.Error(), tt.errSubstr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// XML parser tests
// ---------------------------------------------------------------------------

func TestXMLParser_SelfClosingRoot(t *testing.T) {
	p := &XMLParser{}
	records, err := p.Parse([]byte("<root/>"), ParserConfig{})
	if err != nil {
		t.Fatalf("unexpected error parsing self-closing root: %v", err)
	}
	// Self-closing root with no records_path returns one empty record.
	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

// ---------------------------------------------------------------------------
// records_path tests
// ---------------------------------------------------------------------------

func TestRecordsPath_Deep(t *testing.T) {
	input := map[string]interface{}{
		"level1": map[string]interface{}{
			"level2": map[string]interface{}{
				"level3": []interface{}{
					map[string]interface{}{"id": "deep1"},
					map[string]interface{}{"id": "deep2"},
				},
			},
		},
	}

	records, err := ExtractByPath(input, "level1.level2.level3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if got := records[0]["id"]; got != "deep1" {
		t.Errorf("records[0][id] = %v, want %q", got, "deep1")
	}
	if got := records[1]["id"]; got != "deep2" {
		t.Errorf("records[1][id] = %v, want %q", got, "deep2")
	}
}

func TestRecordsPath_Missing(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
		path string
	}{
		{
			name: "missing_first_segment",
			data: map[string]interface{}{"other": "value"},
			path: "data.items",
		},
		{
			name: "missing_second_segment",
			data: map[string]interface{}{"data": map[string]interface{}{}},
			path: "data.items",
		},
		{
			name: "non_map_intermediate",
			data: map[string]interface{}{"data": "string_value"},
			path: "data.items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records, err := ExtractByPath(tt.data, tt.path)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(records) != 0 {
				t.Errorf("expected empty records, got %d", len(records))
			}
		})
	}
}

func TestRecordsPath_EmptyPath(t *testing.T) {
	input := []interface{}{
		map[string]interface{}{"id": "a"},
		map[string]interface{}{"id": "b"},
	}

	records, err := ExtractByPath(input, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
}

func TestRecordsPath_NotArray(t *testing.T) {
	input := map[string]interface{}{
		"data": "not_an_array",
	}

	_, err := ExtractByPath(input, "data")
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", "expected array")
	}
	if !strings.Contains(err.Error(), "expected array") {
		t.Errorf("error %q does not contain %q", err.Error(), "expected array")
	}
}

// ---------------------------------------------------------------------------
// truncateRecords / effectiveMaxRecords tests
// ---------------------------------------------------------------------------

func TestEffectiveMaxRecords(t *testing.T) {
	tests := []struct {
		name       string
		configured int
		want       int
	}{
		{name: "zero_uses_default", configured: 0, want: DefaultMaxRecords},
		{name: "negative_uses_default", configured: -1, want: DefaultMaxRecords},
		{name: "normal_value", configured: 500, want: 500},
		{name: "at_max", configured: MaxMaxRecords, want: MaxMaxRecords},
		{name: "above_max_clamped", configured: MaxMaxRecords + 1, want: MaxMaxRecords},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := effectiveMaxRecords(tt.configured); got != tt.want {
				t.Errorf("effectiveMaxRecords(%d) = %d, want %d", tt.configured, got, tt.want)
			}
		})
	}
}

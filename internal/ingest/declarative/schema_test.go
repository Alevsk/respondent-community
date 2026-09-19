package declarative

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestDuration_UnmarshalYAML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{name: "10 seconds", input: `"10s"`, expected: 10 * time.Second},
		{name: "5 minutes", input: `"5m"`, expected: 5 * time.Minute},
		{name: "1 hour 30 minutes", input: `"1h30m"`, expected: 90 * time.Minute},
		{name: "500 milliseconds", input: `"500ms"`, expected: 500 * time.Millisecond},
		{name: "invalid duration", input: `"not-a-duration"`, wantErr: true},
		{name: "empty string", input: `""`, wantErr: true},
		{name: "numeric value", input: `42`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var d Duration
			err := yaml.Unmarshal([]byte(tc.input), &d)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if d.Duration != tc.expected {
				t.Errorf("expected %v, got %v", tc.expected, d.Duration)
			}
		})
	}
}

func TestSourceNameValidator(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid lowercase", input: "usgs_earthquakes", wantErr: false},
		{name: "valid simple", input: "marine_ais", wantErr: false},
		{name: "valid single char", input: "a", wantErr: false},
		{name: "valid with digits", input: "source1_data2", wantErr: false},
		{name: "NATS wildcard", input: "*.>", wantErr: true},
		{name: "dots in name", input: "my.source", wantErr: true},
		{name: "uppercase", input: "MY_SOURCE", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "starts with digit", input: "1source", wantErr: true},
		{name: "starts with underscore", input: "_source", wantErr: true},
		{name: "too long (65 chars)", input: "a" + string(make([]byte, 64)), wantErr: true},
		{name: "max length (64 chars)", input: "a" + repeatChar('b', 63), wantErr: false},
		{name: "contains spaces", input: "my source", wantErr: true},
		{name: "contains dash", input: "my-source", wantErr: true},
	}

	type testStruct struct {
		Name string `validate:"source_name"`
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testStruct{Name: tc.input}
			err := v.Struct(s)
			if tc.wantErr && err == nil {
				t.Errorf("expected validation error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected validation error for %q: %v", tc.input, err)
			}
		})
	}
}

func TestRecordsPathValidator(t *testing.T) {
	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "single segment", input: "features", wantErr: false},
		{name: "two segments", input: "data.items", wantErr: false},
		{name: "max depth 8", input: "a.b.c.d.e.f.g.h", wantErr: false},
		{name: "depth 9 (too deep)", input: "a.b.c.d.e.f.g.h.i", wantErr: true},
		{name: "with digits", input: "data0.items1", wantErr: false},
		{name: "bracket access", input: "data[0]", wantErr: true},
		{name: "wildcard", input: "a.*.b", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "leading dot", input: ".data", wantErr: true},
		{name: "trailing dot", input: "data.", wantErr: true},
		{name: "hyphen in key", input: "broadcast-warn", wantErr: false},
		{name: "hyphen in nested key", input: "data.broadcast-warn", wantErr: false},
		{name: "space rejected", input: "data items", wantErr: true},
		{name: "slash rejected", input: "data/items", wantErr: true},
	}

	type testStruct struct {
		Path string `validate:"records_path"`
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := testStruct{Path: tc.input}
			err := v.Struct(s)
			if tc.wantErr && err == nil {
				t.Errorf("expected validation error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected validation error for %q: %v", tc.input, err)
			}
		})
	}
}

func TestSchemaValidation_Valid(t *testing.T) {
	yamlContent := `
schema_version: 1
name: usgs_earthquakes
source_type: usgs_earthquakes
layer_type: earthquakes
display_name: "USGS Earthquakes"
transport:
  type: http_poll
  url: "https://earthquake.usgs.gov/earthquakes/feed/v1.0/summary/all_hour.geojson"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: geojson
  records_path: "features"
filter: 'has(record.properties.mag) && double(record.properties.mag) >= 1.0'
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: 'record.geometry.coordinates[1]'
  longitude: 'record.geometry.coordinates[0]'
  altitude: 'record.geometry.coordinates[2]'
  timestamp: 'unix_ms(record.properties.time)'
  event_time: 'unix_ms(record.properties.time)'
  event_end: 'unix_ms(record.properties.time + 3600)'
field_mappings:
  - source: 'record.properties.mag'
    target: 'magnitude'
    type: 'float'
recording:
  mode: upsert
cache:
  ttl: "3600s"
display:
  icon:
    shape: ripple
    rotatable: false
    scale: 1.2
  trail:
    color: "#ff006e"
    width: 1.5
    opacity: 0.7
  style:
    color: "#ff006e"
    point_size: 10
`

	var def SourceDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}

	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	if err := v.Struct(def); err != nil {
		t.Fatalf("validation error: %v", err)
	}

	if def.Name != "usgs_earthquakes" {
		t.Errorf("expected name 'usgs_earthquakes', got %q", def.Name)
	}
	if def.Transport.Timeout.Duration != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", def.Transport.Timeout.Duration)
	}
	if def.Transport.Interval.Duration != 60*time.Second {
		t.Errorf("expected interval 60s, got %v", def.Transport.Interval.Duration)
	}
	if def.Display.Icon.Shape != "ripple" {
		t.Errorf("expected icon shape 'ripple', got %q", def.Display.Icon.Shape)
	}
	if def.Observation.EventTime != "unix_ms(record.properties.time)" {
		t.Errorf("expected event_time 'unix_ms(record.properties.time)', got %q", def.Observation.EventTime)
	}
	if def.Observation.EventEnd != "unix_ms(record.properties.time + 3600)" {
		t.Errorf("expected event_end 'unix_ms(record.properties.time + 3600)', got %q", def.Observation.EventEnd)
	}
	if len(def.FieldMappings) != 1 {
		t.Errorf("expected 1 field mapping, got %d", len(def.FieldMappings))
	} else {
		fm := def.FieldMappings[0]
		if fm.Source != "record.properties.mag" {
			t.Errorf("expected field mapping source 'record.properties.mag', got %q", fm.Source)
		}
		if fm.Target != "magnitude" {
			t.Errorf("expected field mapping target 'magnitude', got %q", fm.Target)
		}
		if fm.Type != "float" {
			t.Errorf("expected field mapping type 'float', got %q", fm.Type)
		}
	}
}

func TestSchemaValidation_MissingRequired(t *testing.T) {
	tests := []struct {
		name  string
		yaml  string
		field string
	}{
		{
			name: "missing source_type",
			yaml: `
schema_version: 1
name: test_source
layer_type: test
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
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
			field: "SourceType",
		},
		{
			name: "missing name",
			yaml: `
schema_version: 1
source_type: test
layer_type: test
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
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
			field: "Name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var def SourceDefinition
			if err := yaml.Unmarshal([]byte(tc.yaml), &def); err != nil {
				t.Fatalf("YAML parse error: %v", err)
			}

			v, err := NewValidator()
			if err != nil {
				t.Fatalf("failed to create validator: %v", err)
			}

			err = v.Struct(def)
			if err == nil {
				t.Fatalf("expected validation error for missing %s, got nil", tc.field)
			}
		})
	}
}

func TestSchemaValidation_InvalidFormat(t *testing.T) {
	yamlContent := `
schema_version: 1
name: test_source
source_type: test_source
layer_type: test
display_name: "Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: protobuf
entity:
  external_id: 'record.id'
  name: 'record.name'
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
`

	var def SourceDefinition
	if err := yaml.Unmarshal([]byte(yamlContent), &def); err != nil {
		t.Fatalf("YAML parse error: %v", err)
	}

	v, err := NewValidator()
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	err = v.Struct(def)
	if err == nil {
		t.Fatal("expected validation error for unsupported format 'protobuf', got nil")
	}
}

func TestSchemaValidation_HistorySpec(t *testing.T) {
	tests := []struct {
		name              string
		yamlHistory       string
		wantNil           bool
		wantLookbackHours float64
		wantSpanHours     float64
	}{
		{
			name: "history block present with one year lookback and one week span",
			yamlHistory: `
history:
  max_lookback: "8760h"
  max_range_span: "168h"
`,
			wantNil:           false,
			wantLookbackHours: 8760,
			wantSpanHours:     168,
		},
		{
			name: "history block present with small window",
			yamlHistory: `
history:
  max_lookback: "24h"
  max_range_span: "6h"
`,
			wantNil:           false,
			wantLookbackHours: 24,
			wantSpanHours:     6,
		},
		{
			name:        "no history block",
			yamlHistory: "",
			wantNil:     true,
		},
	}

	baseYAML := `
schema_version: 1
name: test_hist_source
source_type: test_hist_source
layer_type: test_hist_layer
display_name: "Test History"
transport:
  type: http_poll
  url: "https://example.com/api"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
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
`

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fullYAML := baseYAML + tc.yamlHistory

			var def SourceDefinition
			if err := yaml.Unmarshal([]byte(fullYAML), &def); err != nil {
				t.Fatalf("YAML parse error: %v", err)
			}

			if tc.wantNil {
				if def.History != nil {
					t.Errorf("History: got %+v, want nil", def.History)
				}
				return
			}

			if def.History == nil {
				t.Fatal("History: got nil, want non-nil HistorySpec")
			}
			if def.History.MaxLookback.Hours() != tc.wantLookbackHours {
				t.Errorf("MaxLookback hours = %v, want %v",
					def.History.MaxLookback.Hours(), tc.wantLookbackHours)
			}
			if def.History.MaxRangeSpan.Hours() != tc.wantSpanHours {
				t.Errorf("MaxRangeSpan hours = %v, want %v",
					def.History.MaxRangeSpan.Hours(), tc.wantSpanHours)
			}
		})
	}
}

// repeatChar creates a string of n repeated characters.
func repeatChar(c byte, n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}

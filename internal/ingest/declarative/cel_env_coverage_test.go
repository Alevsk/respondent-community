package declarative

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// highEccLine1 and highEccLine2 are TLE strings for a stale ISS-like TLE with
// a very high drag term (ndot = 0.10000000 rev/day^2), causing rapid orbital
// decay that produces a negative altitude when propagated from the distant epoch
// (day 1 of year 2000) to the current date. Both lines pass tleFormatValid checks
// (length >= 69, correct "1 "/"2 " prefixes) but PropagateTLE returns an
// "unrealistic altitude" error, exercising the 0.0 fallback path in the CEL SGP4 functions.
const (
	highEccLine1 = "1 25544U 98067A   00001.00000000  .10000000  00000-0  10270-3 0  9007"
	highEccLine2 = "2 25544  51.6400 100.0000 0007417  40.0000 320.0000 15.49000000400000"
)

func TestLoadFile_InvalidFilterCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_filter
source_type: bad_filter
layer_type: bad_filter_layer
display_name: "Bad Filter"
filter: '!!! invalid syntax @@@'
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
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
	yamlPath := filepath.Join(dir, "bad_filter.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid filter CEL, got nil")
	}
}

func TestLoadFile_InvalidEntityIDCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_entity_id
source_type: bad_entity_id
layer_type: bad_entity_id_layer
display_name: "Bad Entity ID"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: '!!! bad @@@'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
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
	yamlPath := filepath.Join(dir, "bad_entity_id.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid entity.external_id CEL, got nil")
	}
}

func TestLoadFile_InvalidEntityNameCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_entity_name
source_type: bad_entity_name
layer_type: bad_entity_name_layer
display_name: "Bad Entity Name"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: '!!! bad name @@@'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
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
	yamlPath := filepath.Join(dir, "bad_entity_name.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid entity.name CEL, got nil")
	}
}

func TestLoadFile_InvalidObservationLatCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_obs_lat
source_type: bad_obs_lat
layer_type: bad_obs_lat_layer
display_name: "Bad Obs Lat"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '!!! bad lat @@@'
  longitude: 'record.lon'
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
	yamlPath := filepath.Join(dir, "bad_obs_lat.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid observation.latitude CEL, got nil")
	}
}

func TestLoadFile_InvalidObservationLonCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_obs_lon
source_type: bad_obs_lon
layer_type: bad_obs_lon_layer
display_name: "Bad Obs Lon"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: '!!! bad lon @@@'
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
	yamlPath := filepath.Join(dir, "bad_obs_lon.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid observation.longitude CEL, got nil")
	}
}

func TestLoadFile_InvalidObservationTsCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_obs_ts
source_type: bad_obs_ts
layer_type: bad_obs_ts_layer
display_name: "Bad Obs Ts"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: '!!! bad ts @@@'
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
	yamlPath := filepath.Join(dir, "bad_obs_ts.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid observation.timestamp CEL, got nil")
	}
}

func TestLoadFile_InvalidContentHashCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_content_hash
source_type: bad_content_hash
layer_type: bad_content_hash_layer
display_name: "Bad Content Hash"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
  content_hash: '!!! bad hash @@@'
recording:
  mode: upsert
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
	yamlPath := filepath.Join(dir, "bad_content_hash.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid content_hash CEL, got nil")
	}
}

func TestLoadFile_InvalidEventTimeCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_event_time
source_type: bad_event_time
layer_type: bad_event_time_layer
display_name: "Bad Event Time"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
  event_time: '!!! bad event time @@@'
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
	yamlPath := filepath.Join(dir, "bad_event_time.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid event_time CEL, got nil")
	}
}

func TestLoadFile_InvalidEventEndCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_event_end
source_type: bad_event_end
layer_type: bad_event_end_layer
display_name: "Bad Event End"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
  event_end: '!!! bad event end @@@'
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
	yamlPath := filepath.Join(dir, "bad_event_end.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid event_end CEL, got nil")
	}
}

func TestEvalString_Int64Branch(t *testing.T) {
	compiler, _ := NewCELCompiler()
	// Use int() to force an int64 result.
	prg, err := compiler.CompileExpression("int(record.val)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"val": float64(42)}}
	got, err := evalString(prg, activation, 1024)
	if err != nil {
		t.Fatalf("evalString: %v", err)
	}
	if got != "42" {
		t.Errorf("evalString int64 = %q, want %q", got, "42")
	}
}

func TestEvalString_BoolBranch(t *testing.T) {
	compiler, _ := NewCELCompiler()
	prg, err := compiler.CompileExpression("record.flag")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"flag": true}}
	got, err := evalString(prg, activation, 1024)
	if err != nil {
		t.Fatalf("evalString: %v", err)
	}
	if got != "true" {
		t.Errorf("evalString bool = %q, want %q", got, "true")
	}
}

func TestLoadFile_InvalidMetadataCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_metadata
source_type: bad_metadata
layer_type: bad_metadata_layer
display_name: "Bad Metadata"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
  metadata:
    bad_key: '!!! bad meta @@@'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
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
	yamlPath := filepath.Join(dir, "bad_metadata.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid entity.metadata CEL, got nil")
	}
}

func TestLoadFile_InvalidAltitudeCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_altitude
source_type: bad_altitude
layer_type: bad_altitude_layer
display_name: "Bad Altitude"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  altitude: '!!! bad altitude @@@'
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
	yamlPath := filepath.Join(dir, "bad_altitude.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid observation.altitude CEL, got nil")
	}
}

func TestLoadFile_InvalidVelocityCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_velocity
source_type: bad_velocity
layer_type: bad_velocity_layer
display_name: "Bad Velocity"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
  velocity:
    speed: '!!! bad velocity @@@'
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
	yamlPath := filepath.Join(dir, "bad_velocity.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid velocity CEL, got nil")
	}
}

func TestLoadFile_InvalidObsMetadataCEL(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_obs_meta
source_type: bad_obs_meta
layer_type: bad_obs_meta_layer
display_name: "Bad Obs Meta"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
  metadata:
    bad_obs_key: '!!! bad obs meta @@@'
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
	yamlPath := filepath.Join(dir, "bad_obs_meta.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid observation.metadata CEL, got nil")
	}
}

func TestCompileStopWhen_InvalidCELSyntax2(t *testing.T) {
	c := mustNewCompiler(t)
	_, err := c.CompileStopWhen("!!! invalid @@@ syntax")
	if err == nil {
		t.Error("expected error for invalid stop_when CEL, got nil")
	}
}

func TestEvalFloat_Int64Branch(t *testing.T) {
	c := mustCELCompiler(t)
	prg, err := c.CompileExpression("int(record.val)")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{"record": map[string]interface{}{"val": float64(42)}}
	got, err := evalFloat(prg, activation)
	if err != nil {
		t.Fatalf("evalFloat with int64 value: %v", err)
	}
	if got != 42.0 {
		t.Errorf("expected 42.0, got %v", got)
	}
}

func TestEvalString_ConvertToTypeFallback(t *testing.T) {
	compiler := mustNewCompiler(t)

	// 'now()' returns a CEL Timestamp, which is not string/int64/float64/bool.
	// The ConvertToType path converts it to string representation.
	prg, err := compiler.CompileExpression("now()")
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}

	result, err := evalString(prg, map[string]interface{}{"record": map[string]interface{}{}}, 0)
	if err != nil {
		t.Fatalf("evalString: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty timestamp string")
	}
}

func TestCELSGP4Lat_PropagationError(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_lat(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	activation := map[string]interface{}{
		"record": map[string]interface{}{
			"line1": highEccLine1,
			"line2": highEccLine2,
		},
	}
	val, err := evalProgram(t, prg, activation["record"].(map[string]interface{}))
	if err != nil {
		t.Fatalf("eval error (should not propagate, should return 0.0): %v", err)
	}
	got, ok := val.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T", val)
	}
	if got != 0.0 {
		t.Errorf("expected 0.0 for propagation error, got %f", got)
	}
}

func TestCELSGP4Lon_PropagationError(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_lon(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	val, err := evalProgram(t, prg, map[string]interface{}{
		"line1": highEccLine1,
		"line2": highEccLine2,
	})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	got, _ := val.(float64)
	if got != 0.0 {
		t.Errorf("expected 0.0 for propagation error, got %f", got)
	}
}

func TestCELSGP4AltM_PropagationError(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_alt_m(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	val, err := evalProgram(t, prg, map[string]interface{}{
		"line1": highEccLine1,
		"line2": highEccLine2,
	})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	got, _ := val.(float64)
	if got != 0.0 {
		t.Errorf("expected 0.0 for propagation error, got %f", got)
	}
}

func TestCELSGP4VelMps_PropagationError(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`sgp4_vel_mps(record.line1, record.line2)`)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	val, err := evalProgram(t, prg, map[string]interface{}{
		"line1": highEccLine1,
		"line2": highEccLine2,
	})
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}
	got, _ := val.(float64)
	if got != 0.0 {
		t.Errorf("expected 0.0 for propagation error, got %f", got)
	}
}

func TestCELParseISO8601_NegativeOffset(t *testing.T) {
	c := mustNewCompiler(t)
	prg, err := c.CompileExpression(`parse_iso8601(record.ts)`)
	if err != nil {
		t.Fatalf("CompileExpression: %v", err)
	}
	// A datetime string with no Z/z/+ characters; the '-' at position 19
	// (after the date part, i.e., i > 10) marks a negative UTC offset.
	// "2026-03-25T10:00:00-05:00" — outer if fires (no Z/z/+), inner loop
	// finds '-' at index 19 > 10, so hasTimezone=true and no "Z" is appended.
	// RFC3339 parsing succeeds.
	activation := map[string]interface{}{
		"record": map[string]interface{}{
			"ts": "2026-03-25T10:00:00-05:00",
		},
	}
	ts, err := evalTimestamp(prg, activation)
	if err != nil {
		t.Fatalf("evalTimestamp: %v", err)
	}
	// The offset is -05:00, so UTC time = 15:00:00 UTC
	expected := time.Date(2026, 3, 25, 15, 0, 0, 0, time.UTC)
	if !ts.Equal(expected) {
		t.Errorf("parse_iso8601 negative offset: got %v, want %v", ts, expected)
	}
}

package declarative

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestTleFormatValid_WrongLine1Prefix(t *testing.T) {
	// line1 has correct length but starts with '2' instead of '1'
	line1 := "2 25544U 98067A   21001.50000000  .00002182  00000-0  47594-4 0  9994"
	line2 := "2 25544  51.6461 339.3984 0001763 190.4136 169.6990 15.49005108265286"
	if tleFormatValid(line1, line2) {
		t.Error("expected false when line1[0] != '1'")
	}
}

func TestTleFormatValid_WrongLine2Prefix(t *testing.T) {
	// line2 has correct length but starts with '1' instead of '2'
	line1 := "1 25544U 98067A   21001.50000000  .00002182  00000-0  47594-4 0  9994"
	line2 := "1 25544  51.6461 339.3984 0001763 190.4136 169.6990 15.49005108265286"
	if tleFormatValid(line1, line2) {
		t.Error("expected false when line2[0] != '2'")
	}
}

func TestSchemaDuration_InvalidValue(t *testing.T) {
	dir := t.TempDir()
	// Craft a YAML with an invalid duration in transport.timeout to trigger
	// Duration.UnmarshalYAML returning an error.
	yaml := `schema_version: 1
name: bad_duration_test
source_type: bad_duration_test
layer_type: bad_duration_test_layer
display_name: "Bad Duration"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "not-a-duration"
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
	yamlPath := filepath.Join(dir, "bad_duration.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid duration string, got nil")
	}
}

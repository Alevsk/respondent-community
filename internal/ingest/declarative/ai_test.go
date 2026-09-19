package declarative

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// baseValidSourceYAML is a minimal valid v1 source YAML used as the foundation
// for AI-related test cases. The `ai:` block is appended by each test.
const baseValidSourceYAML = `
schema_version: 1
name: test_ai_source
source_type: test_ai_source
layer_type: test_layer
display_name: "Test AI Source"
transport:
  type: http_poll
  url: "https://example.com/api/data"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "data.items"
entity:
  external_id: 'record.id'
  name: 'record.id'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'unix_ms(record.time)'
recording:
  mode: append
cache:
  ttl: "300s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#00ff9d"
    width: 1.5
    opacity: 0.7
  style:
    color: "#00ff9d"
    point_size: 8
`

// writeTestYAML writes YAML content to a temp file and returns the path.
func writeTestYAML(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", name, err)
	}
	return path
}

func TestLoader_LoadFile_AIBlockParsesCorrectly(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations:
    - name: classify_entity
      prompt: "Classify this entity based on its metadata."
      filter: 'has(record.type) && record.type == "aircraft"'
      output_schema:
        type: object
        properties:
          category:
            type: string
          confidence:
            type: number
        required:
          - category
          - confidence
      output_target: entity
      max_tokens: 256
      temperature: 0.3
`
	path := writeTestYAML(t, "ai_valid.yaml", yamlContent)

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error loading source with ai block: %v", err)
	}

	def := cs.Definition()
	if def.AI == nil {
		t.Fatal("expected non-nil AI config")
	}
	if !def.AI.Enabled {
		t.Error("expected AI to be enabled")
	}
	if len(def.AI.Operations) != 1 {
		t.Fatalf("expected 1 AI operation, got %d", len(def.AI.Operations))
	}

	op := def.AI.Operations[0]
	if op.Name != "classify_entity" {
		t.Errorf("expected operation name 'classify_entity', got %q", op.Name)
	}
	if op.Filter == "" {
		t.Error("expected non-empty filter on AI operation")
	}
	if op.OutputSchema == nil {
		t.Error("expected non-nil output_schema on AI operation")
	}
	if op.MaxTokens != 256 {
		t.Errorf("expected max_tokens=256, got %d", op.MaxTokens)
	}
	if op.Temperature != 0.3 {
		t.Errorf("expected temperature=0.3, got %f", op.Temperature)
	}

	// Verify the output schema was registered in the schema registry.
	schemaKey := "test_ai_source.classify_entity"
	schemaJSON := loader.SchemaRegistry().SchemaJSON(schemaKey)
	if schemaJSON == "" {
		t.Errorf("expected schema registered under key %q, got empty", schemaKey)
	}
}

func TestLoader_LoadFile_NoAIBlockAIIsNil(t *testing.T) {
	path := writeTestYAML(t, "no_ai.yaml", baseValidSourceYAML)

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.Definition().AI != nil {
		t.Error("expected AI to be nil when no ai: block is present")
	}
}

func TestLoader_LoadFile_AIDisabled(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: false
  operations:
    - name: disabled_op
      prompt: "This should not be validated deeply."
`
	path := writeTestYAML(t, "ai_disabled.yaml", yamlContent)

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	def := cs.Definition()
	if def.AI == nil {
		t.Fatal("expected non-nil AI config even when disabled")
	}
	if def.AI.Enabled {
		t.Error("expected AI to be disabled")
	}

	// Schema registry should be empty since AI is disabled.
	schemaJSON := loader.SchemaRegistry().SchemaJSON("test_ai_source.disabled_op")
	if schemaJSON != "" {
		t.Error("expected no schema registered when AI is disabled")
	}
}

func TestLoader_LoadFile_AIInvalidCELFilter(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations:
    - name: bad_filter_op
      prompt: "Analyze entity."
      filter: 'this is not valid CEL %%% !!!'
`
	path := writeTestYAML(t, "ai_bad_cel.yaml", yamlContent)

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid CEL filter in AI operation, got nil")
	}
	if !strings.Contains(err.Error(), "ai.operations[0].filter") {
		t.Errorf("error should mention ai.operations[0].filter, got: %v", err)
	}
}

func TestLoader_LoadFile_AIOutputSchemaRegistered(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations:
    - name: enrich_metadata
      prompt: "Enrich the entity metadata."
      output_schema:
        type: object
        properties:
          threat_level:
            type: string
            enum:
              - low
              - medium
              - high
        required:
          - threat_level
    - name: classify_type
      prompt: "Classify the entity type."
      filter: 'record.type == "unknown"'
      output_schema:
        type: object
        properties:
          classification:
            type: string
        required:
          - classification
`
	path := writeTestYAML(t, "ai_schemas.yaml", yamlContent)

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both schemas should be registered.
	reg := loader.SchemaRegistry()

	enrichKey := "test_ai_source.enrich_metadata"
	if got := reg.SchemaJSON(enrichKey); got == "" {
		t.Errorf("expected schema registered for %q", enrichKey)
	}

	classifyKey := "test_ai_source.classify_type"
	if got := reg.SchemaJSON(classifyKey); got == "" {
		t.Errorf("expected schema registered for %q", classifyKey)
	}

	// Validate a conforming JSON document against the registered schema.
	if err := reg.Validate(enrichKey, []byte(`{"threat_level":"high"}`)); err != nil {
		t.Errorf("expected valid JSON to pass schema validation: %v", err)
	}

	// Validate a non-conforming JSON document is rejected.
	if err := reg.Validate(enrichKey, []byte(`{"wrong_field":"value"}`)); err == nil {
		t.Error("expected schema validation to fail for non-conforming JSON")
	}
}

func TestLoader_LoadFile_AIValidCELFilter(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations:
    - name: filter_op
      prompt: "Process filtered records."
      filter: 'has(record.status) && record.status == "active"'
`
	path := writeTestYAML(t, "ai_valid_cel.yaml", yamlContent)

	loader := newTestLoader(t, false)
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cs.Definition().AI == nil {
		t.Fatal("expected non-nil AI config")
	}
	if len(cs.Definition().AI.Operations) != 1 {
		t.Fatalf("expected 1 operation, got %d", len(cs.Definition().AI.Operations))
	}
}

func TestLoader_LoadFile_AIEnabledNoOperationsRejected(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations: []
`
	path := writeTestYAML(t, "ai_no_ops.yaml", yamlContent)

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for ai.enabled=true with no operations, got nil")
	}
	if !strings.Contains(err.Error(), "no operations defined") {
		t.Errorf("error should mention 'no operations defined', got: %v", err)
	}
}

func TestLoader_LoadFile_AIMultipleOperationsSecondFilterInvalid(t *testing.T) {
	yamlContent := baseValidSourceYAML + `
ai:
  enabled: true
  operations:
    - name: good_op
      prompt: "Good operation."
      filter: 'record.active == true'
    - name: bad_op
      prompt: "Bad operation."
      filter: '??? invalid CEL'
`
	path := writeTestYAML(t, "ai_second_bad.yaml", yamlContent)

	loader := newTestLoader(t, false)
	_, err := loader.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid CEL in second AI operation, got nil")
	}
	if !strings.Contains(err.Error(), "ai.operations[1].filter") {
		t.Errorf("error should mention ai.operations[1].filter, got: %v", err)
	}
}

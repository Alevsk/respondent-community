package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func classifySchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []any{"category", "confidence"},
		"properties": map[string]any{
			"category": map[string]any{
				"type": "string",
				"enum": []any{"military", "commercial", "private", "unknown"},
			},
			"confidence": map[string]any{
				"type":    "number",
				"minimum": 0,
				"maximum": 1,
			},
		},
	}
}

func TestRegisterFromYAML_ValidSchema(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	assert.NoError(t, err)
}

func TestRegisterFromYAML_InvalidSchemaRejects(t *testing.T) {
	r := NewRegistry()
	// A schema where "type" is an invalid value should fail compilation.
	invalidSchema := map[string]any{
		"type": "not_a_valid_type",
	}
	err := r.RegisterFromYAML("bad_schema", invalidSchema)
	require.Error(t, err)
}

func TestValidate_PassesForValidJSON(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	validJSON := `{"category": "military", "confidence": 0.95}`
	err = r.Validate("classify", []byte(validJSON))
	assert.NoError(t, err)
}

func TestValidate_FailsForMissingRequiredField(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	// Missing "confidence" field.
	missingField := `{"category": "military"}`
	err = r.Validate("classify", []byte(missingField))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "confidence")
}

func TestValidate_FailsForWrongType(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	// "confidence" should be a number, not a string.
	wrongType := `{"category": "military", "confidence": "high"}`
	err = r.Validate("classify", []byte(wrongType))
	require.Error(t, err)
}

func TestValidate_FailsForEnumViolation(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	// "cargo" is not in the enum.
	enumViolation := `{"category": "cargo", "confidence": 0.8}`
	err = r.Validate("classify", []byte(enumViolation))
	require.Error(t, err)
}

func TestValidate_FailsForMinMaxViolation(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	// confidence > 1 violates maximum.
	overMax := `{"category": "military", "confidence": 1.5}`
	err = r.Validate("classify", []byte(overMax))
	require.Error(t, err)

	// confidence < 0 violates minimum.
	underMin := `{"category": "military", "confidence": -0.1}`
	err = r.Validate("classify", []byte(underMin))
	require.Error(t, err)
}

func TestSchemaJSON_ReturnsCorrectJSON(t *testing.T) {
	r := NewRegistry()
	schemaMap := classifySchema()
	err := r.RegisterFromYAML("classify", schemaMap)
	require.NoError(t, err)

	jsonStr := r.SchemaJSON("classify")
	require.NotEmpty(t, jsonStr)

	// Verify the JSON is valid and round-trips.
	var parsed map[string]any
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "object", parsed["type"])
}

func TestSchemaJSON_UnknownKeyReturnsEmpty(t *testing.T) {
	r := NewRegistry()
	result := r.SchemaJSON("nonexistent")
	assert.Empty(t, result)
}

func TestValidate_UnknownKeyReturnsError(t *testing.T) {
	r := NewRegistry()
	err := r.Validate("nonexistent", []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema not found")
}

func TestValidateAndExtract_WithMarkdownFencedJSON(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	fenced := "```json\n{\"category\": \"commercial\", \"confidence\": 0.88}\n```"
	result, err := r.ValidateAndExtract("classify", fenced)
	require.NoError(t, err)
	assert.Equal(t, "commercial", result["category"])
}

func TestValidateAndExtract_WithPlainJSON(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	plain := `{"category": "private", "confidence": 0.5}`
	result, err := r.ValidateAndExtract("classify", plain)
	require.NoError(t, err)
	assert.Equal(t, "private", result["category"])
}

func TestValidateAndExtract_InvalidJSONFails(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	_, err = r.ValidateAndExtract("classify", "not json at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "extract JSON")
}

func TestValidateAndExtract_SchemaViolationFails(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	// Valid JSON but missing required field.
	invalidData := `{"category": "military"}`
	_, err = r.ValidateAndExtract("classify", invalidData)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation")
}

func TestValidateAndExtract_EmptyResponseFails(t *testing.T) {
	r := NewRegistry()
	err := r.RegisterFromYAML("classify", classifySchema())
	require.NoError(t, err)

	_, err = r.ValidateAndExtract("classify", "")
	require.Error(t, err)
}

func TestValidate_IntegerSchema(t *testing.T) {
	r := NewRegistry()
	intSchema := map[string]any{
		"type":     "object",
		"required": []any{"count"},
		"properties": map[string]any{
			"count": map[string]any{
				"type":    "integer",
				"minimum": 0,
				"maximum": 100,
			},
		},
	}
	err := r.RegisterFromYAML("counter", intSchema)
	require.NoError(t, err)

	err = r.Validate("counter", []byte(`{"count": 42}`))
	assert.NoError(t, err)

	err = r.Validate("counter", []byte(`{"count": 150}`))
	require.Error(t, err)
}

func TestRegisterFromYAML_OverwritesExistingKey(t *testing.T) {
	r := NewRegistry()

	schema1 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "string"},
		},
	}
	schema2 := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"b": map[string]any{"type": "integer"},
		},
	}

	err := r.RegisterFromYAML("test", schema1)
	require.NoError(t, err)

	err = r.RegisterFromYAML("test", schema2)
	require.NoError(t, err)

	// The second schema should be active now.
	jsonStr := r.SchemaJSON("test")
	var parsed map[string]any
	err = json.Unmarshal([]byte(jsonStr), &parsed)
	require.NoError(t, err)

	props := parsed["properties"].(map[string]any)
	_, hasB := props["b"]
	assert.True(t, hasB, "schema should have property 'b' from the second registration")
}

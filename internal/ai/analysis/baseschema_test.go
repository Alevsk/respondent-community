package analysis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeBaseSchema_ResultsPathPattern(t *testing.T) {
	// Simulates a typical analysis.d/ output_schema with results_path="assessments".
	schema := map[string]any{
		"type":     "object",
		"required": []any{"assessments", "summary"},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"assessments": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"significance", "assessment"},
					"properties": map[string]any{
						"significance": map[string]any{"type": "string"},
						"assessment":   map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	MergeBaseSchema(schema, "assessments")

	// Attention should be injected into items.properties.
	items := schema["properties"].(map[string]any)["assessments"].(map[string]any)["items"].(map[string]any)
	props := items["properties"].(map[string]any)
	assert.Contains(t, props, "attention", "attention should be added to items.properties")

	attentionDef := props["attention"].(map[string]any)
	assert.Equal(t, "string", attentionDef["type"])
	assert.Contains(t, attentionDef["enum"].([]any), "critical")

	// Required array should include "attention".
	req := items["required"].([]any)
	assert.Contains(t, req, "attention")

	// Top-level properties should NOT have attention.
	topProps := schema["properties"].(map[string]any)
	_, hasTopAttention := topProps["attention"]
	assert.False(t, hasTopAttention, "attention should not be at top level when results_path is set")
}

func TestMergeBaseSchema_SingleResultPattern(t *testing.T) {
	// Simulates a schema without results_path.
	schema := map[string]any{
		"type":     "object",
		"required": []any{"result"},
		"properties": map[string]any{
			"result": map[string]any{"type": "string"},
		},
	}

	MergeBaseSchema(schema, "")

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "attention")

	req := schema["required"].([]any)
	assert.Contains(t, req, "attention")
}

func TestMergeBaseSchema_DoesNotOverwriteExistingAttention(t *testing.T) {
	// If YAML author explicitly defined "attention", don't overwrite.
	customAttention := map[string]any{
		"type": "string",
		"enum": []any{"custom_value"},
	}

	schema := map[string]any{
		"type":     "object",
		"required": []any{"result"},
		"properties": map[string]any{
			"result":    map[string]any{"type": "string"},
			"attention": customAttention,
		},
	}

	MergeBaseSchema(schema, "")

	props := schema["properties"].(map[string]any)
	// Should preserve the custom definition.
	assert.Equal(t, customAttention, props["attention"])
}

func TestMergeBaseSchema_NoPropertiesNoPanic(t *testing.T) {
	// Schema without properties should not panic.
	schema := map[string]any{
		"type": "object",
	}
	require.NotPanics(t, func() {
		MergeBaseSchema(schema, "")
	})
}

func TestMergeBaseSchema_MissingArrayItemsNoPanic(t *testing.T) {
	// results_path points to a field without items — should not panic.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{"type": "array"},
		},
	}
	require.NotPanics(t, func() {
		MergeBaseSchema(schema, "data")
	})
}

func TestExtractAttention_Valid(t *testing.T) {
	for _, level := range []string{"info", "low", "medium", "high", "critical"} {
		result := map[string]any{"attention": level}
		got := extractAttention(result)
		require.NotNil(t, got, "expected non-nil for level %q", level)
		assert.Equal(t, level, *got)
	}
}

func TestExtractAttention_Missing(t *testing.T) {
	result := map[string]any{"severity": "high"}
	got := extractAttention(result)
	assert.Nil(t, got)
}

func TestExtractAttention_Invalid(t *testing.T) {
	result := map[string]any{"attention": "extreme"}
	got := extractAttention(result)
	assert.Nil(t, got)
}

func TestExtractAttention_WrongType(t *testing.T) {
	result := map[string]any{"attention": 42}
	got := extractAttention(result)
	assert.Nil(t, got)
}

func TestExtractAttention_EmptyString(t *testing.T) {
	result := map[string]any{"attention": ""}
	got := extractAttention(result)
	assert.Nil(t, got)
}

func TestMergeBaseSchema_RequiredArrayNotPresent(t *testing.T) {
	// Schema without required array — should still inject property.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{"type": "string"},
		},
	}

	MergeBaseSchema(schema, "")

	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "attention")
	// No required array, so it should not be created.
	_, hasReq := schema["required"]
	assert.False(t, hasReq)
}

// TestMergeBaseSchema_ResultsPathFieldNotObject covers the early-return when the
// field at resultsPath is not a map[string]any (line 52-54).
func TestMergeBaseSchema_ResultsPathFieldNotObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"assessments": "not-a-map", // string instead of map[string]any
		},
	}
	require.NotPanics(t, func() {
		MergeBaseSchema(schema, "assessments")
	})
	// No injection should have occurred at the top level.
	props := schema["properties"].(map[string]any)
	_, hasAttention := props["attention"]
	assert.False(t, hasAttention, "attention must not be injected at top-level when resultsPath field is not a map")
}

// TestMergeBaseSchema_ResultsPathFieldMissing covers the case where properties
// exists but contains no key matching resultsPath.
func TestMergeBaseSchema_ResultsPathFieldMissing(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"other": map[string]any{"type": "string"},
		},
	}
	require.NotPanics(t, func() {
		MergeBaseSchema(schema, "nonexistent")
	})
	props := schema["properties"].(map[string]any)
	_, hasAttention := props["attention"]
	assert.False(t, hasAttention)
}

// TestMergeBaseSchema_ResultsPathItemsNotObject covers lines 56-58 where the
// items key exists but is not a map[string]any.
func TestMergeBaseSchema_ResultsPathItemsNotObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"data": map[string]any{
				"type":  "array",
				"items": "not-a-map", // string instead of map[string]any
			},
		},
	}
	require.NotPanics(t, func() {
		MergeBaseSchema(schema, "data")
	})
	// items is a string — no injection possible.
	dataField := schema["properties"].(map[string]any)["data"].(map[string]any)
	assert.Equal(t, "not-a-map", dataField["items"], "items must not be replaced")
}

// TestMergeBaseSchema_AttentionAlreadyInRequired covers lines 103-106 in
// injectAttention where "attention" is already in the required array.
func TestMergeBaseSchema_AttentionAlreadyInRequired(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"result", "attention"},
		"properties": map[string]any{
			"result": map[string]any{"type": "string"},
			// No "attention" property — so injectAttention will add it to properties
			// but must not duplicate the required entry.
		},
	}

	MergeBaseSchema(schema, "")

	// attention should now be in properties.
	props := schema["properties"].(map[string]any)
	assert.Contains(t, props, "attention")

	// required array must contain "attention" exactly once.
	req := schema["required"].([]any)
	count := 0
	for _, r := range req {
		if s, ok := r.(string); ok && s == "attention" {
			count++
		}
	}
	assert.Equal(t, 1, count, "attention must appear exactly once in required array")
}

// TestMergeBaseSchema_ResultsPathAttentionAlreadyInItemsRequired ensures
// injectAttention does not duplicate "attention" in items.required when it is
// already present.
func TestMergeBaseSchema_ResultsPathAttentionAlreadyInItemsRequired(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"assessments": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"score", "attention"}, // already present
					"properties": map[string]any{
						"score": map[string]any{"type": "number"},
					},
				},
			},
		},
	}

	MergeBaseSchema(schema, "assessments")

	items := schema["properties"].(map[string]any)["assessments"].(map[string]any)["items"].(map[string]any)

	// attention property should be injected.
	props := items["properties"].(map[string]any)
	assert.Contains(t, props, "attention")

	// required must have exactly one "attention" entry.
	req := items["required"].([]any)
	count := 0
	for _, r := range req {
		if s, ok := r.(string); ok && s == "attention" {
			count++
		}
	}
	assert.Equal(t, 1, count, "attention must not be duplicated in items.required")
}

// TestMergeBaseSchema_EnumValues verifies the injected attention definition
// contains the exact enum values in the canonical order.
func TestMergeBaseSchema_EnumValues(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{"type": "string"},
		},
	}

	MergeBaseSchema(schema, "")

	props := schema["properties"].(map[string]any)
	require.Contains(t, props, "attention")

	attentionDef, ok := props["attention"].(map[string]any)
	require.True(t, ok, "attention definition must be a map")

	assert.Equal(t, "string", attentionDef["type"])

	enum, ok := attentionDef["enum"].([]any)
	require.True(t, ok, "enum must be []any")
	assert.Equal(t, []any{"info", "low", "medium", "high", "critical"}, enum)
}

// TestMergeBaseSchema_DeepNestedResultsPath verifies that attention is injected
// only into items.properties and not the top-level properties when resultsPath
// is set, even when the schema has several sibling properties.
func TestMergeBaseSchema_DeepNestedResultsPath(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []any{"assessments", "summary", "confidence"},
		"properties": map[string]any{
			"summary":    map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number"},
			"assessments": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []any{"title", "body"},
					"properties": map[string]any{
						"title": map[string]any{"type": "string"},
						"body":  map[string]any{"type": "string"},
					},
				},
			},
		},
	}

	MergeBaseSchema(schema, "assessments")

	// Attention must be inside items.properties.
	items := schema["properties"].(map[string]any)["assessments"].(map[string]any)["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	assert.Contains(t, itemProps, "attention", "attention must be injected into items.properties")

	// Top-level properties must NOT contain attention.
	topProps := schema["properties"].(map[string]any)
	_, hasTopAttention := topProps["attention"]
	assert.False(t, hasTopAttention, "attention must not appear in top-level properties")
}

// TestExtractAttention_CaseSensitive verifies that mixed- and upper-case values
// are rejected because the function does a case-sensitive map lookup.
func TestExtractAttention_CaseSensitive(t *testing.T) {
	cases := []string{"High", "HIGH", "Critical", "CRITICAL", "Info", "INFO", "Low", "LOW", "Medium", "MEDIUM"}
	for _, v := range cases {
		result := map[string]any{"attention": v}
		got := extractAttention(result)
		assert.Nil(t, got, "expected nil for case-variant value %q", v)
	}
}

// TestExtractAttention_WhitespaceValue verifies that values with surrounding
// whitespace are rejected (not present in ValidAttentionLevels).
func TestExtractAttention_WhitespaceValue(t *testing.T) {
	cases := []string{" high", "high ", " high ", "\thigh", "high\n"}
	for _, v := range cases {
		result := map[string]any{"attention": v}
		got := extractAttention(result)
		assert.Nil(t, got, "expected nil for whitespace-padded value %q", v)
	}
}

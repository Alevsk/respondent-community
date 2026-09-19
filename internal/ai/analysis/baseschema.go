package analysis

import "github.com/Alevsk/respondent/internal/domain"

// baseSchemaAttention defines the JSON Schema fragment for the attention field
// that is injected into every analysis output_schema at load time.
//
// Values:
//   - info:     Background/routine, no action needed
//   - low:      Minor observation, review when convenient
//   - medium:   Notable finding, should be reviewed within hours
//   - high:     Significant finding requiring prompt attention
//   - critical: Urgent finding requiring immediate action
var baseSchemaAttention = map[string]any{
	"type": "string",
	"enum": []any{"info", "low", "medium", "high", "critical"},
}

// BaseSchemaPromptSuffix is appended to every analysis prompt to instruct the
// LLM to include the attention field in its response. This is injected
// automatically by the engine — YAML authors do not need to add it.
const BaseSchemaPromptSuffix = `

REQUIRED — Base schema field (include in EVERY item in your response):
"attention": One of "info", "low", "medium", "high", "critical".
  - info: Routine background observation, no human action needed.
  - low: Minor observation worth noting, review when convenient.
  - medium: Notable finding that should be reviewed within hours.
  - high: Significant finding requiring prompt human attention.
  - critical: Urgent finding requiring immediate action.
Calibrate attention relative to real-world operational impact, not just data novelty. Most routine observations should be "info" or "low".`

// MergeBaseSchema injects the base schema fields (currently: attention) into an
// output_schema map before it is registered in the schema registry.
//
// If resultsPath is non-empty, the schema uses the array-of-items pattern:
//
//	{ properties: { <resultsPath>: { items: { properties: { ... } } } } }
//
// and base fields are injected into items.properties.
//
// If resultsPath is empty, base fields are injected into the top-level properties.
//
// The function also adds "attention" to the "required" array if one exists.
func MergeBaseSchema(schema map[string]any, resultsPath string) {
	if resultsPath != "" {
		// Array-of-items pattern: inject into items.properties.
		props, ok := schema["properties"].(map[string]any)
		if !ok {
			return
		}
		arrayField, ok := props[resultsPath].(map[string]any)
		if !ok {
			return
		}
		items, ok := arrayField["items"].(map[string]any)
		if !ok {
			return
		}
		injectAttention(items)
		return
	}

	// Single-result pattern: inject into top-level properties.
	injectAttention(schema)
}

// extractAttention pulls the "attention" value from an LLM result map, validates
// it against the allowed enum, and returns a pointer (nil if absent or invalid).
func extractAttention(result map[string]any) *string {
	v, ok := result["attention"]
	if !ok {
		return nil
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return nil
	}
	if !domain.ValidAttentionLevels[s] {
		return nil
	}
	return &s
}

// injectAttention adds the attention field to the properties map of a schema object
// and adds "attention" to the required array if it exists and attention isn't already there.
func injectAttention(schemaObj map[string]any) {
	props, ok := schemaObj["properties"].(map[string]any)
	if !ok {
		return
	}

	// Don't overwrite if the YAML author explicitly defined attention.
	if _, exists := props["attention"]; exists {
		return
	}

	props["attention"] = baseSchemaAttention

	// Add to required array.
	if req, ok := schemaObj["required"].([]any); ok {
		// Check if already present.
		for _, r := range req {
			if s, ok := r.(string); ok && s == "attention" {
				return
			}
		}
		schemaObj["required"] = append(req, "attention")
	}
}

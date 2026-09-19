package schema

import (
	"fmt"
	"strings"
)

// CorrectionMessage builds a corrective system instruction for a retry after the
// model's JSON failed schema validation. Naming the exact required top-level keys
// steers the model away from the common failure modes (wrapping the object in an
// extra key, renaming fields, or adding a prose preamble) — and because it
// changes the input, it perturbs the output even at temperature 0.
func CorrectionMessage(outputSchema map[string]any) string {
	keys := RequiredKeys(outputSchema)
	if len(keys) == 0 {
		return "Your previous response did not match the required JSON schema. Respond with ONLY a single flat JSON object that matches the schema exactly — do not wrap it in another object, rename fields, or add any keys or text."
	}
	return fmt.Sprintf(
		"Your previous response did not match the required JSON schema. Respond with ONLY a single flat JSON object whose top-level keys are EXACTLY: %s. Do not wrap them inside another object, do not rename them, and do not add extra keys or any text outside the JSON.",
		strings.Join(keys, ", "),
	)
}

// RequiredKeys extracts the top-level "required" key names from a parsed JSON
// Schema (as loaded from YAML, where arrays are []any of strings).
func RequiredKeys(outputSchema map[string]any) []string {
	raw, ok := outputSchema["required"].([]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for _, k := range raw {
		if ks, ok := k.(string); ok && ks != "" {
			keys = append(keys, ks)
		}
	}
	return keys
}

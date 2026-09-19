// Package schema provides YAML-driven JSON Schema compilation and LLM response validation.
package schema

import (
	"fmt"
	"strings"
)

// ExtractJSON strips markdown code fences from LLM output and returns raw JSON.
func ExtractJSON(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty response")
	}

	// Strip ```json ... ``` or ``` ... ``` fences.
	if strings.HasPrefix(s, "```") {
		idx := strings.Index(s, "\n")
		if idx == -1 {
			return "", fmt.Errorf("malformed code fence")
		}
		s = s[idx+1:]
		end := strings.LastIndex(s, "```")
		if end == -1 {
			return "", fmt.Errorf("unclosed code fence")
		}
		s = strings.TrimSpace(s[:end])
	}

	if len(s) == 0 || (s[0] != '{' && s[0] != '[') {
		return "", fmt.Errorf("response does not contain JSON object or array")
	}
	return s, nil
}

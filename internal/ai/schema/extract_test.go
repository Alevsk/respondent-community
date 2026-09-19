package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractJSON_PlainObject(t *testing.T) {
	input := `{"category": "military", "confidence": 0.95}`
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"category": "military", "confidence": 0.95}`, result)
}

func TestExtractJSON_PlainArray(t *testing.T) {
	input := `[{"id": 1}, {"id": 2}]`
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `[{"id": 1}, {"id": 2}]`, result)
}

func TestExtractJSON_JSONCodeFence(t *testing.T) {
	input := "```json\n{\"category\": \"commercial\"}\n```"
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"category": "commercial"}`, result)
}

func TestExtractJSON_PlainCodeFence(t *testing.T) {
	input := "```\n{\"key\": \"value\"}\n```"
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"key": "value"}`, result)
}

func TestExtractJSON_EmptyString(t *testing.T) {
	_, err := ExtractJSON("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestExtractJSON_WhitespaceOnly(t *testing.T) {
	_, err := ExtractJSON("   \n\t  ")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty response")
}

func TestExtractJSON_NonJSONText(t *testing.T) {
	_, err := ExtractJSON("This is not JSON at all")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not contain JSON")
}

func TestExtractJSON_UnclosedFence(t *testing.T) {
	input := "```json\n{\"key\": \"value\"}"
	_, err := ExtractJSON(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unclosed code fence")
}

func TestExtractJSON_MalformedFenceNoNewline(t *testing.T) {
	input := "```"
	_, err := ExtractJSON(input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "malformed code fence")
}

func TestExtractJSON_FenceWithSurroundingWhitespace(t *testing.T) {
	input := "  \n```json\n  {\"trimmed\": true}  \n```\n  "
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"trimmed": true}`, result)
}

func TestExtractJSON_ArrayInFence(t *testing.T) {
	input := "```json\n[1, 2, 3]\n```"
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `[1, 2, 3]`, result)
}

func TestExtractJSON_FenceWithTextAfterLanguage(t *testing.T) {
	// ```json has text on same line as the opening fence tag
	input := "```json\n{\"a\": 1}\n```"
	result, err := ExtractJSON(input)
	require.NoError(t, err)
	assert.Equal(t, `{"a": 1}`, result)
}

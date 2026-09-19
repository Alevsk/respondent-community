package schema

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// Registry compiles and caches JSON Schemas defined in YAML operation configs,
// then validates LLM responses against them.
type Registry struct {
	mu      sync.RWMutex
	schemas map[string]*compiledSchema
}

type compiledSchema struct {
	schema   *jsonschema.Schema
	jsonText string // original JSON for prompt injection
}

// NewRegistry creates an empty schema registry.
func NewRegistry() *Registry {
	return &Registry{
		schemas: make(map[string]*compiledSchema),
	}
}

// RegisterFromYAML compiles and registers a JSON Schema from the map[string]any
// representation that comes from a YAML output_schema field.
func (r *Registry) RegisterFromYAML(key string, schemaMap map[string]any) error {
	jsonBytes, err := json.Marshal(schemaMap)
	if err != nil {
		return fmt.Errorf("marshal schema to JSON: %w", err)
	}

	jsonText := string(jsonBytes)

	// Parse the JSON into the any representation required by the compiler.
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(jsonText))
	if err != nil {
		return fmt.Errorf("unmarshal schema JSON: %w", err)
	}

	// Use a synthetic URL to identify this schema resource.
	url := "schema://" + key
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(url, doc); err != nil {
		return fmt.Errorf("add schema resource: %w", err)
	}

	compiled, err := compiler.Compile(url)
	if err != nil {
		return fmt.Errorf("compile schema %q: %w", key, err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemas[key] = &compiledSchema{
		schema:   compiled,
		jsonText: jsonText,
	}
	return nil
}

// SchemaJSON returns the JSON string representation of a registered schema.
// Returns an empty string if the key is not found.
func (r *Registry) SchemaJSON(key string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if cs, ok := r.schemas[key]; ok {
		return cs.jsonText
	}
	return ""
}

// Validate validates raw JSON bytes against the schema registered under key.
func (r *Registry) Validate(key string, data []byte) error {
	r.mu.RLock()
	cs, ok := r.schemas[key]
	r.mu.RUnlock()
	if !ok {
		return fmt.Errorf("schema not found: %q", key)
	}

	// Parse JSON into any using jsonschema.UnmarshalJSON for number precision.
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("parse JSON for validation: %w", err)
	}

	return cs.schema.Validate(doc)
}

// ValidateAndExtract extracts JSON from an LLM response (stripping code fences),
// validates it against the registered schema, and returns the parsed map.
func (r *Registry) ValidateAndExtract(key string, rawResponse string) (map[string]any, error) {
	extracted, err := ExtractJSON(rawResponse)
	if err != nil {
		return nil, fmt.Errorf("extract JSON: %w", err)
	}

	if err := r.Validate(key, []byte(extracted)); err != nil {
		return nil, fmt.Errorf("schema validation: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal([]byte(extracted), &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return result, nil
}

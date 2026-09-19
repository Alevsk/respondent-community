// Package config defines AI operation configuration types shared across
// sources.d/, analysis.d/, tasks.d/, and targets.d/.
package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

// SourceAIConfig is the parsed ai: section in source or analysis YAML.
type SourceAIConfig struct {
	Enabled    bool              `yaml:"enabled"`
	Operations []OperationConfig `yaml:"operations" validate:"dive"`
}

// OperationConfig defines a single AI operation.
type OperationConfig struct {
	Name          string            `yaml:"name"           validate:"required"`
	Tags          []string          `yaml:"tags"`
	Filter        string            `yaml:"filter"`
	Prompt        string            `yaml:"prompt"         validate:"required"`
	OutputSchema  map[string]any    `yaml:"output_schema"`
	OutputMapping map[string]string `yaml:"output_mapping"`
	OutputTarget  string            `yaml:"output_target"  validate:"omitempty,oneof=entity observation"`
	Batch         BatchConfig       `yaml:"batch"`
	Retry         RetryConfig       `yaml:"retry"`
	CacheTTL      string            `yaml:"cache_ttl"`
	Priority      int               `yaml:"priority"`
	MaxTokens     int               `yaml:"max_tokens"     validate:"omitempty,min=1,max=128000"`
	Temperature   float64           `yaml:"temperature"    validate:"omitempty,min=0,max=2"`
	Output        OutputConfig      `yaml:"output"`
}

// GetOutputTarget returns the output target, defaulting to "entity".
func (o *OperationConfig) GetOutputTarget() string {
	if o.OutputTarget == "" {
		return "entity"
	}
	return o.OutputTarget
}

// BatchConfig defines batching behavior for enrichment operations.
type BatchConfig struct {
	Size    int    `yaml:"size"    validate:"omitempty,min=1,max=100"`
	Timeout string `yaml:"timeout"`
}

// RetryConfig defines retry behavior for failed LLM calls.
type RetryConfig struct {
	MaxAttempts int    `yaml:"max_attempts" validate:"omitempty,min=0,max=10"`
	Backoff     string `yaml:"backoff"      validate:"omitempty,oneof=exponential fixed"`
}

// RefField specifies a field in the LLM result whose value is an external_id
// for a cross-layer entity. LayerType scopes the lookup to prevent collisions
// when different layers share the same external_id (e.g. "275" exists in both
// conflict_events and internet_infrastructure).
type RefField struct {
	Field     string `yaml:"field"      validate:"required"`
	LayerType string `yaml:"layer_type" validate:"required"`
}

// PatchCoordinatesConfig controls whether the enrichment worker updates
// entity coordinates from the LLM result.
type PatchCoordinatesConfig struct {
	Enabled         bool    `yaml:"enabled"`
	LatField        string  `yaml:"lat_field"`
	LonField        string  `yaml:"lon_field"`
	ConfidenceField string  `yaml:"confidence_field"`
	MinConfidence   float64 `yaml:"min_confidence"`
}

// OutputConfig defines where an operation's results are delivered.
type OutputConfig struct {
	StoreInsights bool   `yaml:"store_insights"`
	InsightType   string `yaml:"insight_type"`
	WebSocketPush bool   `yaml:"websocket_push"`
	Retention     string `yaml:"retention"`
	ResultsPath   string `yaml:"results_path"`
	Target        string `yaml:"target"`
	// MinAttention is the minimum attention level a result must carry to be
	// stored and notified. Results below this floor (including unclassified
	// ones, treated as "info") are dropped. Empty means no per-operation floor;
	// the engine-wide default applies. This is the primary noise control: it
	// lets an analysis suppress its own "no threat / negligible / routine"
	// conclusions instead of paging the operator with them.
	MinAttention     string                 `yaml:"min_attention" validate:"omitempty,oneof=info low medium high critical"`
	RefFields        []RefField             `yaml:"ref_fields"`
	PatchCoordinates PatchCoordinatesConfig `yaml:"patch_coordinates"`
}

// Validate validates the SourceAIConfig using the struct validator.
func (c *SourceAIConfig) Validate(v *validator.Validate) error {
	if !c.Enabled {
		return nil
	}
	if len(c.Operations) == 0 {
		return fmt.Errorf("ai.enabled is true but no operations defined")
	}
	return v.Struct(c)
}

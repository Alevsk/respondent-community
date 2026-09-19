// Package analysis implements the declarative analysis engine that loads
// analysis.d/*.yaml definitions and runs AI analysis jobs on schedule.
package analysis

import (
	"fmt"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
)

// SchemaKey returns the schema registry key for an analysis definition + operation.
// Uses "/" as separator because the schema registry constructs "schema://" + key URLs
// and ":" would be misinterpreted as a port separator.
func SchemaKey(defName, opName string) string {
	return fmt.Sprintf("%s/%s", defName, opName)
}

// AnalysisDefinition is the parsed representation of an analysis.d/ YAML file.
// The AI field reuses the same SourceAIConfig as sources.d/ -- enabled + operations[].
type AnalysisDefinition struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Name          string                  `yaml:"name"          validate:"required"`
	DisplayName   string                  `yaml:"display_name"`
	Enabled       bool                    `yaml:"enabled"`
	Schedule      ScheduleConfig          `yaml:"schedule"      validate:"required"`
	Data          DataConfig              `yaml:"data"          validate:"required"`
	AI            aiconfig.SourceAIConfig `yaml:"ai"            validate:"required"`
}

// ScheduleConfig defines when an analysis job runs.
// Interval and Cron are mutually exclusive; at least one must be set.
type ScheduleConfig struct {
	Interval string `yaml:"interval"` // Go duration string (e.g., "5m", "1h")
	Cron     string `yaml:"cron"`     // cron expression (e.g., "*/5 * * * *")
}

// DataConfig defines what data to fetch for analysis.
type DataConfig struct {
	Layers     []string     `yaml:"layers"`                         // layer types to query; empty = all layers
	Lookback   string       `yaml:"lookback"   validate:"required"` // time window (Go duration string)
	MinRecords int          `yaml:"min_records"`                    // minimum records before analysis runs
	MaxRecords int          `yaml:"max_records"`                    // cap records sent to LLM prompt (0 = use default 500)
	Filter     string       `yaml:"filter"`                         // CEL expression for pre-filtering
	SQL        string       `yaml:"sql"`                            // raw SQL (validated if present)
	Dedup      *DedupConfig `yaml:"dedup,omitempty"`                // dedup config (optional)
}

// DedupConfig defines dedup behavior for analysis insights.
// When configured, records that already have recent insights (within the window)
// are filtered out before calling the LLM, preventing duplicate analysis.
type DedupConfig struct {
	Window    string   `yaml:"window"     validate:"required"`       // Go duration (e.g., "2h")
	KeyFields []string `yaml:"key_fields" validate:"required,min=1"` // fields used as composite dedup key
}

// AnalysisRecord represents a single entity+observation record passed to prompt templates.
type AnalysisRecord struct {
	EntityID   string
	EntityName string
	ExternalID string
	LayerType  string
	Lat        float64
	Lon        float64
	Altitude   float64
	// Metadata is a merged view: entity metadata overlaid with observation metadata.
	// Prompt templates use {{index .Metadata "key"}} to access fields from either source.
	Metadata map[string]string
	// EntityMetadata holds the entity's own metadata (e.g. callsign, type).
	// ObsMetadata holds the observation's metadata (e.g. gs, track).
	// These are exposed separately in CEL filters as entity.metadata / observation.metadata.
	EntityMetadata map[string]string
	ObsMetadata    map[string]string
	Timestamp      string
}

// AnalysisPromptData is the template context available to analysis operation prompts.
type AnalysisPromptData struct {
	RecordCount  int
	Lookback     string
	Records      []AnalysisRecord
	LayerCount   int
	LayerStats   []LayerStat
	OutputSchema string
}

// LayerStat holds per-layer statistics for prompt rendering.
type LayerStat struct {
	LayerType       string
	Count           int
	FieldCoverage   string
	CoordPercent    int
	AltPercent      int
	AvgMetadataKeys int
	UniqueExtIDs    int
	DuplicateExtIDs int
}

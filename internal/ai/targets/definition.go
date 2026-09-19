// Package targets implements the declarative target delivery engine that loads
// targets.d/*.yaml definitions and routes content to delivery destinations
// with optional AI-powered content adaptation.
package targets

import (
	"fmt"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
)

// SchemaKey returns the schema registry key for a target definition + operation.
// Uses "/" as separator because the schema registry constructs "schema://" + key URLs
// and ":" would be misinterpreted as a port separator.
func SchemaKey(targetName, opName string) string {
	return fmt.Sprintf("target/%s/%s", targetName, opName)
}

// TargetDefinition is the parsed representation of a targets.d/ YAML file.
type TargetDefinition struct {
	SchemaVersion int                     `yaml:"schema_version"`
	Name          string                  `yaml:"name"       validate:"required"`
	Type          string                  `yaml:"type"       validate:"required"`
	Enabled       bool                    `yaml:"enabled"`
	Connection    map[string]string       `yaml:"connection"`
	AI            aiconfig.SourceAIConfig `yaml:"ai"`
}

// Package tasks implements the declarative task pipeline engine that loads
// tasks.d/*.yaml definitions and executes multi-step processing pipelines
// where AI is a first-class step type.
package tasks

import (
	"fmt"
)

// SchemaKey returns the schema registry key for a task definition + step.
// Uses "/" as separator because the schema registry constructs "schema://" + key URLs
// and ":" would be misinterpreted as a port separator.
func SchemaKey(taskName, stepName string) string {
	return fmt.Sprintf("task/%s/%s", taskName, stepName)
}

// TaskDefinition is the parsed representation of a tasks.d/ YAML file.
type TaskDefinition struct {
	SchemaVersion int           `yaml:"schema_version"`
	Name          string        `yaml:"name"          validate:"required"`
	DisplayName   string        `yaml:"display_name"`
	Enabled       bool          `yaml:"enabled"`
	Trigger       TriggerConfig `yaml:"trigger"       validate:"required"`
	Steps         []StepConfig  `yaml:"steps"         validate:"required,min=1,dive"`
}

// TriggerConfig defines when a task pipeline is triggered.
type TriggerConfig struct {
	Type   string `yaml:"type"   validate:"required,oneof=event schedule"`
	Source string `yaml:"source"` // source name for event triggers
	Filter string `yaml:"filter"` // CEL expression
	// For schedule triggers:
	Interval string `yaml:"interval"`
	Cron     string `yaml:"cron"`
}

// StepConfig defines a single step in a task pipeline.
type StepConfig struct {
	Name string `yaml:"name" validate:"required"`
	Type string `yaml:"type" validate:"required,oneof=ai http_request target"`

	// AI step fields:
	Prompt       string         `yaml:"prompt"`
	OutputSchema map[string]any `yaml:"output_schema"`
	MaxTokens    int            `yaml:"max_tokens"    validate:"omitempty,min=1,max=128000"`
	Temperature  float64        `yaml:"temperature"   validate:"omitempty,min=0,max=2"`

	// HTTP step fields:
	URL     string            `yaml:"url"`
	Method  string            `yaml:"method"`
	Headers map[string]string `yaml:"headers"`

	// Target step fields:
	Targets []string `yaml:"targets"`
}

// StepResult holds the output of an executed step.
type StepResult struct {
	Name   string
	Result any    // parsed result (map[string]any for AI steps, string for HTTP steps)
	Raw    string // raw string output
	Error  error
}

// StepContext provides the execution context for template rendering in steps.
// Each step can access the trigger data and results from all previous steps.
type StepContext struct {
	// Entity holds trigger data about the entity that triggered the task.
	Entity map[string]any

	// Steps holds results from all previously executed steps, keyed by step name.
	Steps map[string]*StepResult
}

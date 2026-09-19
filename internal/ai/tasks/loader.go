package tasks

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/Alevsk/respondent/internal/ai"
	"github.com/Alevsk/respondent/internal/ai/schema"
)

// Loader parses and validates tasks.d/*.yaml definitions.
type Loader struct {
	validate *validator.Validate
	schemas  *schema.Registry
	celEnv   *cel.Env
	logger   zerolog.Logger
}

// NewLoader creates a Loader with the given schema registry and logger.
func NewLoader(schemas *schema.Registry, logger zerolog.Logger) (*Loader, error) {
	celEnv, err := newTaskCELEnv()
	if err != nil {
		return nil, fmt.Errorf("create task CEL env: %w", err)
	}

	return &Loader{
		validate: validator.New(),
		schemas:  schemas,
		celEnv:   celEnv,
		logger:   logger,
	}, nil
}

// newTaskCELEnv creates a CEL environment for task trigger filter evaluation.
// Variables: entity (map) — same as enrichment worker.
func newTaskCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("entity", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("observation", cel.MapType(cel.StringType, cel.DynType)),
		ext.Strings(),
	)
}

// LoadDefinitions reads all *.yaml and *.yml files from the given directory,
// parses, validates, and returns them keyed by definition name.
func (l *Loader) LoadDefinitions(dir string) (map[string]*TaskDefinition, error) {
	pattern := filepath.Join(dir, "*.yaml")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("glob %s: %w", pattern, err)
	}

	// Also include .yml files.
	ymlFiles, err := filepath.Glob(filepath.Join(dir, "*.yml"))
	if err != nil {
		return nil, fmt.Errorf("glob *.yml: %w", err)
	}
	files = append(files, ymlFiles...)

	definitions := make(map[string]*TaskDefinition, len(files))

	for _, f := range files {
		def, err := l.loadFile(f)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", filepath.Base(f), err)
		}

		if _, exists := definitions[def.Name]; exists {
			return nil, fmt.Errorf("duplicate task definition name %q in %s", def.Name, filepath.Base(f))
		}

		definitions[def.Name] = def
		l.logger.Info().
			Str("name", def.Name).
			Str("file", filepath.Base(f)).
			Bool("enabled", def.Enabled).
			Int("steps", len(def.Steps)).
			Msg("loaded task definition")
	}

	return definitions, nil
}

// loadFile parses and validates a single task YAML file.
func (l *Loader) loadFile(path string) (*TaskDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var def TaskDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Struct validation.
	if err := l.validate.Struct(&def); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	// Trigger-specific validation.
	if err := l.validateTrigger(def.Trigger); err != nil {
		return nil, err
	}

	// Validate step-specific fields.
	stepNames := make(map[string]struct{}, len(def.Steps))
	for i, step := range def.Steps {
		if _, exists := stepNames[step.Name]; exists {
			return nil, fmt.Errorf("duplicate step name %q at index %d", step.Name, i)
		}
		stepNames[step.Name] = struct{}{}

		if err := l.validateStep(def.Name, i, step); err != nil {
			return nil, fmt.Errorf("step[%d] %q: %w", i, step.Name, err)
		}
	}

	return &def, nil
}

// validateTrigger validates trigger-specific configuration.
func (l *Loader) validateTrigger(tc TriggerConfig) error {
	switch tc.Type {
	case "event":
		if tc.Source == "" {
			return fmt.Errorf("trigger: event trigger requires a source")
		}
	case "schedule":
		hasInterval := tc.Interval != ""
		hasCron := tc.Cron != ""
		if !hasInterval && !hasCron {
			return fmt.Errorf("trigger: schedule trigger requires interval or cron")
		}
		if hasInterval && hasCron {
			return fmt.Errorf("trigger: interval and cron are mutually exclusive")
		}
		if hasInterval {
			d, err := time.ParseDuration(tc.Interval)
			if err != nil {
				return fmt.Errorf("trigger.interval %q: %w", tc.Interval, err)
			}
			if d <= 0 {
				return fmt.Errorf("trigger.interval must be positive, got %s", tc.Interval)
			}
		}
	}

	// Compile CEL filter if present.
	if tc.Filter != "" {
		if err := l.compileCELFilter(tc.Filter); err != nil {
			return fmt.Errorf("trigger.filter CEL: %w", err)
		}
	}

	return nil
}

// validateStep validates step-specific fields based on step type.
func (l *Loader) validateStep(taskName string, _ int, step StepConfig) error {
	switch step.Type {
	case "ai":
		if step.Prompt == "" {
			return fmt.Errorf("ai step requires a prompt")
		}
		// Register output schema if present.
		if len(step.OutputSchema) > 0 {
			key := SchemaKey(taskName, step.Name)
			if err := l.schemas.RegisterFromYAML(key, step.OutputSchema); err != nil {
				return fmt.Errorf("output_schema: %w", err)
			}
		}
	case "http_request":
		if step.URL == "" {
			return fmt.Errorf("http_request step requires a url")
		}
	case "target":
		if len(step.Targets) == 0 {
			return fmt.Errorf("target step requires at least one target")
		}
	}
	return nil
}

// compileCELFilter compiles a CEL expression and returns an error if invalid.
func (l *Loader) compileCELFilter(expr string) error {
	ast, issues := l.celEnv.Compile(expr)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("compile: %w", issues.Err())
	}

	_, err := l.celEnv.Program(ast,
		cel.EvalOptions(cel.OptTrackCost),
		cel.CostLimit(ai.CELFilterCostLimit),
	)
	if err != nil {
		return fmt.Errorf("program: %w", err)
	}

	return nil
}

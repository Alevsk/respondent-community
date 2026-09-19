package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/ext"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/Alevsk/respondent/internal/ai"
	"github.com/Alevsk/respondent/internal/ai/schema"
)

// Loader parses and validates analysis.d/*.yaml definitions.
type Loader struct {
	validate *validator.Validate
	schemas  *schema.Registry
	celEnv   *cel.Env
	logger   zerolog.Logger
}

// NewLoader creates a Loader with the given schema registry and logger.
func NewLoader(schemas *schema.Registry, logger zerolog.Logger) (*Loader, error) {
	celEnv, err := newAnalysisCELEnv()
	if err != nil {
		return nil, fmt.Errorf("create analysis CEL env: %w", err)
	}

	return &Loader{
		validate: validator.New(),
		schemas:  schemas,
		celEnv:   celEnv,
		logger:   logger,
	}, nil
}

// newAnalysisCELEnv creates a CEL environment for analysis filter evaluation.
// Variables: entity (map), observation (map) -- same as enrichment worker.
func newAnalysisCELEnv() (*cel.Env, error) {
	return cel.NewEnv(
		cel.Variable("entity", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("observation", cel.MapType(cel.StringType, cel.DynType)),
		ext.Strings(),
	)
}

// LoadDefinitions reads all *.yaml files from the given directory, parses,
// validates, and returns them keyed by definition name.
func (l *Loader) LoadDefinitions(dir string) (map[string]*AnalysisDefinition, error) {
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

	definitions := make(map[string]*AnalysisDefinition, len(files))

	for _, f := range files {
		// TEMPLATE.yaml (and any TEMPLATE*.yaml) is documentation scaffolding with
		// placeholder values, not a runnable definition. Skip it — the same
		// convention used for source definitions in sources.d/.
		if strings.HasPrefix(strings.ToUpper(filepath.Base(f)), "TEMPLATE") {
			continue
		}

		def, err := l.loadFile(f)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", filepath.Base(f), err)
		}

		if _, exists := definitions[def.Name]; exists {
			return nil, fmt.Errorf("duplicate analysis definition name %q in %s", def.Name, filepath.Base(f))
		}

		definitions[def.Name] = def
		l.logger.Info().
			Str("name", def.Name).
			Str("file", filepath.Base(f)).
			Bool("enabled", def.Enabled).
			Msg("loaded analysis definition")
	}

	return definitions, nil
}

// loadFile parses and validates a single analysis YAML file.
func (l *Loader) loadFile(path string) (*AnalysisDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var def AnalysisDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Struct validation.
	if err := l.validate.Struct(&def); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	// Schedule validation: at least one of interval or cron must be set.
	if err := validateSchedule(def.Schedule); err != nil {
		return nil, err
	}

	// Lookback must be a valid Go duration.
	if _, err := time.ParseDuration(def.Data.Lookback); err != nil {
		return nil, fmt.Errorf("invalid lookback duration %q: %w", def.Data.Lookback, err)
	}

	// Validate dedup config if present.
	if def.Data.Dedup != nil {
		if err := l.validate.Struct(def.Data.Dedup); err != nil {
			return nil, fmt.Errorf("dedup config: %w", err)
		}
		if _, err := time.ParseDuration(def.Data.Dedup.Window); err != nil {
			return nil, fmt.Errorf("invalid dedup.window duration %q: %w", def.Data.Dedup.Window, err)
		}
		l.logger.Info().
			Str("name", def.Name).
			Str("window", def.Data.Dedup.Window).
			Strs("key_fields", def.Data.Dedup.KeyFields).
			Msg("dedup enabled for analysis definition")
	}

	// Validate data-level CEL filter if present.
	if def.Data.Filter != "" {
		if err := l.compileCELFilter(def.Data.Filter); err != nil {
			return nil, fmt.Errorf("invalid data.filter CEL: %w", err)
		}
	}

	// AI section validation.
	if def.AI.Enabled {
		if err := def.AI.Validate(l.validate); err != nil {
			return nil, fmt.Errorf("ai section: %w", err)
		}
	}

	// Compile each operation's CEL filter and register output schemas.
	for i, op := range def.AI.Operations {
		if op.Filter != "" {
			if err := l.compileCELFilter(op.Filter); err != nil {
				return nil, fmt.Errorf("operation[%d] %q filter: %w", i, op.Name, err)
			}
		}

		if len(op.OutputSchema) > 0 {
			// Inject base schema fields (attention) before registration.
			MergeBaseSchema(op.OutputSchema, op.Output.ResultsPath)

			key := SchemaKey(def.Name, op.Name)
			if err := l.schemas.RegisterFromYAML(key, op.OutputSchema); err != nil {
				return nil, fmt.Errorf("operation[%d] %q output_schema: %w", i, op.Name, err)
			}
		}
	}

	return &def, nil
}

// validateSchedule checks that exactly one of interval or cron is set,
// and that the provided value is syntactically valid.
func validateSchedule(sc ScheduleConfig) error {
	hasInterval := sc.Interval != ""
	hasCron := sc.Cron != ""

	if !hasInterval && !hasCron {
		return fmt.Errorf("schedule: at least one of interval or cron must be set")
	}
	if hasInterval && hasCron {
		return fmt.Errorf("schedule: interval and cron are mutually exclusive")
	}

	if hasInterval {
		d, err := time.ParseDuration(sc.Interval)
		if err != nil {
			return fmt.Errorf("schedule.interval %q: %w", sc.Interval, err)
		}
		if d <= 0 {
			return fmt.Errorf("schedule.interval must be positive, got %s", sc.Interval)
		}
	}

	// Cron expression syntax is validated later when the cron scheduler adds it.
	// Basic sanity check: non-empty.
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

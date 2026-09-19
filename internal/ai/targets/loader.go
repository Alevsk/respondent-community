package targets

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"
)

// schemaRegistrar registers output schemas parsed from target definitions.
// *schema.Registry satisfies it; the loader depends on the interface (DIP).
type schemaRegistrar interface {
	RegisterFromYAML(key string, schemaMap map[string]any) error
}

// Loader parses and validates targets.d/*.yaml definitions.
type Loader struct {
	validate *validator.Validate
	schemas  schemaRegistrar
	logger   zerolog.Logger
}

// NewLoader creates a Loader with the given schema registry and logger.
func NewLoader(schemas schemaRegistrar, logger zerolog.Logger) *Loader {
	return &Loader{
		validate: validator.New(),
		schemas:  schemas,
		logger:   logger,
	}
}

// LoadDefinitions reads all *.yaml and *.yml files from the given directory,
// parses, validates, and returns them keyed by definition name.
func (l *Loader) LoadDefinitions(dir string) (map[string]*TargetDefinition, error) {
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

	definitions := make(map[string]*TargetDefinition, len(files))

	for _, f := range files {
		def, err := l.loadFile(f)
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", filepath.Base(f), err)
		}

		if _, exists := definitions[def.Name]; exists {
			return nil, fmt.Errorf("duplicate target definition name %q in %s", def.Name, filepath.Base(f))
		}

		definitions[def.Name] = def
		l.logger.Info().
			Str("name", def.Name).
			Str("type", def.Type).
			Str("file", filepath.Base(f)).
			Bool("enabled", def.Enabled).
			Msg("loaded target definition")
	}

	return definitions, nil
}

// loadFile parses and validates a single target YAML file.
func (l *Loader) loadFile(path string) (*TargetDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var def TargetDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}

	// Struct validation.
	if err := l.validate.Struct(&def); err != nil {
		return nil, fmt.Errorf("validation: %w", err)
	}

	// AI section validation and schema registration.
	if def.AI.Enabled {
		if err := def.AI.Validate(l.validate); err != nil {
			return nil, fmt.Errorf("ai section: %w", err)
		}

		for i, op := range def.AI.Operations {
			if len(op.OutputSchema) > 0 {
				key := SchemaKey(def.Name, op.Name)
				if err := l.schemas.RegisterFromYAML(key, op.OutputSchema); err != nil {
					return nil, fmt.Errorf("operation[%d] %q output_schema: %w", i, op.Name, err)
				}
			}
		}
	}

	return &def, nil
}

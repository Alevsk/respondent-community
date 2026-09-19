package main

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"

	aiconfig "github.com/Alevsk/respondent/internal/ai/config"
	"github.com/Alevsk/respondent/internal/ai/enrichment"
	"github.com/Alevsk/respondent/internal/ai/schema"
)

// Regression: enrichment op schemas must register under the compound
// "<source>.<operation>" key the worker looks up — not the bare op name, which
// silently failed every enrichment with "schema not found".
func TestRegisterEnrichmentSchemas_UsesCompoundKey(t *testing.T) {
	reg := schema.NewRegistry()
	cfgs := map[string]*aiconfig.SourceAIConfig{
		"adsb_military": {
			Operations: []aiconfig.OperationConfig{
				{
					Name: "military_aircraft_classification",
					OutputSchema: map[string]any{
						"type":       "object",
						"required":   []any{"role"},
						"properties": map[string]any{"role": map[string]any{"type": "string"}},
					},
				},
			},
		},
	}

	registerEnrichmentSchemas(reg, cfgs, zerolog.Nop())

	compound := enrichment.SchemaKey("adsb_military", "military_aircraft_classification")
	assert.NotEmpty(t, reg.SchemaJSON(compound), "schema must be registered under the compound key the worker uses")
	assert.Empty(t, reg.SchemaJSON("military_aircraft_classification"), "must NOT register under the bare op name")
}

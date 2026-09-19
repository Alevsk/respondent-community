package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/ai/schema"
)

func newTestLoader(t *testing.T) *Loader {
	t.Helper()
	reg := schema.NewRegistry()
	logger := zerolog.Nop()
	loader, err := NewLoader(reg, logger)
	require.NoError(t, err)
	return loader
}

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- Valid definition tests ---

func TestLoader_ValidDefinition(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "valid.yaml", `
schema_version: 1
name: test_analysis
display_name: "Test Analysis"
enabled: true
schedule:
  interval: "5m"
data:
  layers:
    - flights_commercial
  lookback: "1h"
  min_records: 10
ai:
  enabled: true
  operations:
    - name: test_op
      prompt: "Analyze {{.RecordCount}} records from the last {{.Lookback}}"
      output_schema:
        type: object
        required: [result]
        properties:
          result: { type: string }
      output:
        store_insights: true
        insight_type: "test"
        retention: "168h"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["test_analysis"]
	require.NotNil(t, def)
	assert.Equal(t, 1, def.SchemaVersion)
	assert.Equal(t, "test_analysis", def.Name)
	assert.Equal(t, "Test Analysis", def.DisplayName)
	assert.True(t, def.Enabled)
	assert.Equal(t, "5m", def.Schedule.Interval)
	assert.Equal(t, []string{"flights_commercial"}, def.Data.Layers)
	assert.Equal(t, "1h", def.Data.Lookback)
	assert.Equal(t, 10, def.Data.MinRecords)
	assert.True(t, def.AI.Enabled)
	assert.Len(t, def.AI.Operations, 1)
	assert.Equal(t, "test_op", def.AI.Operations[0].Name)
}

func TestLoader_CronSchedule(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "cron.yaml", `
schema_version: 1
name: cron_analysis
enabled: true
schedule:
  cron: "0 */5 * * * *"
data:
  lookback: "30m"
ai:
  enabled: true
  operations:
    - name: cron_op
      prompt: "Analyze data"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["cron_analysis"]
	assert.Equal(t, "0 */5 * * * *", def.Schedule.Cron)
	assert.Empty(t, def.Schedule.Interval)
}

func TestLoader_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "first.yaml", `
schema_version: 1
name: first_analysis
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: first_op
      prompt: "First analysis"
`)
	writeYAML(t, dir, "second.yaml", `
schema_version: 1
name: second_analysis
enabled: false
schedule:
  interval: "10m"
data:
  lookback: "2h"
ai:
  enabled: true
  operations:
    - name: second_op
      prompt: "Second analysis"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 2)
	assert.Contains(t, defs, "first_analysis")
	assert.Contains(t, defs, "second_analysis")
	assert.True(t, defs["first_analysis"].Enabled)
	assert.False(t, defs["second_analysis"].Enabled)
}

func TestLoader_EmptyLayersMeansAllLayers(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "all_layers.yaml", `
schema_version: 1
name: all_layer_analysis
enabled: true
schedule:
  interval: "30m"
data:
  layers: []
  lookback: "30m"
  min_records: 20
ai:
  enabled: true
  operations:
    - name: all_layer_op
      prompt: "Analyze all layers"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)

	def := defs["all_layer_analysis"]
	assert.Empty(t, def.Data.Layers)
}

func TestLoader_WithCELFilter(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "filtered.yaml", `
schema_version: 1
name: filtered_analysis
enabled: true
schedule:
  interval: "5m"
data:
  layers:
    - flights_commercial
  lookback: "1h"
  filter: |
    has(entity.metadata.flight)
ai:
  enabled: true
  operations:
    - name: filtered_op
      prompt: "Analyze filtered data"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Contains(t, defs["filtered_analysis"].Data.Filter, "has(entity.metadata.flight)")
}

func TestLoader_WithOutputSchemaCompilation(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "schema.yaml", `
schema_version: 1
name: schema_analysis
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: schema_op
      prompt: "Test"
      output_schema:
        type: object
        required: [anomalies]
        properties:
          anomalies:
            type: array
            items:
              type: object
              required: [title]
              properties:
                title: { type: string }
`)

	reg := schema.NewRegistry()
	logger := zerolog.Nop()
	loader, err := NewLoader(reg, logger)
	require.NoError(t, err)

	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)

	// Verify schema was registered with the correct key format.
	schemaJSON := reg.SchemaJSON(SchemaKey("schema_analysis", "schema_op"))
	assert.NotEmpty(t, schemaJSON)
	assert.Contains(t, schemaJSON, "anomalies")
}

func TestLoader_DisabledAISkipsValidation(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "disabled_ai.yaml", `
schema_version: 1
name: disabled_ai_analysis
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: false
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
}

func TestLoader_YMLExtensionSupported(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yml", `
schema_version: 1
name: yml_analysis
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: yml_op
      prompt: "Test yml"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.Contains(t, defs, "yml_analysis")
}

func TestLoader_SkipsTemplateFiles(t *testing.T) {
	dir := t.TempDir()
	// A real definition that must load.
	writeYAML(t, dir, "real.yaml", `
schema_version: 1
name: real_analysis
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: real_op
      prompt: "Real"
`)
	// TEMPLATE.yaml is scaffolding with placeholder values that are not valid
	// SQL/config. It must be skipped entirely — never parsed as a definition,
	// and never aborting the load of sibling definitions.
	writeYAML(t, dir, "TEMPLATE.yaml", `
schema_version: 1
name: <REQUIRED>
enabled: false
schedule:
  interval: "<REQUIRED>"
data:
  lookback: "<REQUIRED>"
  sql: |
    <your read-only SELECT here>
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.Contains(t, defs, "real_analysis")
	assert.NotContains(t, defs, "<REQUIRED>", "TEMPLATE.yaml must not be loaded as a definition")
}

// --- Validation failure tests ---

func TestLoader_MissingNameFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_name.yaml", `
schema_version: 1
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestLoader_MissingScheduleFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_schedule.yaml", `
schema_version: 1
name: no_schedule
enabled: true
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	// Validation triggers because schedule has neither interval nor cron.
	assert.Contains(t, err.Error(), "schedule")
}

func TestLoader_MissingLookbackFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_lookback.yaml", `
schema_version: 1
name: no_lookback
enabled: true
schedule:
  interval: "5m"
data:
  layers:
    - flights
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Lookback")
}

func TestLoader_InvalidLookbackDurationFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_lookback.yaml", `
schema_version: 1
name: bad_lookback
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "not_a_duration"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid lookback duration")
}

func TestLoader_InvalidCELFilterFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_cel.yaml", `
schema_version: 1
name: bad_cel
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
  filter: |
    this is not valid cel !!!
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CEL")
}

func TestLoader_InvalidOperationFilterFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_op_cel.yaml", `
schema_version: 1
name: bad_op_cel
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
      filter: "invalid cel $$"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "filter")
}

func TestLoader_InvalidOutputSchemaFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_schema.yaml", `
schema_version: 1
name: bad_schema
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: bad_op
      prompt: "test"
      output_schema:
        type: invalid_type_here
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output_schema")
}

func TestLoader_BothIntervalAndCronFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "both.yaml", `
schema_version: 1
name: both_schedule
enabled: true
schedule:
  interval: "5m"
  cron: "*/5 * * * *"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestLoader_NeitherIntervalNorCronFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "neither.yaml", `
schema_version: 1
name: neither_schedule
enabled: true
schedule: {}
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schedule")
}

func TestLoader_NegativeIntervalFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "neg_interval.yaml", `
schema_version: 1
name: neg_interval
enabled: true
schedule:
  interval: "-5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestLoader_InvalidYAMLFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "invalid.yaml", `
this is not: valid: yaml: [
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "YAML")
}

func TestLoader_DuplicateNamesFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "first.yaml", `
schema_version: 1
name: duplicate_name
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)
	writeYAML(t, dir, "second.yaml", `
schema_version: 1
name: duplicate_name
enabled: true
schedule:
  interval: "10m"
data:
  lookback: "2h"
ai:
  enabled: true
  operations:
    - name: op2
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestLoader_EmptyDirectoryReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
}

func TestLoader_EnabledAINoOperationsFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_ops.yaml", `
schema_version: 1
name: no_ops
enabled: true
schedule:
  interval: "5m"
data:
  lookback: "1h"
ai:
  enabled: true
  operations: []
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operations")
}

func TestLoader_InvalidIntervalDurationFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_interval.yaml", `
schema_version: 1
name: bad_interval
enabled: true
schedule:
  interval: "not_a_duration"
data:
  lookback: "1h"
ai:
  enabled: true
  operations:
    - name: op1
      prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interval")
}

func TestLoader_WithSQLField(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "with_sql.yaml", `
schema_version: 1
name: sql_analysis
enabled: true
schedule:
  interval: "10m"
data:
  layers:
    - flights_commercial
  lookback: "15m"
  sql: |
    SELECT e.id, e.name FROM entities e WHERE e.layer_type = 'flights_commercial'
ai:
  enabled: true
  operations:
    - name: sql_op
      prompt: "Analyze with SQL data"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Contains(t, defs["sql_analysis"].Data.SQL, "SELECT")
}

// --- validateSchedule unit tests ---

func TestValidateSchedule(t *testing.T) {
	tests := []struct {
		name    string
		sc      ScheduleConfig
		wantErr string
	}{
		{
			name: "valid interval",
			sc:   ScheduleConfig{Interval: "5m"},
		},
		{
			name: "valid cron",
			sc:   ScheduleConfig{Cron: "0 */5 * * * *"},
		},
		{
			name:    "neither set",
			sc:      ScheduleConfig{},
			wantErr: "at least one",
		},
		{
			name:    "both set",
			sc:      ScheduleConfig{Interval: "5m", Cron: "* * * * *"},
			wantErr: "mutually exclusive",
		},
		{
			name:    "invalid interval",
			sc:      ScheduleConfig{Interval: "bad"},
			wantErr: "interval",
		},
		{
			name:    "zero interval",
			sc:      ScheduleConfig{Interval: "0s"},
			wantErr: "positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSchedule(tt.sc)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

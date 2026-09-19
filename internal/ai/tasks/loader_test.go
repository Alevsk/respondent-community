package tasks

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

func newTestLoaderWithRegistry(t *testing.T) (*Loader, *schema.Registry) {
	t.Helper()
	reg := schema.NewRegistry()
	logger := zerolog.Nop()
	loader, err := NewLoader(reg, logger)
	require.NoError(t, err)
	return loader, reg
}

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- Valid definition tests ---

func TestLoader_ValidEventTaskDefinition(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "valid.yaml", `
schema_version: 1
name: test_task
display_name: "Test Task Pipeline"
enabled: true

trigger:
  type: event
  source: usgs_earthquakes
  filter: |
    double(entity.metadata.magnitude) >= 5.0

steps:
  - name: fetch_data
    type: http_request
    url: "https://api.example.com/data"

  - name: analyze
    type: ai
    prompt: "Analyze this data"
    output_schema:
      type: object
      required: [result]
      properties:
        result: { type: string }

  - name: deliver
    type: target
    targets:
      - slack_alerts
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["test_task"]
	require.NotNil(t, def)
	assert.Equal(t, 1, def.SchemaVersion)
	assert.Equal(t, "test_task", def.Name)
	assert.Equal(t, "Test Task Pipeline", def.DisplayName)
	assert.True(t, def.Enabled)
	assert.Equal(t, "event", def.Trigger.Type)
	assert.Equal(t, "usgs_earthquakes", def.Trigger.Source)
	assert.Contains(t, def.Trigger.Filter, "magnitude")
	assert.Len(t, def.Steps, 3)

	assert.Equal(t, "fetch_data", def.Steps[0].Name)
	assert.Equal(t, "http_request", def.Steps[0].Type)
	assert.Equal(t, "https://api.example.com/data", def.Steps[0].URL)

	assert.Equal(t, "analyze", def.Steps[1].Name)
	assert.Equal(t, "ai", def.Steps[1].Type)
	assert.Equal(t, "Analyze this data", def.Steps[1].Prompt)

	assert.Equal(t, "deliver", def.Steps[2].Name)
	assert.Equal(t, "target", def.Steps[2].Type)
	assert.Equal(t, []string{"slack_alerts"}, def.Steps[2].Targets)
}

func TestLoader_ValidScheduleTaskDefinition(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "scheduled.yaml", `
schema_version: 1
name: scheduled_task
enabled: true

trigger:
  type: schedule
  interval: "5m"

steps:
  - name: analyze
    type: ai
    prompt: "Analyze recent data"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["scheduled_task"]
	require.NotNil(t, def)
	assert.Equal(t, "schedule", def.Trigger.Type)
	assert.Equal(t, "5m", def.Trigger.Interval)
}

func TestLoader_ValidScheduleCronTask(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "cron.yaml", `
schema_version: 1
name: cron_task
enabled: true

trigger:
  type: schedule
  cron: "0 */5 * * *"

steps:
  - name: analyze
    type: ai
    prompt: "Analyze"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["cron_task"]
	assert.Equal(t, "0 */5 * * *", def.Trigger.Cron)
	assert.Empty(t, def.Trigger.Interval)
}

func TestLoader_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "first.yaml", `
schema_version: 1
name: first_task
enabled: true
trigger:
  type: event
  source: source_a
steps:
  - name: step1
    type: ai
    prompt: "First"
`)
	writeYAML(t, dir, "second.yaml", `
schema_version: 1
name: second_task
enabled: false
trigger:
  type: event
  source: source_b
steps:
  - name: step1
    type: ai
    prompt: "Second"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 2)
	assert.Contains(t, defs, "first_task")
	assert.Contains(t, defs, "second_task")
	assert.True(t, defs["first_task"].Enabled)
	assert.False(t, defs["second_task"].Enabled)
}

func TestLoader_YMLExtensionSupported(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yml", `
schema_version: 1
name: yml_task
enabled: true
trigger:
  type: event
  source: test_source
steps:
  - name: step1
    type: ai
    prompt: "Test yml"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.Contains(t, defs, "yml_task")
}

func TestLoader_EmptyDirectoryReturnsEmpty(t *testing.T) {
	dir := t.TempDir()

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Empty(t, defs)
}

func TestLoader_OutputSchemaRegistered(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "schema.yaml", `
schema_version: 1
name: schema_task
enabled: true
trigger:
  type: event
  source: test_source
steps:
  - name: analyze
    type: ai
    prompt: "Analyze"
    output_schema:
      type: object
      required: [impact_level]
      properties:
        impact_level:
          type: string
          enum: [low, medium, high]
`)

	loader, reg := newTestLoaderWithRegistry(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)

	// Verify schema was registered.
	schemaJSON := reg.SchemaJSON(SchemaKey("schema_task", "analyze"))
	assert.NotEmpty(t, schemaJSON)
	assert.Contains(t, schemaJSON, "impact_level")
}

func TestLoader_CELFilterCompilation(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "cel.yaml", `
schema_version: 1
name: cel_task
enabled: true
trigger:
  type: event
  source: test_source
  filter: |
    has(entity.metadata.magnitude) && double(entity.metadata.magnitude) >= 5.0
steps:
  - name: step1
    type: ai
    prompt: "Test"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
}

func TestLoader_HTTPStepWithHeaders(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "http.yaml", `
schema_version: 1
name: http_task
enabled: true
trigger:
  type: event
  source: test_source
steps:
  - name: fetch_data
    type: http_request
    url: "https://api.example.com/data"
    method: POST
    headers:
      Authorization: "Bearer token"
      Content-Type: "application/json"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	step := defs["http_task"].Steps[0]
	assert.Equal(t, "POST", step.Method)
	assert.Equal(t, "Bearer token", step.Headers["Authorization"])
	assert.Equal(t, "application/json", step.Headers["Content-Type"])
}

func TestLoader_AIStepWithMaxTokensAndTemperature(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "ai.yaml", `
schema_version: 1
name: ai_task
enabled: true
trigger:
  type: event
  source: test_source
steps:
  - name: analyze
    type: ai
    prompt: "Analyze"
    max_tokens: 2000
    temperature: 0.5
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	step := defs["ai_task"].Steps[0]
	assert.Equal(t, 2000, step.MaxTokens)
	assert.InDelta(t, 0.5, step.Temperature, 0.001)
}

// --- Validation failure tests ---

func TestLoader_MissingNameFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_name.yaml", `
schema_version: 1
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestLoader_MissingTriggerFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_trigger.yaml", `
schema_version: 1
name: no_trigger
enabled: true
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Type")
}

func TestLoader_MissingStepsFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_steps.yaml", `
schema_version: 1
name: no_steps
enabled: true
trigger:
  type: event
  source: test
steps: []
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Steps")
}

func TestLoader_InvalidStepTypeFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_type.yaml", `
schema_version: 1
name: bad_type
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: invalid_type
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Type")
}

func TestLoader_InvalidTriggerTypeFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_trigger.yaml", `
schema_version: 1
name: bad_trigger
enabled: true
trigger:
  type: invalid_trigger
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Type")
}

func TestLoader_EventTriggerMissingSourceFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_source.yaml", `
schema_version: 1
name: no_source
enabled: true
trigger:
  type: event
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source")
}

func TestLoader_ScheduleTriggerMissingIntervalAndCronFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_interval.yaml", `
schema_version: 1
name: no_interval
enabled: true
trigger:
  type: schedule
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interval or cron")
}

func TestLoader_ScheduleTriggerBothIntervalAndCronFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "both.yaml", `
schema_version: 1
name: both_schedule
enabled: true
trigger:
  type: schedule
  interval: "5m"
  cron: "*/5 * * * *"
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestLoader_ScheduleTriggerNegativeIntervalFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "neg.yaml", `
schema_version: 1
name: neg_interval
enabled: true
trigger:
  type: schedule
  interval: "-5m"
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestLoader_ScheduleTriggerInvalidIntervalFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_interval.yaml", `
schema_version: 1
name: bad_interval
enabled: true
trigger:
  type: schedule
  interval: "not_a_duration"
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "interval")
}

func TestLoader_InvalidCELFilterFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_cel.yaml", `
schema_version: 1
name: bad_cel
enabled: true
trigger:
  type: event
  source: test
  filter: |
    this is not valid cel !!!
steps:
  - name: step1
    type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CEL")
}

func TestLoader_AIStepMissingPromptFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_prompt.yaml", `
schema_version: 1
name: no_prompt
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: ai
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "prompt")
}

func TestLoader_HTTPStepMissingURLFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_url.yaml", `
schema_version: 1
name: no_url
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: http_request
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestLoader_TargetStepMissingTargetsFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_targets.yaml", `
schema_version: 1
name: no_targets
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: target
    targets: []
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "target")
}

func TestLoader_DuplicateStepNamesFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "dup_steps.yaml", `
schema_version: 1
name: dup_steps
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: ai
    prompt: "First"
  - name: step1
    type: ai
    prompt: "Second"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate step name")
}

func TestLoader_DuplicateTaskNamesFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "first.yaml", `
schema_version: 1
name: duplicate_name
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: ai
    prompt: "test"
`)
	writeYAML(t, dir, "second.yaml", `
schema_version: 1
name: duplicate_name
enabled: true
trigger:
  type: event
  source: test2
steps:
  - name: step1
    type: ai
    prompt: "test2"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
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

func TestLoader_InvalidOutputSchemaFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "bad_schema.yaml", `
schema_version: 1
name: bad_schema
enabled: true
trigger:
  type: event
  source: test
steps:
  - name: step1
    type: ai
    prompt: "test"
    output_schema:
      type: invalid_type_here
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output_schema")
}

func TestLoader_MissingStepNameFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_step_name.yaml", `
schema_version: 1
name: no_step_name
enabled: true
trigger:
  type: event
  source: test
steps:
  - type: ai
    prompt: "test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

// --- SchemaKey test ---

func TestSchemaKey(t *testing.T) {
	tests := []struct {
		taskName string
		stepName string
		want     string
	}{
		{"earthquake_impact_report", "impact_analysis", "task/earthquake_impact_report/impact_analysis"},
		{"task1", "step1", "task/task1/step1"},
	}

	for _, tt := range tests {
		t.Run(tt.taskName+"/"+tt.stepName, func(t *testing.T) {
			assert.Equal(t, tt.want, SchemaKey(tt.taskName, tt.stepName))
		})
	}
}

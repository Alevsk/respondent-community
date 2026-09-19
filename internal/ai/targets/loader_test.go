package targets

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
	return NewLoader(reg, logger)
}

func newTestLoaderWithRegistry(t *testing.T) (*Loader, *schema.Registry) {
	t.Helper()
	reg := schema.NewRegistry()
	logger := zerolog.Nop()
	return NewLoader(reg, logger), reg
}

func writeYAML(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return path
}

// --- Valid definition tests ---

func TestLoader_ValidSlackTarget(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "slack.yaml", `
schema_version: 1
name: slack_alerts
type: slack
enabled: true

connection:
  webhook_url: "${SLACK_WEBHOOK_URL}"
  channel: "#geo-alerts"

ai:
  enabled: true
  operations:
    - name: slack_formatter
      prompt: "Format this alert for Slack"
      output_schema:
        type: object
        required: [text]
        properties:
          text: { type: string }
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["slack_alerts"]
	require.NotNil(t, def)
	assert.Equal(t, 1, def.SchemaVersion)
	assert.Equal(t, "slack_alerts", def.Name)
	assert.Equal(t, "slack", def.Type)
	assert.True(t, def.Enabled)
	assert.Equal(t, "${SLACK_WEBHOOK_URL}", def.Connection["webhook_url"])
	assert.Equal(t, "#geo-alerts", def.Connection["channel"])
	assert.True(t, def.AI.Enabled)
	assert.Len(t, def.AI.Operations, 1)
	assert.Equal(t, "slack_formatter", def.AI.Operations[0].Name)
}

func TestLoader_ValidDiscordTarget(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "discord.yaml", `
schema_version: 1
name: discord_alerts
type: discord
enabled: true

connection:
  webhook_url: "${DISCORD_WEBHOOK_URL}"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["discord_alerts"]
	assert.Equal(t, "discord", def.Type)
	assert.False(t, def.AI.Enabled) // AI not configured
}

func TestLoader_ValidWebhookTarget(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "webhook.yaml", `
schema_version: 1
name: custom_webhook
type: webhook
enabled: true

connection:
  url: "https://example.com/hook"
  method: POST
  auth_header: "Bearer ${WEBHOOK_TOKEN}"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	require.Len(t, defs, 1)

	def := defs["custom_webhook"]
	assert.Equal(t, "webhook", def.Type)
	assert.Equal(t, "https://example.com/hook", def.Connection["url"])
	assert.Equal(t, "POST", def.Connection["method"])
}

func TestLoader_ValidEmailTarget(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "email.yaml", `
schema_version: 1
name: email_team
type: email
enabled: false

connection:
  smtp_host: "smtp.example.com"
  smtp_port: "587"
  from: "alerts@example.com"
  to: "team@example.com"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)

	def := defs["email_team"]
	assert.Equal(t, "email", def.Type)
	assert.False(t, def.Enabled)
	assert.Equal(t, "smtp.example.com", def.Connection["smtp_host"])
}

func TestLoader_MultipleTargets(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "slack.yaml", `
schema_version: 1
name: slack_target
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
`)
	writeYAML(t, dir, "discord.yaml", `
schema_version: 1
name: discord_target
type: discord
enabled: true
connection:
  webhook_url: "https://discord.com/api/webhooks/test"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 2)
	assert.Contains(t, defs, "slack_target")
	assert.Contains(t, defs, "discord_target")
}

func TestLoader_YMLExtensionSupported(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "test.yml", `
schema_version: 1
name: yml_target
type: webhook
enabled: true
connection:
  url: "https://example.com"
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
	assert.Contains(t, defs, "yml_target")
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
name: schema_target
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
ai:
  enabled: true
  operations:
    - name: formatter
      prompt: "Format"
      output_schema:
        type: object
        required: [text]
        properties:
          text: { type: string }
`)

	loader, reg := newTestLoaderWithRegistry(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)

	// Verify schema was registered with the correct key format.
	schemaJSON := reg.SchemaJSON(SchemaKey("schema_target", "formatter"))
	assert.NotEmpty(t, schemaJSON)
	assert.Contains(t, schemaJSON, "text")
}

func TestLoader_DisabledAISkipsValidation(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "disabled.yaml", `
schema_version: 1
name: disabled_ai
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
ai:
  enabled: false
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Len(t, defs, 1)
}

func TestLoader_NoConnectionConfig(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_conn.yaml", `
schema_version: 1
name: no_conn_target
type: webhook
enabled: true
`)

	loader := newTestLoader(t)
	defs, err := loader.LoadDefinitions(dir)
	require.NoError(t, err)
	assert.Nil(t, defs["no_conn_target"].Connection)
}

// --- Validation failure tests ---

func TestLoader_MissingNameFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_name.yaml", `
schema_version: 1
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestLoader_MissingTypeFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_type.yaml", `
schema_version: 1
name: no_type_target
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Type")
}

func TestLoader_DuplicateNamesFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "first.yaml", `
schema_version: 1
name: duplicate_name
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
`)
	writeYAML(t, dir, "second.yaml", `
schema_version: 1
name: duplicate_name
type: discord
enabled: true
connection:
  webhook_url: "https://discord.com/api/webhooks/test"
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
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
ai:
  enabled: true
  operations:
    - name: formatter
      prompt: "Format"
      output_schema:
        type: invalid_type_here
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "output_schema")
}

func TestLoader_EnabledAINoOperationsFails(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "no_ops.yaml", `
schema_version: 1
name: no_ops
type: slack
enabled: true
connection:
  webhook_url: "https://hooks.slack.com/test"
ai:
  enabled: true
  operations: []
`)

	loader := newTestLoader(t)
	_, err := loader.LoadDefinitions(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no operations")
}

// --- SchemaKey test ---

func TestSchemaKey(t *testing.T) {
	tests := []struct {
		targetName string
		opName     string
		want       string
	}{
		{"slack_alerts", "formatter", "target/slack_alerts/formatter"},
		{"target1", "op1", "target/target1/op1"},
	}

	for _, tt := range tests {
		t.Run(tt.targetName+"/"+tt.opName, func(t *testing.T) {
			assert.Equal(t, tt.want, SchemaKey(tt.targetName, tt.opName))
		})
	}
}

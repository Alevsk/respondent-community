package targets

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *Registry {
	return NewRegistry(zerolog.Nop())
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := newTestRegistry()

	def := &TargetDefinition{
		Name:    "slack_alerts",
		Type:    "slack",
		Enabled: true,
		Connection: map[string]string{
			"webhook_url": "https://hooks.slack.com/test",
			"channel":     "#alerts",
		},
	}

	reg.Register(def)

	got, ok := reg.Get("slack_alerts")
	require.True(t, ok)
	assert.Equal(t, "slack_alerts", got.Name)
	assert.Equal(t, "slack", got.Type)
	assert.True(t, got.Enabled)
	assert.Equal(t, "https://hooks.slack.com/test", got.Connection["webhook_url"])
}

func TestRegistry_GetNotFound(t *testing.T) {
	reg := newTestRegistry()

	_, ok := reg.Get("nonexistent")
	assert.False(t, ok)
}

func TestRegistry_List(t *testing.T) {
	reg := newTestRegistry()

	reg.Register(&TargetDefinition{Name: "slack", Type: "slack", Enabled: true})
	reg.Register(&TargetDefinition{Name: "discord", Type: "discord", Enabled: true})
	reg.Register(&TargetDefinition{Name: "email", Type: "email", Enabled: false})

	list := reg.List()
	assert.Len(t, list, 3)

	names := make(map[string]bool)
	for _, def := range list {
		names[def.Name] = true
	}
	assert.True(t, names["slack"])
	assert.True(t, names["discord"])
	assert.True(t, names["email"])
}

func TestRegistry_ListEmpty(t *testing.T) {
	reg := newTestRegistry()
	list := reg.List()
	assert.Empty(t, list)
}

func TestRegistry_LoadFromDefinitions(t *testing.T) {
	reg := newTestRegistry()

	defs := map[string]*TargetDefinition{
		"slack":   {Name: "slack", Type: "slack", Enabled: true},
		"discord": {Name: "discord", Type: "discord", Enabled: true},
	}

	reg.LoadFromDefinitions(defs)

	list := reg.List()
	assert.Len(t, list, 2)

	got, ok := reg.Get("slack")
	require.True(t, ok)
	assert.Equal(t, "slack", got.Type)
}

func TestRegistry_RegisterOverwrite(t *testing.T) {
	reg := newTestRegistry()

	reg.Register(&TargetDefinition{Name: "target1", Type: "slack", Enabled: true})
	reg.Register(&TargetDefinition{Name: "target1", Type: "discord", Enabled: false})

	got, ok := reg.Get("target1")
	require.True(t, ok)
	// Should have the overwritten value.
	assert.Equal(t, "discord", got.Type)
	assert.False(t, got.Enabled)
}

func TestRegistry_Deliver_Placeholder(t *testing.T) {
	reg := newTestRegistry()
	reg.Register(&TargetDefinition{Name: "slack_alerts", Type: "slack", Enabled: true})

	err := reg.Deliver(context.Background(), "slack_alerts", map[string]any{"text": "hello"})
	assert.NoError(t, err)
}

func TestRegistry_Deliver_NotFound(t *testing.T) {
	reg := newTestRegistry()

	err := reg.Deliver(context.Background(), "nonexistent", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_Deliver_Disabled(t *testing.T) {
	reg := newTestRegistry()
	reg.Register(&TargetDefinition{Name: "disabled", Type: "slack", Enabled: false})

	err := reg.Deliver(context.Background(), "disabled", nil)
	assert.NoError(t, err) // should silently skip
}

func TestRegistry_Deliver_CancelledContext(t *testing.T) {
	reg := newTestRegistry()
	reg.Register(&TargetDefinition{Name: "target1", Type: "slack", Enabled: true})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := reg.Deliver(ctx, "target1", nil)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestRegistry_ConnectionConfigParsing(t *testing.T) {
	reg := newTestRegistry()

	def := &TargetDefinition{
		Name:    "webhook_target",
		Type:    "webhook",
		Enabled: true,
		Connection: map[string]string{
			"url":         "https://example.com/hook",
			"method":      "POST",
			"auth_header": "Bearer ${TOKEN}",
		},
	}

	reg.Register(def)

	got, ok := reg.Get("webhook_target")
	require.True(t, ok)
	assert.Equal(t, "https://example.com/hook", got.Connection["url"])
	assert.Equal(t, "POST", got.Connection["method"])
	assert.Equal(t, "Bearer ${TOKEN}", got.Connection["auth_header"])
}

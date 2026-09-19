package main

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/config"
)

// TestLoadConfig_DefaultsFromViper proves that the community defaults are sourced
// from viper at load time (registered by setCommunityDefaults via InitViper),
// NOT applied by Validate(). With no config file and no env, LoadConfig() must
// return the community defaults directly.
func TestLoadConfig_DefaultsFromViper(t *testing.T) {
	// viper is a global singleton — reset before and after so we don't leak
	// state into other tests.
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Real production flow: community InitViper → sharedconfig.InitViper.
	// No config file (""), so ConfigFileNotFoundError is expected and ignored,
	// matching how the binary tolerates a missing file when --config is absent.
	if err := InitViper(""); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			require.NoError(t, err)
		}
	}

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// Defaults must come from viper, before Validate() is ever called.
	assert.Equal(t, "./respondent.db", cfg.Database.Path)
	assert.Equal(t, 8090, cfg.Server.Port)
	assert.Equal(t, 30*time.Second, cfg.Server.ReadTimeout)
	assert.Equal(t, 30*time.Second, cfg.Server.WriteTimeout)
	assert.Equal(t, 120*time.Second, cfg.Server.IdleTimeout)
	assert.Equal(t, "info", cfg.Logging.Level)
	assert.Equal(t, 2, cfg.AI.Workers.Enrichment)
	assert.Equal(t, 1, cfg.AI.Workers.Analysis)

	// Geocoder defaults: wired so enrichment geo-resolution (tier 2 + tier 3) works.
	assert.Equal(t, "nominatim", cfg.Geocoder.Provider)
	assert.Equal(t, "https://nominatim.openstreetmap.org", cfg.Geocoder.Nominatim.BaseURL)
	assert.Equal(t, 1.0, cfg.Geocoder.RateLimit.RequestsPerSecond)
	assert.Equal(t, 5, cfg.Geocoder.RateLimit.Burst)
	assert.True(t, cfg.Geocoder.Cache.Enabled)
	assert.Equal(t, "168h", cfg.Geocoder.Cache.TTL)
	assert.Equal(t, "1h", cfg.Geocoder.Cache.NegativeTTL)
}

// TestLoadConfig_SharedLLMDefaults proves the latent LLM-defaults bug is fixed:
// because community InitViper now routes through sharedconfig.InitViper, the
// shared LLM defaults land in the community Config via the single Unmarshal.
func TestLoadConfig_SharedLLMDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := InitViper(""); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			require.NoError(t, err)
		}
	}

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// Shared LLM defaults registered by sharedconfig.InitViper must be present.
	assert.Equal(t, "gpt-4o", cfg.LLM.OpenAI.Model)
	assert.Equal(t, "claude-sonnet-4-20250514", cfg.LLM.Anthropic.Model)
	assert.Equal(t, "https://api.openai.com/v1", cfg.LLM.OpenAI.BaseURL)
}

func TestConfig_Validate_AIRequiresLLM(t *testing.T) {
	cfg := &Config{
		AI: AIConfig{Enabled: true},
	}
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "llm.provider")
}

func TestConfig_Validate_AIWithLLM(t *testing.T) {
	cfg := &Config{
		AI:  AIConfig{Enabled: true},
		LLM: config.LLMConfig{Provider: "ollama"},
	}
	require.NoError(t, cfg.Validate())
}

func TestConfig_Validate_AIWithLMStudio(t *testing.T) {
	cfg := &Config{
		AI:  AIConfig{Enabled: true},
		LLM: config.LLMConfig{Provider: "lmstudio", LMStudio: config.LLMProviderConfig{BaseURL: "http://localhost:1234/v1"}},
	}
	require.NoError(t, cfg.Validate())
}

// TestConfig_Validate_NoDefaults proves Validate() no longer applies defaults.
// Defaults live in viper now (single source of truth); Validate() only checks
// the ai.enabled → llm.provider invariant. A zero-value Config (no AI) must
// pass validation untouched.
func TestConfig_Validate_NoDefaults(t *testing.T) {
	cfg := &Config{}
	require.NoError(t, cfg.Validate())

	// Validate must NOT have backfilled defaults — those come from viper.
	assert.Equal(t, "", cfg.Database.Path)
	assert.Equal(t, 0, cfg.Server.Port)
	assert.Equal(t, time.Duration(0), cfg.Server.ReadTimeout)
	assert.Equal(t, "", cfg.Logging.Level)
	assert.Equal(t, 0, cfg.AI.Workers.Enrichment)
	assert.Equal(t, 0, cfg.AI.Workers.Analysis)
}

// TestLoadConfig_EnvOverrides proves env overrides DEFAULTS through the ACTUAL
// production path: community InitViper → sharedconfig.InitViper (AutomaticEnv +
// RESPONDENT_ prefix + registered defaults) → LoadConfig.
func TestLoadConfig_EnvOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	t.Setenv("RESPONDENT_DATABASE_PATH", "/tmp/e2e-test.db")
	t.Setenv("RESPONDENT_SERVER_PORT", "8091")

	if err := InitViper(""); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			require.NoError(t, err)
		}
	}

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// Env vars must win over the built-in viper defaults.
	assert.Equal(t, "/tmp/e2e-test.db", cfg.Database.Path)
	assert.Equal(t, 8091, cfg.Server.Port)
}

// TestLoadConfig_EnvBeatsFile proves env > FILE > default through the real flow.
// This mirrors the e2e production ordering:
//
//	RESPONDENT_DATABASE_PATH/RESPONDENT_SERVER_PORT env
//	  + ./bin/community serve --config respondent.yaml
//	  → InitViper(cfgFile) → LoadConfig()
func TestLoadConfig_EnvBeatsFile(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	// Write a temp config file with explicit file-level values.
	dir := t.TempDir()
	cfgFile := dir + "/respondent.yaml"
	err := os.WriteFile(cfgFile,
		[]byte("database:\n  path: ./from-file.db\nserver:\n  port: 9999\n"), 0o644)
	require.NoError(t, err)

	// Set env vars that must beat the file values.
	t.Setenv("RESPONDENT_DATABASE_PATH", "./from-env.db")
	t.Setenv("RESPONDENT_SERVER_PORT", "7777")

	// Real production flow: community InitViper reads the config file via
	// sharedconfig.InitViper, wiring AutomaticEnv + prefix.
	require.NoError(t, InitViper(cfgFile))

	cfg, err := LoadConfig()
	require.NoError(t, err)

	// Env must win over the file — not just over the default.
	assert.Equal(t, "./from-env.db", cfg.Database.Path, "env RESPONDENT_DATABASE_PATH must override config-file database.path")
	assert.Equal(t, 7777, cfg.Server.Port, "env RESPONDENT_SERVER_PORT must override config-file server.port")
}

// TestInitViper_ExplicitMissingConfig pins the contract that main.go relies on:
// when --config points to a file that does not exist, InitViper must return a
// non-nil error that is NOT a viper.ConfigFileNotFoundError. Viper surfaces this
// as a generic read error (*os.PathError), which main.go treats as fatal.
func TestInitViper_ExplicitMissingConfig(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	missing := dir + "/does-not-exist.yaml"

	err := InitViper(missing)

	require.Error(t, err, "explicit missing config must be a hard error")
	var notFound viper.ConfigFileNotFoundError
	assert.False(t, errors.As(err, &notFound),
		"explicit missing config must NOT be ConfigFileNotFoundError — it must propagate as a fatal generic error")
}

// TestLoadConfig_RetentionDefaults pins the database.retention defaults registered
// by setCommunityDefaults: retention is ON by default with a 5GB cap so a
// single-machine instance never grows unbounded, plus sane tuning knobs.
func TestLoadConfig_RetentionDefaults(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	if err := InitViper(""); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			require.NoError(t, err)
		}
	}

	cfg, err := LoadConfig()
	require.NoError(t, err)

	r := cfg.Database.Retention
	assert.Equal(t, "5GB", r.MaxSize, "retention must default to a 5GB cap (on by default)")
	assert.Equal(t, 0.90, r.LowWaterRatio)
	assert.Equal(t, 10*time.Minute, r.CheckInterval)
	assert.Equal(t, 5000, r.BatchSize)
	assert.Equal(t, 200, r.MaxBatchesPerTick)
	assert.Equal(t, 10000, r.IncrementalVacuumPages)

	b, err := r.MaxSizeBytes()
	require.NoError(t, err)
	assert.Equal(t, int64(5_000_000_000), b, "5GB resolves to 5e9 bytes (decimal SI)")
}

// TestRetentionConfig_MaxSizeBytes verifies humanized byte parsing: empty disables
// (0, no error), SI/IEC suffixes parse, garbage errors.
func TestRetentionConfig_MaxSizeBytes(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"", 0, false},
		{"   ", 0, false},
		{"50GB", 50_000_000_000, false},
		{"500MB", 500_000_000, false},
		{"2GiB", 2_147_483_648, false},
		{"notasize", 0, true},
	}
	for _, c := range cases {
		got, err := RetentionConfig{MaxSize: c.in}.MaxSizeBytes()
		if c.wantErr {
			assert.Error(t, err, "input %q", c.in)
			continue
		}
		require.NoError(t, err, "input %q", c.in)
		assert.Equal(t, c.want, got, "input %q", c.in)
	}
}

// TestConfig_Validate_Retention proves retention invariants are enforced only when
// enabled (max_size set), and a disabled block with junk values still passes.
func TestConfig_Validate_Retention(t *testing.T) {
	base := func() *Config {
		return &Config{Database: DatabaseConfig{Retention: RetentionConfig{
			MaxSize: "50GB", LowWaterRatio: 0.9, CheckInterval: time.Minute,
			BatchSize: 5000, MaxBatchesPerTick: 200, IncrementalVacuumPages: 10000,
		}}}
	}

	// Disabled: empty max_size means the numeric invariants are not checked.
	disabled := &Config{Database: DatabaseConfig{Retention: RetentionConfig{
		MaxSize: "", LowWaterRatio: 5, BatchSize: -1,
	}}}
	require.NoError(t, disabled.Validate(), "disabled retention must skip numeric checks")

	require.NoError(t, base().Validate(), "valid enabled retention must pass")

	bad := func(mut func(*Config)) error {
		c := base()
		mut(c)
		return c.Validate()
	}
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.LowWaterRatio = 1.0 }), "ratio >= 1")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.LowWaterRatio = 0 }), "ratio <= 0")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.BatchSize = 0 }), "batch_size <= 0")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.MaxBatchesPerTick = 0 }), "max_batches_per_tick <= 0")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.IncrementalVacuumPages = 0 }), "incremental_vacuum_pages <= 0")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.CheckInterval = 0 }), "check_interval <= 0")
	assert.Error(t, bad(func(c *Config) { c.Database.Retention.MaxSize = "garbage" }), "malformed max_size")
}

// TestLoadConfig_FileBeatsDefault proves FILE > default: with no env, file values
// win over the registered viper defaults.
func TestLoadConfig_FileBeatsDefault(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)

	dir := t.TempDir()
	cfgFile := dir + "/respondent.yaml"
	err := os.WriteFile(cfgFile,
		[]byte("database:\n  path: ./from-file.db\nserver:\n  port: 9999\n"), 0o644)
	require.NoError(t, err)

	require.NoError(t, InitViper(cfgFile))

	cfg, err := LoadConfig()
	require.NoError(t, err)

	assert.Equal(t, "./from-file.db", cfg.Database.Path)
	assert.Equal(t, 9999, cfg.Server.Port)
}

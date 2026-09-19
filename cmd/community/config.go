package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/viper"

	"github.com/Alevsk/respondent/internal/config"
)

// Config holds all community edition configuration.
type Config struct {
	Database DatabaseConfig        `yaml:"database" mapstructure:"database"`
	Server   ServerConfig          `yaml:"server" mapstructure:"server"`
	Ingest   IngestConfig          `yaml:"ingest" mapstructure:"ingest"`
	AI       AIConfig              `yaml:"ai" mapstructure:"ai"`
	LLM      config.LLMConfig      `yaml:"llm" mapstructure:"llm"`
	Geocoder config.GeocoderConfig `yaml:"geocoder" mapstructure:"geocoder"`
	Logging  LoggingConfig         `yaml:"logging" mapstructure:"logging"`
	Frontend FrontendConfig        `yaml:"frontend" mapstructure:"frontend"`
}

// FrontendConfig holds client-safe values the server hands to the embedded SPA at
// runtime via GET /config.json, so per-deployment frontend config reaches the
// already-compiled bundle without a rebuild. These are exposed to the browser by
// design — never put server secrets here.
type FrontendConfig struct {
	// CesiumIonToken is a Cesium ion CLIENT token (env RESPONDENT_FRONTEND_CESIUM_ION_TOKEN).
	// Empty = the frontend falls back to its build-time token / Stadia-OSM imagery.
	CesiumIonToken string `yaml:"cesium_ion_token" mapstructure:"cesium_ion_token"`
}

// DatabaseConfig configures the SQLite database.
type DatabaseConfig struct {
	Path      string          `yaml:"path" mapstructure:"path"`
	Retention RetentionConfig `yaml:"retention" mapstructure:"retention"`
}

// RetentionConfig configures the SQLite size-cap retention routine. It is opt-in:
// retention runs only when MaxSize resolves to a positive byte count. The tuning
// knobs (BatchSize, MaxBatchesPerTick, IncrementalVacuumPages) are SQLite-specific
// and bound the lock contention the background prune places on the single writer.
type RetentionConfig struct {
	// MaxSize is the HIGH watermark as a humanized byte size (e.g. "50GB"). Empty
	// disables retention. Parsed via MaxSizeBytes.
	MaxSize string `yaml:"max_size" mapstructure:"max_size"`
	// LowWaterRatio is the LOW watermark as a fraction of MaxSize; pruning continues
	// until size <= MaxSize*LowWaterRatio. The MaxSize→low gap is the hysteresis band.
	LowWaterRatio float64 `yaml:"low_water_ratio" mapstructure:"low_water_ratio"`
	// CheckInterval is how often the maintenance goroutine measures size and prunes.
	CheckInterval time.Duration `yaml:"check_interval" mapstructure:"check_interval"`
	// BatchSize is the number of observation rows deleted per transaction.
	BatchSize int `yaml:"batch_size" mapstructure:"batch_size"`
	// MaxBatchesPerTick caps how many delete batches one tick runs; if still over the
	// low watermark, the next tick continues (convergent, idempotent).
	MaxBatchesPerTick int `yaml:"max_batches_per_tick" mapstructure:"max_batches_per_tick"`
	// IncrementalVacuumPages is N for PRAGMA incremental_vacuum(N) after a prune,
	// bounding the reclaim lock window.
	IncrementalVacuumPages int `yaml:"incremental_vacuum_pages" mapstructure:"incremental_vacuum_pages"`
}

// MaxSizeBytes parses MaxSize into a byte count. An empty (or whitespace) MaxSize
// returns 0, which means retention is disabled. A malformed value returns an error.
func (r RetentionConfig) MaxSizeBytes() (int64, error) {
	s := strings.TrimSpace(r.MaxSize)
	if s == "" {
		return 0, nil
	}
	n, err := humanize.ParseBytes(s)
	if err != nil {
		return 0, fmt.Errorf("parse database.retention.max_size %q: %w", r.MaxSize, err)
	}
	return int64(n), nil
}

// ServerConfig configures the HTTP server.
type ServerConfig struct {
	Port         int           `yaml:"port" mapstructure:"port"`
	ReadTimeout  time.Duration `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout" mapstructure:"idle_timeout"`
}

// IngestConfig configures source ingestion.
type IngestConfig struct {
	// Enabled gates the live feeder. Defaults to true; set false (e.g. via
	// RESPONDENT_INGEST_ENABLED=false) to serve a fully static instance with no
	// source fetching or WebSocket data churn — used by the deterministic e2e suite.
	Enabled    bool   `yaml:"enabled" mapstructure:"enabled"`
	SourcesDir string `yaml:"sources_dir" mapstructure:"sources_dir"`
	DevMode    bool   `yaml:"dev_mode" mapstructure:"dev_mode"`
}

// AIConfig configures AI features (enrichment + analysis).
type AIConfig struct {
	Enabled     bool          `yaml:"enabled" mapstructure:"enabled"`
	AnalysisDir string        `yaml:"analysis_dir" mapstructure:"analysis_dir"`
	Workers     WorkersConfig `yaml:"workers" mapstructure:"workers"`
	// MinAttention is the engine-wide floor for storing/notifying analysis
	// insights when an analysis sets no output.min_attention of its own. One of
	// info/low/medium/high/critical. "low" drops pure-info routine observations
	// platform-wide; a per-analysis output.min_attention overrides it.
	MinAttention string `yaml:"min_attention" mapstructure:"min_attention"`
}

// WorkersConfig configures AI worker concurrency.
type WorkersConfig struct {
	Enrichment int `yaml:"enrichment" mapstructure:"enrichment"`
	Analysis   int `yaml:"analysis" mapstructure:"analysis"`
}

// LoggingConfig configures logging.
type LoggingConfig struct {
	Level string `yaml:"level" mapstructure:"level"`
}

// InitViper initializes viper for the community edition, mirroring the canonical
// per-service pattern (see internal/app/respondent/config/viper.go): register
// service-specific defaults first, then delegate to the shared InitViper which
// wires the RESPONDENT_ env prefix, AutomaticEnv, shared defaults, and reads the
// config file.
//
// Registering keys as viper defaults is what makes AutomaticEnv + viper.Unmarshal
// pick up env overrides — there is no need for hand-written viper.BindEnv calls.
//
// Returns viper.ConfigFileNotFoundError if the config file is not found (the
// caller can ignore it when --config was not explicitly provided).
func InitViper(cfgFile string) error {
	// Set community-specific defaults first (before calling shared InitViper).
	setCommunityDefaults()

	// Call shared InitViper (sets shared defaults, wires env, reads config file).
	return config.InitViper(cfgFile)
}

// setCommunityDefaults registers the community edition's default values on viper.
// These are the single source of truth for community defaults; Validate() no
// longer applies them. Mirrors setSharedDefaultsOn / respondent setDefaults.
func setCommunityDefaults() {
	// Database (SQLite — community-specific).
	viper.SetDefault("database.path", "./respondent.db")

	// Database size-cap retention. ON by default with a 5GB cap so a single-machine
	// instance never grows unbounded (the observations table is append-only — flight
	// history alone reached ~10M rows / 8GB in testing, which thrashes the page
	// cache and slows every query). Operators can raise/lower it or set "" to
	// disable. Pruning never throttles live ingestion; it chases the cap.
	viper.SetDefault("database.retention.max_size", "5GB")
	viper.SetDefault("database.retention.low_water_ratio", 0.90)
	viper.SetDefault("database.retention.check_interval", 10*time.Minute)
	viper.SetDefault("database.retention.batch_size", 5000)
	viper.SetDefault("database.retention.max_batches_per_tick", 200)
	viper.SetDefault("database.retention.incremental_vacuum_pages", 10000)

	// Server.
	viper.SetDefault("server.port", 8090)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)
	viper.SetDefault("server.idle_timeout", 120*time.Second)

	// Logging.
	viper.SetDefault("logging.level", "info")

	// Ingestion (live feeder enabled by default; RESPONDENT_INGEST_ENABLED=false
	// serves a static instance for the deterministic e2e suite).
	viper.SetDefault("ingest.enabled", true)

	// AI workers.
	viper.SetDefault("ai.workers.enrichment", 2)
	viper.SetDefault("ai.workers.analysis", 1)

	// AI insight noise floor: drop analysis insights below this attention level
	// (store + notify) unless an analysis overrides via output.min_attention.
	viper.SetDefault("ai.min_attention", "low")

	// Geocoder for enrichment geo-resolution. Tier 2 calls Nominatim for specific
	// localities (city/region); tier 3 is the offline country-centroid fallback
	// (a comprehensive built-in table covers all countries — country_centroids in
	// the config only override specific entries). Set provider "none" for a fully
	// offline instance (tiers 1+3 only, no external calls).
	viper.SetDefault("geocoder.provider", "nominatim")
	viper.SetDefault("geocoder.nominatim.base_url", "https://nominatim.openstreetmap.org")
	viper.SetDefault("geocoder.nominatim.user_agent", "respondent/1.0 (geospatial-intelligence)")
	viper.SetDefault("geocoder.rate_limit.requests_per_second", 1.0)
	viper.SetDefault("geocoder.rate_limit.burst", 5)
	viper.SetDefault("geocoder.cache.enabled", true)
	viper.SetDefault("geocoder.cache.ttl", "168h")
	viper.SetDefault("geocoder.cache.negative_ttl", "1h")
	viper.SetDefault("geocoder.cache.key_prefix", "geocoder:")

	// Frontend runtime config served to the SPA via GET /config.json. Empty by
	// default; env RESPONDENT_FRONTEND_CESIUM_ION_TOKEN supplies it per deployment.
	viper.SetDefault("frontend.cesium_ion_token", "")
}

// LoadConfig loads community config from viper. Must be called after InitViper.
//
// Env overrides and defaults are handled by viper (AutomaticEnv + the keys
// registered by setCommunityDefaults / sharedconfig defaults), so a single
// Unmarshal picks up env > file > default automatically.
func LoadConfig() (*Config, error) {
	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	return cfg, nil
}

// Validate checks the config for required fields. Defaults live in viper (set by
// setCommunityDefaults via InitViper), so Validate only enforces invariants.
func (c *Config) Validate() error {
	// Validate LLM config when AI is enabled.
	if c.AI.Enabled {
		if c.LLM.Provider == "" {
			return fmt.Errorf("ai.enabled requires llm.provider to be set")
		}
	}

	// Validate retention only when enabled (max_size set). A malformed max_size is
	// always an error; the numeric invariants are checked only when retention is on.
	maxBytes, err := c.Database.Retention.MaxSizeBytes()
	if err != nil {
		return err
	}
	if maxBytes > 0 {
		r := c.Database.Retention
		if r.LowWaterRatio <= 0 || r.LowWaterRatio >= 1 {
			return fmt.Errorf("database.retention.low_water_ratio must be between 0 and 1 (exclusive), got %v", r.LowWaterRatio)
		}
		if r.CheckInterval <= 0 {
			return fmt.Errorf("database.retention.check_interval must be > 0, got %v", r.CheckInterval)
		}
		if r.BatchSize <= 0 {
			return fmt.Errorf("database.retention.batch_size must be > 0, got %d", r.BatchSize)
		}
		if r.MaxBatchesPerTick <= 0 {
			return fmt.Errorf("database.retention.max_batches_per_tick must be > 0, got %d", r.MaxBatchesPerTick)
		}
		if r.IncrementalVacuumPages <= 0 {
			return fmt.Errorf("database.retention.incremental_vacuum_pages must be > 0, got %d", r.IncrementalVacuumPages)
		}
	}

	return nil
}

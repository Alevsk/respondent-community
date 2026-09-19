// Package config provides feeder-specific configuration handling.
package config

import (
	"fmt"

	"github.com/spf13/viper"

	"github.com/Alevsk/respondent/internal/config"
)

// Config holds all configuration for the feeder service.
type Config struct {
	Database config.DatabaseConfig `yaml:"database" mapstructure:"database"`
	Valkey   config.ValkeyConfig   `yaml:"valkey" mapstructure:"valkey"`
	Ingest   config.IngestConfig   `yaml:"ingest" mapstructure:"ingest"`
	Feeder   config.FeederConfig   `yaml:"feeder" mapstructure:"feeder"`
	Logging  config.LoggingConfig  `yaml:"logging" mapstructure:"logging"`
	AI       config.AIConfig       `yaml:"ai" mapstructure:"ai"`
}

// LoadFromViper loads configuration from Viper.
func LoadFromViper() (*Config, error) {
	var cfg Config

	cfg.Database = config.LoadDatabaseConfig()
	cfg.Valkey = config.LoadValkeyConfig()
	cfg.Logging = config.LoadLoggingConfig()

	// Ingest
	cfg.Ingest.SourcesDir = viper.GetString("ingest.sources_dir")
	cfg.Ingest.DevMode = viper.GetBool("ingest.dev_mode")

	// Feeder
	cfg.Feeder.BatchSize = viper.GetInt("feeder.batch_size")
	cfg.Feeder.WorkerCount = viper.GetInt("feeder.worker_count")
	cfg.Feeder.RetryLimit = viper.GetInt("feeder.retry_limit")
	cfg.Feeder.RetryBackoff = viper.GetDuration("feeder.retry_backoff")

	// AI config (optional — defaults applied if section absent)
	cfg.AI = config.DefaultAIConfig()
	if sub := viper.Sub("ai"); sub != nil {
		if err := sub.Unmarshal(&cfg.AI); err != nil {
			return nil, fmt.Errorf("failed to unmarshal ai config: %w", err)
		}
	}

	return &cfg, nil
}

// Validate checks that required feeder configuration fields are set.
func (c *Config) Validate() error {
	if c.Database.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Database.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if c.Valkey.Addr == "" {
		return fmt.Errorf("valkey.addr is required")
	}
	return nil
}

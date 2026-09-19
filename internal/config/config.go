// Package config provides unified configuration handling for all Respondent services.
package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"

	"github.com/Alevsk/respondent/internal/domain"
)

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Host            string        `yaml:"host" mapstructure:"host"`
	Port            int           `yaml:"port" mapstructure:"port"`
	Name            string        `yaml:"name" mapstructure:"name"`
	User            string        `yaml:"user" mapstructure:"user"`
	Password        string        `yaml:"password" mapstructure:"password"`
	SSLMode         string        `yaml:"ssl_mode" mapstructure:"ssl_mode"`
	MaxConns        int           `yaml:"max_conns" mapstructure:"max_conns"`
	MinConns        int           `yaml:"min_conns" mapstructure:"min_conns"`
	MaxConnLifetime time.Duration `yaml:"max_conn_lifetime" mapstructure:"max_conn_lifetime"`
}

// GetDSN returns the PostgreSQL DSN.
func (c DatabaseConfig) GetDSN() string {
	return fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.Host, c.Port, c.Name, c.User, c.Password, c.SSLMode)
}

// NATSConfig holds NATS connection configuration for real-time features.
type NATSConfig struct {
	URL       string `yaml:"url" mapstructure:"url"`
	ClusterID string `yaml:"cluster_id" mapstructure:"cluster_id"`
	ClientID  string `yaml:"client_id" mapstructure:"client_id"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level  string `yaml:"level" mapstructure:"level"`
	Format string `yaml:"format" mapstructure:"format"` // "json" or "console"
}

// TracingConfig holds OpenTelemetry tracing settings.
type TracingConfig struct {
	Enabled      bool    `yaml:"enabled" mapstructure:"enabled"`
	OTLPEndpoint string  `yaml:"otlp_endpoint" mapstructure:"otlp_endpoint"`
	SampleRate   float64 `yaml:"sample_rate" mapstructure:"sample_rate"`
}

// ValkeyConfig holds Valkey/Redis cache connection settings.
type ValkeyConfig struct {
	Addr        string `yaml:"addr" mapstructure:"addr"`
	Password    string `yaml:"password" mapstructure:"password"`
	DB          int    `yaml:"db" mapstructure:"db"`
	PoolSize    int    `yaml:"pool_size" mapstructure:"pool_size"`
	MinIdleConn int    `yaml:"min_idle_conns" mapstructure:"min_idle_conns"`
}

// LoadValkeyConfig loads Valkey configuration from viper.
func LoadValkeyConfig() ValkeyConfig {
	return ValkeyConfig{
		Addr:        viper.GetString("valkey.addr"),
		Password:    viper.GetString("valkey.password"),
		DB:          viper.GetInt("valkey.db"),
		PoolSize:    viper.GetInt("valkey.pool_size"),
		MinIdleConn: viper.GetInt("valkey.min_idle_conns"),
	}
}

// FeederConfig holds feeder-specific settings.
type FeederConfig struct {
	BatchSize    int           `yaml:"batch_size" mapstructure:"batch_size"`
	WorkerCount  int           `yaml:"worker_count" mapstructure:"worker_count"`
	RetryLimit   int           `yaml:"retry_limit" mapstructure:"retry_limit"`
	RetryBackoff time.Duration `yaml:"retry_backoff" mapstructure:"retry_backoff"`
}

// IngestConfig holds ingestion source configuration shared by feeder and server.
// All source definitions are now loaded declaratively from SourcesDir (sources.d/*.yaml).
// WebSocket/realtime server configuration lives in the respondent server's own config package.
type IngestConfig struct {
	SourcesDir string `yaml:"sources_dir" mapstructure:"sources_dir"`
	DevMode    bool   `yaml:"dev_mode" mapstructure:"dev_mode"`
}

// LLMConfig holds configuration for the AI/LLM provider integration.
type LLMConfig struct {
	Provider  string            `yaml:"provider" mapstructure:"provider"` // Active provider type (openai, anthropic, gemini, xai, zai, ollama, lmstudio).
	OpenAI    LLMProviderConfig `yaml:"openai" mapstructure:"openai"`
	Anthropic LLMProviderConfig `yaml:"anthropic" mapstructure:"anthropic"`
	Gemini    LLMProviderConfig `yaml:"gemini" mapstructure:"gemini"`
	XAI       LLMProviderConfig `yaml:"xai" mapstructure:"xai"`
	ZAI       LLMProviderConfig `yaml:"zai" mapstructure:"zai"`
	Ollama    LLMProviderConfig `yaml:"ollama" mapstructure:"ollama"`
	LMStudio  LLMProviderConfig `yaml:"lmstudio" mapstructure:"lmstudio"`
}

// LLMProviderConfig holds per-provider settings.
type LLMProviderConfig struct {
	APIKey    string `yaml:"api_key" mapstructure:"api_key"`
	Model     string `yaml:"model" mapstructure:"model"`
	MaxTokens int    `yaml:"max_tokens" mapstructure:"max_tokens"`
	BaseURL   string `yaml:"base_url" mapstructure:"base_url"`
	// Timeout is the per-request HTTP timeout (Go duration string, e.g. "120s").
	// Reasoning models (GLM) can exceed shorter defaults on large prompts.
	Timeout string `yaml:"timeout" mapstructure:"timeout"`
	// EnableThinking turns on the GLM reasoning phase (zai only). Off by default:
	// structured/schema-constrained output does not benefit from chain-of-thought
	// and reasoning burns tokens + latency and can truncate the answer.
	EnableThinking bool `yaml:"enable_thinking" mapstructure:"enable_thinking"`
}

// GeocoderConfig holds geocoder service configuration.
type GeocoderConfig struct {
	Provider         string                  `yaml:"provider" mapstructure:"provider"`
	Nominatim        GeocoderNominatimConfig `yaml:"nominatim" mapstructure:"nominatim"`
	Google           GeocoderProviderConfig  `yaml:"google" mapstructure:"google"`
	Mapbox           GeocoderProviderConfig  `yaml:"mapbox" mapstructure:"mapbox"`
	RateLimit        GeocoderRateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Cache            GeocoderCacheConfig     `yaml:"cache" mapstructure:"cache"`
	CountryCentroids []CountryCentroidEntry  `yaml:"country_centroids" mapstructure:"country_centroids"`
}

// CountryCentroidEntry maps an ISO 3166-1 alpha-3 country code to its approximate centroid.
type CountryCentroidEntry struct {
	ISO3 string  `yaml:"iso3" mapstructure:"iso3"`
	Lat  float64 `yaml:"lat" mapstructure:"lat"`
	Lon  float64 `yaml:"lon" mapstructure:"lon"`
}

// CentroidMap converts the CountryCentroids slice into the map[string][2]float64
// format used by the enrichment worker's Tier 3 geo-resolution.
func (c GeocoderConfig) CentroidMap() map[string][2]float64 {
	if len(c.CountryCentroids) == 0 {
		return nil
	}
	m := make(map[string][2]float64, len(c.CountryCentroids))
	for _, e := range c.CountryCentroids {
		m[e.ISO3] = [2]float64{e.Lat, e.Lon}
	}
	return m
}

// GeocoderNominatimConfig holds Nominatim-specific connection settings.
type GeocoderNominatimConfig struct {
	BaseURL   string `yaml:"base_url" mapstructure:"base_url"`
	UserAgent string `yaml:"user_agent" mapstructure:"user_agent"`
}

// GeocoderProviderConfig holds generic API-key-based provider settings.
type GeocoderProviderConfig struct {
	EnvVar string `yaml:"env_var" mapstructure:"env_var"`
}

// GeocoderRateLimitConfig controls the token-bucket rate limiter.
type GeocoderRateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second" mapstructure:"requests_per_second"`
	Burst             int     `yaml:"burst" mapstructure:"burst"`
}

// GeocoderCacheConfig controls the optional geocoder KV cache layer.
type GeocoderCacheConfig struct {
	Enabled     bool   `yaml:"enabled" mapstructure:"enabled"`
	TTL         string `yaml:"ttl" mapstructure:"ttl"`
	NegativeTTL string `yaml:"negative_ttl" mapstructure:"negative_ttl"`
	KeyPrefix   string `yaml:"key_prefix" mapstructure:"key_prefix"`
}

// Validate checks that required DatabaseConfig fields are set.
func (c DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("database.host is required")
	}
	if c.Name == "" {
		return fmt.Errorf("database.name is required")
	}
	if c.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if c.Password == "" {
		return fmt.Errorf("database.password is required")
	}
	return nil
}

// Validate checks that required NATSConfig fields are set.
func (c NATSConfig) Validate() error {
	if c.URL == "" {
		return fmt.Errorf("nats.url is required")
	}
	return nil
}

// Validate checks that required ValkeyConfig fields are set.
func (c ValkeyConfig) Validate() error {
	if c.Addr == "" {
		return fmt.Errorf("valkey.addr is required")
	}
	return nil
}

// Validate checks that the LLMConfig is internally consistent.
// If a provider is specified, the corresponding sub-config must have required fields.
func (c LLMConfig) Validate() error {
	switch c.Provider {
	case "openai":
		if c.OpenAI.APIKey == "" {
			return fmt.Errorf("llm.openai.api_key is required when provider is openai")
		}
	case "anthropic":
		if c.Anthropic.APIKey == "" {
			return fmt.Errorf("llm.anthropic.api_key is required when provider is anthropic")
		}
	case "gemini":
		if c.Gemini.APIKey == "" {
			return fmt.Errorf("llm.gemini.api_key is required when provider is gemini")
		}
	case "xai":
		if c.XAI.APIKey == "" {
			return fmt.Errorf("llm.xai.api_key is required when provider is xai")
		}
	case "zai":
		if c.ZAI.APIKey == "" {
			return fmt.Errorf("llm.zai.api_key is required when provider is zai")
		}
	case "ollama":
		if c.Ollama.BaseURL == "" {
			return fmt.Errorf("llm.ollama.base_url is required when provider is ollama")
		}
	case "lmstudio":
		if c.LMStudio.BaseURL == "" {
			return fmt.Errorf("llm.lmstudio.base_url is required when provider is lmstudio")
		}
	case "":
		// No provider set; nothing to validate.
	default:
		return fmt.Errorf("llm.provider %q is not a recognized provider", c.Provider)
	}
	return nil
}

// FilteringMode is a type alias for domain.FilteringModeValue.
// The canonical definition lives in internal/domain/enums.go.
type FilteringMode = domain.FilteringModeValue

// Filtering mode constants delegated to the domain package.
const (
	FilteringViewport = domain.FilteringViewport
	FilteringAll      = domain.FilteringAll
)

// ObservationRecordMode is a type alias for domain.ObservationRecordMode.
// The canonical definition lives in internal/domain/enums.go.
type ObservationRecordMode = domain.ObservationRecordMode

// Observation record mode constants delegated to the domain package.
const (
	RecordAppend = domain.RecordAppend
	RecordUpsert = domain.RecordUpsert
	RecordDedupe = domain.RecordDedupe
)

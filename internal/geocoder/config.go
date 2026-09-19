package geocoder

import (
	"fmt"
	"time"
)

const (
	defaultRPS         = 1.0
	defaultBurst       = 5
	defaultTTL         = 168 * time.Hour // 7 days
	defaultNegativeTTL = time.Hour       // short: lets transient no-match self-heal
	defaultKeyPrefix   = "geocoder:"
)

// Config holds the complete configuration for the geocoder factory.
type Config struct {
	Provider  string          `yaml:"provider" mapstructure:"provider"`
	Nominatim NominatimConfig `yaml:"nominatim" mapstructure:"nominatim"`
	Google    ProviderConfig  `yaml:"google" mapstructure:"google"`
	Mapbox    ProviderConfig  `yaml:"mapbox" mapstructure:"mapbox"`
	RateLimit RateLimitConfig `yaml:"rate_limit" mapstructure:"rate_limit"`
	Cache     CacheConfig     `yaml:"cache" mapstructure:"cache"`
}

// NominatimConfig holds Nominatim-specific settings.
type NominatimConfig struct {
	BaseURL   string `yaml:"base_url" mapstructure:"base_url"`
	UserAgent string `yaml:"user_agent" mapstructure:"user_agent"`
}

// ProviderConfig holds generic API-key-based provider settings.
type ProviderConfig struct {
	EnvVar string `yaml:"env_var" mapstructure:"env_var"`
}

// RateLimitConfig controls the token-bucket rate limiter applied to all providers.
type RateLimitConfig struct {
	RequestsPerSecond float64 `yaml:"requests_per_second" mapstructure:"requests_per_second"`
	Burst             int     `yaml:"burst" mapstructure:"burst"`
}

// CacheConfig controls the optional KVCache layer.
type CacheConfig struct {
	Enabled bool   `yaml:"enabled" mapstructure:"enabled"`
	TTL     string `yaml:"ttl" mapstructure:"ttl"`
	// NegativeTTL is how long a no-match is remembered (empty = 1h default).
	NegativeTTL string `yaml:"negative_ttl" mapstructure:"negative_ttl"`
	KeyPrefix   string `yaml:"key_prefix" mapstructure:"key_prefix"`
}

// NewFromConfig builds a fully-decorated Geocoder from cfg.
// The decoration order (innermost → outermost) is:
//
//  1. Provider (e.g. Nominatim)
//  2. Rate limiter
//  3. Cache (if enabled and kvCache != nil)
//
// kvCache may be nil; in that case caching is skipped regardless of cfg.Cache.Enabled.
func NewFromConfig(cfg Config, kvCache KVCache) (Geocoder, error) {
	var inner Geocoder

	switch cfg.Provider {
	case "none", "disabled":
		// Offline mode: no external provider. Geo-resolution still works via the
		// LLM coordinates (tier 1) and the country-centroid fallback (tier 3).
		return nil, nil
	case "nominatim", "":
		inner = NewNominatim(cfg.Nominatim.BaseURL, cfg.Nominatim.UserAgent)
	default:
		return nil, fmt.Errorf("geocoder: unsupported provider %q", cfg.Provider)
	}

	// Apply rate limiter.
	rps := cfg.RateLimit.RequestsPerSecond
	if rps <= 0 {
		rps = defaultRPS
	}
	burst := cfg.RateLimit.Burst
	if burst <= 0 {
		burst = defaultBurst
	}
	var g Geocoder = NewRateLimitedGeocoder(inner, rps, burst)

	// Apply cache if enabled and a cache backend is provided.
	if cfg.Cache.Enabled && kvCache != nil {
		ttl := parseTTL(cfg.Cache.TTL, defaultTTL)
		negativeTTL := parseTTL(cfg.Cache.NegativeTTL, defaultNegativeTTL)
		prefix := cfg.Cache.KeyPrefix
		if prefix == "" {
			prefix = defaultKeyPrefix
		}
		g = NewCachedGeocoder(g, kvCache, ttl, negativeTTL, prefix)
	}

	return g, nil
}

// parseTTL parses a duration string, returning fallback on empty or invalid input.
func parseTTL(s string, fallback time.Duration) time.Duration {
	if s == "" {
		return fallback
	}
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}

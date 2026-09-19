package main

import (
	"github.com/Alevsk/respondent/internal/config"
	"github.com/Alevsk/respondent/internal/geocoder"
)

// toGeocoderConfig maps the shared config.GeocoderConfig (parsed from the
// `geocoder:` block) onto the geocoder package's own Config. Country centroids
// are handled separately (merged onto the built-in table) and so are omitted here.
func toGeocoderConfig(c config.GeocoderConfig) geocoder.Config {
	return geocoder.Config{
		Provider: c.Provider,
		Nominatim: geocoder.NominatimConfig{
			BaseURL:   c.Nominatim.BaseURL,
			UserAgent: c.Nominatim.UserAgent,
		},
		RateLimit: geocoder.RateLimitConfig{
			RequestsPerSecond: c.RateLimit.RequestsPerSecond,
			Burst:             c.RateLimit.Burst,
		},
		Cache: geocoder.CacheConfig{
			Enabled:     c.Cache.Enabled,
			TTL:         c.Cache.TTL,
			NegativeTTL: c.Cache.NegativeTTL,
			KeyPrefix:   c.Cache.KeyPrefix,
		},
	}
}

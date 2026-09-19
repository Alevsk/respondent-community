package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Alevsk/respondent/internal/config"
)

func TestToGeocoderConfig_MapsAllFields(t *testing.T) {
	in := config.GeocoderConfig{
		Provider:  "nominatim",
		Nominatim: config.GeocoderNominatimConfig{BaseURL: "https://nom.example", UserAgent: "ua/1.0"},
		RateLimit: config.GeocoderRateLimitConfig{RequestsPerSecond: 2.5, Burst: 7},
		Cache:     config.GeocoderCacheConfig{Enabled: true, TTL: "24h", NegativeTTL: "30m", KeyPrefix: "geo:"},
	}

	got := toGeocoderConfig(in)

	assert.Equal(t, "nominatim", got.Provider)
	assert.Equal(t, "https://nom.example", got.Nominatim.BaseURL)
	assert.Equal(t, "ua/1.0", got.Nominatim.UserAgent)
	assert.Equal(t, 2.5, got.RateLimit.RequestsPerSecond)
	assert.Equal(t, 7, got.RateLimit.Burst)
	assert.True(t, got.Cache.Enabled)
	assert.Equal(t, "24h", got.Cache.TTL)
	assert.Equal(t, "30m", got.Cache.NegativeTTL)
	assert.Equal(t, "geo:", got.Cache.KeyPrefix)
}

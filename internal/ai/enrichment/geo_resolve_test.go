package enrichment

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/geocoder"
)

// stubGeocoder returns a fixed result or error and counts forward calls.
type stubGeocoder struct {
	result *geocoder.Result
	err    error
	calls  int
}

func (s *stubGeocoder) Geocode(_ context.Context, _ string) (*geocoder.Result, error) {
	s.calls++
	return s.result, s.err
}
func (s *stubGeocoder) ReverseGeocode(_ context.Context, _, _ float64) (*geocoder.Result, error) {
	return s.result, s.err
}
func (s *stubGeocoder) Name() string { return "stub" }

func TestResolveCoordinates_Tier1_HighConfidenceLLM(t *testing.T) {
	llmResult := map[string]any{
		"lat": 50.45, "lon": 30.52,
		"location_confidence": 0.92, "country": "Ukraine",
	}
	lat, lon, source, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		nil, nil,
	)
	require.NoError(t, err)
	assert.InDelta(t, 50.45, lat, 0.001)
	assert.InDelta(t, 30.52, lon, 0.001)
	assert.Equal(t, "llm", source)
}

func TestResolveCoordinates_Tier2_GeocoderFallback(t *testing.T) {
	llmResult := map[string]any{
		"lat": 0.0, "lon": 0.0,
		"location_confidence": 0.3, "city": "Kharkiv", "country": "Ukraine",
	}
	geo := &stubGeocoder{result: &geocoder.Result{Lat: 49.99, Lon: 36.23, Source: "nominatim"}}
	lat, lon, source, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		geo, nil,
	)
	require.NoError(t, err)
	assert.InDelta(t, 49.99, lat, 0.001)
	assert.InDelta(t, 36.23, lon, 0.001)
	assert.Equal(t, "geocoder:stub", source)
}

func TestResolveCoordinates_Tier3_CentroidFallback(t *testing.T) {
	llmResult := map[string]any{"location_confidence": 0.2, "country_iso3": "UKR"}
	lookup := map[string][2]float64{"UKR": {48.38, 31.17}}
	lat, lon, source, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		&stubGeocoder{result: nil}, lookup,
	)
	require.NoError(t, err)
	assert.InDelta(t, 48.38, lat, 0.001)
	assert.InDelta(t, 31.17, lon, 0.001)
	assert.Equal(t, "centroid", source)
}

func TestResolveCoordinates_NoGeoContent_ReturnsSentinel(t *testing.T) {
	// Non-geographic content: low confidence, no locality, no centroid match.
	llmResult := map[string]any{"location_confidence": 0.1, "country": "Global"}
	_, _, _, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		&stubGeocoder{result: nil}, nil,
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrNoGeoContent)
}

func TestResolveCoordinates_CountryOnly_SkipsGeocoder(t *testing.T) {
	// A bare country (no city/region) must not spend an external geocoder call;
	// it resolves via the centroid map instead.
	llmResult := map[string]any{"location_confidence": 0.3, "country": "Global", "country_iso3": "USA"}
	geo := &stubGeocoder{result: &geocoder.Result{Lat: 1, Lon: 1}}
	lat, lon, source, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		geo, map[string][2]float64{"USA": {37.09, -95.71}},
	)
	require.NoError(t, err)
	assert.Equal(t, "centroid", source)
	assert.InDelta(t, 37.09, lat, 0.001)
	assert.InDelta(t, -95.71, lon, 0.001)
	assert.Equal(t, 0, geo.calls, "country-only must not call the external geocoder")
}

func TestResolveCoordinates_GeocoderError_CentroidStillWins(t *testing.T) {
	// A geocoder transport failure must not block the centroid fallback.
	llmResult := map[string]any{"location_confidence": 0.3, "city": "Kharkiv", "country_iso3": "UKR"}
	geo := &stubGeocoder{err: errors.New("nominatim down")}
	_, _, source, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		geo, map[string][2]float64{"UKR": {48.38, 31.17}},
	)
	require.NoError(t, err)
	assert.Equal(t, "centroid", source)
}

func TestResolveCoordinates_GeocoderError_NoFallback_PropagatesError(t *testing.T) {
	// When the geocoder fails and nothing else resolves, the genuine error is
	// surfaced (not ErrNoGeoContent) so operators see a real outage.
	llmResult := map[string]any{"location_confidence": 0.3, "city": "Kharkiv"}
	geo := &stubGeocoder{err: errors.New("nominatim down")}
	_, _, _, err := ResolveCoordinates(
		context.Background(), llmResult,
		&PatchCoordinatesCfg{LatField: "lat", LonField: "lon", ConfidenceField: "location_confidence", MinConfidence: 0.7},
		geo, nil,
	)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNoGeoContent)
	assert.Contains(t, err.Error(), "nominatim down")
}

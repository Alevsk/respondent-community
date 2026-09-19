package enrichment

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Alevsk/respondent/internal/geocoder"
)

// ErrNoGeoContent signals that an entity simply has no resolvable location — the
// LLM gave no usable coordinates, no specific locality to geocode, and no
// country centroid matched. It is an expected outcome for non-geographic content
// (e.g. a global cybersecurity advisory), not a failure, so callers log it at
// debug rather than warn. A genuine geocoder transport error is returned
// instead, so the two can be told apart.
var ErrNoGeoContent = errors.New("geo resolution: no resolvable location")

// PatchCoordinatesCfg holds the field names and threshold used by ResolveCoordinates.
// It is derived from PatchCoordinatesConfig in the YAML operation config.
type PatchCoordinatesCfg struct {
	LatField        string
	LonField        string
	ConfidenceField string
	MinConfidence   float64
}

// ResolveCoordinates attempts to obtain valid geographic coordinates from three
// sources in descending preference order:
//
//  1. LLM-provided lat/lon when confidence >= cfg.MinConfidence and the
//     coordinates are non-zero and within valid ranges.
//  2. Geocoding API — only when a specific locality (city or region) is present.
//     A bare country name is left to tier 3, which is instant and avoids spending
//     a rate-limited external call (and an external dependency) on data the
//     centroid map already covers.
//  3. Country centroid lookup via the centroids map (keyed by ISO 3166-1 alpha-3).
//
// Returns ErrNoGeoContent when there was simply nothing to resolve, or the
// underlying geocoder error when the external provider failed — letting callers
// log the expected case quietly and surface genuine outages.
func ResolveCoordinates(
	ctx context.Context,
	llmResult map[string]any,
	cfg *PatchCoordinatesCfg,
	geo geocoder.Geocoder,
	centroids map[string][2]float64,
) (lat, lon float64, source string, err error) {
	// Tier 1: LLM coordinates with high confidence.
	confidence := extractFloat(llmResult, cfg.ConfidenceField)
	if confidence >= cfg.MinConfidence {
		llmLat := extractFloat(llmResult, cfg.LatField)
		llmLon := extractFloat(llmResult, cfg.LonField)
		if llmLat >= -90 && llmLat <= 90 && llmLon >= -180 && llmLon <= 180 && (llmLat != 0 || llmLon != 0) {
			return llmLat, llmLon, "llm", nil
		}
	}

	// Tier 2: Geocoding API — only for a specific locality, never a bare country.
	var geoErr error
	if geo != nil {
		if query := buildLocalityQuery(llmResult); query != "" {
			result, gErr := geo.Geocode(ctx, query)
			switch {
			case gErr != nil:
				geoErr = gErr // remember; still try the centroid fallback below
			case result != nil:
				return result.Lat, result.Lon, "geocoder:" + geo.Name(), nil
			}
		}
	}

	// Tier 3: Country centroid.
	if centroids != nil {
		iso3 := extractString(llmResult, "country_iso3")
		if coords, ok := centroids[iso3]; ok {
			return coords[0], coords[1], "centroid", nil
		}
	}

	if geoErr != nil {
		return 0, 0, "", fmt.Errorf("geo resolution: geocoder failed: %w", geoErr)
	}
	return 0, 0, "", ErrNoGeoContent
}

// buildLocalityQuery constructs a geocoding query from the LLM result, but only
// when a specific sub-country locality (city or region) is present — that is what
// makes an external lookup worthwhile. The country is appended for disambiguation
// when a locality exists. A country alone returns "" so the caller skips tier 2.
func buildLocalityQuery(llmResult map[string]any) string {
	city := extractString(llmResult, "city")
	region := extractString(llmResult, "region")
	if city == "" && region == "" {
		return ""
	}
	parts := make([]string, 0, 3)
	if city != "" {
		parts = append(parts, city)
	}
	if region != "" {
		parts = append(parts, region)
	}
	if country := extractString(llmResult, "country"); country != "" {
		parts = append(parts, country)
	}
	return strings.Join(parts, ", ")
}

// extractFloat retrieves a float64 value from a map[string]any, returning 0
// when the key is absent or the value is not a numeric type.
func extractFloat(m map[string]any, key string) float64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// extractString retrieves a string value from a map[string]any, returning ""
// when the key is absent or the value is not a string.
func extractString(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Package geocoder provides geocoding and reverse geocoding abstractions
// for resolving location names to coordinates and vice versa.
package geocoder

import "context"

// Result holds the output of a geocoding or reverse geocoding operation.
type Result struct {
	Lat        float64
	Lon        float64
	Country    string // ISO 3166-1 alpha-2 country code (e.g. "UA")
	Region     string
	City       string
	Confidence float64 // 0.0–1.0
	Source     string  // provider name
}

// Geocoder is the interface implemented by all geocoding providers.
type Geocoder interface {
	// Geocode resolves a human-readable query (place name, address, etc.) to
	// geographic coordinates and location metadata. Returns nil, nil when no
	// result is found.
	Geocode(ctx context.Context, query string) (*Result, error)

	// ReverseGeocode resolves a latitude/longitude pair to location metadata.
	// Returns nil, nil when no result is found.
	ReverseGeocode(ctx context.Context, lat, lon float64) (*Result, error)

	// Name returns the provider identifier, e.g. "nominatim".
	Name() string
}

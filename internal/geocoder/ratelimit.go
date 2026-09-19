package geocoder

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimitedGeocoder wraps a Geocoder with a token-bucket rate limiter to
// enforce a maximum request rate. This is required when using public APIs such
// as Nominatim that have strict per-IP usage policies.
type RateLimitedGeocoder struct {
	inner   Geocoder
	limiter *rate.Limiter
}

// NewRateLimitedGeocoder creates a RateLimitedGeocoder that limits calls to the
// inner Geocoder to rps requests per second with a burst of burst tokens.
func NewRateLimitedGeocoder(inner Geocoder, rps float64, burst int) *RateLimitedGeocoder {
	return &RateLimitedGeocoder{
		inner:   inner,
		limiter: rate.NewLimiter(rate.Limit(rps), burst),
	}
}

// Name implements Geocoder by delegating to the inner provider.
func (r *RateLimitedGeocoder) Name() string {
	return r.inner.Name()
}

// Geocode implements Geocoder. It blocks until the rate limiter grants a token
// or the context is cancelled.
func (r *RateLimitedGeocoder) Geocode(ctx context.Context, query string) (*Result, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.inner.Geocode(ctx, query)
}

// ReverseGeocode implements Geocoder. It blocks until the rate limiter grants a
// token or the context is cancelled.
func (r *RateLimitedGeocoder) ReverseGeocode(ctx context.Context, lat, lon float64) (*Result, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.inner.ReverseGeocode(ctx, lat, lon)
}

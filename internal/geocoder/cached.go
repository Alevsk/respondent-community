package geocoder

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// errCacheMiss is a sentinel returned by KVCache.Get when the key is absent.
var errCacheMiss = errors.New("cache miss")

// KVCache is the minimal key/value cache interface required by CachedGeocoder.
// The Valkey-backed implementation in internal/cache satisfies this interface.
type KVCache interface {
	// Get retrieves a value by key. Returns errCacheMiss when not found.
	Get(ctx context.Context, key string) (string, error)
	// Set stores a value with an optional TTL. A zero TTL means no expiry.
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// negativeCacheMarker is the sentinel stored (with negativeTTL) to remember that
// a query yielded no match, so repeated misses don't re-hit the upstream
// provider. The short TTL lets a transient upstream outage self-heal.
const negativeCacheMarker = "\x00nomatch\x00"

// CachedGeocoder wraps a Geocoder with a KVCache to avoid redundant network calls.
// Successful matches are cached for ttl; no-match results are cached for the
// shorter negativeTTL (when > 0) so a flood of unlocatable queries does not
// hammer the upstream provider, while still self-healing after the short window.
type CachedGeocoder struct {
	inner       Geocoder
	cache       KVCache
	ttl         time.Duration
	negativeTTL time.Duration
	keyPrefix   string
}

// NewCachedGeocoder creates a CachedGeocoder that delegates to inner. Successful
// results are cached for ttl; no-match results for negativeTTL (0 disables
// negative caching). All cache keys are prefixed with keyPrefix for namespace
// isolation per deployment.
func NewCachedGeocoder(inner Geocoder, cache KVCache, ttl, negativeTTL time.Duration, keyPrefix string) *CachedGeocoder {
	return &CachedGeocoder{
		inner:       inner,
		cache:       cache,
		ttl:         ttl,
		negativeTTL: negativeTTL,
		keyPrefix:   keyPrefix,
	}
}

// Name implements Geocoder by delegating to the inner provider.
func (c *CachedGeocoder) Name() string {
	return c.inner.Name()
}

// Geocode implements Geocoder with a read-through cache strategy.
func (c *CachedGeocoder) Geocode(ctx context.Context, query string) (*Result, error) {
	key := c.forwardKey(query)

	if r, negative, ok := c.lookup(ctx, key); ok {
		if negative {
			return nil, nil
		}
		return r, nil
	}

	// Cache miss — call inner provider.
	result, err := c.inner.Geocode(ctx, query)
	if err != nil {
		return nil, err
	}
	c.store(ctx, key, result)
	return result, nil
}

// ReverseGeocode implements Geocoder with a read-through cache strategy.
func (c *CachedGeocoder) ReverseGeocode(ctx context.Context, lat, lon float64) (*Result, error) {
	key := c.reverseKey(lat, lon)

	if r, negative, ok := c.lookup(ctx, key); ok {
		if negative {
			return nil, nil
		}
		return r, nil
	}

	result, err := c.inner.ReverseGeocode(ctx, lat, lon)
	if err != nil {
		return nil, err
	}
	c.store(ctx, key, result)
	return result, nil
}

// forwardKey builds the cache key for a forward geocode query.
// Uses a SHA-256 prefix to keep keys compact and safe for any cache backend.
func (c *CachedGeocoder) forwardKey(query string) string {
	h := sha256.Sum256([]byte(query))
	return fmt.Sprintf("%sfwd:%x", c.keyPrefix, h[:16]) // 32 hex chars
}

// reverseKey builds the cache key for a reverse geocode request.
// Coordinates are rounded to 4 decimal places (~11 m precision) so that
// near-identical coordinates map to the same cache entry.
func (c *CachedGeocoder) reverseKey(lat, lon float64) string {
	return fmt.Sprintf("%srev:%.4f,%.4f", c.keyPrefix, lat, lon)
}

// lookup retrieves a cache entry. ok=false is a cache miss (or corrupt entry);
// when ok=true, negative=true means a cached no-match and result is nil.
func (c *CachedGeocoder) lookup(ctx context.Context, key string) (result *Result, negative bool, ok bool) {
	raw, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, false, false
	}
	if raw == negativeCacheMarker {
		return nil, true, true
	}
	var r Result
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, false, false // treat a corrupt entry as a miss
	}
	return &r, false, true
}

// store caches a provider response: the marshaled Result for a match, or the
// negative marker (when negativeTTL > 0) for a no-match.
func (c *CachedGeocoder) store(ctx context.Context, key string, result *Result) {
	if result == nil {
		if c.negativeTTL > 0 {
			_ = c.cache.Set(ctx, key, negativeCacheMarker, c.negativeTTL)
		}
		return
	}
	c.set(ctx, key, result)
}

// set marshals result to JSON and writes it to the cache. Errors are silently
// swallowed — a cache write failure should not prevent the caller from getting
// a valid response.
func (c *CachedGeocoder) set(ctx context.Context, key string, result *Result) {
	data, err := json.Marshal(result)
	if err != nil {
		return
	}
	_ = c.cache.Set(ctx, key, string(data), c.ttl)
}

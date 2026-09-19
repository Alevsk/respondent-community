package geocoder

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCache is a thread-safe in-memory KVCache for testing.
type mockCache struct {
	mu    sync.RWMutex
	store map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]string)}
}

func (m *mockCache) Get(_ context.Context, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.store[key]
	if !ok {
		return "", errCacheMiss
	}
	return v, nil
}

func (m *mockCache) Set(_ context.Context, key, value string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store[key] = value
	return nil
}

// mockGeocoder tracks call count and returns a fixed result or nil.
type mockGeocoder struct {
	mu        sync.Mutex
	callCount int
	result    *Result
}

func (m *mockGeocoder) Geocode(_ context.Context, _ string) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.result, nil
}

func (m *mockGeocoder) ReverseGeocode(_ context.Context, _, _ float64) (*Result, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	return m.result, nil
}

func (m *mockGeocoder) Name() string { return "mock" }

func (m *mockGeocoder) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func TestCachedGeocoder_CacheHit(t *testing.T) {
	fixedResult := &Result{
		Lat:    51.5074,
		Lon:    -0.1278,
		City:   "London",
		Source: "mock",
	}

	inner := &mockGeocoder{result: fixedResult}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, 24*time.Hour, time.Hour, "test:")

	ctx := context.Background()

	// First call: cache miss, inner should be called.
	res1, err := cached.Geocode(ctx, "London")
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.Equal(t, "London", res1.City)
	assert.Equal(t, 1, inner.count())

	// Second call: cache hit, inner must NOT be called again.
	res2, err := cached.Geocode(ctx, "London")
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, "London", res2.City)
	assert.Equal(t, 1, inner.count(), "inner should not be called on cache hit")
}

func TestCachedGeocoder_NoMatch_NegativelyCached(t *testing.T) {
	// inner returns nil (no match). With negativeTTL > 0 the miss is remembered,
	// so a flood of unlocatable queries does not re-hit the provider.
	inner := &mockGeocoder{result: nil}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, 24*time.Hour, time.Hour, "test:")

	ctx := context.Background()

	res1, err := cached.Geocode(ctx, "Nowhere")
	require.NoError(t, err)
	assert.Nil(t, res1)
	assert.Equal(t, 1, inner.count())

	// Second call — the no-match was cached, inner must NOT be called again.
	res2, err := cached.Geocode(ctx, "Nowhere")
	require.NoError(t, err)
	assert.Nil(t, res2)
	assert.Equal(t, 1, inner.count(), "no-match should be negatively cached")
}

func TestCachedGeocoder_NoMatch_NotCachedWhenNegativeTTLZero(t *testing.T) {
	// negativeTTL == 0 disables negative caching: every miss re-hits the provider.
	inner := &mockGeocoder{result: nil}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, 24*time.Hour, 0, "test:")

	ctx := context.Background()

	_, err := cached.Geocode(ctx, "Nowhere")
	require.NoError(t, err)
	_, err = cached.Geocode(ctx, "Nowhere")
	require.NoError(t, err)
	assert.Equal(t, 2, inner.count(), "negativeTTL=0 must disable negative caching")
}

func TestCachedGeocoder_ReverseGeocode_NoMatch_NegativelyCached(t *testing.T) {
	// ReverseGeocode must negatively cache no-match results symmetrically with Geocode.
	inner := &mockGeocoder{result: nil}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, 24*time.Hour, time.Hour, "test:")

	ctx := context.Background()

	res1, err := cached.ReverseGeocode(ctx, 10.0, 20.0)
	require.NoError(t, err)
	assert.Nil(t, res1)
	assert.Equal(t, 1, inner.count())

	res2, err := cached.ReverseGeocode(ctx, 10.0, 20.0)
	require.NoError(t, err)
	assert.Nil(t, res2)
	assert.Equal(t, 1, inner.count(), "reverse no-match should be negatively cached")
}

func TestCachedGeocoder_Name(t *testing.T) {
	inner := &mockGeocoder{}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, time.Hour, time.Hour, "")
	assert.Equal(t, "mock", cached.Name())
}

func TestCachedGeocoder_ReverseGeocode_CacheHit(t *testing.T) {
	fixedResult := &Result{
		Lat:    48.8566,
		Lon:    2.3522,
		City:   "Paris",
		Source: "mock",
	}

	inner := &mockGeocoder{result: fixedResult}
	cache := newMockCache()
	cached := NewCachedGeocoder(inner, cache, 24*time.Hour, time.Hour, "test:")

	ctx := context.Background()

	// First call hits inner.
	res1, err := cached.ReverseGeocode(ctx, 48.8566, 2.3522)
	require.NoError(t, err)
	require.NotNil(t, res1)
	assert.Equal(t, "Paris", res1.City)
	assert.Equal(t, 1, inner.count())

	// Second call with same coordinates hits cache.
	res2, err := cached.ReverseGeocode(ctx, 48.8566, 2.3522)
	require.NoError(t, err)
	require.NotNil(t, res2)
	assert.Equal(t, "Paris", res2.City)
	assert.Equal(t, 1, inner.count(), "inner should not be called on reverse cache hit")
}

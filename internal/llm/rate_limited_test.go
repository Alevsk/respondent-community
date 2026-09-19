package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// newCallCountProvider returns a mockProvider that counts Complete calls.
// It uses the existing mockProvider struct from provider_test.go.
func newCallCountProvider(name string) *callCountProvider {
	return &callCountProvider{name: name}
}

// callCountProvider is a Provider that counts calls and optionally returns errors.
// Separate from the existing mockProvider to avoid redeclaration conflicts.
type callCountProvider struct {
	name      string
	calls     int
	err       error
	healthErr error
}

func (m *callCountProvider) Complete(_ context.Context, _ *CompletionRequest) (*CompletionResponse, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &CompletionResponse{Content: "ok", Model: "test"}, nil
}

func (m *callCountProvider) Name() string                              { return m.name }
func (m *callCountProvider) SupportsProvider(providerType string) bool { return providerType == m.name }
func (m *callCountProvider) HealthCheck(_ context.Context) error       { return m.healthErr }

func TestRateLimitedProvider_PassesThrough(t *testing.T) {
	inner := newCallCountProvider("test")
	p := NewRateLimitedProvider(inner, 100, nil, nil)

	resp, err := p.Complete(context.Background(), &CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, 1, inner.calls)
}

func TestRateLimitedProvider_NamePassesThrough(t *testing.T) {
	inner := newCallCountProvider("openai")
	p := NewRateLimitedProvider(inner, 10, nil, nil)

	assert.Equal(t, "openai", p.Name())
	assert.True(t, p.SupportsProvider("openai"))
	assert.False(t, p.SupportsProvider("anthropic"))
}

func TestRateLimitedProvider_HealthCheckPassesThrough(t *testing.T) {
	inner := newCallCountProvider("test")
	p := NewRateLimitedProvider(inner, 10, nil, nil)

	assert.NoError(t, p.HealthCheck(context.Background()))
}

func TestRateLimitedProvider_RespectsRateLimit(t *testing.T) {
	inner := newCallCountProvider("test")
	// 1 request per second, burst 1.
	p := NewRateLimitedProvider(inner, 1, nil, nil)

	// First call should succeed immediately.
	_, err := p.Complete(context.Background(), &CompletionRequest{})
	require.NoError(t, err)

	// Second call should block. Use a tight context to prove it waits.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = p.Complete(ctx, &CompletionRequest{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	// Inner provider should NOT have been called for the timed-out request.
	assert.Equal(t, 1, inner.calls)
}

func TestRateLimitedProvider_ZeroRate_ReturnsUnwrapped(t *testing.T) {
	inner := newCallCountProvider("test")
	p := NewRateLimitedProvider(inner, 0, nil, nil)

	// Should be the exact same pointer — no wrapping.
	assert.Equal(t, inner, p)
}

func TestRateLimitedProvider_SharedLimiter(t *testing.T) {
	inner := newCallCountProvider("test")
	limiter := rate.NewLimiter(rate.Limit(1), 1)

	p := NewRateLimitedProvider(inner, 1, limiter, nil)

	// Exhaust the token externally.
	require.True(t, limiter.Allow())

	// Provider should block because the shared limiter is exhausted.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := p.Complete(ctx, &CompletionRequest{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRateLimitedProvider_InnerError(t *testing.T) {
	inner := &callCountProvider{name: "test", err: ErrRateLimited}
	p := NewRateLimitedProvider(inner, 100, nil, nil)

	_, err := p.Complete(context.Background(), &CompletionRequest{})
	assert.ErrorIs(t, err, ErrRateLimited)
}

// rateLimitedMockRegistry is a minimal Registry for testing RateLimitedRegistry.
type rateLimitedMockRegistry struct {
	provider Provider
	err      error
}

func (r *rateLimitedMockRegistry) Register(_ Provider)                    {}
func (r *rateLimitedMockRegistry) GetProvider(_ string) (Provider, error) { return r.provider, r.err }
func (r *rateLimitedMockRegistry) ListProviders() []string                { return []string{"test"} }
func (r *rateLimitedMockRegistry) GetPreferredProvider(_ context.Context) (Provider, error) {
	return r.provider, r.err
}

func TestRateLimitedRegistry_GetPreferredProvider_WrapsProvider(t *testing.T) {
	inner := newCallCountProvider("test")
	reg := &rateLimitedMockRegistry{provider: inner}
	limiter := rate.NewLimiter(rate.Limit(1), 1)

	wrapped := NewRateLimitedRegistry(reg, limiter, nil)

	p, err := wrapped.GetPreferredProvider(context.Background())
	require.NoError(t, err)

	// The returned provider should be rate-limited, not the raw inner.
	_, ok := p.(*RateLimitedProvider)
	assert.True(t, ok, "expected *RateLimitedProvider, got %T", p)
	assert.Equal(t, "test", p.Name())
}

func TestRateLimitedRegistry_GetPreferredProvider_SharesLimiter(t *testing.T) {
	inner := newCallCountProvider("test")
	reg := &rateLimitedMockRegistry{provider: inner}
	limiter := rate.NewLimiter(rate.Limit(1), 1)

	wrapped := NewRateLimitedRegistry(reg, limiter, nil)

	p, err := wrapped.GetPreferredProvider(context.Background())
	require.NoError(t, err)

	// Exhaust the shared limiter.
	require.True(t, limiter.Allow())

	// The provider from the registry should also be blocked.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = p.Complete(ctx, &CompletionRequest{})
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestRateLimitedRegistry_GetPreferredProvider_PropagatesError(t *testing.T) {
	reg := &rateLimitedMockRegistry{err: ErrProviderUnavailable}
	limiter := rate.NewLimiter(rate.Limit(10), 10)

	wrapped := NewRateLimitedRegistry(reg, limiter, nil)

	_, err := wrapped.GetPreferredProvider(context.Background())
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestRateLimitedRegistry_PassThroughMethods(t *testing.T) {
	inner := newCallCountProvider("test")
	reg := &rateLimitedMockRegistry{provider: inner}
	limiter := rate.NewLimiter(rate.Limit(10), 10)

	wrapped := NewRateLimitedRegistry(reg, limiter, nil)

	assert.Equal(t, []string{"test"}, wrapped.ListProviders())

	p, err := wrapped.GetProvider("test")
	require.NoError(t, err)
	assert.Equal(t, inner, p)
}

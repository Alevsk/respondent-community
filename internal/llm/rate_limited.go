package llm

import (
	"context"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
)

// RateLimitedProvider wraps a Provider with a token-bucket rate limiter.
// All callers sharing the same limiter instance share a single global budget.
type RateLimitedProvider struct {
	inner   Provider
	limiter *rate.Limiter
	logger  zerolog.Logger
}

// NewRateLimitedProvider wraps inner with a token-bucket rate limiter.
// ratePerSec sets the sustained rate (tokens/sec) and burst size.
// If ratePerSec <= 0, the inner provider is returned unwrapped (no-op).
// If limiter is nil, a new one is created from ratePerSec.
// If logger is nil, a no-op logger is used.
func NewRateLimitedProvider(inner Provider, ratePerSec int, limiter *rate.Limiter, logger *zerolog.Logger) Provider {
	if ratePerSec <= 0 {
		return inner
	}
	if limiter == nil {
		limiter = rate.NewLimiter(rate.Limit(ratePerSec), ratePerSec)
	}
	var log zerolog.Logger
	if logger != nil {
		log = *logger
	} else {
		log = zerolog.Nop()
	}
	return &RateLimitedProvider{inner: inner, limiter: limiter, logger: log}
}

// Complete waits for a rate limiter token, then delegates to the inner provider.
// If the rate limiter cannot grant a token before the context deadline, the
// context error (DeadlineExceeded or Canceled) is returned directly so callers
// can use errors.Is for standard context sentinel values.
func (p *RateLimitedProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if err := p.limiter.Wait(ctx); err != nil {
		// rate.Limiter.Wait returns a plain fmt.Errorf (not wrapping the ctx
		// error) when it determines upfront that the wait would exceed the
		// context deadline. Normalise to the standard sentinel so callers can
		// rely on errors.Is(err, context.DeadlineExceeded / context.Canceled).
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		// Wait also returns immediately with its own error when the remaining
		// deadline is shorter than the required wait duration, before the
		// context actually expires. In that case ctx.Err() is still nil but
		// the context has a deadline — treat it as DeadlineExceeded.
		if _, hasDeadline := ctx.Deadline(); hasDeadline {
			return nil, context.DeadlineExceeded
		}
		return nil, err
	}
	return p.inner.Complete(ctx, req)
}

// Name returns the inner provider's name.
func (p *RateLimitedProvider) Name() string { return p.inner.Name() }

// SupportsProvider delegates to the inner provider.
func (p *RateLimitedProvider) SupportsProvider(providerType string) bool {
	return p.inner.SupportsProvider(providerType)
}

// HealthCheck delegates to the inner provider without rate limiting.
func (p *RateLimitedProvider) HealthCheck(ctx context.Context) error {
	return p.inner.HealthCheck(ctx)
}

// RateLimitedRegistry wraps a Registry so that providers returned by
// GetPreferredProvider are rate-limited. The limiter is shared with any
// RateLimitedProvider instances to enforce a single global budget.
type RateLimitedRegistry struct {
	inner   Registry
	limiter *rate.Limiter
	logger  zerolog.Logger
}

// NewRateLimitedRegistry wraps a Registry with a shared rate limiter.
// If logger is nil, a no-op logger is used.
func NewRateLimitedRegistry(inner Registry, limiter *rate.Limiter, logger *zerolog.Logger) Registry {
	var log zerolog.Logger
	if logger != nil {
		log = *logger
	} else {
		log = zerolog.Nop()
	}
	return &RateLimitedRegistry{inner: inner, limiter: limiter, logger: log}
}

// Register delegates to the inner registry.
func (r *RateLimitedRegistry) Register(provider Provider) { r.inner.Register(provider) }

// GetProvider delegates to the inner registry without wrapping.
func (r *RateLimitedRegistry) GetProvider(providerType string) (Provider, error) {
	return r.inner.GetProvider(providerType)
}

// GetPreferredProvider returns the preferred provider wrapped with rate limiting.
func (r *RateLimitedRegistry) GetPreferredProvider(ctx context.Context) (Provider, error) {
	p, err := r.inner.GetPreferredProvider(ctx)
	if err != nil {
		return nil, err
	}
	return &RateLimitedProvider{inner: p, limiter: r.limiter, logger: r.logger}, nil
}

// ListProviders delegates to the inner registry.
func (r *RateLimitedRegistry) ListProviders() []string { return r.inner.ListProviders() }

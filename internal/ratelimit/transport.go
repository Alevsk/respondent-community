// Package ratelimit provides a per-host, priority-aware HTTP rate limiter.
//
// A single Transport is shared by every HTTP client that talks to a rate-limited
// upstream (e.g. the feeder's global crawl and the realtime on-demand viewport
// fills both hit api.adsb.lol). It paces requests per host with a token bucket so
// their combined rate stays under the upstream's limit, and serves high-priority
// requests (user-driven on-demand fills) ahead of low-priority ones (the
// background crawl). Hosts without a configured limit pass through unthrottled.
package ratelimit

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

type ctxKey int

const priorityKey ctxKey = iota

// WithPriority marks a request's context as high priority, so it acquires a rate
// token ahead of low-priority (background) requests competing for the same host.
func WithPriority(ctx context.Context) context.Context {
	return context.WithValue(ctx, priorityKey, true)
}

func isHighPriority(ctx context.Context) bool {
	v, _ := ctx.Value(priorityKey).(bool)
	return v
}

// hostBucket paces one host and lets high-priority callers go first.
type hostBucket struct {
	lim         *rate.Limiter
	pendingHigh int64 // atomic: number of high-priority waiters
}

// wait blocks until a token is available or ctx is done. A low-priority waiter
// yields while any high-priority request is pending, so on-demand fills preempt
// the background crawl without either being able to permanently starve the bucket.
func (b *hostBucket) wait(ctx context.Context, high bool) error {
	if high {
		atomic.AddInt64(&b.pendingHigh, 1)
		defer atomic.AddInt64(&b.pendingHigh, -1)
		return b.lim.Wait(ctx)
	}
	for atomic.LoadInt64(&b.pendingHigh) > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return b.lim.Wait(ctx)
}

// Transport is an http.RoundTripper that paces outbound requests per host.
type Transport struct {
	base    http.RoundTripper
	mu      sync.Mutex
	buckets map[string]*hostBucket // host -> bucket; a nil value means "unlimited"
	limits  map[string]rate.Limit
	bursts  map[string]int
}

// NewTransport wraps base (defaulting to http.DefaultTransport) with per-host
// rate limiting. Configure host limits via SetHostLimit before traffic starts.
func NewTransport(base http.RoundTripper) *Transport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &Transport{
		base:    base,
		buckets: make(map[string]*hostBucket),
		limits:  make(map[string]rate.Limit),
		bursts:  make(map[string]int),
	}
}

// SetHostLimit configures the sustained request rate (perSecond) and burst for a
// host (e.g. "api.adsb.lol"). A host with no limit configured is not throttled.
func (t *Transport) SetHostLimit(host string, perSecond float64, burst int) {
	if perSecond <= 0 {
		return
	}
	if burst < 1 {
		burst = 1
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.limits[host] = rate.Limit(perSecond)
	t.bursts[host] = burst
	delete(t.buckets, host) // drop any cached bucket so the new limit applies
}

func (t *Transport) bucketFor(host string) *hostBucket {
	t.mu.Lock()
	defer t.mu.Unlock()
	if b, ok := t.buckets[host]; ok {
		return b // may be nil (explicitly unlimited)
	}
	lim, ok := t.limits[host]
	if !ok {
		t.buckets[host] = nil // cache "unlimited" so we don't re-check every request
		return nil
	}
	b := &hostBucket{lim: rate.NewLimiter(lim, t.bursts[host])}
	t.buckets[host] = b
	return b
}

// RoundTrip paces the request against its host's bucket (if any), then delegates.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if b := t.bucketFor(req.URL.Hostname()); b != nil {
		if err := b.wait(req.Context(), isHighPriority(req.Context())); err != nil {
			return nil, err
		}
	}
	return t.base.RoundTrip(req)
}

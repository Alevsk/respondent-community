package ratelimit

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingRT is a base RoundTripper that records the order requests reach it.
type recordingRT struct {
	mu    sync.Mutex
	order []string
	count int64
}

func (r *recordingRT) RoundTrip(req *http.Request) (*http.Response, error) {
	atomic.AddInt64(&r.count, 1)
	r.mu.Lock()
	r.order = append(r.order, req.Header.Get("X-Tag"))
	r.mu.Unlock()
	return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Header: make(http.Header)}, nil
}

func req(t *testing.T, host, tag string) *http.Request {
	t.Helper()
	r, err := http.NewRequest(http.MethodGet, "https://"+host+"/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("X-Tag", tag)
	return r
}

func TestTransport_UnconfiguredHostPassesThrough(t *testing.T) {
	base := &recordingRT{}
	tr := NewTransport(base)
	tr.SetHostLimit("limited.example", 1, 1) // a DIFFERENT host is limited

	start := time.Now()
	for range 5 {
		if _, err := tr.RoundTrip(req(t, "free.example", "f")); err != nil {
			t.Fatal(err)
		}
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Errorf("unconfigured host should not be throttled, took %v", elapsed)
	}
	if atomic.LoadInt64(&base.count) != 5 {
		t.Errorf("expected 5 passthrough requests, got %d", base.count)
	}
}

func TestTransport_RateLimitsConfiguredHost(t *testing.T) {
	base := &recordingRT{}
	tr := NewTransport(base)
	tr.SetHostLimit("api.example", 20, 1) // 20/s, burst 1 → ~50ms between tokens

	start := time.Now()
	for range 4 {
		if _, err := tr.RoundTrip(req(t, "api.example", "a")); err != nil {
			t.Fatal(err)
		}
	}
	// 4 requests at 20/s burst 1: 1 immediate + 3 paced ≈ 3*50ms = 150ms minimum.
	if elapsed := time.Since(start); elapsed < 120*time.Millisecond {
		t.Errorf("expected rate limiting to pace requests, took only %v", elapsed)
	}
}

func TestTransport_LowYieldsToPendingHighs(t *testing.T) {
	base := &recordingRT{}
	tr := NewTransport(base)
	tr.SetHostLimit("api.example", 2, 1) // 2/s, burst 1 → 500ms between tokens

	// Consume the burst token so the contended requests must wait for tokens.
	if _, err := tr.RoundTrip(req(t, "api.example", "warmup")); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	high := func(tag string) {
		defer wg.Done()
		r := req(t, "api.example", tag)
		r = r.WithContext(WithPriority(r.Context()))
		_, _ = tr.RoundTrip(r)
	}

	// Two high-priority requests become pending first (they reserve the next tokens).
	wg.Add(2)
	go high("high1")
	go high("high2")
	time.Sleep(60 * time.Millisecond) // ensure both highs are pending

	// A low-priority request must yield until BOTH highs have been served.
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = tr.RoundTrip(req(t, "api.example", "low"))
	}()

	wg.Wait()

	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.order) == 0 || base.order[len(base.order)-1] != "low" {
		t.Errorf("low must be served last, after both pending highs: order=%v", base.order)
	}
}

func TestWithPriority_FlagRoundTrips(t *testing.T) {
	if !isHighPriority(WithPriority(t.Context())) {
		t.Error("WithPriority context should report high priority")
	}
	if isHighPriority(t.Context()) {
		t.Error("plain context should report low priority")
	}
}

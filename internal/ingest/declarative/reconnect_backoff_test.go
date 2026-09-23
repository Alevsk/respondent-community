package declarative

import (
	"testing"
	"time"
)

// A long-lived stream that keeps failing drives attempt into the hundreds.
// Production logged "attempt":1199 with "delay_ms":0 — the backoff overflowed
// int64 and went negative, which slipped past the maxDelay clamp and turned
// reconnection into a hot loop against a third-party endpoint.
func TestReconnectBackoffStaysBoundedAtHighAttemptCounts(t *testing.T) {
	spec := &ReconnectSpec{}
	max := 60 * time.Second
	for _, attempt := range []int{1, 10, 33, 34, 35, 40, 63, 64, 100, 1199} {
		got := reconnectBackoff(spec, attempt)
		if got <= 0 {
			t.Errorf("attempt %d: delay %v is not positive; reconnect becomes a hot loop", attempt, got)
		}
		if got > max {
			t.Errorf("attempt %d: delay %v exceeds max %v", attempt, got, max)
		}
	}
}

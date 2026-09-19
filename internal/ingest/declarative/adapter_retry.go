package declarative

import (
	"math"
	"math/rand"
	"time"
)

// calculateBackoff computes the retry delay based on the retry spec.
// Uses "equal jitter": delay/2 + rand(0, delay/2) to avoid thundering herd.
func calculateBackoff(spec *RetrySpec, attempt int) time.Duration {
	if spec == nil {
		return time.Second
	}

	initialDelay := spec.InitialDelay.Duration
	if initialDelay <= 0 {
		initialDelay = time.Second
	}

	maxDelay := spec.MaxDelay.Duration
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}

	var delay time.Duration
	switch spec.Backoff {
	case "exponential":
		delay = initialDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	case "fixed":
		delay = initialDelay
	default:
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	// Equal jitter: base = delay/2, jitter = rand(0, delay/2)
	// Result range: [delay/2, delay)
	half := delay / 2
	jitter := time.Duration(rand.Float64() * float64(half))
	return half + jitter
}

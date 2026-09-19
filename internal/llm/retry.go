package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"
)

// DefaultMaxRetries is the default number of retries for LLM calls.
const DefaultMaxRetries = 2

// RetryableComplete calls provider.Complete with exponential backoff and jitter.
// It retries on transient errors (rate limiting, provider unavailability, server errors).
// maxRetries is the number of retry attempts after the initial call (0 means no retries).
func RetryableComplete(ctx context.Context, provider Provider, req *CompletionRequest, maxRetries int) (*CompletionResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second // quadratic backoff
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
			// Add jitter: random duration in [0, 1s).
			jitter := time.Duration(rand.Int64N(int64(time.Second)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff + jitter):
			}
		}

		resp, err := provider.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}

		if !IsRetryable(err) {
			return nil, err
		}
		lastErr = err
	}
	return nil, fmt.Errorf("LLM call failed after %d retries: %w", maxRetries, lastErr)
}

// IsRetryable returns true if the error indicates a transient condition that
// may succeed on retry (rate limiting, provider unavailability, server errors).
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check sentinel errors.
	if errors.Is(err, ErrRateLimited) || errors.Is(err, ErrProviderUnavailable) {
		return true
	}

	// Check for HTTP 5xx or 429 in error message (common pattern in provider errors).
	msg := err.Error()
	if strings.Contains(msg, "status 429") ||
		strings.Contains(msg, "status 500") ||
		strings.Contains(msg, "status 502") ||
		strings.Contains(msg, "status 503") ||
		strings.Contains(msg, "status 504") {
		return true
	}

	// Connection-level errors and timeouts are transient.
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "Client.Timeout") {
		return true
	}

	return false
}

package llm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider is a test double for llm.Provider.
type fakeProvider struct {
	responses []*CompletionResponse
	errors    []error
	callCount atomic.Int32
}

func (f *fakeProvider) Complete(_ context.Context, _ *CompletionRequest) (*CompletionResponse, error) {
	idx := int(f.callCount.Add(1)) - 1
	if idx < len(f.errors) && f.errors[idx] != nil {
		return nil, f.errors[idx]
	}
	if idx < len(f.responses) {
		return f.responses[idx], nil
	}
	return &CompletionResponse{Content: "fallback"}, nil
}

func (f *fakeProvider) Name() string                        { return "fake" }
func (f *fakeProvider) SupportsProvider(_ string) bool      { return true }
func (f *fakeProvider) HealthCheck(_ context.Context) error { return nil }

func TestRetryableComplete_SuccessOnFirstAttempt(t *testing.T) {
	provider := &fakeProvider{
		responses: []*CompletionResponse{{Content: "ok"}},
	}

	resp, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 3)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, int32(1), provider.callCount.Load())
}

func TestRetryableComplete_SuccessAfterRetry(t *testing.T) {
	provider := &fakeProvider{
		errors:    []error{ErrRateLimited, nil},
		responses: []*CompletionResponse{nil, {Content: "retried"}},
	}

	resp, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 3)
	require.NoError(t, err)
	assert.Equal(t, "retried", resp.Content)
	assert.Equal(t, int32(2), provider.callCount.Load())
}

func TestRetryableComplete_ExhaustsRetries(t *testing.T) {
	provider := &fakeProvider{
		errors: []error{ErrRateLimited, ErrRateLimited, ErrRateLimited},
	}

	_, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 2)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call failed after 2 retries")
	assert.ErrorIs(t, err, ErrRateLimited)
	assert.Equal(t, int32(3), provider.callCount.Load()) // initial + 2 retries
}

func TestRetryableComplete_NonRetryableErrorReturnsImmediately(t *testing.T) {
	terminalErr := errors.New("authentication failed: invalid API key")
	provider := &fakeProvider{
		errors: []error{terminalErr},
	}

	_, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 3)
	require.Error(t, err)
	assert.Equal(t, terminalErr, err)
	assert.Equal(t, int32(1), provider.callCount.Load()) // no retries
}

func TestRetryableComplete_ZeroRetries(t *testing.T) {
	provider := &fakeProvider{
		responses: []*CompletionResponse{{Content: "ok"}},
	}

	resp, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 0)
	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
	assert.Equal(t, int32(1), provider.callCount.Load())
}

func TestRetryableComplete_ZeroRetries_Error(t *testing.T) {
	provider := &fakeProvider{
		errors: []error{ErrRateLimited},
	}

	_, err := RetryableComplete(context.Background(), provider, &CompletionRequest{}, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM call failed after 0 retries")
	assert.Equal(t, int32(1), provider.callCount.Load())
}

func TestRetryableComplete_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	provider := &fakeProvider{
		errors: []error{ErrRateLimited, ErrRateLimited, ErrRateLimited},
	}

	// Cancel after a short delay so the backoff sleep is interrupted.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := RetryableComplete(ctx, provider, &CompletionRequest{}, 5)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}

// --- IsRetryable tests ---

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "rate limited sentinel",
			err:      ErrRateLimited,
			expected: true,
		},
		{
			name:     "provider unavailable sentinel",
			err:      ErrProviderUnavailable,
			expected: true,
		},
		{
			name:     "wrapped rate limited",
			err:      errors.Join(errors.New("wrapper"), ErrRateLimited),
			expected: true,
		},
		{
			name:     "status 429 in message",
			err:      errors.New("API returned status 429: rate limit exceeded"),
			expected: true,
		},
		{
			name:     "status 500 in message",
			err:      errors.New("API returned status 500: internal server error"),
			expected: true,
		},
		{
			name:     "status 502 in message",
			err:      errors.New("API returned status 502: bad gateway"),
			expected: true,
		},
		{
			name:     "status 503 in message",
			err:      errors.New("API returned status 503: service unavailable"),
			expected: true,
		},
		{
			name:     "status 504 in message",
			err:      errors.New("API returned status 504: gateway timeout"),
			expected: true,
		},
		{
			name:     "connection refused",
			err:      errors.New("dial tcp: connection refused"),
			expected: true,
		},
		{
			name:     "connection reset",
			err:      errors.New("read tcp: connection reset by peer"),
			expected: true,
		},
		{
			name:     "i/o timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "dns error no such host",
			err:      errors.New("dial tcp: lookup api.example.com: no such host"),
			expected: true,
		},
		{
			name:     "status 400 not retryable",
			err:      errors.New("API returned status 400: bad request"),
			expected: false,
		},
		{
			name:     "status 401 not retryable",
			err:      errors.New("API returned status 401: unauthorized"),
			expected: false,
		},
		{
			name:     "status 403 not retryable",
			err:      errors.New("API returned status 403: forbidden"),
			expected: false,
		},
		{
			name:     "context deadline exceeded",
			err:      errors.New("Post ...: context deadline exceeded"),
			expected: true,
		},
		{
			name:     "client timeout",
			err:      errors.New("Client.Timeout exceeded while awaiting headers"),
			expected: true,
		},
		{
			name:     "generic error not retryable",
			err:      errors.New("something went wrong"),
			expected: false,
		},
		{
			name:     "invalid response not retryable",
			err:      ErrInvalidResponse,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, IsRetryable(tc.err))
		})
	}
}

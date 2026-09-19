package declarative

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestReconnectLoop_SuccessfulFirstConnect(t *testing.T) {
	logger := logging.NewNopLogger()

	connectCalls := 0
	recvLoopCalls := 0

	ctx, cancel := context.WithCancel(context.Background())

	err := reconnectLoop(ctx, nil, "test_source", logger,
		func(ctx context.Context) error {
			connectCalls++
			return nil
		},
		func(ctx context.Context) error {
			recvLoopCalls++
			cancel() // Simulate graceful shutdown after first recv loop
			return ctx.Err()
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if connectCalls != 1 {
		t.Errorf("connectCalls = %d, want 1", connectCalls)
	}
	if recvLoopCalls != 1 {
		t.Errorf("recvLoopCalls = %d, want 1", recvLoopCalls)
	}
}

func TestReconnectLoop_ReconnectAfterDisconnect(t *testing.T) {
	logger := logging.NewNopLogger()

	connectCalls := 0

	ctx, cancel := context.WithCancel(context.Background())

	err := reconnectLoop(ctx, &ReconnectSpec{
		MaxAttempts:  0, // infinite
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Millisecond},
	}, "test_source", logger,
		func(ctx context.Context) error {
			connectCalls++
			return nil
		},
		func(ctx context.Context) error {
			if connectCalls >= 3 {
				cancel()
				return ctx.Err()
			}
			return errors.New("connection lost")
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if connectCalls < 3 {
		t.Errorf("connectCalls = %d, want >= 3", connectCalls)
	}
}

func TestReconnectLoop_MaxAttemptsReached(t *testing.T) {
	logger := logging.NewNopLogger()

	connectCalls := 0

	err := reconnectLoop(context.Background(), &ReconnectSpec{
		MaxAttempts:  3,
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Millisecond},
	}, "test_source", logger,
		func(ctx context.Context) error {
			connectCalls++
			return errors.New("connect failed")
		},
		func(ctx context.Context) error {
			t.Fatal("recvLoop should not be called when connect fails")
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error for max attempts reached, got nil")
	}
	// With MaxAttempts=3: attempts 0,1,2 connect (fail), then attempt 3 hits the max check
	if connectCalls != 3 {
		t.Errorf("connectCalls = %d, want 3", connectCalls)
	}
}

func TestReconnectLoop_ContextCancellationStops(t *testing.T) {
	logger := logging.NewNopLogger()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := reconnectLoop(ctx, &ReconnectSpec{
		MaxAttempts:  0,
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 5 * time.Second}, // Long delay
		MaxDelay:     Duration{Duration: 5 * time.Second},
	}, "test_source", logger,
		func(ctx context.Context) error {
			return errors.New("connect failed")
		},
		func(ctx context.Context) error {
			return nil
		},
	)

	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Errorf("expected context error, got %v", err)
	}
}

func TestReconnectBackoff_Exponential(t *testing.T) {
	spec := &ReconnectSpec{
		Backoff:      "exponential",
		InitialDelay: Duration{Duration: 100 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Second},
	}

	tests := []struct {
		attempt int
		minMs   int64
		maxMs   int64
	}{
		{attempt: 1, minMs: 50, maxMs: 100},  // 100ms * 2^0 = 100ms, jitter [50, 100)
		{attempt: 2, minMs: 100, maxMs: 200}, // 100ms * 2^1 = 200ms, jitter [100, 200)
		{attempt: 3, minMs: 200, maxMs: 400}, // 100ms * 2^2 = 400ms, jitter [200, 400)
		{attempt: 4, minMs: 400, maxMs: 800}, // 100ms * 2^3 = 800ms, jitter [400, 800)
	}

	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			// Run multiple times to account for jitter
			for i := 0; i < 20; i++ {
				delay := reconnectBackoff(spec, tc.attempt)
				ms := delay.Milliseconds()
				if ms < tc.minMs || ms > tc.maxMs {
					t.Errorf("attempt %d: delay %dms not in range [%d, %d]",
						tc.attempt, ms, tc.minMs, tc.maxMs)
				}
			}
		})
	}
}

func TestReconnectBackoff_Fixed(t *testing.T) {
	spec := &ReconnectSpec{
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 200 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Second},
	}

	// All attempts should produce delays in [100ms, 200ms) (half + jitter)
	for attempt := 1; attempt <= 5; attempt++ {
		delay := reconnectBackoff(spec, attempt)
		ms := delay.Milliseconds()
		if ms < 100 || ms > 200 {
			t.Errorf("attempt %d: fixed backoff delay %dms not in range [100, 200]",
				attempt, ms)
		}
	}
}

func TestReconnectBackoff_MaxDelayCap(t *testing.T) {
	spec := &ReconnectSpec{
		Backoff:      "exponential",
		InitialDelay: Duration{Duration: time.Second},
		MaxDelay:     Duration{Duration: 5 * time.Second},
	}

	// Very high attempt should be capped at maxDelay
	delay := reconnectBackoff(spec, 20)
	if delay > 5*time.Second {
		t.Errorf("delay %v exceeds maxDelay 5s", delay)
	}
	// With equal jitter, minimum is maxDelay/2
	if delay < 2500*time.Millisecond {
		t.Errorf("delay %v below expected minimum of 2.5s", delay)
	}
}

func TestReconnectBackoff_NilSpec(t *testing.T) {
	// Should use defaults (1s initial, 60s max, exponential)
	delay := reconnectBackoff(nil, 1)
	// 1s * 2^0 = 1s, jitter [500ms, 1000ms)
	ms := delay.Milliseconds()
	if ms < 500 || ms > 1000 {
		t.Errorf("nil spec delay %dms not in expected range [500, 1000]", ms)
	}
}

func TestReconnectLoop_ResetAfterStableConnection(t *testing.T) {
	logger := logging.NewNopLogger()

	connectCalls := 0

	ctx, cancel := context.WithCancel(context.Background())

	err := reconnectLoop(ctx, &ReconnectSpec{
		MaxAttempts:  5,
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Millisecond},
		ResetAfter:   Duration{Duration: 50 * time.Millisecond}, // Short for testing
	}, "test_source", logger,
		func(ctx context.Context) error {
			connectCalls++
			return nil
		},
		func(ctx context.Context) error {
			if connectCalls == 1 {
				// First connection: stay connected long enough to reset counter
				time.Sleep(60 * time.Millisecond)
				return errors.New("disconnect after stable period")
			}
			// Second connection: shutdown
			cancel()
			return ctx.Err()
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if connectCalls < 2 {
		t.Errorf("connectCalls = %d, want >= 2", connectCalls)
	}
}

func TestReconnectLoop_ConnectFailThenSucceed(t *testing.T) {
	logger := logging.NewNopLogger()

	connectCalls := 0

	ctx, cancel := context.WithCancel(context.Background())

	err := reconnectLoop(ctx, &ReconnectSpec{
		MaxAttempts:  10,
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 10 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Millisecond},
	}, "test_source", logger,
		func(ctx context.Context) error {
			connectCalls++
			if connectCalls < 3 {
				return errors.New("transient failure")
			}
			return nil
		},
		func(ctx context.Context) error {
			cancel()
			return ctx.Err()
		},
	)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if connectCalls != 3 {
		t.Errorf("connectCalls = %d, want 3", connectCalls)
	}
}

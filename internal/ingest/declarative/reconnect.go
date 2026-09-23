package declarative

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// reconnectLoop manages the connect -> recv -> reconnect lifecycle for StreamTransport.
// It calls connectFn to establish a connection and recvLoop to process messages.
// On disconnect, it reconnects with backoff per the ReconnectSpec.
func reconnectLoop(
	ctx context.Context,
	spec *ReconnectSpec,
	sourceName string,
	logger *logging.Logger,
	connectFn func(ctx context.Context) error,
	recvLoop func(ctx context.Context) error,
) error {
	attempt := 0
	maxAttempts := 0 // 0 = infinite
	if spec != nil {
		maxAttempts = spec.MaxAttempts
	}

	resetAfter := 5 * time.Minute
	if spec != nil && spec.ResetAfter.Duration > 0 {
		resetAfter = spec.ResetAfter.Duration
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			if maxAttempts > 0 && attempt >= maxAttempts {
				logger.Error("max reconnection attempts reached",
					logging.String("source_name", sourceName),
					logging.Int("max_attempts", maxAttempts),
				)
				return fmt.Errorf("max reconnection attempts (%d) reached for %q", maxAttempts, sourceName)
			}

			delay := reconnectBackoff(spec, attempt)
			logger.Warn("transport reconnecting",
				logging.String("source_name", sourceName),
				logging.Int("attempt", attempt),
				logging.Any("delay_ms", delay.Milliseconds()),
			)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}

		if err := connectFn(ctx); err != nil {
			logger.Error("transport connect failed",
				logging.String("source_name", sourceName),
				logging.Int("attempt", attempt),
				logging.Err("error", err),
			)
			attempt++
			continue
		}

		logger.Info("transport connected",
			logging.String("source_name", sourceName),
		)

		connectedAt := time.Now()

		err := recvLoop(ctx)
		if err != nil && ctx.Err() == nil {
			logger.Warn("transport disconnected",
				logging.String("source_name", sourceName),
				logging.Err("error", err),
			)
		}

		// If connection was stable long enough, reset backoff counter
		if time.Since(connectedAt) >= resetAfter {
			attempt = 0
		}

		attempt++

		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// reconnectBackoff computes delay for reconnection attempts using equal jitter.
func reconnectBackoff(spec *ReconnectSpec, attempt int) time.Duration {
	initialDelay := time.Second
	maxDelay := 60 * time.Second
	backoff := "exponential"

	if spec != nil {
		if spec.InitialDelay.Duration > 0 {
			initialDelay = spec.InitialDelay.Duration
		}
		if spec.MaxDelay.Duration > 0 {
			maxDelay = spec.MaxDelay.Duration
		}
		if spec.Backoff != "" {
			backoff = spec.Backoff
		}
	}

	var delay time.Duration
	switch backoff {
	case "exponential":
		// Saturate at maxDelay rather than computing the full 2^n. A stream that
		// keeps failing drives attempt into the hundreds, where initialDelay*2^n
		// overflows int64 nanoseconds and wraps negative; a negative delay slips
		// past the clamp below and makes time.After fire immediately, turning
		// reconnection into a hot loop against the upstream. Comparing against
		// maxDelay>>n decides the same thing without ever overflowing.
		delay = maxDelay
		if n := attempt - 1; n >= 0 && n < 63 && initialDelay <= maxDelay>>uint(n) {
			delay = initialDelay << uint(n)
		}
	case "fixed":
		delay = initialDelay
	default:
		delay = initialDelay
	}

	if delay > maxDelay {
		delay = maxDelay
	}

	// Equal jitter
	half := delay / 2
	jitter := time.Duration(rand.Float64() * float64(half))
	return half + jitter
}

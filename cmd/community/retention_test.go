package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/domain"
)

// fakeMaintenanceRepo records EnforceSizeCap calls for the driver test.
type fakeMaintenanceRepo struct {
	mu    sync.Mutex
	calls int
}

func (f *fakeMaintenanceRepo) DatabaseSizeBytes(_ context.Context) (int64, error) { return 0, nil }

func (f *fakeMaintenanceRepo) EnforceSizeCap(_ context.Context, _ int64, _ float64) (domain.RetentionResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return domain.RetentionResult{}, nil
}

func (f *fakeMaintenanceRepo) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

// TestRunStorageRetention_RunsImmediatelyThenStopsOnCancel proves the driver runs
// one pass at startup (not waiting a full interval) and returns promptly on cancel.
func TestRunStorageRetention_RunsImmediatelyThenStopsOnCancel(t *testing.T) {
	repo := &fakeMaintenanceRepo{}
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		// A long interval ensures the only call observed is the immediate startup pass.
		runStorageRetention(ctx, repo, 1000, 0.9, time.Hour, zerolog.Nop())
		close(done)
	}()

	require.Eventually(t, func() bool { return repo.callCount() >= 1 }, 2*time.Second, 5*time.Millisecond,
		"driver must run one pass immediately at startup")

	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runStorageRetention did not return on context cancel")
	}

	assert.GreaterOrEqual(t, repo.callCount(), 1)
}

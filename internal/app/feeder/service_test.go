package feeder

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/ingest"
)

// blockingSource is a minimal LayerSource whose Start blocks until the body
// function returns. Snapshot returns no entities so the persist path is a no-op.
type blockingSource struct {
	name string
	body func(ctx context.Context)
}

func (s *blockingSource) Start(ctx context.Context) error {
	s.body(ctx)
	return nil
}
func (s *blockingSource) Stop() error { return nil }
func (s *blockingSource) Snapshot(_ context.Context) ([]*domain.Entity, []*domain.Observation, error) {
	return nil, nil, nil
}
func (s *blockingSource) Stream(_ context.Context, _ chan<- *domain.EntityUpdate) error { return nil }
func (s *blockingSource) Name() string                                                  { return s.name }
func (s *blockingSource) LayerType() string                                             { return s.name }
func (s *blockingSource) SupportsSourceType(_ domain.SourceType) bool                   { return false }

// newTestIngestionService builds a real IngestionService with a single fake
// source registered. The fake source's Start method calls body(ctx), so placing
// a blocking call there stalls the ingestSource execution inside the goroutine
// that Start() launches. A very long ticker interval ensures only the initial
// ingest (before the ticker loop) executes.
func newTestIngestionService(t *testing.T, logger zerolog.Logger, body func(ctx context.Context)) *IngestionService {
	t.Helper()

	const fakeName = "test-fake-source"

	registry := ingest.NewSourceRegistry()
	registry.Register(&blockingSource{name: fakeName, body: body})

	return &IngestionService{
		logger:         logger,
		registry:       registry,
		enabledSources: []string{fakeName},
		sourceConfigs: map[string]SourceConfig{
			fakeName: {
				Name:     fakeName,
				Interval: 24 * time.Hour, // long interval: only the initial run fires
			},
		},
		tickers: make([]*time.Ticker, 0),
		stopCh:  make(chan struct{}),
	}
}

// TestIngestionService_StopWaitsForInflight asserts Stop() does not return until
// the per-source goroutines have exited, so no goroutine can write to the DB
// after Stop() (and therefore after the deferred db.Close in serve.go).
func TestIngestionService_StopWaitsForInflight(t *testing.T) {
	var running int32
	released := make(chan struct{})

	// Minimal service with one fake source goroutine simulating an in-flight
	// ingest that only finishes when released. Mirrors the Start() goroutine
	// shape (service.go:113-136) but with an injected blocking body.
	s := newTestIngestionService(t, zerolog.Nop(), func(ctx context.Context) {
		atomic.StoreInt32(&running, 1)
		select {
		case <-released:
		case <-ctx.Done():
		}
		atomic.StoreInt32(&running, 0)
	})

	ctx := context.Background()
	s.Start(ctx)

	// Wait until the fake ingest is in-flight.
	deadline := time.Now().Add(2 * time.Second)
	for atomic.LoadInt32(&running) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("fake ingest never started")
		}
		time.Sleep(time.Millisecond)
	}

	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()

	// Stop() must block while the ingest is still in-flight.
	select {
	case <-done:
		t.Fatal("Stop() returned before the in-flight ingest finished")
	case <-time.After(100 * time.Millisecond):
	}

	close(released) // let the ingest finish

	select {
	case <-done:
		if atomic.LoadInt32(&running) != 0 {
			t.Fatal("Stop() returned while a goroutine was still running")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() did not return after the ingest finished")
	}
}

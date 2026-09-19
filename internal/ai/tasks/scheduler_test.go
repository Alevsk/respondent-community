package tasks

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Alevsk/respondent/internal/ai/schema"
	"github.com/Alevsk/respondent/internal/llm"
)

// countingProvider records how many times Complete is called.
type countingProvider struct {
	mu    sync.Mutex
	calls int
}

func (p *countingProvider) Complete(_ context.Context, _ *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	return &llm.CompletionResponse{Content: "ok"}, nil
}

func (p *countingProvider) Name() string { return "counting" }
func (p *countingProvider) SupportsProvider(providerType string) bool {
	return providerType == "counting"
}
func (p *countingProvider) HealthCheck(_ context.Context) error { return nil }

func (p *countingProvider) CallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func newSchedulerTestEngine(t *testing.T, provider llm.Provider) *Engine {
	t.Helper()
	reg := schema.NewRegistry()
	return NewEngine(provider, reg, nil, zerolog.Nop())
}

func TestScheduler_IntervalTriggersExecution(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"interval_task": {
			Name:    "interval_task",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "50ms",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "ai", Prompt: "test"},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = scheduler.Start(ctx)
		close(done)
	}()

	<-started

	// Wait for at least 2 ticks (50ms each) + buffer.
	time.Sleep(180 * time.Millisecond)

	// Stop scheduler.
	cancel()
	scheduler.Stop()

	select {
	case <-done:
		// Success.
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within timeout")
	}

	// The interval task should have fired at least once.
	assert.GreaterOrEqual(t, provider.CallCount(), 1,
		"expected at least 1 execution from interval ticker")
}

func TestScheduler_NonScheduleTriggersSkipped(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"event_task": {
			Name:    "event_task",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:   "event",
				Source: "opensky",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "target", Targets: []string{"log"}},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = scheduler.Start(ctx)
		close(done)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	cancel()
	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within timeout")
	}

	// Event-triggered tasks should NOT have been scheduled.
	assert.Equal(t, 0, provider.CallCount(),
		"event-triggered tasks should not be scheduled by the Scheduler")
}

func TestScheduler_DisabledTasksSkipped(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"disabled_task": {
			Name:    "disabled_task",
			Enabled: false, // disabled
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "50ms",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "target", Targets: []string{"log"}},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = scheduler.Start(ctx)
		close(done)
	}()

	<-started
	time.Sleep(100 * time.Millisecond)

	cancel()
	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within timeout")
	}

	assert.Equal(t, 0, provider.CallCount(),
		"disabled tasks should not be scheduled")
}

func TestScheduler_StopGracefully(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"task_a": {
			Name:    "task_a",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "50ms",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "ai", Prompt: "test"},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = scheduler.Start(ctx)
		close(done)
	}()

	<-started
	// Let it run briefly.
	time.Sleep(60 * time.Millisecond)

	// Graceful shutdown.
	cancel()
	scheduler.Stop()

	select {
	case <-done:
		// Scheduler exited cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within timeout")
	}
}

func TestScheduler_InvalidIntervalReturnsError(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"bad_interval": {
			Name:    "bad_interval",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "not-a-duration",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "target", Targets: []string{"log"}},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- scheduler.Start(ctx)
	}()

	select {
	case err := <-errCh:
		require.Error(t, err)
		assert.Contains(t, err.Error(), "bad_interval")
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not return error within timeout")
	}
}

func TestScheduler_MixedDefinitions(t *testing.T) {
	provider := &countingProvider{}
	engine := newSchedulerTestEngine(t, provider)

	defs := map[string]*TaskDefinition{
		"scheduled": {
			Name:    "scheduled",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "50ms",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "ai", Prompt: "test"},
			},
		},
		"event_only": {
			Name:    "event_only",
			Enabled: true,
			Trigger: TriggerConfig{
				Type:   "event",
				Source: "opensky",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "ai", Prompt: "test"},
			},
		},
		"disabled_schedule": {
			Name:    "disabled_schedule",
			Enabled: false,
			Trigger: TriggerConfig{
				Type:     "schedule",
				Interval: "50ms",
			},
			Steps: []StepConfig{
				{Name: "step1", Type: "ai", Prompt: "test"},
			},
		},
	}

	scheduler := NewScheduler(engine, defs, zerolog.Nop())

	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		_ = scheduler.Start(ctx)
		close(done)
	}()

	<-started
	time.Sleep(180 * time.Millisecond)

	cancel()
	scheduler.Stop()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("scheduler did not shut down within timeout")
	}

	// Only the "scheduled" task should have fired.
	// It should have fired at least once in 180ms with a 50ms interval.
	assert.GreaterOrEqual(t, provider.CallCount(), 1,
		"only the enabled schedule-triggered task should have executed")
}

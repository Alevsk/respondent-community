package declarative

import (
	"context"
	"testing"
	"time"
)

func TestBatcher_FlushOnContextCancel(t *testing.T) {
	spec := &BatchingSpec{
		Mode:   "window",
		Window: Duration{Duration: 10 * time.Second}, // long window so ticker won't fire
	}
	b := NewBatcher(spec)

	ctx, cancel := context.WithCancel(context.Background())

	runDone := make(chan struct{})
	go func() {
		b.Run(ctx)
		close(runDone)
	}()

	// Add messages to the buffer while Run is active.
	b.Add([]byte(`msg1`))
	b.Add([]byte(`msg2`))

	// Cancel context to trigger the ctx.Done() branch in Run which calls flush().
	cancel()

	// Read the flushed batch from the channel (buffered capacity = 16).
	select {
	case batch := <-b.Batches():
		if len(batch) != 2 {
			t.Errorf("expected 2 messages in flushed batch on ctx cancel, got %d", len(batch))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected batch to be flushed on context cancel")
	}

	// Wait for Run to return.
	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestBatcher_Window_StopCh(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 10 * time.Second}, // very long window
		MaxSize: 1000,
	})

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		b.Run(context.Background())
	}()

	// Give Run() time to start and enter the select loop.
	time.Sleep(20 * time.Millisecond)

	// Stop without cancelling context: triggers stopCh case in Run.
	// Buffer is empty so flush() is a no-op (avoids send-on-closed-channel).
	b.Stop()

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit after Stop()")
	}
}

func TestBatcher_Window_StopChEmptyBuffer(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 10 * time.Second},
		MaxSize: 1000,
	})

	runDone := make(chan struct{})
	go func() {
		defer close(runDone)
		b.Run(context.Background())
	}()

	// Give Run() time to start and enter the select loop.
	time.Sleep(20 * time.Millisecond)

	// Stop() triggers the stopCh case in Run's select (buffer is empty -> no panic).
	b.Stop()

	select {
	case <-runDone:
	case <-time.After(2 * time.Second):
		t.Fatal("Run() did not exit after Stop()")
	}
}

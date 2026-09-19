package declarative

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestBatcher_PerMessage_ImmediateFlush(t *testing.T) {
	b := NewBatcher(&BatchingSpec{Mode: "per_message"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Add([]byte("msg1"))
	b.Add([]byte("msg2"))

	// Each Add should produce a batch of 1
	for i := 0; i < 2; i++ {
		select {
		case batch := <-b.Batches():
			if len(batch) != 1 {
				t.Errorf("batch %d: len = %d, want 1", i, len(batch))
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for batch %d", i)
		}
	}
}

func TestBatcher_PerMessage_NilSpec(t *testing.T) {
	// nil spec defaults to per_message mode
	b := NewBatcher(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Add([]byte("msg"))

	select {
	case batch := <-b.Batches():
		if len(batch) != 1 {
			t.Errorf("batch len = %d, want 1", len(batch))
		}
		if string(batch[0]) != "msg" {
			t.Errorf("batch[0] = %q, want %q", string(batch[0]), "msg")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for batch")
	}
}

func TestBatcher_Window_TimerFlush(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 100 * time.Millisecond},
		MaxSize: 1000,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Add([]byte("msg1"))
	b.Add([]byte("msg2"))
	b.Add([]byte("msg3"))

	// Should flush after ~100ms window
	select {
	case batch := <-b.Batches():
		if len(batch) != 3 {
			t.Errorf("batch len = %d, want 3", len(batch))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for window flush")
	}
}

func TestBatcher_Window_MaxSizeFlush(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 10 * time.Second}, // Very long window
		MaxSize: 3,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	b.Add([]byte("msg1"))
	b.Add([]byte("msg2"))
	b.Add([]byte("msg3")) // Should trigger flush at maxSize=3

	select {
	case batch := <-b.Batches():
		if len(batch) != 3 {
			t.Errorf("batch len = %d, want 3", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for maxSize flush")
	}
}

func TestBatcher_Window_ContextCancellationFlushesBuffer(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 10 * time.Second},
		MaxSize: 1000,
	})

	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		b.Run(ctx)
	}()

	b.Add([]byte("msg1"))
	b.Add([]byte("msg2"))

	// Give time for messages to be buffered
	time.Sleep(50 * time.Millisecond)

	// Cancel context should trigger flush
	cancel()
	wg.Wait()

	// Drain the flush channel
	select {
	case batch := <-b.Batches():
		if len(batch) != 2 {
			t.Errorf("batch len = %d, want 2", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for flush after context cancellation")
	}
}

func TestBatcher_Stop_SafeMultipleCalls(t *testing.T) {
	b := NewBatcher(nil)

	// Should not panic
	b.Stop()
	b.Stop()
	b.Stop()
}

func TestBatcher_Window_EmptyFlush(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 50 * time.Millisecond},
		MaxSize: 1000,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go b.Run(ctx)

	// Wait for a window tick with no messages
	time.Sleep(100 * time.Millisecond)

	// No batch should be produced for empty buffer
	select {
	case batch := <-b.Batches():
		t.Errorf("unexpected batch with %d messages from empty buffer", len(batch))
	default:
		// Expected: no batch
	}

	cancel()
}

// TestBatcher_NoPanicOnConcurrentStop reproduces the "send on closed channel"
// crash: many concurrent Add() senders racing the ctx-cancel that triggers both
// Run's Stop() and an explicit Stop(), mirroring adapter.go's two Stop() callers.
// Under the old code (Stop closed flushCh) this panics intermittently; the fix
// (Stop closes only stopCh) must make it never panic. Run with -race.
func TestBatcher_NoPanicOnConcurrentStop(t *testing.T) {
	for round := 0; round < 200; round++ {
		b := NewBatcher(&BatchingSpec{Mode: "per_message"})
		ctx, cancel := context.WithCancel(context.Background())

		go b.Run(ctx) // Run calls b.Stop() on ctx.Done (mirrors batching.go:87)

		var wg sync.WaitGroup
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					b.Add([]byte("msg")) // sender racing the close
				}
			}()
		}

		// Drain so per_message Add() senders are not all blocked on a full buffer.
		go func() {
			for {
				select {
				case <-b.Batches():
				case <-ctx.Done():
					return
				}
			}
		}()

		time.Sleep(time.Millisecond)
		cancel()  // triggers Run -> Stop()
		b.Stop()  // explicit second Stop(), mirroring adapter.go:366
		wg.Wait() // a panic in any Add() goroutine crashes the test
	}
}

func TestBatcher_Window_MultipleFlushes(t *testing.T) {
	b := NewBatcher(&BatchingSpec{
		Mode:    "window",
		Window:  Duration{Duration: 80 * time.Millisecond},
		MaxSize: 1000,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.Run(ctx)

	// First batch
	b.Add([]byte("batch1_msg1"))
	b.Add([]byte("batch1_msg2"))

	select {
	case batch := <-b.Batches():
		if len(batch) != 2 {
			t.Errorf("first batch len = %d, want 2", len(batch))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first batch")
	}

	// Second batch
	b.Add([]byte("batch2_msg1"))

	select {
	case batch := <-b.Batches():
		if len(batch) != 1 {
			t.Errorf("second batch len = %d, want 1", len(batch))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for second batch")
	}
}

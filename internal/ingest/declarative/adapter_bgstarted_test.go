package declarative

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingStreamTransport blocks on Connect until the context is cancelled,
// allowing tests to verify bgStarted idempotency while the goroutine is alive.
type blockingStreamTransport struct {
	connectCount atomic.Int32
	recvCount    atomic.Int32
	closeCount   atomic.Int32
}

func (b *blockingStreamTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

func (b *blockingStreamTransport) Connect(ctx context.Context) error {
	b.connectCount.Add(1)
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingStreamTransport) Recv(_ context.Context) ([]byte, error) {
	b.recvCount.Add(1)
	return nil, errors.New("not implemented")
}

func (b *blockingStreamTransport) Close() error {
	b.closeCount.Add(1)
	return nil
}

// blockingListenTransport blocks on Listen until the context is cancelled.
type blockingListenTransport struct {
	listenCount atomic.Int32
	closeCount  atomic.Int32
}

func (b *blockingListenTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

func (b *blockingListenTransport) Listen(ctx context.Context, _ chan<- []byte) error {
	b.listenCount.Add(1)
	<-ctx.Done()
	return nil
}

func (b *blockingListenTransport) Close() error {
	b.closeCount.Add(1)
	return nil
}

// TestStart_StreamTransport_Idempotent verifies that calling Start() multiple
// times with a StreamTransport only launches one background goroutine.
// This is the core test for the bgStarted CompareAndSwap guard.
func TestStart_StreamTransport_Idempotent(t *testing.T) {
	transport := &blockingStreamTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Call Start 10 times — simulates the ticker calling Start on every tick.
	for i := 0; i < 10; i++ {
		if err := adapter.Start(ctx); err != nil {
			t.Fatalf("Start() call %d returned error: %v", i, err)
		}
	}

	// Give the goroutine a moment to call Connect.
	time.Sleep(50 * time.Millisecond)

	connects := transport.connectCount.Load()
	if connects != 1 {
		t.Errorf("expected exactly 1 Connect call, got %d (bgStarted guard failed)", connects)
	}

	// bgStarted should be true while the goroutine is alive.
	if !adapter.bgStarted.Load() {
		t.Error("bgStarted should be true while streaming goroutine is active")
	}
}

// TestStart_ListenTransport_Idempotent verifies that calling Start() multiple
// times with a ListenTransport only launches one background goroutine.
func TestStart_ListenTransport_Idempotent(t *testing.T) {
	transport := &blockingListenTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for i := 0; i < 10; i++ {
		if err := adapter.Start(ctx); err != nil {
			t.Fatalf("Start() call %d returned error: %v", i, err)
		}
	}

	time.Sleep(50 * time.Millisecond)

	listens := transport.listenCount.Load()
	if listens != 1 {
		t.Errorf("expected exactly 1 Listen call, got %d (bgStarted guard failed)", listens)
	}

	if !adapter.bgStarted.Load() {
		t.Error("bgStarted should be true while listen goroutine is active")
	}
}

// TestStart_StreamTransport_ResetsAfterExit verifies that bgStarted is reset
// to false when the streaming goroutine exits, allowing a restart on the next tick.
func TestStart_StreamTransport_ResetsAfterExit(t *testing.T) {
	transport := &blockingStreamTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())

	// First Start — launches goroutine.
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if !adapter.bgStarted.Load() {
		t.Fatal("bgStarted should be true after Start")
	}

	// Cancel context — goroutine should exit and reset bgStarted.
	cancel()
	time.Sleep(100 * time.Millisecond)

	if adapter.bgStarted.Load() {
		t.Error("bgStarted should be false after goroutine exits")
	}

	// Second Start with a new context — should launch a new goroutine.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	if err := adapter.Start(ctx2); err != nil {
		t.Fatalf("second Start() returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	connects := transport.connectCount.Load()
	if connects != 2 {
		t.Errorf("expected 2 Connect calls after restart, got %d", connects)
	}

	if !adapter.bgStarted.Load() {
		t.Error("bgStarted should be true after restart")
	}
}

// TestStart_ListenTransport_ResetsAfterExit verifies the same reset behavior
// for ListenTransport.
func TestStart_ListenTransport_ResetsAfterExit(t *testing.T) {
	transport := &blockingListenTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if !adapter.bgStarted.Load() {
		t.Fatal("bgStarted should be true after Start")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)

	if adapter.bgStarted.Load() {
		t.Error("bgStarted should be false after goroutine exits")
	}

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	if err := adapter.Start(ctx2); err != nil {
		t.Fatalf("second Start() returned error: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	listens := transport.listenCount.Load()
	if listens != 2 {
		t.Errorf("expected 2 Listen calls after restart, got %d", listens)
	}
}

// TestStart_StreamTransport_ConcurrentCalls verifies that concurrent Start()
// calls don't race on bgStarted (tests the atomic guard under contention).
func TestStart_StreamTransport_ConcurrentCalls(t *testing.T) {
	transport := &blockingStreamTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = adapter.Start(ctx)
		}()
	}
	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	connects := transport.connectCount.Load()
	if connects != 1 {
		t.Errorf("expected exactly 1 Connect call under concurrent Start, got %d", connects)
	}
}

// TestStart_ListenTransport_ConcurrentCalls verifies the same for ListenTransport.
func TestStart_ListenTransport_ConcurrentCalls(t *testing.T) {
	transport := &blockingListenTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = adapter.Start(ctx)
		}()
	}
	wg.Wait()

	time.Sleep(50 * time.Millisecond)

	listens := transport.listenCount.Load()
	if listens != 1 {
		t.Errorf("expected exactly 1 Listen call under concurrent Start, got %d", listens)
	}
}

// TestStart_StreamTransport_ErrorResetsFlag verifies that if startStreaming
// returns an error (not from context cancellation), bgStarted is still reset.
func TestStart_StreamTransport_ErrorResetsFlag(t *testing.T) {
	// Use a transport that connects but immediately returns an error from Recv.
	// The reconnect loop has no max_attempts in our test YAML (defaults to infinite),
	// so we use a short-lived context to force exit.
	transport := &mockStreamTransport{
		messages: [][]byte{}, // no messages — Recv returns error immediately
	}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Wait for context to expire and goroutine to exit.
	<-ctx.Done()
	time.Sleep(100 * time.Millisecond)

	if adapter.bgStarted.Load() {
		t.Error("bgStarted should be false after streaming goroutine exits")
	}
}

// TestStart_ListenTransport_ErrorResetsFlag verifies the same for ListenTransport.
func TestStart_ListenTransport_ErrorResetsFlag(t *testing.T) {
	transport := &mockListenTransport{
		listenErr: errors.New("listen failed"),
	}
	adapter := newStreamingAdapterForTest(t, transport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	if adapter.bgStarted.Load() {
		t.Error("bgStarted should be false after listen goroutine exits with error")
	}
}

// TestBgStarted_InitiallyFalse verifies that a new adapter has bgStarted=false.
func TestBgStarted_InitiallyFalse(t *testing.T) {
	transport := &blockingStreamTransport{}
	adapter := newStreamingAdapterForTest(t, transport)

	if adapter.bgStarted.Load() {
		t.Error("bgStarted should be false on a new adapter")
	}
}

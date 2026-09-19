package declarative

import (
	"context"
	"sync"
	"time"
)

// Batcher accumulates messages from streaming transports and flushes them
// as batches based on time window and/or max size.
type Batcher struct {
	mode    string // "per_message" or "window"
	window  time.Duration
	maxSize int

	mu       sync.Mutex
	buffer   [][]byte
	flushCh  chan [][]byte
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewBatcher creates a Batcher from the batching spec.
// If spec is nil, defaults to per_message mode.
func NewBatcher(spec *BatchingSpec) *Batcher {
	mode := "per_message"
	window := 5 * time.Second
	maxSize := 1000

	if spec != nil {
		mode = spec.Mode
		if spec.Window.Duration > 0 {
			window = spec.Window.Duration
		}
		if spec.MaxSize > 0 {
			maxSize = spec.MaxSize
		}
	}

	return &Batcher{
		mode:    mode,
		window:  window,
		maxSize: maxSize,
		buffer:  make([][]byte, 0, maxSize),
		flushCh: make(chan [][]byte, 16),
		stopCh:  make(chan struct{}),
	}
}

// Add adds a message to the batcher. In per_message mode, it immediately
// flushes. In window mode, it buffers until window expires or maxSize is reached.
func (b *Batcher) Add(msg []byte) {
	if b.mode == "per_message" {
		batch := make([][]byte, 1)
		batch[0] = msg
		select {
		case b.flushCh <- batch:
		case <-b.stopCh:
		}
		return
	}

	// Window mode
	b.mu.Lock()
	b.buffer = append(b.buffer, msg)
	shouldFlush := len(b.buffer) >= b.maxSize
	var batch [][]byte
	if shouldFlush {
		batch = b.buffer
		b.buffer = make([][]byte, 0, b.maxSize)
	}
	b.mu.Unlock()

	if shouldFlush && batch != nil {
		select {
		case b.flushCh <- batch:
		case <-b.stopCh:
		}
	}
}

// Run starts the window timer goroutine. Must be called before Add() for window mode.
// Blocks until ctx is cancelled or Stop() is called.
func (b *Batcher) Run(ctx context.Context) {
	if b.mode == "per_message" {
		<-ctx.Done()
		b.Stop()
		return
	}

	ticker := time.NewTicker(b.window)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.flush()
			b.Stop()
			return
		case <-b.stopCh:
			b.flush()
			return
		case <-ticker.C:
			b.flush()
		}
	}
}

// flush sends buffered messages to flushCh.
func (b *Batcher) flush() {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.buffer
	b.buffer = make([][]byte, 0, b.maxSize)
	b.mu.Unlock()

	select {
	case b.flushCh <- batch:
	case <-b.stopCh:
	}
}

// Batches returns the channel that receives flushed batches.
func (b *Batcher) Batches() <-chan [][]byte {
	return b.flushCh
}

// Stop signals the batcher to stop. Safe to call multiple times.
//
// flushCh is intentionally NOT closed: it has multiple senders (Add and flush),
// and closing a channel with concurrent senders races into a "send on closed
// channel" panic. Senders observe stopCh in their select and stop sending;
// consumers (see adapter.go) terminate via ctx.Done(), not via a closed channel.
func (b *Batcher) Stop() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
}

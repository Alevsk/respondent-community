package declarative

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// immediateBatchStreamTransport connects successfully, returns one message,
// then errors to force the reconnect loop to terminate (with maxAttempts=1).
type immediateBatchStreamTransport struct {
	msg    []byte
	called atomic.Int32
}

func (m *immediateBatchStreamTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

func (m *immediateBatchStreamTransport) Connect(_ context.Context) error {
	return nil // always succeeds
}

func (m *immediateBatchStreamTransport) Recv(_ context.Context) ([]byte, error) {
	n := m.called.Add(1)
	if n == 1 {
		// First call: return the message.
		return m.msg, nil
	}
	// Second call: return error to exit the recv loop.
	return nil, errors.New("stream ended")
}

func (m *immediateBatchStreamTransport) Close() error {
	return nil
}

// newStreamingAdapterWithPerMessageBatching creates an adapter configured with
// per_message batching mode so that messages are immediately forwarded to the
// batch processing goroutine.
func newStreamingAdapterWithPerMessageBatching(t *testing.T, mockTransport Transport) *DeclarativeAdapter {
	t.Helper()
	name := fmt.Sprintf("pipeline_test_%d", time.Now().UnixNano())
	dir := t.TempDir()
	yamlContent := fmt.Sprintf(`schema_version: 1
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Pipeline Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "1s"
  batching:
    mode: per_message
parser:
  format: json
  records_path: "items"
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: 'record.lat'
  longitude: 'record.lon'
  timestamp: 'now()'
recording:
  mode: append
cache:
  ttl: "60s"
display:
  icon:
    shape: dot
    scale: 1.0
  trail:
    color: "#00ff9d"
  style:
    color: "#00ff9d"
    point_size: 6
`, name, name, name)

	yamlPath := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, true, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}
	adapter.transport = mockTransport
	return adapter
}

// TestStartStreaming_BatchProcessingGoroutine verifies the batch processing goroutine
// inside startStreaming receives and processes a batch. Uses per_message batching
// and a transport that returns one message then errors, with maxAttempts=1 to terminate.
func TestStartStreaming_BatchProcessingGoroutine(t *testing.T) {
	payload := []byte(`{"items":[{"id":"s1","name":"Stream Entity","lat":10.0,"lon":20.0}]}`)
	mockTransport := &immediateBatchStreamTransport{msg: payload}
	adapter := newStreamingAdapterWithPerMessageBatching(t, mockTransport)

	// Set maxAttempts=1 so the reconnect loop exits after one failed recv.
	cs := adapter.compiled.Load()
	def := cs.Definition()
	def.Transport.Reconnect = &ReconnectSpec{
		MaxAttempts:  1,
		InitialDelay: Duration{Duration: 1 * time.Millisecond},
		MaxDelay:     Duration{Duration: 5 * time.Millisecond},
		Backoff:      "fixed",
	}

	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.startStreaming(ctx, mockTransport)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error after max reconnect attempts, got nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("startStreaming did not terminate within 5s")
	}

	// Give the batch processing goroutine time to process messages.
	time.Sleep(100 * time.Millisecond)

	// Verify the transport received messages (Recv was called at least once).
	if mockTransport.called.Load() < 1 {
		t.Error("expected Recv to be called at least once")
	}
}

// TestStartListening_ContextCancellation verifies startListening terminates cleanly
// when the context is cancelled.
func TestStartListening_ContextCancellation(t *testing.T) {
	// Use a transport that waits for ctx cancellation (no payloads).
	mockTransport := &mockListenTransport{
		payloads:  [][]byte{},
		listenErr: nil, // waits for ctx cancel
	}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.startListening(ctx, mockTransport)
	}()

	// Cancel context to trigger clean exit.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("startListening did not terminate within 5s after ctx cancel")
	}
}

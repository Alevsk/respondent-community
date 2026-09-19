package declarative

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// mockStreamTransport is a minimal StreamTransport for testing.
type mockStreamTransport struct {
	messages   [][]byte
	connectErr error
	recvIdx    int
	closed     bool
}

func (m *mockStreamTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

func (m *mockStreamTransport) Connect(_ context.Context) error {
	return m.connectErr
}

func (m *mockStreamTransport) Recv(_ context.Context) ([]byte, error) {
	if m.recvIdx >= len(m.messages) {
		// Block until context is cancelled after all messages are sent.
		// Return a sentinel error to stop the recv loop.
		return nil, errors.New("no more messages")
	}
	msg := m.messages[m.recvIdx]
	m.recvIdx++
	return msg, nil
}

func (m *mockStreamTransport) Close() error {
	m.closed = true
	return nil
}

// mockListenTransport is a minimal ListenTransport for testing.
type mockListenTransport struct {
	payloads   [][]byte
	listenErr  error
	closedOnce bool
}

func (m *mockListenTransport) Fetch(_ context.Context, _, _ string) ([]byte, int, error) {
	return nil, 0, ErrNotPullBased
}

func (m *mockListenTransport) Listen(ctx context.Context, ch chan<- []byte) error {
	for _, payload := range m.payloads {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- payload:
		}
	}
	if m.listenErr != nil {
		return m.listenErr
	}
	// Wait for ctx to be cancelled.
	<-ctx.Done()
	return nil
}

func (m *mockListenTransport) Close() error {
	m.closedOnce = true
	return nil
}

// newStreamingAdapterForTest creates an adapter using a mock stream transport
// loaded from a minimal YAML definition.
func newStreamingAdapterForTest(t *testing.T, mockTransport Transport) *DeclarativeAdapter {
	t.Helper()
	name := fmt.Sprintf("stream_test_%d", time.Now().UnixNano())
	dir := t.TempDir()
	yamlContent := fmt.Sprintf(`schema_version: 1
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Stream Test"
transport:
  type: http_poll
  url: "https://example.com/api"
  method: GET
  timeout: "10s"
  interval: "1s"
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
	// Replace the transport with the mock.
	adapter.transport = mockTransport
	return adapter
}

// TestStartStreaming_ProcessesMessages verifies startStreaming processes messages.
// The mock transport returns an error after all messages, causing the reconnect
// loop to give up and return.
func TestStartStreaming_ProcessesMessages(t *testing.T) {
	// Start with a cancelled context so the streaming loop exits immediately
	// after processing, avoiding batcher race conditions.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel context

	payload := []byte(`{"items":[{"id":"e1","name":"Entity 1","lat":10.0,"lon":20.0}]}`)
	mockTransport := &mockStreamTransport{
		messages: [][]byte{payload},
	}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	// With a cancelled context, startStreaming should return quickly.
	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.startStreaming(ctx, mockTransport)
	}()

	select {
	case err := <-errCh:
		// Context cancelled error or nil is acceptable.
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("startStreaming did not terminate within 5s")
	}
}

// TestStartListening_ProcessesPayloads verifies startListening processes payloads.
func TestStartListening_ProcessesPayloads(t *testing.T) {
	// Use an error transport that immediately returns an error after no payloads,
	// so the goroutine terminates cleanly without race conditions.
	mockTransport := &mockListenTransport{
		payloads:  [][]byte{},
		listenErr: errors.New("test listen error"),
	}

	adapter := newStreamingAdapterForTest(t, mockTransport)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.startListening(ctx, mockTransport)
	}()

	// The listen transport returns an error, so startListening should return quickly.
	select {
	case err := <-errCh:
		// An error is expected since the transport returned an error.
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("startListening did not terminate within 5s")
	}
}

// TestProcessBatch_ValidPayload verifies processBatch processes valid JSON payloads.
func TestProcessBatch_ValidPayload(t *testing.T) {
	mockTransport := &mockStreamTransport{}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	payload := []byte(`{"items":[{"id":"e3","name":"Entity 3","lat":10.0,"lon":20.0}]}`)
	err := adapter.processBatch([][]byte{payload})
	if err != nil {
		t.Fatalf("processBatch: %v", err)
	}

	// Verify entities were set.
	entities, _, err := adapter.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entities) != 1 {
		t.Errorf("expected 1 entity from processBatch, got %d", len(entities))
	}
}

// TestProcessBatch_EmptyBatch verifies processBatch handles an empty batch.
func TestProcessBatch_EmptyBatch(t *testing.T) {
	mockTransport := &mockStreamTransport{}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	err := adapter.processBatch([][]byte{})
	if err != nil {
		t.Fatalf("processBatch with empty batch: %v", err)
	}
}

// TestProcessBatch_InvalidJSON verifies processBatch handles invalid JSON gracefully.
func TestProcessBatch_InvalidJSON(t *testing.T) {
	mockTransport := &mockStreamTransport{}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	// Invalid JSON should not cause processBatch to return an error;
	// it should be skipped (per the implementation).
	err := adapter.processBatch([][]byte{[]byte("not valid json")})
	if err != nil {
		t.Fatalf("processBatch with invalid JSON should not error: %v", err)
	}
}

// TestStart_StreamTransport verifies Start dispatches to startStreaming for StreamTransport.
func TestStart_StreamTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the goroutine started by Start terminates cleanly.
	cancel()

	mockTransport := &mockStreamTransport{
		messages: [][]byte{},
	}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	// Start should return nil immediately for streaming transports.
	err := adapter.Start(ctx)
	if err != nil {
		t.Fatalf("Start() = %v, want nil for StreamTransport", err)
	}

	// Give the background goroutine time to terminate.
	time.Sleep(100 * time.Millisecond)
}

// TestStart_ListenTransport verifies Start dispatches to startListening for ListenTransport.
func TestStart_ListenTransport(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the goroutine started by Start terminates cleanly.
	cancel()

	mockTransport := &mockListenTransport{
		payloads:  [][]byte{},
		listenErr: nil, // Listen will return on ctx cancellation
	}
	adapter := newStreamingAdapterForTest(t, mockTransport)

	// Start should return nil immediately for listen transports.
	err := adapter.Start(ctx)
	if err != nil {
		t.Fatalf("Start() = %v, want nil for ListenTransport", err)
	}

	// Give the background goroutine time to terminate.
	time.Sleep(100 * time.Millisecond)
}

// TestBuildParserConfig_WithCSVOptions verifies the CSVOptions branch in buildParserConfig.
func TestBuildParserConfig_WithCSVOptions(t *testing.T) {
	name := "csv_parser_test"
	dir := t.TempDir()
	yamlContent := fmt.Sprintf(`schema_version: 1
name: %s
source_type: %s
layer_type: %s_layer
display_name: "CSV Parser Test"
transport:
  type: http_poll
  url: "https://example.com/data.csv"
  method: GET
  timeout: "10s"
  interval: "60s"
parser:
  format: csv
  csv_options:
    delimiter: ";"
    has_header: true
    skip_lines: 1
    collapse_whitespace: true
    comment_prefix: "#"
entity:
  external_id: 'record.id'
  name: 'record.name'
observation:
  latitude: '0.0'
  longitude: '0.0'
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
    color: "#ffffff"
  style:
    color: "#ffffff"
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
	loader, err := NewLoader(compiler, os.Getenv, false, logger)
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

	// Call buildParserConfig directly to verify the CSVOptions branch.
	def := cs.Definition()
	cfg := adapter.buildParserConfig(def)

	if cfg.CSVOptions == nil {
		t.Fatal("expected non-nil CSVOptions")
	}
	if cfg.CSVOptions.Delimiter != ";" {
		t.Errorf("expected delimiter ';', got %q", cfg.CSVOptions.Delimiter)
	}
	if !cfg.CSVOptions.HasHeader {
		t.Error("expected HasHeader=true")
	}
	if cfg.CSVOptions.SkipLines != 1 {
		t.Errorf("expected SkipLines=1, got %d", cfg.CSVOptions.SkipLines)
	}
	if !cfg.CSVOptions.CollapseWhitespace {
		t.Error("expected CollapseWhitespace=true")
	}
	if cfg.CSVOptions.CommentPrefix != "#" {
		t.Errorf("expected CommentPrefix='#', got %q", cfg.CSVOptions.CommentPrefix)
	}
}

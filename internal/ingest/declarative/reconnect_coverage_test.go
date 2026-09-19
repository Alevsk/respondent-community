package declarative

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

func TestCalculateBackoff_FixedBackoff(t *testing.T) {
	spec := &RetrySpec{
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 500 * time.Millisecond},
		MaxDelay:     Duration{Duration: 10 * time.Second},
	}
	for attempt := 1; attempt <= 5; attempt++ {
		d := calculateBackoff(spec, attempt)
		// equal jitter: result is in [initialDelay/2, initialDelay)
		if d < 0 || d > 500*time.Millisecond {
			t.Errorf("attempt %d: expected delay in [0, 500ms], got %v", attempt, d)
		}
	}
}

func TestCalculateBackoff_DefaultBackoff(t *testing.T) {
	spec := &RetrySpec{
		Backoff:      "unknown_strategy",
		InitialDelay: Duration{Duration: 200 * time.Millisecond},
		MaxDelay:     Duration{Duration: 5 * time.Second},
	}
	d := calculateBackoff(spec, 1)
	if d < 0 {
		t.Errorf("expected non-negative backoff, got %v", d)
	}
}

func TestReconnectBackoff_HighAttempt(t *testing.T) {
	spec := &ReconnectSpec{
		MaxAttempts:  10,
		InitialDelay: Duration{Duration: 100 * time.Millisecond},
		MaxDelay:     Duration{Duration: 1 * time.Second},
		Backoff:      "exponential",
	}
	// Very high attempt number should still return at most maxDelay.
	d := reconnectBackoff(spec, 100)
	if d > spec.MaxDelay.Duration {
		t.Errorf("reconnectBackoff(100) = %v, want <= maxDelay %v", d, spec.MaxDelay.Duration)
	}
}

func TestSchemaReconnectSpec_InvalidDuration(t *testing.T) {
	dir := t.TempDir()
	yaml := `schema_version: 1
name: bad_reconnect_dur
source_type: bad_reconnect_dur
layer_type: bad_reconnect_dur_layer
display_name: "Bad Reconnect Duration"
transport:
  type: websocket
  url: "wss://example.com/ws"
  timeout: "10s"
  interval: "60s"
  reconnect:
    max_attempts: 3
    initial_delay: "not-a-duration"
    max_delay: "30s"
    backoff: "exponential"
parser:
  format: json
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
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`
	yamlPath := filepath.Join(dir, "bad_reconnect_dur.yaml")
	if err := os.WriteFile(yamlPath, []byte(yaml), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	_, err := loader.LoadFile(yamlPath)
	if err == nil {
		t.Error("expected error for invalid reconnect.initial_delay duration, got nil")
	}
}

func TestReconnectBackoff_DefaultCase(t *testing.T) {
	spec := &ReconnectSpec{
		Backoff:      "unknown_backoff_xyz",
		InitialDelay: Duration{Duration: 100 * time.Millisecond},
		MaxDelay:     Duration{Duration: 500 * time.Millisecond},
	}
	delay := reconnectBackoff(spec, 1)
	if delay <= 0 {
		t.Errorf("expected positive delay for default backoff, got %v", delay)
	}
}

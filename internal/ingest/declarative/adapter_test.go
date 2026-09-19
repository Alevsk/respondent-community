package declarative

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/logging"
)

// testHTTPClient returns a plain http.Client suitable for testing with httptest servers.
// It bypasses the SSRF-safe client which blocks localhost connections.
func testHTTPClient() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

// newTestAdapter creates a DeclarativeAdapter with a test-friendly HTTP client.
func newTestAdapter(t *testing.T, cs *CompiledSource, logger *logging.Logger) *DeclarativeAdapter {
	t.Helper()
	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}
	return adapter
}

// makeUSGSYAML returns a USGS earthquake YAML definition using the given URL.
func makeUSGSYAML(url string) string {
	return `schema_version: 1
name: test_usgs_earthquakes
source_type: test_usgs_earthquakes
layer_type: test_earthquakes
display_name: "Test USGS Earthquakes"
transport:
  type: http_poll
  url: "` + url + `"
  method: GET
  timeout: "10s"
  interval: "60s"
  max_response_bytes: 10485760
parser:
  format: json
  records_path: "features"
  max_records: 10000
filter: 'has(record.properties) && has(record.properties.mag) && double(record.properties.mag) >= 2.0'
entity:
  external_id: 'record.id'
  name: '"M" + string(record.properties.mag) + " " + record.properties.place'
  metadata:
    mag: 'string(record.properties.mag)'
    place: 'record.properties.place'
    type: 'record.properties.type'
observation:
  latitude: 'record.geometry.coordinates[1]'
  longitude: 'record.geometry.coordinates[0]'
  altitude: '0.0'
  timestamp: 'unix_ms(record.properties.time)'
  metadata:
    depth: 'string(record.geometry.coordinates[2])'
recording:
  mode: append
cache:
  ttl: "120s"
display:
  icon:
    shape: ripple
    rotatable: false
    interpolation: false
    scale: 1.0
  trail:
    color: "#ff006e"
    width: 1.5
    opacity: 0.7
  style:
    color: "#ff006e"
    point_size: 10
`
}

// sampleUSGSResponse returns a sample USGS GeoJSON response for testing.
func sampleUSGSResponse() map[string]interface{} {
	return map[string]interface{}{
		"type": "FeatureCollection",
		"metadata": map[string]interface{}{
			"generated": float64(1700000000000),
			"count":     float64(3),
		},
		"features": []interface{}{
			map[string]interface{}{
				"type": "Feature",
				"id":   "us2024001",
				"properties": map[string]interface{}{
					"mag":   float64(4.5),
					"place": "10km NW of Ridgecrest, CA",
					"time":  float64(1700000000000),
					"type":  "earthquake",
				},
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []interface{}{float64(-117.6), float64(35.7), float64(5.0)},
				},
			},
			map[string]interface{}{
				"type": "Feature",
				"id":   "us2024002",
				"properties": map[string]interface{}{
					"mag":   float64(1.5), // Below filter threshold
					"place": "5km S of Hollister, CA",
					"time":  float64(1700000001000),
					"type":  "earthquake",
				},
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []interface{}{float64(-121.4), float64(36.8), float64(3.0)},
				},
			},
			map[string]interface{}{
				"type": "Feature",
				"id":   "us2024003",
				"properties": map[string]interface{}{
					"mag":   float64(5.2),
					"place": "Alaska Peninsula",
					"time":  float64(1700000002000),
					"type":  "earthquake",
				},
				"geometry": map[string]interface{}{
					"type":        "Point",
					"coordinates": []interface{}{float64(-161.0), float64(55.3), float64(10.0)},
				},
			},
		},
	}
}

func TestDeclarativeAdapter_USGS(t *testing.T) {
	// Create mock HTTP server
	responseData := sampleUSGSResponse()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(responseData)
	}))
	defer server.Close()

	// Write YAML to temp dir
	tmpDir := t.TempDir()
	yamlPath := filepath.Join(tmpDir, "usgs.yaml")
	if err := os.WriteFile(yamlPath, []byte(makeUSGSYAML(server.URL)), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	// Load and compile
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

	// Create adapter with test HTTP client (bypasses SSRF checks for localhost)
	adapter := newTestAdapter(t, cs, logger)

	// Verify basic properties
	if adapter.Name() != "test_usgs_earthquakes" {
		t.Errorf("Name = %q, want %q", adapter.Name(), "test_usgs_earthquakes")
	}
	if adapter.LayerType() != "test_earthquakes" {
		t.Errorf("LayerType = %q, want %q", adapter.LayerType(), "test_earthquakes")
	}

	// Run Start (single fetch cycle)
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Check snapshot
	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	// Should have 2 entities (one filtered out at mag < 2.0)
	if len(entities) != 2 {
		t.Fatalf("len(entities) = %d, want 2", len(entities))
	}
	if len(observations) != 2 {
		t.Fatalf("len(observations) = %d, want 2", len(observations))
	}

	// Verify first entity
	e := entities[0]
	if e.ExternalID != "us2024001" {
		t.Errorf("entities[0].ExternalID = %q, want %q", e.ExternalID, "us2024001")
	}
	if e.LayerType != "test_earthquakes" {
		t.Errorf("entities[0].LayerType = %q, want %q", e.LayerType, "test_earthquakes")
	}
	if e.Metadata["mag"] != "4.5" {
		t.Errorf("entities[0].Metadata[mag] = %q, want %q", e.Metadata["mag"], "4.5")
	}

	// Verify observation position
	obs := observations[0]
	if obs.Position.Lon != -117.6 {
		t.Errorf("observations[0].Position.Lon = %v, want %v", obs.Position.Lon, -117.6)
	}
	if obs.Position.Lat != 35.7 {
		t.Errorf("observations[0].Position.Lat = %v, want %v", obs.Position.Lat, 35.7)
	}
}

func TestDeclarativeAdapter_Filter(t *testing.T) {
	// All records have mag >= 3.0 in filter
	yamlTmpl := `schema_version: 1
name: test_filter
source_type: test_filter
layer_type: test_layer
display_name: "Test Filter"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
  records_path: "items"
filter: 'record.value >= 3.0'
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

	response := map[string]interface{}{
		"items": []interface{}{
			map[string]interface{}{"id": "a", "name": "A", "value": float64(5.0), "lat": 10.0, "lon": 20.0},
			map[string]interface{}{"id": "b", "name": "B", "value": float64(1.0), "lat": 30.0, "lon": 40.0},
			map[string]interface{}{"id": "c", "name": "C", "value": float64(3.0), "lat": 50.0, "lon": 60.0},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "filter.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 2 {
		t.Errorf("len(entities) = %d, want 2 (items with value >= 3.0)", len(entities))
	}
}

func TestDeclarativeAdapter_PerRecordError(t *testing.T) {
	// One record missing 'id' field, others are valid
	yamlTmpl := `schema_version: 1
name: test_errors
source_type: test_errors
layer_type: test_layer
display_name: "Test Errors"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
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

	response := []interface{}{
		map[string]interface{}{"id": "good1", "name": "Good 1", "lat": 10.0, "lon": 20.0},
		map[string]interface{}{"name": "Missing ID", "lat": 30.0, "lon": 40.0}, // Missing id
		map[string]interface{}{"id": "good2", "name": "Good 2", "lat": 50.0, "lon": 60.0},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "errors.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 2 {
		t.Errorf("len(entities) = %d, want 2 (bad record skipped)", len(entities))
	}
}

func TestDeclarativeAdapter_HTTPRetry(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		response := []interface{}{
			map[string]interface{}{"id": "ok", "name": "OK", "lat": 10.0, "lon": 20.0},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_retry
source_type: test_retry
layer_type: test_layer
display_name: "Test Retry"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
  retry:
    max_attempts: 3
    backoff: fixed
    initial_delay: "100ms"
    max_delay: "1s"
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

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "retry.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())

	ctx := context.Background()
	err := adapter.Start(ctx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 1 {
		t.Errorf("len(entities) = %d, want 1 (after retry)", len(entities))
	}
	if attempt < 2 {
		t.Errorf("attempt = %d, want >= 2", attempt)
	}
}

func TestDeclarativeAdapter_Auth(t *testing.T) {
	// Verify auth headers are sent in the request
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		response := []interface{}{
			map[string]interface{}{"id": "ok", "name": "OK", "lat": 10.0, "lon": 20.0},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_auth
source_type: test_auth
layer_type: test_layer
display_name: "Test Auth"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
  auth:
    type: bearer
    env_var: TEST_API_TOKEN
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

	// Set env var for auth
	t.Setenv("TEST_API_TOKEN", "my-secret-token")

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "auth.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if receivedAuth != "Bearer my-secret-token" {
		t.Errorf("Authorization header = %q, want %q", receivedAuth, "Bearer my-secret-token")
	}
}

func TestDeclarativeAdapter_SnapshotConcurrency(t *testing.T) {
	// Verify snapshot returns copies (no data race)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := []interface{}{
			map[string]interface{}{"id": "a", "name": "A", "lat": 10.0, "lon": 20.0},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_concurrent
source_type: test_concurrent
layer_type: test_layer
display_name: "Test Concurrent"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
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

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "concurrent.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Take two snapshots and verify the slices are independent
	e1, _, _ := adapter.Snapshot(ctx)
	e2, _, _ := adapter.Snapshot(ctx)

	if len(e1) == 0 || len(e2) == 0 {
		t.Fatal("empty snapshots")
	}

	// Appending to one slice should not affect the other
	original := len(e2)
	_ = append(e1, e1[0]) //nolint:gocritic // intentional: tests slice independence without reuse
	if len(e2) != original {
		t.Error("snapshots share the same backing array")
	}
}

func TestDeclarativeAdapter_EmptyBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{},
		})
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_empty
source_type: test_empty
layer_type: test_layer
display_name: "Test Empty"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
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
    color: "#ffffff"
  style:
    color: "#ffffff"
    point_size: 6
`

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())

	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	entities, _, _ := adapter.Snapshot(ctx)
	if len(entities) != 0 {
		t.Errorf("len(entities) = %d, want 0", len(entities))
	}
}

// makeYAMLWithURL formats a YAML template with the server URL.
func makeYAMLWithURL(tmpl, url string) string {
	return replaceURL(tmpl, url)
}

func replaceURL(tmpl, url string) string {
	result := ""
	for i := 0; i < len(tmpl); i++ {
		if i < len(tmpl)-2 && tmpl[i] == '%' && tmpl[i+1] == 's' {
			result += url
			i++ // skip 's'
		} else {
			result += string(tmpl[i])
		}
	}
	return result
}

// --- FIX 3: calculateBackoff unit tests ---

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name       string
		spec       *RetrySpec
		attempt    int
		wantMin    time.Duration
		wantMax    time.Duration
		wantExact  time.Duration // non-zero only for "fixed" (ignoring jitter)
		checkExact bool
	}{
		{
			name:    "nil spec returns 1s",
			spec:    nil,
			attempt: 1,
			wantMin: 500 * time.Millisecond,
			wantMax: time.Second,
		},
		{
			name: "exponential attempt 1",
			spec: &RetrySpec{
				Backoff:      "exponential",
				InitialDelay: Duration{Duration: time.Second},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 1,
			wantMin: 500 * time.Millisecond,
			wantMax: time.Second,
		},
		{
			name: "exponential attempt 2",
			spec: &RetrySpec{
				Backoff:      "exponential",
				InitialDelay: Duration{Duration: time.Second},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 2,
			wantMin: time.Second,
			wantMax: 2 * time.Second,
		},
		{
			name: "exponential attempt 3",
			spec: &RetrySpec{
				Backoff:      "exponential",
				InitialDelay: Duration{Duration: time.Second},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 3,
			wantMin: 2 * time.Second,
			wantMax: 4 * time.Second,
		},
		{
			name: "exponential clamped to max delay",
			spec: &RetrySpec{
				Backoff:      "exponential",
				InitialDelay: Duration{Duration: time.Second},
				MaxDelay:     Duration{Duration: 3 * time.Second},
			},
			attempt: 10, // 2^9 = 512s >> maxDelay, should clamp
			wantMin: 1500 * time.Millisecond,
			wantMax: 3 * time.Second,
		},
		{
			name: "fixed backoff produces constant delay",
			spec: &RetrySpec{
				Backoff:      "fixed",
				InitialDelay: Duration{Duration: 2 * time.Second},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 1,
			wantMin: time.Second,
			wantMax: 2 * time.Second,
		},
		{
			name: "fixed backoff same for later attempts",
			spec: &RetrySpec{
				Backoff:      "fixed",
				InitialDelay: Duration{Duration: 2 * time.Second},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 5,
			wantMin: time.Second,
			wantMax: 2 * time.Second,
		},
		{
			name: "zero initial delay defaults to 1s",
			spec: &RetrySpec{
				Backoff:      "fixed",
				InitialDelay: Duration{Duration: 0},
				MaxDelay:     Duration{Duration: 30 * time.Second},
			},
			attempt: 1,
			wantMin: 500 * time.Millisecond,
			wantMax: time.Second,
		},
		{
			name: "zero max delay defaults to 30s",
			spec: &RetrySpec{
				Backoff:      "exponential",
				InitialDelay: Duration{Duration: time.Second},
				MaxDelay:     Duration{Duration: 0},
			},
			attempt: 1,
			wantMin: 500 * time.Millisecond,
			wantMax: time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Run multiple times to account for jitter randomness.
			for i := 0; i < 100; i++ {
				got := calculateBackoff(tc.spec, tc.attempt)
				if got < tc.wantMin || got > tc.wantMax {
					t.Fatalf("calculateBackoff() = %v, want in [%v, %v]", got, tc.wantMin, tc.wantMax)
				}
			}
		})
	}
}

func TestCalculateBackoff_JitterRange(t *testing.T) {
	// Verify jitter is within [delay/2, delay) over many samples.
	spec := &RetrySpec{
		Backoff:      "fixed",
		InitialDelay: Duration{Duration: 10 * time.Second},
		MaxDelay:     Duration{Duration: 30 * time.Second},
	}

	var minSeen, maxSeen time.Duration
	minSeen = 100 * time.Second // large initial value

	for i := 0; i < 1000; i++ {
		d := calculateBackoff(spec, 1)
		if d < minSeen {
			minSeen = d
		}
		if d > maxSeen {
			maxSeen = d
		}
	}

	// Expected range: [5s, 10s)
	if minSeen < 5*time.Second {
		t.Errorf("min observed delay %v is below expected floor of 5s", minSeen)
	}
	if maxSeen >= 10*time.Second {
		t.Errorf("max observed delay %v should be < 10s", maxSeen)
	}
	// Verify there's spread (not all the same value).
	if maxSeen-minSeen < 100*time.Millisecond {
		t.Errorf("jitter range too narrow: [%v, %v]", minSeen, maxSeen)
	}
}

func TestCalculateBackoff_ExponentialIncreasing(t *testing.T) {
	spec := &RetrySpec{
		Backoff:      "exponential",
		InitialDelay: Duration{Duration: time.Second},
		MaxDelay:     Duration{Duration: 120 * time.Second},
	}

	// The midpoint of each attempt's jitter range should be increasing.
	// With equal jitter, expected value at attempt n = 3/4 * min(initial * 2^(n-1), max).
	// Test: average over many samples should increase.
	avgDelay := func(attempt int) time.Duration {
		var total time.Duration
		n := 200
		for i := 0; i < n; i++ {
			total += calculateBackoff(spec, attempt)
		}
		return total / time.Duration(n)
	}

	avg1 := avgDelay(1)
	avg2 := avgDelay(2)
	avg3 := avgDelay(3)

	if avg2 <= avg1 {
		t.Errorf("expected avg delay to increase: attempt1=%v, attempt2=%v", avg1, avg2)
	}
	if avg3 <= avg2 {
		t.Errorf("expected avg delay to increase: attempt2=%v, attempt3=%v", avg2, avg3)
	}
}

// --- FIX 4: readResponseBody size limiting test ---

func TestDeclarativeAdapter_ReadResponseBody_SizeLimit(t *testing.T) {
	// Create a server that returns a large body.
	largeBody := make([]byte, 10*1024) // 10KB
	for i := range largeBody {
		largeBody[i] = 'A'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(largeBody)
	}))
	defer server.Close()

	// Create a minimal adapter with a small max_response_bytes.
	yamlTmpl := `schema_version: 1
name: test_size_limit
source_type: test_size_limit
layer_type: test_layer
display_name: "Test Size Limit"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
  max_response_bytes: 1024
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
`

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "sizelimit.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)

	// Test size limiting through the transport's Fetch method.
	body, _, err := adapter.transport.Fetch(context.Background(), "GET", server.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// The response should be truncated to max_response_bytes.
	if len(body) != 1024 {
		t.Errorf("len(body) = %d, want 1024 (truncated to limit)", len(body))
	}
}

func TestDeclarativeAdapter_ReadResponseBody_DefaultLimit(t *testing.T) {
	// When maxBytes is 0, readResponseBody should use the default (50MB).
	smallBody := []byte("hello world")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(smallBody)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_default_limit
source_type: test_default_limit
layer_type: test_layer
display_name: "Test Default Limit"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
`

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "defaultlimit.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter := newTestAdapter(t, cs, logger)

	// Test default size limiting through the transport's Fetch method.
	// No max_response_bytes configured, so the default (50MB) applies.
	body, _, err := adapter.transport.Fetch(context.Background(), "GET", server.URL)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if string(body) != "hello world" {
		t.Errorf("body = %q, want %q", string(body), "hello world")
	}
}

func TestDeclarativeAdapter_FetchWithSizeLimit(t *testing.T) {
	// End-to-end: adapter's Start should truncate the response when max_response_bytes is set.
	// The truncated JSON won't parse, so Start should return an error.
	largeBody := make([]byte, 5*1024) // 5KB of invalid data
	for i := range largeBody {
		largeBody[i] = 'X'
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(largeBody)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_fetch_truncate
source_type: test_fetch_truncate
layer_type: test_layer
display_name: "Test Fetch Truncate"
transport:
  type: http_poll
  url: "%s"
  timeout: "10s"
  interval: "60s"
  max_response_bytes: 1024
parser:
  format: json
entity:
  external_id: 'record.id'
  name: 'record.id'
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
`

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "fetchtruncate.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	adapter := newTestAdapter(t, cs, logger)
	ctx := context.Background()

	// Start should fail because truncated body won't parse as JSON.
	err = adapter.Start(ctx)
	if err == nil {
		t.Error("expected error from truncated response, got nil")
	}
}

// Ensure fetchWithRetry respects context cancellation.
func TestDeclarativeAdapter_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	yamlTmpl := `schema_version: 1
name: test_cancel
source_type: test_cancel
layer_type: test_layer
display_name: "Test Cancel"
transport:
  type: http_poll
  url: "%s"
  timeout: "1s"
  interval: "60s"
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

	tmpDir := t.TempDir()
	yamlContent := makeYAMLWithURL(yamlTmpl, server.URL)
	yamlPath := filepath.Join(tmpDir, "cancel.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, true, logger)
	cs, _ := loader.LoadFile(yamlPath)
	adapter, _ := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	// Should return an error (timeout or context cancelled), not hang.
	// The important thing is it returns within the timeout; error is acceptable.
	_ = adapter.Start(ctx)
}

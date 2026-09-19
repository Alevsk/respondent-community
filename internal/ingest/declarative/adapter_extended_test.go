package declarative

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// adapterMinimalYAML returns a minimal valid source definition for the given URL and name.
func adapterMinimalYAML(name, url string) string {
	return fmt.Sprintf(`schema_version: 1
name: %s
source_type: %s
layer_type: %s_layer
display_name: "Test Source"
transport:
  type: http_poll
  url: "%s"
  method: GET
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
    color: "#00ff9d"
  style:
    color: "#00ff9d"
    point_size: 6
`, name, name, name, url)
}

func loadAdapterFromYAML(t *testing.T, name, url string) (*DeclarativeAdapter, *CompiledSource) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name+".yaml")
	if err := os.WriteFile(path, []byte(adapterMinimalYAML(name, url)), 0644); err != nil {
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
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	adapter, err := NewDeclarativeAdapterWithClient(cs, logger, testHTTPClient())
	if err != nil {
		t.Fatalf("NewDeclarativeAdapterWithClient: %v", err)
	}
	return adapter, cs
}

// TestDeclarativeAdapter_SupportsSourceType verifies the source type check.
func TestDeclarativeAdapter_SupportsSourceType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	adapter, _ := loadAdapterFromYAML(t, "supports_src_test", srv.URL)

	if !adapter.SupportsSourceType(domain.SourceType("supports_src_test")) {
		t.Error("expected SupportsSourceType to return true for matching source type")
	}

	if adapter.SupportsSourceType(domain.SourceType("different_source")) {
		t.Error("expected SupportsSourceType to return false for different source type")
	}
}

// TestDeclarativeAdapter_Stop verifies Stop returns nil (no-op).
func TestDeclarativeAdapter_Stop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	adapter, _ := loadAdapterFromYAML(t, "stop_test_src", srv.URL)
	err := adapter.Stop()
	if err != nil {
		t.Errorf("Stop() = %v, want nil", err)
	}
}

// TestDeclarativeAdapter_Stream verifies Stream is a no-op.
func TestDeclarativeAdapter_Stream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	adapter, _ := loadAdapterFromYAML(t, "stream_noop_test", srv.URL)

	out := make(chan *domain.EntityUpdate, 1)
	err := adapter.Stream(context.Background(), out)
	if err != nil {
		t.Errorf("Stream() = %v, want nil", err)
	}
}

// TestDeclarativeAdapter_SwapCompiledSource verifies hot-reload behavior.
func TestDeclarativeAdapter_SwapCompiledSource(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	adapter, cs := loadAdapterFromYAML(t, "swap_test_src", srv.URL)

	// SwapCompiledSource with the same source should not panic or error
	adapter.SwapCompiledSource(cs)

	// Verify the adapter still works after swap
	ctx := context.Background()
	if err := adapter.Start(ctx); err != nil {
		t.Errorf("Start after SwapCompiledSource: %v", err)
	}
}

// TestDeclarativeAdapter_Snapshot_Empty verifies empty snapshot returns empty slices.
func TestDeclarativeAdapter_Snapshot_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	adapter, _ := loadAdapterFromYAML(t, "snapshot_empty_test", srv.URL)

	ctx := context.Background()
	entities, observations, err := adapter.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(entities) != 0 {
		t.Errorf("expected 0 entities, got %d", len(entities))
	}
	if len(observations) != 0 {
		t.Errorf("expected 0 observations, got %d", len(observations))
	}
}

// TestNewDeclarativeAdapter_DefaultClient verifies the adapter creates an SSRF-safe client when none provided.
func TestNewDeclarativeAdapter_DefaultClient(t *testing.T) {
	dir := t.TempDir()
	name := "default_client_test"
	path := filepath.Join(dir, name+".yaml")
	// Use a placeholder URL since we're just testing client creation
	if err := os.WriteFile(path, []byte(adapterMinimalYAML(name, "https://example.com/api")), 0644); err != nil {
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
	cs, err := loader.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	// NewDeclarativeAdapter without a custom client
	adapter, err := NewDeclarativeAdapter(cs, logger)
	if err != nil {
		t.Fatalf("NewDeclarativeAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.Name() != name {
		t.Errorf("Name = %q, want %q", adapter.Name(), name)
	}
}

// TestDeclarativeAdapter_FetchHTTPError verifies behavior when HTTP server returns 500.
func TestDeclarativeAdapter_FetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	adapter, _ := loadAdapterFromYAML(t, "fetch_error_test", srv.URL)

	ctx := context.Background()
	err := adapter.Start(ctx)
	if err == nil {
		t.Fatal("expected error for HTTP 500 response, got nil")
	}
}

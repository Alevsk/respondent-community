package declarative

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

// fakeAdapterManager records calls for test assertions.
type fakeAdapterManager struct {
	mu       sync.Mutex
	started  []string
	stopped  []string
	swapped  []string
	startErr error
	stopErr  error
	swapErr  error
}

func (f *fakeAdapterManager) StartDeclarativeAdapter(_ context.Context, cs *CompiledSource) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, cs.Definition().Name)
	return f.startErr
}

func (f *fakeAdapterManager) StopDeclarativeAdapter(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, name)
	return f.stopErr
}

func (f *fakeAdapterManager) SwapDeclarativeAdapter(name string, _ *CompiledSource) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.swapped = append(f.swapped, name)
	return f.swapErr
}

func (f *fakeAdapterManager) getStarted() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.started))
	copy(cp, f.started)
	return cp
}

func (f *fakeAdapterManager) getStopped() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.stopped))
	copy(cp, f.stopped)
	return cp
}

func (f *fakeAdapterManager) getSwapped() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]string, len(f.swapped))
	copy(cp, f.swapped)
	return cp
}

// minimalSourceYAML returns a minimal valid source YAML for testing with a unique name.
func minimalSourceYAML(name string) string {
	return `schema_version: 1
name: ` + name + `
source_type: ` + name + `
layer_type: test_layer_` + name + `
display_name: "Test ` + name + `"
transport:
  type: http_poll
  url: "https://example.com/api"
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
}

func newTestWatcher(t *testing.T, dir string, manager AdapterManager) (*Watcher, *Registry) {
	t.Helper()
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, os.Getenv, false, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	registry := NewRegistry()
	w := NewWatcher(dir, loader, registry, manager, domain.NewDynamicSourceRegistry(), logger)
	return w, registry
}

// waitForCondition polls a condition function until it returns true or timeout.
func waitForCondition(timeout time.Duration, check func() bool) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(50 * time.Millisecond)
	}
	return false
}

func TestWatcher_NewFileDetected(t *testing.T) {
	dir := t.TempDir()
	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = w.Start(ctx) }()

	// Wait for watcher to initialize.
	time.Sleep(200 * time.Millisecond)

	// Write a new YAML file.
	yamlPath := filepath.Join(dir, "new_source.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("new_source")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	// Wait for debounce (2s) + ticker (500ms) + processing time.
	ok := waitForCondition(5*time.Second, func() bool {
		return len(mgr.getStarted()) > 0
	})
	if !ok {
		t.Fatal("timed out waiting for adapter to be started")
	}

	started := mgr.getStarted()
	if len(started) != 1 || started[0] != "new_source" {
		t.Errorf("started = %v, want [new_source]", started)
	}

	// Verify registry was updated.
	if _, exists := registry.Get("new_source"); !exists {
		t.Error("expected source to be in registry")
	}
}

func TestWatcher_ModifiedFileTriggersSwap(t *testing.T) {
	dir := t.TempDir()

	// Pre-create a source file before starting the watcher.
	yamlPath := filepath.Join(dir, "existing.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("existing")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	// Pre-load the source into the registry so the watcher treats modification as a swap.
	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	registry.Add("existing", cs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = w.Start(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Modify the file (rewrite with same content but different display_name).
	modifiedYAML := minimalSourceYAML("existing")
	if err := os.WriteFile(yamlPath, []byte(modifiedYAML), 0644); err != nil {
		t.Fatalf("write modified YAML: %v", err)
	}

	ok := waitForCondition(5*time.Second, func() bool {
		return len(mgr.getSwapped()) > 0
	})
	if !ok {
		t.Fatal("timed out waiting for adapter to be swapped")
	}

	swapped := mgr.getSwapped()
	if len(swapped) != 1 || swapped[0] != "existing" {
		t.Errorf("swapped = %v, want [existing]", swapped)
	}
}

func TestWatcher_DeletedFileTriggersRemoval(t *testing.T) {
	dir := t.TempDir()

	// Pre-create source.
	yamlPath := filepath.Join(dir, "deleteme.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("deleteme")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	// Pre-load.
	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, _ := loader.LoadFile(yamlPath)
	registry.Add("deleteme", cs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = w.Start(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Track the known source so handleFileChange can find it for removal.
	w.mu.Lock()
	w.knownSources["deleteme.yaml"] = "deleteme"
	w.mu.Unlock()

	// Delete the file.
	if err := os.Remove(yamlPath); err != nil {
		t.Fatalf("remove YAML: %v", err)
	}

	ok := waitForCondition(5*time.Second, func() bool {
		return len(mgr.getStopped()) > 0
	})
	if !ok {
		t.Fatal("timed out waiting for adapter to be stopped")
	}

	stopped := mgr.getStopped()
	if len(stopped) != 1 || stopped[0] != "deleteme" {
		t.Errorf("stopped = %v, want [deleteme]", stopped)
	}

	// Verify registry removal.
	if _, exists := registry.Get("deleteme"); exists {
		t.Error("expected source to be removed from registry")
	}
}

func TestWatcher_InvalidYAMLDoesNotCrash(t *testing.T) {
	dir := t.TempDir()

	// Pre-create a valid source.
	validPath := filepath.Join(dir, "valid.yaml")
	if err := os.WriteFile(validPath, []byte(minimalSourceYAML("valid")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, _ := loader.LoadFile(validPath)
	registry.Add("valid", cs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = w.Start(ctx) }()
	time.Sleep(200 * time.Millisecond)

	// Write an invalid YAML file.
	invalidPath := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(invalidPath, []byte("not: [valid: yaml: {"), 0644); err != nil {
		t.Fatalf("write invalid YAML: %v", err)
	}

	// Wait for debounce to process.
	time.Sleep(4 * time.Second)

	// The valid source should still be in the registry.
	if _, exists := registry.Get("valid"); !exists {
		t.Error("valid source should still be in registry after invalid YAML was added")
	}

	// No adapters should have been stopped.
	if len(mgr.getStopped()) != 0 {
		t.Errorf("no adapters should have been stopped, got %v", mgr.getStopped())
	}
}

func TestWatcher_RateLimiting(t *testing.T) {
	dir := t.TempDir()

	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	// Pre-create source and register it.
	yamlPath := filepath.Join(dir, "ratelimit.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("ratelimit")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, _ := loader.LoadFile(yamlPath)
	registry.Add("ratelimit", cs)

	// Simulate that this source was recently reloaded.
	w.mu.Lock()
	w.knownSources["ratelimit.yaml"] = "ratelimit"
	w.lastReload["ratelimit"] = time.Now()
	w.mu.Unlock()

	// Call handleFileChange directly (simulating a debounced event).
	ctx := context.Background()
	w.handleFileChange(ctx, yamlPath)

	// Should have been rate limited -- no swap should occur.
	if len(mgr.getSwapped()) != 0 {
		t.Errorf("expected rate limiting to prevent swap, got swapped = %v", mgr.getSwapped())
	}
}

func TestWatcher_NonYAMLFilesIgnored(t *testing.T) {
	// Verify isYAMLFile correctly filters.
	tests := []struct {
		name string
		file string
		want bool
	}{
		{"yaml extension", "source.yaml", true},
		{"yml extension", "source.yml", true},
		{"YAML uppercase", "source.YAML", true},
		{"txt extension", "readme.txt", false},
		{"json extension", "config.json", false},
		{"no extension", "Makefile", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isYAMLFile(tc.file)
			if got != tc.want {
				t.Errorf("isYAMLFile(%q) = %v, want %v", tc.file, got, tc.want)
			}
		})
	}
}

package declarative

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

func TestWatcher_HandleFileChange_SwapError(t *testing.T) {
	dir := t.TempDir()

	yamlPath := filepath.Join(dir, "swaperr.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("swaperr")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{swapErr: errors.New("swap failed")}
	w, registry := newTestWatcher(t, dir, mgr)

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	registry.Add("swaperr", cs)

	ctx := context.Background()
	// Call handleFileChange directly to trigger the swap error path.
	w.handleFileChange(ctx, yamlPath)

	// Despite swap error, the function should return without panicking.
	swapped := mgr.getSwapped()
	if len(swapped) != 1 || swapped[0] != "swaperr" {
		t.Errorf("expected swaperr to be attempted, got %v", swapped)
	}
}

func TestWatcher_HandleFileChange_StartError(t *testing.T) {
	dir := t.TempDir()

	yamlPath := filepath.Join(dir, "starterr.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("starterr")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{startErr: errors.New("start failed")}
	w, _ := newTestWatcher(t, dir, mgr)
	// Don't add to registry so it's treated as a new source.

	ctx := context.Background()
	w.handleFileChange(ctx, yamlPath)

	started := mgr.getStarted()
	if len(started) != 1 || started[0] != "starterr" {
		t.Errorf("expected start to be attempted, got %v", started)
	}
}

func TestWatcher_HandleSourceRemoval_StopError(t *testing.T) {
	dir := t.TempDir()

	yamlPath := filepath.Join(dir, "stopdel.yaml")
	if err := os.WriteFile(yamlPath, []byte(minimalSourceYAML("stopdel")), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	mgr := &fakeAdapterManager{stopErr: errors.New("stop failed")}
	w, registry := newTestWatcher(t, dir, mgr)

	compiler, _ := NewCELCompiler()
	logger := logging.NewNopLogger()
	loader, _ := NewLoader(compiler, os.Getenv, false, logger)
	cs, err := loader.LoadFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	registry.Add("stopdel", cs)
	w.mu.Lock()
	w.knownSources["stopdel.yaml"] = "stopdel"
	w.mu.Unlock()

	// Call handleSourceRemoval directly.
	w.handleSourceRemoval("stopdel", "stopdel.yaml")

	// Stop should have been called despite error.
	stopped := mgr.getStopped()
	if len(stopped) != 1 || stopped[0] != "stopdel" {
		t.Errorf("expected stopdel to be stopped, got %v", stopped)
	}

	// Source should be removed from registry.
	if _, exists := registry.Get("stopdel"); exists {
		t.Error("expected source to be removed from registry after removal")
	}
}

func TestWatcher_HandleSourceRemoval_NonExistentSource(t *testing.T) {
	dir := t.TempDir()
	mgr := &fakeAdapterManager{}
	w, _ := newTestWatcher(t, dir, mgr)

	// handleSourceRemoval for a source not in registry should be a no-op.
	w.handleSourceRemoval("nonexistent", "nonexistent.yaml")

	if len(mgr.getStopped()) != 0 {
		t.Error("expected no stop calls for nonexistent source")
	}
}

func TestWatcher_Start_ContextCancelExit(t *testing.T) {
	dir := t.TempDir()
	mgr := &fakeAdapterManager{}
	w, _ := newTestWatcher(t, dir, mgr)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Start(ctx)
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error on ctx cancel, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Start() did not return after ctx cancel")
	}
}

func TestWatcher_Start_NonExistentDir(t *testing.T) {
	nonExistentDir := filepath.Join(t.TempDir(), "nonexistent")
	mgr := &fakeAdapterManager{}
	w, _ := newTestWatcher(t, nonExistentDir, mgr)

	err := w.Start(context.Background())
	if err == nil {
		t.Error("expected error for non-existent directory, got nil")
	}
}

func TestWatcher_HandleFileChange_RateLimitWindow(t *testing.T) {
	dir := t.TempDir()
	mgr := &fakeAdapterManager{}
	w, registry := newTestWatcher(t, dir, mgr)

	// Pre-load a source into registry.
	yamlContent := minimalSourceYAML("rl_window_src")
	yamlPath := filepath.Join(dir, "rl_window_src.yaml")
	if err := os.WriteFile(yamlPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}
	compiler, _ := NewCELCompiler()
	loader, _ := NewLoader(compiler, os.Getenv, true, logging.NewNopLogger())
	cs, _ := loader.LoadFile(yamlPath)
	registry.Add("rl_window_src", cs)

	// Set lastReload to a very recent time to trigger rate limiting.
	w.mu.Lock()
	w.knownSources["rl_window_src.yaml"] = "rl_window_src"
	w.lastReload["rl_window_src"] = time.Now() // just reloaded
	w.mu.Unlock()

	// handleFileChange should be rate-limited.
	w.handleFileChange(context.Background(), yamlPath)
	if len(mgr.getSwapped()) != 0 {
		t.Error("expected rate limiting to prevent swap")
	}
}

func TestWatcher_Start_InvalidDir(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, os.Getenv, false, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	registry := NewRegistry()

	watcher := NewWatcher("/nonexistent/directory/xyz123", loader, registry, nil, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = watcher.Start(ctx)
	if err == nil {
		t.Fatal("expected error for nonexistent directory, got nil")
	}
}

func TestWatcher_Start_FileEventAndReload(t *testing.T) {
	dir := t.TempDir()

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, os.Getenv, false, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	registry := NewRegistry()
	manager := &fakeAdapterManager{}

	watcher := NewWatcher(dir, loader, registry, manager, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- watcher.Start(ctx)
	}()

	// Wait for watcher to start.
	time.Sleep(50 * time.Millisecond)

	// Write a valid source YAML file to trigger a file event.
	yamlContent := minimalSourceYAML("watcher_reload_source")
	srcFile := filepath.Join(dir, "watcher_reload_source.yaml")
	if err := os.WriteFile(srcFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("write YAML: %v", err)
	}

	// Wait for the debounce to fire (debounceQuietTime = 2s + ticker 500ms).
	time.Sleep(3 * time.Second)

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("Start returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return within timeout")
	}
}

func TestWatcher_Start_CtxCancelBeforeEvents(t *testing.T) {
	dir := t.TempDir()

	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	loader, err := NewLoader(compiler, func(s string) string { return "" }, false, logging.NewNopLogger())
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	registry := NewRegistry()
	watcher := NewWatcher(dir, loader, registry, nil, domain.NewDynamicSourceRegistry(), logging.NewNopLogger())

	// Cancel the context immediately.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = watcher.Start(ctx)
	if err != nil {
		t.Errorf("expected nil when context already cancelled, got: %v", err)
	}
}

func TestWatcher_Start_AddDirError(t *testing.T) {
	compiler, err := NewCELCompiler()
	if err != nil {
		t.Fatalf("NewCELCompiler: %v", err)
	}
	logger := logging.NewNopLogger()
	loader, err := NewLoader(compiler, func(s string) string { return "" }, false, logger)
	if err != nil {
		t.Fatalf("NewLoader: %v", err)
	}
	registry := NewRegistry()
	// Use a non-existent directory to trigger watcher.Add error.
	watcher := NewWatcher("/nonexistent/path/that/does/not/exist", loader, registry, nil, domain.NewDynamicSourceRegistry(), logger)

	// Start should return an error because the directory does not exist.
	err = watcher.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
	t.Logf("Got expected error: %v", err)
}

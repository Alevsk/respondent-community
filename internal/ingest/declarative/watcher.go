package declarative

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Alevsk/respondent/internal/domain"
	"github.com/Alevsk/respondent/internal/logging"
)

const (
	// debounceQuietTime is the duration of quiet time before triggering a reload.
	debounceQuietTime = 2 * time.Second
	// minReloadInterval is the minimum time between reloads per source.
	minReloadInterval = 30 * time.Second
)

// AdapterManager is the interface that the watcher uses to manage adapter lifecycle.
// This avoids importing the ingest or adapters package (circular dependency prevention).
type AdapterManager interface {
	// StartDeclarativeAdapter registers and starts a new declarative adapter.
	StartDeclarativeAdapter(ctx context.Context, cs *CompiledSource) error
	// StopDeclarativeAdapter stops and unregisters a declarative adapter by source name.
	StopDeclarativeAdapter(name string) error
	// SwapDeclarativeAdapter swaps the CompiledSource for a running declarative adapter.
	SwapDeclarativeAdapter(name string, cs *CompiledSource) error
}

// Watcher watches a directory of YAML source definitions and manages hot-reload.
// It debounces file events and coordinates with the adapter manager for lifecycle changes.
type Watcher struct {
	dir      string
	loader   *Loader
	registry *Registry
	manager  AdapterManager
	dynReg   *domain.DynamicSourceRegistry
	logger   *logging.Logger

	// Track last reload time per source for rate limiting
	mu           sync.Mutex
	lastReload   map[string]time.Time
	knownSources map[string]string // filename -> source name
}

// NewWatcher creates a new source file watcher.
func NewWatcher(dir string, loader *Loader, registry *Registry, manager AdapterManager, dynReg *domain.DynamicSourceRegistry, logger *logging.Logger) *Watcher {
	return &Watcher{
		dir:          dir,
		loader:       loader,
		registry:     registry,
		manager:      manager,
		dynReg:       dynReg,
		logger:       logger.WithSource("declarative-watcher"),
		lastReload:   make(map[string]time.Time),
		knownSources: make(map[string]string),
	}
}

// Start begins watching the sources directory for changes.
// It blocks until the context is cancelled.
func (w *Watcher) Start(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer func() { _ = watcher.Close() }()

	if err := watcher.Add(w.dir); err != nil {
		return err
	}

	w.logger.Info("watching sources directory",
		logging.String("dir", w.dir),
	)

	// Initialize known sources from initial load
	w.initKnownSources()

	// Debounce: accumulate events per file
	pending := make(map[string]time.Time)
	var pendingMu sync.Mutex

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if !isYAMLFile(event.Name) {
				continue
			}

			pendingMu.Lock()
			pending[event.Name] = time.Now()
			pendingMu.Unlock()

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Error("fsnotify error",
				logging.Err("error", err),
			)

		case <-ticker.C:
			pendingMu.Lock()
			now := time.Now()
			var ready []string
			for file, lastEvent := range pending {
				if now.Sub(lastEvent) >= debounceQuietTime {
					ready = append(ready, file)
				}
			}
			for _, file := range ready {
				delete(pending, file)
			}
			pendingMu.Unlock()

			for _, file := range ready {
				w.handleFileChange(ctx, file)
			}
		}
	}
}

// initKnownSources populates the filename -> source name map from the registry.
func (w *Watcher) initKnownSources() {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, cs := range w.registry.List() {
		// We don't know the original filename, but we track by source name.
		// New files will be tracked as they are detected.
		w.knownSources[cs.Definition().Name] = cs.Definition().Name
	}
}

// handleFileChange processes a file change event after debounce.
func (w *Watcher) handleFileChange(ctx context.Context, file string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	basename := filepath.Base(file)

	// Check if the file was deleted
	cs, err := w.loader.LoadFile(file)
	if err != nil {
		// File may have been deleted or is invalid YAML.
		// Check if we had a known source for this filename.
		if sourceName, known := w.knownSources[basename]; known {
			w.handleSourceRemoval(sourceName, basename)
			return
		}

		// Unknown file with errors -- log and skip.
		w.logger.Error("failed to load source definition",
			logging.String("file", file),
			logging.Err("error", err),
		)
		return
	}

	sourceName := cs.Definition().Name

	// Rate limit: max 1 reload per 30s per source
	if lastTime, exists := w.lastReload[sourceName]; exists {
		if time.Since(lastTime) < minReloadInterval {
			w.logger.Debug("skipping reload (rate limited)",
				logging.String("source_name", sourceName),
			)
			return
		}
	}

	// Check if this is a new source or modification
	_, exists := w.registry.Get(sourceName)

	if exists {
		// Source modification: swap CompiledSource
		w.registry.Add(sourceName, cs)
		if w.manager != nil {
			if err := w.manager.SwapDeclarativeAdapter(sourceName, cs); err != nil {
				w.logger.Error("failed to swap declarative adapter",
					logging.String("source_name", sourceName),
					logging.Err("error", err),
				)
				return
			}
		}
		w.logger.Info("hot-reloaded source definition",
			logging.String("source_name", sourceName),
			logging.String("file", file),
		)
	} else {
		// New source: register and start
		def := cs.Definition()
		w.dynReg.Register(
			domain.SourceType(def.SourceType),
			domain.LayerType(def.LayerType),
		)
		w.registry.Add(sourceName, cs)
		if w.manager != nil {
			if err := w.manager.StartDeclarativeAdapter(ctx, cs); err != nil {
				w.logger.Error("failed to start declarative adapter",
					logging.String("source_name", sourceName),
					logging.Err("error", err),
				)
				return
			}
		}
		w.logger.Info("added new source definition",
			logging.String("source_name", sourceName),
			logging.String("source_type", def.SourceType),
			logging.String("layer_type", def.LayerType),
			logging.String("file", file),
		)
	}

	w.knownSources[basename] = sourceName
	w.lastReload[sourceName] = time.Now()
}

// handleSourceRemoval stops and unregisters a source that was removed.
func (w *Watcher) handleSourceRemoval(sourceName, basename string) {
	cs, exists := w.registry.Get(sourceName)
	if !exists {
		return
	}

	def := cs.Definition()

	if w.manager != nil {
		if err := w.manager.StopDeclarativeAdapter(sourceName); err != nil {
			w.logger.Error("failed to stop declarative adapter",
				logging.String("source_name", sourceName),
				logging.Err("error", err),
			)
		}
	}

	w.registry.Remove(sourceName)
	w.dynReg.Unregister(domain.SourceType(def.SourceType))

	delete(w.knownSources, basename)
	delete(w.lastReload, sourceName)

	w.logger.Info("removed source definition",
		logging.String("source_name", sourceName),
	)
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".yaml" || ext == ".yml"
}

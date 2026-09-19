package ingest

import (
	"context"
	"sync"
	"time"

	"github.com/Alevsk/respondent/internal/domain"
)

// LayerSource is the interface that all data source adapters must implement.
type LayerSource interface {
	// Start begins the ingestion process
	Start(ctx context.Context) error

	// Stop gracefully stops the ingestion process
	Stop() error

	// Snapshot returns all current entities for this layer
	Snapshot(ctx context.Context) ([]*domain.Entity, []*domain.Observation, error)

	// Stream sends entity updates to the output channel
	Stream(ctx context.Context, out chan<- *domain.EntityUpdate) error

	// Name returns the source name
	Name() string

	// LayerType returns the layer type identifier
	LayerType() string

	// SupportsSourceType returns true if this adapter handles the given source type.
	SupportsSourceType(sourceType domain.SourceType) bool
}

// SourceConfig holds configuration for a data source
type SourceConfig struct {
	Name      string
	Type      string
	Config    map[string]any
	Enabled   bool
	Interval  time.Duration
	RateLimit int
}

// SourceRegistry manages registered source adapters.
// It supports lookup by both name and source type.
type SourceRegistry struct {
	mu      sync.RWMutex
	sources map[string]LayerSource
}

// NewSourceRegistry creates a new source registry.
func NewSourceRegistry() *SourceRegistry {
	return &SourceRegistry{
		sources: make(map[string]LayerSource),
	}
}

// Register adds a source to the registry.
func (r *SourceRegistry) Register(source LayerSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[source.Name()] = source
}

// Get retrieves a source by name.
func (r *SourceRegistry) Get(name string) (LayerSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	source, ok := r.sources[name]
	return source, ok
}

// GetByType retrieves a source that supports the given source type.
// Returns the first matching source, or false if none found.
func (r *SourceRegistry) GetByType(sourceType domain.SourceType) (LayerSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, source := range r.sources {
		if source.SupportsSourceType(sourceType) {
			return source, true
		}
	}
	return nil, false
}

// List returns all registered sources.
func (r *SourceRegistry) List() []LayerSource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sources := make([]LayerSource, 0, len(r.sources))
	for _, source := range r.sources {
		sources = append(sources, source)
	}
	return sources
}

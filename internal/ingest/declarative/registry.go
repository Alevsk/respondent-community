package declarative

import (
	"sync"
)

// Registry is a thread-safe in-memory registry mapping source names to CompiledSource.
type Registry struct {
	mu      sync.RWMutex
	sources map[string]*CompiledSource
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		sources: make(map[string]*CompiledSource),
	}
}

// Add registers a compiled source by name. Overwrites any existing entry.
func (r *Registry) Add(name string, cs *CompiledSource) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sources[name] = cs
}

// Remove removes a compiled source by name. No-op if the name does not exist.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sources, name)
}

// Get retrieves a compiled source by name. Returns nil, false if not found.
func (r *Registry) Get(name string) (*CompiledSource, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cs, ok := r.sources[name]
	return cs, ok
}

// List returns all registered compiled sources as a slice.
func (r *Registry) List() []*CompiledSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*CompiledSource, 0, len(r.sources))
	for _, cs := range r.sources {
		result = append(result, cs)
	}
	return result
}

package targets

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"
)

// Registry manages loaded target definitions and provides delivery routing.
// For now, Deliver() is a placeholder that logs the delivery attempt.
// Actual delivery adapters (Slack webhook, email, Discord) will be
// implemented in future phases.
type Registry struct {
	mu      sync.RWMutex
	targets map[string]*TargetDefinition
	logger  zerolog.Logger
}

// NewRegistry creates an empty target registry.
func NewRegistry(logger zerolog.Logger) *Registry {
	return &Registry{
		targets: make(map[string]*TargetDefinition),
		logger:  logger,
	}
}

// Register adds a target definition to the registry.
func (r *Registry) Register(def *TargetDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.targets[def.Name] = def
}

// LoadFromDefinitions populates the registry from a map of loaded definitions.
func (r *Registry) LoadFromDefinitions(defs map[string]*TargetDefinition) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for name, def := range defs {
		r.targets[name] = def
		r.logger.Debug().
			Str("name", name).
			Str("type", def.Type).
			Bool("enabled", def.Enabled).
			Msg("registered target")
	}
}

// Get returns the target definition for the given name, or false if not found.
func (r *Registry) Get(name string) (*TargetDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.targets[name]
	return def, ok
}

// List returns all registered target definitions.
func (r *Registry) List() []*TargetDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*TargetDefinition, 0, len(r.targets))
	for _, def := range r.targets {
		result = append(result, def)
	}
	return result
}

// Deliver routes a payload to the named target.
// This is currently a placeholder that logs the delivery attempt.
// Actual delivery adapters will be implemented in a future phase.
func (r *Registry) Deliver(ctx context.Context, targetName string, payload any) error {
	r.mu.RLock()
	def, ok := r.targets[targetName]
	r.mu.RUnlock()

	if !ok {
		return fmt.Errorf("target %q not found", targetName)
	}

	if !def.Enabled {
		r.logger.Debug().
			Str("target", targetName).
			Msg("skipping delivery to disabled target")
		return nil
	}

	// Check context cancellation.
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	r.logger.Info().
		Str("target", targetName).
		Str("type", def.Type).
		Bool("ai_enabled", def.AI.Enabled).
		Msg("placeholder delivery: actual adapter not yet implemented")

	return nil
}

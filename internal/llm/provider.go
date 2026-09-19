// Package llm provides a multi-provider LLM integration layer.
package llm

import (
	"context"
	"errors"
	"sync"
)

var (
	// ErrProviderNotFound is returned when a provider is not found in the registry.
	ErrProviderNotFound = errors.New("llm provider not found for provider type")
	// ErrCompletionFailed is returned when a completion request fails.
	ErrCompletionFailed = errors.New("completion failed")
	// ErrProviderUnavailable is returned when the provider is not available.
	ErrProviderUnavailable = errors.New("provider is unavailable")
	// ErrRateLimited is returned when the provider rate limit is exceeded.
	ErrRateLimited = errors.New("provider rate limit exceeded")
	// ErrInvalidResponse is returned when the provider returns an invalid response.
	ErrInvalidResponse = errors.New("invalid response from provider")
)

// Provider performs LLM chat completions.
type Provider interface {
	// Complete sends a chat completion request and returns the response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Name returns the provider identifier.
	Name() string

	// SupportsProvider returns true if this handles the given provider type.
	SupportsProvider(providerType string) bool

	// HealthCheck verifies the provider is available.
	HealthCheck(ctx context.Context) error
}

// Message represents a single chat message.
type Message struct {
	Role    string // "system", "user", "assistant"
	Content string
}

// ResponseFormat controls the output format of LLM responses.
type ResponseFormat string

const (
	// ResponseFormatText is the default — provider returns free-form text.
	ResponseFormatText ResponseFormat = ""
	// ResponseFormatJSON instructs providers that support it to return valid JSON.
	ResponseFormatJSON ResponseFormat = "json_object"
)

// CompletionRequest contains parameters for a chat completion.
type CompletionRequest struct {
	Messages       []Message
	MaxTokens      int            // Override provider default if > 0.
	Temperature    float64        // 0.0–2.0; 0 uses provider default.
	ResponseFormat ResponseFormat // Optional; providers that support JSON mode use this.
}

// CompletionResponse contains the result of a chat completion.
type CompletionResponse struct {
	Content      string
	Model        string
	FinishReason string
	Usage        Usage
}

// Usage reports token consumption for a completion.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Registry manages available LLM providers.
type Registry interface {
	Register(provider Provider)
	GetProvider(providerType string) (Provider, error)
	GetPreferredProvider(ctx context.Context) (Provider, error)
	ListProviders() []string
}

// DefaultRegistry is the default implementation of Registry.
type DefaultRegistry struct {
	mu            sync.RWMutex
	providers     []Provider
	priorities    map[string]int
	preferredType string // provider type set via SetPreferred; used by GetPreferredProvider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *DefaultRegistry {
	return &DefaultRegistry{
		providers:  make([]Provider, 0),
		priorities: make(map[string]int),
	}
}

// SetPreferred records the user-configured provider type (from llm.provider).
// GetPreferredProvider will return this provider first if it is registered and
// healthy, falling back to priority-based selection otherwise.
func (r *DefaultRegistry) SetPreferred(providerType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preferredType = providerType
}

// Register adds a provider to the registry.
func (r *DefaultRegistry) Register(provider Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
}

// RegisterWithPriority adds a provider with a specific priority.
func (r *DefaultRegistry) RegisterWithPriority(provider Provider, priority int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
	r.priorities[provider.Name()] = priority
}

// GetProvider returns the provider for the given type.
func (r *DefaultRegistry) GetProvider(providerType string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.SupportsProvider(providerType) {
			return provider, nil
		}
	}
	return nil, ErrProviderNotFound
}

// GetPreferredProvider returns the user-configured provider if one was set via
// SetPreferred and it is registered and healthy. Otherwise it falls back to
// the highest-priority healthy provider in the registry.
func (r *DefaultRegistry) GetPreferredProvider(ctx context.Context) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Try the explicitly configured provider first.
	if r.preferredType != "" {
		for _, provider := range r.providers {
			if provider.SupportsProvider(r.preferredType) {
				if err := provider.HealthCheck(ctx); err == nil {
					return provider, nil
				}
				break // found but unhealthy — fall through to priority selection
			}
		}
	}

	// Fallback: highest-priority healthy provider.
	var bestProvider Provider
	bestPriority := -1

	for _, provider := range r.providers {
		priority := r.priorities[provider.Name()]
		if priority > bestPriority {
			if err := provider.HealthCheck(ctx); err == nil {
				bestProvider = provider
				bestPriority = priority
			}
		}
	}

	if bestProvider == nil {
		return nil, ErrProviderUnavailable
	}
	return bestProvider, nil
}

// ListProviders returns all registered provider names.
func (r *DefaultRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, len(r.providers))
	for i, p := range r.providers {
		names[i] = p.Name()
	}
	return names
}

// ProviderType constants for LLM providers.
const (
	ProviderTypeOpenAI    = "openai"
	ProviderTypeAnthropic = "anthropic"
	ProviderTypeGemini    = "gemini"
	ProviderTypeXAI       = "xai"
	ProviderTypeZAI       = "zai"
	ProviderTypeOllama    = "ollama"
	ProviderTypeLMStudio  = "lmstudio"
)

// ValidProviderType returns true if the provider type is valid.
func ValidProviderType(providerType string) bool {
	switch providerType {
	case ProviderTypeOpenAI, ProviderTypeAnthropic, ProviderTypeGemini,
		ProviderTypeXAI, ProviderTypeZAI, ProviderTypeOllama, ProviderTypeLMStudio:
		return true
	}
	return false
}

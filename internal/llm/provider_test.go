package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockProvider struct {
	name           string
	supportedType  string
	completeErr    error
	completeResp   *CompletionResponse
	healthCheckErr error
}

func (m *mockProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if m.completeErr != nil {
		return nil, m.completeErr
	}
	if m.completeResp != nil {
		return m.completeResp, nil
	}
	return &CompletionResponse{Content: "test response"}, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) SupportsProvider(providerType string) bool {
	return providerType == m.supportedType
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return m.healthCheckErr
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.providers)
	assert.NotNil(t, registry.priorities)
	assert.Empty(t, registry.providers)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()
	provider := &mockProvider{name: "test", supportedType: "test"}

	registry.Register(provider)

	assert.Len(t, registry.providers, 1)
	assert.Equal(t, provider, registry.providers[0])
}

func TestRegistry_RegisterMultiple(t *testing.T) {
	registry := NewRegistry()
	provider1 := &mockProvider{name: "test1", supportedType: "test1"}
	provider2 := &mockProvider{name: "test2", supportedType: "test2"}

	registry.Register(provider1)
	registry.Register(provider2)

	assert.Len(t, registry.providers, 2)
}

func TestRegistry_RegisterWithPriority(t *testing.T) {
	registry := NewRegistry()
	provider := &mockProvider{name: "test", supportedType: "test"}

	registry.RegisterWithPriority(provider, 10)

	assert.Len(t, registry.providers, 1)
	assert.Equal(t, 10, registry.priorities["test"])
}

func TestRegistry_RegisterWithPriorityMultiple(t *testing.T) {
	registry := NewRegistry()
	provider1 := &mockProvider{name: "low", supportedType: "low"}
	provider2 := &mockProvider{name: "high", supportedType: "high"}

	registry.RegisterWithPriority(provider1, 1)
	registry.RegisterWithPriority(provider2, 100)

	assert.Len(t, registry.providers, 2)
	assert.Equal(t, 1, registry.priorities["low"])
	assert.Equal(t, 100, registry.priorities["high"])
}

func TestRegistry_GetProvider(t *testing.T) {
	registry := NewRegistry()
	provider := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI}
	registry.Register(provider)

	got, err := registry.GetProvider(ProviderTypeOpenAI)

	require.NoError(t, err)
	assert.Equal(t, provider, got)
}

func TestRegistry_GetProvider_NotFound(t *testing.T) {
	registry := NewRegistry()
	provider := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI}
	registry.Register(provider)

	got, err := registry.GetProvider(ProviderTypeAnthropic)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestRegistry_GetProvider_EmptyRegistry(t *testing.T) {
	registry := NewRegistry()

	got, err := registry.GetProvider(ProviderTypeOpenAI)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrProviderNotFound)
}

func TestRegistry_GetProvider_MultipleProviders(t *testing.T) {
	registry := NewRegistry()
	provider1 := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI}
	provider2 := &mockProvider{name: "anthropic", supportedType: ProviderTypeAnthropic}
	registry.Register(provider1)
	registry.Register(provider2)

	got, err := registry.GetProvider(ProviderTypeAnthropic)

	require.NoError(t, err)
	assert.Equal(t, provider2, got)
}

func TestRegistry_GetPreferredProvider(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	provider := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI, healthCheckErr: nil}
	registry.RegisterWithPriority(provider, 10)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, provider, got)
}

func TestRegistry_GetPreferredProvider_NoHealthyProviders(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	provider := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI, healthCheckErr: errors.New("unhealthy")}
	registry.RegisterWithPriority(provider, 10)

	got, err := registry.GetPreferredProvider(ctx)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestRegistry_GetPreferredProvider_SelectsHighestPriority(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	lowProvider := &mockProvider{name: "low", supportedType: "low"}
	highProvider := &mockProvider{name: "high", supportedType: "high"}
	registry.RegisterWithPriority(lowProvider, 1)
	registry.RegisterWithPriority(highProvider, 100)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, highProvider, got)
}

func TestRegistry_GetPreferredProvider_SkipsUnhealthyLowerPriority(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	highProvider := &mockProvider{name: "high", supportedType: "high", healthCheckErr: errors.New("unhealthy")}
	lowProvider := &mockProvider{name: "low", supportedType: "low"}
	registry.RegisterWithPriority(highProvider, 100)
	registry.RegisterWithPriority(lowProvider, 1)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, lowProvider, got)
}

func TestRegistry_GetPreferredProvider_EmptyRegistry(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	got, err := registry.GetPreferredProvider(ctx)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestRegistry_GetPreferredProvider_NoPriority(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	provider := &mockProvider{name: "test", supportedType: "test"}
	registry.Register(provider)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, provider, got)
}

func TestRegistry_GetPreferredProvider_MixedPriorityAndNoPriority(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	noPrioProvider := &mockProvider{name: "noprio", supportedType: "noprio"}
	highProvider := &mockProvider{name: "high", supportedType: "high"}
	registry.Register(noPrioProvider)
	registry.RegisterWithPriority(highProvider, 10)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, highProvider, got)
}

func TestRegistry_ListProviders(t *testing.T) {
	registry := NewRegistry()
	provider1 := &mockProvider{name: "openai", supportedType: ProviderTypeOpenAI}
	provider2 := &mockProvider{name: "anthropic", supportedType: ProviderTypeAnthropic}
	registry.Register(provider1)
	registry.Register(provider2)

	names := registry.ListProviders()

	assert.Len(t, names, 2)
	assert.Contains(t, names, "openai")
	assert.Contains(t, names, "anthropic")
}

func TestRegistry_ListProviders_Empty(t *testing.T) {
	registry := NewRegistry()

	names := registry.ListProviders()

	assert.Empty(t, names)
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	registry := NewRegistry()

	done := make(chan bool)

	for i := 0; i < 10; i++ {
		go func(id int) {
			provider := &mockProvider{name: "test", supportedType: "test"}
			registry.Register(provider)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		go func() {
			_ = registry.ListProviders()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		go func() {
			_, _ = registry.GetProvider("test")
			done <- true
		}()
	}

	for i := 0; i < 30; i++ {
		<-done
	}
}

func TestValidProviderType(t *testing.T) {
	tests := []struct {
		providerType string
		expected     bool
	}{
		{ProviderTypeOpenAI, true},
		{ProviderTypeAnthropic, true},
		{ProviderTypeGemini, true},
		{ProviderTypeXAI, true},
		{ProviderTypeZAI, true},
		{ProviderTypeOllama, true},
		{ProviderTypeLMStudio, true},
		{"invalid", false},
		{"", false},
		{"openai ", false},
		{" openai", false},
		{"OPENAI", false},
	}

	for _, tt := range tests {
		t.Run(tt.providerType, func(t *testing.T) {
			assert.Equal(t, tt.expected, ValidProviderType(tt.providerType))
		})
	}
}

func TestProviderTypeConstants(t *testing.T) {
	assert.Equal(t, "openai", ProviderTypeOpenAI)
	assert.Equal(t, "anthropic", ProviderTypeAnthropic)
	assert.Equal(t, "gemini", ProviderTypeGemini)
	assert.Equal(t, "xai", ProviderTypeXAI)
	assert.Equal(t, "zai", ProviderTypeZAI)
	assert.Equal(t, "ollama", ProviderTypeOllama)
	assert.Equal(t, "lmstudio", ProviderTypeLMStudio)
}

func TestErrors(t *testing.T) {
	assert.Error(t, ErrProviderNotFound)
	assert.Error(t, ErrCompletionFailed)
	assert.Error(t, ErrProviderUnavailable)
	assert.Error(t, ErrRateLimited)
	assert.Error(t, ErrInvalidResponse)

	assert.Equal(t, "llm provider not found for provider type", ErrProviderNotFound.Error())
	assert.Equal(t, "completion failed", ErrCompletionFailed.Error())
	assert.Equal(t, "provider is unavailable", ErrProviderUnavailable.Error())
	assert.Equal(t, "provider rate limit exceeded", ErrRateLimited.Error())
	assert.Equal(t, "invalid response from provider", ErrInvalidResponse.Error())
}

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello, world!",
	}

	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello, world!", msg.Content)
}

func TestCompletionRequest(t *testing.T) {
	req := CompletionRequest{
		Messages: []Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hi"},
		},
		MaxTokens:   100,
		Temperature: 0.7,
	}

	assert.Len(t, req.Messages, 2)
	assert.Equal(t, 100, req.MaxTokens)
	assert.Equal(t, 0.7, req.Temperature)
}

func TestCompletionResponse(t *testing.T) {
	resp := CompletionResponse{
		Content:      "Hello!",
		Model:        "gpt-4",
		FinishReason: "stop",
		Usage: Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	assert.Equal(t, "Hello!", resp.Content)
	assert.Equal(t, "gpt-4", resp.Model)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestUsage(t *testing.T) {
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	assert.Equal(t, 100, usage.PromptTokens)
	assert.Equal(t, 50, usage.CompletionTokens)
	assert.Equal(t, 150, usage.TotalTokens)
}

func TestRegistry_GetPreferredProvider_PriorityOrder(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	provider1 := &mockProvider{name: "p1", supportedType: "p1"}
	provider2 := &mockProvider{name: "p2", supportedType: "p2"}
	provider3 := &mockProvider{name: "p3", supportedType: "p3"}

	registry.RegisterWithPriority(provider1, 5)
	registry.RegisterWithPriority(provider2, 50)
	registry.RegisterWithPriority(provider3, 10)

	got, err := registry.GetPreferredProvider(ctx)

	require.NoError(t, err)
	assert.Equal(t, provider2, got)
}

func TestRegistry_SetPreferred_ReturnsConfiguredProvider(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	anthropic := &mockProvider{name: "anthropic", supportedType: ProviderTypeAnthropic}
	xai := &mockProvider{name: "xai", supportedType: ProviderTypeXAI}
	registry.RegisterWithPriority(anthropic, 9) // higher priority
	registry.RegisterWithPriority(xai, 7)       // lower priority

	// Without SetPreferred, anthropic wins by priority.
	got, err := registry.GetPreferredProvider(ctx)
	require.NoError(t, err)
	assert.Equal(t, anthropic, got)

	// After SetPreferred("xai"), xai wins despite lower priority.
	registry.SetPreferred(ProviderTypeXAI)
	got, err = registry.GetPreferredProvider(ctx)
	require.NoError(t, err)
	assert.Equal(t, xai, got)
}

func TestRegistry_SetPreferred_FallsBackWhenUnhealthy(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	anthropic := &mockProvider{name: "anthropic", supportedType: ProviderTypeAnthropic}
	xai := &mockProvider{name: "xai", supportedType: ProviderTypeXAI, healthCheckErr: errors.New("unhealthy")}
	registry.RegisterWithPriority(anthropic, 9)
	registry.RegisterWithPriority(xai, 7)
	registry.SetPreferred(ProviderTypeXAI)

	// Preferred xai is unhealthy → falls back to anthropic by priority.
	got, err := registry.GetPreferredProvider(ctx)
	require.NoError(t, err)
	assert.Equal(t, anthropic, got)
}

func TestRegistry_SetPreferred_NotRegistered(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	anthropic := &mockProvider{name: "anthropic", supportedType: ProviderTypeAnthropic}
	registry.RegisterWithPriority(anthropic, 9)
	registry.SetPreferred(ProviderTypeXAI) // not registered

	// Preferred not found → falls back to priority-based selection.
	got, err := registry.GetPreferredProvider(ctx)
	require.NoError(t, err)
	assert.Equal(t, anthropic, got)
}

func TestRegistry_GetPreferredProvider_AllUnhealthy(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()
	provider1 := &mockProvider{name: "p1", supportedType: "p1", healthCheckErr: errors.New("err1")}
	provider2 := &mockProvider{name: "p2", supportedType: "p2", healthCheckErr: errors.New("err2")}
	provider3 := &mockProvider{name: "p3", supportedType: "p3", healthCheckErr: errors.New("err3")}

	registry.RegisterWithPriority(provider1, 100)
	registry.RegisterWithPriority(provider2, 50)
	registry.RegisterWithPriority(provider3, 10)

	got, err := registry.GetPreferredProvider(ctx)

	assert.Nil(t, got)
	assert.ErrorIs(t, err, ErrProviderUnavailable)
}

func TestResponseFormat_Constants(t *testing.T) {
	assert.Equal(t, ResponseFormat(""), ResponseFormatText)
	assert.Equal(t, ResponseFormat("json_object"), ResponseFormatJSON)
}

func TestCompletionRequest_ResponseFormat(t *testing.T) {
	req := &CompletionRequest{
		Messages:       []Message{{Role: "user", Content: "test"}},
		ResponseFormat: ResponseFormatJSON,
	}
	assert.Equal(t, ResponseFormatJSON, req.ResponseFormat)
}
